package main

import (
	"time"

	"github.com/rs/zerolog/log"
)

// PipelineRunner is the interface for running the pipeline steps
type PipelineRunner interface {
	runPipeline(string, map[string]string, estafettePipeline) (estafettePipelineRunResult, error)
	runPipelines(estafetteManifest, string, map[string]string) (estafetteRunPipelinesResult, error)
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

type estafettePipelineRunResult struct {
	Pipeline            estafettePipeline
	IsDockerImagePulled bool
	DockerImageSize     int64
	DockerPullDuration  time.Duration
	DockerPullError     error
	DockerRunDuration   time.Duration
	DockerRunError      error
	OtherError          error
	Status              string
}

// Errors combines the different type of errors that occurred during this pipeline step
func (result *estafettePipelineRunResult) Errors() (errors []error) {

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

// HasErrors indicates whether any errors happened in this pipeline step
func (result *estafettePipelineRunResult) HasErrors() bool {

	errors := result.Errors()

	return len(errors) > 0
}

func (pr *pipelineRunnerImpl) runPipeline(dir string, envvars map[string]string, p estafettePipeline) (result estafettePipelineRunResult, err error) {

	result.Pipeline = p

	log.Info().Msgf("Starting pipeline '%v'", p.Name)

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
	result.DockerRunError = pr.dockerRunner.runDockerRun(dir, envvars, p)
	result.DockerRunDuration = time.Since(dockerRunStart)
	if result.DockerRunError != nil {
		return result, result.DockerRunError
	}

	log.Info().Msgf("Finished pipeline '%v' successfully", p.Name)

	return
}

type estafetteRunPipelinesResult struct {
	PipelineResults []estafettePipelineRunResult
}

// Errors combines the different type of errors that occurred during this pipeline step
func (result *estafetteRunPipelinesResult) Errors() (errors []error) {

	for _, pr := range result.PipelineResults {
		if pr.HasErrors() {
			errors = append(errors, pr.Errors()...)
		}
	}

	return errors
}

// HasErrors indicates whether any errors happened in this pipeline step
func (result *estafetteRunPipelinesResult) HasErrors() bool {

	errors := result.Errors()

	return len(errors) > 0
}

func (pr *pipelineRunnerImpl) runPipelines(manifest estafetteManifest, dir string, envvars map[string]string) (result estafetteRunPipelinesResult, err error) {

	// set default build status if not set
	err = pr.envvarHelper.initBuildStatus()
	if err != nil {
		return
	}

	for _, p := range manifest.Pipelines {

		whenEvaluationResult, err := pr.whenEvaluator.evaluate(p.When, pr.whenEvaluator.getParameters())
		if err != nil {
			return result, err
		}

		if whenEvaluationResult {

			r, err := pr.runPipeline(dir, envvars, *p)
			if err != nil {

				// set 'failed' build status
				pr.envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "failed")
				envvars[pr.envvarHelper.getEstafetteEnvvarName("ESTAFETTE_BUILD_STATUS")] = "failed"

				r.Status = "FAILED"
				r.OtherError = err

				result.PipelineResults = append(result.PipelineResults, r)

				continue
			}

			// set 'succeeded' build status
			pr.envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
			envvars[pr.envvarHelper.getEstafetteEnvvarName("ESTAFETTE_BUILD_STATUS")] = "succeeded"
			r.Status = "SUCCEEDED"

			result.PipelineResults = append(result.PipelineResults, r)

		} else {

			// if an error has happened in one of the previous steps or the when expression evaluates to false we still want to render the following steps in the result table
			r := estafettePipelineRunResult{
				Pipeline: *p,
				Status:   "SKIPPED",
			}

			result.PipelineResults = append(result.PipelineResults, r)

			continue
		}
	}

	return
}
