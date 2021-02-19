package docker

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"

	"github.com/estafette/estafette-ci-builder/clients/envvar"
	"github.com/estafette/estafette-ci-builder/clients/obfuscation"
	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

// Client pulls and runs docker containers
//go:generate mockgen -package=docker -destination ./mock.go -source=client.go
type Client interface {
	IsImagePulled(stageName string, containerImage string) bool
	IsTrustedImage(stageName string, containerImage string) bool
	HasInjectedCredentials(stageName string, containerImage string) bool
	PullImage(ctx context.Context, stageName string, containerImage string) error
	GetImageSize(containerImage string) (int64, error)
	StartStageContainer(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, stage manifest.EstafetteStage) (containerID string, err error)
	StartServiceContainer(ctx context.Context, envvars map[string]string, service manifest.EstafetteService) (containerID string, err error)
	RunReadinessProbeContainer(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error)
	TailContainerLogs(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth, runIndex int, multiStage *bool) (err error)
	StopSingleStageServiceContainers(ctx context.Context, parentStage manifest.EstafetteStage)
	StopMultiStageServiceContainers(ctx context.Context)
	StartDockerDaemon() error
	WaitForDockerDaemon()
	CreateDockerClient() (*dockerclient.Client, error)
	CreateNetworks(ctx context.Context) error
	DeleteNetworks(ctx context.Context) error
	StopAllContainers()
}

// NewClient returns a new Client
func NewClient(ctx context.Context, envvarClient envvar.Client, obfuscationClient obfuscation.Client, config contracts.BuilderConfig, tailLogsChannel chan contracts.TailLogLine) (Client, error) {
	return &client{
		envvarClient:                          envvarClient,
		obfuscationClient:                     obfuscationClient,
		config:                                config,
		tailLogsChannel:                       tailLogsChannel,
		runningStageContainerIDs:              make([]string, 0),
		runningSingleStageServiceContainerIDs: make([]string, 0),
		runningMultiStageServiceContainerIDs:  make([]string, 0),
		runningReadinessProbeContainerIDs:     make([]string, 0),
		networks:                              map[string]string{},
		entrypointTemplateDir:                 "/entrypoint-templates",
	}, nil
}

type client struct {
	envvarClient      envvar.Client
	obfuscationClient obfuscation.Client
	dockerClient      *dockerclient.Client
	config            contracts.BuilderConfig
	tailLogsChannel   chan contracts.TailLogLine

	runningStageContainerIDs              []string
	runningSingleStageServiceContainerIDs []string
	runningMultiStageServiceContainerIDs  []string
	runningReadinessProbeContainerIDs     []string
	// networkBridge                         string
	// networkBridgeID                       string
	networks              map[string]string
	entrypointTemplateDir string
}

func (c *client) IsImagePulled(stageName string, containerImage string) bool {

	log.Info().Msgf("[%v] Checking if docker image '%v' exists locally...", stageName, containerImage)

	imageSummaries, err := c.dockerClient.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		return false
	}

	for _, summary := range imageSummaries {
		if foundation.StringArrayContains(summary.RepoTags, containerImage) {
			return true
		}
	}

	return false
}

func (c *client) PullImage(ctx context.Context, stageName string, containerImage string) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "PullImage")
	defer span.Finish()
	span.SetTag("docker-image", containerImage)

	log.Info().Msgf("[%v] Pulling docker image '%v'", stageName, containerImage)

	rc, err := c.dockerClient.ImagePull(context.Background(), containerImage, c.getImagePullOptions(containerImage))
	if err != nil {
		return err
	}
	defer rc.Close()

	// wait for image pull to finish
	_, err = ioutil.ReadAll(rc)
	if err != nil {
		return err
	}

	return
}

func (c *client) GetImageSize(containerImage string) (totalSize int64, err error) {

	items, err := c.dockerClient.ImageHistory(context.Background(), containerImage)
	if err != nil {
		return totalSize, err
	}

	for _, item := range items {
		totalSize += item.Size
	}

	return totalSize, nil
}

