package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/stretchr/testify/assert"
)

var (
	dockerRunnerMock    = &dockerRunnerMockImpl{}
	cancellationChannel = make(chan struct{})
	pipelineRunner      = NewPipelineRunner(envvarHelper, whenEvaluator, dockerRunnerMock, true, cancellationChannel, tailLogsChannel)
)

func TestRunStage(t *testing.T) {

	t.Run("ReturnsErrorWhenPullImageFails", func(t *testing.T) {

		depth := 0
		runIndex := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			return fmt.Errorf("Failed pulling image")
		}

		// act
		err := pipelineRunner.runStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed pulling image", err.Error())
	})

	t.Run("ReturnsErrorWhenGetImageSizeFails", func(t *testing.T) {

		depth := 0
		runIndex := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error { return nil }
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, fmt.Errorf("Failed getting image size")
		}

		// act
		err := pipelineRunner.runStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed getting image size", err.Error())
	})

	t.Run("ReturnsErrorWhenStartStageContainerFails", func(t *testing.T) {

		depth := 0
		runIndex := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return true }
		dockerRunnerMock.startStageContainerFunc = func(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error) {
			return "", fmt.Errorf("Failed starting container")
		}

		// act
		err := pipelineRunner.runStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed starting container", err.Error())
	})

	t.Run("ReturnsErrorWhenTailContainerLogsFails", func(t *testing.T) {

		depth := 0
		runIndex := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return true }
		dockerRunnerMock.startStageContainerFunc = func(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			return fmt.Errorf("Failed tailing container logs")
		}

		// act
		err := pipelineRunner.runStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed tailing container logs", err.Error())
	})

	t.Run("ReturnsNoErrorWhenContainerPullsStartsAndLogs", func(t *testing.T) {

		depth := 0
		runIndex := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		isImagePulledFuncCalled := false
		pullImageFuncCalled := false
		getImageSizeFuncCalled := false
		startStageContainerFuncCalled := false
		tailContainerLogsFuncCalled := false
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { isImagePulledFuncCalled = true; return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			pullImageFuncCalled = true
			return nil
		}
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			getImageSizeFuncCalled = true
			return 0, nil
		}
		dockerRunnerMock.startStageContainerFunc = func(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error) {
			startStageContainerFuncCalled = true
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			tailContainerLogsFuncCalled = true
			return nil
		}

		// act
		err := pipelineRunner.runStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.Nil(t, err)
		assert.True(t, isImagePulledFuncCalled)
		assert.True(t, pullImageFuncCalled)
		assert.True(t, getImageSizeFuncCalled)
		assert.True(t, startStageContainerFuncCalled)
		assert.True(t, tailContainerLogsFuncCalled)
	})

	t.Run("SendsSequenceOfRunningAndSucceededMessageToChannelForSuccessfulRunWhenImageIsAlreadyPulled", func(t *testing.T) {

		depth := 0
		runIndex := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}

		// reset channel to start with no messages
		resetChannel(tailLogsChannel)

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return true }
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, nil
		}
		dockerRunnerMock.startStageContainerFunc = func(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			return nil
		}

		// act
		err := pipelineRunner.runStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.Nil(t, err)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		succeededStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusSucceeded, *succeededStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingRunningAndSucceededMessageToChannelForSuccessfulRun", func(t *testing.T) {

		depth := 0
		runIndex := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}

		// reset channel to start with no messages
		resetChannel(tailLogsChannel)

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			return nil
		}
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, nil
		}
		dockerRunnerMock.startStageContainerFunc = func(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			return nil
		}

		// act
		err := pipelineRunner.runStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.Nil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		succeededStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusSucceeded, *succeededStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingRunningAndFailedMessageToChannelForFailingRun", func(t *testing.T) {

		depth := 0
		runIndex := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}

		// reset channel to start with no messages
		resetChannel(tailLogsChannel)

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			return nil
		}
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, nil
		}
		dockerRunnerMock.startStageContainerFunc = func(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			return fmt.Errorf("Failed tailing container logs")
		}

		// act
		err := pipelineRunner.runStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		failedStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusFailed, *failedStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingRunningAndCanceledMessageToChannelForCanceledRun", func(t *testing.T) {

		depth := 0
		runIndex := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}

		// reset channel to start with no messages
		resetChannel(tailLogsChannel)

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			return nil
		}
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, nil
		}
		dockerRunnerMock.startStageContainerFunc = func(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			return nil
		}

		// act
		go pipelineRunner.stopPipelineOnCancellation()
		cancellationChannel <- struct{}{}
		err := pipelineRunner.runStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.Nil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		canceledStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusCanceled, *canceledStatusMessage.Status)

		pipelineRunner.resetCancellation()
	})

	t.Run("SendsSequenceOfPendingRunningAndCanceledMessageToChannelForCanceledRunEvenWhenRunFails", func(t *testing.T) {

		depth := 0
		runIndex := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}

		// reset channel to start with no messages
		resetChannel(tailLogsChannel)

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			return nil
		}
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, nil
		}
		dockerRunnerMock.startStageContainerFunc = func(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			return fmt.Errorf("Failed tailing container logs")
		}

		// act
		go pipelineRunner.stopPipelineOnCancellation()
		cancellationChannel <- struct{}{}
		err := pipelineRunner.runStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		canceledStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusCanceled, *canceledStatusMessage.Status)

		pipelineRunner.resetCancellation()
	})
}

