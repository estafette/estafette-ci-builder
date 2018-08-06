package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/estafette/estafette-ci-contracts"

	"github.com/rs/zerolog/log"
	"github.com/sethgrid/pester"
)

// EndOfLifeHelper has methods to shutdown the runner after a fatal or successful run
type EndOfLifeHelper interface {
	handleGocdFatal(error, string)
	handleFatal(contracts.BuildLog, error, string)
	sendBuildFinishedEvent(string)
	sendBuildJobLogEvent(buildLog contracts.BuildLog)
}

type endOfLifeHelperImpl struct {
	envvarHelper EnvvarHelper
}

// NewEndOfLifeHelper returns a new EndOfLifeHelper
func NewEndOfLifeHelper(envvarHelper EnvvarHelper) EndOfLifeHelper {
	return &endOfLifeHelperImpl{
		envvarHelper: envvarHelper,
	}
}

func (elh *endOfLifeHelperImpl) handleGocdFatal(err error, message string) {

	log.Fatal().Err(err).Msg(message)
	os.Exit(1)
}

func (elh *endOfLifeHelperImpl) handleFatal(buildLog contracts.BuildLog, err error, message string) {

	// add error messages as step to show in logs
	fatalStep := contracts.BuildLogStep{
		Step: "init",
		LogLines: []contracts.BuildLogLine{
			contracts.BuildLogLine{
				Timestamp:  time.Now().UTC(),
				StreamType: "stderr",
				Text:       err.Error(),
			},
			contracts.BuildLogLine{
				Timestamp:  time.Now().UTC(),
				StreamType: "stderr",
				Text:       message,
			},
		},
		ExitCode: -1,
		Status:   "failed",
	}
	buildLog.Steps = append(buildLog.Steps, fatalStep)

	elh.sendBuildJobLogEvent(buildLog)
	elh.sendBuildFinishedEvent("failed")
	log.Error().Err(err).Msg(message)
	os.Exit(0)
}

func (elh *endOfLifeHelperImpl) sendBuildJobLogEvent(buildLog contracts.BuildLog) {

	ciServerBuilderPostLogsURL := elh.envvarHelper.getEstafetteEnv("ESTAFETTE_CI_SERVER_POST_LOGS_URL")
	ciAPIKey := elh.envvarHelper.getEstafetteEnv("ESTAFETTE_CI_API_KEY")
	jobName := elh.envvarHelper.getEstafetteEnv("ESTAFETTE_BUILD_JOB_NAME")

	if ciServerBuilderPostLogsURL != "" && ciAPIKey != "" && jobName != "" {

		// convert BuildJobLogs to json
		var requestBody io.Reader

		data, err := json.Marshal(buildLog)
		if err != nil {
			log.Error().Err(err).Msgf("Failed marshalling BuildJobLogs for job %v", jobName)
			return
		}
		requestBody = bytes.NewReader(data)

		// create client, in order to add headers
		client := pester.New()
		client.MaxRetries = 3
		client.Backoff = pester.ExponentialJitterBackoff
		client.KeepLog = true
		request, err := http.NewRequest("POST", ciServerBuilderPostLogsURL, requestBody)
		if err != nil {
			log.Error().Err(err).Msgf("Failed creating http client for job %v", jobName)
			return
		}

		// add headers
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", ciAPIKey))
		request.Header.Add("Content-Type", "application/json")

		// perform actual request
		response, err := client.Do(request)
		if err != nil {
			log.Error().Err(err).Msgf("Failed performing http request to %v for job %v", ciServerBuilderPostLogsURL, jobName)
			return
		}

		defer response.Body.Close()

		log.Debug().Str("url", ciServerBuilderPostLogsURL).Msg("Sent ci-builder logs v2")
	}
}

func (elh *endOfLifeHelperImpl) sendBuildFinishedEvent(buildStatus string) {

	ciServerBuilderEventsURL := elh.envvarHelper.getEstafetteEnv("ESTAFETTE_CI_SERVER_BUILDER_EVENTS_URL")
	ciAPIKey := elh.envvarHelper.getEstafetteEnv("ESTAFETTE_CI_API_KEY")
	jobName := elh.envvarHelper.getEstafetteEnv("ESTAFETTE_BUILD_JOB_NAME")

	if ciServerBuilderEventsURL != "" && ciAPIKey != "" && jobName != "" {
		// convert EstafetteCiBuilderEvent to json
		var requestBody io.Reader

		ciBuilderEvent := EstafetteCiBuilderEvent{
			JobName:      jobName,
			RepoSource:   elh.envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_SOURCE"),
			RepoOwner:    strings.Split(elh.envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_NAME"), "/")[0],
			RepoName:     strings.Split(elh.envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_NAME"), "/")[1],
			RepoRevision: elh.envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_REVISION"),
			BuildStatus:  buildStatus,
		}
		data, err := json.Marshal(ciBuilderEvent)
		if err != nil {
			log.Error().Err(err).Msgf("Failed marshalling EstafetteCiBuilderEvent for job %v", jobName)
			return
		}
		requestBody = bytes.NewReader(data)

		// create client, in order to add headers
		client := pester.New()
		client.MaxRetries = 3
		client.Backoff = pester.ExponentialJitterBackoff
		client.KeepLog = true
		request, err := http.NewRequest("POST", ciServerBuilderEventsURL, requestBody)
		if err != nil {
			log.Error().Err(err).Msgf("Failed creating http client for job %v", jobName)
			return
		}

		// add headers
		request.Header.Add("X-Estafette-Event", fmt.Sprintf("builder:%v", buildStatus))
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", ciAPIKey))

		// perform actual request
		response, err := client.Do(request)
		if err != nil {
			log.Error().Err(err).Msgf("Failed performing http request to %v for job %v", ciServerBuilderEventsURL, jobName)
			return
		}

		defer response.Body.Close()

		log.Debug().Str("url", ciServerBuilderEventsURL).Msg("Notified ci-api that ci-builder has finished")
	}
}
