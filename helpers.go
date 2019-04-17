package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

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

func transformEstafetteEnvvarsToBuildLogStep(estafetteEnvvars map[string]string) (buildLogSteps []contracts.BuildLogStep) {

	buildLogSteps = make([]contracts.BuildLogStep, 0)

	initStep := contracts.BuildLogStep{
		Step:         "envvars",
		LogLines:     []contracts.BuildLogLine{},
		ExitCode:     0,
		Status:       "SUCCEEDED",
		AutoInjected: true,
	}

	timestamp := time.Now().UTC()

	// explanation
	initStep.LogLines = append(initStep.LogLines, contracts.BuildLogLine{
		LineNumber: 1,
		Timestamp:  timestamp,
		StreamType: "stdout",
		Text:       "All available estafette environment variables; the _DNS_SAFE suffixed ones can be used to set dns labels. Since leading digits are not allowed some of them are empty.",
	})
	initStep.LogLines = append(initStep.LogLines, contracts.BuildLogLine{
		LineNumber: 2,
		Timestamp:  timestamp,
		StreamType: "stdout",
		Text:       "",
	})

	// keys in a map are deliberately unsorted, so sort them here
	sortedKeys := make([]string, 0, len(estafetteEnvvars))
	for key := range estafetteEnvvars {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	// log all estafette environment variables and their values
	lineNumber := 3
	for _, key := range sortedKeys {
		initStep.LogLines = append(initStep.LogLines, contracts.BuildLogLine{
			LineNumber: lineNumber,
			Timestamp:  timestamp,
			StreamType: "stdout",
			Text:       fmt.Sprintf("%v: %v", key, estafetteEnvvars[key]),
		})

		lineNumber++
	}

	buildLogSteps = append(buildLogSteps, initStep)

	return
}

func transformPipelineRunResultToBuildLogSteps(estafetteEnvvars map[string]string, result estafetteRunStagesResult) (buildLogSteps []contracts.BuildLogStep) {

	buildLogSteps = transformEstafetteEnvvarsToBuildLogStep(estafetteEnvvars)

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
		IsTrusted:    result.IsTrustedImage,
	}
}
