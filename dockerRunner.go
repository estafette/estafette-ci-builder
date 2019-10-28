package main

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
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

// DockerRunner pulls and runs docker containers
type DockerRunner interface {
	isDockerImagePulled(stageName string, containerImage string) bool
	runDockerPull(ctx context.Context, stageName string, containerImage string) error
	getDockerImageSize(containerImage string) (int64, error)
	runDockerRun(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error)
	runDockerRunService(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error)
	tailDockerLogs(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (logLines []contracts.BuildLogLine, exitCode int64, canceled bool, err error)
	stopServices(ctx context.Context, parentStage *manifest.EstafetteStage, services []*manifest.EstafetteService)

	startDockerDaemon() error
	waitForDockerDaemon()
	createDockerClient() (*client.Client, error)
	getImagePullOptions(containerImage string) types.ImagePullOptions
	isTrustedImage(stageName string, containerImage string) bool
	stopContainerOnCancellation()
	deleteContainerID(containerID string)
	removeContainer(containerID string) error
	createBridgeNetwork(ctx context.Context) error
	deleteBridgeNetwork(ctx context.Context) error
}

type dockerRunnerImpl struct {
	envvarHelper        EnvvarHelper
	obfuscator          Obfuscator
	dockerClient        *client.Client
	runAsJob            bool
	config              contracts.BuilderConfig
	cancellationChannel chan struct{}
	containerIDs        map[string]string
	canceled            bool
	networkBridge       string
	networkBridgeID     string
}

// NewDockerRunner returns a new DockerRunner
func NewDockerRunner(envvarHelper EnvvarHelper, obfuscator Obfuscator, runAsJob bool, config contracts.BuilderConfig, cancellationChannel chan struct{}) DockerRunner {
	return &dockerRunnerImpl{
		envvarHelper:        envvarHelper,
		obfuscator:          obfuscator,
		runAsJob:            runAsJob,
		config:              config,
		cancellationChannel: cancellationChannel,
		containerIDs:        make(map[string]string, 0),
		networkBridge:       "estafette",
	}
}

func (dr *dockerRunnerImpl) isDockerImagePulled(stageName string, containerImage string) bool {

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

func (dr *dockerRunnerImpl) runDockerPull(ctx context.Context, stageName string, containerImage string) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "DockerPull")
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

func (dr *dockerRunnerImpl) getDockerImageSize(containerImage string) (totalSize int64, err error) {

	items, err := dr.dockerClient.ImageHistory(context.Background(), containerImage)
	if err != nil {
		return totalSize, err
	}

	for _, item := range items {
		totalSize += item.Size
	}

	return totalSize, nil
}

func (dr *dockerRunnerImpl) runDockerRun(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, parentStage *manifest.EstafetteStage, p manifest.EstafetteStage) (containerID string, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "DockerRun")
	defer span.Finish()
	span.SetTag("docker-image", p.ContainerImage)

	// check if image is trusted image
	trustedImage := dr.config.GetTrustedImage(p.ContainerImage)

	// run docker with image and commands from yaml

	// define entrypoint
	entrypoint := make([]string, 0)
	entrypoint = []string{p.Shell}
	if runtime.GOOS == "windows" && p.Shell == "powershell" {
		entrypoint = append(entrypoint, "-Command")
	} else if runtime.GOOS == "windows" && p.Shell == "cmd" {
		entrypoint = append(entrypoint, "/S", "/C")
	} else {
		entrypoint = append(entrypoint, "-c")
	}

	// define commands
	cmds := make([]string, 0)
	cmdStopOnErrorFlag := ""
	cmdSeparator := ";"
	if runtime.GOOS == "windows" && p.Shell == "powershell" {
		cmdStopOnErrorFlag = "$ErrorActionPreference = 'Stop'; $ProgressPreference = 'SilentlyContinue'; "
		if dr.config.DockerDaemonMTU != nil && *dr.config.DockerDaemonMTU != "" {
			mtu, err := strconv.Atoi(*dr.config.DockerDaemonMTU)
			if err == nil {
				mtu -= 50
				cmdStopOnErrorFlag += fmt.Sprintf("Write-Host 'Updating MTU to %v...'; Get-NetAdapter | Where-Object Name -like \"*Ethernet*\" | ForEach-Object { & netsh interface ipv4 set subinterface $_.InterfaceIndex mtu=%v store=persistent }; ", mtu, mtu)
			}
		}
		cmdSeparator = ";"
	} else if runtime.GOOS == "windows" && p.Shell == "cmd" {
		cmdStopOnErrorFlag = ""
		cmdSeparator = " && "
	} else {
		cmdStopOnErrorFlag = "set -e; "
		cmdSeparator = ";"
	}
	cmds = append(cmds, cmdStopOnErrorFlag+strings.Join(p.Commands, cmdSeparator))

	// add custom properties as ESTAFETTE_EXTENSION_... envvar
	extensionEnvVars := map[string]string{}
	if p.CustomProperties != nil && len(p.CustomProperties) > 0 {
		for k, v := range p.CustomProperties {
			extensionkey := dr.envvarHelper.getEstafetteEnvvarName(fmt.Sprintf("ESTAFETTE_EXTENSION_%v", dr.envvarHelper.toUpperSnake(k)))

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
		customProperties := p.CustomProperties
		customProperties["env"] = p.EnvVars

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
	}

	// add credentials if trusted image with injectedCredentialTypes
	credentialEnvVars := map[string]string{}
	if trustedImage != nil {
		// add credentials as ESTAFETTE_CREDENTIALS_... envvar with snake cased credential type so they can be unmarshalled separately in the image

		credentialMap := dr.config.GetCredentialsForTrustedImage(*trustedImage)
		for credentialType, credentialsForType := range credentialMap {

			credentialkey := dr.envvarHelper.getEstafetteEnvvarName(fmt.Sprintf("ESTAFETTE_CREDENTIALS_%v", dr.envvarHelper.toUpperSnake(credentialType)))

			// convert credentialsForType to json string
			credentialsForTypeBytes, err := json.Marshal(credentialsForType)
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to marshal credentials of type %v for envvar %v", credentialType, credentialkey)
			}

			// set envvar
			credentialEnvVars[credentialkey] = string(credentialsForTypeBytes)

			log.Debug().Msgf("Set envvar %v to credentials of type %v", credentialkey, credentialType)
		}
	}

	// combine and override estafette and global envvars with pipeline envvars
	combinedEnvVars := dr.envvarHelper.overrideEnvvars(envvars, p.EnvVars, extensionEnvVars, credentialEnvVars)

	// decrypt secrets in all envvars
	combinedEnvVars = dr.envvarHelper.decryptSecrets(combinedEnvVars)

	// define docker envvars and expand ESTAFETTE_ variables
	dockerEnvVars := make([]string, 0)
	if combinedEnvVars != nil && len(combinedEnvVars) > 0 {
		for k, v := range combinedEnvVars {
			dockerEnvVars = append(dockerEnvVars, fmt.Sprintf("%v=%v", k, os.Expand(v, dr.envvarHelper.getEstafetteEnv)))
		}
	}

	// define binds
	binds := make([]string, 0)
	binds = append(binds, fmt.Sprintf("%v:%v", dir, os.Expand(p.WorkingDirectory, dr.envvarHelper.getEstafetteEnv)))

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
		Image:        p.ContainerImage,
		WorkingDir:   os.Expand(p.WorkingDirectory, dr.envvarHelper.getEstafetteEnv),
	}
	if len(p.Commands) > 0 {
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
		Binds:       binds,
		Privileged:  privileged,
		NetworkMode: "host",
	}, nil, "")
	if err != nil {
		return "", err
	}

	// // connect to user-defined network
	// err = dr.dockerClient.NetworkConnect(ctx, dr.networkBridgeID, resp.ID, nil)
	// if err != nil {
	// 	log.Error().Err(err).Msgf("Failed connecting container %v to network %v", resp.ID, dr.networkBridgeID)
	// 	return
	// }

	containerKey := ""
	if parentStage != nil {
		containerKey += parentStage.Name + "-nested-"
	}
	containerKey += p.Name

	containerID = resp.ID
	dr.containerIDs[containerKey] = resp.ID

	// start container
	if err = dr.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return
	}

	return
}

