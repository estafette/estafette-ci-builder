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
	RunStage(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error)
	RunStageWithRetry(ctx context.Context, depth int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error)
	RunService(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, service manifest.EstafetteService) (err error)
	RunStages(ctx context.Context, depth int, stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (buildLogSteps []*contracts.BuildLogStep, err error)
	RunParallelStages(ctx context.Context, depth int, dir string, envvars map[string]string, parentStage manifest.EstafetteStage, parallelStages []*manifest.EstafetteStage) (err error)
	RunServices(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, services []*manifest.EstafetteService) (err error)
	StopPipelineOnCancellation()
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

func (pr *pipelineRunnerImpl) RunStage(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "runStage")
	defer span.Finish()
	span.SetTag("stage", stage.Name)

	parentStageName := ""
	stagePlaceholder := fmt.Sprintf("[%v]", stage.Name)
	if parentStage != nil {
		parentStageName = parentStage.Name
		stagePlaceholder = fmt.Sprintf("[%v] [%v]", parentStageName, stage.Name)
	}

	log.Info().Msgf("%v Starting stage '%v'", stagePlaceholder, stage.Name)
	var isPulledImage bool
	var isTrustedImage bool
	var imagePullDuration time.Duration
	var imageSize int64
	var buildLogStepDockerImage *contracts.BuildLogStepDockerImage

	if stage.ContainerImage != "" {
		stage.ContainerImage = os.Expand(stage.ContainerImage, pr.envvarHelper.getEstafetteEnv)
		isPulledImage = pr.dockerRunner.IsImagePulled(stage.Name, stage.ContainerImage)
		isTrustedImage = pr.dockerRunner.IsTrustedImage(stage.Name, stage.ContainerImage)

		if !isPulledImage || runtime.GOOS == "windows" {

			buildLogStepDockerImage = &contracts.BuildLogStepDockerImage{
				Name:      getContainerImageName(stage.ContainerImage),
				Tag:       getContainerImageTag(stage.ContainerImage),
				IsTrusted: isTrustedImage,
				IsPulled:  isPulledImage,
			}

			// start pulling stage
			pr.sendStatusMessage(stage, parentStageName, depth, runIndex, buildLogStepDockerImage, nil, contracts.StatusPending)

			// pull docker image
			dockerPullStart := time.Now()
			err = pr.dockerRunner.PullImage(ctx, stage.Name, stage.ContainerImage)
			imagePullDuration = time.Since(dockerPullStart)
		}

		// set docker image size
		if err == nil {
			imageSize, err = pr.dockerRunner.GetImageSize(stage.ContainerImage)
		}

		if err == nil {
			buildLogStepDockerImage = &contracts.BuildLogStepDockerImage{
				Name:         getContainerImageName(stage.ContainerImage),
				Tag:          getContainerImageTag(stage.ContainerImage),
				IsTrusted:    isTrustedImage,
				IsPulled:     isPulledImage,
				ImageSize:    imageSize,
				PullDuration: imagePullDuration,
			}
		}
	}

	if err == nil {
		// start running stage
		pr.sendStatusMessage(stage, parentStageName, depth, runIndex, buildLogStepDockerImage, nil, contracts.StatusRunning)
	}

	// run commands in docker container
	dockerRunStart := time.Now()

	if err == nil && len(stage.Services) > 0 {
		// this stage has service containers, start them first
		err = pr.RunServices(ctx, envvars, stage, stage.Services)
	}

	if err == nil {
		if len(stage.ParallelStages) > 0 {
			if depth == 0 {
				err = pr.RunParallelStages(ctx, depth+1, dir, envvars, stage, stage.ParallelStages)
			} else {
				log.Warn().Msgf("%v Can't run parallel stages nested inside nested stages", stagePlaceholder)
			}
		} else {
			var containerID string
			containerID, err = pr.dockerRunner.StartStageContainer(ctx, depth, runIndex, dir, envvars, parentStage, stage)
			if err == nil {
				err = pr.dockerRunner.TailContainerLogs(ctx, containerID, parentStageName, stage.Name, "stage", depth, runIndex)
			}
		}
	}

	runDuration := time.Since(dockerRunStart)

	// finalize stage
	finalStatus := contracts.StatusSucceeded
	if pr.canceled {
		log.Info().Msgf("%v Canceled pipeline '%v'", stagePlaceholder, stage.Name)
		finalStatus = contracts.StatusCanceled
	} else if err != nil {
		log.Warn().Err(err).Msgf("%v Pipeline '%v' container failed", stagePlaceholder, stage.Name)
		finalStatus = contracts.StatusFailed
	} else {
		log.Info().Msgf("%v Finished pipeline '%v' successfully", stagePlaceholder, stage.Name)
	}

	pr.sendStatusMessage(stage, parentStageName, depth, runIndex, nil, &runDuration, finalStatus)

	if len(stage.Services) > 0 {
		// this stage has service containers, stop them now that the stage has finished
		go pr.dockerRunner.StopServiceContainers(ctx, &stage, stage.Services)
	}

	return
}

