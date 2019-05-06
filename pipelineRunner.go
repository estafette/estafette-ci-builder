package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/rs/zerolog/log"
)

// PipelineRunner is the interface for running the pipeline steps
type PipelineRunner interface {
	runStage(string, map[string]string, manifest.EstafetteStage) (estafetteStageRunResult, error)
	runStages([]*manifest.EstafetteStage, string, map[string]string) (estafetteRunStagesResult, error)
	prefetchImages([]*manifest.EstafetteStage)
	stopPipelineOnCancellation()
}

type pipelineRunnerImpl struct {
	envvarHelper        EnvvarHelper
	whenEvaluator       WhenEvaluator
	dockerRunner        DockerRunner
	runAsJob            bool
	cancellationChannel chan struct{}
	canceled            bool
}

// NewPipelineRunner returns a new PipelineRunner
func NewPipelineRunner(envvarHelper EnvvarHelper, whenEvaluator WhenEvaluator, dockerRunner DockerRunner, runAsJob bool, cancellationChannel chan struct{}) PipelineRunner {
	return &pipelineRunnerImpl{
		envvarHelper:        envvarHelper,
		whenEvaluator:       whenEvaluator,
		dockerRunner:        dockerRunner,
		runAsJob:            runAsJob,
		cancellationChannel: cancellationChannel,
	}
}

type estafetteStageRunResult struct {
	Stage               manifest.EstafetteStage
	RunIndex            int
	IsDockerImagePulled bool
	IsTrustedImage      bool
	DockerImageSize     int64
	DockerPullDuration  time.Duration
	DockerPullError     error
	DockerRunDuration   time.Duration
	DockerRunError      error
	OtherError          error
	ExitCode            int64
	Status              string
	LogLines            []contracts.BuildLogLine
	Canceled            bool
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
	result.IsTrustedImage = pr.dockerRunner.isTrustedImage(p)

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
	result.LogLines, result.ExitCode, result.Canceled, result.DockerRunError = pr.dockerRunner.runDockerRun(dir, envvars, p)
	result.DockerRunDuration = time.Since(dockerRunStart)

	// log tailing - finalize stage
	if pr.runAsJob {

		status := "SUCCEEDED"
		if result.DockerRunError != nil {
			status = "FAILED"
		}
		if result.Canceled {
			status = "CANCELED"
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
		err = result.DockerRunError
	}

	if result.Canceled {
		log.Info().Msgf("[%v] Canceled pipeline '%v'", p.Name, p.Name)
	} else if err != nil {
		log.Warn().Err(err).Msgf("[%v] Pipeline '%v' container failed", p.Name, p.Name)
	} else {
		log.Info().Msgf("[%v] Finished pipeline '%v' successfully", p.Name, p.Name)
	}

	return
}

type estafetteRunStagesResult struct {
	StageResults []estafetteStageRunResult
	canceled     bool
}

// AggregatedErrors combines the different type of errors that occurred during this pipeline stage
func (result *estafetteRunStagesResult) AggregatedErrors() (errors []error) {

	for _, pr := range result.StageResults {
		if pr.RunIndex == pr.Stage.Retries && pr.HasErrors() {
			errors = append(errors, pr.Errors()...)
		}
	}

	return errors
}

// HasAggregatedErrors indicates whether any errors happened in all pipeline stages excluding retried stages that succeeded eventually
func (result *estafetteRunStagesResult) HasAggregatedErrors() bool {

	errors := result.AggregatedErrors()

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

		// handle cancellation happening in between stages
		if pr.canceled {
			result.canceled = pr.canceled
			return
		}

		whenEvaluationResult, err := pr.whenEvaluator.evaluate(p.Name, p.When, pr.whenEvaluator.getParameters())
		if err != nil {
			result.canceled = pr.canceled
			return result, err
		}

		if whenEvaluationResult {

			runIndex := 0
			for runIndex <= p.Retries {
				r, err := pr.runStage(dir, envvars, *p)
				r.RunIndex = runIndex

				// if canceled during stage stop further execution
				if r.Canceled || pr.canceled {
					r.Status = "CANCELED"
					result.StageResults = append(result.StageResults, r)
					result.canceled = r.Canceled
					return result, nil
				}

				if err != nil {

					// add error to log lines
					r.LogLines = append(r.LogLines, contracts.BuildLogLine{
						LineNumber: 1,
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

					runIndex++

					continue
				}

				// set 'succeeded' build status
				r.Status = "SUCCEEDED"

				result.StageResults = append(result.StageResults, r)

				break
			}

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

	result.canceled = pr.canceled
	return
}

func (pr *pipelineRunnerImpl) prefetchImages(stages []*manifest.EstafetteStage) {

	prefetchStart := time.Now()

	// deduplicate stages by image path
	dedupedStages := []*manifest.EstafetteStage{}
	for _, p := range stages {

		// test if it's already added
		alreadyAdded := false
		for _, d := range dedupedStages {
			if p.ContainerImage == d.ContainerImage {
				alreadyAdded = true
				break
			}
		}

		// added if it hasn't been added before
		if !alreadyAdded {
			dedupedStages = append(dedupedStages, p)
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(dedupedStages))

	// pull all images in parallel
	for _, p := range dedupedStages {
		go func(p manifest.EstafetteStage) {
			defer wg.Done()
			log.Debug().Msgf("Prefetching image %v...", p.ContainerImage)
			pr.dockerRunner.runDockerPull(p)
		}(*p)
	}

	// wait for all pulls to finish
	wg.Wait()
	prefetchDuration := time.Since(prefetchStart)

	log.Debug().Msgf("Done prefetching %v images in %v seconds", len(dedupedStages), prefetchDuration.Seconds)
}

func (pr *pipelineRunnerImpl) stopPipelineOnCancellation() {
	// wait for cancellation
	<-pr.cancellationChannel

	pr.canceled = true
}