func TestRunStageWithRetry(t *testing.T) {

	t.Run("ReturnsErrorWhenRunStageFailsWithZeroRetries", func(t *testing.T) {

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
			Retries:        0,
		}

		// set mock responses
		callCount := 0
		dockerRunnerMock.reset()
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			callCount++
			return fmt.Errorf("Failed tailing container logs")
		}

		// act
		err := pipelineRunner.runStageWithRetry(context.Background(), depth, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed tailing container logs", err.Error())
		assert.Equal(t, 1, callCount)
	})

	t.Run("ReturnsErrorWhenRunStageFailsWithAllRetries", func(t *testing.T) {

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
			Retries:        2,
		}

		// set mock responses
		iteration := 0
		callCount := 0
		dockerRunnerMock.reset()
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {

			defer func() { iteration++ }()
			callCount++

			switch iteration {
			case 0:
				return fmt.Errorf("Failed tailing container logs")
			case 1:
				return fmt.Errorf("Failed tailing container logs")
			case 2:
				return fmt.Errorf("Failed tailing container logs")
			}

			return fmt.Errorf("Shouldn't call it this often")
		}

		// act
		err := pipelineRunner.runStageWithRetry(context.Background(), depth, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed tailing container logs", err.Error())
		assert.Equal(t, 3, callCount)
	})

	t.Run("ReturnsErrorWhenRunStageFailsWithAllRetries", func(t *testing.T) {

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
			Retries:        2,
		}

		// set mock responses
		iteration := 0
		callCount := 0
		dockerRunnerMock.reset()
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {

			defer func() { iteration++ }()
			callCount++

			switch iteration {
			case 0:
				return fmt.Errorf("Failed tailing container logs")
			case 1:
				return fmt.Errorf("Failed tailing container logs")
			case 2:
				return nil
			}

			return fmt.Errorf("Shouldn't call it this often")
		}

		// act
		err := pipelineRunner.runStageWithRetry(context.Background(), depth, dir, envvars, parentStage, stage)

		assert.Nil(t, err)
		assert.Equal(t, 3, callCount)
	})
}

