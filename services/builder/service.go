package builder

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/estafette/estafette-ci-builder/api"
	"github.com/estafette/estafette-ci-builder/clients/docker"
	"github.com/estafette/estafette-ci-builder/clients/envvar"
	"github.com/estafette/estafette-ci-builder/clients/estafetteciapi"
	"github.com/estafette/estafette-ci-builder/clients/obfuscation"
	"github.com/estafette/estafette-ci-builder/clients/readiness"
	"github.com/estafette/estafette-ci-builder/services/pipeline"
	contracts "github.com/estafette/estafette-ci-contracts"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
)

// Service runs builds for different types of integrations
//go:generate mockgen -package=builder -destination ./mock.go -source=service.go
type Service interface {
	RunReadinessProbe(protocol, host string, port int, path, hostname string, timeoutSeconds int)
	RunEstafetteBuildJob(runAsJob bool)
}

// NewService returns a new CIBuilder
func NewService(ctx context.Context, applicationInfo foundation.ApplicationInfo, pipelineService pipeline.Service, dockerClient docker.Client, envvarClient envvar.Client, obfuscationClient obfuscation.Client, estafetteciapiClient estafetteciapi.Client, readinessClient readiness.Client, builderConfig contracts.BuilderConfig, credentialsBytes []byte) (Service, error) {
	return &service{
		applicationInfo:      applicationInfo,
		pipelineService:      pipelineService,
		dockerClient:         dockerClient,
		envvarClient:         envvarClient,
		obfuscationClient:    obfuscationClient,
		estafetteciapiClient: estafetteciapiClient,
		readinessClient:      readinessClient,
		builderConfig:        builderConfig,
		credentialsBytes:     credentialsBytes,
	}, nil
}

type service struct {
	applicationInfo      foundation.ApplicationInfo
	pipelineService      pipeline.Service
	dockerClient         docker.Client
	envvarClient         envvar.Client
	obfuscationClient    obfuscation.Client
	estafetteciapiClient estafetteciapi.Client
	readinessClient      readiness.Client
	builderConfig        contracts.BuilderConfig
	credentialsBytes     []byte
}

func (s *service) RunReadinessProbe(protocol, host string, port int, path, hostname string, timeoutSeconds int) {
	err := s.readinessClient.WaitForReadiness(protocol, host, port, path, hostname, timeoutSeconds)
	if err != nil {
		log.Fatal().Err(err).Msgf("Readiness probe failed")
	}

	// readiness probe succeeded, exiting cleanly
	os.Exit(0)
}

