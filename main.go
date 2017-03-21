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
	data, err := ioutil.ReadFile("./.estafette.yaml")
	if err != nil {
		log.Fatal(err)
	}
	var estafetteManifest estafetteManifest
	if err := estafetteManifest.UnmarshalYAML(data); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", estafetteManifest)

	// get current working directory
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}

	for n, p := range estafetteManifest.Pipelines {

		fmt.Printf("Pipeline name: %v\n", n)

		// run docker with image and steps from yaml
		cmd := exec.Command("docker", "run", "--rm", "--entrypoint", "", "-v", fmt.Sprintf("%v:/estafette", dir), "-w", "/estafette", p.ContainerImage, "/bin/bash", "-c", strings.Join(p.Commands, ";"))
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
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					log.Printf("Exit Status: %d", status.ExitStatus())
					os.Exit(status.ExitStatus())
				}
			} else {
				log.Fatal(err)
			}
		}
	}
}
