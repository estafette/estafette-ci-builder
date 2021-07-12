package builder

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	contracts "github.com/estafette/estafette-ci-contracts"
	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
	foundation "github.com/estafette/estafette-foundation"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestRunStage(t *testing.T) {

	t.Run("ReturnsErrorWhenPullImageFails", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}
		stageIndex := 0

		// set mock responses
		containerRunnerMock.EXPECT().PullImage(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("Failed pulling image"))
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunStage(context.Background(), depth, dir, envvars, parentStage, stage, stageIndex)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed pulling image", err.Error())
	})

	t.Run("ReturnsErrorWhenGetImageSizeFails", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}
		stageIndex := 0

		// set mock responses
		containerRunnerMock.EXPECT().GetImageSize(gomock.Any(), gomock.Any()).Return(int64(0), fmt.Errorf("Failed getting image size"))
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunStage(context.Background(), depth, dir, envvars, parentStage, stage, stageIndex)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed getting image size", err.Error())
	})

	t.Run("ReturnsErrorWhenStartStageContainerFails", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}
		stageIndex := 0

		// set mock responses
		containerRunnerMock.EXPECT().StartStageContainer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", fmt.Errorf("Failed starting container"))
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunStage(context.Background(), depth, dir, envvars, parentStage, stage, stageIndex)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed starting container", err.Error())
	})

	t.Run("ReturnsErrorWhenTailContainerLogsFails", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}
		stageIndex := 0

		// set mock responses
		containerRunnerMock.EXPECT().TailContainerLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("Failed tailing container logs"))
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunStage(context.Background(), depth, dir, envvars, parentStage, stage, stageIndex)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed tailing container logs", err.Error())
	})

	t.Run("ReturnsNoErrorWhenContainerPullsStartsAndLogs", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}
		stageIndex := 0

		// set mock responses
		containerRunnerMock.EXPECT().IsImagePulled(gomock.Any(), gomock.Any(), gomock.Any()).Return(false)
		containerRunnerMock.EXPECT().PullImage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		containerRunnerMock.EXPECT().GetImageSize(gomock.Any(), gomock.Any()).Return(int64(0), nil)
		containerRunnerMock.EXPECT().StartStageContainer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("abc", nil)
		containerRunnerMock.EXPECT().TailContainerLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunStage(context.Background(), depth, dir, envvars, parentStage, stage, stageIndex)

		assert.Nil(t, err)
	})

	t.Run("SendsSequenceOfRunningAndSucceededMessageToChannelForSuccessfulRunWhenImageIsAlreadyPulled", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		tailLogsChannel, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}
		stageIndex := 0

		// set mock responses
		containerRunnerMock.EXPECT().IsImagePulled(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunStage(context.Background(), depth, dir, envvars, parentStage, stage, stageIndex)

		assert.Nil(t, err)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusRunning, *runningStatusMessage.Status)

		succeededStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusSucceeded, *succeededStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingAndRunningAndSucceededMessageToChannelForSuccessfulRun", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		tailLogsChannel, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}
		stageIndex := 0

		// set mock responses
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunStage(context.Background(), depth, dir, envvars, parentStage, stage, stageIndex)

		assert.Nil(t, err)

		pendingStatusAndImageInfoMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusPending, *pendingStatusAndImageInfoMessage.Status)
		assert.NotNil(t, pendingStatusAndImageInfoMessage.Image)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusRunning, *runningStatusMessage.Status)

		succeededStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusSucceeded, *succeededStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingAndRunningAndFailedMessageToChannelForFailingRun", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		tailLogsChannel, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}
		stageIndex := 0

		// set mock responses
		containerRunnerMock.EXPECT().TailContainerLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("Failed tailing container logs"))
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunStage(context.Background(), depth, dir, envvars, parentStage, stage, stageIndex)

		assert.NotNil(t, err)

		pendingStatusAndImageInfoMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusPending, *pendingStatusAndImageInfoMessage.Status)
		assert.NotNil(t, pendingStatusAndImageInfoMessage.Image)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusRunning, *runningStatusMessage.Status)

		failedStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusFailed, *failedStatusMessage.Status)
	})

	t.Run("SendsCanceledMessageToChannelForCanceledRun", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		tailLogsChannel, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}
		stageIndex := 0

		// set mock responses
		setDefaultMockExpectancies(containerRunnerMock)
		ctx, cancel := context.WithCancel(context.Background())

		// act
		go pipelineRunner.StopPipelineOnCancellation(ctx)
		cancel()
		time.Sleep(10 * time.Millisecond)
		err := pipelineRunner.RunStage(ctx, depth, dir, envvars, parentStage, stage, stageIndex)

		assert.Nil(t, err)

		canceledStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusCanceled, *canceledStatusMessage.Status)
	})

	t.Run("SendsCanceledMessageToChannelForCanceledRunEvenWhenRunFails", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		tailLogsChannel, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		var parentStage *manifest.EstafetteStage = nil
		stage := manifest.EstafetteStage{
			Name:           "stage-a",
			ContainerImage: "alpine:latest",
		}
		stageIndex := 0

		// set mock responses
		containerRunnerMock.EXPECT().TailContainerLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("Failed tailing container logs")).AnyTimes()
		setDefaultMockExpectancies(containerRunnerMock)
		ctx, cancel := context.WithCancel(context.Background())

		// act
		go pipelineRunner.StopPipelineOnCancellation(ctx)
		cancel()
		time.Sleep(10 * time.Millisecond)
		err := pipelineRunner.RunStage(ctx, depth, dir, envvars, parentStage, stage, stageIndex)

		assert.Nil(t, err)

		canceledStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusCanceled, *canceledStatusMessage.Status)
	})

	t.Run("SendsMessagesWithDepthAndParentStageSet", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		tailLogsChannel, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		depth := 1
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
		stageIndex := 0

		// set mock responses
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunStage(context.Background(), depth, dir, envvars, parentStage, stage, stageIndex)

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

func TestRunService(t *testing.T) {

	t.Run("ReturnsErrorWhenPullImageFails", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		containerRunnerMock.EXPECT().PullImage(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("Failed pulling image"))
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed pulling image", err.Error())
	})

	t.Run("ReturnsErrorWhenGetImageSizeFails", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		containerRunnerMock.EXPECT().GetImageSize(gomock.Any(), gomock.Any()).Return(int64(0), fmt.Errorf("Failed getting image size"))
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed getting image size", err.Error())
	})

	t.Run("ReturnsErrorWhenStartStageContainerFails", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		containerRunnerMock.EXPECT().StartServiceContainer(gomock.Any(), gomock.Any(), gomock.Any()).Return("", fmt.Errorf("Failed starting container"))
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		assert.NotNil(t, err)
		assert.Equal(t, "Failed starting container", err.Error())
	})

	t.Run("ReturnsNoErrorWhenTailContainerLogsFailsSinceItRunsInTheBackground", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		var wg sync.WaitGroup
		wg.Add(1)
		containerRunnerMock.EXPECT().TailContainerLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth int, multiStage *bool) (err error) {
				defer wg.Done()
				return fmt.Errorf("Failed tailing container logs")
			})
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.Nil(t, err)
	})

	t.Run("ReturnsErrorWhenRunReadinessProbeContainerFails", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		var wg sync.WaitGroup
		wg.Add(1)
		containerRunnerMock.EXPECT().TailContainerLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth int, multiStage *bool) (err error) {
				defer wg.Done()
				return nil
			})
		containerRunnerMock.EXPECT().RunReadinessProbeContainer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("Failed readiness probe"))
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.NotNil(t, err)
		assert.Equal(t, "Failed readiness probe", err.Error())
	})

	t.Run("ReturnsNoErrorWhenContainerPullsStartsAndLogs", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		containerRunnerMock.EXPECT().IsImagePulled(gomock.Any(), gomock.Any(), gomock.Any()).Return(false)
		containerRunnerMock.EXPECT().PullImage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		containerRunnerMock.EXPECT().GetImageSize(gomock.Any(), gomock.Any()).Return(int64(0), nil)
		containerRunnerMock.EXPECT().StartServiceContainer(gomock.Any(), gomock.Any(), gomock.Any()).Return("abc", nil)
		var wg sync.WaitGroup
		wg.Add(1)
		containerRunnerMock.EXPECT().TailContainerLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth int, multiStage *bool) (err error) {
				defer wg.Done()
				return nil
			})
		containerRunnerMock.EXPECT().RunReadinessProbeContainer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.Nil(t, err)
	})

	t.Run("SendsRunningMessageToChannelForSuccessfulRunWhenImageIsAlreadyPulled", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		tailLogsChannel, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		containerRunnerMock.EXPECT().IsImagePulled(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
		var wg sync.WaitGroup
		wg.Add(1)
		containerRunnerMock.EXPECT().TailContainerLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth int, multiStage *bool) (err error) {
				defer wg.Done()
				return nil
			})
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.Nil(t, err)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusRunning, *runningStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingAndRunningMessageToChannelForSuccessfulStartAndReadiness", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		tailLogsChannel, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		var wg sync.WaitGroup
		wg.Add(1)
		containerRunnerMock.EXPECT().TailContainerLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth int, multiStage *bool) (err error) {
				defer wg.Done()
				return nil
			})
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.Nil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusRunning, *runningStatusMessage.Status)
	})

	t.Run("SendsSequenceOfPendingAndRunningAndFailedMessageToChannelForFailingReadiness", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		tailLogsChannel, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		var wg sync.WaitGroup
		wg.Add(1)
		containerRunnerMock.EXPECT().TailContainerLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth int, multiStage *bool) (err error) {
				defer wg.Done()
				// ensure tailing doesn't set status before the main routine does
				time.Sleep(100 * time.Millisecond)
				return nil
			})
		containerRunnerMock.EXPECT().RunReadinessProbeContainer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("Failed readiness probe"))
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		err := pipelineRunner.RunService(context.Background(), envvars, parentStage, service)

		// wait for tailContainerLogsFunc to finish
		wg.Wait()

		assert.NotNil(t, err)

		pendingStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusPending, *pendingStatusMessage.Status)

		runningStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusRunning, *runningStatusMessage.Status)

		failedStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusFailed, *failedStatusMessage.Status)
	})

	t.Run("SendsCanceledMessageToChannelForCanceledRun", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		tailLogsChannel, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		envvars := map[string]string{}
		parentStage := manifest.EstafetteStage{
			Name: "stage-a",
		}
		service := manifest.EstafetteService{
			Name:           "service-a",
			ContainerImage: "alpine:latest",
		}

		// set mock responses
		setDefaultMockExpectancies(containerRunnerMock)
		ctx, cancel := context.WithCancel(context.Background())

		// act
		go pipelineRunner.StopPipelineOnCancellation(ctx)
		cancel()
		time.Sleep(10 * time.Millisecond)
		err := pipelineRunner.RunService(ctx, envvars, parentStage, service)

		assert.Nil(t, err)

		canceledStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusCanceled, *canceledStatusMessage.Status)
	})

	t.Run("SendsCanceledMessageToChannelForCanceledRunEvenWhenReadinessFails", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		tailLogsChannel, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		setDefaultMockExpectancies(containerRunnerMock)
		ctx, cancel := context.WithCancel(context.Background())

		// act
		go pipelineRunner.StopPipelineOnCancellation(ctx)
		cancel()

		time.Sleep(10 * time.Millisecond)
		err := pipelineRunner.RunService(ctx, envvars, parentStage, service)

		assert.Nil(t, err)

		canceledStatusMessage := <-tailLogsChannel
		assert.Equal(t, contracts.LogStatusCanceled, *canceledStatusMessage.Status)
	})
}

