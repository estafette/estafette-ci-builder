package main

import (
	"fmt"
	"os"
	"time"

	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/rs/zerolog/log"
)

// PipelineRunner is the interface for running the pipeline steps
type PipelineRunner interface {
	runStage(string, map[string]string, manifest.EstafetteStage) (estafetteStageRunResult, error)
	runStages([]*manifest.EstafetteStage, string, map[string]string) (estafetteRunStagesResult, error)
}

type pipelineRunnerImpl struct {
	envvarHelper  EnvvarHelper
	whenEvaluator WhenEvaluator
	dockerRunner  DockerRunner
	runAsJob      bool
}

// NewPipelineRunner returns a new PipelineRunner
func NewPipelineRunner(envvarHelper EnvvarHelper, whenEvaluator WhenEvaluator, dockerRunner DockerRunner, runAsJob bool) PipelineRunner {
	return &pipelineRunnerImpl{
		envvarHelper:  envvarHelper,
		whenEvaluator: whenEvaluator,
		dockerRunner:  dockerRunner,
		runAsJob:      runAsJob,
	}
}

type estafetteStageRunResult struct {
	Stage               manifest.EstafetteStage
	RunIndex            int
	IsDockerImagePulled bool
	DockerImageSize     int64
	DockerPullDuration  time.Duration
	DockerPullError     error
	DockerRunDuration   time.Duration
	DockerRunError      error
	OtherError          error
	ExitCode            int64
	Status              string
	LogLines            []contracts.BuildLogLine
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

	p.ContainerImage = os.Expand(p.ContainerImage, pr.envvarHelper.getEstafetteEnv)

	result.Stage = p
	result.LogLines = make([]contracts.BuildLogLine, 0)

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

	// log tailing - start stage
	if pr.runAsJob {
		tailLogLine := contracts.TailLogLine{
			Step:         p.Name,
			Image:        getBuildLogStepDockerImage(result),
			AutoInjected: &result.Stage.AutoInjected,
		}

		// log as json, to be tailed when looking at live logs from gui
		log.Info().Interface("tailLogLine", tailLogLine).Msg("")
	}

	// run commands in docker container
	dockerRunStart := time.Now()
	result.LogLines, result.ExitCode, result.DockerRunError = pr.dockerRunner.runDockerRun(dir, envvars, p)
	result.DockerRunDuration = time.Since(dockerRunStart)

	// log tailing - finalize stage
	if pr.runAsJob {

		status := "SUCCEEDED"
		if result.DockerRunError != nil {
			status = "FAILED"
		}

		tailLogLine := contracts.TailLogLine{
			Step:     p.Name,
			Duration: &result.DockerRunDuration,
			ExitCode: &result.ExitCode,
			Status:   &status,
		}

		// log as json, to be tailed when looking at live logs from gui
		log.Info().Interface("tailLogLine", tailLogLine).Msg("")
	}

	if result.DockerRunError != nil {
		return result, result.DockerRunError
	}

	log.Info().Msgf("[%v] Finished pipeline '%v' successfully", p.Name, p.Name)

	return
}

type estafetteRunStagesResult struct {
	StageResults []estafetteStageRunResult
	Status       string
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

func (pr *pipelineRunnerImpl) runStages(stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (result estafetteRunStagesResult, err error) {

	// set default build status if not set
	err = pr.envvarHelper.initBuildStatus()
	if err != nil {
		return
	}

	if len(stages) == 0 {
		return result, fmt.Errorf("Manifest has no stages, failing the build")
	}

	for _, p := range stages {

		whenEvaluationResult, err := pr.whenEvaluator.evaluate(p.Name, p.When, pr.whenEvaluator.getParameters())
		if err != nil {
			return result, err
		}

		if whenEvaluationResult {

			runIndex := 0
			for runIndex <= p.Retries {

				r, err := pr.runStage(dir, envvars, *p)
				if err != nil {

					// add error to log lines
					r.RunIndex = runIndex
					r.LogLines = append(r.LogLines, contracts.BuildLogLine{
						Timestamp:  time.Now().UTC(),
						StreamType: "stderr",
						Text:       err.Error(),
					})

					// set 'failed' build status
					if runIndex == p.Retries {
						pr.envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "failed")
						envvars[pr.envvarHelper.getEstafetteEnvvarName("ESTAFETTE_BUILD_STATUS")] = "failed"
					}

					r.Status = "FAILED"
					r.OtherError = err

					result.StageResults = append(result.StageResults, r)
					// set 'failed' build status
					result.Status = "failed"

					runIndex++

					continue
				}

				r.Status = "SUCCEEDED"

				result.StageResults = append(result.StageResults, r)
				// set 'succeeded' build status
				result.Status = "succeeded"

				break
			}

		} else {

			// if an error has happened in one of the previous steps or the when expression evaluates to false we still want to render the following steps in the result table
			r := estafetteStageRunResult{
				Stage:  *p,
				Status: "SKIPPED",
			}

			result.StageResults = append(result.StageResults, r)
			result.Status = "succeeded"

			continue
		}
	}

	return
}
