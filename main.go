package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func main() {

	// read yaml
	manifest, err := readManifest(".estafette.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// get current working directory
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	dir = strings.Replace(filepath.ToSlash(dir), "C:", "/c", 1)

	fmt.Printf("[estafette] Running %v pipelines\n", len(manifest.Pipelines))

	for _, p := range manifest.Pipelines {

		fmt.Printf("[estafette] Starting pipeline '%v'\n", p.Name)

		// set default for shell path or override if set in yaml file
		shellPath := "/bin/sh"
		if p.Shell != "" {
			shellPath = p.Shell
		}

		// set default for working directory or override if set in yaml file
		workingDirectory := "/estafette-work"
		if p.WorkingDirectory != "" {
			workingDirectory = p.WorkingDirectory
		}

		// pull docker image
		fmt.Printf("[estafette] Running command 'docker pull %v'\n", p.ContainerImage)
		dockerPullCmd := exec.Command("docker", "pull", p.ContainerImage)
		if err := dockerPullCmd.Start(); err != nil {
			log.Fatal(err)
		}

		if err := dockerPullCmd.Wait(); err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				// The program has exited with an exit code != 0

				// This works on both Unix and Windows. Although package
				// syscall is generally platform dependent, WaitStatus is
				// defined for both Unix and Windows and in both cases has
				// an ExitStatus() method with the same signature.
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok && status.ExitStatus() > 0 {
					os.Exit(status.ExitStatus())
				}
			} else {
				log.Fatal(err)
			}
		}

		// run docker with image and commands from yaml
		fmt.Printf("[estafette] Running command 'docker run --privileged --rm --entrypoint \"\" -v %v:%v -v /var/run/docker.sock:/var/run/docker.sock -w %v %v %v -c %v'\n", dir, workingDirectory, workingDirectory, p.ContainerImage, shellPath, strings.Join(p.Commands, ";"))
		dockerRunCmd := exec.Command("docker", "run", "--privileged", "--rm", "--entrypoint", "", "-v", fmt.Sprintf("%v:%v", dir, workingDirectory), "-v", "/var/run/docker.sock:/var/run/docker.sock", "-w", workingDirectory, p.ContainerImage, shellPath, "-c", strings.Join(p.Commands, ";"))
		stdout, err := dockerRunCmd.StdoutPipe()
		if err != nil {
			log.Fatal(err)
		}
		stderr, err := dockerRunCmd.StderrPipe()
		if err != nil {
			log.Fatal(err)
		}
		if err := dockerRunCmd.Start(); err != nil {
			log.Fatal(err)
		}

		// read command's stdout and stderr line by line
		multi := io.MultiReader(stdout, stderr)

		in := bufio.NewScanner(multi)

		for in.Scan() {
			log.Printf(in.Text()) // write each line to your log, or anything you need
		}
		if err := in.Err(); err != nil {
			log.Printf("[estafette] Error: %s", err)
		}

		if err := dockerRunCmd.Wait(); err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				// The program has exited with an exit code != 0

				// This works on both Unix and Windows. Although package
				// syscall is generally platform dependent, WaitStatus is
				// defined for both Unix and Windows and in both cases has
				// an ExitStatus() method with the same signature.
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok && status.ExitStatus() > 0 {
					os.Exit(status.ExitStatus())
				}
			} else {
				log.Fatal(err)
			}
		}

		fmt.Printf("[estafette] Finished pipeline '%v' successfully\n", p.Name)
	}

	os.Exit(0)
}