func TestRunStages(t *testing.T) {

	t.Run("CallsCreateBridgeNetwork", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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

		containerRunnerMock.EXPECT().CreateNetworks(gomock.Any()).Return(nil)
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		_, _ = pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)
	})

	t.Run("CallsDeleteBridgeNetwork", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		containerRunnerMock.EXPECT().DeleteNetworks(gomock.Any()).Return(nil)
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		_, _ = pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)
	})

	t.Run("CallsStopMultiStageServiceContainers", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		containerRunnerMock.EXPECT().StopMultiStageServiceContainers(gomock.Any())
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		_, _ = pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)
	})

	t.Run("ReturnsErrorWhenFirstStageFails", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		containerRunnerMock.EXPECT().PullImage(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("Failed pulling image"))
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		_, err := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		if assert.NotNil(t, err) {
			assert.Equal(t, "Failed pulling image", err.Error())
		}
	})

	t.Run("ReturnsErrorWhenFirstStageFailsButSecondRunsSuccessfully", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		containerRunnerMock.EXPECT().PullImage(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, stageName string, containerImage string) (err error) {
				defer func() { iteration++ }()

				switch iteration {
				case 0:
					return fmt.Errorf("Failed pulling image")
				case 1:
					return nil
				}

				return fmt.Errorf("Shouldn't call it this often")
			}).Times(2)
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		_, err := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		if assert.NotNil(t, err) {
			assert.Equal(t, "Failed pulling image", err.Error())
		}
	})

	t.Run("SkipsStagesWhichWhenClauseEvaluatesToFalse", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		containerRunnerMock.EXPECT().PullImage(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, stageName string, containerImage string) (err error) {
				defer func() { iteration++ }()

				switch iteration {
				case 0:
					return fmt.Errorf("Failed pulling image")
				case 1:
					return nil
				}

				return fmt.Errorf("Shouldn't call it this often")
			}).Times(2)
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		_, _ = pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)
	})

	t.Run("SendsSkippedStatusMessageForSkippedStage", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		containerRunnerMock.EXPECT().PullImage(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, stageName string, containerImage string) (err error) {
				defer func() { iteration++ }()

				switch iteration {
				case 0:
					return fmt.Errorf("Failed pulling image")
				case 1:
					return nil
				}

				return fmt.Errorf("Shouldn't call it this often")
			}).Times(2)
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		buildLogSteps, _ := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		if assert.Equal(t, 3, len(buildLogSteps)) {
			assert.Equal(t, contracts.LogStatusFailed, buildLogSteps[0].Status)
			assert.Equal(t, contracts.LogStatusSkipped, buildLogSteps[1].Status)
			assert.Equal(t, contracts.LogStatusSucceeded, buildLogSteps[2].Status)
		}

		assert.Equal(t, contracts.LogStatusFailed, contracts.GetAggregatedStatus(buildLogSteps))
	})

	t.Run("SetsPullDurationAndRunDurationForStage", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		containerRunnerMock.EXPECT().PullImage(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, stageName string, containerImage string) (err error) {
				time.Sleep(50 * time.Millisecond)
				return nil
			})
		containerRunnerMock.EXPECT().TailContainerLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth int, multiStage *bool) (err error) {
				time.Sleep(100 * time.Millisecond)
				return nil
			})
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		buildLogSteps, _ := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		if assert.Equal(t, 1, len(buildLogSteps)) {
			assert.GreaterOrEqual(t, buildLogSteps[0].Image.PullDuration.Milliseconds(), int64(50))
			assert.GreaterOrEqual(t, buildLogSteps[0].Duration.Milliseconds(), int64(100))
		}
	})

	t.Run("InjectsBuilderInfoStageWhenEnableBuilderInfoStageInjectionIsCalledBeforeRunStages", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)

		containerRunnerMock.EXPECT().Info(gomock.Any()).Return("docker info").Times(1)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

		depth := 0
		dir := "/estafette-work"
		envvars := map[string]string{}
		stages := []*manifest.EstafetteStage{
			{
				Name:           "stage-a",
				ContainerImage: "alpine:latest",
				When:           "status == 'succeeded'",
			},
		}
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		pipelineRunner.EnableBuilderInfoStageInjection()
		buildLogSteps, _ := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		if assert.Equal(t, 2, len(buildLogSteps)) {
			assert.Equal(t, "builder-info", buildLogSteps[0].Step)
			assert.Equal(t, contracts.LogStatusSucceeded, buildLogSteps[0].Status)
			assert.True(t, buildLogSteps[0].AutoInjected)
			assert.Equal(t, 2, len(buildLogSteps[0].LogLines))
		}
	})

	t.Run("SendsCanceledStageForAllStagesWhenFirstStageGetsCanceled", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		setDefaultMockExpectancies(containerRunnerMock)
		ctx, cancel := context.WithCancel(context.Background())

		// act
		go pipelineRunner.StopPipelineOnCancellation(ctx)
		cancel()
		time.Sleep(10 * time.Millisecond)
		buildLogSteps, _ := pipelineRunner.RunStages(ctx, depth, stages, dir, envvars)

		if assert.Equal(t, 3, len(buildLogSteps)) {
			assert.Equal(t, contracts.LogStatusCanceled, buildLogSteps[0].Status)
			assert.Equal(t, contracts.LogStatusCanceled, buildLogSteps[1].Status)
			assert.Equal(t, contracts.LogStatusCanceled, buildLogSteps[2].Status)
		}

		assert.Equal(t, contracts.LogStatusCanceled, contracts.GetAggregatedStatus(buildLogSteps))
	})
}

