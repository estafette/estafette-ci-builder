package builder

import (
	"context"

	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
)

// ContainerRunner allows containers to be started
//go:generate mockgen -package=builder -destination ./container_runner_mock.go -source=container_runner.go
type ContainerRunner interface {
	IsImagePulled(ctx context.Context, stageName string, containerImage string) bool
	IsTrustedImage(stageName string, containerImage string) bool
	HasInjectedCredentials(stageName string, containerImage string) bool
	PullImage(ctx context.Context, stageName string, containerImage string) error
	GetImageSize(ctx context.Context, containerImage string) (int64, error)
	StartStageContainer(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, stage manifest.EstafetteStage, stageIndex int) (containerID string, err error)
	StartServiceContainer(ctx context.Context, envvars map[string]string, service manifest.EstafetteService) (containerID string, err error)
	RunReadinessProbeContainer(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error)
	TailContainerLogs(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth, runIndex int, multiStage *bool) (err error)
	StopSingleStageServiceContainers(ctx context.Context, parentStage manifest.EstafetteStage)
	StopMultiStageServiceContainers(ctx context.Context)
	StartDockerDaemon() error
	WaitForDockerDaemon()
	CreateDockerClient() error
	CreateNetworks(ctx context.Context) error
	DeleteNetworks(ctx context.Context) error
	StopAllContainers(ctx context.Context)
	Info(ctx context.Context) string
}
