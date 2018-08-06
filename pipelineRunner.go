package main

import (
	"fmt"
	"time"

	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/rs/zerolog/log"
)

// PipelineRunner is the interface for running the pipeline steps
type PipelineRunner interface {
	runStage(string, map[string]string, manifest.EstafetteStage) (estafetteStageRunResult, error)
	runStages(manifest.EstafetteManifest, string, map[string]string) (estafetteRunStagesResult, error)
}

type pipelineRunnerImpl struct {
	envvarHelper  EnvvarHelper
	whenEvaluator WhenEvaluator
	dockerRunner  DockerRunner
}

// NewPipelineRunner returns a new PipelineRunner
func NewPipelineRunner(envvarHelper EnvvarHelper, whenEvaluator WhenEvaluator, dockerRunner DockerRunner) PipelineRunner {
	return &pipelineRunnerImpl{
		envvarHelper:  envvarHelper,
		whenEvaluator: whenEvaluator,
		dockerRunner:  dockerRunner,
	}
}

type estafetteStageRunResult struct {
	Stage               manifest.EstafetteStage
	IsDockerImagePulled bool
	DockerImageSize     int64
	DockerPullDuration  time.Duration
	DockerPullError     error
	DockerRunDuration   time.Duration
	DockerRunError      error
	OtherError          error
	ExitCode            int64
	Status              string
	LogLines            []buildJobLogLine
}

// Errors combines the different type of errors that occurred during this pipeline stage
func (result *estafetteStageRunResult) Errors() (errors []error) {

	if result.DockerPullError != nil {
		errors = append(errors, result.DockerPullError)
	}

	if result.DockerRunError != nil {
		errors = append(errors, result.DockerRunError)
	}

	if result.OtherError != nil {
		errors = append(errors, result.OtherError)
	}

	return errors
}

// HasErrors indicates whether any errors happened in this pipeline stage
func (result *estafetteStageRunResult) HasErrors() bool {

	errors := result.Errors()

	return len(errors) > 0
}

func (pr *pipelineRunnerImpl) runStage(dir string, envvars map[string]string, p manifest.EstafetteStage) (result estafetteStageRunResult, err error) {

	result.Stage = p
	result.LogLines = make([]buildJobLogLine, 0)

	log.Info().Msgf("[%v] Starting pipeline '%v'", p.Name, p.Name)

	result.IsDockerImagePulled = pr.dockerRunner.isDockerImagePulled(p)

	if !result.IsDockerImagePulled {

		// pull docker image
		dockerPullStart := time.Now()
		result.DockerPullError = pr.dockerRunner.runDockerPull(p)
		result.DockerPullDuration = time.Since(dockerPullStart)
		if result.DockerPullError != nil {
			return result, result.DockerPullError
		}

		// set docker image size
		size, err := pr.dockerRunner.getDockerImageSize(p)
		if err != nil {
			return result, err
		}
		result.DockerImageSize = size

	}

	// run commands in docker container
	dockerRunStart := time.Now()
	result.LogLines, result.ExitCode, result.DockerRunError = pr.dockerRunner.runDockerRun(dir, envvars, p)
	result.DockerRunDuration = time.Since(dockerRunStart)
	if result.DockerRunError != nil {
		return result, result.DockerRunError
	}

	log.Info().Msgf("[%v] Finished pipeline '%v' successfully", p.Name, p.Name)

	return
}

type estafetteRunStagesResult struct {
	StageResults []estafetteStageRunResult
}

// Errors combines the different type of errors that occurred during this pipeline stage
func (result *estafetteRunStagesResult) Errors() (errors []error) {

	for _, pr := range result.StageResults {
		if pr.HasErrors() {
			errors = append(errors, pr.Errors()...)
		}
	}

	return errors
}

// HasErrors indicates whether any errors happened in this pipeline stages
func (result *estafetteRunStagesResult) HasErrors() bool {

	errors := result.Errors()

	return len(errors) > 0
}

func (pr *pipelineRunnerImpl) runStages(manifest manifest.EstafetteManifest, dir string, envvars map[string]string) (result estafetteRunStagesResult, err error) {

	// set default build status if not set
	err = pr.envvarHelper.initBuildStatus()
	if err != nil {
		return
	}

	if len(manifest.Stages) == 0 {
		return result, fmt.Errorf("Manifest has no stages, failing the build")
	}

	for _, p := range manifest.Stages {

		whenEvaluationResult, err := pr.whenEvaluator.evaluate(p.Name, p.When, pr.whenEvaluator.getParameters())
		if err != nil {
			return result, err
		}

		if whenEvaluationResult {

			r, err := pr.runStage(dir, envvars, *p)
			if err != nil {

				// add error to log lines
				r.LogLines = append(r.LogLines, buildJobLogLine{
					timestamp: time.Now().UTC(),
					logLevel:  "stderr",
					logText:   err.Error(),
				})

				// set 'failed' build status
				pr.envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "failed")
				envvars[pr.envvarHelper.getEstafetteEnvvarName("ESTAFETTE_BUILD_STATUS")] = "failed"

				r.Status = "FAILED"
				r.OtherError = err

				result.StageResults = append(result.StageResults, r)

				continue
			}

			// set 'succeeded' build status
			r.Status = "SUCCEEDED"

			result.StageResults = append(result.StageResults, r)

		} else {

			// if an error has happened in one of the previous steps or the when expression evaluates to false we still want to render the following steps in the result table
			r := estafetteStageRunResult{
				Stage:  *p,
				Status: "SKIPPED",
			}

			result.StageResults = append(result.StageResults, r)

			continue
		}
	}

	return
}