func (c *client) StartStageContainer(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, stage manifest.EstafetteStage) (containerID string, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "StartStageContainer")
	defer span.Finish()
	span.SetTag("docker-image", stage.ContainerImage)

	// check if image is trusted image
	trustedImage := c.config.GetTrustedImage(stage.ContainerImage)

	entrypoint, cmds, binds, err := c.initContainerStartVariables(stage.Shell, stage.Commands, stage.RunCommandsInForeground, stage.CustomProperties, trustedImage)
	if err != nil {
		return
	}

	// add custom properties as ESTAFETTE_EXTENSION_... envvar
	extensionEnvVars := c.generateExtensionEnvvars(stage.CustomProperties, stage.EnvVars)

	// add stage name to envvars
	if stage.EnvVars == nil {
		stage.EnvVars = map[string]string{}
	}
	stage.EnvVars["ESTAFETTE_STAGE_NAME"] = stage.Name

	// combine and override estafette and global envvars with stage envvars
	combinedEnvVars := c.envvarClient.OverrideEnvvars(envvars, stage.EnvVars, extensionEnvVars)

	// decrypt secrets in all envvars
	combinedEnvVars = c.envvarClient.DecryptSecrets(combinedEnvVars, c.envvarClient.GetPipelineName())

	// define docker envvars and expand ESTAFETTE_ variables
	dockerEnvVars := make([]string, 0)
	if combinedEnvVars != nil && len(combinedEnvVars) > 0 {
		for k, v := range combinedEnvVars {
			dockerEnvVars = append(dockerEnvVars, fmt.Sprintf("%v=%v", k, os.Expand(v, c.envvarClient.GetEstafetteEnv)))
		}
	}

	// define binds
	binds = append(binds, fmt.Sprintf("%v:%v", dir, os.Expand(stage.WorkingDirectory, c.envvarClient.GetEstafetteEnv)))

	// check if this is a trusted image with RunDocker set to true
	if trustedImage != nil && trustedImage.RunDocker {
		if runtime.GOOS == "windows" {
			if foundation.PathExists(`\\.\pipe\docker_engine`) {
				binds = append(binds, `\\.\pipe\docker_engine:\\.\pipe\docker_engine`)
			}
			if foundation.PathExists("C:/Program Files/Docker") {
				binds = append(binds, "C:/Program Files/Docker:C:/dod")
			}
		} else {
			if foundation.PathExists("/var/run/docker.sock") {
				binds = append(binds, "/var/run/docker.sock:/var/run/docker.sock")
			}
			if foundation.PathExists("/usr/local/bin/docker") {
				binds = append(binds, "/usr/local/bin/docker:/dod/docker")
			}
		}
	}

	// define config
	config := container.Config{
		AttachStdout: true,
		AttachStderr: true,
		Env:          dockerEnvVars,
		Image:        stage.ContainerImage,
		WorkingDir:   os.Expand(stage.WorkingDirectory, c.envvarClient.GetEstafetteEnv),
	}
	if len(stage.Commands) > 0 {
		if trustedImage != nil && !trustedImage.AllowCommands && len(trustedImage.InjectedCredentialTypes) > 0 {
			// return stage as failed with error message indicating that this trusted image doesn't allow commands
			err = fmt.Errorf("This trusted image does not allow for commands to be set as a protection against snooping injected credentials")
			return
		}

		// only override entrypoint when commands are set, so extensions can work without commands
		config.Entrypoint = entrypoint
		// only pass commands when they are set, so extensions can work without
		config.Cmd = cmds
	}
	if trustedImage != nil && trustedImage.RunDocker {
		if runtime.GOOS != "windows" {
			currentUser, err := user.Current()
			if err == nil && currentUser != nil {
				config.User = fmt.Sprintf("%v:%v", currentUser.Uid, currentUser.Gid)
				log.Debug().Msgf("Setting docker user to %v", config.User)
			} else {
				log.Debug().Err(err).Msg("Can't retrieve current user")
			}
		} else {
			log.Debug().Msg("Not setting docker user for windows")
		}
	}

	// check if this is a trusted image with RunPrivileged or RunDocker set to true
	privileged := false
	if trustedImage != nil && runtime.GOOS != "windows" {
		privileged = trustedImage.RunDocker || trustedImage.RunPrivileged
	}

	// create container
	resp, err := c.dockerClient.ContainerCreate(ctx, &config, &container.HostConfig{
		Binds:      binds,
		Privileged: privileged,
		AutoRemove: true,
		LogConfig: container.LogConfig{
			Type: "local",
			Config: map[string]string{
				"max-size": "20m",
				"max-file": "5",
				"compress": "true",
				"mode":     "non-blocking",
			},
		},
	}, &network.NetworkingConfig{}, nil, "")
	if err != nil {
		return "", err
	}

	// connect to any configured networks
	for networkName, networkID := range c.networks {
		err = c.dockerClient.NetworkConnect(ctx, networkID, resp.ID, nil)
		if err != nil {
			log.Error().Err(err).Msgf("Failed connecting container %v to network %v with id %v", resp.ID, networkName, networkID)
			return
		}
	}

	containerID = resp.ID
	c.runningStageContainerIDs = c.addRunningContainerID(c.runningStageContainerIDs, containerID)

	// start container
	if err = c.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return
	}

	return
}

