package builder

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	foundation "github.com/estafette/estafette-foundation"
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
	EnableBuilderInfoStageInjection()
}

type pipelineRunnerImpl struct {
	envvarHelper           EnvvarHelper
	whenEvaluator          WhenEvaluator
	dockerRunner           DockerRunner
	runAsJob               bool
	cancellationChannel    chan struct{}
	tailLogsChannel        chan contracts.TailLogLine
	buildLogSteps          []*contracts.BuildLogStep
	canceled               bool
	injectBuilderInfoStage bool
	applicationInfo        foundation.ApplicationInfo
}

// NewPipelineRunner returns a new PipelineRunner
func NewPipelineRunner(envvarHelper EnvvarHelper, whenEvaluator WhenEvaluator, dockerRunner DockerRunner, runAsJob bool, cancellationChannel chan struct{}, tailLogsChannel chan contracts.TailLogLine, applicationInfo foundation.ApplicationInfo) PipelineRunner {
	return &pipelineRunnerImpl{
		envvarHelper:        envvarHelper,
		whenEvaluator:       whenEvaluator,
		dockerRunner:        dockerRunner,
		runAsJob:            runAsJob,
		cancellationChannel: cancellationChannel,
		tailLogsChannel:     tailLogsChannel,
		buildLogSteps:       make([]*contracts.BuildLogStep, 0),
		applicationInfo:     applicationInfo,
	}
}

func (pr *pipelineRunnerImpl) RunStage(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunStage")
	defer span.Finish()
	span.SetTag("stage", stage.Name)

	// init some variables
	parentStageName, stagePlaceholder, autoInjected := pr.initStageVariables(ctx, depth, runIndex, dir, envvars, parentStage, stage)
	stage.ContainerImage = os.Expand(stage.ContainerImage, pr.envvarHelper.getEstafetteEnv)

	log.Info().Msgf("%v Starting stage", stagePlaceholder)

	// pull image, get size and send pending/running status messages
	err = pr.pullImageIfNeeded(ctx, stage.Name, parentStageName, stage.ContainerImage, contracts.LogTypeStage, depth, runIndex, autoInjected)
	defer pr.handleStageFinish(ctx, depth, runIndex, dir, envvars, parentStage, stage, time.Now(), &err)
	if pr.canceled || err != nil {
		return
	}

	if len(stage.Services) > 0 {
		// this stage has service containers, start them first
		err = pr.RunServices(ctx, envvars, stage, stage.Services)
		if pr.canceled || err != nil {
			return
		}
	}

	if len(stage.ParallelStages) > 0 {
		if depth == 0 {
			err = pr.RunParallelStages(ctx, depth+1, dir, envvars, stage, stage.ParallelStages)
			if pr.canceled || err != nil {
				return
			}
		} else {
			log.Warn().Msgf("%v Can't run parallel stages nested inside nested stages", stagePlaceholder)
		}
	} else if stage.ContainerImage != "" {
		var containerID string
		containerID, err = pr.dockerRunner.StartStageContainer(ctx, depth, runIndex, dir, envvars, stage)
		if pr.canceled || err != nil {
			return
		}

		err = pr.dockerRunner.TailContainerLogs(ctx, containerID, parentStageName, stage.Name, contracts.LogTypeStage, depth, runIndex, nil)
		if pr.canceled || err != nil {
			return
		}
	}

	return
}

