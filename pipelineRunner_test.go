package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	contracts "github.com/estafette/estafette-ci-contracts"
	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/stretchr/testify/assert"
)

func TestRunStage(t *testing.T) {

	t.Run("ReturnsErrorWhenPullImageFails", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

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
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			return fmt.Errorf("Failed pulling image")
		}

		// act
		err := pipelineRunner.RunStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed pulling image", err.Error())
	})

	t.Run("ReturnsErrorWhenGetImageSizeFails", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

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
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error { return nil }
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, fmt.Errorf("Failed getting image size")
		}

		// act
		err := pipelineRunner.RunStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed getting image size", err.Error())
	})

	t.Run("ReturnsErrorWhenStartStageContainerFails", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

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
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return true }
		dockerRunnerMock.startStageContainerFunc = func(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error) {
			return "", fmt.Errorf("Failed starting container")
		}

		// act
		err := pipelineRunner.RunStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed starting container", err.Error())
	})

	t.Run("ReturnsErrorWhenTailContainerLogsFails", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

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
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return true }
		dockerRunnerMock.startStageContainerFunc = func(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error) {
			return "abc", nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			return fmt.Errorf("Failed tailing container logs")
		}

		// act
		err := pipelineRunner.RunStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed tailing container logs", err.Error())
	})

	t.Run("ReturnsNoErrorWhenContainerPullsStartsAndLogs", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

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
		err := pipelineRunner.RunStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.Nil(t, err)
		assert.True(t, isImagePulledFuncCalled)
		assert.True(t, pullImageFuncCalled)
		assert.True(t, getImageSizeFuncCalled)
		assert.True(t, startStageContainerFuncCalled)
		assert.True(t, tailContainerLogsFuncCalled)
	})

	t.Run("SendsSequenceOfRunningAndSucceededMessageToChannelForSuccessfulRunWhenImageIsAlreadyPulled", func(t *testing.T) {

		dockerRunnerMock, tailLogsChannel, _, pipelineRunner := resetState()

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
		err := pipelineRunner.RunStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.Nil(t, err)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		succeededStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusSucceeded, *succeededStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingRunningAndSucceededMessageToChannelForSuccessfulRun", func(t *testing.T) {

		dockerRunnerMock, tailLogsChannel, _, pipelineRunner := resetState()

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
		err := pipelineRunner.RunStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.Nil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		succeededStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusSucceeded, *succeededStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingRunningAndFailedMessageToChannelForFailingRun", func(t *testing.T) {

		dockerRunnerMock, tailLogsChannel, _, pipelineRunner := resetState()

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
		err := pipelineRunner.RunStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		failedStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusFailed, *failedStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingRunningAndCanceledMessageToChannelForCanceledRun", func(t *testing.T) {

		dockerRunnerMock, tailLogsChannel, cancellationChannel, pipelineRunner := resetState()

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
		go pipelineRunner.StopPipelineOnCancellation()
		cancellationChannel <- struct{}{}
		err := pipelineRunner.RunStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.Nil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		canceledStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusCanceled, *canceledStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingRunningAndCanceledMessageToChannelForCanceledRunEvenWhenRunFails", func(t *testing.T) {

		dockerRunnerMock, tailLogsChannel, cancellationChannel, pipelineRunner := resetState()

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
		go pipelineRunner.StopPipelineOnCancellation()
		cancellationChannel <- struct{}{}
		err := pipelineRunner.RunStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		canceledStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusCanceled, *canceledStatusMessage.Status)
	})

	t.Run("SendsMessagesWithDepthAndParentStageSet", func(t *testing.T) {

		dockerRunnerMock, tailLogsChannel, _, pipelineRunner := resetState()

		depth := 1
		runIndex := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		stage := manifest.EstafetteStage{
			Name:           "nested-stage-0",
			ContainerImage: "alpine:latest",
		}
		var parentStage *manifest.EstafetteStage = &manifest.EstafetteStage{
			Name: "stage-a",
			ParallelStages: []*manifest.EstafetteStage{
				&stage,
			},
		}

		// set mock responses
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
		err := pipelineRunner.RunStage(context.Background(), depth, runIndex, dir, envvars, parentStage, stage)

		assert.Nil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, "nested-stage-0", pendingStatusMessage.Step)
		assert.Equal(t, 1, pendingStatusMessage.Depth)
		assert.Equal(t, "stage-a", pendingStatusMessage.ParentStage)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, "nested-stage-0", runningStatusMessage.Step)
		assert.Equal(t, 1, runningStatusMessage.Depth)
		assert.Equal(t, "stage-a", runningStatusMessage.ParentStage)

		succeededStatusMessage := <-tailLogsChannel
		assert.Equal(t, "nested-stage-0", succeededStatusMessage.Step)
		assert.Equal(t, 1, succeededStatusMessage.Depth)
		assert.Equal(t, "stage-a", succeededStatusMessage.ParentStage)
	})
}

