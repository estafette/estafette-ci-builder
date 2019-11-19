package main

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
)

var (
	tailLogsChannel = make(chan contracts.TailLogLine, 10000)
	dockerRunner    = NewDockerRunner(envvarHelper, NewObfuscator(secretHelper), true, contracts.BuilderConfig{DockerNetwork: &contracts.DockerNetworkConfig{Name: "estafette-integration", Subnet: "192.168.4.1/24", Gateway: "192.168.4.1"}}, make(chan struct{}), tailLogsChannel)
)

func resetChannel(channel chan contracts.TailLogLine) {
	for len(channel) > 0 {
		<-channel
	}
}

func init() {
	dockerRunner.createDockerClient()
}

type dockerRunnerMockImpl struct {
	isImagePulledFunc               func(stageName string, containerImage string) bool
	pullImageFunc                   func(ctx context.Context, stageName string, containerImage string) error
	getImageSizeFunc                func(containerImage string) (int64, error)
	startStageContainerFunc         func(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error)
	startServiceContainerFunc       func(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error)
	runReadinessProbeContainerFunc  func(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error)
	tailContainerLogsFunc           func(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error)
	stopServiceContainersFunc       func(ctx context.Context, parentStage *manifest.EstafetteStage, services []*manifest.EstafetteService)
	startDockerDaemonFunc           func() error
	waitForDockerDaemonFunc         func()
	createDockerClientFunc          func() (*client.Client, error)
	getImagePullOptionsFunc         func(containerImage string) types.ImagePullOptions
	isTrustedImageFunc              func(stageName string, containerImage string) bool
	stopContainerOnCancellationFunc func()
	deleteContainerIDFunc           func(containerID string)
	removeContainerFunc             func(containerID string) error
	createBridgeNetworkFunc         func(ctx context.Context) error
	deleteBridgeNetworkFunc         func(ctx context.Context) error
}

func (d *dockerRunnerMockImpl) isImagePulled(stageName string, containerImage string) bool {
	if d.isImagePulledFunc == nil {
		return false
	}
	return d.isImagePulledFunc(stageName, containerImage)
}
func (d *dockerRunnerMockImpl) pullImage(ctx context.Context, stageName string, containerImage string) error {
	if d.pullImageFunc == nil {
		return nil
	}
	return d.pullImageFunc(ctx, stageName, containerImage)
}
func (d *dockerRunnerMockImpl) getImageSize(containerImage string) (int64, error) {
	if d.getImageSizeFunc == nil {
		return 0, nil
	}
	return d.getImageSizeFunc(containerImage)
}
func (d *dockerRunnerMockImpl) startStageContainer(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error) {
	if d.startStageContainerFunc == nil {
		return "abc", nil
	}
	return d.startStageContainerFunc(ctx, depth, runIndex, dir, envvars, parentStage, p)
}
func (d *dockerRunnerMockImpl) startServiceContainer(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {
	if d.startServiceContainerFunc == nil {
		return "abc", nil
	}
	return d.startServiceContainerFunc(ctx, envvars, parentStage, service)
}
func (d *dockerRunnerMockImpl) runReadinessProbeContainer(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
	if d.runReadinessProbeContainerFunc == nil {
		return nil
	}
	return d.runReadinessProbeContainerFunc(ctx, parentStage, service, readiness)
}
func (d *dockerRunnerMockImpl) tailContainerLogs(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (err error) {
	if d.tailContainerLogsFunc == nil {
		return nil
	}
	return d.tailContainerLogsFunc(ctx, containerID, parentStageName, stageName, stageType, depth, runIndex)
}
func (d *dockerRunnerMockImpl) stopServiceContainers(ctx context.Context, parentStage *manifest.EstafetteStage, services []*manifest.EstafetteService) {
	if d.stopServiceContainersFunc != nil {
		d.stopServiceContainersFunc(ctx, parentStage, services)
	}
}
func (d *dockerRunnerMockImpl) startDockerDaemon() error {
	if d.startDockerDaemonFunc == nil {
		return nil
	}
	return d.startDockerDaemonFunc()
}
func (d *dockerRunnerMockImpl) waitForDockerDaemon() {
	if d.waitForDockerDaemonFunc != nil {
		d.waitForDockerDaemonFunc()
	}
}
func (d *dockerRunnerMockImpl) createDockerClient() (*client.Client, error) {
	if d.createDockerClientFunc != nil {
		return nil, nil
	}
	return d.createDockerClientFunc()
}
func (d *dockerRunnerMockImpl) getImagePullOptions(containerImage string) types.ImagePullOptions {
	if d.getImagePullOptionsFunc == nil {
		return types.ImagePullOptions{}
	}
	return d.getImagePullOptionsFunc(containerImage)
}
func (d *dockerRunnerMockImpl) isTrustedImage(stageName string, containerImage string) bool {
	if d.isTrustedImageFunc == nil {
		return false
	}
	return d.isTrustedImageFunc(stageName, containerImage)
}
func (d *dockerRunnerMockImpl) stopContainerOnCancellation() {
	if d.stopContainerOnCancellationFunc != nil {
		d.stopContainerOnCancellationFunc()
	}
}
func (d *dockerRunnerMockImpl) deleteContainerID(containerID string) {
	if d.deleteContainerIDFunc != nil {
		d.deleteContainerIDFunc(containerID)
	}
}
func (d *dockerRunnerMockImpl) removeContainer(containerID string) error {
	if d.removeContainerFunc == nil {
		return nil
	}
	return d.removeContainerFunc(containerID)
}
func (d *dockerRunnerMockImpl) createBridgeNetwork(ctx context.Context) error {
	if d.createBridgeNetworkFunc == nil {
		return nil
	}
	return d.createBridgeNetworkFunc(ctx)
}
func (d *dockerRunnerMockImpl) deleteBridgeNetwork(ctx context.Context) error {
	if d.deleteBridgeNetworkFunc == nil {
		return nil
	}
	return d.deleteBridgeNetworkFunc(ctx)
}

func (d *dockerRunnerMockImpl) reset() {

	d.isImagePulledFunc = nil
	d.pullImageFunc = nil
	d.getImageSizeFunc = nil
	d.startStageContainerFunc = nil
	d.startServiceContainerFunc = nil
	d.runReadinessProbeContainerFunc = nil
	d.tailContainerLogsFunc = nil
	d.stopServiceContainersFunc = nil
	d.startDockerDaemonFunc = nil
	d.waitForDockerDaemonFunc = nil
	d.createDockerClientFunc = nil
	d.getImagePullOptionsFunc = nil
	d.isTrustedImageFunc = nil
	d.stopContainerOnCancellationFunc = nil
	d.deleteContainerIDFunc = nil
	d.removeContainerFunc = nil
	d.createBridgeNetworkFunc = nil
	d.deleteBridgeNetworkFunc = nil
}