func (pr *pipelineRunnerImpl) RunStageWithRetry(ctx context.Context, depth int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "runStageWithRetry")
	defer span.Finish()
	span.SetTag("stage", stage.Name)

	runIndex := 0
	retries := stage.Retries
	if (len(stage.ParallelStages) > 0 || len(stage.Services) > 0) && retries > 0 {
		// retries are not supported in combination with parallel stages or services for now
		retries = 0
	}

	parentStageName := ""
	if parentStage != nil {
		parentStageName = parentStage.Name
	}

	// retry until successful or number of retries is maxed out
	for runIndex <= retries {
		err = pr.RunStage(ctx, depth, runIndex, dir, envvars, parentStage, stage)

		// if canceled during stage stop further execution
		if pr.canceled {
			return nil
		}

		// if execution is successful, we're done
		if err == nil {
			return nil
		}

		// create log line for error
		logLineObject := contracts.BuildLogLine{
			Timestamp:  time.Now().UTC(),
			StreamType: "stderr",
			Text:       err.Error(),
		}

		pr.tailLogsChannel <- contracts.TailLogLine{
			Step:        stage.Name,
			ParentStage: parentStageName,
			Type:        "stage",
			Depth:       depth,
			RunIndex:    runIndex,
			LogLine:     &logLineObject,
		}

		runIndex++
	}

	if err != nil {
		return err
	}

	return nil
}