func (c *client) StartServiceContainer(ctx context.Context, envvars map[string]string, service manifest.EstafetteService) (containerID string, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "StartServiceContainer")
	defer span.Finish()
	span.SetTag("docker-image", service.ContainerImage)

	// check if image is trusted image
	trustedImage := c.config.GetTrustedImage(service.ContainerImage)

	entrypoint, cmds, binds, err := c.initContainerStartVariables(service.Shell, service.Commands, service.RunCommandsInForeground, service.CustomProperties, trustedImage)
	if err != nil {
		return
	}

	// add custom properties as ESTAFETTE_EXTENSION_... envvar
	extensionEnvVars := c.generateExtensionEnvvars(service.CustomProperties, service.EnvVars)

	// add service name to envvars
	if service.EnvVars == nil {
		service.EnvVars = map[string]string{}
	}
	service.EnvVars["ESTAFETTE_SERVICE_NAME"] = service.Name

	// combine and override estafette and global envvars with pipeline envvars
	combinedEnvVars := c.envvarClient.OverrideEnvvars(envvars, service.EnvVars, extensionEnvVars)

	// decrypt secrets in all envvars
	combinedEnvVars = c.envvarClient.DecryptSecrets(combinedEnvVars, c.envvarClient.GetPipelineName())

	// define docker envvars and expand ESTAFETTE_ variables
	dockerEnvVars := make([]string, 0)
	if combinedEnvVars != nil && len(combinedEnvVars) > 0 {
		for k, v := range combinedEnvVars {
			dockerEnvVars = append(dockerEnvVars, fmt.Sprintf("%v=%v", k, os.Expand(v, c.envvarClient.GetEstafetteEnv)))
		}
	}

	// check if this is a trusted image with RunDocker set to true
	if trustedImage != nil && trustedImage.RunDocker {
		if runtime.GOOS == "windows" {
			if foundation.PathExists(`\\.\pipe\docker_engine`) {
				binds = append(binds, `\\.\pipe\docker_engine:\\.\pipe\docker_engine`)
			}
			if foundation.PathExists("C:/Program Files/Docker") {
				binds = append(binds, "C:/Program Files/Docker:C:/dod")
			}
		} else {
			if foundation.PathExists("/var/run/docker.sock") {
				binds = append(binds, "/var/run/docker.sock:/var/run/docker.sock")
			}
			if foundation.PathExists("/usr/local/bin/docker") {
				binds = append(binds, "/usr/local/bin/docker:/dod/docker")
			}
		}
	}

	// define config
	config := container.Config{
		AttachStdout: true,
		AttachStderr: true,
		Env:          dockerEnvVars,
		Image:        service.ContainerImage,
	}

	if len(service.Commands) > 0 {
		if trustedImage != nil && !trustedImage.AllowCommands && len(trustedImage.InjectedCredentialTypes) > 0 {
			// return stage as failed with error message indicating that this trusted image doesn't allow commands
			err = fmt.Errorf("This trusted image does not allow for commands to be set as a protection against snooping injected credentials")
			return
		}

		// only pass commands when they are set, so extensions can work without
		config.Cmd = cmds
		// only override entrypoint when commands are set, so extensions can work without commands
		config.Entrypoint = entrypoint
	}

	if trustedImage != nil && trustedImage.RunDocker {
		if runtime.GOOS != "windows" {
			currentUser, err := user.Current()
			if err == nil && currentUser != nil {
				config.User = fmt.Sprintf("%v:%v", currentUser.Uid, currentUser.Gid)
				log.Debug().Msgf("Setting docker user to %v", config.User)
			} else {
				log.Debug().Err(err).Msg("Can't retrieve current user")
			}
		} else {
			log.Debug().Msg("Not setting docker user for windows")
		}
	}

	// check if this is a trusted image with RunPrivileged or RunDocker set to true
	privileged := false
	if trustedImage != nil && runtime.GOOS != "windows" {
		privileged = trustedImage.RunDocker || trustedImage.RunPrivileged
	}

	// create container
	resp, err := c.dockerClient.ContainerCreate(ctx, &config, &container.HostConfig{
		Binds:      binds,
		Privileged: privileged,
		AutoRemove: true,
		LogConfig: container.LogConfig{
			Type: "local",
			Config: map[string]string{
				"max-size": "20m",
				"max-file": "5",
				"compress": "true",
				"mode":     "non-blocking",
			},
		},
	}, &network.NetworkingConfig{}, nil, service.Name)
	if err != nil {
		return
	}

	// connect to any configured networks
	for networkName, networkID := range c.networks {
		err = c.dockerClient.NetworkConnect(ctx, networkID, resp.ID, nil)
		if err != nil {
			log.Error().Err(err).Msgf("Failed connecting container %v to network %v with id %v", resp.ID, networkName, networkID)
			return
		}
	}

	containerID = resp.ID
	if service.MultiStage != nil && *service.MultiStage {
		c.runningMultiStageServiceContainerIDs = c.addRunningContainerID(c.runningMultiStageServiceContainerIDs, containerID)
	} else {
		c.runningSingleStageServiceContainerIDs = c.addRunningContainerID(c.runningSingleStageServiceContainerIDs, containerID)
	}

	// start container
	if err = c.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return
	}

	return
}