func TestRunService(t *testing.T) {

	t.Run("ReturnsErrorWhenPullImageFails", func(t *testing.T) {

		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = &manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			return fmt.Errorf("Failed pulling image")
		}

		// act
		err := pipelineRunner.runService(context.Background(), envvars, parentStage, service)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed pulling image", err.Error())
	})

	t.Run("ReturnsErrorWhenGetImageSizeFails", func(t *testing.T) {

		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = &manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error { return nil }
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, fmt.Errorf("Failed getting image size")
		}

		// act
		err := pipelineRunner.runService(context.Background(), envvars, parentStage, service)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed getting image size", err.Error())
	})

	t.Run("ReturnsErrorWhenStartStageContainerFails", func(t *testing.T) {

		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = &manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return true }
		dockerRunnerMock.startServiceContainerFunc = func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
			return "", fmt.Errorf("Failed starting container")
		}

		// act
		err := pipelineRunner.runService(context.Background(), envvars, parentStage, service)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed starting container", err.Error())
	})

	t.Run("ReturnsNoErrorWhenTailContainerLogsFailsSinceItRunsInTheBackground", func(t *testing.T) {

		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = &manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return true }
		dockerRunnerMock.startServiceContainerFunc = func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			time.Sleep(1 * time.Second)
			return fmt.Errorf("Failed tailing container logs")
		}

		// act
		err := pipelineRunner.runService(context.Background(), envvars, parentStage, service)

		assert.Nil(t, err)
	})

	t.Run("ReturnsErrorWhenRunReadinessProbeContainerFails", func(t *testing.T) {

		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = &manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
			Readiness:      &manifest.ReadinessProbe{},
		}

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return true }
		dockerRunnerMock.startServiceContainerFunc = func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			return nil
		}
		dockerRunnerMock.runReadinessProbeContainerFunc = func(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
			return fmt.Errorf("Failed readiness probe")
		}

		// act
		err := pipelineRunner.runService(context.Background(), envvars, parentStage, service)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed readiness probe", err.Error())
	})

	t.Run("ReturnsNoErrorWhenContainerPullsStartsAndLogs", func(t *testing.T) {

		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = &manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
			Readiness:      &manifest.ReadinessProbe{},
		}

		// set mock responses
		isImagePulledFuncCalled := false
		pullImageFuncCalled := false
		getImageSizeFuncCalled := false
		startServiceContainerFuncCalled := false
		tailContainerLogsFuncCalled := false
		runReadinessProbeContainerFuncCalled := false
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { isImagePulledFuncCalled = true; return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			pullImageFuncCalled = true
			return nil
		}
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			getImageSizeFuncCalled = true
			return 0, nil
		}
		dockerRunnerMock.startServiceContainerFunc = func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
			startServiceContainerFuncCalled = true
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			tailContainerLogsFuncCalled = true
			return nil
		}
		dockerRunnerMock.runReadinessProbeContainerFunc = func(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
			runReadinessProbeContainerFuncCalled = true
			return nil
		}

		// act
		err := pipelineRunner.runService(context.Background(), envvars, parentStage, service)

		// wait for 4 messages to be received, to ensure tailing is done as well
		<-tailLogsChannel
		<-tailLogsChannel
		<-tailLogsChannel
		<-tailLogsChannel

		assert.Nil(t, err)
		assert.True(t, isImagePulledFuncCalled)
		assert.True(t, pullImageFuncCalled)
		assert.True(t, getImageSizeFuncCalled)
		assert.True(t, startServiceContainerFuncCalled)
		assert.True(t, tailContainerLogsFuncCalled)
		assert.True(t, runReadinessProbeContainerFuncCalled)
	})

	t.Run("SendsSequenceOfRunningAndRunningMessageToChannelForSuccessfulRunWhenImageIsAlreadyPulled", func(t *testing.T) {

		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = &manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// reset channel to start with no messages
		resetChannel(tailLogsChannel)

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return true }
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, nil
		}
		dockerRunnerMock.startServiceContainerFunc = func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			time.Sleep(1 * time.Second)
			return nil
		}

		// act
		err := pipelineRunner.runService(context.Background(), envvars, parentStage, service)

		assert.Nil(t, err)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		stillRunningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *stillRunningStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingRunningAndRunningMessageToChannelForSuccessfulStartAndReadiness", func(t *testing.T) {

		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = &manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
			Readiness:      &manifest.ReadinessProbe{},
		}

		// reset channel to start with no messages
		resetChannel(tailLogsChannel)

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			return nil
		}
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, nil
		}
		dockerRunnerMock.startServiceContainerFunc = func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			time.Sleep(1 * time.Second)
			return nil
		}
		dockerRunnerMock.runReadinessProbeContainerFunc = func(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
			return nil
		}

		// act
		err := pipelineRunner.runService(context.Background(), envvars, parentStage, service)

		assert.Nil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		stillRunningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *stillRunningStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingRunningAndFailedMessageToChannelForFailingReadiness", func(t *testing.T) {

		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = &manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
			Readiness:      &manifest.ReadinessProbe{},
		}

		// reset channel to start with no messages
		resetChannel(tailLogsChannel)

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			return nil
		}
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, nil
		}
		dockerRunnerMock.startServiceContainerFunc = func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			time.Sleep(1 * time.Second)
			return nil
		}
		dockerRunnerMock.runReadinessProbeContainerFunc = func(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
			return fmt.Errorf("Failed readiness probe")
		}

		// act
		err := pipelineRunner.runService(context.Background(), envvars, parentStage, service)

		assert.NotNil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		failedStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusFailed, *failedStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingRunningAndCanceledMessageToChannelForCanceledRun", func(t *testing.T) {

		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = &manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// reset channel to start with no messages
		resetChannel(tailLogsChannel)

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			return nil
		}
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, nil
		}
		dockerRunnerMock.startServiceContainerFunc = func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			time.Sleep(1 * time.Second)
			return nil
		}

		// act
		go pipelineRunner.stopPipelineOnCancellation()
		cancellationChannel <- struct{}{}
		err := pipelineRunner.runService(context.Background(), envvars, parentStage, service)

		assert.Nil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		canceledStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusCanceled, *canceledStatusMessage.Status)

		pipelineRunner.resetCancellation()
	})

	t.Run("SendsSequenceOfPendingRunningAndCanceledMessageToChannelForCanceledRunEvenWhenReadinessFails", func(t *testing.T) {

		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = &manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
			Readiness:      &manifest.ReadinessProbe{},
		}

		// reset channel to start with no messages
		resetChannel(tailLogsChannel)

		// set mock responses
		dockerRunnerMock.reset()
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			return nil
		}
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, nil
		}
		dockerRunnerMock.startServiceContainerFunc = func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			time.Sleep(1 * time.Second)
			return nil
		}
		dockerRunnerMock.runReadinessProbeContainerFunc = func(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
			return fmt.Errorf("Failed readiness probe")
		}

		// act
		go pipelineRunner.stopPipelineOnCancellation()
		cancellationChannel <- struct{}{}
		err := pipelineRunner.runService(context.Background(), envvars, parentStage, service)

		assert.NotNil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		canceledStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusCanceled, *canceledStatusMessage.Status)

		pipelineRunner.resetCancellation()
	})
}

