package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func main() {

	// read yaml
	fmt.Println("[Reading .estafette.yaml file]")
	data, err := ioutil.ReadFile(".estafette.yaml")
	if err != nil {
		log.Fatal(err)
	}
	var estafetteManifest estafetteManifest
	if err := estafetteManifest.UnmarshalYAML(data); err != nil {
		log.Fatal(err)
	}

	fmt.Println("[Finished reading .estafette.yaml file successfully]")

	// get current working directory
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}

	dir = strings.Replace(filepath.ToSlash(dir), "C:", "/c", 1)

	for n, p := range estafetteManifest.Pipelines {

		fmt.Printf("[Starting pipeline '%v']\n", n)

		// set default for shell path or override if set in yaml file
		shellPath := "/bin/bash"
		if p.Shell != "" {
			shellPath = p.Shell
		}

		// set default for working directory or override if set in yaml file
		workingDirectory := "/estafette-work"
		if p.WorkingDirectory != "" {
			workingDirectory = p.WorkingDirectory
		}

		// run docker with image and steps from yaml
		fmt.Printf("[Running command 'docker run --privileged --rm --entrypoint \"\" -v %v:%v -v /var/run:/var/run -v /volume/usr/local/bin:/volume/usr/local/bin -w %v %v %v -c %v']\n", dir, workingDirectory, workingDirectory, p.ContainerImage, shellPath, strings.Join(p.Commands, ";"))
		cmd := exec.Command("docker", "run", "--privileged", "--rm", "--entrypoint", "", "-v", fmt.Sprintf("%v:%v", dir, workingDirectory), "-v", "/var/run:/var/run", "-v", "/volume/usr/local/bin:/volume/usr/local/bin", "-w", workingDirectory, p.ContainerImage, shellPath, "-c", strings.Join(p.Commands, ";"))
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatal(err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}

		// read command's stdout and stderr line by line
		multi := io.MultiReader(stdout, stderr)

		in := bufio.NewScanner(multi)

		for in.Scan() {
			log.Printf(in.Text()) // write each line to your log, or anything you need
		}
		if err := in.Err(); err != nil {
			log.Printf("error: %s", err)
		}

		if err := cmd.Wait(); err != nil {
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

		fmt.Printf("[Finished pipeline '%v' successfully]\n", n)
		os.Exit(0)
	}
}
