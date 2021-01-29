package builder

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
	"github.com/docker/docker/client"
	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

// DockerRunner pulls and runs docker containers
type DockerRunner interface {
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
	CreateDockerClient() (*client.Client, error)
	CreateBridgeNetwork(ctx context.Context) error
	DeleteBridgeNetwork(ctx context.Context) error
	StopAllContainers()
}

// NewDockerRunner returns a new DockerRunner
func NewDockerRunner(envvarHelper EnvvarHelper, obfuscator Obfuscator, config contracts.BuilderConfig, tailLogsChannel chan contracts.TailLogLine) DockerRunner {

	networkBridge := "estafette"

	if runtime.GOOS == "windows" {
		networkBridge += "-" + generateRandomString(5)
	}

	return &dockerRunnerImpl{
		envvarHelper:                          envvarHelper,
		obfuscator:                            obfuscator,
		config:                                config,
		tailLogsChannel:                       tailLogsChannel,
		runningStageContainerIDs:              make([]string, 0),
		runningSingleStageServiceContainerIDs: make([]string, 0),
		runningMultiStageServiceContainerIDs:  make([]string, 0),
		runningReadinessProbeContainerIDs:     make([]string, 0),
		networkBridge:                         networkBridge,
		entrypointTemplateDir:                 "/entrypoint-templates",
		entrypointTargetDir:                   "/estafette-entrypoints",
	}
}

type dockerRunnerImpl struct {
	envvarHelper    EnvvarHelper
	obfuscator      Obfuscator
	dockerClient    *client.Client
	config          contracts.BuilderConfig
	tailLogsChannel chan contracts.TailLogLine

	runningStageContainerIDs              []string
	runningSingleStageServiceContainerIDs []string
	runningMultiStageServiceContainerIDs  []string
	runningReadinessProbeContainerIDs     []string
	networkBridge                         string
	networkBridgeID                       string
	entrypointTemplateDir                 string
	entrypointTargetDir                   string
}

func (dr *dockerRunnerImpl) IsImagePulled(stageName string, containerImage string) bool {

	log.Info().Msgf("[%v] Checking if docker image '%v' exists locally...", stageName, containerImage)

	imageSummaries, err := dr.dockerClient.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		return false
	}

	for _, summary := range imageSummaries {
		if contains(summary.RepoTags, containerImage) {
			return true
		}
	}

	return false
}

func (dr *dockerRunnerImpl) PullImage(ctx context.Context, stageName string, containerImage string) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "PullImage")
	defer span.Finish()
	span.SetTag("docker-image", containerImage)

	log.Info().Msgf("[%v] Pulling docker image '%v'", stageName, containerImage)

	rc, err := dr.dockerClient.ImagePull(context.Background(), containerImage, dr.getImagePullOptions(containerImage))
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

func (dr *dockerRunnerImpl) GetImageSize(containerImage string) (totalSize int64, err error) {

	items, err := dr.dockerClient.ImageHistory(context.Background(), containerImage)
	if err != nil {
		return totalSize, err
	}

	for _, item := range items {
		totalSize += item.Size
	}

	return totalSize, nil
}

