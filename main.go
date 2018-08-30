package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/docker/docker/api/types"
	"github.com/estafette/estafette-ci-builder/config"
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

		// create docker client
		_, err := dockerRunner.createDockerClient()
		if err != nil {
			endOfLifeHelper.handleGocdFatal(err, "Failed creating a docker client")
		}

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

		log.Info().Msgf("Running %v stages", len(manifest.Stages))

		err = envvarHelper.setEstafetteGlobalEnvvars()
		if err != nil {
			endOfLifeHelper.handleGocdFatal(err, "Setting global environment variables failed")
		}

		// collect estafette and 'global' envvars from manifest
		estafetteEnvvars := envvarHelper.collectEstafetteEnvvars(manifest)
		globalEnvvars := envvarHelper.collectGlobalEnvvars(manifest)

		// merge estafette and global envvars
		envvars := envvarHelper.overrideEnvvars(estafetteEnvvars, globalEnvvars)

		result, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)
		if err != nil {
			endOfLifeHelper.handleGocdFatal(err, "Executing stages from manifest failed")
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
		buildVersion := envvarHelper.getEstafetteEnv("ESTAFETTE_BUILD_VERSION")
		releaseName := envvarHelper.getEstafetteEnv("ESTAFETTE_RELEASE_NAME")
		releaseIDValue := envvarHelper.getEstafetteEnv("ESTAFETTE_RELEASE_ID")
		releaseID, _ := strconv.Atoi(releaseIDValue)

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

		// check whether this is a regular build or a release
		stages := manifest.Stages
		if releaseID > 0 {
			// check if the release is defined
			releaseExists := false
			for _, r := range manifest.Releases {
				if r.Name == releaseName {
					releaseExists = true
					stages = r.Stages
				}
			}
			if !releaseExists {
				endOfLifeHelper.handleFatal(buildLog, err, fmt.Sprintf("Release %v does not exist", releaseName))
			}
			log.Info().Msgf("Starting release %v at version %v...", releaseName, buildVersion)
		} else {
			log.Info().Msgf("Starting build version %v...", buildVersion)
		}

		// create docker client
		dockerClient, err := dockerRunner.createDockerClient()
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Failed creating a docker client")
		}

		// get private container registries credentials
		registriesJSON := os.Getenv("ESTAFETTE_CI_REGISTRIES_JSON")
		if registriesJSON != "" {
			var registries []*config.PrivateContainerRegistryConfig
			json.Unmarshal([]byte(registriesJSON), &registries)

			// check if any of the used stages use an image from one of the defined registries and log in to the registry in that case
			for _, registry := range registries {
				for _, stage := range stages {
					if strings.HasPrefix(stage.ContainerImage, registry.Server) {
						// log in to this registry
						log.Info().Msgf("Authenticating registry %v", registry.Server)

						_, err = dockerClient.RegistryLogin(context.Background(), types.AuthConfig{
							ServerAddress: fmt.Sprintf("https://%v", registry.Server),
							Username:      registry.Username,
							Password:      registry.Password,
						})
						if err != nil {
							endOfLifeHelper.handleFatal(buildLog, err, fmt.Sprintf("Failed authenticating registry %v", registry.Server))
						}

						break
					}
				}
			}
		}

		// collect estafette envvars and run stages from manifest
		log.Info().Msgf("Running %v stages", len(stages))
		estafetteEnvvars := envvarHelper.collectEstafetteEnvvars(manifest)
		globalEnvvars := envvarHelper.collectGlobalEnvvars(manifest)
		envvars := envvarHelper.overrideEnvvars(estafetteEnvvars, globalEnvvars)
		result, err := pipelineRunner.runStages(stages, dir, envvars)
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Executing stages from manifest failed")
		}

		// send result to ci-api
		log.Info().Interface("result", result).Msg("Finished running stages")
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
