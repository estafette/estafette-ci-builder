package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/olekukonko/tablewriter"
)

var (
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()
)

func main() {

	fmt.Printf("[estafette] Starting estafette ci builder (version=%v, branch=%v, revision=%v, buildDate=%v, goVersion=%v)\n", version, branch, revision, buildDate, goVersion)

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

	envvars := collectEstafetteEnvvars(manifest)

	result, firstErr := runPipelines(manifest, dir, envvars)

	renderStats(result)

	handleExit(firstErr)
}

func handleExit(firstErr error) {

	if firstErr != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func renderStats(result estafetteRunPipelinesResult) {

	data := make([][]string, 0)

	dockerPullDurationTotal := 0.0
	dockerRunDurationTotal := 0.0
	for _, s := range result.PipelineResults {

		data = append(data, []string{
			s.Pipeline.Name,
			s.Pipeline.ContainerImage,
			fmt.Sprintf("%.0f", s.DockerPullDuration.Seconds()),
			fmt.Sprintf("%.0f", s.DockerRunDuration.Seconds()),
			fmt.Sprintf("%.0f", s.DockerPullDuration.Seconds()+s.DockerRunDuration.Seconds()),
			s.Status,
			s.Detail,
		})

		dockerPullDurationTotal += s.DockerPullDuration.Seconds()
		dockerRunDurationTotal += s.DockerRunDuration.Seconds()
	}

	fmt.Println("")
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Pipeline", "Image", "Pull (s)", "Run (s)", "Total (s)", "Status", "Detail"})
	table.SetFooter([]string{"", "Total", fmt.Sprintf("%.0f", dockerPullDurationTotal), fmt.Sprintf("%.0f", dockerRunDurationTotal), fmt.Sprintf("%.0f", dockerPullDurationTotal+dockerRunDurationTotal), "", ""})
	table.SetBorder(false)
	table.AppendBulk(data)
	table.Render()
	fmt.Println("")
}
