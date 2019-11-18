package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
)

// PipelineRunner is the interface for running the pipeline steps
type PipelineRunner interface {
	runStage(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (result estafetteStageRunResult, err error)
	runService(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (result estafetteServiceRunResult, err error)
	runStages(ctx context.Context, depth int, stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (result estafetteRunStagesResult, err error)
	runParallelStages(ctx context.Context, depth int, parentStage *manifest.EstafetteStage, stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (result estafetteStageRunResult, err error)
	runServices(ctx context.Context, parentStage *manifest.EstafetteStage, services []*manifest.EstafetteService, envvars map[string]string) (result estafetteStageRunResult, err error)
	stopPipelineOnCancellation()
	tailLogs(ctx context.Context)
	getLogs(ctx context.Context) []*contracts.BuildLogStep
}

type pipelineRunnerImpl struct {
	envvarHelper        EnvvarHelper
	whenEvaluator       WhenEvaluator
	dockerRunner        DockerRunner
	runAsJob            bool
	cancellationChannel chan struct{}
	tailLogsChannel     chan contracts.TailLogLine
	buildLogSteps       []*contracts.BuildLogStep
	canceled            bool
}

// NewPipelineRunner returns a new PipelineRunner
func NewPipelineRunner(envvarHelper EnvvarHelper, whenEvaluator WhenEvaluator, dockerRunner DockerRunner, runAsJob bool, cancellationChannel chan struct{}, tailLogsChannel chan contracts.TailLogLine) PipelineRunner {
	return &pipelineRunnerImpl{
		envvarHelper:        envvarHelper,
		whenEvaluator:       whenEvaluator,
		dockerRunner:        dockerRunner,
		runAsJob:            runAsJob,
		cancellationChannel: cancellationChannel,
		tailLogsChannel:     tailLogsChannel,
		buildLogSteps:       make([]*contracts.BuildLogStep, 0),
	}
}

type estafetteStageRunResult struct {
	Stage                 manifest.EstafetteStage
	Depth                 int
	RunIndex              int
	IsDockerImagePulled   bool
	IsTrustedImage        bool
	DockerImageSize       int64
	DockerPullDuration    time.Duration
	DockerPullError       error
	DockerRunDuration     time.Duration
	DockerRunError        error
	OtherError            error
	ExitCode              int64
	Status                string
	LogLines              []contracts.BuildLogLine
	Canceled              bool
	ParallelStagesResults []estafetteStageRunResult
	ServicesResults       []estafetteServiceRunResult
}

type estafetteServiceRunResult struct {
	Service             manifest.EstafetteService
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

	for _, pr := range result.ParallelStagesResults {
		if pr.RunIndex == pr.Stage.Retries && pr.HasErrors() {
			errors = append(errors, pr.Errors()...)
		}
	}
	for _, s := range result.ServicesResults {
		if s.HasErrors() {
			errors = append(errors, s.Errors()...)
		}
	}

	return errors
}

func (result *estafetteServiceRunResult) Errors() (errors []error) {

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

// HasErrors indicates whether any errors happened in this pipeline stage
func (result *estafetteServiceRunResult) HasErrors() bool {

	errors := result.Errors()

	return len(errors) > 0
}

func (pr *pipelineRunnerImpl) runStage(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (result estafetteStageRunResult, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunStage")
	defer span.Finish()
	span.SetTag("stage", p.Name)

	parentStageName := ""
	if parentStage != nil {
		parentStageName = parentStage.Name
	}

	p.ContainerImage = os.Expand(p.ContainerImage, pr.envvarHelper.getEstafetteEnv)

	result.Stage = p
	result.Depth = depth
	result.RunIndex = runIndex
	result.LogLines = make([]contracts.BuildLogLine, 0)

	log.Info().Msgf("[%v] Starting stage '%v'", p.Name, p.Name)

	if p.ContainerImage != "" {
		result.IsDockerImagePulled = pr.dockerRunner.isDockerImagePulled(p.Name, p.ContainerImage)
		result.IsTrustedImage = pr.dockerRunner.isTrustedImage(p.Name, p.ContainerImage)

		if !result.IsDockerImagePulled || runtime.GOOS == "windows" {

			// log tailing - start stage as pending
			status := "PENDING"
			pr.tailLogsChannel <- contracts.TailLogLine{
				Step:         p.Name,
				ParentStage:  parentStageName,
				Type:         "stage",
				Depth:        depth,
				RunIndex:     runIndex,
				Image:        getBuildLogStepDockerImage(result),
				AutoInjected: &result.Stage.AutoInjected,
				Status:       &status,
			}

			// pull docker image
			dockerPullStart := time.Now()
			result.DockerPullError = pr.dockerRunner.runDockerPull(ctx, p.Name, p.ContainerImage)
			result.DockerPullDuration = time.Since(dockerPullStart)
			if result.DockerPullError != nil {
				return result, result.DockerPullError
			}
		}

		// set docker image size
		size, err := pr.dockerRunner.getDockerImageSize(p.ContainerImage)
		if err != nil {
			return result, err
		}
		result.DockerImageSize = size
	}

	// log tailing - start stage
	status := "RUNNING"
	pr.tailLogsChannel <- contracts.TailLogLine{
		Step:         p.Name,
		ParentStage:  parentStageName,
		Type:         "stage",
		Depth:        depth,
		RunIndex:     runIndex,
		Image:        getBuildLogStepDockerImage(result),
		AutoInjected: &result.Stage.AutoInjected,
		Status:       &status,
	}

	// run commands in docker container
	dockerRunStart := time.Now()

	parentStageName = ""
	if parentStage != nil {
		parentStageName = parentStage.Name
	}

	if len(p.Services) > 0 {
		// this stage has service containers, start them first
		innerResult, err := pr.runServices(ctx, &p, p.Services, envvars)

		result.ServicesResults = innerResult.ServicesResults
		result.Canceled = innerResult.Canceled
		result.DockerRunError = err
	}

	if result.DockerRunError == nil {
		if len(p.ParallelStages) > 0 {
			if depth == 0 {
				innerResult, err := pr.runParallelStages(ctx, depth+1, &p, p.ParallelStages, dir, envvars)

				result.ParallelStagesResults = innerResult.ParallelStagesResults
				result.Canceled = innerResult.Canceled
				result.DockerRunError = err
			} else {
				log.Warn().Msgf("Can't run parallel stages nested inside nested stages")
			}
		} else {
			containerID, dockerRunError := pr.dockerRunner.runDockerRun(ctx, depth, runIndex, dir, envvars, parentStage, p)
			result.DockerRunError = dockerRunError
			if dockerRunError == nil {
				result.LogLines, result.ExitCode, result.Canceled, result.DockerRunError = pr.dockerRunner.tailDockerLogs(ctx, containerID, parentStageName, p.Name, "stage", depth, runIndex)
			}
		}
	}

	result.DockerRunDuration = time.Since(dockerRunStart)

	// log tailing - finalize stage
	status = "SUCCEEDED"
	if result.DockerRunError != nil {
		status = "FAILED"
	}
	if result.Canceled {
		status = "CANCELED"
	}

	pr.tailLogsChannel <- contracts.TailLogLine{
		Step:        p.Name,
		ParentStage: parentStageName,
		Type:        "stage",
		Depth:       depth,
		RunIndex:    runIndex,
		Duration:    &result.DockerRunDuration,
		ExitCode:    &result.ExitCode,
		Status:      &status,
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

	if len(p.Services) > 0 {
		// this stage has service containers, stop them now that the stage has finished
		go pr.dockerRunner.stopServices(ctx, &p, p.Services)
	}

	return
}

func (pr *pipelineRunnerImpl) runService(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (result estafetteServiceRunResult, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunService")
	defer span.Finish()
	span.SetTag("service", service.Name)

	service.ContainerImage = os.Expand(service.ContainerImage, pr.envvarHelper.getEstafetteEnv)

	parentStageName := ""
	if parentStage != nil {
		parentStageName = parentStage.Name
	}

	result.Service = service
	result.LogLines = make([]contracts.BuildLogLine, 0)

	log.Info().Msgf("[%v] Starting service container '%v'", parentStageName, service.Name)

	if service.ContainerImage != "" {
		result.IsDockerImagePulled = pr.dockerRunner.isDockerImagePulled(parentStageName, service.ContainerImage)
		result.IsTrustedImage = pr.dockerRunner.isTrustedImage(parentStageName, service.ContainerImage)
		if !result.IsDockerImagePulled || runtime.GOOS == "windows" {

			// log tailing - start stage as pending
			status := "PENDING"
			pr.tailLogsChannel <- contracts.TailLogLine{
				Step:        service.Name,
				ParentStage: parentStageName,
				Type:        "service",
				Image:       getBuildLogStepDockerImageForService(result),
				Status:      &status,
			}

			// pull docker image
			err = pr.dockerRunner.runDockerPull(ctx, parentStageName, service.ContainerImage)
		}

		// set docker image size
		size, err := pr.dockerRunner.getDockerImageSize(service.ContainerImage)
		if err != nil {
			return result, err
		}
		result.DockerImageSize = size
	}

	// log tailing - start stage
	status := "RUNNING"
	pr.tailLogsChannel <- contracts.TailLogLine{
		Step:        service.Name,
		ParentStage: parentStageName,
		Type:        "service",
		Image:       getBuildLogStepDockerImageForService(result),
		Status:      &status,
	}

	// run commands in docker container
	dockerRunStart := time.Now()

	containerID, _, dockerRunError := pr.dockerRunner.runDockerRunService(ctx, envvars, parentStage, service)
	result.DockerRunError = dockerRunError

	if dockerRunError == nil {
		go pr.dockerRunner.tailDockerLogs(ctx, containerID, parentStageName, service.Name, "service", 1, 0)
	}

	// wait for service to be ready if readiness probe is defined
	if service.Readiness != nil && parentStage != nil {
		log.Info().Msgf("[%v] Starting readiness probe...", parentStage.Name)
		err = pr.dockerRunner.runDockerRunReadinessProber(ctx, *parentStage, service, *service.Readiness)
	}
	result.DockerRunDuration = time.Since(dockerRunStart)

	// log tailing - finalize stage
	status = "SUCCEEDED"
	if result.DockerRunError != nil {
		status = "FAILED"
	}
	if result.Canceled {
		status = "CANCELED"
	}

	pr.tailLogsChannel <- contracts.TailLogLine{
		Step:        service.Name,
		ParentStage: parentStageName,
		Type:        "service",
		Duration:    &result.DockerRunDuration,
		ExitCode:    &result.ExitCode,
		Status:      &status,
	}

	if result.DockerRunError != nil {
		err = result.DockerRunError
	}

	if result.Canceled {
		log.Info().Msgf("[%v] Canceled service '%v'", parentStageName, service.Name)
	} else if err != nil {
		log.Warn().Err(err).Msgf("[%v] Service '%v' container failed", parentStageName, service.Name)
	} else {
		log.Info().Msgf("[%v] Started service '%v' successfully", parentStageName, service.Name)
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

func (pr *pipelineRunnerImpl) runStages(ctx context.Context, depth int, stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (result estafetteRunStagesResult, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunStages")
	defer span.Finish()

	err = pr.dockerRunner.createBridgeNetwork(ctx)
	if err != nil {
		return
	}
	defer func(ctx context.Context) {
		_ = pr.dockerRunner.deleteBridgeNetwork(ctx)
	}(ctx)

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

				r, err := pr.runStage(ctx, depth, runIndex, dir, envvars, nil, *p)
				r.RunIndex = runIndex

				// if canceled during stage stop further execution
				if r.Canceled || pr.canceled {
					r.Status = "CANCELED"
					result.StageResults = append(result.StageResults, r)
					result.canceled = true
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
				Depth:  depth,
				Status: "SKIPPED",
			}

			result.StageResults = append(result.StageResults, r)

			status := "SKIPPED"
			pr.tailLogsChannel <- contracts.TailLogLine{
				Step:         p.Name,
				Type:         "stage",
				Depth:        depth,
				RunIndex:     0,
				Image:        getBuildLogStepDockerImage(r),
				AutoInjected: &r.Stage.AutoInjected,
				Status:       &status,
			}

			continue
		}
	}

	result.canceled = pr.canceled

	return
}

func (pr *pipelineRunnerImpl) runParallelStages(ctx context.Context, depth int, parentStage *manifest.EstafetteStage, stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (result estafetteStageRunResult, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunStages")
	defer span.Finish()

	// set default build status if not set
	err = pr.envvarHelper.initBuildStatus()
	if err != nil {
		return
	}

	if len(stages) == 0 {
		return result, fmt.Errorf("Manifest has no stages, failing the build")
	}

	var wg sync.WaitGroup
	wg.Add(len(stages))

	results := make(chan estafetteStageRunResult, len(stages))
	errors := make(chan error, len(stages))

	for _, p := range stages {

		go func(ctx context.Context, p *manifest.EstafetteStage, dir string, envvars map[string]string) {
			defer wg.Done()

			// handle cancellation happening in between stages
			if pr.canceled {
				return
			}

			whenEvaluationResult, err := pr.whenEvaluator.evaluate(p.Name, p.When, pr.whenEvaluator.getParameters())
			if err != nil {
				errors <- err
				return
			}

			if whenEvaluationResult {

				runIndex := 0
				for runIndex <= p.Retries {

					r, err := pr.runStage(ctx, depth, runIndex, dir, envvars, parentStage, *p)
					r.RunIndex = runIndex

					// if canceled during stage stop further execution
					if r.Canceled || pr.canceled {
						r.Status = "CANCELED"

						results <- r

						return
					}

					if err != nil {

						// add error to log lines
						r.LogLines = append(r.LogLines, contracts.BuildLogLine{
							LineNumber: 1,
							Timestamp:  time.Now().UTC(),
							StreamType: "stderr",
							Text:       err.Error(),
						})

						r.Status = "FAILED"
						r.OtherError = err

						results <- r
						errors <- err

						runIndex++

						continue
					}

					// set 'succeeded' build status
					r.Status = "SUCCEEDED"

					results <- r

					break
				}

			} else {

				// if an error has happened in one of the previous steps or the when expression evaluates to false we still want to render the following steps in the result table
				r := estafetteStageRunResult{
					Stage:  *p,
					Status: "SKIPPED",
				}

				results <- r

				status := "SKIPPED"
				pr.tailLogsChannel <- contracts.TailLogLine{
					Step:         p.Name,
					ParentStage:  parentStage.Name,
					Type:         "stage",
					Depth:        depth,
					RunIndex:     0,
					Image:        getBuildLogStepDockerImage(r),
					AutoInjected: &r.Stage.AutoInjected,
					Status:       &status,
				}
			}
		}(ctx, p, dir, envvars)
	}

	// TODO as soon as one parallel stage fails cancel the others

	wg.Wait()

	result.Canceled = pr.canceled

	close(results)
	result.Status = "SUCCEEDED"
	for r := range results {
		result.ParallelStagesResults = append(result.ParallelStagesResults, r)

		// if canceled during one of the stages stop further execution
		if r.Canceled || pr.canceled {
			result.Status = "CANCELED"
			result.Canceled = true

			continue
		}

		// ensure outer stage gets correct status
		if !result.Canceled && r.Status == "FAILED" {
			result.Status = r.Status

			pr.envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "failed")
		}
	}

	close(errors)
	for e := range errors {
		err = e
		result.OtherError = err
		return
	}

	return
}

func (pr *pipelineRunnerImpl) runServices(ctx context.Context, parentStage *manifest.EstafetteStage, services []*manifest.EstafetteService, envvars map[string]string) (result estafetteStageRunResult, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunServices")
	defer span.Finish()

	var wg sync.WaitGroup
	wg.Add(len(services))

	results := make(chan estafetteServiceRunResult, len(services))
	errors := make(chan error, len(services))

	for _, s := range services {
		go func(ctx context.Context, parentStage *manifest.EstafetteStage, s *manifest.EstafetteService, envvars map[string]string) {
			defer wg.Done()

			if parentStage != nil {
				log.Info().Msgf("Starting service '%v' for stage '%v'...", s.ContainerImage, parentStage.Name)
			}

			whenEvaluationResult, err := pr.whenEvaluator.evaluate(s.Name, s.When, pr.whenEvaluator.getParameters())
			if err != nil {
				errors <- err
				return
			}

			if whenEvaluationResult {
				r, err := pr.runService(ctx, envvars, parentStage, *s)
				results <- r
				if err != nil {
					errors <- err
				}
			} else {

				results <- estafetteServiceRunResult{
					Service: *s,
					Status:  "SKIPPED",
				}
			}
		}(ctx, parentStage, s, envvars)
	}

	wg.Wait()

	close(results)
	result.Status = "SUCCEEDED"
	for r := range results {
		result.ServicesResults = append(result.ServicesResults, r)

		// if canceled during one of the stages stop further execution
		if r.Canceled || pr.canceled {
			result.Status = "CANCELED"
			result.Canceled = true

			continue
		}

		// ensure outer stage gets correct status
		if !result.Canceled && r.Status == "FAILED" {
			result.Status = r.Status

			pr.envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "failed")
		}

		if !result.Canceled && r.Status == "SKIPPED" {
			result.Status = r.Status
		}
	}

	close(errors)
	for e := range errors {
		err = e
		return
	}

	return
}

func (pr *pipelineRunnerImpl) stopPipelineOnCancellation() {
	// wait for cancellation
	<-pr.cancellationChannel

	pr.canceled = true
}

func (pr *pipelineRunnerImpl) tailLogs(ctx context.Context) {
	for {
		tailLogLine, more := <-pr.tailLogsChannel
		if more {
			if pr.runAsJob {
				// this provides log streaming capabilities in the web interface
				log.Info().Interface("tailLogLine", tailLogLine).Msg("")
			} else if tailLogLine.LogLine != nil {
				// this is for go.cd
				if tailLogLine.ParentStage != "" {
					log.Info().Msgf("[%v][%v] %v", tailLogLine.ParentStage, tailLogLine.Step, tailLogLine.LogLine.Text)
				} else {
					log.Info().Msgf("[%v] %v", tailLogLine.Step, tailLogLine.LogLine.Text)
				}
			}

			pr.upsertTailLogLine(tailLogLine)

		} else {
			// channel is closed, we can return now
			return
		}
	}
}

func (pr *pipelineRunnerImpl) getLogs(ctx context.Context) []*contracts.BuildLogStep {

	// close log channel
	// close(pr.tailLogsChannel)

	return pr.buildLogSteps
}

func (pr *pipelineRunnerImpl) upsertTailLogLine(tailLogLine contracts.TailLogLine) {

	// check if tailLogLine.Step (in combination with parentstage and type if applicable) already exists in pr.buildLogSteps
	mainStage := pr.getMainBuildLogStep(tailLogLine)
	if mainStage == nil {
		if tailLogLine.ParentStage != "" {
			mainStage = &contracts.BuildLogStep{
				Step: tailLogLine.ParentStage,
			}
			pr.buildLogSteps = append(pr.buildLogSteps, mainStage)
		} else {
			mainStage = &contracts.BuildLogStep{
				Step:     tailLogLine.Step,
				RunIndex: tailLogLine.RunIndex,
			}
			pr.buildLogSteps = append(pr.buildLogSteps, mainStage)
		}
	}

	if tailLogLine.ParentStage != "" {
		if tailLogLine.Type == "stage" {
			nestedStage := pr.getNestedBuildLogStep(tailLogLine)
			if nestedStage == nil {
				nestedStage = &contracts.BuildLogStep{
					Step: tailLogLine.Step,
				}
				mainStage.NestedSteps = append(mainStage.NestedSteps, nestedStage)
			}

			// set non-identifying properties
			if tailLogLine.LogLine != nil {
				nestedStage.LogLines = append(nestedStage.LogLines, *tailLogLine.LogLine)
			}
			if tailLogLine.Image != nil {
				nestedStage.Image = tailLogLine.Image
			}
			if tailLogLine.Duration != nil {
				nestedStage.Duration = *tailLogLine.Duration
			}
			if tailLogLine.ExitCode != nil {
				nestedStage.ExitCode = *tailLogLine.ExitCode
			}
			if tailLogLine.Status != nil {
				nestedStage.Status = *tailLogLine.Status
			}
			if tailLogLine.AutoInjected != nil {
				nestedStage.AutoInjected = *tailLogLine.AutoInjected
			}

		} else if tailLogLine.Type == "service" {
			nestedService := pr.getNestedBuildLogService(tailLogLine)
			if nestedService == nil {
				nestedService = &contracts.BuildLogStep{
					Step: tailLogLine.Step,
				}
				mainStage.Services = append(mainStage.Services, nestedService)
			}

			// set non-identifying properties
			if tailLogLine.LogLine != nil {
				nestedService.LogLines = append(nestedService.LogLines, *tailLogLine.LogLine)
			}
			if tailLogLine.Image != nil {
				nestedService.Image = tailLogLine.Image
			}
			if tailLogLine.Duration != nil {
				nestedService.Duration = *tailLogLine.Duration
			}
			if tailLogLine.ExitCode != nil {
				nestedService.ExitCode = *tailLogLine.ExitCode
			}
			if tailLogLine.Status != nil {
				nestedService.Status = *tailLogLine.Status
			}
			if tailLogLine.AutoInjected != nil {
				nestedService.AutoInjected = *tailLogLine.AutoInjected
			}
		}
	} else {

		// set non-identifying properties
		if tailLogLine.LogLine != nil {
			mainStage.LogLines = append(mainStage.LogLines, *tailLogLine.LogLine)
		}
		if tailLogLine.Image != nil {
			mainStage.Image = tailLogLine.Image
		}
		if tailLogLine.Duration != nil {
			mainStage.Duration = *tailLogLine.Duration
		}
		if tailLogLine.ExitCode != nil {
			mainStage.ExitCode = *tailLogLine.ExitCode
		}
		if tailLogLine.Status != nil {
			mainStage.Status = *tailLogLine.Status
		}
		if tailLogLine.AutoInjected != nil {
			mainStage.AutoInjected = *tailLogLine.AutoInjected
		}
	}
}

func (pr *pipelineRunnerImpl) getMainBuildLogStep(tailLogLine contracts.TailLogLine) *contracts.BuildLogStep {

	stepToFind := tailLogLine.Step
	if tailLogLine.ParentStage != "" {
		stepToFind = tailLogLine.ParentStage
	}

	for _, bls := range pr.buildLogSteps {
		if bls.Step == stepToFind {
			if bls.RunIndex == tailLogLine.RunIndex {
				return bls
			}
		}
	}

	return nil
}

func (pr *pipelineRunnerImpl) getNestedBuildLogStep(tailLogLine contracts.TailLogLine) *contracts.BuildLogStep {

	if tailLogLine.ParentStage == "" || tailLogLine.Type != "stage" {
		return nil
	}

	for _, bls := range pr.buildLogSteps {
		if bls.Step == tailLogLine.ParentStage {
			if tailLogLine.ParentStage != "" {
				// we have to look deeper
				if tailLogLine.Type == "stage" {
					// look inside the parallel stages
					for _, ns := range bls.NestedSteps {
						if ns.Step == tailLogLine.Step && ns.RunIndex == tailLogLine.RunIndex {
							return ns
						}
					}
				}
			}
		}
	}

	return nil
}

func (pr *pipelineRunnerImpl) getNestedBuildLogService(tailLogLine contracts.TailLogLine) *contracts.BuildLogStep {

	if tailLogLine.ParentStage == "" || tailLogLine.Type != "service" {
		return nil
	}

	for _, bls := range pr.buildLogSteps {
		if bls.Step == tailLogLine.ParentStage {
			if tailLogLine.ParentStage != "" {
				// we have to look deeper
				if tailLogLine.Type == "service" {
					// look inside the services
					for _, s := range bls.Services {
						if s.Step == tailLogLine.Step {
							return s
						}
					}
				}
			}
		}
	}

	return nil
}
