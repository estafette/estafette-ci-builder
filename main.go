package main

import (
	"encoding/json"
	"fmt"
	stdlog "log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/alecthomas/kingpin"
	contracts "github.com/estafette/estafette-ci-contracts"
	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()

	builderConfigFlag         = kingpin.Flag("builder-config", "The Estafette server passes in this json structure to parameterize the build, set trusted images and inject credentials.").Envar("BUILDER_CONFIG").String()
	secretDecryptionKey       = kingpin.Flag("secret-decryption-key", "The AES-256 key used to decrypt secrets that have been encrypted with it.").Envar("SECRET_DECRYPTION_KEY").String()
	secretDecryptionKeyBase64 = kingpin.Flag("secret-decryption-key-base64", "The base64 encoded AES-256 key used to decrypt secrets that have been encrypted with it.").Envar("SECRET_DECRYPTION_KEY_BASE64").String()
	runAsJob                  = kingpin.Flag("run-as-job", "To run the builder as a job and prevent build failures to fail the job.").Default("false").OverrideDefaultFromEnvar("RUN_AS_JOB").Bool()
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

	// read builder config from envvar and unset envar; will replace parameterizing the job via separate envvars
	var builderConfig contracts.BuilderConfig
	builderConfigJSON := *builderConfigFlag
	if builderConfigJSON == "" {
		log.Fatal().Msg("BUILDER_CONFIG envvar is not set")
	}
	os.Unsetenv("BUILDER_CONFIG")

	// unmarshal builder config
	err := json.Unmarshal([]byte(builderConfigJSON), &builderConfig)
	if err != nil {
		log.Fatal().Err(err).Interface("builderConfigJSON", builderConfigJSON).Msg("Failed to unmarshal BUILDER_CONFIG")
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

	// bootstrap
	obfuscator := NewObfuscator(secretHelper)
	envvarHelper := NewEnvvarHelper("ESTAFETTE_", secretHelper, obfuscator)
	whenEvaluator := NewWhenEvaluator(envvarHelper)
	dockerRunner := NewDockerRunner(envvarHelper, obfuscator, *runAsJob, builderConfig, cancellationChannel)
	pipelineRunner := NewPipelineRunner(envvarHelper, whenEvaluator, dockerRunner, *runAsJob, cancellationChannel)
	endOfLifeHelper := NewEndOfLifeHelper(*runAsJob, builderConfig)

	// detect controlling server
	ciServer := envvarHelper.getCiServer()

	if ciServer == "estafette" {
		// unset all ESTAFETTE_ envvars so they don't get abused by non-estafette components
		envvarHelper.unsetEstafetteEnvvars()
	}

	if ciServer == "gocd" {

		fatalHandler := NewGocdFatalHandler()

		// pretty print for go.cd integration
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().
			Timestamp().
			Logger()

		stdlog.SetFlags(0)
		stdlog.SetOutput(log.Logger)

		// log startup message
		log.Info().
			Str("branch", branch).
			Str("revision", revision).
			Str("buildDate", buildDate).
			Str("goVersion", goVersion).
			Msgf("Starting estafette-ci-builder version %v...", version)

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
		result, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Executing stages from manifest failed")
		}

		renderStats(result)

		handleExit(result)

	} else if ciServer == "estafette" {

		// log as severity for stackdriver logging to recognize the level
		zerolog.LevelFieldName = "severity"

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
		log.Logger = zerolog.New(os.Stdout).With().
			Timestamp().
			Str("app", "estafette-ci-builder").
			Str("version", version).
			Str("jobName", *builderConfig.JobName).
			Interface("git", builderConfig.Git).
			Logger()

		stdlog.SetFlags(0)
		stdlog.SetOutput(log.Logger)

		// log startup message
		log.Info().
			Str("branch", branch).
			Str("revision", revision).
			Str("buildDate", buildDate).
			Str("goVersion", goVersion).
			Msgf("Starting estafette-ci-builder version %v...", version)

		// start docker daemon
		err = dockerRunner.startDockerDaemon()
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Error starting docker daemon")
		}

		// wait for docker daemon to be ready for usage
		dockerRunner.waitForDockerDaemon()

		// listen to cancellation in order to stop any running pipeline or container
		go pipelineRunner.stopPipelineOnCancellation()
		go dockerRunner.stopContainerOnCancellation()

		// get current working directory
		dir, err := os.Getwd()
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Getting current working directory failed")
		}

		// set some envvars
		err = envvarHelper.setEstafetteGlobalEnvvars()
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Setting global environment variables failed")
		}

		// initialize obfuscator
		err = obfuscator.CollectSecrets(*builderConfig.Manifest)
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Collecting secrets to obfuscate failed")
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
				endOfLifeHelper.handleFatal(buildLog, nil, fmt.Sprintf("Release %v does not exist", builderConfig.ReleaseParams.ReleaseName))
			}
			log.Info().Msgf("Starting release %v at version %v...", builderConfig.ReleaseParams.ReleaseName, builderConfig.BuildVersion.Version)
		} else {
			log.Info().Msgf("Starting build version %v...", builderConfig.BuildVersion.Version)
		}

		err = envvarHelper.setEstafetteStagesEnvvar(stages)
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Setting ESTAFETTE_STAGES environment variable failed")
		}

		// create docker client
		_, err = dockerRunner.createDockerClient()
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Failed creating a docker client")
		}

		// collect estafette envvars and run stages from manifest
		log.Info().Msgf("Running %v stages", len(stages))
		estafetteEnvvars := envvarHelper.collectEstafetteEnvvarsAndLabels(*builderConfig.Manifest)
		globalEnvvars := envvarHelper.collectGlobalEnvvars(*builderConfig.Manifest)
		envvars := envvarHelper.overrideEnvvars(estafetteEnvvars, globalEnvvars)

		// run stages
		result, err := pipelineRunner.runStages(stages, dir, envvars)
		if err != nil && !result.canceled {
			endOfLifeHelper.handleFatal(buildLog, err, "Executing stages from manifest failed")
		}

		// send result to ci-api
		log.Info().Interface("result", result).Msg("Finished running stages")
		buildLog.Steps = transformPipelineRunResultToBuildLogSteps(estafetteEnvvars, result)
		buildStatus := "succeeded"
		if result.HasAggregatedErrors() {
			buildStatus = "failed"
		}
		if result.canceled {
			buildStatus = "canceled"
		}

		_ = endOfLifeHelper.sendBuildFinishedEvent(buildStatus)
		_ = endOfLifeHelper.sendBuildJobLogEvent(buildLog)
		_ = endOfLifeHelper.sendBuildCleanEvent(buildStatus)

		if *runAsJob {
			os.Exit(0)
		} else {
			handleExit(result)
		}

	} else {
		// Set up a simple console logger
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().
			Timestamp().
			Logger()

		log.Warn().Msgf("The CI Server (\"%s\") is not recognized, exiting.", ciServer)
	}
}