func (c *client) RunReadinessProbeContainer(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "RunReadinessProbeContainer")
	defer span.Finish()

	readinessProberImage := "estafette/scratch:latest"
	isPulled := c.IsImagePulled(service.Name+"-prober", readinessProberImage)
	if !isPulled {
		err = c.PullImage(ctx, service.Name+"-prober", readinessProberImage)
		if err != nil {
			return err
		}
	}

	envvars := map[string]string{
		"RUN_AS_READINESS_PROBE":    "true",
		"READINESS_PROTOCOL":        readiness.Protocol,
		"READINESS_HOST":            service.Name,
		"READINESS_PORT":            strconv.Itoa(readiness.Port),
		"READINESS_PATH":            readiness.Path,
		"READINESS_HOSTNAME":        readiness.Hostname,
		"READINESS_TIMEOUT_SECONDS": strconv.Itoa(readiness.TimeoutSeconds),
		"ESTAFETTE_LOG_FORMAT":      "console",
	}

	// decrypt secrets in all envvars
	envvars = c.envvarClient.DecryptSecrets(envvars, c.envvarClient.GetPipelineName())

	// define docker envvars and expand ESTAFETTE_ variables
	dockerEnvVars := make([]string, 0)
	if envvars != nil && len(envvars) > 0 {
		for k, v := range envvars {
			dockerEnvVars = append(dockerEnvVars, fmt.Sprintf("%v=%v", k, os.Expand(v, c.envvarClient.GetEstafetteEnv)))
		}
	}

	// mount the builder binary and trusted certs into the image
	binds := make([]string, 0)
	binds = append(binds, "/estafette-ci-builder:/estafette-ci-builder")
	binds = append(binds, "/etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt")

	// define config
	config := container.Config{
		AttachStdout: true,
		AttachStderr: true,
		Entrypoint:   []string{"/estafette-ci-builder"},
		Env:          dockerEnvVars,
		Image:        readinessProberImage,
	}

	// create container
	resp, err := c.dockerClient.ContainerCreate(ctx, &config, &container.HostConfig{
		Binds:      binds,
		AutoRemove: true,
		LogConfig: container.LogConfig{
			Type: "local",
			Config: map[string]string{
				"max-size": "20m",
				"max-file": "5",
				"compress": "true",
				"mode":     "non-blocking",
			},
		},
	}, &network.NetworkingConfig{}, nil, "")
	if err != nil {
		return
	}

	// connect to any configured networks
	for networkName, networkID := range c.networks {
		err = c.dockerClient.NetworkConnect(ctx, networkID, resp.ID, nil)
		if err != nil {
			log.Error().Err(err).Msgf("Failed connecting container %v to network %v with id %v", resp.ID, networkName, networkID)
			return
		}
	}

	containerID := resp.ID
	c.runningReadinessProbeContainerIDs = c.addRunningContainerID(c.runningReadinessProbeContainerIDs, containerID)

	// start container
	if err = c.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return
	}

	// follow logs
	rc, err := c.dockerClient.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: false,
		Follow:     true,
		Details:    false,
	})
	if err != nil {
		return err
	}
	defer rc.Close()

	// stream logs to stdout with buffering
	in := bufio.NewReader(rc)
	var readError error
	for {
		// strip first 8 bytes, they contain docker control characters (https://github.com/docker/docker-ce/blob/v18.06.1-ce/components/engine/client/container_logs.go#L23-L32)
		headers := make([]byte, 8)
		n, readError := in.Read(headers)
		if readError != nil {
			break
		}

		if n < 8 {
			// doesn't seem to be a valid header
			continue
		}

		// read the rest of the line until we hit end of line
		logLine, readError := in.ReadBytes('\n')
		if readError != nil {
			break
		}

		log.Debug().Msgf("[%v][%v] %v", parentStage.Name, service.Name, string(logLine))
	}

	if readError != nil && readError != io.EOF {
		return readError
	}

	// wait for container to stop running
	resultC, errC := c.dockerClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	var exitCode int64
	select {
	case result := <-resultC:
		exitCode = result.StatusCode
	case err = <-errC:
		return err
	}

	// clear container id
	c.runningReadinessProbeContainerIDs = c.removeRunningContainerID(c.runningReadinessProbeContainerIDs, containerID)

	if exitCode != 0 {
		return fmt.Errorf("Failed with exit code: %v", exitCode)
	}

	return
}

func (c *client) TailContainerLogs(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth, runIndex int, multiStage *bool) (err error) {

	lineNumber := 1

	// follow logs
	rc, err := c.dockerClient.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: false,
		Follow:     true,
		Details:    false,
	})
	if err != nil {
		return err
	}
	defer rc.Close()

	// stream logs to stdout with buffering
	in := bufio.NewReader(rc)
	var readError error
	for {
		// strip first 8 bytes, they contain docker control characters (https://github.com/docker/docker-ce/blob/v18.06.1-ce/components/engine/client/container_logs.go#L23-L32)
		headers := make([]byte, 8)
		n, readError := in.Read(headers)
		if readError != nil {
			break
		}

		if n < 8 {
			// doesn't seem to be a valid header
			continue
		}

		// inspect the docker log header for stream type

		// first byte contains the streamType
		// -   0: stdin (will be written on stdout)
		// -   1: stdout
		// -   2: stderr
		// -   3: system error
		streamType := ""
		switch headers[0] {
		case 1:
			streamType = "stdout"
		case 2:
			streamType = "stderr"
		default:
			continue
		}

		// read the rest of the line until we hit end of line
		logLine, readError := in.ReadBytes('\n')
		if readError != nil {
			break
		}

		// strip headers and obfuscate secret values
		logLineString := c.obfuscationClient.Obfuscate(string(logLine))

		// create object for tailing logs and storing in the db when done
		logLineObject := contracts.BuildLogLine{
			LineNumber: lineNumber,
			Timestamp:  time.Now().UTC(),
			StreamType: streamType,
			Text:       logLineString,
		}
		lineNumber++

		// log as json, to be tailed when looking at live logs from gui
		c.tailLogsChannel <- contracts.TailLogLine{
			Step:        stageName,
			ParentStage: parentStageName,
			Type:        stageType,
			Depth:       depth,
			RunIndex:    runIndex,
			LogLine:     &logLineObject,
		}
	}

	if readError != nil && readError != io.EOF {
		log.Error().Msgf("[%v] Error: %v", stageName, readError)
		return readError
	}

	// wait for container to stop running
	resultC, errC := c.dockerClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	var exitCode int64
	select {
	case result := <-resultC:
		exitCode = result.StatusCode
	case err = <-errC:
		log.Warn().Err(err).Msgf("Container %v exited with error", containerID)
		return err
	}

	// clear container id
	if stageType == contracts.LogTypeStage {
		c.runningStageContainerIDs = c.removeRunningContainerID(c.runningStageContainerIDs, containerID)
	} else if stageType == contracts.LogTypeService && multiStage != nil {
		if *multiStage {
			c.runningMultiStageServiceContainerIDs = c.removeRunningContainerID(c.runningMultiStageServiceContainerIDs, containerID)
		} else {
			c.runningSingleStageServiceContainerIDs = c.removeRunningContainerID(c.runningSingleStageServiceContainerIDs, containerID)
		}
	}

	if exitCode != 0 {
		return fmt.Errorf("Failed with exit code: %v", exitCode)
	}

	return err
}

