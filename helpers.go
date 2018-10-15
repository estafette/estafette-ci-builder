package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/estafette/estafette-ci-contracts"

	"github.com/olekukonko/tablewriter"
)

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func handleExit(result estafetteRunStagesResult) {

	if result.HasErrors() {
		os.Exit(1)
	}

	os.Exit(0)
}

func renderStats(result estafetteRunStagesResult) {

	data := make([][]string, 0)

	dockerPullDurationTotal := 0.0
	dockerRunDurationTotal := 0.0
	dockerImageSizeTotal := int64(0)
	statusTotal := "SUCCEEDED"

	for _, s := range result.StageResults {

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

		if s.Status == "FAILED" {
			statusTotal = "FAILED"
		}

		data = append(data, []string{
			s.Stage.Name,
			s.Stage.ContainerImage,
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

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Pipeline", "Image", "Size (MB)", "Pull (s)", "Run (s)", "Total (s)", "Status", "Detail"})
	table.SetFooter([]string{"", "Total", fmt.Sprintf("%v", dockerImageSizeTotal/1024/1024), fmt.Sprintf("%.0f", dockerPullDurationTotal), fmt.Sprintf("%.0f", dockerRunDurationTotal), fmt.Sprintf("%.0f", dockerPullDurationTotal+dockerRunDurationTotal), statusTotal, ""})
	table.SetBorder(false)
	table.AppendBulk(data)
	table.Render()
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func transformPipelineRunResultToBuildLogSteps(result estafetteRunStagesResult) (buildLogSteps []contracts.BuildLogStep) {

	buildLogSteps = make([]contracts.BuildLogStep, 0)

	for _, r := range result.StageResults {

		bls := contracts.BuildLogStep{
			Step:         r.Stage.Name,
			Image:        getBuildLogStepDockerImage(r),
			Duration:     r.DockerRunDuration,
			LogLines:     r.LogLines,
			ExitCode:     r.ExitCode,
			Status:       r.Status,
			AutoInjected: r.Stage.AutoInjected,
			RunIndex:     r.RunIndex,
		}

		buildLogSteps = append(buildLogSteps, bls)
	}

	return
}

func getBuildLogStepDockerImage(result estafetteStageRunResult) *contracts.BuildLogStepDockerImage {

	containerImageArray := strings.Split(result.Stage.ContainerImage, ":")
	containerImageName := containerImageArray[0]
	containerImageTag := "latest"
	if len(containerImageArray) > 1 {
		containerImageTag = containerImageArray[1]
	}

	pullError := ""
	if result.DockerPullError != nil {
		pullError = result.DockerPullError.Error()
	}

	return &contracts.BuildLogStepDockerImage{
		Name:         containerImageName,
		Tag:          containerImageTag,
		IsPulled:     result.IsDockerImagePulled,
		ImageSize:    result.DockerImageSize,
		PullDuration: result.DockerPullDuration,
		Error:        pullError,
	}
}
