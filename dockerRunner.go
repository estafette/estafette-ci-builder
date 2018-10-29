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
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker-ce/components/engine/client"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"

	"github.com/rs/zerolog/log"
)

// DockerRunner pulls and runs docker containers
type DockerRunner interface {
	isDockerImagePulled(manifest.EstafetteStage) bool
	runDockerPull(manifest.EstafetteStage) error
	getDockerImageSize(manifest.EstafetteStage) (int64, error)
	runDockerRun(string, map[string]string, manifest.EstafetteStage) ([]contracts.BuildLogLine, int64, error)

	startDockerDaemon() error
	waitForDockerDaemon()
	createDockerClient() (*client.Client, error)
	getImagePullOptions(containerImage string) types.ImagePullOptions
}

type dockerRunnerImpl struct {
	envvarHelper EnvvarHelper
	obfuscator   Obfuscator
	dockerClient *client.Client
	runAsJob     bool
	config       contracts.BuilderConfig
}

// NewDockerRunner returns a new DockerRunner
func NewDockerRunner(envvarHelper EnvvarHelper, obfuscator Obfuscator, runAsJob bool, config contracts.BuilderConfig) DockerRunner {
	return &dockerRunnerImpl{
		envvarHelper: envvarHelper,
		obfuscator:   obfuscator,
		runAsJob:     runAsJob,
		config:       config,
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

func (dr *dockerRunnerImpl) runDockerPull(p manifest.EstafetteStage) (err error) {

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

func (dr *dockerRunnerImpl) runDockerRun(dir string, envvars map[string]string, p manifest.EstafetteStage) (logLines []contracts.BuildLogLine, exitCode int64, err error) {

	logLines = make([]contracts.BuildLogLine, 0)
	exitCode = -1

	// check if image is trusted image
	trustedImage := dr.config.GetTrustedImage(p.ContainerImage)

	// run docker with image and commands from yaml
	ctx := context.Background()

	// define commands
	cmdSlice := make([]string, 0)
	cmdSlice = append(cmdSlice, "set -e;"+strings.Join(p.Commands, ";"))

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
				log.Warn().Interface("customProperty", v).Msgf("Cannot turn custom property %v into extension envvar", k)
			}
		}

		// also add add custom properties as json object in ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES envvar
		customPropertiesBytes, err := json.Marshal(p.CustomProperties)
		if err == nil {
			extensionEnvVars["ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES"] = string(customPropertiesBytes)
		} else {
			log.Warn().Err(err).Interface("customProperty", p.CustomProperties).Msg("Cannot marshal custom properties for ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES envvar")
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

	// define entrypoint
	entrypoint := make([]string, 0)
	entrypoint = append(entrypoint, p.Shell)
	entrypoint = append(entrypoint, "-c")

	// define binds
	binds := make([]string, 0)
	if runtime.GOOS != "windows" {
		binds = append(binds, fmt.Sprintf("%v:%v", dir, os.Expand(p.WorkingDirectory, dr.envvarHelper.getEstafetteEnv)))
	}

	// check if this is a trusted image with RunDocker set to true
	if trustedImage != nil && trustedImage.RunDocker {
		if ok, _ := pathExists("/var/run/docker.sock"); ok {
			binds = append(binds, "/var/run/docker.sock:/var/run/docker.sock")
		}
	}

	// if ok, _ := pathExists("/var/run/secrets/kubernetes.io/serviceaccount"); ok {
	// 	binds = append(binds, "/var/run/secrets/kubernetes.io/serviceaccount:/var/run/secrets/kubernetes.io/serviceaccount")
	// }

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
		config.Cmd = cmdSlice
		// only override entrypoint when commands are set, so extensions can work without commands
		config.Entrypoint = entrypoint
	}

	// check if this is a trusted image with RunPrivileged or RunDocker set to true
	privileged := false
	if trustedImage != nil {
		privileged = trustedImage.RunDocker || trustedImage.RunPrivileged
	}

	// create container
	resp, err := dr.dockerClient.ContainerCreate(ctx, &config, &container.HostConfig{
		Binds:      binds,
		AutoRemove: true,
		Privileged: privileged,
	}, &network.NetworkingConfig{}, "")
	if err != nil {
		return
	}

	// start container
	if err := dr.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return logLines, exitCode, err
	}

	// follow logs
	rc, err := dr.dockerClient.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: false,
		Follow:     true,
		Details:    false,
	})
	defer rc.Close()
	if err != nil {
		return
	}

	// stream logs to stdout with buffering
	in := bufio.NewReader(rc)
	var readError error
	for {

		// strip first 8 bytes, they contain docker control characters (https://github.com/docker/docker-ce/blob/v18.06.1-ce/components/engine/client/container_logs.go#L23-L32)
		logLine, readError := in.ReadBytes('\n')

		if readError != nil {
			break
		}

		// skip this log line if it has no docker log headers
		// if len(logLine) <= 8 {
		// 	continue
		// }

		// inspect the docker log header for stream type

		// first byte contains the streamType
		// -   0: stdin (will be written on stdout)
		// -   1: stdout
		// -   2: stderr
		// -   3: system error
		streamType := "stdout"
		if len(logLines) > 0 {
			switch logLine[0] {
			case 1:
				streamType = "stdout"
			case 2:
				streamType = "stderr"
				// default:
				// 	continue
			}
		}

		// strip headers and obfuscate secret values
		// logLineString := dr.obfuscator.Obfuscate(string(logLine[8:]))
		logLineString := dr.obfuscator.Obfuscate(fmt.Sprintf("%x: %v", logLine[0], string(logLine)))

		// create object for tailing logs and storing in the db when done
		logLineObject := contracts.BuildLogLine{
			Timestamp:  time.Now().UTC(),
			StreamType: streamType,
			Text:       logLineString,
		}

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
		return logLines, exitCode, readError
	}

	// wait for container to stop running
	resultC, errC := dr.dockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)

	select {
	case result := <-resultC:
		exitCode = result.StatusCode
	case err = <-errC:
		return
	}

	if exitCode != 0 {
		return logLines, exitCode, fmt.Errorf("Failed with exit code: %v", exitCode)
	}

	return
}

func (dr *dockerRunnerImpl) startDockerDaemon() error {

	// dockerd --host=unix:///var/run/docker.sock --host=tcp://0.0.0.0:2375 --storage-driver=$STORAGE_DRIVER &
	log.Debug().Msg("Starting docker daemon...")
	args := []string{"--host=unix:///var/run/docker.sock", "--host=tcp://0.0.0.0:2375", "--storage-driver=overlay2"}
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

					return types.ImagePullOptions{
						RegistryAuth: authStr,
					}
				}
				log.Error().Err(err).Msgf("Failed marshaling docker auth config for container image %v", containerImage)
				break
			}
		}
	}

	return types.ImagePullOptions{}
}