func (c *client) StopSingleStageServiceContainers(ctx context.Context, parentStage manifest.EstafetteStage) {

	log.Info().Msgf("[%v] Stopping single-stage service containers...", parentStage.Name)

	// the service containers should be the only ones running, so just stop all containers
	c.stopContainers(c.runningSingleStageServiceContainerIDs)

	log.Info().Msgf("[%v] Stopped single-stage service containers...", parentStage.Name)
}

func (c *client) StopMultiStageServiceContainers(ctx context.Context) {

	log.Info().Msg("Stopping multi-stage service containers...")

	// the service containers should be the only ones running, so just stop all containers
	c.stopContainers(c.runningMultiStageServiceContainerIDs)

	log.Info().Msg("Stopped multi-stage service containers...")
}

func (c *client) StartDockerDaemon() error {
	if c.config.DockerConfig != nil && c.config.DockerConfig.RunType != contracts.DockerRunTypeDinD {
		return nil
	}

	// dockerd --host=unix:///var/run/docker.sock --host=tcp://0.0.0.0:2375 --mtu=1500 &
	log.Debug().Msg("Starting docker daemon...")
	args := []string{"--host=unix:///var/run/docker.sock", "--host=tcp://0.0.0.0:2375"}

	// if an mtu is configured pass it to the docker daemon
	if c.config.DockerConfig != nil && c.config.DockerConfig.RunType == contracts.DockerRunTypeDinD && c.config.DockerConfig.MTU > 0 {
		args = append(args, fmt.Sprintf("--mtu=%v", c.config.DockerConfig.MTU))
	}

	// if a bip is configured pass it to the docker daemon
	if c.config.DockerConfig != nil && c.config.DockerConfig.RunType == contracts.DockerRunTypeDinD && c.config.DockerConfig.BIP != "" {
		args = append(args, fmt.Sprintf("--bip=%v", c.config.DockerConfig.BIP))
	}

	// if a registry mirror is configured pass it to the docker daemon
	if c.config.DockerConfig != nil && c.config.DockerConfig.RunType == contracts.DockerRunTypeDinD && c.config.DockerConfig.RegistryMirror != "" {
		args = append(args, fmt.Sprintf("--registry-mirror=%v", c.config.DockerConfig.RegistryMirror))
	}

	dockerDaemonCommand := exec.Command("dockerd", args...)
	dockerDaemonCommand.Stdout = log.Logger
	dockerDaemonCommand.Stderr = log.Logger
	err := dockerDaemonCommand.Start()
	if err != nil {
		return err
	}

	return nil
}

func (c *client) WaitForDockerDaemon() {
	if c.config.DockerConfig != nil && c.config.DockerConfig.RunType != contracts.DockerRunTypeDinD {
		return
	}

	// wait until /var/run/docker.sock exists
	log.Debug().Msg("Waiting for docker daemon to be ready for use...")
	for {
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			// does not exist
			time.Sleep(1 * time.Second)
		} else {
			// file exists, break out of for loop
			break
		}
	}
	log.Debug().Msg("Docker daemon is ready for use")
}

func (c *client) CreateDockerClient() (*dockerclient.Client, error) {

	dockerClient, err := dockerclient.NewEnvClient()
	if err != nil {
		return dockerClient, err
	}
	c.dockerClient = dockerClient

	return dockerClient, err
}

