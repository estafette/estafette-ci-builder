package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"

	"github.com/alecthomas/kingpin"
	contracts "github.com/estafette/estafette-ci-contracts"
	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
)

var (
	appgroup  string
	app       string
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()

	builderConfigFlag         = kingpin.Flag("builder-config", "The Estafette server passes in this json structure to parameterize the build, set trusted images and inject credentials.").Envar("BUILDER_CONFIG").String()
	builderConfigPath         = kingpin.Flag("builder-config-path", "The path to the builder config json stored in a mounted file, to parameterize the build, set trusted images and inject credentials.").Envar("BUILDER_CONFIG_PATH").String()
	secretDecryptionKey       = kingpin.Flag("secret-decryption-key", "The AES-256 key used to decrypt secrets that have been encrypted with it.").Envar("SECRET_DECRYPTION_KEY").String()
	secretDecryptionKeyBase64 = kingpin.Flag("secret-decryption-key-base64", "The base64 encoded AES-256 key used to decrypt secrets that have been encrypted with it.").Envar("SECRET_DECRYPTION_KEY_BASE64").String()
	runAsJob                  = kingpin.Flag("run-as-job", "To run the builder as a job and prevent build failures to fail the job.").Default("false").OverrideDefaultFromEnvar("RUN_AS_JOB").Bool()
	podName                   = kingpin.Flag("pod-name", "The name of the pod.").Envar("POD_NAME").String()
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	// define channel to catch SIGTERM and send out cancellation to stop further execution of stages and send the final state and logs to the ci server
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
	cancellationChannel := make(chan struct{})
	go func(osSignals chan os.Signal, cancellationChannel chan struct{}) {
		// wait for sigterm
		<-osSignals
		// broadcast a cancellation
		close(cancellationChannel)
	}(osSignals, cancellationChannel)

	// support both base64 encoded decryption key and non-encoded
	secretDecryptionKeyBase64Encoded := *secretDecryptionKeyBase64 != ""
	decryptionKey := *secretDecryptionKey
	if secretDecryptionKeyBase64Encoded {
		decryptionKey = *secretDecryptionKeyBase64
	}

	secretHelper := crypt.NewSecretHelper(decryptionKey, secretDecryptionKeyBase64Encoded)

	// bootstrap
	obfuscator := NewObfuscator(secretHelper)
	envvarHelper := NewEnvvarHelper("ESTAFETTE_", secretHelper, obfuscator)
	whenEvaluator := NewWhenEvaluator(envvarHelper)

	// detect controlling server
	ciServer := envvarHelper.getCiServer()

	// configure json logging
	foundation.InitLogging(appgroup, app, version, branch, revision, buildDate, ciServer != "estafette")

	// read builder config either from file or envvar
	var builderConfigJSON []byte

	if *builderConfigPath != "" {

		log.Info().Msgf("Reading builder config from file %v...", *builderConfigPath)

		var err error
		builderConfigJSON, err = ioutil.ReadFile(*builderConfigPath)
		if err != nil {
			log.Fatal().Err(err).Interface("builderConfigJSON", builderConfigJSON).Msgf("Failed to read builder config file at %v", *builderConfigPath)
		}

	} else if *builderConfigFlag != "" {

		log.Info().Msg("Reading builder config from envvar BUILDER_CONFIG...")

		builderConfigJSON = []byte(*builderConfigFlag)
		os.Unsetenv("BUILDER_CONFIG")

	} else {

		log.Fatal().Msg("Neither BUILDER_CONFIG_PATH nor BUILDER_CONFIG envvar is not set; one of them is required")

	}

	// unmarshal builder config
	var builderConfig contracts.BuilderConfig
	err := json.Unmarshal(builderConfigJSON, &builderConfig)
	if err != nil {
		log.Fatal().Err(err).Interface("builderConfigJSON", builderConfigJSON).Msg("Failed to unmarshal builder config")
	}

	// decrypt all credentials
	decryptedCredentials := []*contracts.CredentialConfig{}
	for _, c := range builderConfig.Credentials {

		// loop all additional properties and decrypt
		decryptedAdditionalProperties := map[string]interface{}{}
		for key, value := range c.AdditionalProperties {
			if s, isString := value.(string); isString {
				decryptedAdditionalProperties[key], err = secretHelper.DecryptAllEnvelopes(s)
				if err != nil {
					log.Fatal().Err(err).Msgf("Failed decrypting credential %v property %v", c.Name, key)
				}
			} else {
				decryptedAdditionalProperties[key] = value
			}
		}
		c.AdditionalProperties = decryptedAdditionalProperties

		decryptedCredentials = append(decryptedCredentials, c)
	}
	builderConfig.Credentials = decryptedCredentials

	// bootstrap continued
	dockerRunner := NewDockerRunner(envvarHelper, obfuscator, *runAsJob, builderConfig, cancellationChannel)
	pipelineRunner := NewPipelineRunner(envvarHelper, whenEvaluator, dockerRunner, *runAsJob, cancellationChannel)
	endOfLifeHelper := NewEndOfLifeHelper(*runAsJob, builderConfig, *podName)

	if ciServer == "gocd" {

		fatalHandler := NewGocdFatalHandler()

		// create docker client
		_, err := dockerRunner.createDockerClient()
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Failed creating a docker client")
		}

		// read yaml
		manifest, err := manifest.ReadManifestFromFile(".estafette.yaml")
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Reading .estafette.yaml manifest failed")
		}

		// initialize obfuscator
		err = obfuscator.CollectSecrets(manifest)
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Collecting secrets to obfuscate failed")
		}

		// get current working directory
		dir, err := os.Getwd()
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Getting current working directory failed")
		}

		log.Info().Msgf("Running %v stages", len(manifest.Stages))

		err = envvarHelper.setEstafetteGlobalEnvvars()
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Setting global environment variables failed")
		}
		err = envvarHelper.setEstafetteStagesEnvvar(manifest.Stages)
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Setting ESTAFETTE_STAGES environment variable failed")
		}

		// collect estafette and 'global' envvars from manifest
		estafetteEnvvars := envvarHelper.collectEstafetteEnvvarsAndLabels(manifest)
		globalEnvvars := envvarHelper.collectGlobalEnvvars(manifest)

		// merge estafette and global envvars
		envvars := envvarHelper.overrideEnvvars(estafetteEnvvars, globalEnvvars)

		// run stages
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Executing stages from manifest failed")
		}

		renderStats(result)

		handleExit(result)

	} else if ciServer == "estafette" {

		closer := initJaeger(app)
		defer closer.Close()

		// unset all ESTAFETTE_ envvars so they don't get abused by non-estafette components
		envvarHelper.unsetEstafetteEnvvars()

		envvarHelper.setEstafetteBuilderConfigEnvvars(builderConfig)

		buildLog := contracts.BuildLog{
			RepoSource:   builderConfig.Git.RepoSource,
			RepoOwner:    builderConfig.Git.RepoOwner,
			RepoName:     builderConfig.Git.RepoName,
			RepoBranch:   builderConfig.Git.RepoBranch,
			RepoRevision: builderConfig.Git.RepoRevision,
			Steps:        make([]contracts.BuildLogStep, 0),
		}

		// set some default fields added to all logs
		log.Logger = log.Logger.With().
			Str("jobName", *builderConfig.JobName).
			Interface("git", builderConfig.Git).
			Logger()

		if runtime.GOOS == "windows" {
			if builderConfig.DockerDaemonMTU != nil && *builderConfig.DockerDaemonMTU != "" {
				mtu, err := strconv.Atoi(*builderConfig.DockerDaemonMTU)
				if err != nil {
					log.Warn().Err(err).Msgf("Failed to parse mtu %v from config", *builderConfig.DockerDaemonMTU)
				} else {
					mtu -= 50
					// update any ethernet interfaces to this mtu
					interfaces, err := net.Interfaces()
					if err != nil {
						log.Warn().Err(err).Msg("Failed to retrieve network interfaces")
					} else {
						log.Info().Interface("interfaces", interfaces).Msg("Retrieved network interfaces")
						for _, i := range interfaces {
							if i.MTU > mtu && i.Flags&net.FlagLoopback == 0 {
								log.Info().Interface("interface", i).Msgf("Updating MTU for interface '%v' with flags %v from %v to %v", i.Name, i.Flags.String(), i.MTU, mtu)
								i.MTU = mtu
							} else {
								log.Info().Interface("interface", i).Msgf("MTU for interface '%v' with flags %v is set to %v, no need to update...", i.Name, i.Flags.String(), i.MTU)
							}
						}
					}
				}
			}
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
		_ = endOfLifeHelper.sendBuildStartedEvent(ctx)

		// start docker daemon
		if runtime.GOOS != "windows" {
			dockerDaemonStartSpan, _ := opentracing.StartSpanFromContext(ctx, "StartDockerDaemon")
			err = dockerRunner.startDockerDaemon()
			if err != nil {
				endOfLifeHelper.handleFatal(ctx, buildLog, err, "Error starting docker daemon")
			}

			// wait for docker daemon to be ready for usage
			dockerRunner.waitForDockerDaemon()
			dockerDaemonStartSpan.Finish()
		}

		// listen to cancellation in order to stop any running pipeline or container
		go pipelineRunner.stopPipelineOnCancellation()
		go dockerRunner.stopContainerOnCancellation()

		// get current working directory
		dir := envvarHelper.getWorkDir()
		if dir == "" {
			endOfLifeHelper.handleFatal(ctx, buildLog, nil, "Getting working directory from environment variable ESTAFETTE_WORKDIR failed")
		}

		// set some envvars
		err = envvarHelper.setEstafetteGlobalEnvvars()
		if err != nil {
			endOfLifeHelper.handleFatal(ctx, buildLog, err, "Setting global environment variables failed")
		}

		// initialize obfuscator
		err = obfuscator.CollectSecrets(*builderConfig.Manifest)
		if err != nil {
			endOfLifeHelper.handleFatal(ctx, buildLog, err, "Collecting secrets to obfuscate failed")
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
				endOfLifeHelper.handleFatal(ctx, buildLog, nil, fmt.Sprintf("Release %v does not exist", builderConfig.ReleaseParams.ReleaseName))
			}
			log.Info().Msgf("Starting release %v at version %v...", builderConfig.ReleaseParams.ReleaseName, builderConfig.BuildVersion.Version)
		} else {
			log.Info().Msgf("Starting build version %v...", builderConfig.BuildVersion.Version)
		}

		err = envvarHelper.setEstafetteStagesEnvvar(stages)
		if err != nil {
			endOfLifeHelper.handleFatal(ctx, buildLog, err, "Setting ESTAFETTE_STAGES environment variable failed")
		}

		// create docker client
		_, err = dockerRunner.createDockerClient()
		if err != nil {
			endOfLifeHelper.handleFatal(ctx, buildLog, err, "Failed creating a docker client")
		}

		// collect estafette envvars and run stages from manifest
		log.Info().Msgf("Running %v stages", len(stages))
		estafetteEnvvars := envvarHelper.collectEstafetteEnvvarsAndLabels(*builderConfig.Manifest)
		globalEnvvars := envvarHelper.collectGlobalEnvvars(*builderConfig.Manifest)
		envvars := envvarHelper.overrideEnvvars(estafetteEnvvars, globalEnvvars)

		// run stages
		result, err := pipelineRunner.runStages(ctx, 0, stages, dir, envvars)
		if err != nil && !result.canceled {
			endOfLifeHelper.handleFatal(ctx, buildLog, err, "Executing stages from manifest failed")
		}

		// send result to ci-api
		log.Info().Interface("result", result).Msg("Finished running stages")
		buildLog.Steps = transformRunStagesResultToBuildLogSteps(result)
		buildStatus := "succeeded"
		if result.HasAggregatedErrors() {
			buildStatus = "failed"
		}
		if result.canceled {
			buildStatus = "canceled"
		}

		_ = endOfLifeHelper.sendBuildFinishedEvent(ctx, buildStatus)
		_ = endOfLifeHelper.sendBuildJobLogEvent(ctx, buildLog)
		_ = endOfLifeHelper.sendBuildCleanEvent(ctx, buildStatus)

		// finish and flush so it gets sent to the tracing backend
		rootSpan.Finish()
		closer.Close()

		if *runAsJob {
			os.Exit(0)
		} else {
			handleExit(result)
		}

	} else {
		log.Warn().Msgf("The CI Server (\"%s\") is not recognized, exiting.", ciServer)
	}
}

// initJaeger returns an instance of Jaeger Tracer that can be configured with environment variables
// https://github.com/jaegertracing/jaeger-client-go#environment-variables
func initJaeger(service string) io.Closer {

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
