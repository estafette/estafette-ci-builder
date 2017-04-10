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

func handleExit(result estafetteRunPipelinesResult) {

	if result.HasErrors() {
		os.Exit(1)
	}

	os.Exit(0)
}

func renderStats(result estafetteRunPipelinesResult) {

	data := make([][]string, 0)

	dockerPullDurationTotal := 0.0
	dockerRunDurationTotal := 0.0
	var dockerImageSizeTotal int64

	for _, s := range result.PipelineResults {

		dockerImageSize := fmt.Sprintf("%v", s.DockerImageSize/1024/1024)
		dockerPullDuration := fmt.Sprintf("%.0f", s.DockerPullDuration.Seconds())

		if s.IsDockerImagePulled {
			dockerImageSize = ""
			dockerPullDuration = ""
		}

		detail := ""
		if s.HasErrors() {
			for _, err := range s.Errors() {
				detail += err.Error()
			}
		}

		data = append(data, []string{
			s.Pipeline.Name,
			s.Pipeline.ContainerImage,
			dockerImageSize,
			dockerPullDuration,
			fmt.Sprintf("%.0f", s.DockerRunDuration.Seconds()),
			fmt.Sprintf("%.0f", s.DockerPullDuration.Seconds()+s.DockerRunDuration.Seconds()),
			s.Status,
			detail,
		})

		dockerPullDurationTotal += s.DockerPullDuration.Seconds()
		dockerRunDurationTotal += s.DockerRunDuration.Seconds()
		dockerImageSizeTotal += s.DockerImageSize
	}

	fmt.Println("")
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Pipeline", "Image", "Size (MB)", "Pull (s)", "Run (s)", "Total (s)", "Status", "Detail"})
	table.SetFooter([]string{"", "Total", fmt.Sprintf("%v", dockerImageSizeTotal/1024/1024), fmt.Sprintf("%.0f", dockerPullDurationTotal), fmt.Sprintf("%.0f", dockerRunDurationTotal), fmt.Sprintf("%.0f", dockerPullDurationTotal+dockerRunDurationTotal), "", ""})
	table.SetBorder(false)
	table.AppendBulk(data)
	table.Render()
	fmt.Println("")
}