func (c *client) getImagePullOptions(containerImage string) types.ImagePullOptions {

	containerRegistryCredentials := c.config.GetCredentialsByType("container-registry")

	if len(containerRegistryCredentials) > 0 {
		for _, credential := range containerRegistryCredentials {
			containerImageSlice := strings.Split(containerImage, "/")
			containerRepo := strings.Join(containerImageSlice[:len(containerImageSlice)-1], "/")

			if containerRepo == credential.AdditionalProperties["repository"].(string) {
				authConfig := types.AuthConfig{
					Username: credential.AdditionalProperties["username"].(string),
					Password: credential.AdditionalProperties["password"].(string),
				}
				encodedJSON, err := json.Marshal(authConfig)
				if err == nil {
					authStr := base64.URLEncoding.EncodeToString(encodedJSON)

					log.Debug().Msgf("Using credential '%v' to authenticate for image '%v'", credential.Name, containerImage)

					return types.ImagePullOptions{
						RegistryAuth: authStr,
					}
				}

				log.Error().Err(err).Msgf("Failed marshaling docker auth config for container image %v", containerImage)

				break
			}
		}
	}

	// when no container-registry credentials apply use container-registry-pull as fallback to avoid docker hub rate limiting issues
	containerRegistryPullCredentials := c.config.GetCredentialsByType("container-registry-pull")

	if len(containerRegistryPullCredentials) > 0 {
		// no real need to loop, since we only need one of these, but works anyway
		for _, credential := range containerRegistryPullCredentials {
			authConfig := types.AuthConfig{
				Username: credential.AdditionalProperties["username"].(string),
				Password: credential.AdditionalProperties["password"].(string),
			}
			encodedJSON, err := json.Marshal(authConfig)
			if err == nil {
				authStr := base64.URLEncoding.EncodeToString(encodedJSON)

				log.Debug().Msgf("Using credential '%v' to authenticate for image '%v'", credential.Name, containerImage)

				return types.ImagePullOptions{
					RegistryAuth: authStr,
				}
			}

			log.Error().Err(err).Msgf("Failed marshaling docker auth config for container image %v", containerImage)

			break
		}
	}

	return types.ImagePullOptions{}
}

func (c *client) IsTrustedImage(stageName string, containerImage string) bool {

	log.Info().Msgf("[%v] Checking if docker image '%v' is trusted...", stageName, containerImage)

	// check if image is trusted image
	trustedImage := c.config.GetTrustedImage(containerImage)

	return trustedImage != nil
}

func (c *client) HasInjectedCredentials(stageName string, containerImage string) bool {

	log.Info().Msgf("[%v] Checking if docker image '%v' has injected credentials...", stageName, containerImage)

	// check if image has injected credentials
	trustedImage := c.config.GetTrustedImage(containerImage)
	if trustedImage == nil {
		return false
	}

	credentialMap := c.config.GetCredentialsForTrustedImage(*trustedImage)

	return len(credentialMap) > 0
}

func (c *client) stopContainer(containerID string) error {

	log.Debug().Msgf("Stopping container with id %v", containerID)

	timeout := 20 * time.Second
	err := c.dockerClient.ContainerStop(context.Background(), containerID, &timeout)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed stopping container with id %v", containerID)
		return err
	}

	log.Info().Msgf("Stopped container with id %v", containerID)
	return nil
}

func (c *client) stopContainers(containerIDs []string) {

	if len(containerIDs) > 0 {
		log.Info().Msgf("Stopping %v containers", len(containerIDs))

		var wg sync.WaitGroup
		wg.Add(len(containerIDs))

		for _, id := range containerIDs {
			go func(id string) {
				defer wg.Done()
				c.stopContainer(id)
			}(id)
		}

		wg.Wait()

		log.Info().Msgf("Stopped %v containers", len(containerIDs))
	} else {
		log.Info().Msg("No containers to stop")
	}
}

func (c *client) StopAllContainers() {

	allRunningContainerIDs := append(c.runningStageContainerIDs, c.runningSingleStageServiceContainerIDs...)
	allRunningContainerIDs = append(allRunningContainerIDs, c.runningMultiStageServiceContainerIDs...)
	allRunningContainerIDs = append(allRunningContainerIDs, c.runningReadinessProbeContainerIDs...)

	c.stopContainers(allRunningContainerIDs)
}

func (c *client) addRunningContainerID(containerIDs []string, containerID string) []string {

	log.Debug().Msgf("Adding container id %v to containerIDs", containerID)

	return append(containerIDs, containerID)
}

func (c *client) removeRunningContainerID(containerIDs []string, containerID string) []string {

	log.Debug().Msgf("Removing container id %v from containerIDs", containerID)

	purgedContainerIDs := []string{}

	for _, id := range containerIDs {
		if id != containerID {
			purgedContainerIDs = append(purgedContainerIDs, id)
		}
	}

	return purgedContainerIDs
}

func (c *client) CreateNetworks(ctx context.Context) error {

	if c.config.DockerConfig != nil {

		// fetch existing networks, so we're not creating ones that already exist
		currentNetworks, err := c.dockerClient.NetworkList(ctx, types.NetworkListOptions{})
		if err != nil {
			return err
		}

		for _, nw := range c.config.DockerConfig.Networks {

			// check if network already exists
			networkExists := false
			for _, cnw := range currentNetworks {
				if cnw.Name == nw.Name {
					networkExists = true
					break
				}
			}

			if networkExists {
				log.Info().Msgf("Docker network %v already exists, no need to create it", nw.Name)
				continue
			}

			log.Info().Msgf("Creating docker network %v with values %v", nw.Name, nw)

			options := types.NetworkCreate{}
			if nw.Driver != "" {
				options.IPAM = &network.IPAM{
					Driver: nw.Driver,
				}

				if nw.Subnet != "" && nw.Gateway != "" {
					options.IPAM.Config = []network.IPAMConfig{
						{
							Subnet:  nw.Subnet,
							Gateway: nw.Gateway,
						},
					}
				}
			}

			resp, err := c.dockerClient.NetworkCreate(ctx, nw.Name, options)

			if err != nil {
				log.Error().Err(err).Msgf("Failed creating docker network %v", nw.Name)
				return err
			}

			c.networks[nw.Name] = resp.ID

			log.Info().Msgf("Succesfully created docker network %v with id %v", nw.Name, resp.ID)
		}
	}

	return nil
}

