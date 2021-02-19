package pipeline

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/estafette/estafette-ci-builder/api"
	"github.com/estafette/estafette-ci-builder/clients/docker"
	"github.com/estafette/estafette-ci-builder/clients/envvar"
	"github.com/estafette/estafette-ci-builder/services/evaluation"
	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
)

// Service is the interface for running the pipeline steps
//go:generate mockgen -package=pipeline -destination ./mock.go -source=service.go
type Service interface {
	RunStage(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error)
	RunStageWithRetry(ctx context.Context, depth int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error)
	RunService(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, service manifest.EstafetteService) (err error)
	RunStages(ctx context.Context, depth int, stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (buildLogSteps []*contracts.BuildLogStep, err error)
	RunParallelStages(ctx context.Context, depth int, dir string, envvars map[string]string, parentStage manifest.EstafetteStage, parallelStages []*manifest.EstafetteStage) (err error)
	RunServices(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, services []*manifest.EstafetteService) (err error)
	StopPipelineOnCancellation()
	EnableBuilderInfoStageInjection()
}

// NewService returns a new pipeline.Service
func NewService(ctx context.Context, envvarClient envvar.Client, evaluationService evaluation.Service, dockerClient docker.Client, runAsJob bool, cancellationChannel chan struct{}, tailLogsChannel chan contracts.TailLogLine, applicationInfo foundation.ApplicationInfo) (Service, error) {
	return &service{
		envvarClient:        envvarClient,
		evaluationService:   evaluationService,
		dockerClient:        dockerClient,
		runAsJob:            runAsJob,
		cancellationChannel: cancellationChannel,
		tailLogsChannel:     tailLogsChannel,
		buildLogSteps:       make([]*contracts.BuildLogStep, 0),
		applicationInfo:     applicationInfo,
	}, nil
}

type service struct {
	envvarClient           envvar.Client
	evaluationService      evaluation.Service
	dockerClient           docker.Client
	runAsJob               bool
	cancellationChannel    chan struct{}
	tailLogsChannel        chan contracts.TailLogLine
	buildLogSteps          []*contracts.BuildLogStep
	canceled               bool
	injectBuilderInfoStage bool
	applicationInfo        foundation.ApplicationInfo
}

func (s *service) RunStage(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunStage")
	defer span.Finish()
	span.SetTag("stage", stage.Name)

	// init some variables
	parentStageName, stagePlaceholder, autoInjected := s.initStageVariables(ctx, depth, runIndex, dir, envvars, parentStage, stage)
	stage.ContainerImage = os.Expand(stage.ContainerImage, s.envvarClient.GetEstafetteEnv)

	log.Info().Msgf("%v Starting stage", stagePlaceholder)

	// pull image, get size and send pending/running status messages
	err = s.pullImageIfNeeded(ctx, stage.Name, parentStageName, stage.ContainerImage, contracts.LogTypeStage, depth, runIndex, autoInjected)
	defer s.handleStageFinish(ctx, depth, runIndex, dir, envvars, parentStage, stage, time.Now(), &err)
	if s.canceled || err != nil {
		return
	}

	if len(stage.Services) > 0 {
		// this stage has service containers, start them first
		err = s.RunServices(ctx, envvars, stage, stage.Services)
		if s.canceled || err != nil {
			return
		}
	}

	if len(stage.ParallelStages) > 0 {
		if depth == 0 {
			err = s.RunParallelStages(ctx, depth+1, dir, envvars, stage, stage.ParallelStages)
			if s.canceled || err != nil {
				return
			}
		} else {
			log.Warn().Msgf("%v Can't run parallel stages nested inside nested stages", stagePlaceholder)
		}
	} else if stage.ContainerImage != "" {
		var containerID string
		containerID, err = s.dockerClient.StartStageContainer(ctx, depth, runIndex, dir, envvars, stage)
		if s.canceled || err != nil {
			return
		}

		err = s.dockerClient.TailContainerLogs(ctx, containerID, parentStageName, stage.Name, contracts.LogTypeStage, depth, runIndex, nil)
		if s.canceled || err != nil {
			return
		}
	}

	return
}

func (s *service) initStageVariables(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (parentStageName string, stagePlaceholder string, autoInjected *bool) {

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

func (s *service) handleStageFinish(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage, dockerRunStart time.Time, errPointer *error) {

	err := *errPointer

	// init some variables
	parentStageName, stagePlaceholder, autoInjected := s.initStageVariables(ctx, depth, runIndex, dir, envvars, parentStage, stage)

	// finalize stage
	finalStatus := contracts.LogStatusSucceeded
	if s.canceled {
		log.Info().Msgf("%v Stage canceled", stagePlaceholder)
		finalStatus = contracts.LogStatusCanceled
	} else if err != nil {
		log.Warn().Err(err).Msgf("%v Stage failed", stagePlaceholder)
		finalStatus = contracts.LogStatusFailed
	} else {
		log.Info().Msgf("%v Stage succeeded", stagePlaceholder)
	}

	if len(stage.Services) > 0 {
		// this stage has service containers, stop them now that the stage has finished
		s.dockerClient.StopSingleStageServiceContainers(ctx, stage)
	}

	runDurationValue := time.Since(dockerRunStart)
	runDuration := &runDurationValue

	s.sendStatusMessage(stage.Name, parentStageName, contracts.LogTypeStage, depth, runIndex, autoInjected, nil, runDuration, finalStatus)
}

func (s *service) RunStageWithRetry(ctx context.Context, depth int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, stage manifest.EstafetteStage) (err error) {

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
		err = s.RunStage(ctx, depth, runIndex, dir, envvars, parentStage, stage)

		// if execution is successful, we're done
		if s.canceled || err == nil {
			return
		}

		// create log line for error
		logLineObject := contracts.BuildLogLine{
			LineNumber: 10000,
			Timestamp:  time.Now().UTC(),
			StreamType: "stderr",
			Text:       err.Error(),
		}

		s.tailLogsChannel <- contracts.TailLogLine{
			Step:        stage.Name,
			ParentStage: parentStageName,
			Type:        contracts.LogTypeStage,
			Depth:       depth,
			RunIndex:    runIndex,
			LogLine:     &logLineObject,
		}

		runIndex++
	}

	if s.canceled || err != nil {
		return
	}

	return
}

func (s *service) RunService(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, service manifest.EstafetteService) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunService")
	defer span.Finish()
	span.SetTag("service", service.Name)

	// init some variables
	service.ContainerImage = os.Expand(service.ContainerImage, s.envvarClient.GetEstafetteEnv)
	depth := 1
	runIndex := 0

	log.Info().Msgf("[%v] [%v] Starting service", parentStage.Name, service.Name)

	// pull image, get size and send pending/running status messages
	err = s.pullImageIfNeeded(ctx, service.Name, parentStage.Name, service.ContainerImage, contracts.LogTypeService, depth, runIndex, nil)
	dockerRunStart := time.Now()
	defer s.handleServiceFinish(ctx, envvars, parentStage, service, true, dockerRunStart, &err)
	if s.canceled || err != nil {
		return
	}

	var containerID string
	containerID, err = s.dockerClient.StartServiceContainer(ctx, envvars, service)
	if s.canceled || err != nil {
		return
	}

	// start log tailing in background
	go func(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, service manifest.EstafetteService, containerID string) {
		var err error
		defer s.handleServiceFinish(ctx, envvars, parentStage, service, false, dockerRunStart, &err)
		err = s.dockerClient.TailContainerLogs(ctx, containerID, parentStage.Name, service.Name, contracts.LogTypeService, 1, 0, service.MultiStage)
	}(ctx, envvars, parentStage, service, containerID)

	// wait for service to be ready if readiness probe is defined
	if service.Readiness != nil {
		log.Info().Msgf("[%v] Starting readiness probe...", parentStage.Name)
		err = s.dockerClient.RunReadinessProbeContainer(ctx, parentStage, service, *service.Readiness)
		if s.canceled || err != nil {
			return
		}
	}

	return
}

func (s *service) handleServiceFinish(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, service manifest.EstafetteService, skipSucceeded bool, dockerRunStart time.Time, errPointer *error) {

	err := *errPointer

	// finalize stage
	finalStatus := contracts.LogStatusSucceeded
	if s.canceled {
		log.Info().Msgf("[%v] [%v] Service canceled", parentStage.Name, service.Name)
		finalStatus = contracts.LogStatusCanceled
	} else if err != nil {
		log.Warn().Err(err).Msgf("[%v] [%v] Service failed", parentStage.Name, service.Name)
		finalStatus = contracts.LogStatusFailed
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

	s.sendStatusMessage(service.Name, parentStage.Name, contracts.LogTypeService, 1, 0, nil, nil, runDuration, finalStatus)
}

func (s *service) RunStages(ctx context.Context, depth int, stages []*manifest.EstafetteStage, dir string, envvars map[string]string) (buildLogSteps []*contracts.BuildLogStep, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunStages")
	defer span.Finish()

	// start log tailing
	s.buildLogSteps = make([]*contracts.BuildLogStep, 0)
	tailLogsDone := make(chan struct{}, 1)
	go s.tailLogs(ctx, tailLogsDone, stages)

	err = s.dockerClient.CreateNetworks(ctx)
	if err != nil {
		return
	}
	defer func(ctx context.Context) {
		_ = s.dockerClient.DeleteNetworks(ctx)
	}(ctx)

	// set default build status at the start
	err = s.envvarClient.InitBuildStatus()
	if err != nil {
		return
	}

	if len(stages) == 0 {
		return buildLogSteps, fmt.Errorf("Manifest has no stages, failing the build")
	}

	// creates first injected stage with builder info
	if s.injectBuilderInfoStage {
		s.logBuilderInfo(s.applicationInfo)
	}

	log.Info().Msgf("Running %v stages", len(stages))

	var finalErr error
	for _, st := range stages {
		func(stage *manifest.EstafetteStage) {
			defer func(stage *manifest.EstafetteStage) {
				// handle cancellation happening in between stages
				if s.canceled {
					// set canceled status for all the next stages
					s.forceStatusForStage(*stage, contracts.LogStatusCanceled)
				}
			}(stage)

			var whenEvaluationResult bool
			whenEvaluationResult, err = s.evaluationService.Evaluate(stage.Name, stage.When, s.evaluationService.GetParameters())
			if err != nil {
				// set 'failed' build status
				s.envvarClient.SetEstafetteEnv("ESTAFETTE_BUILD_STATUS", "failed")
				envvars[s.envvarClient.GetEstafetteEnvvarName("ESTAFETTE_BUILD_STATUS")] = "failed"
				finalErr = err

				return
			}

			if s.canceled {
				return
			}
			if whenEvaluationResult {
				err = s.RunStageWithRetry(ctx, depth, dir, envvars, nil, *stage)
				if s.canceled {
					return
				}
				if err != nil {
					// set 'failed' build status
					s.envvarClient.SetEstafetteEnv("ESTAFETTE_BUILD_STATUS", "failed")
					envvars[s.envvarClient.GetEstafetteEnvvarName("ESTAFETTE_BUILD_STATUS")] = "failed"
					finalErr = err
				}
			} else {
				// if an error has happened in one of the previous steps or the when expression evaluates to false we still want to render the following steps in the result table
				s.forceStatusForStage(*stage, contracts.LogStatusSkipped)
			}
		}(st)
	}

	s.dockerClient.StopMultiStageServiceContainers(ctx)

	<-tailLogsDone

	return s.getLogs(ctx), finalErr
}

func (s *service) RunParallelStages(ctx context.Context, depth int, dir string, envvars map[string]string, parentStage manifest.EstafetteStage, parallelStages []*manifest.EstafetteStage) (err error) {

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
			if s.canceled {
				return
			}

			whenEvaluationResult, err := s.evaluationService.Evaluate(stage.Name, stage.When, s.evaluationService.GetParameters())
			if s.canceled || err != nil {
				if err != nil {
					errors <- err
				}
				return
			}

			if whenEvaluationResult {

				err = s.RunStageWithRetry(ctx, depth, dir, envvars, &parentStage, stage)

				if s.canceled || err != nil {
					if err != nil {
						errors <- err
					}
					return
				}

			} else {

				// if an error has happened in one of the previous steps or the when expression evaluates to false we still want to render the following steps in the result table
				status := contracts.LogStatusSkipped
				s.tailLogsChannel <- contracts.TailLogLine{
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

func (s *service) RunServices(ctx context.Context, envvars map[string]string, parentStage manifest.EstafetteStage, services []*manifest.EstafetteService) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "RunServices")
	defer span.Finish()

	var wg sync.WaitGroup
	wg.Add(len(services))

	errors := make(chan error, len(services))

	for _, se := range services {
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

			whenEvaluationResult, err := s.evaluationService.Evaluate(service.Name, service.When, s.evaluationService.GetParameters())

			if s.canceled || err != nil {
				if err != nil {
					errors <- err
				}
				return
			}

			if whenEvaluationResult {
				err := s.RunService(ctx, envvars, parentStage, service)
				if s.canceled {
					return
				} else if err != nil {

					// create log line for error
					logLineObject := contracts.BuildLogLine{
						LineNumber: 10000,
						Timestamp:  time.Now().UTC(),
						StreamType: "stderr",
						Text:       err.Error(),
					}
					s.tailLogsChannel <- contracts.TailLogLine{
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
		}(ctx, envvars, parentStage, *se)
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

func (s *service) StopPipelineOnCancellation() {
	// wait for cancellation
	<-s.cancellationChannel

	s.canceled = true

	s.dockerClient.StopAllContainers()
}

func (s *service) EnableBuilderInfoStageInjection() {
	s.injectBuilderInfoStage = true
}

func (s *service) resetCancellation() {
	s.canceled = false
}

func (s *service) pullImageIfNeeded(ctx context.Context, stageName, parentStageName, containerImage string, containerType contracts.LogType, depth int, runIndex int, autoInjected *bool) (err error) {

	var isPulledImage bool
	var isTrustedImage bool
	var hasInjectedCredentials bool
	var imagePullDuration time.Duration
	var imageSize int64
	var buildLogStepDockerImage *contracts.BuildLogStepDockerImage

	if !s.canceled && containerImage != "" {

		isPulledImage = s.dockerClient.IsImagePulled(stageName, containerImage)
		isTrustedImage = s.dockerClient.IsTrustedImage(stageName, containerImage)
		hasInjectedCredentials = s.dockerClient.HasInjectedCredentials(stageName, containerImage)

		buildLogStepDockerImage = &contracts.BuildLogStepDockerImage{
			Name:                   api.GetContainerImageName(containerImage),
			Tag:                    api.GetContainerImageTag(containerImage),
			IsTrusted:              isTrustedImage,
			HasInjectedCredentials: hasInjectedCredentials,
			IsPulled:               isPulledImage,
		}

		if !s.canceled && (!isPulledImage || runtime.GOOS == "windows") {

			// start pulling stage
			s.sendStatusMessage(stageName, parentStageName, containerType, depth, runIndex, autoInjected, buildLogStepDockerImage, nil, contracts.LogStatusPending)

			// pull docker image
			dockerPullStart := time.Now()
			err = s.dockerClient.PullImage(ctx, stageName, containerImage)
			imagePullDuration = time.Since(dockerPullStart)

			// set docker image size
			if !s.canceled && err == nil {
				imageSize, err = s.dockerClient.GetImageSize(containerImage)
			}

			if !s.canceled && err == nil {
				buildLogStepDockerImage = &contracts.BuildLogStepDockerImage{
					Name:                   api.GetContainerImageName(containerImage),
					Tag:                    api.GetContainerImageTag(containerImage),
					IsTrusted:              isTrustedImage,
					HasInjectedCredentials: hasInjectedCredentials,
					IsPulled:               isPulledImage,
					ImageSize:              imageSize,
					PullDuration:           imagePullDuration,
				}
			}
		}
	}

	if !s.canceled && err == nil {
		// start running stage
		s.sendStatusMessage(stageName, parentStageName, containerType, depth, runIndex, autoInjected, buildLogStepDockerImage, nil, contracts.LogStatusRunning)
	}

	return
}

func (s *service) sendStatusMessage(step, parentStageName string, containerType contracts.LogType, depth int, runIndex int, autoInjected *bool, image *contracts.BuildLogStepDockerImage, runDuration *time.Duration, status contracts.LogStatus) {

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

	s.tailLogsChannel <- tailLogLine
}

func (s *service) forceStatusForStage(stage manifest.EstafetteStage, status contracts.LogStatus) {

	var autoInjected *bool
	var image *contracts.BuildLogStepDockerImage
	if stage.ContainerImage != "" {
		isTrustedImage := s.dockerClient.IsTrustedImage(stage.Name, stage.ContainerImage)
		hasInjectedCredentials := s.dockerClient.HasInjectedCredentials(stage.Name, stage.ContainerImage)
		image = &contracts.BuildLogStepDockerImage{
			Name:                   api.GetContainerImageName(stage.ContainerImage),
			Tag:                    api.GetContainerImageTag(stage.ContainerImage),
			IsTrusted:              isTrustedImage,
			HasInjectedCredentials: hasInjectedCredentials,
		}
	}
	if stage.AutoInjected {
		autoInjected = &stage.AutoInjected
	}

	s.sendStatusMessage(stage.Name, "", contracts.LogTypeStage, 0, 0, autoInjected, image, nil, status)

	// loop through all parallel stages and set status
	for _, ps := range stage.ParallelStages {
		var autoInjected *bool
		var image *contracts.BuildLogStepDockerImage
		if ps.ContainerImage != "" {
			isTrustedImage := s.dockerClient.IsTrustedImage(ps.Name, ps.ContainerImage)
			hasInjectedCredentials := s.dockerClient.HasInjectedCredentials(ps.Name, ps.ContainerImage)
			image = &contracts.BuildLogStepDockerImage{
				Name:                   api.GetContainerImageName(ps.ContainerImage),
				Tag:                    api.GetContainerImageTag(ps.ContainerImage),
				IsTrusted:              isTrustedImage,
				HasInjectedCredentials: hasInjectedCredentials,
			}
		}
		if ps.AutoInjected {
			autoInjected = &ps.AutoInjected
		}

		s.sendStatusMessage(ps.Name, stage.Name, contracts.LogTypeStage, 1, 0, autoInjected, image, nil, status)
	}

	// loop through all services and set status
	for _, se := range stage.Services {
		var image *contracts.BuildLogStepDockerImage
		if se.ContainerImage != "" {
			isTrustedImage := s.dockerClient.IsTrustedImage(se.Name, se.ContainerImage)
			hasInjectedCredentials := s.dockerClient.HasInjectedCredentials(se.Name, se.ContainerImage)
			image = &contracts.BuildLogStepDockerImage{
				Name:                   api.GetContainerImageName(se.ContainerImage),
				Tag:                    api.GetContainerImageTag(se.ContainerImage),
				IsTrusted:              isTrustedImage,
				HasInjectedCredentials: hasInjectedCredentials,
			}
		}

		s.sendStatusMessage(se.Name, stage.Name, contracts.LogTypeService, 1, 0, nil, image, nil, status)
	}
}

func (s *service) tailLogs(ctx context.Context, tailLogsDone chan struct{}, stages []*manifest.EstafetteStage) {

	allLogsReceived := make(chan struct{}, 1)

	for {
		select {
		case tailLogLine := <-s.tailLogsChannel:
			if s.runAsJob {
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

			s.upsertTailLogLine(tailLogLine)

			if tailLogLine.Status != nil && s.isFinalStageComplete(stages) {
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

func (s *service) logBuilderInfo(applicationInfo foundation.ApplicationInfo) {

	builderVersionMessage := fmt.Sprintf("Starting \x1b[1m%v\x1b[0m version \x1b[1m%v\x1b[0m... \x1b[36mbranch=\x1b[0m%v \x1b[36mbuildDate=\x1b[0m%v \x1b[36mgoVersion=\x1b[0m%v \x1b[36mos=\x1b[0m%v \x1b[36mrevision=\x1b[0m%v", applicationInfo.App, applicationInfo.Version, applicationInfo.Branch, applicationInfo.BuildDate, applicationInfo.GoVersion(), applicationInfo.OperatingSystem(), applicationInfo.Revision)

	logLineObject := contracts.BuildLogLine{
		LineNumber: 1,
		Timestamp:  time.Now().UTC(),
		StreamType: "stdout",
		Text:       builderVersionMessage,
	}

	status := contracts.LogStatusSucceeded
	trueValue := true
	s.tailLogsChannel <- contracts.TailLogLine{
		Step:         "builder-info",
		Type:         contracts.LogTypeStage,
		LogLine:      &logLineObject,
		Status:       &status,
		AutoInjected: &trueValue,
	}
}

func (s *service) getLogs(ctx context.Context) []*contracts.BuildLogStep {
	return s.buildLogSteps
}

func (s *service) upsertTailLogLine(tailLogLine contracts.TailLogLine) {

	// check if tailLogLine.Step (in combination with parentstage and type if applicable) already exists in s.buildLogSteps
	mainStage := s.getMainBuildLogStep(tailLogLine)
	if mainStage == nil {
		if tailLogLine.ParentStage != "" {
			mainStage = &contracts.BuildLogStep{
				Step:     tailLogLine.ParentStage,
				Depth:    0,
				RunIndex: tailLogLine.RunIndex,
			}
			s.buildLogSteps = append(s.buildLogSteps, mainStage)
		} else {
			mainStage = &contracts.BuildLogStep{
				Step:     tailLogLine.Step,
				Depth:    0,
				RunIndex: tailLogLine.RunIndex,
			}
			s.buildLogSteps = append(s.buildLogSteps, mainStage)
		}
	}

	if tailLogLine.ParentStage != "" {
		if tailLogLine.Type == contracts.LogTypeStage {
			nestedStage := s.getNestedBuildLogStep(tailLogLine)
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
			nestedService := s.getNestedBuildLogService(tailLogLine)
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

func (s *service) getMainBuildLogStep(tailLogLine contracts.TailLogLine) *contracts.BuildLogStep {

	stepToFind := tailLogLine.Step
	if tailLogLine.ParentStage != "" {
		stepToFind = tailLogLine.ParentStage
	}

	for _, bls := range s.buildLogSteps {
		if bls.Step == stepToFind {
			if bls.RunIndex == tailLogLine.RunIndex {
				return bls
			}
		}
	}

	return nil
}

func (s *service) getNestedBuildLogStep(tailLogLine contracts.TailLogLine) *contracts.BuildLogStep {

	if tailLogLine.ParentStage == "" || tailLogLine.Type != contracts.LogTypeStage {
		return nil
	}

	for _, bls := range s.buildLogSteps {
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

func (s *service) getNestedBuildLogService(tailLogLine contracts.TailLogLine) *contracts.BuildLogStep {

	if tailLogLine.ParentStage == "" || tailLogLine.Type != contracts.LogTypeService {
		return nil
	}

	for _, bls := range s.buildLogSteps {
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

func (s *service) isFinalStageComplete(stages []*manifest.EstafetteStage) bool {

	// s.buildLogSteps
	if len(s.buildLogSteps) > 0 && len(stages) > 0 {

		lastBuildLogsStep := s.buildLogSteps[len(s.buildLogSteps)-1]
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
		for _, st := range s.buildLogSteps {
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
