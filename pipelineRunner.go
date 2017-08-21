package main

import (
	"time"

	"github.com/rs/zerolog/log"
)

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

func runPipeline(dir string, envvars map[string]string, p estafettePipeline) (result estafettePipelineRunResult, err error) {

	result.Pipeline = p

	log.Info().Msgf("Starting pipeline '%v'", p.Name)

	result.IsDockerImagePulled = isDockerImagePulled(p)

	if !result.IsDockerImagePulled {

		// pull docker image
		dockerPullStart := time.Now()
		result.DockerPullError = runDockerPull(p)
		result.DockerPullDuration = time.Since(dockerPullStart)
		if result.DockerPullError != nil {
			return result, result.DockerPullError
		}

		// set docker image size
		size, err := getDockerImageSize(p)
		if err != nil {
			return result, err
		}
		result.DockerImageSize = size

	}

	// run commands in docker container
	dockerRunStart := time.Now()
	result.DockerRunError = runDockerRun(dir, envvars, p)
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

func runPipelines(manifest estafetteManifest, dir string, envvars map[string]string) (result estafetteRunPipelinesResult, err error) {

	// set default build status if not set
	err = initBuildStatus()
	if err != nil {
		return
	}

	for _, p := range manifest.Pipelines {

		whenEvaluationResult, err := whenEvaluator(p.When, whenParameters())
		if err != nil {
			return result, err
		}

		if whenEvaluationResult {

			r, err := runPipeline(dir, envvars, *p)
			if err != nil {

				// set 'failed' build status
				setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "failed")
				envvars[getEstafetteEnvvarName("ESTAFETTE_BUILD_STATUS")] = "failed"

				r.Status = "FAILED"
				r.OtherError = err

				result.PipelineResults = append(result.PipelineResults, r)

				continue
			}

			// set 'succeeded' build status
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
