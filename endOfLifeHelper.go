package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/estafette/estafette-ci-contracts"

	"github.com/rs/zerolog/log"
	"github.com/sethgrid/pester"
)

// EndOfLifeHelper has methods to shutdown the runner after a fatal or successful run
type EndOfLifeHelper interface {
	handleFatal(contracts.BuildLog, error, string)
	sendBuildFinishedEvent(string)
	sendBuildJobLogEvent(buildLog contracts.BuildLog)
}

type endOfLifeHelperImpl struct {
	runAsJob bool
	config   contracts.BuilderConfig
}

// NewEndOfLifeHelper returns a new EndOfLifeHelper
func NewEndOfLifeHelper(runAsJob bool, config contracts.BuilderConfig) EndOfLifeHelper {
	return &endOfLifeHelperImpl{
		runAsJob: runAsJob,
		config:   config,
	}
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

	if elh.runAsJob {
		log.Error().Err(err).Msg(message)
		os.Exit(0)
	} else {
		log.Fatal().Err(err).Msg(message)
	}
}

func (elh *endOfLifeHelperImpl) sendBuildJobLogEvent(buildLog contracts.BuildLog) {

	ciServerBuilderPostLogsURL := elh.config.CIServer.PostLogsURL
	ciAPIKey := elh.config.CIServer.APIKey
	jobName := *elh.config.JobName

	if ciServerBuilderPostLogsURL != "" && ciAPIKey != "" && jobName != "" {

		// convert BuildJobLogs to json
		var requestBody io.Reader

		var data []byte
		var err error
		if *elh.config.Action == "release" {
			// copy buildLog to releaseLog and marshal that
			releaseLog := contracts.ReleaseLog{
				ID:         buildLog.ID,
				RepoSource: buildLog.RepoSource,
				RepoOwner:  buildLog.RepoOwner,
				RepoName:   buildLog.RepoName,
				ReleaseID:  strconv.Itoa(elh.config.ReleaseParams.ReleaseID),
				Steps:      buildLog.Steps,
				InsertedAt: buildLog.InsertedAt,
			}
			data, err = json.Marshal(releaseLog)
			if err != nil {
				log.Error().Err(err).Msgf("Failed marshalling ReleaseLog for job %v", jobName)
				return
			}
		} else {
			data, err = json.Marshal(buildLog)
			if err != nil {
				log.Error().Err(err).Msgf("Failed marshalling BuildLog for job %v", jobName)
				return
			}
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

	ciServerBuilderEventsURL := elh.config.CIServer.BuilderEventsURL
	ciAPIKey := elh.config.CIServer.APIKey
	jobName := *elh.config.JobName

	if ciServerBuilderEventsURL != "" && ciAPIKey != "" && jobName != "" {
		// convert EstafetteCiBuilderEvent to json
		var requestBody io.Reader

		buildID := ""
		if elh.config.BuildParams != nil {
			buildID = strconv.Itoa(elh.config.BuildParams.BuildID)
		}

		releaseID := ""
		if elh.config.ReleaseParams != nil {
			releaseID = strconv.Itoa(elh.config.ReleaseParams.ReleaseID)
		}

		ciBuilderEvent := EstafetteCiBuilderEvent{
			JobName:      jobName,
			RepoSource:   elh.config.Git.RepoSource,
			RepoOwner:    elh.config.Git.RepoOwner,
			RepoName:     elh.config.Git.RepoName,
			RepoBranch:   elh.config.Git.RepoBranch,
			RepoRevision: elh.config.Git.RepoRevision,
			ReleaseID:    releaseID,
			BuildID:      buildID,
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
		request.Header.Add("X-Estafette-Event-Job-Name", jobName)
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