func (pr *pipelineRunnerImpl) RunService(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, service manifest.EstafetteService) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "runService")
	defer span.Finish()
	span.SetTag("service", service.Name)

	service.ContainerImage = os.Expand(service.ContainerImage, pr.envvarHelper.getEstafetteEnv)

	parentStageName := parentStage.Name

	log.Info().Msgf("[%v] Starting service container '%v'", parentStageName, service.Name)

	var isPulledImage bool
	var isTrustedImage bool
	var imagePullDuration time.Duration
	var imageSize int64

	if service.ContainerImage != "" {
		isPulledImage = pr.dockerRunner.IsImagePulled(service.Name, service.ContainerImage)
		isTrustedImage = pr.dockerRunner.IsTrustedImage(service.Name, service.ContainerImage)

		if !isPulledImage || runtime.GOOS == "windows" {

			// log tailing - start stage as pending
			pendingStatus := contracts.StatusPending
			pr.tailLogsChannel <- contracts.TailLogLine{
				Step:        service.Name,
				ParentStage: parentStageName,
				Type:        "service",
				Depth:       1,
				Image: &contracts.BuildLogStepDockerImage{
					Name:      getContainerImageName(service.ContainerImage),
					Tag:       getContainerImageTag(service.ContainerImage),
					IsTrusted: isTrustedImage,
				},
				Status: &pendingStatus,
			}

			// pull docker image
			dockerPullStart := time.Now()
			err = pr.dockerRunner.PullImage(ctx, service.Name, service.ContainerImage)
			if err != nil {
				return err
			}

			imagePullDuration = time.Since(dockerPullStart)
		}

		// set docker image size
		imageSize, err = pr.dockerRunner.GetImageSize(service.ContainerImage)
		if err != nil {
			return err
		}
	}

	// log tailing - start stage
	runningStatus := contracts.StatusRunning
	pr.tailLogsChannel <- contracts.TailLogLine{
		Step:        service.Name,
		ParentStage: parentStageName,
		Type:        "service",
		Depth:       1,
		Image: &contracts.BuildLogStepDockerImage{
			Name:         getContainerImageName(service.ContainerImage),
			Tag:          getContainerImageTag(service.ContainerImage),
			IsTrusted:    isTrustedImage,
			IsPulled:     isPulledImage,
			ImageSize:    imageSize,
			PullDuration: imagePullDuration,
		},
		Status: &runningStatus,
	}

	// run commands in docker container
	dockerRunStart := time.Now()

	containerID, err := pr.dockerRunner.StartServiceContainer(ctx, envvars, &parentStage, service)
	if err == nil {
		go func(ctx context.Context, containerID, parentStageName string, service manifest.EstafetteService) {
			err := pr.dockerRunner.TailContainerLogs(ctx, containerID, parentStageName, service.Name, "service", 1, 0)

			finalStatusAfterTailing := contracts.StatusSucceeded
			if pr.canceled {
				log.Info().Msgf("[%v] Canceled service '%v'", parentStageName, service.Name)
				finalStatusAfterTailing = contracts.StatusCanceled
			} else if err != nil {
				log.Warn().Err(err).Msgf("[%v] Service '%v' container failed", parentStageName, service.Name)
				finalStatusAfterTailing = contracts.StatusFailed
			} else {
				log.Info().Msgf("[%v] Started service '%v' successfully", parentStageName, service.Name)
			}

			pr.tailLogsChannel <- contracts.TailLogLine{
				Step:        service.Name,
				ParentStage: parentStageName,
				Type:        "service",
				Depth:       1,
				Status:      &finalStatusAfterTailing,
			}
		}(ctx, containerID, parentStageName, service)
	}

	// wait for service to be ready if readiness probe is defined
	if service.Readiness != nil {
		log.Info().Msgf("[%v] Starting readiness probe...", parentStage.Name)
		err = pr.dockerRunner.RunReadinessProbeContainer(ctx, parentStage, service, *service.Readiness)
	}
	runDuration := time.Since(dockerRunStart)

	// log tailing - finalize stage
	finalStatus := contracts.StatusRunning
	if pr.canceled {
		log.Info().Msgf("[%v] Canceled service '%v'", parentStageName, service.Name)
		finalStatus = contracts.StatusCanceled
	} else if err != nil {
		log.Warn().Err(err).Msgf("[%v] Service '%v' container failed", parentStageName, service.Name)
		finalStatus = contracts.StatusFailed
	} else {
		log.Info().Msgf("[%v] Started service '%v' successfully", parentStageName, service.Name)
	}

	pr.tailLogsChannel <- contracts.TailLogLine{
		Step:        service.Name,
		ParentStage: parentStageName,
		Type:        "service",
		Depth:       1,
		Duration:    &runDuration,
		Status:      &finalStatus,
	}

	return
}

