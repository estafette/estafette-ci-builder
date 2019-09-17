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
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

// DockerRunner pulls and runs docker containers
type DockerRunner interface {
	isDockerImagePulled(manifest.EstafetteStage) bool
	runDockerPull(context.Context, manifest.EstafetteStage) error
	getDockerImageSize(manifest.EstafetteStage) (int64, error)
	runDockerRun(context.Context, string, map[string]string, manifest.EstafetteStage) ([]contracts.BuildLogLine, int64, bool, error)

	startDockerDaemon() error
	waitForDockerDaemon()
	createDockerClient() (*client.Client, error)
	getImagePullOptions(containerImage string) types.ImagePullOptions
	isTrustedImage(manifest.EstafetteStage) bool
	stopContainerOnCancellation()
}

type dockerRunnerImpl struct {
	envvarHelper        EnvvarHelper
	obfuscator          Obfuscator
	dockerClient        *client.Client
	runAsJob            bool
	config              contracts.BuilderConfig
	cancellationChannel chan struct{}
	containerID         string
	canceled            bool
}

// NewDockerRunner returns a new DockerRunner
func NewDockerRunner(envvarHelper EnvvarHelper, obfuscator Obfuscator, runAsJob bool, config contracts.BuilderConfig, cancellationChannel chan struct{}) DockerRunner {
	return &dockerRunnerImpl{
		envvarHelper:        envvarHelper,
		obfuscator:          obfuscator,
		runAsJob:            runAsJob,
		config:              config,
		cancellationChannel: cancellationChannel,
	}
}

func (dr *dockerRunnerImpl) isDockerImagePulled(p manifest.EstafetteStage) bool {

	log.Info().Msgf("[%v] Checking if docker image '%v' exists locally...", p.Name, p.ContainerImage)

	imageSummaries, err := dr.dockerClient.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		return false
	}

	for _, summary := range imageSummaries {
		if contains(summary.RepoTags, p.ContainerImage) {
			return true
		}
	}

	return false
}

func (dr *dockerRunnerImpl) runDockerPull(ctx context.Context, p manifest.EstafetteStage) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "DockerPull")
	defer span.Finish()
	span.SetTag("docker-image", p.ContainerImage)

	log.Info().Msgf("[%v] Pulling docker image '%v'", p.Name, p.ContainerImage)

	rc, err := dr.dockerClient.ImagePull(context.Background(), p.ContainerImage, dr.getImagePullOptions(p.ContainerImage))
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

func (dr *dockerRunnerImpl) getDockerImageSize(p manifest.EstafetteStage) (totalSize int64, err error) {

	items, err := dr.dockerClient.ImageHistory(context.Background(), p.ContainerImage)
	if err != nil {
		return totalSize, err
	}

	for _, item := range items {
		totalSize += item.Size
	}

	return totalSize, nil
}

func (dr *dockerRunnerImpl) runDockerRun(ctx context.Context, dir string, envvars map[string]string, p manifest.EstafetteStage) ([]contracts.BuildLogLine, int64, bool, error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "DockerRun")
	defer span.Finish()
	span.SetTag("docker-image", p.ContainerImage)

	logLines := make([]contracts.BuildLogLine, 0)
	lineNumber := 1
	exitCode := int64(-1)

	// check if image is trusted image
	trustedImage := dr.config.GetTrustedImage(p.ContainerImage)

	// run docker with image and commands from yaml

	// define entrypoint
	entrypoint := make([]string, 0)
	if runtime.GOOS == "windows" {
		if p.Shell == "powershell" {
			entrypoint = []string{"powershell", "$ErrorActionPreference = 'Stop';", "$ProgressPreference = 'SilentlyContinue';"}
		} else {
			entrypoint = []string{"cmd", "/S", "/C"}
		}
	} else {
		entrypoint = []string{p.Shell, "-c", "set -e;"}
	}

	// define commands
	cmdSeparator := ";"
	if runtime.GOOS == "windows" && p.Shell != "powershell" {
		cmdSeparator = " &&"
	}
	cmdSlice := make([]string, 0)
	for _, c := range p.Commands {
		cmdSlice = append(cmdSlice, c+cmdSeparator)
	}

	log.Debug().Msgf("> %v %v", strings.Join(entrypoint, " "), strings.Join(cmdSlice, " "))

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
			err := fmt.Errorf("This trusted image does not allow for commands to be set as a protection against snooping injected credentials")
			return logLines, exitCode, dr.canceled, err
		}

		// only pass commands when they are set, so extensions can work without
		config.Cmd = cmdSlice
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
	}, &network.NetworkingConfig{}, "")
	if err != nil {
		return logLines, exitCode, dr.canceled, err
	}
	dr.containerID = resp.ID

	// start container
	if err := dr.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return logLines, exitCode, dr.canceled, err
	}

	// follow logs
	rc, err := dr.dockerClient.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
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
			log.Debug().Msgf("Cancelled tailing logs for container %v", dr.containerID)
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
				Step:    p.Name,
				LogLine: &logLineObject,
			}
			log.Info().Interface("tailLogLine", tailLogLine).Msg("")
		} else {
			log.Info().Msgf("[%v] %v", p.Name, logLineString)
		}

		// add to log lines send when build/release job is finished
		logLines = append(logLines, logLineObject)
	}

	if readError != nil && readError != io.EOF {
		log.Error().Msgf("[%v] Error: %v", p.Name, readError)
		return logLines, exitCode, dr.canceled, readError
	}

	// wait for container to stop running
	resultC, errC := dr.dockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)

	select {
	case result := <-resultC:
		exitCode = result.StatusCode
	case err = <-errC:
		log.Warn().Err(err).Msgf("Container %v exited with error", dr.containerID)
		return logLines, exitCode, dr.canceled, err
	}

	// clear container id
	dr.containerID = ""

	if exitCode != 0 {
		return logLines, exitCode, dr.canceled, fmt.Errorf("Failed with exit code: %v", exitCode)
	}

	return logLines, exitCode, dr.canceled, err
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

func (dr *dockerRunnerImpl) isTrustedImage(p manifest.EstafetteStage) bool {

	log.Info().Msgf("[%v] Checking if docker image '%v' is trusted...", p.Name, p.ContainerImage)

	// check if image is trusted image
	trustedImage := dr.config.GetTrustedImage(p.ContainerImage)

	return trustedImage != nil
}

func (dr *dockerRunnerImpl) stopContainerOnCancellation() {
	// wait for cancellation
	<-dr.cancellationChannel

	dr.canceled = true

	if dr.containerID != "" {
		timeout := 20 * time.Second
		err := dr.dockerClient.ContainerStop(context.Background(), dr.containerID, &timeout)
		if err != nil {
			log.Warn().Err(err).Msgf("Stopping container %v for cancellation failed", dr.containerID)
		} else {
			log.Info().Msgf("Stopped container %v for cancellation", dr.containerID)
		}
	} else {
		log.Info().Msg("No container to stop for cancellation")
	}
}