func (pr *pipelineRunnerImpl) initStageVariables(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (parentStageName string, stagePlaceholder string, autoInjected *bool) {

	stagePlaceholder = fmt.Sprintf("[%v]", stage.Name)
	if parentStage != nil {
		parentStageName = parentStage.Name
		stagePlaceholder = fmt.Sprintf("[%v] [%v]", parentStageName, stage.Name)
	}
	if stage.AutoInjected {
		autoInjected = &stage.AutoInjected
	}

	return
}

func (pr *pipelineRunnerImpl) handleStageFinish(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage, dockerRunStart time.Time, errPointer *error) {

	err := *errPointer

	// init some variables
	parentStageName, stagePlaceholder, autoInjected := pr.initStageVariables(ctx, depth, runIndex, dir, envvars, parentStage, stage)

	// finalize stage
	finalStatus := contracts.StatusSucceeded
	if pr.canceled {
		log.Info().Msgf("%v Stage canceled", stagePlaceholder)
		finalStatus = contracts.StatusCanceled
	} else if err != nil {
		log.Warn().Err(err).Msgf("%v Stage failed", stagePlaceholder)
		finalStatus = contracts.StatusFailed
	} else {
		log.Info().Msgf("%v Stage succeeded", stagePlaceholder)
	}

	if len(stage.Services) > 0 {
		// this stage has service containers, stop them now that the stage has finished
		pr.dockerRunner.StopSingleStageServiceContainers(ctx, stage)
	}

	runDurationValue := time.Since(dockerRunStart)
	runDuration := &runDurationValue

	pr.sendStatusMessage(stage.Name, parentStageName, contracts.LogTypeStage, depth, runIndex, autoInjected, nil, runDuration, finalStatus.ToLogStatus())
}

func (pr *pipelineRunnerImpl) RunStageWithRetry(ctx context.Context, depth int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunStageWithRetry")
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

		// if execution is successful, we're done
		if pr.canceled || err == nil {
			return
		}

		// create log line for error
		logLineObject := contracts.BuildLogLine{
			LineNumber: 10000,
			Timestamp:  time.Now().UTC(),
			StreamType: "stderr",
			Text:       err.Error(),
		}

		pr.tailLogsChannel <- contracts.TailLogLine{
			Step:        stage.Name,
			ParentStage: parentStageName,
			Type:        contracts.LogTypeStage,
			Depth:       depth,
			RunIndex:    runIndex,
			LogLine:     &logLineObject,
		}

		runIndex++
	}

	if pr.canceled || err != nil {
		return
	}

	return
}

func (pr *pipelineRunnerImpl) RunService(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, service manifest.EstafetteService) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunService")
	defer span.Finish()
	span.SetTag("service", service.Name)

	// init some variables
	service.ContainerImage = os.Expand(service.ContainerImage, pr.envvarHelper.getEstafetteEnv)
	depth := 1
	runIndex := 0

	log.Info().Msgf("[%v] [%v] Starting service", parentStage.Name, service.Name)

	// pull image, get size and send pending/running status messages
	err = pr.pullImageIfNeeded(ctx, service.Name, parentStage.Name, service.ContainerImage, contracts.LogTypeService, depth, runIndex, nil)
	dockerRunStart := time.Now()
	defer pr.handleServiceFinish(ctx, envvars, parentStage, service, true, dockerRunStart, &err)
	if pr.canceled || err != nil {
		return
	}

	var containerID string
	containerID, err = pr.dockerRunner.StartServiceContainer(ctx, envvars, service)
	if pr.canceled || err != nil {
		return
	}

	// start log tailing in background
	go func(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, service manifest.EstafetteService, containerID string) {
		var err error
		defer pr.handleServiceFinish(ctx, envvars, parentStage, service, false, dockerRunStart, &err)
		err = pr.dockerRunner.TailContainerLogs(ctx, containerID, parentStage.Name, service.Name, contracts.LogTypeService, 1, 0, service.MultiStage)
	}(ctx, envvars, parentStage, service, containerID)

	// wait for service to be ready if readiness probe is defined
	if service.Readiness != nil {
		log.Info().Msgf("[%v] Starting readiness probe...", parentStage.Name)
		err = pr.dockerRunner.RunReadinessProbeContainer(ctx, parentStage, service, *service.Readiness)
		if pr.canceled || err != nil {
			return
		}
	}

	return
}

func (pr *pipelineRunnerImpl) handleServiceFinish(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, service manifest.EstafetteService, skipSucceeded bool, dockerRunStart time.Time, errPointer *error) {

	err := *errPointer

	// finalize stage
	finalStatus := contracts.StatusSucceeded
	if pr.canceled {
		log.Info().Msgf("[%v] [%v] Service canceled", parentStage.Name, service.Name)
		finalStatus = contracts.StatusCanceled
	} else if err != nil {
		log.Warn().Err(err).Msgf("[%v] [%v] Service failed", parentStage.Name, service.Name)
		finalStatus = contracts.StatusFailed
	} else {
		if skipSucceeded {
			// no need to set a status, this service container remains in running status until the main stage - see RunStage - has finished
			log.Info().Msgf("[%v] [%v] Service up and running", parentStage.Name, service.Name)
			return
		}
		log.Info().Msgf("[%v] [%v] Service succeeded", parentStage.Name, service.Name)
	}

	runDurationValue := time.Since(dockerRunStart)
	runDuration := &runDurationValue

	pr.sendStatusMessage(service.Name, parentStage.Name, contracts.LogTypeService, 1, 0, nil, nil, runDuration, finalStatus.ToLogStatus())
}

