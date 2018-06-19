package main

import (
	"encoding/json"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/estafette/estafette-ci-contracts"
	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	stdlog "log"
)

var (
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()

	secretDecryptionKey = kingpin.Flag("secret-decryption-key", "The AES-256 key used to decrypt secrets that have been encrypted with it.").String()
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	// bootstrap
	secretHelper := crypt.NewSecretHelper(*secretDecryptionKey)
	envvarHelper := NewEnvvarHelper("ESTAFETTE_", secretHelper)
	whenEvaluator := NewWhenEvaluator(envvarHelper)
	dockerRunner := NewDockerRunner(envvarHelper)
	pipelineRunner := NewPipelineRunner(envvarHelper, whenEvaluator, dockerRunner)
	endOfLifeHelper := NewEndOfLifeHelper(envvarHelper)

	// detect controlling server
	ciServer := envvarHelper.getEstafetteEnv("ESTAFETTE_CI_SERVER")

	if ciServer == "gocd" {

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
			Msg("Starting estafette-ci-builder...")

		// read yaml
		manifest, err := manifest.ReadManifestFromFile(".estafette.yaml")
		if err != nil {
			endOfLifeHelper.handleGocdFatal(err, "Reading .estafette.yaml manifest failed")
		}

		// get current working directory
		dir, err := os.Getwd()
		if err != nil {
			endOfLifeHelper.handleGocdFatal(err, "Getting current working directory failed")
		}

		log.Info().Msgf("Running %v pipelines", len(manifest.Pipelines))

		err = envvarHelper.setEstafetteGlobalEnvvars()
		if err != nil {
			endOfLifeHelper.handleGocdFatal(err, "Setting global environment variables failed")
		}

		// collect estafette and 'global' envvars from manifest
		estafetteEnvvars := envvarHelper.collectEstafetteEnvvars(manifest)
		globalEnvvars := envvarHelper.collectGlobalEnvvars(manifest)

		// merge estafette and global envvars
		envvars := envvarHelper.overrideEnvvars(estafetteEnvvars, globalEnvvars)

		result, err := pipelineRunner.runPipelines(manifest, dir, envvars)
		if err != nil {
			endOfLifeHelper.handleGocdFatal(err, "Executing pipelines from manifest failed")
		}

		renderStats(result)

		handleExit(result)

	} else if ciServer == "estafette" {

		// log as severity for stackdriver logging to recognize the level
		zerolog.LevelFieldName = "severity"

		gitName := envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_NAME")
		gitBranch := envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_BRANCH")
		gitRevision := envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_REVISION")
		jobName := envvarHelper.getEstafetteEnv("ESTAFETTE_BUILD_JOB_NAME")
		builderTrack := envvarHelper.getEstafetteEnv("ESTAFETTE_CI_BUILDER_TRACK")
		if builderTrack == "" {
			builderTrack = "stable"
		}

		buildLog := contracts.BuildLog{
			RepoSource:   envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_SOURCE"),
			RepoOwner:    strings.Split(gitName, "/")[0],
			RepoName:     strings.Split(gitName, "/")[1],
			RepoBranch:   gitBranch,
			RepoRevision: gitRevision,
			Steps:        make([]contracts.BuildLogStep, 0),
		}

		// log to file and stdout
		logFile, err := os.OpenFile("/log.txt", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create log file log.txt")
		}
		defer logFile.Close()
		multiLogWriter := io.MultiWriter(os.Stdout, logFile)

		// set some default fields added to all logs
		log.Logger = zerolog.New(multiLogWriter).With().
			Timestamp().
			Str("app", "estafette-ci-builder").
			Str("version", version).
			Str("jobName", jobName).
			Str("gitName", gitName).
			Str("gitBranch", gitBranch).
			Str("gitRevision", gitRevision).
			Logger()

		stdlog.SetFlags(0)
		stdlog.SetOutput(log.Logger)

		// log startup message
		log.Info().
			Str("branch", branch).
			Str("revision", revision).
			Str("buildDate", buildDate).
			Str("goVersion", goVersion).
			Msg("Starting estafette-ci-builder...")

		// start docker daemon
		err = dockerRunner.startDockerDaemon()
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Error starting docker daemon")
		}

		// wait for docker daemon to be ready for usage
		dockerRunner.waitForDockerDaemon()

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

		// get manifest from envvar and unmarshal
		manifestJSON := os.Getenv("ESTAFETTE_CI_MANIFEST_JSON")
		var manifest manifest.EstafetteManifest
		json.Unmarshal([]byte(manifestJSON), &manifest)

		log.Info().Msgf("Starting build version %v...", envvarHelper.getEstafetteEnv("ESTAFETTE_BUILD_VERSION"))

		// collect estafette envvars and run pipelines from manifest
		log.Info().Msgf("Running %v pipelines", len(manifest.Pipelines))
		estafetteEnvvars := envvarHelper.collectEstafetteEnvvars(manifest)
		globalEnvvars := envvarHelper.collectGlobalEnvvars(manifest)
		envvars := envvarHelper.overrideEnvvars(estafetteEnvvars, globalEnvvars)
		result, err := pipelineRunner.runPipelines(manifest, dir, envvars)
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Executing pipelines from manifest failed")
		}

		// send result to ci-api
		log.Info().Interface("result", result).Msg("Finished running pipelines")
		buildLog.Steps = transformPipelineRunResultToBuildLogSteps(result)
		endOfLifeHelper.sendBuildJobLogEvent(buildLog)
		buildStatus := "succeeded"
		if result.HasErrors() {
			buildStatus = "failed"
		}
		endOfLifeHelper.sendBuildFinishedEvent(buildStatus)
		os.Exit(0)
	} else {
		// Set up a simple console logger
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().
			Timestamp().
			Logger()

		log.Warn().Msgf("The CI Server (\"%s\") is not recognized, exiting.", ciServer)
	}
}