func (dr *dockerRunnerImpl) runDockerRunService(ctx context.Context, envvars map[string]string, parentStage *manifest.EstafetteStage, service manifest.EstafetteService) (containerID string, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "DockerRunService")
	defer span.Finish()
	span.SetTag("docker-image", service.ContainerImage)

	networkResources, networkListError := dr.dockerClient.NetworkList(ctx, types.NetworkListOptions{})
	log.Debug().Interface("networkResources", networkResources).Interface("networkListError", networkListError).Msg("Listing docker networks")

	// check if image is trusted image
	trustedImage := dr.config.GetTrustedImage(service.ContainerImage)

	// combine and override estafette and global envvars with pipeline envvars
	combinedEnvVars := dr.envvarHelper.overrideEnvvars(envvars, service.EnvVars)

	// decrypt secrets in all envvars
	combinedEnvVars = dr.envvarHelper.decryptSecrets(combinedEnvVars)

	// define docker envvars and expand ESTAFETTE_ variables
	dockerEnvVars := make([]string, 0)
	if combinedEnvVars != nil && len(combinedEnvVars) > 0 {
		for k, v := range combinedEnvVars {
			dockerEnvVars = append(dockerEnvVars, fmt.Sprintf("%v=%v", k, os.Expand(v, dr.envvarHelper.getEstafetteEnv)))
		}
	}

	// check if this is a trusted image with RunDocker set to true
	binds := make([]string, 0)
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

	// define commands
	if service.Command != "" {
		config.Cmd = []string{service.Command}
	}

	// create exposed ports and portbindings
	ports := []string{}
	for _, p := range service.Ports {
		hostPort := p.Port
		if p.HostPort != nil {
			hostPort = *p.HostPort
		}
		ports = append(ports, fmt.Sprintf("127.0.0.1:%v:%v/tcp", hostPort, p.Port))
	}
	exposedPorts, portBindings, err := nat.ParsePortSpecs(ports)
	if err != nil {
		return
	}
	log.Debug().Interface("exposedPorts", exposedPorts).Interface("portBindings", portBindings).Interface("ports", ports).Msgf("Exposing ports...")
	config.ExposedPorts = exposedPorts

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
		Binds:        binds,
		Privileged:   privileged,
		PortBindings: portBindings,
		NetworkMode:  "host",
	}, nil, service.Name)
	if err != nil {
		return
	}

	// // connect to user-defined network
	// err = dr.dockerClient.NetworkConnect(ctx, dr.networkBridgeID, resp.ID, nil)
	// if err != nil {
	// 	log.Error().Err(err).Msgf("Failed connecting container %v to network %v", resp.ID, dr.networkBridgeID)
	// 	return
	// }

	containerJSON, inspectErr := dr.dockerClient.ContainerInspect(ctx, resp.ID)
	log.Debug().Err(inspectErr).Interface("containerJSON", containerJSON).Msgf("Inspecting container for service %v", service.Name)

	containerKey := ""
	if parentStage != nil {
		containerKey += parentStage.Name + "-service-"
	}
	containerKey += service.Name

	containerID = resp.ID
	dr.containerIDs[containerKey] = resp.ID

	// start container
	if err = dr.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return
	}

	return
}