func (pr *pipelineRunnerImpl) RunStages(ctx context.Context, depth int, stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (buildLogSteps []*contracts.BuildLogStep, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunStages")
	defer span.Finish()

	// start log tailing
	pr.buildLogSteps = make([]*contracts.BuildLogStep, 0)
	tailLogsDone := make(chan struct{}, 1)
	go pr.tailLogs(ctx, tailLogsDone, stages)

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

	// creates first injected stage with builder info
	if pr.injectBuilderInfoStage {
		pr.logBuilderInfo(pr.applicationInfo)
	}

	log.Info().Msgf("Running %v stages", len(stages))

	var finalErr error
	for _, s := range stages {
		func(stage *manifest.EstafetteStage) {
			defer func(stage *manifest.EstafetteStage) {
				// handle cancellation happening in between stages
				if pr.canceled {
					// set canceled status for all the next stages
					pr.forceStatusForStage(*stage, contracts.LogStatusCanceled)
				}
			}(stage)

			var whenEvaluationResult bool
			whenEvaluationResult, err = pr.whenEvaluator.Evaluate(stage.Name, stage.When, pr.whenEvaluator.GetParameters())
			if err != nil {
				// set 'failed' build status
				pr.envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "failed")
				envvars[pr.envvarHelper.getEstafetteEnvvarName("ESTAFETTE_BUILD_STATUS")] = "failed"
				finalErr = err

				return
			}

			if pr.canceled {
				return
			}
			if whenEvaluationResult {
				err = pr.RunStageWithRetry(ctx, depth, dir, envvars, nil, *stage)
				if pr.canceled {
					return
				}
				if err != nil {
					// set 'failed' build status
					pr.envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "failed")
					envvars[pr.envvarHelper.getEstafetteEnvvarName("ESTAFETTE_BUILD_STATUS")] = "failed"
					finalErr = err
				}
			} else {
				// if an error has happened in one of the previous steps or the when expression evaluates to false we still want to render the following steps in the result table
				pr.forceStatusForStage(*stage, contracts.LogStatusSkipped)
			}
		}(s)
	}

	pr.dockerRunner.StopMultiStageServiceContainers(ctx)

	<-tailLogsDone

	return pr.getLogs(ctx), finalErr
}

