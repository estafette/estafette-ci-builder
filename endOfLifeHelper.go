package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/sethgrid/pester"
)

// EndOfLifeHelper has methods to shutdown the runner after a fatal or successful run
type EndOfLifeHelper interface {
	handleFatal(error, string)
	sendBuildFinishedEvent(string)
	sendBuildJobLogEvent()
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

func (elh *endOfLifeHelperImpl) handleFatal(err error, message string) {

	ciServer := elh.envvarHelper.getEstafetteEnv("ESTAFETTE_CI_SERVER")
	if ciServer == "gocd" {
		log.Fatal().Err(err).Msg(message)
		os.Exit(1)
	}

	elh.sendBuildFinishedEvent("builder:failed")
	log.Error().Err(err).Msg(message)
	os.Exit(0)
}

func (elh *endOfLifeHelperImpl) sendBuildJobLogEvent() {

	ciServerBuilderEventsURL := elh.envvarHelper.getEstafetteEnv("ESTAFETTE_CI_SERVER_BUILDER_EVENTS_URL")
	ciAPIKey := elh.envvarHelper.getEstafetteEnv("ESTAFETTE_CI_API_KEY")
	jobName := elh.envvarHelper.getEstafetteEnv("ESTAFETTE_BUILD_JOB_NAME")

	if ciServerBuilderEventsURL != "" && ciAPIKey != "" && jobName != "" {

		logText, err := ioutil.ReadFile("log.txt")
		if err != nil {
			log.Error().Err(err).Msgf("Failed reading log.txt job %v", jobName)
			return
		}

		// convert BuildJobLogs to json
		var requestBody io.Reader

		buildJobLogs := BuildJobLogs{
			RepoFullName: elh.envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_NAME"),
			RepoBranch:   elh.envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_BRANCH"),
			RepoRevision: elh.envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_REVISION"),
			RepoSource:   elh.envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_SOURCE"),
			LogText:      string(logText),
		}
		data, err := json.Marshal(buildJobLogs)
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
		request, err := http.NewRequest("POST", ciServerBuilderEventsURL, requestBody)
		if err != nil {
			log.Error().Err(err).Msgf("Failed creating http client for job %v", jobName)
			return
		}

		// add headers
		request.Header.Add("X-Estafette-Event", "builder:logs")
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", ciAPIKey))

		// perform actual request
		response, err := client.Do(request)
		if err != nil {
			log.Error().Err(err).Msgf("Failed performing http request to %v for job %v", ciServerBuilderEventsURL, jobName)
			return
		}

		defer response.Body.Close()

		log.Debug().Str("url", ciServerBuilderEventsURL).Msg("Sent ci-builder logs")
	}
}

func (elh *endOfLifeHelperImpl) sendBuildFinishedEvent(eventType string) {

	ciServerBuilderEventsURL := elh.envvarHelper.getEstafetteEnv("ESTAFETTE_CI_SERVER_BUILDER_EVENTS_URL")
	ciAPIKey := elh.envvarHelper.getEstafetteEnv("ESTAFETTE_CI_API_KEY")
	jobName := elh.envvarHelper.getEstafetteEnv("ESTAFETTE_BUILD_JOB_NAME")

	if ciServerBuilderEventsURL != "" && ciAPIKey != "" && jobName != "" {
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
		request.Header.Add("X-Estafette-Event", eventType)
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
