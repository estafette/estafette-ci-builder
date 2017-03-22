package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"syscall"
	"time"
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

	fmt.Printf("[estafette] Running command 'docker pull %v'\n", p.ContainerImage)
	dockerPullCmd := exec.Command("docker", "pull", p.ContainerImage)

	// make sure to kill the process when this function exits
	defer dockerPullCmd.Process.Kill()

	// run and wait until completion
	if err := dockerPullCmd.Run(); err != nil {
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

func runDockerRun(dir string, p estafettePipeline) (stat dockerRunStat, err error) {

	// run docker with image and commands from yaml
	start := time.Now()

	fmt.Printf("[estafette] Running command 'docker run --privileged --rm --entrypoint \"\" -v %v:%v -v /var/run/docker.sock:/var/run/docker.sock -w %v %v %v -c %v'\n", dir, p.WorkingDirectory, p.WorkingDirectory, p.ContainerImage, p.Shell, strings.Join(p.Commands, ";"))
	dockerRunCmd := exec.Command("docker", "run", "--privileged", "--rm", "--entrypoint", "", "-v", fmt.Sprintf("%v:%v", dir, p.WorkingDirectory), "-v", "/var/run/docker.sock:/var/run/docker.sock", "-w", p.WorkingDirectory, p.ContainerImage, p.Shell, "-c", strings.Join(p.Commands, ";"))

	// make sure to kill the process when this function exits
	defer dockerRunCmd.Process.Kill()

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
		log.Printf("[%v] %s", p.Name, in.Text()) // write each line to your log, or anything you need
	}
	if err := in.Err(); err != nil {
		log.Printf("[%v] Error: %s", p.Name, err)
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

func runPipeline(dir string, p estafettePipeline) (stat estafettePipelineStat, err error) {

	fmt.Printf("[estafette] Starting pipeline '%v'\n", p.Name)

	// pull docker image
	stat.DockerPullStat, err = runDockerPull(p)
	if err != nil {
		return
	}

	stat.DockerRunStat, err = runDockerRun(dir, p)
	if err != nil {
		return
	}

	fmt.Printf("[estafette] Finished pipeline '%v' successfully\n", p.Name)

	return
}
