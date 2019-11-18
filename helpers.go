package main

import (
	"fmt"
	"os"
	"strings"

	contracts "github.com/estafette/estafette-ci-contracts"

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

	if result.HasAggregatedErrors() {
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
	if result.HasAggregatedErrors() {
		statusTotal = "FAILED"
	}

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
	table.SetHeader([]string{"Stage", "Image", "Size (MB)", "Pull (s)", "Run (s)", "Total (s)", "Status", "Detail"})
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

func transformRunStagesResultToBuildLogSteps(result estafetteRunStagesResult) (buildLogSteps []contracts.BuildLogStep) {

	buildLogSteps = make([]contracts.BuildLogStep, 0)

	for _, r := range result.StageResults {
		buildLogSteps = append(buildLogSteps, transformStageRunResultToBuildLogSteps(r))
	}

	return
}

func transformStageRunResultToBuildLogSteps(result estafetteStageRunResult) (buildLogStep contracts.BuildLogStep) {

	buildLogStep = contracts.BuildLogStep{
		Step:         result.Stage.Name,
		Image:        getBuildLogStepDockerImage(result),
		Duration:     result.DockerRunDuration,
		LogLines:     result.LogLines,
		ExitCode:     result.ExitCode,
		Status:       result.Status,
		AutoInjected: result.Stage.AutoInjected,
		RunIndex:     result.RunIndex,
		Depth:        result.Depth,
		NestedSteps:  []*contracts.BuildLogStep{},
		Services:     []*contracts.BuildLogStep{},
	}

	for _, r := range result.ParallelStagesResults {
		bls := transformStageRunResultToBuildLogSteps(r)
		buildLogStep.NestedSteps = append(buildLogStep.NestedSteps, &bls)
	}

	for _, s := range result.ServicesResults {
		bls := transformServiceRunResultToBuildLogSteps(s)
		buildLogStep.Services = append(buildLogStep.Services, &bls)
	}

	return
}

func transformServiceRunResultToBuildLogSteps(result estafetteServiceRunResult) (buildLogStep contracts.BuildLogStep) {

	buildLogStep = contracts.BuildLogStep{
		Step:     result.Service.Name,
		Image:    getBuildLogStepDockerImageForService(result),
		Duration: result.DockerRunDuration,
		LogLines: result.LogLines,
		ExitCode: result.ExitCode,
		Status:   result.Status,
	}

	return
}

func getBuildLogStepDockerImage(result estafetteStageRunResult) *contracts.BuildLogStepDockerImage {

	containerImageName := ""
	containerImageTag := ""
	pullError := ""
	if result.Stage.ContainerImage != "" {
		containerImageArray := strings.Split(result.Stage.ContainerImage, ":")
		containerImageName = containerImageArray[0]
		containerImageTag = "latest"
		if len(containerImageArray) > 1 {
			containerImageTag = containerImageArray[1]
		}

		if result.DockerPullError != nil {
			pullError = result.DockerPullError.Error()
		}
	}

	return &contracts.BuildLogStepDockerImage{
		Name:         containerImageName,
		Tag:          containerImageTag,
		IsPulled:     result.IsDockerImagePulled,
		ImageSize:    result.DockerImageSize,
		PullDuration: result.DockerPullDuration,
		Error:        pullError,
		IsTrusted:    result.IsTrustedImage,
	}
}

func getBuildLogStepDockerImageForService(result estafetteServiceRunResult) *contracts.BuildLogStepDockerImage {

	containerImageName := ""
	containerImageTag := ""
	pullError := ""
	if result.Service.ContainerImage != "" {
		containerImageArray := strings.Split(result.Service.ContainerImage, ":")
		containerImageName = containerImageArray[0]
		containerImageTag = "latest"
		if len(containerImageArray) > 1 {
			containerImageTag = containerImageArray[1]
		}

		if result.DockerPullError != nil {
			pullError = result.DockerPullError.Error()
		}
	}

	return &contracts.BuildLogStepDockerImage{
		Name:         containerImageName,
		Tag:          containerImageTag,
		IsPulled:     result.IsDockerImagePulled,
		ImageSize:    result.DockerImageSize,
		PullDuration: result.DockerPullDuration,
		Error:        pullError,
		IsTrusted:    result.IsTrustedImage,
	}
}
