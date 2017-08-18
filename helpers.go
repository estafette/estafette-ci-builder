package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/rs/zerolog/log"
)

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func handleExit(result estafetteRunPipelinesResult) {

	if result.HasErrors() {
		os.Exit(1)
	}

	os.Exit(0)
}

func handleFatal(err error, message string) {

	ciServer := getEstafetteEnv("ESTAFETTE_CI_SERVER")
	if ciServer == "gocd" {
		log.Fatal().Err(err).Msg(message)
		os.Exit(1)
	}

	sendBuildFinishedEvent("failed")
	log.Error().Err(err).Msg(message)
	os.Exit(0)
}

func sendBuildFinishedEvent(eventType string) {

	ciServerBaseURL := os.Getenv("ESTAFETTE_CI_SERVER_BASE_URL")
	jobName := os.Getenv("ESTAFETTE_BUILD_JOB_NAME")

	if ciServerBaseURL != "" && jobName != "" {
		buildFinishedURL := strings.TrimRight(ciServerBaseURL, "/") + "/events/estafette/ci-builder"

		// convert EstafetteCiBuilderEvent to json
		var requestBody io.Reader

		ciBuilderEvent := EstafetteCiBuilderEvent{JobName: jobName}
		data, err := json.Marshal(ciBuilderEvent)
		if err != nil {
			log.Error().Err(err).Msgf("Failed marshalling EstafetteCiBuilderEvent for job %v", jobName)
			return
		}
		requestBody = bytes.NewReader(data)

		// create client, in order to add headers
		client := &http.Client{}
		request, err := http.NewRequest("POST", buildFinishedURL, requestBody)
		if err != nil {
			log.Error().Err(err).Msgf("Failed creating http client for job %v", jobName)
			return
		}

		// add headers
		request.Header.Add("X-Estafette-Event", eventType)

		// perform actual request
		response, err := client.Do(request)
		if err != nil {
			log.Error().Err(err).Msgf("Failed performing http request to %v for job %v", buildFinishedURL, jobName)
			return
		}

		defer response.Body.Close()

		log.Debug().Str("url", buildFinishedURL).Msg("Notified ci-api that ci-builder has finished")
	}
}

func renderStats(result estafetteRunPipelinesResult) {

	data := make([][]string, 0)

	dockerPullDurationTotal := 0.0
	dockerRunDurationTotal := 0.0
	dockerImageSizeTotal := int64(0)
	statusTotal := "SUCCEEDED"

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

		if s.Status == "FAILED" {
			statusTotal = "FAILED"
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