func TestRunStageWithRetry(t *testing.T) {

	t.Run("ReturnsErrorWhenRunStageFailsWithZeroRetries", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

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
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			callCount++
			return fmt.Errorf("Failed tailing container logs")
		}

		// act
		err := pipelineRunner.RunStageWithRetry(context.Background(), depth, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed tailing container logs", err.Error())
		assert.Equal(t, 1, callCount)
	})

	t.Run("ReturnsErrorWhenRunStageFailsWithAllRetries", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

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
		err := pipelineRunner.RunStageWithRetry(context.Background(), depth, dir, envvars, parentStage, stage)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed tailing container logs", err.Error())
		assert.Equal(t, 3, callCount)
	})

	t.Run("ReturnsErrorWhenRunStageFailsWithAllRetries", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

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
		err := pipelineRunner.RunStageWithRetry(context.Background(), depth, dir, envvars, parentStage, stage)

		assert.Nil(t, err)
		assert.Equal(t, 3, callCount)
	})

	t.Run("SendsMessageWithErrorAsLogLineForFailingStage", func(t *testing.T) {

		dockerRunnerMock, tailLogsChannel, _, pipelineRunner := resetState()

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
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			return fmt.Errorf("Failed tailing container logs")
		}

		// act
		_ = pipelineRunner.RunStageWithRetry(context.Background(), depth, dir, envvars, parentStage, stage)

		_ = <-tailLogsChannel                // pending state
		_ = <-tailLogsChannel                // running state
		_ = <-tailLogsChannel                // failed state
		errorLogMessage := <-tailLogsChannel // logged error message

		if assert.NotNil(t, errorLogMessage.LogLine) {
			assert.Equal(t, "Failed tailing container logs", errorLogMessage.LogLine.Text)
		}
	})

	t.Run("SendsMessageWithErrorAsLogLineForEachFailingAttempt", func(t *testing.T) {

		dockerRunnerMock, tailLogsChannel, _, pipelineRunner := resetState()

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
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {

			defer func() { iteration++ }()

			switch iteration {
			case 0:
				return fmt.Errorf("Failed tailing container logs attempt 1")
			case 1:
				return fmt.Errorf("Failed tailing container logs attempt 2")
			case 2:
				return fmt.Errorf("Failed tailing container logs attempt 3")
			}

			return fmt.Errorf("Shouldn't call it this often")
		}

		// act
		_ = pipelineRunner.RunStageWithRetry(context.Background(), depth, dir, envvars, parentStage, stage)

		_ = <-tailLogsChannel                // pending state
		_ = <-tailLogsChannel                // running state
		_ = <-tailLogsChannel                // failed state
		errorLogMessage := <-tailLogsChannel // logged error message

		if assert.NotNil(t, errorLogMessage.LogLine) {
			assert.Equal(t, "Failed tailing container logs attempt 1", errorLogMessage.LogLine.Text)
		}

		_ = <-tailLogsChannel               // pending state
		_ = <-tailLogsChannel               // running state
		_ = <-tailLogsChannel               // failed state
		errorLogMessage = <-tailLogsChannel // logged error message

		if assert.NotNil(t, errorLogMessage.LogLine) {
			assert.Equal(t, "Failed tailing container logs attempt 2", errorLogMessage.LogLine.Text)
		}

		_ = <-tailLogsChannel               // pending state
		_ = <-tailLogsChannel               // running state
		_ = <-tailLogsChannel               // failed state
		errorLogMessage = <-tailLogsChannel // logged error message

		if assert.NotNil(t, errorLogMessage.LogLine) {
			assert.Equal(t, "Failed tailing container logs attempt 3", errorLogMessage.LogLine.Text)
		}
	})
}

