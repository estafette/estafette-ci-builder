package main

import (
	"fmt"
	"os"
	"runtime"

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
)

func main() {

	ciServer := getEstafetteEnv("ESTAFETTE_CI_SERVER")
	if ciServer == "gocd" {
		// pretty print for go.cd integration
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().
			Timestamp().
			Logger()
	} else {
		// log as severity for stackdriver logging to recognize the level
		zerolog.LevelFieldName = "severity"

		// set some default fields added to all logs
		log.Logger = zerolog.New(os.Stdout).With().
			Timestamp().
			Str("app", "estafette-ci-builder").
			Str("version", version).
			Logger()
	}

	stdlog.SetFlags(0)
	stdlog.SetOutput(log.Logger)

	// log startup message
	log.Info().
		Str("branch", branch).
		Str("revision", revision).
		Str("buildDate", buildDate).
		Str("goVersion", goVersion).
		Msg("Starting estafette-ci-builder...")

	if ciServer == "estafette" {

		err := startDockerDaemon()
		if err != nil {
			handleFatal(err, "Error starting docker daemon")
		}

		gitName := getEstafetteEnv("ESTAFETTE_GIT_NAME")
		gitURL := getEstafetteEnv("ESTAFETTE_GIT_URL")
		gitBranch := getEstafetteEnv("ESTAFETTE_GIT_BRANCH")
		gitRevision := getEstafetteEnv("ESTAFETTE_GIT_REVISION")

		// git clone to specific branch and revision
		err = gitCloneRevision(gitName, gitURL, gitBranch, gitRevision)
		if err != nil {
			handleFatal(err, fmt.Sprintf("Error cloning git repository %v to branch %v and revision %v...", gitName, gitBranch, gitRevision))
		}

		if !manifestExists(".estafette.yaml") {
			log.Info().Msg(".estafette.yaml file does not exist, exiting...")
			os.Exit(0)
		}

		// check if docker daemon is ready for usage
		waitForDockerDaemon()
	}

	// read yaml
	manifest, err := readManifest(".estafette.yaml")
	if err != nil {
		handleFatal(err, "Reading .estafette.yaml manifest failed")
	}

	// get current working directory
	dir, err := os.Getwd()
	if err != nil {
		handleFatal(err, "Getting current working directory failed")
	}

	log.Info().Msgf("Running %v pipelines", len(manifest.Pipelines))

	err = setEstafetteGlobalEnvvars()
	if err != nil {
		handleFatal(err, "Setting global environment variables failed")
	}

	envvars := collectEstafetteEnvvars(manifest)

	result := runPipelines(manifest, dir, envvars)

	if ciServer == "gocd" {
		renderStats(result)
	}

	if ciServer == "estafette" {
		// todo send result to ci-api
		log.Info().Msg("Finished running pipelines")
		os.Exit(0)
	}

	handleExit(result)
}