func (pr *pipelineRunnerImpl) RunParallelStages(ctx context.Context, depth int, dir string, envvars map[string]string, parentStage manifest.EstafetteStage, parallelStages []*manifest.EstafetteStage) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunParallelStages")
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

			whenEvaluationResult, err := pr.whenEvaluator.Evaluate(stage.Name, stage.When, pr.whenEvaluator.GetParameters())
			if pr.canceled || err != nil {
				if err != nil {
					errors <- err
				}
				return
			}

			if whenEvaluationResult {

				err = pr.RunStageWithRetry(ctx, depth, dir, envvars, &parentStage, stage)

				if pr.canceled || err != nil {
					if err != nil {
						errors <- err
					}
					return
				}

			} else {

				// if an error has happened in one of the previous steps or the when expression evaluates to false we still want to render the following steps in the result table
				status := contracts.LogStatusSkipped
				pr.tailLogsChannel <- contracts.TailLogLine{
					Step:         stage.Name,
					ParentStage:  parentStage.Name,
					Type:         contracts.LogTypeStage,
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

			log.Info().Msgf("[%v] [%v] Starting service...", parentStage.Name, service.Name)

			// set some defaults here, until they get passed in from the api
			if service.When == "" {
				service.When = "status == 'succeeded'"
			}
			if service.Shell == "" {
				service.Shell = "/bin/sh"
			}

			whenEvaluationResult, err := pr.whenEvaluator.Evaluate(service.Name, service.When, pr.whenEvaluator.GetParameters())

			if pr.canceled || err != nil {
				if err != nil {
					errors <- err
				}
				return
			}

			if whenEvaluationResult {
				err := pr.RunService(ctx, envvars, parentStage, service)
				if pr.canceled {
					return
				} else if err != nil {

					// create log line for error
					logLineObject := contracts.BuildLogLine{
						LineNumber: 10000,
						Timestamp:  time.Now().UTC(),
						StreamType: "stderr",
						Text:       err.Error(),
					}
					pr.tailLogsChannel <- contracts.TailLogLine{
						Step:        service.Name,
						ParentStage: parentStage.Name,
						Type:        contracts.LogTypeService,
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

	pr.dockerRunner.StopAllContainers()
}

func (pr *pipelineRunnerImpl) EnableBuilderInfoStageInjection() {
	pr.injectBuilderInfoStage = true
}

func (pr *pipelineRunnerImpl) resetCancellation() {
	pr.canceled = false
}

func (pr *pipelineRunnerImpl) pullImageIfNeeded(ctx context.Context, stageName, parentStageName, containerImage string, containerType contracts.LogType, depth int, runIndex int, autoInjected *bool) (err error) {

	var isPulledImage bool
	var isTrustedImage bool
	var hasInjectedCredentials bool
	var imagePullDuration time.Duration
	var imageSize int64
	var buildLogStepDockerImage *contracts.BuildLogStepDockerImage

	if !pr.canceled && containerImage != "" {

		isPulledImage = pr.dockerRunner.IsImagePulled(stageName, containerImage)
		isTrustedImage = pr.dockerRunner.IsTrustedImage(stageName, containerImage)
		hasInjectedCredentials = pr.dockerRunner.HasInjectedCredentials(stageName, containerImage)

		buildLogStepDockerImage = &contracts.BuildLogStepDockerImage{
			Name:                   getContainerImageName(containerImage),
			Tag:                    getContainerImageTag(containerImage),
			IsTrusted:              isTrustedImage,
			HasInjectedCredentials: hasInjectedCredentials,
			IsPulled:               isPulledImage,
		}

		if !pr.canceled && (!isPulledImage || runtime.GOOS == "windows") {

			// start pulling stage
			pr.sendStatusMessage(stageName, parentStageName, containerType, depth, runIndex, autoInjected, buildLogStepDockerImage, nil, contracts.LogStatusPending)

			// pull docker image
			dockerPullStart := time.Now()
			err = pr.dockerRunner.PullImage(ctx, stageName, containerImage)
			imagePullDuration = time.Since(dockerPullStart)

			// set docker image size
			if !pr.canceled && err == nil {
				imageSize, err = pr.dockerRunner.GetImageSize(containerImage)
			}

			if !pr.canceled && err == nil {
				buildLogStepDockerImage = &contracts.BuildLogStepDockerImage{
					Name:                   getContainerImageName(containerImage),
					Tag:                    getContainerImageTag(containerImage),
					IsTrusted:              isTrustedImage,
					HasInjectedCredentials: hasInjectedCredentials,
					IsPulled:               isPulledImage,
					ImageSize:              imageSize,
					PullDuration:           imagePullDuration,
				}
			}
		}
	}

	if !pr.canceled && err == nil {
		// start running stage
		pr.sendStatusMessage(stageName, parentStageName, containerType, depth, runIndex, autoInjected, buildLogStepDockerImage, nil, contracts.LogStatusRunning)
	}

	return
}

func (pr *pipelineRunnerImpl) sendStatusMessage(step, parentStageName string, containerType contracts.LogType, depth int, runIndex int, autoInjected *bool, image *contracts.BuildLogStepDockerImage, runDuration *time.Duration, status contracts.LogStatus) {

	tailLogLine := contracts.TailLogLine{
		Step:         step,
		ParentStage:  parentStageName,
		Type:         containerType,
		Depth:        depth,
		RunIndex:     runIndex,
		Image:        image,
		Duration:     runDuration,
		Status:       &status,
		AutoInjected: autoInjected,
	}

	pr.tailLogsChannel <- tailLogLine
}

func (pr *pipelineRunnerImpl) forceStatusForStage(stage manifest.EstafetteStage, status contracts.LogStatus) {

	var autoInjected *bool
	var image *contracts.BuildLogStepDockerImage
	if stage.ContainerImage != "" {
		isTrustedImage := pr.dockerRunner.IsTrustedImage(stage.Name, stage.ContainerImage)
		hasInjectedCredentials := pr.dockerRunner.HasInjectedCredentials(stage.Name, stage.ContainerImage)
		image = &contracts.BuildLogStepDockerImage{
			Name:                   getContainerImageName(stage.ContainerImage),
			Tag:                    getContainerImageTag(stage.ContainerImage),
			IsTrusted:              isTrustedImage,
			HasInjectedCredentials: hasInjectedCredentials,
		}
	}
	if stage.AutoInjected {
		autoInjected = &stage.AutoInjected
	}

	pr.sendStatusMessage(stage.Name, "", contracts.LogTypeStage, 0, 0, autoInjected, image, nil, status)

	// loop through all parallel stages and set status
	for _, ps := range stage.ParallelStages {
		var autoInjected *bool
		var image *contracts.BuildLogStepDockerImage
		if ps.ContainerImage != "" {
			isTrustedImage := pr.dockerRunner.IsTrustedImage(ps.Name, ps.ContainerImage)
			hasInjectedCredentials := pr.dockerRunner.HasInjectedCredentials(ps.Name, ps.ContainerImage)
			image = &contracts.BuildLogStepDockerImage{
				Name:                   getContainerImageName(ps.ContainerImage),
				Tag:                    getContainerImageTag(ps.ContainerImage),
				IsTrusted:              isTrustedImage,
				HasInjectedCredentials: hasInjectedCredentials,
			}
		}
		if ps.AutoInjected {
			autoInjected = &ps.AutoInjected
		}

		pr.sendStatusMessage(ps.Name, stage.Name, contracts.LogTypeStage, 1, 0, autoInjected, image, nil, status)
	}

	// loop through all services and set status
	for _, s := range stage.Services {
		var image *contracts.BuildLogStepDockerImage
		if s.ContainerImage != "" {
			isTrustedImage := pr.dockerRunner.IsTrustedImage(s.Name, s.ContainerImage)
			hasInjectedCredentials := pr.dockerRunner.HasInjectedCredentials(s.Name, s.ContainerImage)
			image = &contracts.BuildLogStepDockerImage{
				Name:                   getContainerImageName(s.ContainerImage),
				Tag:                    getContainerImageTag(s.ContainerImage),
				IsTrusted:              isTrustedImage,
				HasInjectedCredentials: hasInjectedCredentials,
			}
		}

		pr.sendStatusMessage(s.Name, stage.Name, contracts.LogTypeService, 1, 0, nil, image, nil, status)
	}
}

func (pr *pipelineRunnerImpl) tailLogs(ctx context.Context, tailLogsDone chan struct{}, stages []*manifest.EstafetteStage) {

	allLogsReceived := make(chan struct{}, 1)

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

			if tailLogLine.Status != nil && pr.isFinalStageComplete(stages) {
				// signal that running stages have finished so taillogs can stop
				allLogsReceived <- struct{}{}
			}

		case <-allLogsReceived:
			// signal that tailing logs is done
			tailLogsDone <- struct{}{}
			return
		}
	}
}

func (pr *pipelineRunnerImpl) logBuilderInfo(applicationInfo foundation.ApplicationInfo) {

	builderVersionMessage := fmt.Sprintf("Starting \x1b[1m%v\x1b[0m version \x1b[1m%v\x1b[0m... \x1b[36mbranch=\x1b[0m%v \x1b[36mbuildDate=\x1b[0m%v \x1b[36mgoVersion=\x1b[0m%v \x1b[36mos=\x1b[0m%v \x1b[36mrevision=\x1b[0m%v", applicationInfo.App, applicationInfo.Version, applicationInfo.Branch, applicationInfo.BuildDate, applicationInfo.GoVersion(), applicationInfo.OperatingSystem(), applicationInfo.Revision)

	logLineObject := contracts.BuildLogLine{
		LineNumber: 1,
		Timestamp:  time.Now().UTC(),
		StreamType: "stdout",
		Text:       builderVersionMessage,
	}

	status := contracts.LogStatusSucceeded
	trueValue := true
	pr.tailLogsChannel <- contracts.TailLogLine{
		Step:         "builder-info",
		Type:         contracts.LogTypeStage,
		LogLine:      &logLineObject,
		Status:       &status,
		AutoInjected: &trueValue,
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
				Depth:    0,
				RunIndex: tailLogLine.RunIndex,
			}
			pr.buildLogSteps = append(pr.buildLogSteps, mainStage)
		} else {
			mainStage = &contracts.BuildLogStep{
				Step:     tailLogLine.Step,
				Depth:    0,
				RunIndex: tailLogLine.RunIndex,
			}
			pr.buildLogSteps = append(pr.buildLogSteps, mainStage)
		}
	}

	if tailLogLine.ParentStage != "" {
		if tailLogLine.Type == contracts.LogTypeStage {
			nestedStage := pr.getNestedBuildLogStep(tailLogLine)
			if nestedStage == nil {
				nestedStage = &contracts.BuildLogStep{
					Step:     tailLogLine.Step,
					Depth:    1,
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

		} else if tailLogLine.Type == contracts.LogTypeService {
			nestedService := pr.getNestedBuildLogService(tailLogLine)
			if nestedService == nil {
				nestedService = &contracts.BuildLogStep{
					Step:     tailLogLine.Step,
					Depth:    1,
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

	if tailLogLine.ParentStage == "" || tailLogLine.Type != contracts.LogTypeStage {
		return nil
	}

	for _, bls := range pr.buildLogSteps {
		if bls.Step == tailLogLine.ParentStage {
			if tailLogLine.ParentStage != "" {
				// we have to look deeper
				if tailLogLine.Type == contracts.LogTypeStage {
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

	if tailLogLine.ParentStage == "" || tailLogLine.Type != contracts.LogTypeService {
		return nil
	}

	for _, bls := range pr.buildLogSteps {
		if bls.Step == tailLogLine.ParentStage {
			if tailLogLine.ParentStage != "" {
				// we have to look deeper
				if tailLogLine.Type == contracts.LogTypeService {
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

func (pr *pipelineRunnerImpl) isFinalStageComplete(stages []*manifest.EstafetteStage) bool {

	// pr.buildLogSteps
	if len(pr.buildLogSteps) > 0 && len(stages) > 0 {

		lastBuildLogsStep := pr.buildLogSteps[len(pr.buildLogSteps)-1]
		lastStage := stages[len(stages)-1]

		// check if the last tracked step is actually the last stage
		if lastBuildLogsStep.Step != lastStage.Name {
			return false
		}
		if len(lastBuildLogsStep.NestedSteps) != len(lastStage.ParallelStages) {
			return false
		}
		if len(lastBuildLogsStep.Services) != len(lastStage.Services) {
			return false
		}

		// check that all parallel stages are done
		allNestedStepsAreDone := true
		for _, ns := range lastBuildLogsStep.NestedSteps {
			switch ns.Status {
			case contracts.LogStatusSucceeded,
				contracts.LogStatusFailed,
				contracts.LogStatusSkipped,
				contracts.LogStatusCanceled:

			default:
				allNestedStepsAreDone = false
			}
		}
		if !allNestedStepsAreDone {
			return false
		}

		// check that all service containers are done
		allNestedServicesAreDone := true
		for _, st := range pr.buildLogSteps {
			for _, s := range st.Services {
				switch s.Status {
				case contracts.LogStatusSucceeded,
					contracts.LogStatusFailed,
					contracts.LogStatusSkipped,
					contracts.LogStatusCanceled:

				default:
					allNestedServicesAreDone = false
				}
			}
		}
		if !allNestedServicesAreDone {
			return false
		}

		switch lastBuildLogsStep.Status {
		case contracts.LogStatusSucceeded,
			contracts.LogStatusFailed,
			contracts.LogStatusSkipped,
			contracts.LogStatusCanceled:

			return true
		}
	}

	return false
}