func (dr *dockerRunnerImpl) tailDockerLogs(ctx context.Context, containerID, parentStageName, stageName, stageType string, depth, runIndex int) (logLines []contracts.BuildLogLine, exitCode int64, canceled bool, err error) {

	logLines = make([]contracts.BuildLogLine, 0)
	exitCode = int64(-1)
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
		return logLines, exitCode, dr.canceled, err
	}
	defer rc.Close()

	// stream logs to stdout with buffering
	in := bufio.NewReader(rc)
	var readError error
	for {

		if dr.canceled {
			log.Debug().Msgf("Cancelled tailing logs for container %v", containerID)
			return logLines, exitCode, dr.canceled, err
		}

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

		if dr.runAsJob {
			// log as json, to be tailed when looking at live logs from gui
			tailLogLine := contracts.TailLogLine{
				Step:        stageName,
				ParentStage: parentStageName,
				Type:        stageType,
				Depth:       depth,
				RunIndex:    runIndex,
				LogLine:     &logLineObject,
			}
			log.Info().Interface("tailLogLine", tailLogLine).Msg("")
		} else {
			log.Info().Msgf("[%v] %v", stageName, logLineString)
		}

		// add to log lines send when build/release job is finished
		logLines = append(logLines, logLineObject)
	}

	if readError != nil && readError != io.EOF {
		log.Error().Msgf("[%v] Error: %v", stageName, readError)
		return logLines, exitCode, dr.canceled, readError
	}

	// wait for container to stop running
	resultC, errC := dr.dockerClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	select {
	case result := <-resultC:
		exitCode = result.StatusCode
	case err = <-errC:
		log.Warn().Err(err).Msgf("Container %v exited with error", containerID)
		return logLines, exitCode, dr.canceled, err
	}

	// clear container id
	dr.deleteContainerID(containerID)

	if exitCode != 0 {
		return logLines, exitCode, dr.canceled, fmt.Errorf("Failed with exit code: %v", exitCode)
	}

	return logLines, exitCode, dr.canceled, err
}

