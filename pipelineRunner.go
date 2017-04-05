package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/docker/docker/api/types"
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

	containerImage := p.ContainerImage
	if !strings.Contains(containerImage, "/") {
		containerImage = "docker.io/library/" + containerImage
	}

	_, err = cli.ImagePull(context.Background(), containerImage, types.ImagePullOptions{})
	if err != nil {
		return stat, err
	}

	// cmd := "docker"

	// // add docker command and options
	// argsSlice := make([]string, 0)
	// argsSlice = append(argsSlice, "pull")
	// argsSlice = append(argsSlice, p.ContainerImage)

	// fmt.Printf("[estafette] Running command '%v %v'\n", cmd, strings.Join(argsSlice, " "))
	// dockerPullCmd := exec.Command(cmd, argsSlice...)

	// // run and wait until completion
	// if err := dockerPullCmd.Run(); err != nil {
	// 	if exiterr, ok := err.(*exec.ExitError); ok {
	// 		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok && status.ExitStatus() > 0 {
	// 			stat.ExitCode = status.ExitStatus()
	// 			return stat, err
	// 		}
	// 	} else {
	// 		return stat, err
	// 	}
	// }

	stat.Duration = time.Since(start)

	return
}

func runDockerRun(dir string, envvars map[string]string, p estafettePipeline) (stat dockerRunStat, err error) {

	// run docker with image and commands from yaml
	start := time.Now()

	// ctx := context.Background()
	// cli, err := client.NewEnvClient()
	// if err != nil {
	// 	return stat, err
	// }

	// cmdSlice := make([]string, 0)
	// cmdSlice = append(cmdSlice, p.Shell)
	// cmdSlice = append(cmdSlice, "-c")
	// cmdSlice = append(cmdSlice, "set -e;"+os.ExpandEnv(strings.Join(p.Commands, ";")))

	// resp, err := cli.ContainerCreate(ctx, &container.Config{

	// 	// Hostname        string              // Hostname
	// 	// Domainname      string              // Domainname
	// 	// User            string              // User that will run the command(s) inside the container, also support user:group
	// 	// AttachStdin     bool                // Attach the standard input, makes possible user interaction
	// 	// AttachStdout    bool                // Attach the standard output
	// 	// AttachStderr    bool                // Attach the standard error
	// 	// ExposedPorts    nat.PortSet         `json:",omitempty"` // List of exposed ports
	// 	// Tty             bool                // Attach standard streams to a tty, including stdin if it is not closed.
	// 	// OpenStdin       bool                // Open stdin
	// 	// StdinOnce       bool                // If true, close stdin after the 1 attached client disconnects.
	// 	// Env             []string            // List of environment variable to set in the container
	// 	// Cmd             strslice.StrSlice   // Command to run when starting the container
	// 	// Healthcheck     *HealthConfig       `json:",omitempty"` // Healthcheck describes how to check the container is healthy
	// 	// ArgsEscaped     bool                `json:",omitempty"` // True if command is already escaped (Windows specific)
	// 	// Image           string              // Name of the image as it was passed by the operator (e.g. could be symbolic)
	// 	// Volumes         map[string]struct{} // List of volumes (mounts) used for the container
	// 	// WorkingDir      string              // Current directory (PWD) in the command will be launched
	// 	// Entrypoint      strslice.StrSlice   // Entrypoint to run when starting the container
	// 	// NetworkDisabled bool                `json:",omitempty"` // Is network disabled
	// 	// MacAddress      string              `json:",omitempty"` // Mac Address of the container
	// 	// OnBuild         []string            // ONBUILD metadata that were defined on the image Dockerfile
	// 	// Labels          map[string]string   // List of labels set to this container
	// 	// StopSignal      string              `json:",omitempty"` // Signal to stop a container
	// 	// StopTimeout     *int                `json:",omitempty"` // Timeout (in seconds) to stop a container
	// 	// Shell           strslice.StrSlice   `json:",omitempty"` // Shell for shell-form of RUN, CMD, ENTRYPOINT

	// 	Image: p.ContainerImage,
	// 	Cmd:   cmdSlice,
	// }, nil, nil, "")
	// if err != nil {
	// 	return stat, err
	// }

	// if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
	// 	return stat, err
	// }

	// if _, err = cli.ContainerWait(ctx, resp.ID); err != nil {
	// 	return stat, err
	// }

	// out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	// if err != nil {
	// 	return stat, err
	// }

	// io.Copy(os.Stdout, out)

	cmd := "docker"

	// add docker command and options
	argsSlice := make([]string, 0)
	argsSlice = append(argsSlice, "run")
	argsSlice = append(argsSlice, "--privileged")
	argsSlice = append(argsSlice, "--rm")
	argsSlice = append(argsSlice, "--entrypoint")
	argsSlice = append(argsSlice, "")
	argsSlice = append(argsSlice, fmt.Sprintf("--volume=%v:%v", dir, os.ExpandEnv(p.WorkingDirectory)))
	argsSlice = append(argsSlice, "--volume=/var/run/docker.sock:/var/run/docker.sock")
	argsSlice = append(argsSlice, "--volume=/var/run/secrets/kubernetes.io/serviceaccount:/var/run/secrets/kubernetes.io/serviceaccount")
	argsSlice = append(argsSlice, fmt.Sprintf("--workdir=%v", os.ExpandEnv(p.WorkingDirectory)))
	if envvars != nil && len(envvars) > 0 {
		for k, v := range envvars {
			argsSlice = append(argsSlice, "-e")
			argsSlice = append(argsSlice, fmt.Sprintf("\"%v=%v\"", k, v))
		}
	}

	// the actual container to run
	argsSlice = append(argsSlice, p.ContainerImage)

	// the commands to execute in the container
	argsSlice = append(argsSlice, p.Shell)
	argsSlice = append(argsSlice, "-c")
	argsSlice = append(argsSlice, "set -e;"+os.ExpandEnv(strings.Join(p.Commands, ";")))

	fmt.Printf("[estafette] Running command '%v %v'\n", cmd, strings.Join(argsSlice, " "))
	dockerRunCmd := exec.Command(cmd, argsSlice...)

	// pipe logs
	stdout, err := dockerRunCmd.StdoutPipe()
	if err != nil {
		return stat, err
	}
	stderr, err := dockerRunCmd.StderrPipe()
	if err != nil {
		return stat, err
	}

	// start
	if err := dockerRunCmd.Start(); err != nil {
		return stat, err
	}

	// tail logs
	multi := io.MultiReader(stdout, stderr)

	in := bufio.NewScanner(multi)

	for in.Scan() {
		fmt.Printf("[estafette] [%v] %s\n", p.Name, in.Text()) // write each line to your log, or anything you need
	}
	if err := in.Err(); err != nil {
		fmt.Printf("[estafette] [%v] Error: %s\n", p.Name, err)
	}

	// wait for completion
	if err := dockerRunCmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok && status.ExitStatus() > 0 {
				stat.ExitCode = status.ExitStatus()
				return stat, err
			}
		} else {
			return stat, err
		}
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
