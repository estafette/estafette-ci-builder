package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	contracts "github.com/estafette/estafette-ci-contracts"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	tracingLog "github.com/opentracing/opentracing-go/log"

	"github.com/rs/zerolog/log"
	"github.com/sethgrid/pester"
)

// EndOfLifeHelper has methods to shutdown the runner after a fatal or successful run
type EndOfLifeHelper interface {
	handleFatal(context.Context, contracts.BuildLog, error, string)
	sendBuildFinishedEvent(ctx context.Context, buildStatus string) error
	sendBuildCleanEvent(ctx context.Context, buildStatus string) error
	sendBuildJobLogEvent(ctx context.Context, buildLog contracts.BuildLog) error
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

func (elh *endOfLifeHelperImpl) handleFatal(ctx context.Context, buildLog contracts.BuildLog, err error, message string) {

	// add error messages as step to show in logs
	fatalStep := contracts.BuildLogStep{
		Step:     "init",
		LogLines: []contracts.BuildLogLine{},
		ExitCode: -1,
		Status:   "FAILED",
	}
	lineNumber := 1

	if err != nil {
		fatalStep.LogLines = append(fatalStep.LogLines, contracts.BuildLogLine{
			LineNumber: lineNumber,
			Timestamp:  time.Now().UTC(),
			StreamType: "stderr",
			Text:       err.Error(),
		})
		lineNumber++
	}
	if message != "" {
		fatalStep.LogLines = append(fatalStep.LogLines, contracts.BuildLogLine{
			LineNumber: lineNumber,
			Timestamp:  time.Now().UTC(),
			StreamType: "stderr",
			Text:       message,
		})
		lineNumber++
	}

	buildLog.Steps = append(buildLog.Steps, fatalStep)

	_ = elh.sendBuildFinishedEvent(ctx, "failed")
	_ = elh.sendBuildJobLogEvent(ctx, buildLog)
	_ = elh.sendBuildCleanEvent(ctx, "failed")

	if elh.runAsJob {
		log.Error().Err(err).Msg(message)
		os.Exit(0)
	} else {
		log.Fatal().Err(err).Msg(message)
	}
}

func (elh *endOfLifeHelperImpl) sendBuildJobLogEvent(ctx context.Context, buildLog contracts.BuildLog) (err error) {

	err = elh.sendBuildJobLogEventCore(ctx, buildLog)

	if err == nil {
		return
	}

	// strip log lines from successful steps to reduce size of the logs and still keep the useful information
	slimBuildLog := buildLog
	slimBuildLog.Steps = []contracts.BuildLogStep{}
	for _, s := range buildLog.Steps {
		slimBuildLogStep := s
		if s.Status == "SUCCEEDED" {
			if len(s.LogLines) > 0 {
				slimBuildLogStep.LogLines = []contracts.BuildLogLine{
					contracts.BuildLogLine{
						LineNumber: s.LogLines[0].LineNumber,
						Timestamp:  s.LogLines[0].Timestamp,
						StreamType: "stdout",
						Text:       "Truncated logs for reducing total log size; to prevent this use less verbose logging",
					},
				}
			}
		}

		slimBuildLog.Steps = append(slimBuildLog.Steps, slimBuildLogStep)
	}

	return elh.sendBuildJobLogEventCore(ctx, slimBuildLog)
}

func (elh *endOfLifeHelperImpl) sendBuildJobLogEventCore(ctx context.Context, buildLog contracts.BuildLog) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "SendLog")
	defer span.Finish()

	ciServerBuilderPostLogsURL := elh.config.CIServer.PostLogsURL
	ciAPIKey := elh.config.CIServer.APIKey
	jobName := *elh.config.JobName

	if ciServerBuilderPostLogsURL != "" && ciAPIKey != "" && jobName != "" {

		// convert BuildJobLogs to json
		var requestBody io.Reader

		var data []byte
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
		client.MaxRetries = 5
		client.Backoff = pester.ExponentialJitterBackoff
		client.KeepLog = true
		client.Timeout = time.Second * 120
		request, err := http.NewRequest("POST", ciServerBuilderPostLogsURL, requestBody)
		if err != nil {
			log.Error().Err(err).Msgf("Failed creating http client for job %v", jobName)
			return err
		}

		// add tracing context
		ext.SpanKindRPCClient.Set(span)
		ext.HTTPMethod.Set(span, request.Method)
		ext.HTTPUrl.Set(span, request.URL.String())
		span.Tracer().Inject(
			span.Context(),
			opentracing.HTTPHeaders,
			opentracing.HTTPHeadersCarrier(request.Header),
		)

		// add headers
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", ciAPIKey))
		request.Header.Add("Content-Type", "application/json")

		// perform actual request
		response, err := client.Do(request)
		if err != nil {
			log.Error().Err(err).Str("logs", client.LogString()).Msgf("Failed shipping logs to %v for job %v", ciServerBuilderPostLogsURL, jobName)
			return err
		}

		ext.HTTPStatusCode.Set(span, uint16(response.StatusCode))

		defer response.Body.Close()

		log.Debug().Str("logs", client.LogString()).Msgf("Successfully shipped logs to %v for job %v", ciServerBuilderPostLogsURL, jobName)
	}

	return nil
}

func (elh *endOfLifeHelperImpl) sendBuildFinishedEvent(ctx context.Context, buildStatus string) error {
	return elh.sendBuilderEvent(ctx, buildStatus, fmt.Sprintf("builder:%v", buildStatus))
}

func (elh *endOfLifeHelperImpl) sendBuildCleanEvent(ctx context.Context, buildStatus string) error {
	return elh.sendBuilderEvent(ctx, buildStatus, "builder:clean")
}

func (elh *endOfLifeHelperImpl) sendBuilderEvent(ctx context.Context, buildStatus, event string) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "SendBuildStatus")
	defer span.Finish()
	span.SetTag("build-status", buildStatus)
	span.SetTag("event-type", event)

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
			return err
		}
		requestBody = bytes.NewReader(data)

		// create client, in order to add headers
		client := pester.New()
		client.MaxRetries = 5
		client.Backoff = pester.ExponentialJitterBackoff
		client.KeepLog = true
		client.Timeout = time.Second * 10
		request, err := http.NewRequest("POST", ciServerBuilderEventsURL, requestBody)
		if err != nil {
			log.Error().Err(err).Msgf("Failed creating http client for job %v", jobName)
			return err
		}

		// add tracing context
		ext.SpanKindRPCClient.Set(span)
		ext.HTTPMethod.Set(span, request.Method)
		ext.HTTPUrl.Set(span, request.URL.String())
		span.Tracer().Inject(
			span.Context(),
			opentracing.HTTPHeaders,
			opentracing.HTTPHeadersCarrier(request.Header),
		)

		// add headers
		request.Header.Add("X-Estafette-Event", event)
		request.Header.Add("X-Estafette-Event-Job-Name", jobName)
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", ciAPIKey))

		// perform actual request
		response, err := client.Do(request)
		if err != nil {
			span.SetTag("error", true)
			span.LogFields(
				tracingLog.String("error", err.Error()),
			)
			log.Error().Err(err).Str("pesterLogs", client.LogString()).Msgf("Failed performing http request to %v for job %v", ciServerBuilderEventsURL, jobName)
			return err
		}

		ext.HTTPStatusCode.Set(span, uint16(response.StatusCode))

		defer response.Body.Close()

		log.Debug().Str("pesterLogs", client.LogString()).Str("url", ciServerBuilderEventsURL).Msgf("Succesfully sent %v event to api", event)
	}

	return nil
}