func (dr *dockerRunnerImpl) stopServices(ctx context.Context, parentStage *manifest.EstafetteStage, services []*manifest.EstafetteService) {

	if parentStage != nil {
		log.Info().Msgf("Stopping services for stage '%v'...", parentStage.Name)
	}

	var wg sync.WaitGroup
	wg.Add(len(services))

	for _, s := range services {
		go func(parentStage *manifest.EstafetteStage, s *manifest.EstafetteService) {
			defer wg.Done()

			parentStageName := ""
			if parentStage != nil {
				parentStageName = parentStage.Name
			}

			if s.ContinueAfterStage {
				if parentStage != nil {
					log.Info().Msgf("Not stopping service '%v' for stage '%v', it has continueAfterStage = true...", s.Name, parentStage.Name)
				}
				return
			}
			if parentStage != nil {
				log.Info().Msgf("Stopping service '%v' for stage '%v'...", s.Name, parentStage.Name)
			}

			containerKey := ""
			if parentStage != nil {
				containerKey += parentStage.Name + "-service-"
			}
			containerKey += s.Name

			if id, ok := dr.containerIDs[containerKey]; ok {
				//do something here
				err := dr.stopContainer(id)

				if dr.runAsJob {
					// log tailing - finalize stage
					status := "SUCCEEDED"
					if err != nil {
						status = "FAILED"
					}

					tailLogLine := contracts.TailLogLine{
						Step:        s.Name,
						ParentStage: parentStageName,
						Type:        "service",
						Status:      &status,
					}

					// log as json, to be tailed when looking at live logs from gui
					log.Info().Interface("tailLogLine", tailLogLine).Msg("")
				}

				removeErr := dr.removeContainer(id)
				if removeErr != nil {
					log.Warn().Err(removeErr).Msgf("Failed removing service %v container with id %v", s.Name, id)
				}

			}
		}(parentStage, s)
	}
	wg.Wait()

	if parentStage != nil {
		log.Info().Msgf("Stopped services for stage '%v'", parentStage.Name)
	}
}

func (dr *dockerRunnerImpl) startDockerDaemon() error {

	mtu := "1500"
	if dr.config.DockerDaemonMTU != nil && *dr.config.DockerDaemonMTU != "" {
		mtu = *dr.config.DockerDaemonMTU
	}

	// dockerd --host=unix:///var/run/docker.sock --host=tcp://0.0.0.0:2375 --mtu=1500 &
	log.Debug().Msg("Starting docker daemon...")
	args := []string{"--host=unix:///var/run/docker.sock", "--host=tcp://0.0.0.0:2375", fmt.Sprintf("--mtu=%v", mtu)}

	// if a registry mirror is set in config configured docker daemon to use it
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

func (dr *dockerRunnerImpl) waitForDockerDaemon() {

	// wait until /var/run/docker.sock exists
	log.Debug().Msg("Waiting for docker daemon to be ready for use...")
	for {
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			// does not exist
			time.Sleep(1000 * time.Millisecond)
		} else {
			// file exists, break out of for loop
			break
		}
	}
	log.Debug().Msg("Docker daemon is ready for use")
}

