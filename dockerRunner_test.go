package main

import (
	"context"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/client"
	contracts "github.com/estafette/estafette-ci-contracts"
	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/stretchr/testify/assert"
)

type dockerRunnerMockImpl struct {
	isImagePulledFunc                func(stageName string, containerImage string) bool
	pullImageFunc                    func(ctx context.Context, stageName string, containerImage string) error
	getImageSizeFunc                 func(containerImage string) (int64, error)
	startStageContainerFunc          func(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, stage manifest.EstafetteStage) (containerID string, err error)
	startServiceContainerFunc        func(ctx context.Context, envvars map[string]string, service manifest.EstafetteService) (containerID string, err error)
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

func (d *dockerRunnerMockImpl) StartStageContainer(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, stage manifest.EstafetteStage) (containerID string, err error) {
	if d.startStageContainerFunc == nil {
		return "abc", nil
	}
	return d.startStageContainerFunc(ctx, depth, runIndex, dir, envvars, stage)
}

func (d *dockerRunnerMockImpl) StartServiceContainer(ctx context.Context, envvars map[string]string, service manifest.EstafetteService) (containerID string, err error) {
	if d.startServiceContainerFunc == nil {
		return "abc", nil
	}
	return d.startServiceContainerFunc(ctx, envvars, service)
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

func TestGenerateEntrypointScript(t *testing.T) {

	t.Run("ReturnsVariablesForOneCommand", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "./templates",
			entrypointTargetDir:   "",
		}

		// act
		path, extension, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"go test ./..."}, false)

		assert.Nil(t, err)
		assert.True(t, strings.Contains(path, "estafette-entrypoint-"))
		assert.Equal(t, extension, "sh")

		bytes, err := ioutil.ReadFile(path)
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\n\necho -e \"\\x1b[38;5;250m> cat /estafette-entrypoint.sh &\\x1b[0m\"\ncat /estafette-entrypoint.sh\n\necho -e \"\\x1b[38;5;250m> exec go test ./...\\x1b[0m\"\nexec go test ./...", string(bytes))
	})

	t.Run("ReturnsVariablesForTwoOrMoreCommands", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "./templates",
			entrypointTargetDir:   "",
		}

		// act
		path, extension, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"go test ./...", "go build"}, false)

		assert.Nil(t, err)
		assert.True(t, strings.Contains(path, "estafette-entrypoint-"))
		assert.Equal(t, extension, "sh")

		bytes, err := ioutil.ReadFile(path)
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\n\necho -e \"\\x1b[38;5;250m> cat /estafette-entrypoint.sh &\\x1b[0m\"\ncat /estafette-entrypoint.sh\necho -e \"\\x1b[38;5;250m> go test ./... &\\x1b[0m\"\ngo test ./... &\ntrap \"kill $!; wait; exit\" 1 2 15\nwait\n\necho -e \"\\x1b[38;5;250m> exec go build\\x1b[0m\"\nexec go build", string(bytes))
	})

	t.Run("DoesNotRunVariableAssignmentInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "./templates",
			entrypointTargetDir:   "",
		}

		// act
		path, extension, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"go test ./...", "export MY_TITLE_2=abc", "echo $MY_TITLE_2", "go build"}, false)

		assert.Nil(t, err)
		assert.True(t, strings.Contains(path, "estafette-entrypoint-"))
		assert.Equal(t, extension, "sh")

		bytes, err := ioutil.ReadFile(path)
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\n\necho -e \"\\x1b[38;5;250m> cat /estafette-entrypoint.sh &\\x1b[0m\"\ncat /estafette-entrypoint.sh\necho -e \"\\x1b[38;5;250m> go test ./... &\\x1b[0m\"\ngo test ./... &\ntrap \"kill $!; wait; exit\" 1 2 15\nwait\necho -e \"\\x1b[38;5;250m> export MY_TITLE_2=abc\\x1b[0m\"\nexport MY_TITLE_2=abc\necho -e \"\\x1b[38;5;250m> echo $MY_TITLE_2 &\\x1b[0m\"\necho $MY_TITLE_2 &\ntrap \"kill $!; wait; exit\" 1 2 15\nwait\n\necho -e \"\\x1b[38;5;250m> exec go build\\x1b[0m\"\nexec go build", string(bytes))
	})

	t.Run("DoesNotRunCommandsWithOrInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "./templates",
			entrypointTargetDir:   "",
		}

		// act
		path, extension, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"false || true", "go build"}, false)

		assert.Nil(t, err)
		assert.True(t, strings.Contains(path, "estafette-entrypoint-"))
		assert.Equal(t, extension, "sh")

		bytes, err := ioutil.ReadFile(path)
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\n\necho -e \"\\x1b[38;5;250m> cat /estafette-entrypoint.sh &\\x1b[0m\"\ncat /estafette-entrypoint.sh\necho -e \"\\x1b[38;5;250m> false || true\\x1b[0m\"\nfalse || true\n\necho -e \"\\x1b[38;5;250m> exec go build\\x1b[0m\"\nexec go build", string(bytes))
	})

	t.Run("DoesNotRunCommandsWithAndInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "./templates",
			entrypointTargetDir:   "",
		}

		// act
		path, extension, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"false && true", "go build"}, false)

		assert.Nil(t, err)
		assert.True(t, strings.Contains(path, "estafette-entrypoint-"))
		assert.Equal(t, extension, "sh")

		bytes, err := ioutil.ReadFile(path)
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\n\necho -e \"\\x1b[38;5;250m> cat /estafette-entrypoint.sh &\\x1b[0m\"\ncat /estafette-entrypoint.sh\necho -e \"\\x1b[38;5;250m> false && true\\x1b[0m\"\nfalse && true\n\necho -e \"\\x1b[38;5;250m> exec go build\\x1b[0m\"\nexec go build", string(bytes))
	})

	t.Run("DoesNotRunCommandsWithPipeInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "./templates",
			entrypointTargetDir:   "",
		}

		// act
		path, extension, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"cat kubernetes.yaml | kubectl apply -f -", "kubectl rollout status deploy/myapp"}, false)

		assert.Nil(t, err)
		assert.True(t, strings.Contains(path, "estafette-entrypoint-"))
		assert.Equal(t, extension, "sh")

		bytes, err := ioutil.ReadFile(path)
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\n\necho -e \"\\x1b[38;5;250m> cat /estafette-entrypoint.sh &\\x1b[0m\"\ncat /estafette-entrypoint.sh\necho -e \"\\x1b[38;5;250m> cat kubernetes.yaml | kubectl apply -f -\\x1b[0m\"\ncat kubernetes.yaml | kubectl apply -f -\n\necho -e \"\\x1b[38;5;250m> exec kubectl rollout status deploy/myapp\\x1b[0m\"\nexec kubectl rollout status deploy/myapp", string(bytes))
	})

	t.Run("DoesNotRunCommandsWithChangeDirectoryInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "./templates",
			entrypointTargetDir:   "",
		}

		// act
		path, extension, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"cd subdir", "ls -latr"}, false)

		assert.Nil(t, err)
		assert.True(t, strings.Contains(path, "estafette-entrypoint-"))
		assert.Equal(t, extension, "sh")

		bytes, err := ioutil.ReadFile(path)
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\n\necho -e \"\\x1b[38;5;250m> cat /estafette-entrypoint.sh &\\x1b[0m\"\ncat /estafette-entrypoint.sh\necho -e \"\\x1b[38;5;250m> cd subdir\\x1b[0m\"\ncd subdir\n\necho -e \"\\x1b[38;5;250m> exec ls -latr\\x1b[0m\"\nexec ls -latr", string(bytes))
	})

	t.Run("DoesNotRunCommandsWithSemicolonInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "./templates",
			entrypointTargetDir:   "",
		}

		// act
		path, extension, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"if [ \"${VARIABLE}\" -ne \"\" ]; then echo $VARIABLE; fi", "go build"}, false)

		assert.Nil(t, err)
		assert.True(t, strings.Contains(path, "estafette-entrypoint-"))
		assert.Equal(t, extension, "sh")

		bytes, err := ioutil.ReadFile(path)
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\n\necho -e \"\\x1b[38;5;250m> cat /estafette-entrypoint.sh &\\x1b[0m\"\ncat /estafette-entrypoint.sh\necho -e \"\\x1b[38;5;250m> if [ \"${VARIABLE}\" -ne \"\" ]; then echo $VARIABLE; fi\\x1b[0m\"\nif [ \"${VARIABLE}\" -ne \"\" ]; then echo $VARIABLE; fi\n\necho -e \"\\x1b[38;5;250m> exec go build\\x1b[0m\"\nexec go build", string(bytes))
	})

	t.Run("DoesNotRunAnyCommandInBackgroundWhenRunCommandsInForegroundIsTrue", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "./templates",
			entrypointTargetDir:   "",
		}

		// act
		path, extension, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"go test ./...", "go build"}, true)

		assert.Nil(t, err)
		assert.True(t, strings.Contains(path, "estafette-entrypoint-"))
		assert.Equal(t, extension, "sh")

		bytes, err := ioutil.ReadFile(path)
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\n\necho -e \"\\x1b[38;5;250m> cat /estafette-entrypoint.sh &\\x1b[0m\"\ncat /estafette-entrypoint.sh\necho -e \"\\x1b[38;5;250m> go test ./...\\x1b[0m\"\ngo test ./...\n\necho -e \"\\x1b[38;5;250m> go build\\x1b[0m\"\ngo build", string(bytes))
	})
}

func getDockerRunnerAndMocks() (chan contracts.TailLogLine, DockerRunner) {

	secretHelper := crypt.NewSecretHelper("SazbwMf3NZxVVbBqQHebPcXCqrVn3DDp", false)
	envvarHelper := NewEnvvarHelper("TESTPREFIX_", secretHelper, obfuscator)
	obfuscator := NewObfuscator(secretHelper)
	config := contracts.BuilderConfig{}
	tailLogsChannel := make(chan contracts.TailLogLine, 10000)

	dockerRunner := NewDockerRunner(envvarHelper, obfuscator, config, tailLogsChannel)

	return tailLogsChannel, dockerRunner
}