func TestRunService(t *testing.T) {

	t.Run("ReturnsErrorWhenPullImageFails", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			return fmt.Errorf("Failed pulling image")
		}

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed pulling image", err.Error())
	})

	t.Run("ReturnsErrorWhenGetImageSizeFails", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return false }
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error { return nil }
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, fmt.Errorf("Failed getting image size")
		}

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed getting image size", err.Error())
	})

	t.Run("ReturnsErrorWhenStartStageContainerFails", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return true }
		dockerRunnerMock.startServiceContainerFunc = func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
			return "", fmt.Errorf("Failed starting container")
		}

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed starting container", err.Error())
	})

	t.Run("ReturnsNoErrorWhenTailContainerLogsFailsSinceItRunsInTheBackground", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return true }
		dockerRunnerMock.startServiceContainerFunc = func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
			return "abc", nil
		}
		var wg sync.WaitGroup
		wg.Add(1)
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			defer wg.Done()
			return fmt.Errorf("Failed tailing container logs")
		}

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.Nil(t, err)
	})

	t.Run("ReturnsErrorWhenRunReadinessProbeContainerFails", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
			Readiness:      &manifest.ReadinessProbe{},
		}

		// set mock responses
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return true }
		dockerRunnerMock.startServiceContainerFunc = func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
			return "abc", nil
		}
		var wg sync.WaitGroup
		wg.Add(1)
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			defer wg.Done()
			return nil
		}
		dockerRunnerMock.runReadinessProbeContainerFunc = func(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
			return fmt.Errorf("Failed readiness probe")
		}

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.NotNil(t, err)
		assert.Equal(t, "Failed readiness probe", err.Error())
	})

	t.Run("ReturnsNoErrorWhenContainerPullsStartsAndLogs", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
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
		var wg sync.WaitGroup
		wg.Add(1)
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			defer wg.Done()
			tailContainerLogsFuncCalled = true
			return nil
		}
		dockerRunnerMock.runReadinessProbeContainerFunc = func(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
			runReadinessProbeContainerFuncCalled = true
			return nil
		}

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.Nil(t, err)
		assert.True(t, isImagePulledFuncCalled)
		assert.True(t, pullImageFuncCalled)
		assert.True(t, getImageSizeFuncCalled)
		assert.True(t, startServiceContainerFuncCalled)
		assert.True(t, tailContainerLogsFuncCalled)
		assert.True(t, runReadinessProbeContainerFuncCalled)
	})

	t.Run("SendsRunningMessageToChannelForSuccessfulRunWhenImageIsAlreadyPulled", func(t *testing.T) {

		dockerRunnerMock, tailLogsChannel, _, pipelineRunner := resetState()

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		dockerRunnerMock.isImagePulledFunc = func(stageName string, containerImage string) bool { return true }
		dockerRunnerMock.getImageSizeFunc = func(containerImage string) (int64, error) {
			return 0, nil
		}
		dockerRunnerMock.startServiceContainerFunc = func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
			return "abc", nil
		}
		var wg sync.WaitGroup
		wg.Add(1)
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			defer wg.Done()
			return nil
		}

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.Nil(t, err)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingAndRunningMessageToChannelForSuccessfulStartAndReadiness", func(t *testing.T) {

		dockerRunnerMock, tailLogsChannel, _, pipelineRunner := resetState()

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
			Readiness:      &manifest.ReadinessProbe{},
		}

		// set mock responses
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
		var wg sync.WaitGroup
		wg.Add(1)
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			defer wg.Done()
			return nil
		}
		dockerRunnerMock.runReadinessProbeContainerFunc = func(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
			return nil
		}

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.Nil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingRunningAndFailedMessageToChannelForFailingReadiness", func(t *testing.T) {

		dockerRunnerMock, tailLogsChannel, _, pipelineRunner := resetState()

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
			Readiness:      &manifest.ReadinessProbe{},
		}

		// set mock responses
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
		var wg sync.WaitGroup
		wg.Add(1)
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			defer wg.Done()
			// ensure tailing doesn't set status before the main routine does
			time.Sleep(100 * time.Millisecond)
			return nil
		}
		dockerRunnerMock.runReadinessProbeContainerFunc = func(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
			return fmt.Errorf("Failed readiness probe")
		}

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.NotNil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		failedStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusFailed, *failedStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingRunningAndCanceledMessageToChannelForCanceledRun", func(t *testing.T) {

		dockerRunnerMock, tailLogsChannel, cancellationChannel, pipelineRunner := resetState()

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
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
		var wg sync.WaitGroup
		wg.Add(1)
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			defer wg.Done()
			// ensure tailing doesn't set status before the main routine does
			time.Sleep(100 * time.Millisecond)
			return nil
		}

		// act
		go pipelineRunner.StopPipelineOnCancellation()
		cancellationChannel <- struct{}{}
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.Nil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		canceledStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusCanceled, *canceledStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingRunningAndCanceledMessageToChannelForCanceledRunEvenWhenReadinessFails", func(t *testing.T) {

		dockerRunnerMock, tailLogsChannel, cancellationChannel, pipelineRunner := resetState()

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
			Readiness:      &manifest.ReadinessProbe{},
		}

		// set mock responses
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
		var wg sync.WaitGroup
		wg.Add(1)
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			defer wg.Done()
			// ensure tailing doesn't set status before the main routine does
			time.Sleep(100 * time.Millisecond)
			return nil
		}
		dockerRunnerMock.runReadinessProbeContainerFunc = func(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
			return fmt.Errorf("Failed readiness probe")
		}

		// act
		go pipelineRunner.StopPipelineOnCancellation()
		cancellationChannel <- struct{}{}
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.NotNil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusRunning, *runningStatusMessage.Status)

		canceledStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.StatusCanceled, *canceledStatusMessage.Status)
	})
}