func (dr *dockerRunnerImpl) StartStageContainer(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, stage manifest.EstafetteStage) (containerID string, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "StartStageContainer")
	defer span.Finish()
	span.SetTag("docker-image", stage.ContainerImage)

	// check if image is trusted image
	trustedImage := dr.config.GetTrustedImage(stage.ContainerImage)

	entrypoint, cmds, binds := dr.initContainerStartVariables(stage.Shell, stage.Commands, stage.RunCommandsInForeground)

	// add custom properties as ESTAFETTE_EXTENSION_... envvar
	extensionEnvVars := dr.generateExtensionEnvvars(stage.CustomProperties, stage.EnvVars)

	// mount injected credentials as files
	credentialsdir, err := dr.generateCredentialsFiles(trustedImage)
	if err != nil {
		return
	}
	if credentialsdir != "" {
		if runtime.GOOS == "windows" {
			binds = append(binds, fmt.Sprintf("%v:C:/credentials", credentialsdir))
		} else {
			binds = append(binds, fmt.Sprintf("%v:/credentials", credentialsdir))
		}
	}

	// add stage name to envvars
	if stage.EnvVars == nil {
		stage.EnvVars = map[string]string{}
	}
	stage.EnvVars["ESTAFETTE_STAGE_NAME"] = stage.Name

	// combine and override estafette and global envvars with stage envvars
	combinedEnvVars := dr.envvarHelper.OverrideEnvvars(envvars, stage.EnvVars, extensionEnvVars)

	// decrypt secrets in all envvars
	combinedEnvVars = dr.envvarHelper.decryptSecrets(combinedEnvVars, dr.envvarHelper.GetPipelineName())

	// define docker envvars and expand ESTAFETTE_ variables
	dockerEnvVars := make([]string, 0)
	if combinedEnvVars != nil && len(combinedEnvVars) > 0 {
		for k, v := range combinedEnvVars {
			dockerEnvVars = append(dockerEnvVars, fmt.Sprintf("%v=%v", k, os.Expand(v, dr.envvarHelper.getEstafetteEnv)))
		}
	}

	// define binds
	binds = append(binds, fmt.Sprintf("%v:%v", dir, os.Expand(stage.WorkingDirectory, dr.envvarHelper.getEstafetteEnv)))

	// check if this is a trusted image with RunDocker set to true
	if trustedImage != nil && trustedImage.RunDocker {
		if runtime.GOOS == "windows" {
			if ok, _ := pathExists(`\\.\pipe\docker_engine`); ok {
				binds = append(binds, `\\.\pipe\docker_engine:\\.\pipe\docker_engine`)
			}
			if ok, _ := pathExists("C:/Program Files/Docker"); ok {
				binds = append(binds, "C:/Program Files/Docker:C:/dod")
			}
		} else {
			if ok, _ := pathExists("/var/run/docker.sock"); ok {
				binds = append(binds, "/var/run/docker.sock:/var/run/docker.sock")
			}
			if ok, _ := pathExists("/usr/local/bin/docker"); ok {
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
		WorkingDir:   os.Expand(stage.WorkingDirectory, dr.envvarHelper.getEstafetteEnv),
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
	resp, err := dr.dockerClient.ContainerCreate(ctx, &config, &container.HostConfig{
		Binds:      binds,
		Privileged: privileged,
		AutoRemove: true,
	}, &network.NetworkingConfig{}, "")
	if err != nil {
		return "", err
	}

	// connect to user-defined network
	err = dr.dockerClient.NetworkConnect(ctx, dr.networkBridgeID, resp.ID, nil)
	if err != nil {
		log.Error().Err(err).Msgf("Failed connecting container %v to network %v", resp.ID, dr.networkBridgeID)
		return
	}

	containerID = resp.ID
	dr.runningStageContainerIDs = dr.addRunningContainerID(dr.runningStageContainerIDs, containerID)

	// start container
	if err = dr.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return
	}

	return
}

func (dr *dockerRunnerImpl) StartServiceContainer(ctx context.Context, envvars map[string]string, service manifest.EstafetteService) (containerID string, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "StartServiceContainer")
	defer span.Finish()
	span.SetTag("docker-image", service.ContainerImage)

	// check if image is trusted image
	trustedImage := dr.config.GetTrustedImage(service.ContainerImage)

	entrypoint, cmds, binds := dr.initContainerStartVariables(service.Shell, service.Commands, service.RunCommandsInForeground)

	// add custom properties as ESTAFETTE_EXTENSION_... envvar
	extensionEnvVars := dr.generateExtensionEnvvars(service.CustomProperties, service.EnvVars)

	// mount injected credentials as files
	credentialsdir, err := dr.generateCredentialsFiles(trustedImage)
	if err != nil {
		return
	}
	if credentialsdir != "" {
		if runtime.GOOS == "windows" {
			binds = append(binds, fmt.Sprintf("%v:C:/credentials", credentialsdir))
		} else {
			binds = append(binds, fmt.Sprintf("%v:/credentials", credentialsdir))
		}
	}

	// add service name to envvars
	if service.EnvVars == nil {
		service.EnvVars = map[string]string{}
	}
	service.EnvVars["ESTAFETTE_SERVICE_NAME"] = service.Name

	// combine and override estafette and global envvars with pipeline envvars
	combinedEnvVars := dr.envvarHelper.OverrideEnvvars(envvars, service.EnvVars, extensionEnvVars)

	// decrypt secrets in all envvars
	combinedEnvVars = dr.envvarHelper.decryptSecrets(combinedEnvVars, dr.envvarHelper.GetPipelineName())

	// define docker envvars and expand ESTAFETTE_ variables
	dockerEnvVars := make([]string, 0)
	if combinedEnvVars != nil && len(combinedEnvVars) > 0 {
		for k, v := range combinedEnvVars {
			dockerEnvVars = append(dockerEnvVars, fmt.Sprintf("%v=%v", k, os.Expand(v, dr.envvarHelper.getEstafetteEnv)))
		}
	}

	// check if this is a trusted image with RunDocker set to true
	if trustedImage != nil && trustedImage.RunDocker {
		if runtime.GOOS == "windows" {
			if ok, _ := pathExists(`\\.\pipe\docker_engine`); ok {
				binds = append(binds, `\\.\pipe\docker_engine:\\.\pipe\docker_engine`)
			}
			if ok, _ := pathExists("C:/Program Files/Docker"); ok {
				binds = append(binds, "C:/Program Files/Docker:C:/dod")
			}
		} else {
			if ok, _ := pathExists("/var/run/docker.sock"); ok {
				binds = append(binds, "/var/run/docker.sock:/var/run/docker.sock")
			}
			if ok, _ := pathExists("/usr/local/bin/docker"); ok {
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
	resp, err := dr.dockerClient.ContainerCreate(ctx, &config, &container.HostConfig{
		Binds:      binds,
		Privileged: privileged,
		AutoRemove: true,
	}, &network.NetworkingConfig{}, service.Name)
	if err != nil {
		return
	}

	// connect to user-defined network
	err = dr.dockerClient.NetworkConnect(ctx, dr.networkBridgeID, resp.ID, nil)
	if err != nil {
		log.Error().Err(err).Msgf("Failed connecting container %v to network %v", resp.ID, dr.networkBridgeID)
		return
	}

	containerID = resp.ID
	if service.MultiStage != nil && *service.MultiStage {
		dr.runningMultiStageServiceContainerIDs = dr.addRunningContainerID(dr.runningMultiStageServiceContainerIDs, containerID)
	} else {
		dr.runningSingleStageServiceContainerIDs = dr.addRunningContainerID(dr.runningSingleStageServiceContainerIDs, containerID)
	}

	// start container
	if err = dr.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return
	}

	return
}

func (dr *dockerRunnerImpl) RunReadinessProbeContainer(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "RunReadinessProbeContainer")
	defer span.Finish()

	readinessProberImage := "estafette/scratch:latest"
	isPulled := dr.IsImagePulled(service.Name+"-prober", readinessProberImage)
	if !isPulled {
		err = dr.PullImage(ctx, service.Name+"-prober", readinessProberImage)
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
	envvars = dr.envvarHelper.decryptSecrets(envvars, dr.envvarHelper.GetPipelineName())

	// define docker envvars and expand ESTAFETTE_ variables
	dockerEnvVars := make([]string, 0)
	if envvars != nil && len(envvars) > 0 {
		for k, v := range envvars {
			dockerEnvVars = append(dockerEnvVars, fmt.Sprintf("%v=%v", k, os.Expand(v, dr.envvarHelper.getEstafetteEnv)))
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
	resp, err := dr.dockerClient.ContainerCreate(ctx, &config, &container.HostConfig{
		Binds:      binds,
		AutoRemove: true,
	}, &network.NetworkingConfig{}, "")
	if err != nil {
		return
	}

	// connect to user-defined network
	err = dr.dockerClient.NetworkConnect(ctx, dr.networkBridgeID, resp.ID, nil)
	if err != nil {
		log.Error().Err(err).Msgf("Failed connecting container %v to network %v", resp.ID, dr.networkBridgeID)
		return
	}

	containerID := resp.ID
	dr.runningReadinessProbeContainerIDs = dr.addRunningContainerID(dr.runningReadinessProbeContainerIDs, containerID)

	// start container
	if err = dr.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return
	}

	// follow logs
	rc, err := dr.dockerClient.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
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
	resultC, errC := dr.dockerClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	var exitCode int64
	select {
	case result := <-resultC:
		exitCode = result.StatusCode
	case err = <-errC:
		return err
	}

	// clear container id
	dr.runningReadinessProbeContainerIDs = dr.removeRunningContainerID(dr.runningReadinessProbeContainerIDs, containerID)

	if exitCode != 0 {
		return fmt.Errorf("Failed with exit code: %v", exitCode)
	}

	return
}

func (dr *dockerRunnerImpl) TailContainerLogs(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth, runIndex int, multiStage *bool) (err error) {

	lineNumber := 1

	// follow logs
	rc, err := dr.dockerClient.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
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
		logLineString := dr.obfuscator.Obfuscate(string(logLine))

		// create object for tailing logs and storing in the db when done
		logLineObject := contracts.BuildLogLine{
			LineNumber: lineNumber,
			Timestamp:  time.Now().UTC(),
			StreamType: streamType,
			Text:       logLineString,
		}
		lineNumber++

		// log as json, to be tailed when looking at live logs from gui
		dr.tailLogsChannel <- contracts.TailLogLine{
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
	resultC, errC := dr.dockerClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

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
		dr.runningStageContainerIDs = dr.removeRunningContainerID(dr.runningStageContainerIDs, containerID)
	} else if stageType == contracts.LogTypeService && multiStage != nil {
		if *multiStage {
			dr.runningMultiStageServiceContainerIDs = dr.removeRunningContainerID(dr.runningMultiStageServiceContainerIDs, containerID)
		} else {
			dr.runningSingleStageServiceContainerIDs = dr.removeRunningContainerID(dr.runningSingleStageServiceContainerIDs, containerID)
		}
	}

	if exitCode != 0 {
		return fmt.Errorf("Failed with exit code: %v", exitCode)
	}

	return err
}

func (dr *dockerRunnerImpl) StopSingleStageServiceContainers(ctx context.Context, parentStage manifest.EstafetteStage) {

	log.Info().Msgf("[%v] Stopping single-stage service containers...", parentStage.Name)

	// the service containers should be the only ones running, so just stop all containers
	dr.stopContainers(dr.runningSingleStageServiceContainerIDs)

	log.Info().Msgf("[%v] Stopped single-stage service containers...", parentStage.Name)
}

func (dr *dockerRunnerImpl) StopMultiStageServiceContainers(ctx context.Context) {

	log.Info().Msg("Stopping multi-stage service containers...")

	// the service containers should be the only ones running, so just stop all containers
	dr.stopContainers(dr.runningMultiStageServiceContainerIDs)

	log.Info().Msg("Stopped multi-stage service containers...")
}

func (dr *dockerRunnerImpl) StartDockerDaemon() error {

	// dockerd --host=unix:///var/run/docker.sock --host=tcp://0.0.0.0:2375 --mtu=1500 &
	log.Debug().Msg("Starting docker daemon...")
	args := []string{"--host=unix:///var/run/docker.sock", "--host=tcp://0.0.0.0:2375"}

	// if an mtu is configured pass it to the docker daemon
	if dr.config.DockerDaemonMTU != nil && *dr.config.DockerDaemonMTU != "" {
		args = append(args, fmt.Sprintf("--mtu=%v", *dr.config.DockerDaemonMTU))
	}

	// if a bip is configured pass it to the docker daemon
	if dr.config.DockerDaemonBIP != nil && *dr.config.DockerDaemonBIP != "" {
		args = append(args, fmt.Sprintf("--bip=%v", *dr.config.DockerDaemonBIP))
	}

	// if a registry mirror is configured pass it to the docker daemon
	if dr.config.RegistryMirror != nil && *dr.config.RegistryMirror != "" {
		args = append(args, fmt.Sprintf("--registry-mirror=%v", *dr.config.RegistryMirror))
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

func (dr *dockerRunnerImpl) WaitForDockerDaemon() {

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

func (dr *dockerRunnerImpl) CreateDockerClient() (*client.Client, error) {

	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return dockerClient, err
	}
	dr.dockerClient = dockerClient

	return dockerClient, err
}

func (dr *dockerRunnerImpl) getImagePullOptions(containerImage string) types.ImagePullOptions {

	containerRegistryCredentials := dr.config.GetCredentialsByType("container-registry")

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
	containerRegistryPullCredentials := dr.config.GetCredentialsByType("container-registry-pull")

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

func (dr *dockerRunnerImpl) IsTrustedImage(stageName string, containerImage string) bool {

	log.Info().Msgf("[%v] Checking if docker image '%v' is trusted...", stageName, containerImage)

	// check if image is trusted image
	trustedImage := dr.config.GetTrustedImage(containerImage)

	return trustedImage != nil
}

func (dr *dockerRunnerImpl) HasInjectedCredentials(stageName string, containerImage string) bool {

	log.Info().Msgf("[%v] Checking if docker image '%v' has injected credentials...", stageName, containerImage)

	// check if image has injected credentials
	trustedImage := dr.config.GetTrustedImage(containerImage)
	if trustedImage == nil {
		return false
	}

	credentialMap := dr.config.GetCredentialsForTrustedImage(*trustedImage)

	return len(credentialMap) > 0
}

func (dr *dockerRunnerImpl) stopContainer(containerID string) error {

	log.Debug().Msgf("Stopping container with id %v", containerID)

	timeout := 20 * time.Second
	err := dr.dockerClient.ContainerStop(context.Background(), containerID, &timeout)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed stopping container with id %v", containerID)
		return err
	}

	log.Info().Msgf("Stopped container with id %v", containerID)
	return nil
}

func (dr *dockerRunnerImpl) stopContainers(containerIDs []string) {

	if len(containerIDs) > 0 {
		log.Info().Msgf("Stopping %v containers", len(containerIDs))

		var wg sync.WaitGroup
		wg.Add(len(containerIDs))

		for _, id := range containerIDs {
			go func(id string) {
				defer wg.Done()
				dr.stopContainer(id)
			}(id)
		}

		wg.Wait()

		log.Info().Msgf("Stopped %v containers", len(containerIDs))
	} else {
		log.Info().Msg("No containers to stop")
	}
}

func (dr *dockerRunnerImpl) StopAllContainers() {

	allRunningContainerIDs := append(dr.runningStageContainerIDs, dr.runningSingleStageServiceContainerIDs...)
	allRunningContainerIDs = append(allRunningContainerIDs, dr.runningMultiStageServiceContainerIDs...)
	allRunningContainerIDs = append(allRunningContainerIDs, dr.runningReadinessProbeContainerIDs...)

	dr.stopContainers(allRunningContainerIDs)
}

func (dr *dockerRunnerImpl) addRunningContainerID(containerIDs []string, containerID string) []string {

	log.Debug().Msgf("Adding container id %v to containerIDs", containerID)

	return append(containerIDs, containerID)
}

func (dr *dockerRunnerImpl) removeRunningContainerID(containerIDs []string, containerID string) []string {

	log.Debug().Msgf("Removing container id %v from containerIDs", containerID)

	purgedContainerIDs := []string{}

	for _, id := range containerIDs {
		if id != containerID {
			purgedContainerIDs = append(purgedContainerIDs, id)
		}
	}

	return purgedContainerIDs
}

func (dr *dockerRunnerImpl) CreateBridgeNetwork(ctx context.Context) error {

	if dr.networkBridgeID == "" {
		log.Info().Msgf("Creating docker network %v...", dr.networkBridge)

		name := dr.networkBridge
		options := types.NetworkCreate{}
		if dr.config.DockerNetwork != nil && runtime.GOOS != "windows" {
			name = dr.config.DockerNetwork.Name
			options.IPAM = &network.IPAM{
				Driver: "default",
				Config: []network.IPAMConfig{
					{
						Subnet:  dr.config.DockerNetwork.Subnet,
						Gateway: dr.config.DockerNetwork.Gateway,
					},
				},
			}
		}

		resp, err := dr.dockerClient.NetworkCreate(ctx, name, options)

		if err != nil {
			log.Error().Err(err).Msgf("Failed creating docker network %v", dr.networkBridge)
			return err
		}

		dr.networkBridgeID = resp.ID

		log.Info().Msgf("Succesfully created docker network %v", dr.networkBridge)

	} else {
		log.Info().Msgf("Docker network %v already exists with id %v...", dr.networkBridge, dr.networkBridgeID)
	}
	return nil
}

func (dr *dockerRunnerImpl) DeleteBridgeNetwork(ctx context.Context) error {

	if dr.networkBridgeID != "" {
		log.Info().Msgf("Deleting docker network %v with id %v...", dr.networkBridge, dr.networkBridgeID)

		err := dr.dockerClient.NetworkRemove(ctx, dr.networkBridgeID)

		if err != nil {
			log.Error().Err(err).Msgf("Failed deleting docker network %v with id %v", dr.networkBridge, dr.networkBridgeID)
			return err
		}

		dr.networkBridgeID = ""

		log.Info().Msgf("Succesfully deleted docker network %v with id %v", dr.networkBridge, dr.networkBridgeID)
	}

	return nil
}

func (dr *dockerRunnerImpl) generateEntrypointScript(shell string, commands []string, runCommandsInForeground bool) (path string, extension string, err error) {

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

	extension = "sh"
	if runtime.GOOS == "windows" && shell == "powershell" {
		extension = "ps"
	} else if runtime.GOOS == "windows" && shell == "cmd" {
		extension = "cmd"
	}

	templatePath := fmt.Sprintf("%v/estafette-entrypoint.%v", dr.entrypointTemplateDir, extension)

	// create target file to render template to
	targetFile, err := ioutil.TempFile(dr.entrypointTargetDir, "estafette-entrypoint-*.sh")
	if err != nil {
		return "", extension, err
	}
	defer targetFile.Close()
	path = targetFile.Name()

	// read and parse template
	entrypointTemplate, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", extension, err
	}

	err = entrypointTemplate.Execute(targetFile, data)
	if err != nil {
		return "", extension, err
	}

	if runtime.GOOS != "windows" {
		err = os.Chmod(path, 0755)
		if err != nil {
			return "", extension, err
		}
	}

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", extension, err
	}
	log.Debug().Str("entrypoint", string(bytes)).Msgf("Inspecting entrypoint script at %v", path)

	return path, extension, nil
}

func (dr *dockerRunnerImpl) initContainerStartVariables(shell string, commands []string, runCommandsInForeground bool) (entrypoint []string, cmds []string, binds []string) {
	entrypoint = make([]string, 0)
	cmds = make([]string, 0)
	binds = make([]string, 0)

	if len(commands) > 0 {
		// generate entrypoint script
		path, extension, err := dr.generateEntrypointScript(shell, commands, runCommandsInForeground)
		if runtime.GOOS != "windows" && err == nil {
			// use generated entrypoint script for executing commands
			entrypointScriptPath := fmt.Sprintf("/estafette-entrypoint.%v", extension)
			entrypoint = []string{entrypointScriptPath}
			binds = append(binds, fmt.Sprintf("%v:%v", path, entrypointScriptPath))
		} else {
			// generating entrypoint script failed, do it in the old way
			log.Warn().Err(err).Msgf("Generating entrypoint script failed, setting up entrypoint and command in old style")

			// define entrypoint
			entrypoint = []string{shell}
			if runtime.GOOS == "windows" && shell == "powershell" {
				entrypoint = append(entrypoint, "-Command")
			} else if runtime.GOOS == "windows" && shell == "cmd" {
				entrypoint = append(entrypoint, "/S", "/C")
			} else {
				entrypoint = append(entrypoint, "-c")
			}

			// define commands
			cmdStopOnErrorFlag := ""
			cmdSeparator := ";"
			if runtime.GOOS == "windows" && shell == "powershell" {
				cmdStopOnErrorFlag = "$ErrorActionPreference = 'Stop'; $ProgressPreference = 'SilentlyContinue'; "
				if dr.config.DockerDaemonMTU != nil && *dr.config.DockerDaemonMTU != "" {
					mtu, err := strconv.Atoi(*dr.config.DockerDaemonMTU)
					if err == nil {
						// mtu -= 50
						cmdStopOnErrorFlag += fmt.Sprintf("Write-Host 'Updating MTU to %v...'; Get-NetAdapter | Where-Object Name -like \"*Ethernet*\" | ForEach-Object { & netsh interface ipv4 set subinterface $_.InterfaceIndex mtu=%v store=persistent }; ", mtu, mtu)
					}
				}
				cmdSeparator = ";"
			} else if runtime.GOOS == "windows" && shell == "cmd" {
				cmdStopOnErrorFlag = ""
				cmdSeparator = " && "
			} else {
				cmdStopOnErrorFlag = "set -e; "
				cmdSeparator = ";"
			}
			cmds = append(cmds, cmdStopOnErrorFlag+strings.Join(commands, cmdSeparator))
		}
	}

	return
}

func (dr *dockerRunnerImpl) generateExtensionEnvvars(customProperties map[string]interface{}, envvars map[string]string) (extensionEnvVars map[string]string) {
	extensionEnvVars = map[string]string{}
	if customProperties != nil && len(customProperties) > 0 {
		for k, v := range customProperties {
			extensionkey := dr.envvarHelper.getEstafetteEnvvarName(fmt.Sprintf("ESTAFETTE_EXTENSION_%v", foundation.ToUpperSnakeCase(k)))

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

func (dr *dockerRunnerImpl) generateCredentialsFiles(trustedImage *contracts.TrustedImageConfig) (credentialsdir string, err error) {

	if trustedImage != nil {
		// create a tempdir to store credential files in and mount into container
		credentialsdir, err = ioutil.TempDir("", "*-credentials")
		if err != nil {
			return
		}

		// set permissions on directory to avoid non-root containers not to be able to read from the mounted directory
		err = os.Chmod(credentialsdir, 0777)
		if err != nil {
			return
		}

		credentialMap := dr.config.GetCredentialsForTrustedImage(*trustedImage)
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
				return credentialsdir, innerErr
			}

			// expand estafette variables in json file
			credentialsForTypeString := string(credentialsForTypeBytes)
			credentialsForTypeString = os.Expand(credentialsForTypeString, dr.envvarHelper.getEstafetteEnv)

			// write to file
			err = ioutil.WriteFile(filepath, []byte(credentialsForTypeString), 0666)
			if err != nil {
				return
			}

			log.Debug().Msgf("Stored credentials of type %v in file %v", credentialType, filepath)
		}

		if runtime.GOOS == "windows" {
			log.Debug().Msgf("Joining %v and %v to create credentials dir hostpath for mounting credentials", dr.envvarHelper.GetTempDir(), credentialsdir)
			// create full path to credentials dir on host (inside the emptydir representing the workdir) to able to bind it using docker-outside-docker
			credentialsdir = filepath.Join(dr.envvarHelper.GetTempDir(), strings.TrimPrefix(credentialsdir, "C:\\Users\\ContainerAdministrator\\AppData\\Local\\Temp"))
		}
	}

	return
}

func escapeCharsInCommand(command string) string {
	command = strings.Replace(command, "\"", "\\\"", -1)
	command = strings.Replace(command, "$(", "\\$(", -1)
	return command
}