func TestRunStagesWithParallelStages(t *testing.T) {

	t.Run("RunsParallelStagesReturnsBuildLogStepsWithNestedSteps", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		containerRunnerMock.EXPECT().PullImage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		buildLogSteps, _ := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		if assert.Equal(t, 1, len(buildLogSteps)) {
			assert.Equal(t, "stage-a", buildLogSteps[0].Step)
			assert.Equal(t, contracts.LogStatusSucceeded, buildLogSteps[0].Status)
			assert.Equal(t, 0, buildLogSteps[0].Depth)
			if assert.Equal(t, 2, len(buildLogSteps[0].NestedSteps)) {
				assert.Contains(t, []string{"nested-stage-0", "nested-stage-1"}, buildLogSteps[0].NestedSteps[0].Step)
				assert.Equal(t, contracts.LogStatusSucceeded, buildLogSteps[0].NestedSteps[0].Status)
				assert.Equal(t, 1, buildLogSteps[0].NestedSteps[0].Depth)
				assert.Contains(t, []string{"nested-stage-0", "nested-stage-1"}, buildLogSteps[0].NestedSteps[1].Step)
				assert.Equal(t, contracts.LogStatusSucceeded, buildLogSteps[0].NestedSteps[1].Status)
				assert.Equal(t, 1, buildLogSteps[0].NestedSteps[1].Depth)
			}
		}

		assert.Equal(t, contracts.LogStatusSucceeded, contracts.GetAggregatedStatus(buildLogSteps))
	})
}