func (pr *pipelineRunnerImpl) RunStages(ctx context.Context, depth int, stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (buildLogSteps []*contracts.BuildLogStep, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "runStages")
	defer span.Finish()

	// start log tailing
	pr.buildLogSteps = make([]*contracts.BuildLogStep, 0)
	var wg sync.WaitGroup
	wg.Add(1)
	stageExecutionDone := false
	go pr.tailLogs(ctx, &wg, &stageExecutionDone)

	err = pr.dockerRunner.CreateBridgeNetwork(ctx)
	if err != nil {
		return
	}
	defer func(ctx context.Context) {
		_ = pr.dockerRunner.DeleteBridgeNetwork(ctx)
	}(ctx)

	// set default build status at the start
	err = pr.envvarHelper.initBuildStatus()
	if err != nil {
		return
	}

	if len(stages) == 0 {
		return buildLogSteps, fmt.Errorf("Manifest has no stages, failing the build")
	}

	log.Info().Msgf("Running %v stages", len(stages))

	var finalErr error
	for _, p := range stages {

		// handle cancellation happening in between stages
		if pr.canceled {
			return
		}

		var whenEvaluationResult bool
		whenEvaluationResult, err = pr.whenEvaluator.evaluate(p.Name, p.When, pr.whenEvaluator.getParameters())
		if err != nil {
			return buildLogSteps, err
		}

		if whenEvaluationResult {

			err = pr.RunStageWithRetry(ctx, depth, dir, envvars, nil, *p)

			if err != nil {
				// set 'failed' build status
				pr.envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "failed")
				envvars[pr.envvarHelper.getEstafetteEnvvarName("ESTAFETTE_BUILD_STATUS")] = "failed"
				finalErr = err
			}
		} else {

			// if an error has happened in one of the previous steps or the when expression evaluates to false we still want to render the following steps in the result table
			status := contracts.StatusSkipped
			pr.tailLogsChannel <- contracts.TailLogLine{
				Step:         p.Name,
				Type:         "stage",
				Depth:        depth,
				RunIndex:     0,
				AutoInjected: &p.AutoInjected,
				Status:       &status,
			}
		}
	}

	// signal that running stages has finished
	stageExecutionDone = true

	// wait for log tailing to finish
	wg.Wait()

	buildLogSteps = pr.getLogs(ctx)

	return buildLogSteps, finalErr
}

func (pr *pipelineRunnerImpl) RunParallelStages(ctx context.Context, depth int, dir string, envvars map[string]string, parentStage manifest.EstafetteStage, parallelStages []*manifest.EstafetteStage) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunStages")
	defer span.Finish()

	if len(parallelStages) == 0 {
		return fmt.Errorf("Manifest has no stages, failing the build")
	}

	log.Info().Msgf("[%v] Running %v parallel stages", parentStage.Name, len(parallelStages))

	var wg sync.WaitGroup
	wg.Add(len(parallelStages))

	errors := make(chan error, len(parallelStages))

	for _, ps := range parallelStages {
		go func(ctx context.Context, depth int, dir string, envvars map[string]string, parentStage manifest.EstafetteStage, stage manifest.EstafetteStage) {
			defer wg.Done()

			// handle cancellation happening in between stages
			if pr.canceled {
				return
			}

			whenEvaluationResult, err := pr.whenEvaluator.evaluate(stage.Name, stage.When, pr.whenEvaluator.getParameters())
			if err != nil {
				errors <- err
				return
			}

			if whenEvaluationResult {

				err = pr.RunStageWithRetry(ctx, depth, dir, envvars, &parentStage, stage)

				if err != nil {
					errors <- err
					return
				}

			} else {

				// if an error has happened in one of the previous steps or the when expression evaluates to false we still want to render the following steps in the result table
				status := contracts.StatusSkipped
				pr.tailLogsChannel <- contracts.TailLogLine{
					Step:         stage.Name,
					ParentStage:  parentStage.Name,
					Type:         "stage",
					Depth:        depth,
					RunIndex:     0,
					AutoInjected: &stage.AutoInjected,
					Status:       &status,
				}
			}
		}(ctx, depth, dir, envvars, parentStage, *ps)
	}

	// TODO as soon as one parallel stage fails cancel the others

	wg.Wait()

	close(errors)
	for e := range errors {
		err = e
		return
	}

	return
}

