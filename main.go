package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

var (
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()
)

func main() {

	fmt.Printf("[estafette] Starting estafette-ci-builder (version=%v, branch=%v, revision=%v, buildDate=%v, goVersion=%v)\n", version, branch, revision, buildDate, goVersion)

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

	fmt.Printf("[estafette] Running %v pipelines\n", len(manifest.Pipelines))

	err = setEstafetteGlobalEnvvars()
	if err != nil {
		log.Fatal(err)
	}

	envvars := collectEstafetteEnvvars(manifest)

	result := runPipelines(manifest, dir, envvars)

	renderStats(result)

	handleExit(result)
}