func TestRunStagesWithServices(t *testing.T) {

	t.Run("RunsServicesReturnsBuildLogStepsWithServices", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		containerRunnerMock := NewMockContainerRunner(ctrl)
		_, pipelineRunner := getPipelineRunnerAndMocks(ctrl, containerRunnerMock)

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
		containerRunnerMock.EXPECT().PullImage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)

		var wg sync.WaitGroup
		wg.Add(1)
		containerRunnerMock.EXPECT().TailContainerLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth int, multiStage *bool) (err error) {
				if stageType == contracts.LogTypeService {
					wg.Wait()
				}
				return nil
			})
		containerRunnerMock.EXPECT().StopSingleStageServiceContainers(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, parentStage manifest.EstafetteStage) {
				wg.Done()
			})
		setDefaultMockExpectancies(containerRunnerMock)

		// act
		buildLogSteps, _ := pipelineRunner.RunStages(context.Background(), depth, stages, dir, envvars)

		if assert.Equal(t, 1, len(buildLogSteps)) {
			assert.Equal(t, "stage-a", buildLogSteps[0].Step)
			assert.Equal(t, contracts.LogStatusSucceeded, buildLogSteps[0].Status)
			assert.Equal(t, 0, buildLogSteps[0].Depth)
			if assert.Equal(t, 2, len(buildLogSteps[0].Services)) {
				assert.Contains(t, []string{"nested-service-0", "nested-service-1"}, buildLogSteps[0].Services[0].Step)
				assert.Equal(t, contracts.LogStatusSucceeded, buildLogSteps[0].Services[0].Status)
				assert.Equal(t, 1, buildLogSteps[0].Services[0].Depth)
				assert.Contains(t, []string{"nested-service-0", "nested-service-1"}, buildLogSteps[0].Services[1].Step)
				assert.Equal(t, contracts.LogStatusSucceeded, buildLogSteps[0].Services[1].Status)
				assert.Equal(t, 1, buildLogSteps[0].Services[1].Depth)
			}
		}

		assert.Equal(t, contracts.LogStatusSucceeded, contracts.GetAggregatedStatus(buildLogSteps))
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
			Type:        contracts.LogTypeService,
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
			Type:        contracts.LogTypeService,
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
			Type:        contracts.LogTypeService,
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
			Type:        contracts.LogTypeService,
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
			Type:        contracts.LogTypeService,
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
			Type:        contracts.LogTypeStage,
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
			Type:        contracts.LogTypeService,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
	})

	t.Run("AddsMainStageWithDepth0IfServiceContainerStatusComesInFirst", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: make([]*contracts.BuildLogStep, 0),
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        contracts.LogTypeService,
			Depth:       1,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
		assert.Equal(t, 0, pipelineRunner.buildLogSteps[0].Depth)
	})

	t.Run("AddsNestedStageIfDoesNotExist", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        contracts.LogTypeStage,
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
			Type:        contracts.LogTypeStage,
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
			Type:        contracts.LogTypeService,
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
			Type:        contracts.LogTypeService,
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
			Type:        contracts.LogTypeStage,
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
			Type:        contracts.LogTypeService,
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
					Status: contracts.LogStatusPending,
				},
			},
		}
		status := contracts.LogStatusRunning
		tailLogLine := contracts.TailLogLine{
			Step:   "stage-a",
			Status: &status,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, contracts.LogStatusRunning, pipelineRunner.buildLogSteps[0].Status)
	})

	t.Run("SetStatusForNestedStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step:   "nested-stage-0",
							Status: contracts.LogStatusPending,
						},
					},
				},
			},
		}
		status := contracts.LogStatusRunning
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        contracts.LogTypeStage,
			Status:      &status,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, contracts.LogStatusRunning, pipelineRunner.buildLogSteps[0].NestedSteps[0].Status)
	})

	t.Run("SetStatusForNestedService", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step:   "nested-service-0",
							Status: contracts.LogStatusPending,
						},
					},
				},
			},
		}
		status := contracts.LogStatusRunning
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Type:        contracts.LogTypeService,
			Status:      &status,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, contracts.LogStatusRunning, pipelineRunner.buildLogSteps[0].Services[0].Status)
	})

	t.Run("NestsParallelStageMessages", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{},
		}

		statusRunning := contracts.LogStatusRunning
		statusPending := contracts.LogStatusPending
		statusSucceeded := contracts.LogStatusSucceeded

		// stage-a start
		tailLogLine := contracts.TailLogLine{
			Step:   "stage-a",
			Type:   contracts.LogTypeStage,
			Status: &statusRunning,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		// nested-stage-1
		tailLogLine = contracts.TailLogLine{
			Step:        "nested-stage-1",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        contracts.LogTypeStage,
			Status:      &statusPending,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		tailLogLine = contracts.TailLogLine{
			Step:        "nested-stage-1",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        contracts.LogTypeStage,
			Status:      &statusRunning,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		tailLogLine = contracts.TailLogLine{
			Step:        "nested-stage-1",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        contracts.LogTypeStage,
			Status:      &statusSucceeded,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		// nested-stage-0
		tailLogLine = contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        contracts.LogTypeStage,
			Status:      &statusPending,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		tailLogLine = contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        contracts.LogTypeStage,
			Status:      &statusRunning,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		tailLogLine = contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        contracts.LogTypeStage,
			Status:      &statusSucceeded,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		// stage-a finish
		tailLogLine = contracts.TailLogLine{
			Step:   "stage-a",
			Type:   contracts.LogTypeStage,
			Status: &statusSucceeded,
		}
		pipelineRunner.upsertTailLogLine(tailLogLine)

		if assert.Equal(t, 1, len(pipelineRunner.buildLogSteps)) {
			assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
			assert.Equal(t, contracts.LogStatusSucceeded, pipelineRunner.buildLogSteps[0].Status)

			assert.Equal(t, 2, len(pipelineRunner.buildLogSteps[0].NestedSteps))

			assert.Equal(t, "nested-stage-1", pipelineRunner.buildLogSteps[0].NestedSteps[0].Step)
			assert.Equal(t, contracts.LogStatusSucceeded, pipelineRunner.buildLogSteps[0].NestedSteps[0].Status)

			assert.Equal(t, "nested-stage-0", pipelineRunner.buildLogSteps[0].NestedSteps[1].Step)
			assert.Equal(t, contracts.LogStatusSucceeded, pipelineRunner.buildLogSteps[0].NestedSteps[1].Status)
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
					Status: contracts.LogStatusRunning,
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
					Status: contracts.LogStatusPending,
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
					Status: contracts.LogStatusSucceeded,
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
					Status: contracts.LogStatusFailed,
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
					Status: contracts.LogStatusSkipped,
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
					Status: contracts.LogStatusCanceled,
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
					Status: contracts.LogStatusSucceeded,
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
					Status: contracts.LogStatusFailed,
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
					Status: contracts.LogStatusSkipped,
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
					Status: contracts.LogStatusCanceled,
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
					Status: contracts.LogStatusSucceeded,
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
					Status: contracts.LogStatusSucceeded,
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step:   "nested-stage",
							Status: contracts.LogStatusRunning,
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
					Status: contracts.LogStatusSucceeded,
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
					Status: contracts.LogStatusSucceeded,
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step:   "nested-service",
							Status: contracts.LogStatusRunning,
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

	t.Run("ReturnsFalseIfLastStepHasSucceededStatusButMultiStageServicesFromPreviousStagesHaveNotFinished", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "earlier-stage",
					Status: contracts.LogStatusSucceeded,
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step:   "nested-service-1",
							Status: contracts.LogStatusRunning,
						},
					},
				},
				&contracts.BuildLogStep{
					Step:   "last-stage",
					Status: contracts.LogStatusSucceeded,
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

	t.Run("ReturnsTrueIfLastStepHasSucceededStatusAndAllServicesFromPreviousStagesHaveFinished", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "earlier-stage",
					Status: contracts.LogStatusSucceeded,
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step:   "nested-service-1",
							Status: contracts.LogStatusRunning,
						},
					},
				},
				&contracts.BuildLogStep{
					Step:   "last-stage",
					Status: contracts.LogStatusSucceeded,
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
}

func getPipelineRunnerAndMocks(ctrl *gomock.Controller, containerRunner ContainerRunner) (chan contracts.TailLogLine, PipelineRunner) {

	secretHelper := crypt.NewSecretHelper("SazbwMf3NZxVVbBqQHebPcXCqrVn3DDp", false)
	envvarHelper := NewEnvvarHelper("TESTPREFIX_", secretHelper, obfuscator)
	whenEvaluator := NewWhenEvaluator(envvarHelper)

	tailLogsChannel := make(chan contracts.TailLogLine, 10000)
	pipelineRunner := NewPipelineRunner(envvarHelper, whenEvaluator, containerRunner, true, tailLogsChannel, foundation.ApplicationInfo{})

	return tailLogsChannel, pipelineRunner
}

func setDefaultMockExpectancies(containerRunnerMock *MockContainerRunner) {
	containerRunnerMock.EXPECT().IsImagePulled(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	containerRunnerMock.EXPECT().PullImage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	containerRunnerMock.EXPECT().GetImageSize(gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	containerRunnerMock.EXPECT().IsTrustedImage(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	containerRunnerMock.EXPECT().HasInjectedCredentials(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	containerRunnerMock.EXPECT().StartStageContainer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("abc", nil).AnyTimes()
	containerRunnerMock.EXPECT().StartServiceContainer(gomock.Any(), gomock.Any(), gomock.Any()).Return("abc", nil).AnyTimes()
	containerRunnerMock.EXPECT().TailContainerLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	containerRunnerMock.EXPECT().RunReadinessProbeContainer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	containerRunnerMock.EXPECT().CreateNetworks(gomock.Any()).Return(nil).AnyTimes()
	containerRunnerMock.EXPECT().DeleteNetworks(gomock.Any()).Return(nil).AnyTimes()
	containerRunnerMock.EXPECT().StopAllContainers(gomock.Any()).AnyTimes()
	containerRunnerMock.EXPECT().StopMultiStageServiceContainers(gomock.Any()).AnyTimes()
}
