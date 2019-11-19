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
	runStage(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error)
	runStageWithRetry(ctx context.Context, depth int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error)
	runService(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (err error)
	runStages(ctx context.Context, depth int, stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (err error)
	runParallelStages(ctx context.Context, depth int, parentStage *manifest.EstafetteStage, stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (err error)
	runServices(ctx context.Context, parentStage *manifest.EstafetteStage, services []*manifest.EstafetteService, envvars map[string]string) (err error)
	stopPipelineOnCancellation()
	resetCancellation()
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

func (pr *pipelineRunnerImpl) runStage(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "runStage")
	defer span.Finish()
	span.SetTag("stage", stage.Name)

	parentStageName := ""
	if parentStage != nil {
		parentStageName = parentStage.Name
	}

	stage.ContainerImage = os.Expand(stage.ContainerImage, pr.envvarHelper.getEstafetteEnv)

	log.Info().Msgf("[%v] Starting stage '%v'", stage.Name, stage.Name)
	var isPulledImage bool
	var isTrustedImage bool
	var imagePullDuration time.Duration
	var imageSize int64

	if stage.ContainerImage != "" {
		isPulledImage = pr.dockerRunner.isImagePulled(stage.Name, stage.ContainerImage)
		isTrustedImage = pr.dockerRunner.isTrustedImage(stage.Name, stage.ContainerImage)

		if !isPulledImage || runtime.GOOS == "windows" {

			// start pulling stage
			pendingStatus := contracts.StatusPending
			pr.tailLogsChannel <- contracts.TailLogLine{
				Step:        stage.Name,
				ParentStage: parentStageName,
				Type:        "stage",
				Depth:       depth,
				RunIndex:    runIndex,
				Image: &contracts.BuildLogStepDockerImage{
					Name:      getContainerImageName(stage.ContainerImage),
					Tag:       getContainerImageTag(stage.ContainerImage),
					IsTrusted: isTrustedImage,
				},
				AutoInjected: &stage.AutoInjected,
				Status:       &pendingStatus,
			}

			// pull docker image
			dockerPullStart := time.Now()
			err = pr.dockerRunner.pullImage(ctx, stage.Name, stage.ContainerImage)
			if err != nil {
				return err
			}

			imagePullDuration = time.Since(dockerPullStart)
		}

		// set docker image size
		imageSize, err = pr.dockerRunner.getImageSize(stage.ContainerImage)
		if err != nil {
			return err
		}
	}

	// start running stage
	runningStatus := contracts.StatusRunning
	pr.tailLogsChannel <- contracts.TailLogLine{
		Step:        stage.Name,
		ParentStage: parentStageName,
		Type:        "stage",
		Depth:       depth,
		RunIndex:    runIndex,
		Image: &contracts.BuildLogStepDockerImage{
			Name:         getContainerImageName(stage.ContainerImage),
			Tag:          getContainerImageTag(stage.ContainerImage),
			IsTrusted:    isTrustedImage,
			IsPulled:     isPulledImage,
			ImageSize:    imageSize,
			PullDuration: imagePullDuration,
		},
		AutoInjected: &stage.AutoInjected,
		Status:       &runningStatus,
	}

	// run commands in docker container
	dockerRunStart := time.Now()

	parentStageName = ""
	if parentStage != nil {
		parentStageName = parentStage.Name
	}

	if len(stage.Services) > 0 {
		// this stage has service containers, start them first
		err = pr.runServices(ctx, &stage, stage.Services, envvars)
	}

	if err == nil {
		if len(stage.ParallelStages) > 0 {
			if depth == 0 {
				err = pr.runParallelStages(ctx, depth+1, &stage, stage.ParallelStages, dir, envvars)
			} else {
				log.Warn().Msgf("Can't run parallel stages nested inside nested stages")
			}
		} else {
			var containerID string
			containerID, err = pr.dockerRunner.startStageContainer(ctx, depth, runIndex, dir, envvars, parentStage, stage)
			if err == nil {
				err = pr.dockerRunner.tailContainerLogs(ctx, containerID, parentStageName, stage.Name, "stage", depth, runIndex)
			}
		}
	}

	runDuration := time.Since(dockerRunStart)

	// finalize stage
	finalStatus := contracts.StatusSucceeded
	if pr.canceled {
		log.Info().Msgf("[%v] Canceled pipeline '%v'", stage.Name, stage.Name)
		finalStatus = contracts.StatusCanceled
	} else if err != nil {
		log.Warn().Err(err).Msgf("[%v] Pipeline '%v' container failed", stage.Name, stage.Name)
		finalStatus = contracts.StatusFailed
	} else {
		log.Info().Msgf("[%v] Finished pipeline '%v' successfully", stage.Name, stage.Name)
	}
	pr.tailLogsChannel <- contracts.TailLogLine{
		Step:        stage.Name,
		ParentStage: parentStageName,
		Type:        "stage",
		Depth:       depth,
		RunIndex:    runIndex,
		Duration:    &runDuration,
		Status:      &finalStatus,
	}

	if len(stage.Services) > 0 {
		// this stage has service containers, stop them now that the stage has finished
		go pr.dockerRunner.stopServiceContainers(ctx, &stage, stage.Services)
	}

	return
}

func (pr *pipelineRunnerImpl) runStageWithRetry(ctx context.Context, depth int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error) {

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
		err = pr.runStage(ctx, depth, runIndex, dir, envvars, nil, stage)

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

func (pr *pipelineRunnerImpl) runService(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "runService")
	defer span.Finish()
	span.SetTag("service", service.Name)

	service.ContainerImage = os.Expand(service.ContainerImage, pr.envvarHelper.getEstafetteEnv)

	parentStageName := ""
	if parentStage != nil {
		parentStageName = parentStage.Name
	}

	log.Info().Msgf("[%v] Starting service container '%v'", parentStageName, service.Name)

	var isPulledImage bool
	var isTrustedImage bool
	var imagePullDuration time.Duration
	var imageSize int64

	if service.ContainerImage != "" {
		isPulledImage = pr.dockerRunner.isImagePulled(service.Name, service.ContainerImage)
		isTrustedImage = pr.dockerRunner.isTrustedImage(service.Name, service.ContainerImage)

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
			err = pr.dockerRunner.pullImage(ctx, service.Name, service.ContainerImage)
			if err != nil {
				return err
			}

			imagePullDuration = time.Since(dockerPullStart)
		}

		// set docker image size
		imageSize, err = pr.dockerRunner.getImageSize(service.ContainerImage)
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

	containerID, err := pr.dockerRunner.startServiceContainer(ctx, envvars, parentStage, service)
	if err == nil {
		go func(ctx context.Context, containerID, parentStageName string, service manifest.EstafetteService) {
			err := pr.dockerRunner.tailContainerLogs(ctx, containerID, parentStageName, service.Name, "service", 1, 0)

			finalStatus := contracts.StatusSucceeded
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
				Status:      &finalStatus,
			}
		}(ctx, containerID, parentStageName, service)
	}

	// wait for service to be ready if readiness probe is defined
	if service.Readiness != nil && parentStage != nil {
		log.Info().Msgf("[%v] Starting readiness probe...", parentStage.Name)
		err = pr.dockerRunner.runReadinessProbeContainer(ctx, *parentStage, service, *service.Readiness)
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

func (pr *pipelineRunnerImpl) runStages(ctx context.Context, depth int, stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunStages")
	defer span.Finish()

	// start log tailing
	pr.buildLogSteps = make([]*contracts.BuildLogStep, 0)
	go pr.tailLogs(ctx)

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
		return fmt.Errorf("Manifest has no stages, failing the build")
	}

	for _, p := range stages {

		// handle cancellation happening in between stages
		if pr.canceled {
			return
		}

		whenEvaluationResult, err := pr.whenEvaluator.evaluate(p.Name, p.When, pr.whenEvaluator.getParameters())
		if err != nil {
			return err
		}

		if whenEvaluationResult {

			err = pr.runStageWithRetry(ctx, depth, dir, envvars, nil, *p)

			if err != nil {
				// set 'failed' build status
				pr.envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "failed")
				envvars[pr.envvarHelper.getEstafetteEnvvarName("ESTAFETTE_BUILD_STATUS")] = "failed"
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

			continue
		}
	}

	return
}

func (pr *pipelineRunnerImpl) runParallelStages(ctx context.Context, depth int, parentStage *manifest.EstafetteStage, stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunStages")
	defer span.Finish()

	// set default build status if not set
	err = pr.envvarHelper.initBuildStatus()
	if err != nil {
		return
	}

	if len(stages) == 0 {
		return fmt.Errorf("Manifest has no stages, failing the build")
	}

	var wg sync.WaitGroup
	wg.Add(len(stages))

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

				err = pr.runStageWithRetry(ctx, depth, dir, envvars, parentStage, *p)

				if err != nil {
					errors <- err
					return
				}

			} else {

				// if an error has happened in one of the previous steps or the when expression evaluates to false we still want to render the following steps in the result table
				status := contracts.StatusSkipped
				pr.tailLogsChannel <- contracts.TailLogLine{
					Step:         p.Name,
					ParentStage:  parentStage.Name,
					Type:         "stage",
					Depth:        depth,
					RunIndex:     0,
					AutoInjected: &p.AutoInjected,
					Status:       &status,
				}
			}
		}(ctx, p, dir, envvars)
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

func (pr *pipelineRunnerImpl) runServices(ctx context.Context, parentStage *manifest.EstafetteStage, services []*manifest.EstafetteService, envvars map[string]string) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunServices")
	defer span.Finish()

	var wg sync.WaitGroup
	wg.Add(len(services))

	errors := make(chan error, len(services))

	for _, s := range services {
		go func(ctx context.Context, parentStage *manifest.EstafetteStage, s *manifest.EstafetteService, envvars map[string]string) {
			defer wg.Done()

			if parentStage != nil {
				log.Info().Msgf("Starting service '%v' for stage '%v'...", s.ContainerImage, parentStage.Name)
			}

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
				err := pr.runService(ctx, envvars, parentStage, *s)
				if err != nil {
					errors <- err
				}
			} else {
				// TODO send taillogline for skipped
			}
		}(ctx, parentStage, s, envvars)
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

func (pr *pipelineRunnerImpl) stopPipelineOnCancellation() {
	// wait for cancellation
	<-pr.cancellationChannel

	pr.canceled = true
}

func (pr *pipelineRunnerImpl) resetCancellation() {
	pr.canceled = false
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