func TestRunStages(t *testing.T) {

	t.Run("ReturnsErrorWhenFirstStageFails", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name:           "stage-a",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded'",
			},
		}

		// set mock responses
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			return fmt.Errorf("Failed pulling image")
		}

		// act
		_, err := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		if assert.NotNil(t, err) {
			assert.Equal(t, "Failed pulling image", err.Error())
		}
	})

	t.Run("ReturnsErrorWhenFirstStageFailsButSecondRunsSuccessfully", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name:           "stage-a",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded'",
			},
			&manifest.EstafetteStage{
				Name:           "stage-b",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded' || status == 'failed'",
			},
		}

		// set mock responses
		iteration := 0
		callCount := 0
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			defer func() { iteration++ }()
			callCount++

			switch iteration {
			case 0:
				return fmt.Errorf("Failed pulling image")
			case 1:
				return nil
			}

			return fmt.Errorf("Shouldn't call it this often")
		}

		// act
		_, err := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		if assert.NotNil(t, err) {
			assert.Equal(t, "Failed pulling image", err.Error())
		}
		assert.Equal(t, 2, callCount)
	})

	t.Run("SkipsStagesWhichWhenClauseEvaluatesToFalse", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name:           "stage-a",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded'",
			},
			&manifest.EstafetteStage{
				Name:           "stage-b",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded'",
			},
			&manifest.EstafetteStage{
				Name:           "stage-c",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded' || status == 'failed'",
			},
		}

		// set mock responses
		iteration := 0
		callCount := 0
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			defer func() { iteration++ }()
			callCount++

			switch iteration {
			case 0:
				return fmt.Errorf("Failed pulling image")
			case 1:
				return nil
			}

			return fmt.Errorf("Shouldn't call it this often")
		}

		// act
		_, _ = pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		assert.Equal(t, 2, callCount)
	})

	t.Run("SendsSkippedStatusMessageForSkippedStage", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name:           "stage-a",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded'",
			},
			&manifest.EstafetteStage{
				Name:           "stage-b",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded'",
			},
			&manifest.EstafetteStage{
				Name:           "stage-c",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded' || status == 'failed'",
			},
		}

		// set mock responses
		iteration := 0
		callCount := 0
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			defer func() { iteration++ }()
			callCount++

			switch iteration {
			case 0:
				return fmt.Errorf("Failed pulling image")
			case 1:
				return nil
			}

			return fmt.Errorf("Shouldn't call it this often")
		}

		// act
		buildLogSteps, _ := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		assert.Equal(t, 2, callCount)

		if assert.Equal(t, 3, len(buildLogSteps)) {
			assert.Equal(t, contracts.StatusFailed, buildLogSteps[0].Status)
			assert.Equal(t, contracts.StatusSkipped, buildLogSteps[1].Status)
			assert.Equal(t, contracts.StatusSucceeded, buildLogSteps[2].Status)
		}

		assert.Equal(t, contracts.StatusFailed, contracts.GetAggregatedStatus(buildLogSteps))
	})

	t.Run("SetsPullDurationAndRunDurationForStage", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name:           "stage-a",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded'",
			},
		}

		// set mock responses
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		}
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			time.Sleep(100 * time.Millisecond)
			return nil
		}

		// act
		buildLogSteps, _ := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		if assert.Equal(t, 1, len(buildLogSteps)) {
			assert.GreaterOrEqual(t, int64(50), buildLogSteps[0].Duration.Milliseconds())
			assert.GreaterOrEqual(t, int64(100), buildLogSteps[0].Image.PullDuration.Milliseconds())
		}
	})

	t.Run("InjectsBuilderInfoStageWhenEnableBuilderInfoStageInjectionIsCalledBeforeRunStages", func(t *testing.T) {

		_, _, _, pipelineRunner := resetState()

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name:           "stage-a",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded'",
			},
		}

		// act
		pipelineRunner.EnableBuilderInfoStageInjection()
		buildLogSteps, _ := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		if assert.Equal(t, 2, len(buildLogSteps)) {
			assert.Equal(t, "builder-info", buildLogSteps[0].Step)
			assert.Equal(t, contracts.StatusSucceeded, buildLogSteps[0].Status)
			assert.True(t, buildLogSteps[0].AutoInjected)
			assert.Equal(t, 1, len(buildLogSteps[0].LogLines))
		}
	})

	t.Run("CallsCreateBridgeNetwork", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name:           "stage-a",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded'",
			},
		}

		// set mock responses
		createBridgeNetworkFuncCalled := false
		dockerRunnerMock.createBridgeNetworkFunc = func(ctx context.Context) error {
			createBridgeNetworkFuncCalled = true
			return nil
		}

		// act
		_, _ = pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		assert.True(t, createBridgeNetworkFuncCalled)
	})

	t.Run("CallsDeleteBridgeNetwork", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name:           "stage-a",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded'",
			},
		}

		// set mock responses
		deleteBridgeNetworkFuncCalled := false
		dockerRunnerMock.deleteBridgeNetworkFunc = func(ctx context.Context) error {
			deleteBridgeNetworkFuncCalled = true
			return nil
		}

		// act
		_, _ = pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		assert.True(t, deleteBridgeNetworkFuncCalled)
	})
}

