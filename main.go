package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	fmt.Printf("[estafette] Running %v pipelines\n", len(manifest.Pipelines))

	for _, p := range manifest.Pipelines {
		stat, err := runPipeline(dir, *p)
		if err != nil {
			os.Exit(1)
		}

		if stat.ExitCode() > 0 {
			os.Exit(stat.ExitCode())
		}
	}

	os.Exit(0)
}
