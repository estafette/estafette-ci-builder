package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type estafettePipelineRunResult struct {
	Pipeline           estafettePipeline
	DockerPullDuration time.Duration
	DockerPullError    error
	DockerRunDuration  time.Duration
	DockerRunError     error
	Status             string
	Detail             string
}

func runDockerPull(p estafettePipeline) (err error) {

	fmt.Printf("[estafette] Pulling docker container '%v'\n", p.ContainerImage)

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
	if _, err = cli.ContainerWait(ctx, resp.ID); err != nil {
		return err
	}

	return
}

func runPipeline(dir string, envvars map[string]string, p estafettePipeline) (result estafettePipelineRunResult, err error) {

	result.Pipeline = p

	fmt.Printf("[estafette] Starting pipeline '%v'\n", p.Name)

	// pull docker image
	dockerPullStart := time.Now()
	result.DockerPullError = runDockerPull(p)
	result.DockerPullDuration = time.Since(dockerPullStart)
	if result.DockerPullError != nil {
		return result, result.DockerPullError
	}

	// run commands in docker container
	dockerRunStart := time.Now()
	result.DockerRunError = runDockerRun(dir, envvars, p)
	result.DockerRunDuration = time.Since(dockerRunStart)
	if result.DockerRunError != nil {
		return result, result.DockerRunError
	}

	fmt.Printf("[estafette] Finished pipeline '%v' successfully\n", p.Name)

	return
}

type estafetteRunPipelinesResult struct {
	PipelineResults []estafettePipelineRunResult
	Errors          []error
}

func runPipelines(manifest estafetteManifest, dir string, envvars map[string]string) (result estafetteRunPipelinesResult, firstErr error) {

	for _, p := range manifest.Pipelines {

		if firstErr != nil {

			// if an error has happened in one of the previous steps we still want to render the following steps in the result table
			r := estafettePipelineRunResult{
				Pipeline: *p,
				Status:   "SKIPPED",
			}
			result.PipelineResults = append(result.PipelineResults, r)

			continue
		}

		r, err := runPipeline(dir, envvars, *p)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}

			r.Status = "FAILED"
			if r.DockerPullError != nil {
				r.Detail = r.DockerPullError.Error()
			} else if r.DockerRunError != nil {
				r.Detail = r.DockerRunError.Error()
			} else {
				r.Detail = err.Error()
			}

			result.PipelineResults = append(result.PipelineResults, r)

			continue
		}

		r.Status = "SUCCEEDED"
		result.PipelineResults = append(result.PipelineResults, r)
	}

	return
}