func TestRunStages(t *testing.T) {

	// 	t.Run("ReturnsErrorWhenManifestHasNoStages", func(t *testing.T) {

	// 		envvarHelper.unsetEstafetteEnvvars()
	// 		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	// 		manifest := &mft.EstafetteManifest{}
	// 		envvars := map[string]string{}
	// 		dir, _ := os.Getwd()

	// 		// act
	// 		err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

	// 		assert.NotNil(t, err, "Error: %v", err)
	// 	})

	// 	t.Run("ReturnsResultWithInnerResultForEachStageInManifest", func(t *testing.T) {

	// 		envvarHelper.unsetEstafetteEnvvars()
	// 		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	// 		manifest := &mft.EstafetteManifest{}
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
	// 		envvars := map[string]string{}
	// 		dir, _ := os.Getwd()

	// 		// act
	// 		err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)
	// 		buildLogSteps := pipelineRunner.getLogs(context.Background())

	// 		assert.Nil(t, err)
	// 		assert.Equal(t, 1, len(buildLogSteps))
	// 	})

	// 	t.Run("ReturnsResultWithoutErrorsWhenStagesSucceeded", func(t *testing.T) {

	// 		envvarHelper.unsetEstafetteEnvvars()
	// 		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	// 		manifest := &mft.EstafetteManifest{}
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
	// 		envvars := map[string]string{}
	// 		dir, _ := os.Getwd()

	// 		// act
	// 		err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)
	// 		buildLogSteps := pipelineRunner.getLogs(context.Background())

	// 		assert.Nil(t, err)
	// 		assert.True(t, contracts.HasSucceededStatus(buildLogSteps))
	// 	})

	// 	t.Run("ReturnsResultWithSucceededPipelineResultWhenStagesSucceeded", func(t *testing.T) {

	// 		envvarHelper.unsetEstafetteEnvvars()
	// 		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	// 		manifest := &mft.EstafetteManifest{}
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
	// 		envvars := map[string]string{}
	// 		dir, _ := os.Getwd()

	// 		// act
	// 		err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)
	// 		buildLogSteps := pipelineRunner.getLogs(context.Background())

	// 		assert.Nil(t, err)
	// 		assert.Equal(t, "SUCCEEDED", buildLogSteps[0].Status)
	// 	})

	// 	t.Run("ReturnsResultWithErrorsWhenStagesFailed", func(t *testing.T) {

	// 		envvarHelper.unsetEstafetteEnvvars()
	// 		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	// 		manifest := &mft.EstafetteManifest{}
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
	// 		envvars := map[string]string{}
	// 		dir, _ := os.Getwd()

	// 		// act
	// 		err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)
	// 		buildLogSteps := pipelineRunner.getLogs(context.Background())

	// 		assert.Nil(t, err)
	// 		assert.False(t, contracts.HasSucceededStatus(buildLogSteps))
	// 	})

	// 	t.Run("ReturnsResultWithFailedPipelineResultWhenStagesFailed", func(t *testing.T) {

	// 		envvarHelper.unsetEstafetteEnvvars()
	// 		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	// 		manifest := &mft.EstafetteManifest{}
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
	// 		envvars := map[string]string{}
	// 		dir, _ := os.Getwd()

	// 		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
	// 			return fmt.Errorf("Failed excution")
	// 		}

	// 		// act
	// 		err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)
	// 		buildLogSteps := pipelineRunner.getLogs(context.Background())

	// 		assert.Nil(t, err)
	// 		assert.Equal(t, "FAILED", buildLogSteps[0].Status)
	// 	})

	// 	t.Run("ReturnsResultWithoutErrorsWhenStagesSkipped", func(t *testing.T) {

	// 		envvarHelper.unsetEstafetteEnvvars()
	// 		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	// 		manifest := &mft.EstafetteManifest{}
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
	// 		envvars := map[string]string{}
	// 		dir, _ := os.Getwd()

	// 		// act
	// 		err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)
	// 		buildLogSteps := pipelineRunner.getLogs(context.Background())

	// 		assert.Nil(t, err)
	// 		assert.True(t, contracts.HasSucceededStatus(buildLogSteps))
	// 	})

	// 	t.Run("ReturnsResultWithSkippedStageResultWhenStagesSkipped", func(t *testing.T) {

	// 		envvarHelper.unsetEstafetteEnvvars()
	// 		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	// 		manifest := &mft.EstafetteManifest{}
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
	// 		envvars := map[string]string{}
	// 		dir, _ := os.Getwd()

	// 		// act
	// 		err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)
	// 		buildLogSteps := pipelineRunner.getLogs(context.Background())

	// 		assert.Nil(t, err)
	// 		assert.Equal(t, "SKIPPED", buildLogSteps[0].Status)
	// 	})

	// 	t.Run("ReturnsResultForAllStagesWhenFirstStageFails", func(t *testing.T) {

	// 		envvarHelper.unsetEstafetteEnvvars()
	// 		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	// 		manifest := &mft.EstafetteManifest{}
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep2", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep3", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
	// 		envvars := map[string]string{}
	// 		dir, _ := os.Getwd()

	// 		// act
	// 		err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)
	// 		buildLogSteps := pipelineRunner.getLogs(context.Background())

	// 		assert.Nil(t, err)
	// 		assert.Equal(t, "FAILED", buildLogSteps[0].Status)
	// 		assert.Equal(t, "SKIPPED", buildLogSteps[1].Status)
	// 		assert.Equal(t, "SUCCEEDED", buildLogSteps[2].Status)
	// 	})

	// 	t.Run("ReturnsResultWithErrorsWhenFirstStageFailsAndSecondSucceeds", func(t *testing.T) {

	// 		envvarHelper.unsetEstafetteEnvvars()
	// 		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	// 		manifest := &mft.EstafetteManifest{}
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep2", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep3", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
	// 		envvars := map[string]string{}
	// 		dir, _ := os.Getwd()

	// 		// act
	// 		err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)
	// 		buildLogSteps := pipelineRunner.getLogs(context.Background())

	// 		assert.Nil(t, err)
	// 		assert.False(t, contracts.HasSucceededStatus(buildLogSteps))
	// 	})

	// 	t.Run("ReturnsResultWithoutErrorsWhenStagesSucceededAfterRetrial", func(t *testing.T) {

	// 		envvarHelper.unsetEstafetteEnvvars()
	// 		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	// 		cmd := "if [ -f retried ]; then rm retried && exit 0; else touch retried && exit 1; fi;"
	// 		manifest := &mft.EstafetteManifest{}
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestRetryStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Retries: 1, Commands: []string{cmd}, When: "status == 'succeeded'"})
	// 		envvars := map[string]string{}
	// 		dir, _ := os.Getwd()

	// 		// act
	// 		err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)
	// 		buildLogSteps := pipelineRunner.getLogs(context.Background())

	// 		assert.Nil(t, err)
	// 		assert.True(t, contracts.HasSucceededStatus(buildLogSteps))
	// 	})

	// 	t.Run("ReturnsResultWithErrorsWhenStagesFailedAfterRetrial", func(t *testing.T) {

	// 		envvarHelper.unsetEstafetteEnvvars()
	// 		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	// 		cmd := "exit 1"
	// 		manifest := &mft.EstafetteManifest{}
	// 		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestRetryStep2", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Retries: 1, Commands: []string{cmd}, When: "status == 'succeeded'"})
	// 		envvars := map[string]string{}
	// 		dir, _ := os.Getwd()

	// 		// act
	// 		err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)
	// 		buildLogSteps := pipelineRunner.getLogs(context.Background())

	// 		assert.Nil(t, err)
	// 		assert.False(t, contracts.HasSucceededStatus(buildLogSteps))
	// 	})
	// }

	// func TestGetMainBuildLogStep(t *testing.T) {

	// 	t.Run("ReturnsNilIfBuildLogsStepsIsEmpty", func(t *testing.T) {

	// 		pipelineRunner := pipelineRunnerImpl{
	// 			buildLogSteps: make([]*contracts.BuildLogStep, 0),
	// 		}
	// 		tailLogLine := contracts.TailLogLine{
	// 			Step: "stage-a",
	// 		}

	// 		// act
	// 		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

	// 		assert.Nil(t, buildLogStep)
	// 	})

	// 	t.Run("ReturnsNilIfStageDoesNotExistInBuildLogsSteps", func(t *testing.T) {

	// 		pipelineRunner := pipelineRunnerImpl{
	// 			buildLogSteps: []*contracts.BuildLogStep{
	// 				&contracts.BuildLogStep{
	// 					Step: "stage-b",
	// 				},
	// 			},
	// 		}
	// 		tailLogLine := contracts.TailLogLine{
	// 			Step: "stage-a",
	// 		}

	// 		// act
	// 		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

	// 		assert.Nil(t, buildLogStep)
	// 	})

	// 	t.Run("ReturnsBuildLogsStepForStageIfExistsInBuildLogsSteps", func(t *testing.T) {

	// 		pipelineRunner := pipelineRunnerImpl{
	// 			buildLogSteps: []*contracts.BuildLogStep{
	// 				&contracts.BuildLogStep{
	// 					Step: "stage-a",
	// 				},
	// 			},
	// 		}
	// 		tailLogLine := contracts.TailLogLine{
	// 			Step: "stage-a",
	// 		}

	// 		// act
	// 		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

	// 		assert.NotNil(t, buildLogStep)
	// 		assert.Equal(t, "stage-a", buildLogStep.Step)
	// 	})

	// 	t.Run("ReturnsNilIfStageDoesNotExistInBuildLogsStepsWithRunIndex", func(t *testing.T) {

	// 		pipelineRunner := pipelineRunnerImpl{
	// 			buildLogSteps: []*contracts.BuildLogStep{
	// 				&contracts.BuildLogStep{
	// 					Step:     "stage-a",
	// 					RunIndex: 0,
	// 				},
	// 			},
	// 		}
	// 		tailLogLine := contracts.TailLogLine{
	// 			Step:     "stage-a",
	// 			RunIndex: 1,
	// 		}

	// 		// act
	// 		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

	// 		assert.Nil(t, buildLogStep)
	// 	})

	// 	t.Run("ReturnsBuildLogsStepForStageIfExistsInBuildLogsStepsWithRunIndex", func(t *testing.T) {

	// 		pipelineRunner := pipelineRunnerImpl{
	// 			buildLogSteps: []*contracts.BuildLogStep{
	// 				&contracts.BuildLogStep{
	// 					Step:     "stage-a",
	// 					RunIndex: 1,
	// 				},
	// 			},
	// 		}
	// 		tailLogLine := contracts.TailLogLine{
	// 			Step:     "stage-a",
	// 			RunIndex: 1,
	// 		}

	// 		// act
	// 		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

	// 		assert.NotNil(t, buildLogStep)
	// 		assert.Equal(t, "stage-a", buildLogStep.Step)
	// 	})

	// 	t.Run("ReturnsNilIfParentStageDoesNotExistInBuildLogsSteps", func(t *testing.T) {

	// 		pipelineRunner := pipelineRunnerImpl{
	// 			buildLogSteps: []*contracts.BuildLogStep{
	// 				&contracts.BuildLogStep{
	// 					Step: "stage-a",
	// 				},
	// 			},
	// 		}
	// 		tailLogLine := contracts.TailLogLine{
	// 			Step:        "nested-stage-0",
	// 			ParentStage: "stage-b",
	// 		}

	// 		// act
	// 		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

	// 		assert.Nil(t, buildLogStep)
	// 	})

	// 	t.Run("ReturnsBuildLogsStepForStageIfParentStageExistsInBuildLogsSteps", func(t *testing.T) {

	// 		pipelineRunner := pipelineRunnerImpl{
	// 			buildLogSteps: []*contracts.BuildLogStep{
	// 				&contracts.BuildLogStep{
	// 					Step: "stage-a",
	// 				},
	// 			},
	// 		}
	// 		tailLogLine := contracts.TailLogLine{
	// 			Step:        "nested-stage-0",
	// 			ParentStage: "stage-a",
	// 		}

	// 		// act
	// 		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

	// 		assert.NotNil(t, buildLogStep)
	// 		assert.Equal(t, "stage-a", buildLogStep.Step)
	// 	})
	// }

	// func TestGetNestedBuildLogStep(t *testing.T) {

	// 	t.Run("ReturnsNilIfBuildLogsStepsIsEmpty", func(t *testing.T) {

	// 		pipelineRunner := pipelineRunnerImpl{
	// 			buildLogSteps: make([]*contracts.BuildLogStep, 0),
	// 		}
	// 		tailLogLine := contracts.TailLogLine{
	// 			Step:        "nested-stage-0",
	// 			ParentStage: "stage-a",
	// 			Depth:       1,
	// 			Type:        "stage",
	// 		}

	// 		// act
	// 		buildLogStep := pipelineRunner.getNestedBuildLogStep(tailLogLine)

	// 		assert.Nil(t, buildLogStep)
	// 	})

	// 	t.Run("ReturnsNilIfDepthIsZero", func(t *testing.T) {

	// 		pipelineRunner := pipelineRunnerImpl{
	// 			buildLogSteps: []*contracts.BuildLogStep{
	// 				&contracts.BuildLogStep{
	// 					Step: "stage-a",
	// 				},
	// 			},
	// 		}
	// 		tailLogLine := contracts.TailLogLine{
	// 			Step:        "nested-stage-0",
	// 			ParentStage: "stage-a",
	// 			Depth:       0,
	// 			Type:        "stage",
	// 		}

	// 		// act
	// 		buildLogStep := pipelineRunner.getNestedBuildLogStep(tailLogLine)

	// 		assert.Nil(t, buildLogStep)
	// 	})

	// 	t.Run("ReturnsNilIfParentStageExistsButNestedStageDoesNot", func(t *testing.T) {

	// 		pipelineRunner := pipelineRunnerImpl{
	// 			buildLogSteps: []*contracts.BuildLogStep{
	// 				&contracts.BuildLogStep{
	// 					Step: "stage-a",
	// 					NestedSteps: []*contracts.BuildLogStep{
	// 						&contracts.BuildLogStep{
	// 							Step: "nested-stage-1",
	// 						},
	// 					},
	// 				},
	// 			},
	// 		}
	// 		tailLogLine := contracts.TailLogLine{
	// 			Step:        "nested-stage-0",
	// 			ParentStage: "stage-a",
	// 			Depth:       1,
	// 			Type:        "stage",
	// 		}

	// 		// act
	// 		buildLogStep := pipelineRunner.getNestedBuildLogStep(tailLogLine)

	// 		assert.Nil(t, buildLogStep)
	// 	})

	// 	t.Run("ReturnsNilIfParentStageExistsButNestedStageDoesNotAndServiceWithSameNameExists", func(t *testing.T) {

	// 		pipelineRunner := pipelineRunnerImpl{
	// 			buildLogSteps: []*contracts.BuildLogStep{
	// 				&contracts.BuildLogStep{
	// 					Step: "stage-a",
	// 					NestedSteps: []*contracts.BuildLogStep{
	// 						&contracts.BuildLogStep{
	// 							Step: "nested-stage-1",
	// 						},
	// 					},
	// 					Services: []*contracts.BuildLogStep{
	// 						&contracts.BuildLogStep{
	// 							Step: "nested-stage-0",
	// 						},
	// 					},
	// 				},
	// 			},
	// 		}
	// 		tailLogLine := contracts.TailLogLine{
	// 			Step:        "nested-stage-0",
	// 			ParentStage: "stage-a",
	// 			Depth:       1,
	// 			Type:        "stage",
	// 		}

	// 		// act
	// 		buildLogStep := pipelineRunner.getNestedBuildLogStep(tailLogLine)

	// 		assert.Nil(t, buildLogStep)
	// 	})

	// 	t.Run("ReturnsNestedStepIfParentStageAndNestedStageExist", func(t *testing.T) {

	// 		pipelineRunner := pipelineRunnerImpl{
	// 			buildLogSteps: []*contracts.BuildLogStep{
	// 				&contracts.BuildLogStep{
	// 					Step: "stage-a",
	// 					NestedSteps: []*contracts.BuildLogStep{
	// 						&contracts.BuildLogStep{
	// 							Step: "nested-stage-0",
	// 						},
	// 					},
	// 				},
	// 			},
	// 		}
	// 		tailLogLine := contracts.TailLogLine{
	// 			Step:        "nested-stage-0",
	// 			ParentStage: "stage-a",
	// 			Depth:       1,
	// 			Type:        "stage",
	// 		}

	// 		// act
	// 		buildLogStep := pipelineRunner.getNestedBuildLogStep(tailLogLine)

	// 		assert.NotNil(t, buildLogStep)
	// 		assert.Equal(t, "nested-stage-0", buildLogStep.Step)
	// 	})
}