func TestRunStagesWithParallelStages(t *testing.T) {

	t.Run("RunsParallelStagesReturnsBuildLogStepsWithNestedSteps", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "stage-a",
				When: "status == 'succeeded'",
				ParallelStages: []*manifest.EstafetteStage{
					&manifest.EstafetteStage{
						Name:           "nested-stage-0",
						ContainerImage: "alpine:latest",
						When:           "status == 'succeeded'",
					},
					&manifest.EstafetteStage{
						Name:           "nested-stage-1",
						ContainerImage: "alpine:latest",
						When:           "status == 'succeeded'",
					},
				},
			},
		}

		// set mock responses
		callCount := 0
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			callCount++
			return nil
		}

		// act
		buildLogSteps, _ := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		assert.Equal(t, 2, callCount)

		if assert.Equal(t, 1, len(buildLogSteps)) {
			assert.Equal(t, "stage-a", buildLogSteps[0].Step)
			assert.Equal(t, contracts.StatusSucceeded, buildLogSteps[0].Status)
			assert.Equal(t, 0, buildLogSteps[0].Depth)
			if assert.Equal(t, 2, len(buildLogSteps[0].NestedSteps)) {
				assert.Contains(t, []string{"nested-stage-0", "nested-stage-1"}, buildLogSteps[0].NestedSteps[0].Step)
				assert.Equal(t, contracts.StatusSucceeded, buildLogSteps[0].NestedSteps[0].Status)
				assert.Equal(t, 1, buildLogSteps[0].NestedSteps[0].Depth)
				assert.Contains(t, []string{"nested-stage-0", "nested-stage-1"}, buildLogSteps[0].NestedSteps[1].Step)
				assert.Equal(t, contracts.StatusSucceeded, buildLogSteps[0].NestedSteps[1].Status)
				assert.Equal(t, 1, buildLogSteps[0].NestedSteps[1].Depth)
			}
		}

		assert.Equal(t, contracts.StatusSucceeded, contracts.GetAggregatedStatus(buildLogSteps))
	})
}

