package builder

import (
	"context"
	"fmt"
	"io"
	"os"

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
	RunEstafetteBuildJob(pipelineRunner PipelineRunner, dockerRunner DockerRunner, envvarHelper EnvvarHelper, obfuscator Obfuscator, endOfLifeHelper EndOfLifeHelper, builderConfig contracts.BuilderConfig, credentialsBytes []byte, runAsJob bool)
}

type service struct {
	applicationInfo foundation.ApplicationInfo
}

// NewCIBuilder returns a new CIBuilder
func NewCIBuilder(applicationInfo foundation.ApplicationInfo) CIBuilder {
	return &service{
		applicationInfo: applicationInfo,
	}
}

func (b *service) RunReadinessProbe(protocol, host string, port int, path, hostname string, timeoutSeconds int) {
	err := WaitForReadiness(protocol, host, port, path, hostname, timeoutSeconds)
	if err != nil {
		log.Fatal().Err(err).Msgf("Readiness probe failed")
	}

	// readiness probe succeeded, exiting cleanly
	os.Exit(0)
}

func (b *service) RunEstafetteBuildJob(pipelineRunner PipelineRunner, dockerRunner DockerRunner, envvarHelper EnvvarHelper, obfuscator Obfuscator, endOfLifeHelper EndOfLifeHelper, builderConfig contracts.BuilderConfig, credentialsBytes []byte, runAsJob bool) {

	closer := b.initJaeger(b.applicationInfo.App)
	defer closer.Close()

	// unset all ESTAFETTE_ envvars so they don't get abused by non-estafette components
	envvarHelper.UnsetEstafetteEnvvars()

	envvarHelper.SetEstafetteBuilderConfigEnvvars(builderConfig)

	buildLog := contracts.BuildLog{
		RepoSource:   builderConfig.Git.RepoSource,
		RepoOwner:    builderConfig.Git.RepoOwner,
		RepoName:     builderConfig.Git.RepoName,
		RepoBranch:   builderConfig.Git.RepoBranch,
		RepoRevision: builderConfig.Git.RepoRevision,
		Steps:        make([]*contracts.BuildLogStep, 0),
	}

	if os.Getenv("ESTAFETTE_LOG_FORMAT") == "v3" {
		// set some default fields added to all logs
		log.Logger = log.Logger.With().
			Str("jobName", *builderConfig.JobName).
			Interface("git", builderConfig.Git).
			Logger()
	}

	rootSpanName := "RunBuildJob"
	if *builderConfig.Action == "release" {
		rootSpanName = "RunReleaseJob"
	}

	rootSpan := opentracing.StartSpan(rootSpanName)
	defer rootSpan.Finish()

	ctx := context.Background()
	ctx = opentracing.ContextWithSpan(ctx, rootSpan)

	// set running state, so a restarted job will show up as running once a new pod runs
	_ = endOfLifeHelper.SendBuildStartedEvent(ctx)

	// start docker daemon
	dockerDaemonStartSpan, _ := opentracing.StartSpanFromContext(ctx, "StartDockerDaemon")
	err := dockerRunner.StartDockerDaemon()
	if err != nil {
		endOfLifeHelper.HandleFatal(ctx, buildLog, err, "Error starting docker daemon")
	}

	// wait for docker daemon to be ready for usage
	dockerRunner.WaitForDockerDaemon()
	dockerDaemonStartSpan.Finish()

	// listen to cancellation in order to stop any running pipeline or container
	go pipelineRunner.StopPipelineOnCancellation()

	// get current working directory
	dir := envvarHelper.GetWorkDir()
	if dir == "" {
		endOfLifeHelper.HandleFatal(ctx, buildLog, nil, "Getting working directory from environment variable ESTAFETTE_WORKDIR failed")
	}

	// set some envvars
	err = envvarHelper.SetEstafetteGlobalEnvvars()
	if err != nil {
		endOfLifeHelper.HandleFatal(ctx, buildLog, err, "Setting global environment variables failed")
	}

	// initialize obfuscator
	err = obfuscator.CollectSecrets(*builderConfig.Manifest, credentialsBytes, envvarHelper.GetPipelineName())
	if err != nil {
		endOfLifeHelper.HandleFatal(ctx, buildLog, err, "Collecting secrets to obfuscate failed")
	}

	// check whether this is a regular build or a release
	stages := builderConfig.Manifest.Stages
	if *builderConfig.Action == "release" {
		// check if the release is defined
		releaseExists := false
		for _, r := range builderConfig.Manifest.Releases {
			if r.Name == builderConfig.ReleaseParams.ReleaseName {
				releaseExists = true
				stages = r.Stages
			}
		}
		if !releaseExists {
			endOfLifeHelper.HandleFatal(ctx, buildLog, nil, fmt.Sprintf("Release %v does not exist", builderConfig.ReleaseParams.ReleaseName))
		}
		log.Info().Msgf("Starting release %v at version %v...", builderConfig.ReleaseParams.ReleaseName, builderConfig.BuildVersion.Version)
	} else {
		log.Info().Msgf("Starting build version %v...", builderConfig.BuildVersion.Version)
	}

	// create docker client
	_, err = dockerRunner.CreateDockerClient()
	if err != nil {
		endOfLifeHelper.HandleFatal(ctx, buildLog, err, "Failed creating a docker client")
	}

	// collect estafette envvars and run stages from manifest
	log.Info().Msgf("Running %v stages", len(stages))
	estafetteEnvvars := envvarHelper.CollectEstafetteEnvvarsAndLabels(*builderConfig.Manifest)
	globalEnvvars := envvarHelper.CollectGlobalEnvvars(*builderConfig.Manifest)
	envvars := envvarHelper.OverrideEnvvars(estafetteEnvvars, globalEnvvars)

	// run stages
	pipelineRunner.EnableBuilderInfoStageInjection()
	buildLog.Steps, err = pipelineRunner.RunStages(ctx, 0, stages, dir, envvars)
	if err != nil && buildLog.HasUnknownStatus() {
		endOfLifeHelper.HandleFatal(ctx, buildLog, err, "Executing stages from manifest failed")
	}

	// send result to ci-api
	buildStatus := contracts.GetAggregatedStatus(buildLog.Steps)
	_ = endOfLifeHelper.SendBuildFinishedEvent(ctx, buildStatus)
	_ = endOfLifeHelper.SendBuildJobLogEvent(ctx, buildLog)
	_ = endOfLifeHelper.SendBuildCleanEvent(ctx, buildStatus)

	// finish and flush so it gets sent to the tracing backend
	rootSpan.Finish()
	closer.Close()

	if runAsJob {
		os.Exit(0)
	} else {
		HandleExit(buildLog.Steps)
	}
}

// initJaeger returns an instance of Jaeger Tracer that can be configured with environment variables
// https://github.com/jaegertracing/jaeger-client-go#environment-variables
func (b *service) initJaeger(service string) io.Closer {

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
