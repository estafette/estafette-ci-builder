package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type dockerPullStat struct {
	ExitCode int
	Duration time.Duration
}

type dockerRunStat struct {
	ExitCode int
	Duration time.Duration
}

type estafettePipelineStat struct {
	Pipeline       estafettePipeline
	DockerPullStat dockerPullStat
	DockerRunStat  dockerRunStat
}

func (c *estafettePipelineStat) ExitCode() int {
	if c.DockerPullStat.ExitCode > 0 {
		return c.DockerPullStat.ExitCode
	}
	if c.DockerRunStat.ExitCode > 0 {
		return c.DockerRunStat.ExitCode
	}
	return 0
}

func runDockerPull(p estafettePipeline) (stat dockerPullStat, err error) {

	start := time.Now()

	fmt.Printf("[estafette] Pulling docker container '%v'\n", p.ContainerImage)

	cli, err := client.NewEnvClient()
	if err != nil {
		return stat, err
	}

	rc, err := cli.ImagePull(context.Background(), p.ContainerImage, types.ImagePullOptions{})
	defer rc.Close()
	if err != nil {
		return stat, err
	}

	// wait for image pull to finish
	ioutil.ReadAll(rc)

	stat.Duration = time.Since(start)

	return
}

func runDockerRun(dir string, envvars map[string]string, p estafettePipeline) (stat dockerRunStat, err error) {

	// run docker with image and commands from yaml
	start := time.Now()

	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		return stat, err
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
		return stat, err
	}

	// start container
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return stat, err
	}

	// follow logs
	rc, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	defer rc.Close()
	if err != nil {
		return stat, err
	}

	// stream logs to stdout with buffering
	in := bufio.NewScanner(rc)
	for in.Scan() {

		// strip first 8 bytes, they contain docker control characters (https://github.com/docker/docker/issues/7375)
		fmt.Printf("[estafette] [%v] %v\n", p.Name, in.Text()[8:])
	}
	if err := in.Err(); err != nil {
		fmt.Printf("[estafette] [%v] Error: %v\n", p.Name, err)
	}

	// wait for container to stop run
	if _, err = cli.ContainerWait(ctx, resp.ID); err != nil {
		return stat, err
	}

	stat.Duration = time.Since(start)

	return
}

// https://gist.github.com/elwinar/14e1e897fdbe4d3432e1
func toUpperSnake(in string) string {
	runes := []rune(in)
	length := len(runes)

	var out []rune
	for i := 0; i < length; i++ {
		if i > 0 && unicode.IsUpper(runes[i]) && ((i+1 < length && unicode.IsLower(runes[i+1])) || unicode.IsLower(runes[i-1])) {
			out = append(out, '_')
		}
		out = append(out, unicode.ToUpper(runes[i]))
	}

	return string(out)
}

func collectEstafetteEnvvars(m estafetteManifest) (envvars map[string]string) {

	envvars = map[string]string{}

	for _, e := range os.Environ() {
		kvPair := strings.Split(e, "=")
		if len(kvPair) == 2 {
			envvarName := kvPair[0]
			envvarValue := kvPair[1]

			if strings.HasPrefix(envvarName, "ESTAFETTE_") {
				envvars[envvarName] = envvarValue
			}
		}
	}

	// add the labels as envvars
	if m.Labels != nil && len(m.Labels) > 0 {
		for key, value := range m.Labels {

			envvarName := "ESTAFETTE_LABEL_" + toUpperSnake(key)
			envvars[envvarName] = value

			os.Setenv(envvarName, value)
		}
	}

	return
}

func runPipeline(dir string, envvars map[string]string, p estafettePipeline) (stat estafettePipelineStat, err error) {

	stat.Pipeline = p

	fmt.Printf("[estafette] Starting pipeline '%v'\n", p.Name)

	// pull docker image
	stat.DockerPullStat, err = runDockerPull(p)
	if err != nil {
		return
	}

	stat.DockerRunStat, err = runDockerRun(dir, envvars, p)
	if err != nil {
		return
	}

	fmt.Printf("[estafette] Finished pipeline '%v' successfully\n", p.Name)

	return
}
