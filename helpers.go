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

func handleExit(buildLogSteps []*contracts.BuildLogStep) {

	if !contracts.HasSucceededStatus(buildLogSteps) {
		os.Exit(1)
	}

	os.Exit(0)
}

func renderStats(buildLogSteps []*contracts.BuildLogStep) {

	data := make([][]string, 0)

	dockerPullDurationTotal := 0.0
	dockerRunDurationTotal := 0.0
	dockerImageSizeTotal := int64(0)
	statusTotal := contracts.GetAggregatedStatus(buildLogSteps)

	for _, s := range buildLogSteps {

		// set column values
		stage := s.Step
		if s.RunIndex > 0 {
			stage += fmt.Sprintf(" (retry %v)", s.RunIndex)
		}
		image := ""
		imageSize := ""
		imagePullDuration := ""
		stageDuration := fmt.Sprintf("%.0f", s.Duration.Seconds())
		totalDuration := fmt.Sprintf("%.0f", s.Duration.Seconds())
		status := s.Status

		// increment total counters
		dockerRunDurationTotal += s.Duration.Seconds()

		if s.Image != nil {
			// set column values
			image = s.Image.Name
			imageSize = fmt.Sprintf("%v", s.Image.ImageSize/1024/1024)
			imagePullDuration = fmt.Sprintf("%.0f", s.Image.PullDuration.Seconds())
			totalDuration = fmt.Sprintf("%.0f", s.Image.PullDuration.Seconds()+s.Duration.Seconds())

			// increment total counters
			dockerPullDurationTotal += s.Image.PullDuration.Seconds()
			dockerImageSizeTotal += s.Image.ImageSize
		}

		data = append(data, []string{
			stage,
			image,
			imageSize,
			imagePullDuration,
			stageDuration,
			totalDuration,
			status,
		})

	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Stage", "Image", "Size (MB)", "Pull (s)", "Run (s)", "Total (s)", "Status"})
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
