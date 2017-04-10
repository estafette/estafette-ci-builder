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
)

func isDockerImagePulled(p estafettePipeline) bool {

	fmt.Printf("[estafette] Checking if docker image '%v' exists...\n", p.ContainerImage)

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

	fmt.Printf("[estafette] Pulling docker image '%v'\n", p.ContainerImage)

	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	rc, err := cli.ImagePull(context.Background(), p.ContainerImage, types.ImagePullOptions{})
	defer rc.Close()
	if err != nil {
		return err
	}

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
	//summaries, err := cli.ImageList(context.Background(), type.ImageListOptions{})
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
	cmdSlice = append(cmdSlice, p.Shell)
	cmdSlice = append(cmdSlice, "-c")
	cmdSlice = append(cmdSlice, "set -e;"+os.ExpandEnv(strings.Join(p.Commands, ";")))

	// define envvars
	envVars := make([]string, 0)
	if envvars != nil && len(envvars) > 0 {
		for k, v := range envvars {
			envVars = append(envVars, fmt.Sprintf("\"%v=%v\"", k, v))
		}
	}

	// define entrypoint
	entrypoint := make([]string, 0)
	entrypoint = append(entrypoint, "")

	// define binds
	binds := make([]string, 0)
	binds = append(binds, fmt.Sprintf("%v:%v", dir, os.ExpandEnv(p.WorkingDirectory)))
	binds = append(binds, "/var/run/docker.sock:/var/run/docker.sock")
	binds = append(binds, "/var/run/secrets/kubernetes.io/serviceaccount:/var/run/secrets/kubernetes.io/serviceaccount")

	// create container
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		AttachStdout: true,
		AttachStderr: true,
		Env:          envVars,
		Cmd:          cmdSlice,
		Image:        p.ContainerImage,
		WorkingDir:   os.ExpandEnv(p.WorkingDirectory),
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
		output := os.Stdout
		if len(logLine) > 8 {

			headers := []byte(logLine[0:8])

			// first byte contains the streamType
			// -   0: stdin (will be written on stdout)
			// -   1: stdout
			// -   2: stderr
			streamType := headers[0]

			if streamType == 2 {
				output = os.Stderr
			}

			logLine = logLine[8:]
		}

		fmt.Fprintf(output, "[estafette] [%v] %v\n", p.Name, logLine)
	}
	if err := in.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[estafette] [%v] Error: %v\n", p.Name, err)
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
