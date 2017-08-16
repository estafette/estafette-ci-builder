package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/registry"

	"github.com/rs/zerolog/log"
)

func isDockerImagePulled(p estafettePipeline) bool {

	log.Info().Msgf("Checking if docker image '%v' exists locally...", p.ContainerImage)

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

func runDockerPull(p estafettePipeline) (err error) {

	log.Info().Msgf("Pulling docker image '%v'", p.ContainerImage)

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

func getDockerImageSize(p estafettePipeline) (totalSize int64, err error) {

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

func runDockerRun(dir string, envvars map[string]string, p estafettePipeline) (err error) {

	// run docker with image and commands from yaml
	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	// define commands
	cmdSlice := make([]string, 0)
	cmdSlice = append(cmdSlice, "set -e;"+strings.Join(p.Commands, ";"))

	// define envvars
	envVars := make([]string, 0)
	if envvars != nil && len(envvars) > 0 {
		for k, v := range envvars {
			envVars = append(envVars, fmt.Sprintf("%v=%v", k, v))
		}
	}

	// define entrypoint
	entrypoint := make([]string, 0)
	entrypoint = append(entrypoint, p.Shell)
	entrypoint = append(entrypoint, "-c")

	// define binds
	binds := make([]string, 0)
	binds = append(binds, fmt.Sprintf("%v:%v", dir, os.Expand(p.WorkingDirectory, getEstafetteEnv)))
	if ok, _ := pathExists("/var/run/docker.sock"); ok {
		binds = append(binds, "/var/run/docker.sock:/var/run/docker.sock")
	}
	if ok, _ := pathExists("/var/run/secrets/kubernetes.io/serviceaccount"); ok {
		binds = append(binds, "/var/run/secrets/kubernetes.io/serviceaccount:/var/run/secrets/kubernetes.io/serviceaccount")
	}

	// create container
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		AttachStdout: true,
		AttachStderr: true,
		Env:          envVars,
		Cmd:          cmdSlice,
		Image:        p.ContainerImage,
		WorkingDir:   os.Expand(p.WorkingDirectory, getEstafetteEnv),
		Entrypoint:   entrypoint,
	}, &container.HostConfig{
		Binds:      binds,
		AutoRemove: true,
		Privileged: true,
	}, &network.NetworkingConfig{}, "")
	if err != nil {
		return err
	}

	// start container
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	// follow logs
	rc, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	defer rc.Close()
	if err != nil {
		return err
	}

	// stream logs to stdout with buffering
	in := bufio.NewScanner(rc)
	for in.Scan() {

		// strip first 8 bytes, they contain docker control characters (https://github.com/docker/docker/issues/7375)
		logLine := in.Text()
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
			log.Warn().Msgf("[%v] %v", p.Name, logLine)
		} else {
			log.Info().Msgf("[%v] %v", p.Name, logLine)
		}
	}
	if err := in.Err(); err != nil {
		log.Error().Msgf("[%v] Error: %v", p.Name, err)
		return err
	}

	// wait for container to stop run
	exitCode, err := cli.ContainerWait(ctx, resp.ID)
	if err != nil {
		return err
	}

	if exitCode > 0 {
		return fmt.Errorf("Failed with exit code: %v", exitCode)
	}

	return
}

func startDockerDaemon() (dmn *daemon.Daemon, err error) {

	//func NewDaemon(config *config.Config, registryService registry.Service, containerdRemote libcontainerd.Remote) (daemon *Daemon, err error) {

	dmn, err = daemon.NewDaemon(&config.Config{
		CommonConfig: config.CommonConfig{
			Hosts:       []string{"unix:///var/run/docker.sock", "tcp://0.0.0.0:2375"},
			GraphDriver: "overlay2",
		},
	}, registry.NewService(registry.ServiceOptions{}), nil)
	if err != nil {
		return
	}

	return
}

func shutdownDockerDaemon(daemon *daemon.Daemon) error {
	err := daemon.Shutdown()
	if err != nil {
		return err
	}

	return nil
}