func (pr *pipelineRunnerImpl) RunServices(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, services []*manifest.EstafetteService) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunServices")
	defer span.Finish()

	var wg sync.WaitGroup
	wg.Add(len(services))

	errors := make(chan error, len(services))

	for _, s := range services {
		go func(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, service manifest.EstafetteService) {
			defer wg.Done()

			log.Info().Msgf("Starting service '%v' for stage '%v'...", s.ContainerImage, parentStage.Name)

			// set some defaults here, until they get passed in from the api
			if s.When == "" {
				s.When = "status == 'succeeded'"
			}
			if s.Shell == "" {
				s.Shell = "/bin/sh"
			}

			whenEvaluationResult, err := pr.whenEvaluator.evaluate(s.Name, s.When, pr.whenEvaluator.getParameters())
			if err != nil {
				errors <- err
				return
			}

			if whenEvaluationResult {
				err := pr.RunService(ctx, envvars, parentStage, service)
				if err != nil {

					// create log line for error
					logLineObject := contracts.BuildLogLine{
						Timestamp:  time.Now().UTC(),
						StreamType: "stderr",
						Text:       err.Error(),
					}
					pr.tailLogsChannel <- contracts.TailLogLine{
						Step:        s.Name,
						ParentStage: parentStage.Name,
						Type:        "service",
						Depth:       1,
						LogLine:     &logLineObject,
					}

					errors <- err
				}
			} else {
				// TODO send taillogline for skipped
			}
		}(ctx, envvars, parentStage, *s)
	}

	// wait for readiness for all services
	wg.Wait()

	close(errors)
	for e := range errors {
		err = e
		return
	}

	return
}

func (pr *pipelineRunnerImpl) StopPipelineOnCancellation() {
	// wait for cancellation
	<-pr.cancellationChannel

	pr.canceled = true
}

func (pr *pipelineRunnerImpl) resetCancellation() {
	pr.canceled = false
}

func (pr *pipelineRunnerImpl) sendStatusMessage(stage manifest.EstafetteStage, parentStageName string, depth int, runIndex int, image *contracts.BuildLogStepDockerImage, runDuration *time.Duration, status string) {

	tailLogLine := contracts.TailLogLine{
		Step:         stage.Name,
		ParentStage:  parentStageName,
		Type:         "stage",
		Depth:        depth,
		RunIndex:     runIndex,
		AutoInjected: &stage.AutoInjected,
		Status:       &status,
	}

	if image != nil {
		tailLogLine.Image = image
	}
	if runDuration != nil {
		tailLogLine.Duration = runDuration
	}

	pr.tailLogsChannel <- tailLogLine
}

func (pr *pipelineRunnerImpl) tailLogs(ctx context.Context, wg *sync.WaitGroup, stageExecutionDone *bool) {
	defer wg.Done()
	for {
		select {
		case tailLogLine := <-pr.tailLogsChannel:
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

		case <-time.After(1 * time.Second):
			if *stageExecutionDone {
				return
			}
		}
	}
}

func (pr *pipelineRunnerImpl) getLogs(ctx context.Context) []*contracts.BuildLogStep {
	return pr.buildLogSteps
}

func (pr *pipelineRunnerImpl) upsertTailLogLine(tailLogLine contracts.TailLogLine) {

	// check if tailLogLine.Step (in combination with parentstage and type if applicable) already exists in pr.buildLogSteps
	mainStage := pr.getMainBuildLogStep(tailLogLine)
	if mainStage == nil {
		if tailLogLine.ParentStage != "" {
			mainStage = &contracts.BuildLogStep{
				Step:     tailLogLine.ParentStage,
				Depth:    tailLogLine.Depth,
				RunIndex: tailLogLine.RunIndex,
			}
			pr.buildLogSteps = append(pr.buildLogSteps, mainStage)
		} else {
			mainStage = &contracts.BuildLogStep{
				Step:     tailLogLine.Step,
				Depth:    tailLogLine.Depth,
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
					Step:     tailLogLine.Step,
					Depth:    tailLogLine.Depth,
					RunIndex: tailLogLine.RunIndex,
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
					Step:     tailLogLine.Step,
					Depth:    tailLogLine.Depth,
					RunIndex: tailLogLine.RunIndex,
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