func (c *client) DeleteNetworks(ctx context.Context) error {

	for _, nw := range c.config.DockerConfig.Networks {
		if networkID, ok := c.networks[nw.Name]; ok && !nw.Durable {
			log.Info().Msgf("Deleting docker network %v with id %v...", nw.Name, networkID)

			err := c.dockerClient.NetworkRemove(ctx, networkID)
			if err != nil {
				log.Error().Err(err).Msgf("Failed deleting docker network %v with id %v", nw.Name, networkID)
				return err
			}
		}
	}

	return nil
}

func (c *client) generateEntrypointScript(shell string, commands []string, runCommandsInForeground bool) (hostPath, mountPath, entrypointFile string, err error) {

	r, _ := regexp.Compile("[a-zA-Z0-9_]+=|export|shopt|;|cd |\\||&&|\\|\\|")

	firstCommands := []struct {
		Command         string
		EscapedCommand  string
		RunInBackground bool
	}{}
	for _, c := range commands[:len(commands)-1] {
		// check if the command is assigning a value, in which case it shouldn't be run in the background
		match := r.MatchString(c)
		runInBackground := !runCommandsInForeground && !match

		firstCommands = append(firstCommands, struct {
			Command         string
			EscapedCommand  string
			RunInBackground bool
		}{c, escapeCharsInCommand(c), runInBackground})
	}

	lastCommand := commands[len(commands)-1]
	match := r.MatchString(lastCommand)
	runFinalCommandWithExec := !runCommandsInForeground && !match

	data := struct {
		Shell    string
		Commands []struct {
			Command         string
			EscapedCommand  string
			RunInBackground bool
		}
		FinalCommand            string
		EscapedFinalCommand     string
		RunFinalCommandWithExec bool
	}{
		shell,
		firstCommands,
		lastCommand,
		escapeCharsInCommand(lastCommand),
		runFinalCommandWithExec,
	}

	entrypointFile = "entrypoint.sh"
	if runtime.GOOS == "windows" && shell == "powershell" {
		entrypointFile = "entrypoint.ps1"
	} else if runtime.GOOS == "windows" && shell == "cmd" {
		entrypointFile = "entrypoint.bat"
	}

	entrypointdir, err := ioutil.TempDir("", "*-entrypoint")
	if err != nil {
		return
	}

	// set permissions on directory to avoid non-root containers not to be able to read from the mounted directory
	err = os.Chmod(entrypointdir, 0777)
	if err != nil {
		return
	}

	entrypointPath := path.Join(entrypointdir, entrypointFile)

	// read and parse template
	templatePath := path.Join(c.entrypointTemplateDir, entrypointFile)
	entrypointTemplate, err := template.ParseFiles(templatePath)
	if err != nil {
		return
	}

	targetFile, err := os.Create(entrypointPath)
	if err != nil {
		return
	}
	defer targetFile.Close()

	err = entrypointTemplate.Execute(targetFile, data)
	if err != nil {
		return
	}

	err = os.Chmod(entrypointPath, 0777)
	if err != nil {
		return
	}

	entryPointBytes, innerErr := ioutil.ReadFile(entrypointPath)
	if innerErr == nil {
		log.Debug().Str("entrypoint", string(entryPointBytes)).Msgf("Inspecting entrypoint script at %v", entrypointPath)
	}

	hostPath = entrypointdir
	mountPath = "/entrypoint"
	if runtime.GOOS == "windows" {
		hostPath = filepath.Join(c.envvarClient.GetTempDir(), strings.TrimPrefix(hostPath, "C:\\Windows\\TEMP"))
		mountPath = "C:" + mountPath
	}

	return
}

func (c *client) initContainerStartVariables(shell string, commands []string, runCommandsInForeground bool, customProperties map[string]interface{}, trustedImage *contracts.TrustedImageConfig) (entrypoint []string, cmds []string, binds []string, err error) {
	entrypoint = make([]string, 0)
	cmds = make([]string, 0)
	binds = make([]string, 0)

	if len(commands) > 0 {
		// generate entrypoint script
		entrypointHostPath, entrypointMountPath, entrypointFile, innerErr := c.generateEntrypointScript(shell, commands, runCommandsInForeground)
		if innerErr != nil {
			return entrypoint, cmds, binds, innerErr
		}

		// use generated entrypoint script for executing commands
		entrypointFilePath := path.Join(entrypointMountPath, entrypointFile)
		if runtime.GOOS == "windows" && shell == "powershell" {
			entrypointFilePath = fmt.Sprintf("C:\\entrypoint\\%v", entrypointFile)
			entrypoint = []string{"powershell.exe", entrypointFilePath}
		} else if runtime.GOOS == "windows" && shell == "cmd" {
			entrypointFilePath = fmt.Sprintf("C:\\entrypoint\\%v", entrypointFile)
			entrypoint = []string{"cmd.exe", "/C", entrypointFilePath}
		} else {
			entrypoint = []string{entrypointFilePath}
		}

		log.Debug().Interface("entrypoint", entrypoint).Msg("Inspecting entrypoint array")

		binds = append(binds, fmt.Sprintf("%v:%v", entrypointHostPath, entrypointMountPath))
	}

	// mount injected credentials as files
	credentialsHostPath, credentialsMountPath, err := c.generateCredentialsFiles(trustedImage)
	if err != nil {
		return
	}
	if credentialsHostPath != "" && credentialsMountPath != "" {
		binds = append(binds, fmt.Sprintf("%v:%v", credentialsHostPath, credentialsMountPath))
	}

	return
}