func TestRunStagesWithServices(t *testing.T) {

	t.Run("RunsServicesReturnsBuildLogStepsWithServices", func(t *testing.T) {

		dockerRunnerMock, _, _, pipelineRunner := resetState()

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name:           "stage-a",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded'",
				Services: []*manifest.EstafetteService{
					&manifest.EstafetteService{
						Name:           "nested-service-0",
						ContainerImage: "alpine:latest",
						When:           "status == 'succeeded'",
					},
					&manifest.EstafetteService{
						Name:           "nested-service-1",
						ContainerImage: "alpine:latest",
						When:           "status == 'succeeded'",
					},
				},
			},
		}

		// set mock responses
		callCount := 0
		dockerRunnerMock.pullImageFunc = func(ctx context.Context, stageName string, containerImage string) error {
			callCount++
			return nil
		}
		var wg sync.WaitGroup
		wg.Add(1)
		dockerRunnerMock.tailContainerLogsFunc = func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
			if stageType == contracts.TypeService {
				wg.Wait()
			}
			return nil
		}
		dockerRunnerMock.stopServiceContainersFunc = func(ctx context.Context, parentStage manifest.EstafetteStage) {
			wg.Done()
		}

		// act
		buildLogSteps, _ := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		assert.Equal(t, 3, callCount)

		if assert.Equal(t, 1, len(buildLogSteps)) {
			assert.Equal(t, "stage-a", buildLogSteps[0].Step)
			assert.Equal(t, contracts.StatusSucceeded, buildLogSteps[0].Status)
			assert.Equal(t, 0, buildLogSteps[0].Depth)
			if assert.Equal(t, 2, len(buildLogSteps[0].Services)) {
				assert.Contains(t, []string{"nested-service-0", "nested-service-1"}, buildLogSteps[0].Services[0].Step)
				assert.Equal(t, contracts.StatusSucceeded, buildLogSteps[0].Services[0].Status)
				assert.Equal(t, 1, buildLogSteps[0].Services[0].Depth)
				assert.Contains(t, []string{"nested-service-0", "nested-service-1"}, buildLogSteps[0].Services[1].Step)
				assert.Equal(t, contracts.StatusSucceeded, buildLogSteps[0].Services[1].Status)
				assert.Equal(t, 1, buildLogSteps[0].Services[1].Depth)
			}
		}

		assert.Equal(t, contracts.StatusSucceeded, contracts.GetAggregatedStatus(buildLogSteps))
	})
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
			Type:        contracts.TypeService,
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
			Type:        contracts.TypeService,
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
			Type:        contracts.TypeService,
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
			Type:        contracts.TypeService,
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
			Type:        contracts.TypeService,
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
			Type:        contracts.TypeStage,
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
			Type:        contracts.TypeService,
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
			Type:        contracts.TypeStage,
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
			Type:        contracts.TypeStage,
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
			Type:        contracts.TypeService,
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
			Type:        contracts.TypeService,
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
			Type:        contracts.TypeStage,
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
			Type:        contracts.TypeService,
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
			Type:        contracts.TypeStage,
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
			Type:        contracts.TypeService,
			Status:      &status,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, "RUNNING", pipelineRunner.buildLogSteps[0].Services[0].Status)
	})

	t.Run("NestsParallelStageMessages", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{},
		}

		statusRunning := contracts.StatusRunning
		statusPending := contracts.StatusPending
		statusSucceeded := contracts.StatusSucceeded

		// stage-a start
		tailLogLine := contracts.TailLogLine{
			Step:   "stage-a",
			Type:   contracts.TypeStage,
			Status: &statusRunning,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		// nested-stage-1
		tailLogLine = contracts.TailLogLine{
			Step:        "nested-stage-1",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        contracts.TypeStage,
			Status:      &statusPending,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		tailLogLine = contracts.TailLogLine{
			Step:        "nested-stage-1",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        contracts.TypeStage,
			Status:      &statusRunning,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		tailLogLine = contracts.TailLogLine{
			Step:        "nested-stage-1",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        contracts.TypeStage,
			Status:      &statusSucceeded,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		// nested-stage-0
		tailLogLine = contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        contracts.TypeStage,
			Status:      &statusPending,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		tailLogLine = contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        contracts.TypeStage,
			Status:      &statusRunning,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		tailLogLine = contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        contracts.TypeStage,
			Status:      &statusSucceeded,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		// stage-a finish
		tailLogLine = contracts.TailLogLine{
			Step:   "stage-a",
			Type:   contracts.TypeStage,
			Status: &statusSucceeded,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		if assert.Equal(t, 1, len(pipelineRunner.buildLogSteps)) {
			assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
			assert.Equal(t, contracts.StatusSucceeded, pipelineRunner.buildLogSteps[0].Status)

			assert.Equal(t, 2, len(pipelineRunner.buildLogSteps[0].NestedSteps))

			assert.Equal(t, "nested-stage-1", pipelineRunner.buildLogSteps[0].NestedSteps[0].Step)
			assert.Equal(t, contracts.StatusSucceeded, pipelineRunner.buildLogSteps[0].NestedSteps[0].Status)

			assert.Equal(t, "nested-stage-0", pipelineRunner.buildLogSteps[0].NestedSteps[1].Step)
			assert.Equal(t, contracts.StatusSucceeded, pipelineRunner.buildLogSteps[0].NestedSteps[1].Status)
		}
	})
}

func TestIsFinalStageComplete(t *testing.T) {

	t.Run("ReturnsFalseIfBuildLogStepsAreEmpty", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: make([]*contracts.BuildLogStep, 0),
		}
		stages := []*manifest.EstafetteStage{}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.False(t, isComplete)
	})

	t.Run("ReturnsFalseIfLastStepHasRunningStatus", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "last-stage",
					Status: contracts.StatusRunning,
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "last-stage",
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.False(t, isComplete)
	})

	t.Run("ReturnsFalseIfLastStepHasPendingStatus", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "last-stage",
					Status: contracts.StatusPending,
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "last-stage",
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.False(t, isComplete)
	})

	t.Run("ReturnsTrueIfLastStepHasSucceededStatus", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "last-stage",
					Status: contracts.StatusSucceeded,
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "last-stage",
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.True(t, isComplete)
	})

	t.Run("ReturnsTrueIfLastStepHasFailedStatus", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "last-stage",
					Status: contracts.StatusFailed,
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "last-stage",
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.True(t, isComplete)
	})

	t.Run("ReturnsTrueIfLastStepHasSkippedStatus", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "last-stage",
					Status: contracts.StatusSkipped,
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "last-stage",
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.True(t, isComplete)
	})

	t.Run("ReturnsTrueIfLastStepHasCanceledStatus", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "last-stage",
					Status: contracts.StatusCanceled,
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "last-stage",
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.True(t, isComplete)
	})

	t.Run("ReturnsFalseIfLastStepHasSucceededStatusButIsNotTheFinalStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "first-stage",
					Status: contracts.StatusSucceeded,
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "first-stage",
			},
			&manifest.EstafetteStage{
				Name: "last-stage",
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.False(t, isComplete)
	})

	t.Run("ReturnsFalseIfLastStepHasFailedStatusButIsNotTheFinalStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "first-stage",
					Status: contracts.StatusFailed,
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "first-stage",
			},
			&manifest.EstafetteStage{
				Name: "last-stage",
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.False(t, isComplete)
	})

	t.Run("ReturnsFalseIfLastStepHasSkippedStatusButIsNotTheFinalStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "first-stage",
					Status: contracts.StatusSkipped,
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "first-stage",
			},
			&manifest.EstafetteStage{
				Name: "last-stage",
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.False(t, isComplete)
	})

	t.Run("ReturnsFalseIfLastStepHasCanceledStatusButIsNotTheFinalStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "first-stage",
					Status: contracts.StatusCanceled,
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "first-stage",
			},
			&manifest.EstafetteStage{
				Name: "last-stage",
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.False(t, isComplete)
	})

	t.Run("ReturnsFalseIfLastStageHasParallelStagesButLastStepHasNoEqualAmountOfNestedSteps", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "last-stage",
					Status: contracts.StatusSucceeded,
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "last-stage",
				ParallelStages: []*manifest.EstafetteStage{
					&manifest.EstafetteStage{
						Name: "nested-stage",
					},
				},
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.False(t, isComplete)
	})

	t.Run("ReturnsFalseIfLastStepHasSucceededStatusButAnyParallelStagesHavePendingOrRunningStatus", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "last-stage",
					Status: contracts.StatusSucceeded,
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step:   "nested-stage",
							Status: contracts.StatusRunning,
						},
					},
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "last-stage",
				ParallelStages: []*manifest.EstafetteStage{
					&manifest.EstafetteStage{
						Name: "nested-stage",
					},
				},
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.False(t, isComplete)
	})

	t.Run("ReturnsFalseIfLastStageHasServicesButLastStepHasNoEqualAmountOfServices", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "last-stage",
					Status: contracts.StatusSucceeded,
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "last-stage",
				Services: []*manifest.EstafetteService{
					&manifest.EstafetteService{
						Name: "nested-service",
					},
				},
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.False(t, isComplete)
	})

	t.Run("ReturnsFalseIfLastStepHasSucceededStatusButAnyServicesHavePendingOrRunningStatus", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "last-stage",
					Status: contracts.StatusSucceeded,
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step:   "nested-service",
							Status: contracts.StatusRunning,
						},
					},
				},
			},
		}
		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				Name: "last-stage",
				Services: []*manifest.EstafetteService{
					&manifest.EstafetteService{
						Name: "nested-service",
					},
				},
			},
		}

		// act
		isComplete := pipelineRunner.isFinalStageComplete(stages)

		assert.False(t, isComplete)
	})
}

func resetState() (*dockerRunnerMockImpl, chan contracts.TailLogLine, chan struct{}, PipelineRunner) {

	secretHelper := crypt.NewSecretHelper("SazbwMf3NZxVVbBqQHebPcXCqrVn3DDp", false)
	envvarHelper := NewEnvvarHelper("TESTPREFIX_", secretHelper, obfuscator)
	whenEvaluator := NewWhenEvaluator(envvarHelper)
	dockerRunnerMock := &dockerRunnerMockImpl{}
	tailLogsChannel := make(chan contracts.TailLogLine, 10000)
	cancellationChannel := make(chan struct{})
	pipelineRunner := NewPipelineRunner(envvarHelper, whenEvaluator, dockerRunnerMock, true, cancellationChannel, tailLogsChannel)

	return dockerRunnerMock, tailLogsChannel, cancellationChannel, pipelineRunner
}
