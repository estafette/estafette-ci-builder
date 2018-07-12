package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	manifest "github.com/estafette/estafette-ci-manifest"

	"github.com/rs/zerolog/log"
)

// DockerRunner pulls and runs docker containers
type DockerRunner interface {
	isDockerImagePulled(manifest.EstafettePipeline) bool
	runDockerPull(manifest.EstafettePipeline) error
	getDockerImageSize(manifest.EstafettePipeline) (int64, error)
	runDockerRun(string, map[string]string, manifest.EstafettePipeline) ([]buildJobLogLine, int64, error)
	startDockerDaemon() error
	waitForDockerDaemon()
}

type dockerRunnerImpl struct {
	envvarHelper EnvvarHelper
}

// NewDockerRunner returns a new DockerRunner
func NewDockerRunner(envvarHelper EnvvarHelper) DockerRunner {
	return &dockerRunnerImpl{
		envvarHelper: envvarHelper,
	}
}

func (dr *dockerRunnerImpl) isDockerImagePulled(p manifest.EstafettePipeline) bool {

	log.Info().Msgf("[%v] Checking if docker image '%v' exists locally...", p.Name, p.ContainerImage)

	cli, err := client.NewEnvClient()
	if err != nil {
		return false
	}

	imageSummaries, err := cli.ImageList(context.Background(), types.ImageListOptions{})

	for _, summary := range imageSummaries {
		if contains(summary.RepoTags, p.ContainerImage) {
			return true
		}
	}

	return false
}

func (dr *dockerRunnerImpl) runDockerPull(p manifest.EstafettePipeline) (err error) {

	log.Info().Msgf("[%v] Pulling docker image '%v'", p.Name, p.ContainerImage)

	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	rc, err := cli.ImagePull(context.Background(), p.ContainerImage, types.ImagePullOptions{})
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

func (dr *dockerRunnerImpl) getDockerImageSize(p manifest.EstafettePipeline) (totalSize int64, err error) {

	cli, err := client.NewEnvClient()
	if err != nil {
		return totalSize, err
	}

	items, err := cli.ImageHistory(context.Background(), p.ContainerImage)
	if err != nil {
		return totalSize, err
	}

	for _, item := range items {
		totalSize += item.Size
	}

	return totalSize, nil
}

func (dr *dockerRunnerImpl) runDockerRun(dir string, envvars map[string]string, p manifest.EstafettePipeline) (logLines []buildJobLogLine, exitCode int64, err error) {

	logLines = make([]buildJobLogLine, 0)
	exitCode = -1

	// run docker with image and commands from yaml
	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		return
	}

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
				extensionEnvVars[extensionkey] = os.Expand(s, dr.envvarHelper.getEstafetteEnv)
			} else if s, isBool := v.(bool); isBool {
				// if custom property is of type bool add the envvar
				extensionEnvVars[extensionkey] = os.Expand(strconv.FormatBool(s), dr.envvarHelper.getEstafetteEnv)
			} else if s, isInt := v.(int); isInt {
				// if custom property is of type bool add the envvar
				extensionEnvVars[extensionkey] = os.Expand(strconv.FormatInt(int64(s), 10), dr.envvarHelper.getEstafetteEnv)
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
					extensionEnvVars[extensionkey] = os.Expand(strings.Join(stringValues, ","), dr.envvarHelper.getEstafetteEnv)
				} else {
					log.Warn().Interface("customProperty", v).Msgf("Cannot turn custom property %v into extension envvar", k)
				}
			} else {
				log.Warn().Interface("customProperty", v).Msgf("Cannot turn custom property %v into extension envvar", k)
			}
		}
	}

	// combine and override estafette and global envvars with pipeline envvars
	combinedEnvVars := dr.envvarHelper.overrideEnvvars(envvars, p.EnvVars, extensionEnvVars)

	// decrypt secrets
	combinedEnvVars = dr.envvarHelper.decryptSecrets(combinedEnvVars)

	// define docker envvars
	dockerEnvVars := make([]string, 0)
	if combinedEnvVars != nil && len(combinedEnvVars) > 0 {
		for k, v := range combinedEnvVars {
			dockerEnvVars = append(dockerEnvVars, fmt.Sprintf("%v=%v", k, v))
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
	if ok, _ := pathExists("/var/run/docker.sock"); ok {
		binds = append(binds, "/var/run/docker.sock:/var/run/docker.sock")
	}
	if ok, _ := pathExists("/var/run/secrets/kubernetes.io/serviceaccount"); ok {
		binds = append(binds, "/var/run/secrets/kubernetes.io/serviceaccount:/var/run/secrets/kubernetes.io/serviceaccount")
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
		// only pass commands when they are set, so extensions can work without
		config.Cmd = cmdSlice
		// only override entrypoint when commands are set, so extensions can work without commands
		config.Entrypoint = entrypoint
	}

	// create container
	resp, err := cli.ContainerCreate(ctx, &config, &container.HostConfig{
		Binds:      binds,
		AutoRemove: true,
		Privileged: true,
	}, &network.NetworkingConfig{}, "")
	if err != nil {
		return
	}

	// start container
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return logLines, exitCode, err
	}

	// follow logs
	rc, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	defer rc.Close()
	if err != nil {
		return
	}

	// stream logs to stdout with buffering
	in := bufio.NewReader(rc)
	var readError error
	for {

		// strip first 8 bytes, they contain docker control characters (https://github.com/docker/docker/issues/7375)
		logLine, readError := in.ReadBytes('\n')

		if readError != nil {
			break
		}

		logType := "stdout"
		if len(logLine) > 8 {

			headers := []byte(logLine[0:8])

			// first byte contains the streamType
			// -   0: stdin (will be written on stdout)
			// -   1: stdout
			// -   2: stderr
			streamType := headers[0]

			if streamType == 2 {
				logType = "stderr"
			}

			logLine = logLine[8:]
		}

		if logType == "stderr" {
			log.Info().Msgf("[%v] %v", p.Name, string(logLine))
		} else {
			log.Info().Msgf("[%v] %v", p.Name, string(logLine))
		}

		logLines = append(logLines, buildJobLogLine{
			timestamp: time.Now().UTC(),
			logLevel:  logType,
			logText:   string(logLine),
		})
	}

	if readError != nil && readError != io.EOF {
		log.Error().Msgf("[%v] Error: %v", p.Name, readError)
		return logLines, exitCode, readError
	}

	// wait for container to stop run
	exitCode, err = cli.ContainerWait(ctx, resp.ID)
	if err != nil {
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
