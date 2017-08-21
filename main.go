package main

import (
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

		result, err := runPipelines(manifest, dir, envvars)
		if err != nil {
			handleFatal(err, "Executing pipelines from manifest failed")
		}

		renderStats(result)

		handleExit(result)

	} else if ciServer == "estafette" {

		// log as severity for stackdriver logging to recognize the level
		zerolog.LevelFieldName = "severity"

		gitName := getEstafetteEnv("ESTAFETTE_GIT_NAME")
		gitBranch := getEstafetteEnv("ESTAFETTE_GIT_BRANCH")
		gitRevision := getEstafetteEnv("ESTAFETTE_GIT_REVISION")
		jobName := getEstafetteEnv("ESTAFETTE_BUILD_JOB_NAME")

		// set some default fields added to all logs
		log.Logger = zerolog.New(os.Stdout).With().
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
		err := startDockerDaemon()
		if err != nil {
			handleFatal(err, "Error starting docker daemon")
		}

		// wait for docker daemon to be ready for usage
		waitForDockerDaemon()

		// get current working directory
		dir, err := os.Getwd()
		if err != nil {
			handleFatal(err, "Getting current working directory failed")
		}

		// set some envvars
		err = setEstafetteGlobalEnvvars()
		if err != nil {
			handleFatal(err, "Setting global environment variables failed")
		}

		// run git clone via pipeline runner
		estafetteGitCloneManifest := estafetteManifest{
			Pipelines: []*estafettePipeline{
				&estafettePipeline{
					Name:             "git-clone",
					ContainerImage:   "extensions/git-clone:0.0.1",
					Shell:            "/bin/sh",
					WorkingDirectory: "/estafette-work",
					When:             "status == 'succeeded'",
				},
			},
		}

		// collect estafette envvars and run the git clone step
		envvars := collectEstafetteEnvvars(estafetteGitCloneManifest)
		gitCloneResult, err := runPipelines(estafetteGitCloneManifest, dir, envvars)
		if err != nil {
			handleFatal(err, "Executing git clone step failed")
		}

		// check if manifest exists
		if !manifestExists(".estafette.yaml") {
			log.Info().Msg(".estafette.yaml file does not exist, exiting...")
			sendBuildFinishedEvent("builder:nomanifest")
			os.Exit(0)
		}

		// read .estafette.yaml manifest
		manifest, err := readManifest(".estafette.yaml")
		if err != nil {
			handleFatal(err, "Reading .estafette.yaml manifest failed")
		}

		// collect estafette envvars and run pipelines from manifest
		log.Info().Msgf("Running %v pipelines", len(manifest.Pipelines))
		envvars = collectEstafetteEnvvars(manifest)
		result, err := runPipelines(manifest, dir, envvars)
		if err != nil {
			handleFatal(err, "Executing pipelines from manifest failed")
		}

		// merge git clone and manifest result
		result.PipelineResults = append(gitCloneResult.PipelineResults, result.PipelineResults...)

		// send result to ci-api
		log.Info().Interface("result", result).Msg("Finished running pipelines")
		sendBuildFinishedEvent("builder:succeeded")
		os.Exit(0)
	}
}
