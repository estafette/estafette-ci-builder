package main

import (
	"context"

	"github.com/docker/docker/client"
	manifest "github.com/estafette/estafette-ci-manifest"
)

type dockerRunnerMockImpl struct {
	isImagePulledFunc                func(stageName string, containerImage string) bool
	pullImageFunc                    func(ctx context.Context, stageName string, containerImage string) error
	getImageSizeFunc                 func(containerImage string) (int64, error)
	startStageContainerFunc          func(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error)
	startServiceContainerFunc        func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error)
	runReadinessProbeContainerFunc   func(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error)
	tailContainerLogsFunc            func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error)
	stopServiceContainersFunc        func(ctx context.Context, parentStage manifest.EstafetteStage)
	startDockerDaemonFunc            func() error
	waitForDockerDaemonFunc          func()
	createDockerClientFunc           func() (*client.Client, error)
	isTrustedImageFunc               func(stageName string, containerImage string) bool
	stopContainersOnCancellationFunc func()
	stopContainersFunc               func()
	createBridgeNetworkFunc          func(ctx context.Context) error
	deleteBridgeNetworkFunc          func(ctx context.Context) error
}

func (d *dockerRunnerMockImpl) IsImagePulled(stageName string, containerImage string) bool {
	if d.isImagePulledFunc == nil {
		return false
	}
	return d.isImagePulledFunc(stageName, containerImage)
}

func (d *dockerRunnerMockImpl) PullImage(ctx context.Context, stageName string, containerImage string) error {
	if d.pullImageFunc == nil {
		return nil
	}
	return d.pullImageFunc(ctx, stageName, containerImage)
}

func (d *dockerRunnerMockImpl) GetImageSize(containerImage string) (int64, error) {
	if d.getImageSizeFunc == nil {
		return 0, nil
	}
	return d.getImageSizeFunc(containerImage)
}

func (d *dockerRunnerMockImpl) StartStageContainer(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error) {
	if d.startStageContainerFunc == nil {
		return "abc", nil
	}
	return d.startStageContainerFunc(ctx, depth, runIndex, dir, envvars, parentStage, p)
}

func (d *dockerRunnerMockImpl) StartServiceContainer(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
	if d.startServiceContainerFunc == nil {
		return "abc", nil
	}
	return d.startServiceContainerFunc(ctx, envvars, parentStage, service)
}

func (d *dockerRunnerMockImpl) RunReadinessProbeContainer(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
	if d.runReadinessProbeContainerFunc == nil {
		return nil
	}
	return d.runReadinessProbeContainerFunc(ctx, parentStage, service, readiness)
}

func (d *dockerRunnerMockImpl) TailContainerLogs(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
	if d.tailContainerLogsFunc == nil {
		return nil
	}
	return d.tailContainerLogsFunc(ctx, containerID, parentStageName, stageName, stageType, depth, runIndex)
}

func (d *dockerRunnerMockImpl) StopServiceContainers(ctx context.Context, parentStage manifest.EstafetteStage) {
	if d.stopServiceContainersFunc != nil {
		d.stopServiceContainersFunc(ctx, parentStage)
	}
}

func (d *dockerRunnerMockImpl) StartDockerDaemon() error {
	if d.startDockerDaemonFunc == nil {
		return nil
	}
	return d.startDockerDaemonFunc()
}

func (d *dockerRunnerMockImpl) WaitForDockerDaemon() {
	if d.waitForDockerDaemonFunc != nil {
		d.waitForDockerDaemonFunc()
	}
}

func (d *dockerRunnerMockImpl) CreateDockerClient() (*client.Client, error) {
	if d.createDockerClientFunc != nil {
		return nil, nil
	}
	return d.createDockerClientFunc()
}

func (d *dockerRunnerMockImpl) IsTrustedImage(stageName string, containerImage string) bool {
	if d.isTrustedImageFunc == nil {
		return false
	}
	return d.isTrustedImageFunc(stageName, containerImage)
}

func (d *dockerRunnerMockImpl) StopContainersOnCancellation() {
	if d.stopContainersOnCancellationFunc != nil {
		d.stopContainersOnCancellationFunc()
	}
}

func (d *dockerRunnerMockImpl) StopContainers() {
	if d.stopContainersFunc != nil {
		d.stopContainersFunc()
	}
}

func (d *dockerRunnerMockImpl) CreateBridgeNetwork(ctx context.Context) error {
	if d.createBridgeNetworkFunc == nil {
		return nil
	}
	return d.createBridgeNetworkFunc(ctx)
}

func (d *dockerRunnerMockImpl) DeleteBridgeNetwork(ctx context.Context) error {
	if d.deleteBridgeNetworkFunc == nil {
		return nil
	}
	return d.deleteBridgeNetworkFunc(ctx)
}