func (c *client) generateExtensionEnvvars(customProperties map[string]interface{}, envvars map[string]string) (extensionEnvVars map[string]string) {
	extensionEnvVars = map[string]string{}
	if customProperties != nil && len(customProperties) > 0 {
		for k, v := range customProperties {
			extensionkey := c.envvarClient.GetEstafetteEnvvarName(fmt.Sprintf("ESTAFETTE_EXTENSION_%v", foundation.ToUpperSnakeCase(k)))

			if s, isString := v.(string); isString {
				// if custom property is of type string add the envvar
				extensionEnvVars[extensionkey] = s
			} else if s, isBool := v.(bool); isBool {
				// if custom property is of type bool add the envvar
				extensionEnvVars[extensionkey] = strconv.FormatBool(s)
			} else if s, isInt := v.(int); isInt {
				// if custom property is of type bool add the envvar
				extensionEnvVars[extensionkey] = strconv.FormatInt(int64(s), 10)
			} else if s, isFloat := v.(float64); isFloat {
				// if custom property is of type bool add the envvar
				extensionEnvVars[extensionkey] = strconv.FormatFloat(float64(s), 'f', -1, 64)

			} else if i, isInterfaceArray := v.([]interface{}); isInterfaceArray {
				// check whether all array items are of type string
				valid := true
				stringValues := []string{}
				for _, iv := range i {
					if s, isString := iv.(string); isString {
						stringValues = append(stringValues, s)
					} else {
						valid = false
						break
					}
				}

				if valid {
					// if all array items are string, pass as comma-separated list to extension
					extensionEnvVars[extensionkey] = strings.Join(stringValues, ",")
				} else {
					log.Warn().Interface("customProperty", v).Msgf("Cannot turn custom property %v into extension envvar", k)
				}
			} else {
				log.Warn().Interface("customProperty", v).Msgf("Cannot turn custom property %v of type %v into extension envvar", k, reflect.TypeOf(v))
			}
		}

		// add envvar to custom properties
		customProperties := customProperties
		customProperties["env"] = envvars
	}

	// also add add custom properties as json object in ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES envvar
	customPropertiesBytes, err := json.Marshal(customProperties)
	if err == nil {
		extensionEnvVars["ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES"] = string(customPropertiesBytes)
	} else {
		log.Warn().Err(err).Interface("customProperty", customProperties).Msg("Cannot marshal custom properties for ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES envvar")
	}

	// also add add custom properties as json object in ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES_YAML envvar
	customPropertiesYamlBytes, err := yaml.Marshal(customProperties)
	if err == nil {
		extensionEnvVars["ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES_YAML"] = string(customPropertiesYamlBytes)
	} else {
		log.Warn().Err(err).Interface("customProperty", customProperties).Msg("Cannot marshal custom properties for ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES_YAML envvar")
	}

	return
}

func (c *client) generateCredentialsFiles(trustedImage *contracts.TrustedImageConfig) (hostPath, mountPath string, err error) {

	if trustedImage != nil {
		// create a tempdir to store credential files in and mount into container
		credentialsdir, innerErr := ioutil.TempDir("", "*-credentials")
		if innerErr != nil {
			return hostPath, mountPath, innerErr
		}

		// set permissions on directory to avoid non-root containers not to be able to read from the mounted directory
		err = os.Chmod(credentialsdir, 0777)
		if err != nil {
			return
		}

		credentialMap := c.config.GetCredentialsForTrustedImage(*trustedImage)
		if len(credentialMap) == 0 {
			credentialsdir = ""
			return
		}
		for credentialType, credentialsForType := range credentialMap {

			filename := fmt.Sprintf("%v.json", foundation.ToLowerSnakeCase(credentialType))
			filepath := path.Join(credentialsdir, filename)

			// convert credentialsForType to json string
			credentialsForTypeBytes, innerErr := json.Marshal(credentialsForType)
			if innerErr != nil {
				return hostPath, mountPath, innerErr
			}

			// expand estafette variables in json file
			credentialsForTypeString := string(credentialsForTypeBytes)
			credentialsForTypeString = os.Expand(credentialsForTypeString, c.envvarClient.GetEstafetteEnv)

			// write to file
			err = ioutil.WriteFile(filepath, []byte(credentialsForTypeString), 0666)
			if err != nil {
				return
			}

			log.Debug().Msgf("Stored credentials of type %v in file %v", credentialType, filepath)
		}

		hostPath = credentialsdir
		mountPath = "/credentials"
		if runtime.GOOS == "windows" {
			hostPath = filepath.Join(c.envvarClient.GetTempDir(), strings.TrimPrefix(hostPath, "C:\\Windows\\TEMP"))
			mountPath = "C:" + mountPath
		}
	}

	return
}

func escapeCharsInCommand(command string) string {
	command = strings.Replace(command, "\"", "\\\"", -1)
	command = strings.Replace(command, "$(", "\\$(", -1)
	return command
}