func TestGetNestedBuildLogService(t *testing.T) {

	t.Run("ReturnsNilIfBuildLogsStepsIsEmpty", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: make([]*contracts.BuildLogStep, 0),
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        "service",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogService(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsNilIfDepthIsZero", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Depth:       0,
			Type:        "service",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogService(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsNilIfParentStageExistsButNestedStageDoesNot", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-service-1",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        "service",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogService(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsNilIfParentStageExistsButNestedStageDoesNotAndServiceWithSameNameExists", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-service-0",
						},
					},
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-service-1",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        "service",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogService(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsNestedStepIfParentStageAndNestedStageExist", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-service-0",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        "service",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogService(tailLogLine)

		assert.NotNil(t, buildLogStep)
		assert.Equal(t, "nested-service-0", buildLogStep.Step)
	})
}

func TestUpsertTailLogLine(t *testing.T) {

	t.Run("AddsMainStageIfDoesNotExist", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: make([]*contracts.BuildLogStep, 0),
		}
		tailLogLine := contracts.TailLogLine{
			Step: "stage-a",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
	})

	t.Run("DoesNotReaddMainStageIfAlreadyExists", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step: "stage-a",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
	})

	t.Run("AddsMainStageIfDoesNotExistWithRunIndex", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:     "stage-a",
					RunIndex: 0,
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:     "stage-a",
			RunIndex: 1,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 2, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
		assert.Equal(t, 0, pipelineRunner.buildLogSteps[0].RunIndex)
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[1].Step)
		assert.Equal(t, 1, pipelineRunner.buildLogSteps[1].RunIndex)
	})

	t.Run("AddsMainStageIfDoesNotExistForNestedStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        "stage",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
	})

	t.Run("AddsMainStageIfDoesNotExistForNestedService", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        "service",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
	})

	t.Run("AddsNestedStageIfDoesNotExist", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        "stage",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps[0].NestedSteps))
		assert.Equal(t, "nested-stage-0", pipelineRunner.buildLogSteps[0].NestedSteps[0].Step)
	})

	t.Run("DoesNotReaddNestedStageIfAlreadyExists", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-stage-0",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        "stage",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps[0].NestedSteps))
		assert.Equal(t, "nested-stage-0", pipelineRunner.buildLogSteps[0].NestedSteps[0].Step)
	})

	t.Run("AddsNestedServiceIfDoesNotExist", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Type:        "service",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps[0].Services))
		assert.Equal(t, "nested-service-0", pipelineRunner.buildLogSteps[0].Services[0].Step)
	})

	t.Run("DoesNotReaddNestedServiceIfAlreadyExists", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-service-0",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Type:        "service",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps[0].Services))
		assert.Equal(t, "nested-service-0", pipelineRunner.buildLogSteps[0].Services[0].Step)
	})

	t.Run("AddLogLineToMainStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					LogLines: []contracts.BuildLogLine{
						contracts.BuildLogLine{
							LineNumber: 1,
							Text:       "Hi this is the first line",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step: "stage-a",
			LogLine: &contracts.BuildLogLine{
				LineNumber: 2,
				Text:       "Hey I'd like to add a second line",
			},
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 2, len(pipelineRunner.buildLogSteps[0].LogLines))
		assert.Equal(t, 1, pipelineRunner.buildLogSteps[0].LogLines[0].LineNumber)
		assert.Equal(t, 2, pipelineRunner.buildLogSteps[0].LogLines[1].LineNumber)
	})

	t.Run("AddLogLineToNestedStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-stage-0",
							LogLines: []contracts.BuildLogLine{
								contracts.BuildLogLine{
									LineNumber: 1,
									Text:       "Hi this is the first line",
								},
							},
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        "stage",
			LogLine: &contracts.BuildLogLine{
				LineNumber: 2,
				Text:       "Hey I'd like to add a second line",
			},
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 2, len(pipelineRunner.buildLogSteps[0].NestedSteps[0].LogLines))
		assert.Equal(t, 1, pipelineRunner.buildLogSteps[0].NestedSteps[0].LogLines[0].LineNumber)
		assert.Equal(t, 2, pipelineRunner.buildLogSteps[0].NestedSteps[0].LogLines[1].LineNumber)
	})

	t.Run("AddLogLineToNestedService", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-service-0",
							LogLines: []contracts.BuildLogLine{
								contracts.BuildLogLine{
									LineNumber: 1,
									Text:       "Hi this is the first line",
								},
							},
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Type:        "service",
			LogLine: &contracts.BuildLogLine{
				LineNumber: 2,
				Text:       "Hey I'd like to add a second line",
			},
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 2, len(pipelineRunner.buildLogSteps[0].Services[0].LogLines))
		assert.Equal(t, 1, pipelineRunner.buildLogSteps[0].Services[0].LogLines[0].LineNumber)
		assert.Equal(t, 2, pipelineRunner.buildLogSteps[0].Services[0].LogLines[1].LineNumber)
	})

	t.Run("SetStatusForMainStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "stage-a",
					Status: contracts.StatusPending,
				},
			},
		}
		status := "RUNNING"
		tailLogLine := contracts.TailLogLine{
			Step:   "stage-a",
			Status: &status,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, "RUNNING", pipelineRunner.buildLogSteps[0].Status)
	})

	t.Run("SetStatusForNestedStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step:   "nested-stage-0",
							Status: contracts.StatusPending,
						},
					},
				},
			},
		}
		status := "RUNNING"
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        "stage",
			Status:      &status,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, "RUNNING", pipelineRunner.buildLogSteps[0].NestedSteps[0].Status)
	})

	t.Run("SetStatusForNestedService", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step:   "nested-service-0",
							Status: contracts.StatusPending,
						},
					},
				},
			},
		}
		status := "RUNNING"
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Type:        "service",
			Status:      &status,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, "RUNNING", pipelineRunner.buildLogSteps[0].Services[0].Status)
	})
}
