package main

import (
	"os"
	"runtime"

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
)

func main() {

	// bootstrap
	envvarHelper := NewEnvvarHelper("ESTAFETTE_")
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
			endOfLifeHelper.handleFatal(err, "Reading .estafette.yaml manifest failed")
		}

		// get current working directory
		dir, err := os.Getwd()
		if err != nil {
			endOfLifeHelper.handleFatal(err, "Getting current working directory failed")
		}

		log.Info().Msgf("Running %v pipelines", len(manifest.Pipelines))

		err = envvarHelper.setEstafetteGlobalEnvvars()
		if err != nil {
			endOfLifeHelper.handleFatal(err, "Setting global environment variables failed")
		}

		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		result, err := pipelineRunner.runPipelines(manifest, dir, envvars)
		if err != nil {
			endOfLifeHelper.handleFatal(err, "Executing pipelines from manifest failed")
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
		err := dockerRunner.startDockerDaemon()
		if err != nil {
			endOfLifeHelper.handleFatal(err, "Error starting docker daemon")
		}

		// wait for docker daemon to be ready for usage
		dockerRunner.waitForDockerDaemon()

		// get current working directory
		dir, err := os.Getwd()
		if err != nil {
			endOfLifeHelper.handleFatal(err, "Getting current working directory failed")
		}

		// set some envvars
		err = envvarHelper.setEstafetteGlobalEnvvars()
		if err != nil {
			endOfLifeHelper.handleFatal(err, "Setting global environment variables failed")
		}

		// run git clone via pipeline runner
		estafetteGitCloneManifest := manifest.EstafetteManifest{
			Pipelines: []*manifest.EstafettePipeline{
				&manifest.EstafettePipeline{
					Name:             "git-clone",
					ContainerImage:   "extensions/git-clone:0.0.4",
					Shell:            "/bin/sh",
					WorkingDirectory: "/estafette-work",
					When:             "status == 'succeeded'",
				},
			},
		}

		// collect estafette envvars and run the git clone step
		envvars := envvarHelper.collectEstafetteEnvvars(estafetteGitCloneManifest)
		gitCloneResult, err := pipelineRunner.runPipelines(estafetteGitCloneManifest, dir, envvars)
		if err != nil {
			endOfLifeHelper.handleFatal(err, "Executing git clone step failed")
		}

		// check if manifest exists
		if !manifest.Exists(".estafette.yaml") {
			log.Info().Msg(".estafette.yaml file does not exist, exiting...")
			endOfLifeHelper.sendBuildFinishedEvent("builder:nomanifest")
			os.Exit(0)
		}

		// read .estafette.yaml manifest
		manifest, err := manifest.ReadManifestFromFile(".estafette.yaml")
		if err != nil {
			endOfLifeHelper.handleFatal(err, "Reading .estafette.yaml manifest failed")
		}

		// collect estafette envvars and run pipelines from manifest
		log.Info().Msgf("Running %v pipelines", len(manifest.Pipelines))
		envvars = envvarHelper.collectEstafetteEnvvars(manifest)
		result, err := pipelineRunner.runPipelines(manifest, dir, envvars)
		if err != nil {
			endOfLifeHelper.handleFatal(err, "Executing pipelines from manifest failed")
		}

		// merge git clone and manifest result
		result.PipelineResults = append(gitCloneResult.PipelineResults, result.PipelineResults...)

		// send result to ci-api
		log.Info().Interface("result", result).Msg("Finished running pipelines")
		endOfLifeHelper.sendBuildFinishedEvent("builder:succeeded")
		os.Exit(0)
	}
}