func (dr *dockerRunnerImpl) createDockerClient() (*client.Client, error) {

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

					// check whether this is a private repo and should be authenticated in advance
					if private, ok := credential.AdditionalProperties["private"].(bool); !ok || private {
						return types.ImagePullOptions{
							RegistryAuth: authStr,
						}
					}

					// otherwise provide the option to authenticate after an authorization error
					return types.ImagePullOptions{
						PrivilegeFunc: func() (string, error) { return authStr, nil },
					}
				}
				log.Error().Err(err).Msgf("Failed marshaling docker auth config for container image %v", containerImage)

				break
			}
		}
	}

	return types.ImagePullOptions{}
}

func (dr *dockerRunnerImpl) isTrustedImage(stageName string, containerImage string) bool {

	log.Info().Msgf("[%v] Checking if docker image '%v' is trusted...", stageName, containerImage)

	// check if image is trusted image
	trustedImage := dr.config.GetTrustedImage(containerImage)

	return trustedImage != nil
}

func (dr *dockerRunnerImpl) stopContainer(id string) error {
	timeout := 20 * time.Second
	err := dr.dockerClient.ContainerStop(context.Background(), id, &timeout)
	if err != nil {
		log.Warn().Err(err).Msgf("Stopping container %v for cancellation failed", id)
		return err
	}

	log.Info().Msgf("Stopped container %v for cancellation", id)
	return nil
}

func (dr *dockerRunnerImpl) stopContainerOnCancellation() {
	// wait for cancellation

	<-dr.cancellationChannel

	dr.canceled = true

	if len(dr.containerIDs) > 0 {

		var wg sync.WaitGroup
		wg.Add(len(dr.containerIDs))

		for _, id := range dr.containerIDs {
			go func(id string) {
				defer wg.Done()
				dr.stopContainer(id)
			}(id)
		}

		wg.Wait()
	} else {
		log.Info().Msg("No container to stop for cancellation")
	}
}

func (dr *dockerRunnerImpl) deleteContainerID(containerID string) {

	for k, v := range dr.containerIDs {
		if v == containerID {
			delete(dr.containerIDs, k)
			return
		}
	}
}

func (dr *dockerRunnerImpl) removeContainer(containerID string) error {

	err := dr.dockerClient.ContainerRemove(context.Background(), containerID, types.ContainerRemoveOptions{
		Force: true,
	})
	if err != nil {
		return err
	}
	return nil
}

func (dr *dockerRunnerImpl) createBridgeNetwork(ctx context.Context) error {

	log.Info().Msgf("Creating docker network %v...", dr.networkBridge)

	resp, err := dr.dockerClient.NetworkCreate(ctx, dr.networkBridge, types.NetworkCreate{})

	if err != nil {
		log.Error().Err(err).Msgf("Failed creating docker network %v", dr.networkBridge)
		return err
	}

	dr.networkBridgeID = resp.ID

	log.Info().Msgf("Succesfully created docker network %v", dr.networkBridge)
	return nil
}

func (dr *dockerRunnerImpl) deleteBridgeNetwork(ctx context.Context) error {

	if dr.networkBridgeID != "" {
		log.Info().Msgf("Deleting docker network %v with id %v...", dr.networkBridge, dr.networkBridgeID)

		err := dr.dockerClient.NetworkRemove(ctx, dr.networkBridgeID)

		if err != nil {
			log.Error().Err(err).Msgf("Failed deleting docker network %v with id %v", dr.networkBridge, dr.networkBridgeID)
			return err
		}

		log.Info().Msgf("Succesfully deleted docker network %v with id %v", dr.networkBridge, dr.networkBridgeID)
	}

	return nil
}
