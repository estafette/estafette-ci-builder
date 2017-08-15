package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
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

	// log startup message
	log.Info().
		Str("branch", branch).
		Str("revision", revision).
		Str("buildDate", buildDate).
		Str("goVersion", goVersion).
		Msg("Starting estafette-ci-builder...")

	if ciServer == "estafette" {

		gitURL := getEstafetteEnv("ESTAFETTE_GIT_URL")
		gitBranch := getEstafetteEnv("ESTAFETTE_GIT_BRANCH")
		gitRevision := getEstafetteEnv("ESTAFETTE_GIT_REVISION")

		// git clone to specific branch and revision
		err := gitCloneRevision(gitURL, gitBranch, gitRevision)

		if err != nil {
			log.Error().Err(err).
				Str("url", gitURL).
				Str("branch", gitBranch).
				Str("revision", gitRevision).
				Msgf("Error cloning git repository %v to branch %v and revision %v...", gitURL, gitBranch, gitRevision)
		}

		os.Exit(0)
	}

	// read yaml
	manifest, err := readManifest(".estafette.yaml")
	if err != nil {
		log.Fatal().Err(err)
	}

	// get current working directory
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err)
	}

	fmt.Printf("[estafette] Running %v pipelines\n", len(manifest.Pipelines))

	err = setEstafetteGlobalEnvvars()
	if err != nil {
		log.Fatal().Err(err)
	}

	envvars := collectEstafetteEnvvars(manifest)

	result := runPipelines(manifest, dir, envvars)

	renderStats(result)

	handleExit(result)
}