func (s *service) RunEstafetteBuildJob(runAsJob bool) {

	closer := s.initJaeger(s.applicationInfo.App)
	defer closer.Close()

	// unset all ESTAFETTE_ envvars so they don't get abused by non-estafette components
	s.envvarClient.UnsetEstafetteEnvvars()

	s.envvarClient.SetEstafetteBuilderConfigEnvvars(s.builderConfig)

	buildLog := contracts.BuildLog{
		RepoSource:   s.builderConfig.Git.RepoSource,
		RepoOwner:    s.builderConfig.Git.RepoOwner,
		RepoName:     s.builderConfig.Git.RepoName,
		RepoBranch:   s.builderConfig.Git.RepoBranch,
		RepoRevision: s.builderConfig.Git.RepoRevision,
		Steps:        make([]*contracts.BuildLogStep, 0),
	}

	if os.Getenv("ESTAFETTE_LOG_FORMAT") == "v3" {
		// set some default fields added to all logs
		log.Logger = log.Logger.With().
			Str("jobName", *s.builderConfig.JobName).
			Interface("git", s.builderConfig.Git).
			Logger()
	}

	rootSpanName := "RunBuildJob"
	if *s.builderConfig.Action == "release" {
		rootSpanName = "RunReleaseJob"
	}

	rootSpan := opentracing.StartSpan(rootSpanName)
	defer rootSpan.Finish()

	ctx := context.Background()
	ctx = opentracing.ContextWithSpan(ctx, rootSpan)

	// set running state, so a restarted job will show up as running once a new pod runs
	_ = s.estafetteciapiClient.SendBuildStartedEvent(ctx)

	// start docker daemon
	dockerDaemonStartSpan, _ := opentracing.StartSpanFromContext(ctx, "StartDockerDaemon")
	err := s.dockerClient.StartDockerDaemon()
	if err != nil {
		s.estafetteciapiClient.HandleFatal(ctx, buildLog, err, "Error starting docker daemon")
	}

	// wait for docker daemon to be ready for usage
	s.dockerClient.WaitForDockerDaemon()
	dockerDaemonStartSpan.Finish()

	// listen to cancellation in order to stop any running pipeline or container
	go s.pipelineService.StopPipelineOnCancellation()

	// get current working directory
	dir := s.envvarClient.GetWorkDir()
	if dir == "" {
		s.estafetteciapiClient.HandleFatal(ctx, buildLog, nil, "Getting working directory from environment variable ESTAFETTE_WORKDIR failed")
	}

	// set some envvars
	err = s.envvarClient.SetEstafetteGlobalEnvvars()
	if err != nil {
		s.estafetteciapiClient.HandleFatal(ctx, buildLog, err, "Setting global environment variables failed")
	}

	// initialize obfuscationClient
	err = s.obfuscationClient.CollectSecrets(*s.builderConfig.Manifest, s.credentialsBytes, s.envvarClient.GetPipelineName())
	if err != nil {
		s.estafetteciapiClient.HandleFatal(ctx, buildLog, err, "Collecting secrets to obfuscate failed")
	}

	// check whether this is a regular build or a release
	stages := s.builderConfig.Manifest.Stages
	if *s.builderConfig.Action == "release" {
		// check if the release is defined
		releaseExists := false
		for _, r := range s.builderConfig.Manifest.Releases {
			if r.Name == s.builderConfig.ReleaseParams.ReleaseName {
				releaseExists = true
				stages = r.Stages
			}
		}
		if !releaseExists {
			s.estafetteciapiClient.HandleFatal(ctx, buildLog, nil, fmt.Sprintf("Release %v does not exist", s.builderConfig.ReleaseParams.ReleaseName))
		}
		log.Info().Msgf("Starting release %v at version %v...", s.builderConfig.ReleaseParams.ReleaseName, s.builderConfig.BuildVersion.Version)
	} else {
		log.Info().Msgf("Starting build version %v...", s.builderConfig.BuildVersion.Version)
	}

	// create docker client
	_, err = s.dockerClient.CreateDockerClient()
	if err != nil {
		s.estafetteciapiClient.HandleFatal(ctx, buildLog, err, "Failed creating a docker client")
	}

	// collect estafette envvars and run stages from manifest
	log.Info().Msgf("Running %v stages", len(stages))
	estafetteEnvvars := s.envvarClient.CollectEstafetteEnvvarsAndLabels(*s.builderConfig.Manifest)
	globalEnvvars := s.envvarClient.CollectGlobalEnvvars(*s.builderConfig.Manifest)
	envvars := s.envvarClient.OverrideEnvvars(estafetteEnvvars, globalEnvvars)

	// run stages
	s.pipelineService.EnableBuilderInfoStageInjection()
	buildLog.Steps, err = s.pipelineService.RunStages(ctx, 0, stages, dir, envvars)
	if err != nil && buildLog.HasUnknownStatus() {
		s.estafetteciapiClient.HandleFatal(ctx, buildLog, err, "Executing stages from manifest failed")
	}

	// send result to ci-api
	buildStatus := contracts.GetAggregatedStatus(buildLog.Steps)
	_ = s.estafetteciapiClient.SendBuildFinishedEvent(ctx, buildStatus)
	_ = s.estafetteciapiClient.SendBuildJobLogEvent(ctx, buildLog)
	_ = s.estafetteciapiClient.SendBuildCleanEvent(ctx, buildStatus)

	// finish and flush so it gets sent to the tracing backend
	rootSpan.Finish()
	closer.Close()

	if runAsJob {
		os.Exit(0)
	} else {
		api.HandleExit(buildLog.Steps)
	}
}

// initJaeger returns an instance of Jaeger Tracer that can be configured with environment variables
// https://github.com/jaegertracing/jaeger-client-go#environment-variables
func (s *service) initJaeger(service string) io.Closer {

	cfg, err := jaegercfg.FromEnv()
	if err != nil {
		log.Fatal().Err(err).Msg("Generating Jaeger config from environment variables failed")
	}

	closer, err := cfg.InitGlobalTracer(service, jaegercfg.Logger(jaeger.StdLogger))

	if err != nil {
		log.Fatal().Err(err).Msg("Generating Jaeger tracer failed")
	}

	return closer
}
