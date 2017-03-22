package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/olekukonko/tablewriter"
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

	envvars := collectEstafetteEnvvars(manifest)

	statsSlice := make([]estafettePipelineStat, 0)

	for _, p := range manifest.Pipelines {
		stat, err := runPipeline(dir, envvars, *p)
		if err != nil {
			os.Exit(1)
		}

		if stat.ExitCode() > 0 {
			os.Exit(stat.ExitCode())
		}

		statsSlice = append(statsSlice, stat)
	}

	renderStats(statsSlice)

	os.Exit(0)
}

func renderStats(statsSlice []estafettePipelineStat) {

	data := make([][]string, 0)

	dockerPullDurationTotal := 0.0
	dockerRunDurationTotal := 0.0
	for _, s := range statsSlice {
		data = append(data, []string{
			s.Pipeline.Name,
			s.Pipeline.ContainerImage,
			fmt.Sprintf("%.0f", s.DockerPullStat.Duration.Seconds()),
			fmt.Sprintf("%.0f", s.DockerRunStat.Duration.Seconds()),
			fmt.Sprintf("%.0f", s.DockerPullStat.Duration.Seconds()+s.DockerRunStat.Duration.Seconds()),
		})

		dockerPullDurationTotal += s.DockerPullStat.Duration.Seconds()
		dockerRunDurationTotal += s.DockerRunStat.Duration.Seconds()
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Pipeline", "Image", "Pull (s)", "Run (s)", "Total (s)"})
	table.SetFooter([]string{"", "Total", fmt.Sprintf("%.0f", dockerPullDurationTotal), fmt.Sprintf("%.0f", dockerRunDurationTotal), fmt.Sprintf("%.0f", dockerPullDurationTotal+dockerRunDurationTotal)})
	table.SetBorder(false)
	table.AppendBulk(data)
	table.Render()
}
