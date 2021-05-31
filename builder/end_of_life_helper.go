package builder

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
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	tracingLog "github.com/opentracing/opentracing-go/log"
	"github.com/rs/zerolog/log"
	"github.com/sethgrid/pester"
)

// EndOfLifeHelper has methods to shutdown the runner after a fatal or successful run
type EndOfLifeHelper interface {
	HandleFatal(context.Context, contracts.BuildLog, error, string)
	SendBuildStartedEvent(ctx context.Context) error
	SendBuildFinishedEvent(ctx context.Context, buildStatus contracts.LogStatus) error
	SendBuildCleanEvent(ctx context.Context, buildStatus contracts.LogStatus) error
	SendBuildJobLogEvent(ctx context.Context, buildLog contracts.BuildLog) error
}

type endOfLifeHelperImpl struct {
	runAsJob bool
	config   contracts.BuilderConfig
	podName  string
}

// NewEndOfLifeHelper returns a new EndOfLifeHelper
func NewEndOfLifeHelper(runAsJob bool, config contracts.BuilderConfig, podName string) EndOfLifeHelper {
	return &endOfLifeHelperImpl{
		runAsJob: runAsJob,
		config:   config,
		podName:  podName,
	}
}

func (elh *endOfLifeHelperImpl) HandleFatal(ctx context.Context, buildLog contracts.BuildLog, err error, message string) {

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

	buildLog.Steps = append(buildLog.Steps, &fatalStep)

	_ = elh.SendBuildFinishedEvent(ctx, contracts.LogStatusFailed)
	_ = elh.SendBuildJobLogEvent(ctx, buildLog)
	_ = elh.SendBuildCleanEvent(ctx, contracts.LogStatusFailed)

	if elh.runAsJob {
		log.Error().Err(err).Msg(message)
		os.Exit(0)
	} else {
		log.Fatal().Err(err).Msg(message)
	}
}

func (elh *endOfLifeHelperImpl) SendBuildJobLogEvent(ctx context.Context, buildLog contracts.BuildLog) (err error) {

	err = elh.SendBuildJobLogEventCore(ctx, buildLog)

	if err == nil {
		return
	}

	// strip log lines from successful steps to reduce size of the logs and still keep the useful information
	slimBuildLog := buildLog
	slimBuildLog.Steps = []*contracts.BuildLogStep{}
	for _, s := range buildLog.Steps {
		slimBuildLogStep := s
		if s.Status == contracts.LogStatusSucceeded {
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

	return elh.SendBuildJobLogEventCore(ctx, slimBuildLog)
}

func (elh *endOfLifeHelperImpl) SendBuildJobLogEventCore(ctx context.Context, buildLog contracts.BuildLog) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "SendLog")
	defer span.Finish()

	ciServerBuilderPostLogsURL := elh.config.CIServer.PostLogsURL
	jwt := elh.config.CIServer.JWT
	jobName := *elh.config.JobName

	if ciServerBuilderPostLogsURL != "" && jwt != "" && jobName != "" {

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
		} else if *elh.config.Action == "bot" {
			// copy buildLog to botLog and marshal that
			botLog := contracts.BotLog{
				ID:         buildLog.ID,
				RepoSource: buildLog.RepoSource,
				RepoOwner:  buildLog.RepoOwner,
				RepoName:   buildLog.RepoName,
				BotID:      strconv.Itoa(elh.config.BotParams.BotID),
				Steps:      buildLog.Steps,
				InsertedAt: buildLog.InsertedAt,
			}
			data, err = json.Marshal(botLog)
			if err != nil {
				log.Error().Err(err).Msgf("Failed marshalling BotLog for job %v", jobName)
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
		client := pester.NewExtendedClient(&http.Client{Transport: &nethttp.Transport{}})
		client.MaxRetries = 1
		client.Backoff = pester.DefaultBackoff
		client.KeepLog = true
		client.Timeout = time.Second * 60
		request, err := http.NewRequest("POST", ciServerBuilderPostLogsURL, requestBody)
		if err != nil {
			log.Error().Err(err).Msgf("Failed creating http client for job %v", jobName)
			return err
		}

		// add tracing context
		request = request.WithContext(opentracing.ContextWithSpan(request.Context(), span))

		// collect additional information on setting up connections
		request, ht := nethttp.TraceRequest(span.Tracer(), request)

		// add headers
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", jwt))
		request.Header.Add("Content-Type", "application/json")

		// perform actual request
		response, err := client.Do(request)
		if err != nil {
			log.Error().Err(err).Str("logs", client.LogString()).Msgf("Failed shipping logs to %v for job %v: %v", ciServerBuilderPostLogsURL, jobName, client.LogString())
			return err
		}

		defer response.Body.Close()
		ht.Finish()

		log.Debug().Str("logs", client.LogString()).Msgf("Successfully shipped logs to %v for job %v", ciServerBuilderPostLogsURL, jobName)
	}

	return nil
}

func (elh *endOfLifeHelperImpl) SendBuildStartedEvent(ctx context.Context) error {
	buildStatus := contracts.LogStatusRunning
	return elh.sendBuilderEvent(ctx, buildStatus, fmt.Sprintf("builder:%v", buildStatus.ToStatus()))
}

func (elh *endOfLifeHelperImpl) SendBuildFinishedEvent(ctx context.Context, buildStatus contracts.LogStatus) error {
	return elh.sendBuilderEvent(ctx, buildStatus, fmt.Sprintf("builder:%v", buildStatus.ToStatus()))
}

func (elh *endOfLifeHelperImpl) SendBuildCleanEvent(ctx context.Context, buildStatus contracts.LogStatus) error {
	return elh.sendBuilderEvent(ctx, buildStatus, "builder:clean")
}

func (elh *endOfLifeHelperImpl) sendBuilderEvent(ctx context.Context, buildStatus contracts.LogStatus, event string) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "SendBuildStatus")
	defer span.Finish()
	span.SetTag("build-status", buildStatus.ToStatus())
	span.SetTag("event-type", event)

	ciServerBuilderEventsURL := elh.config.CIServer.BuilderEventsURL
	jwt := elh.config.CIServer.JWT
	jobName := *elh.config.JobName

	if ciServerBuilderEventsURL != "" && jwt != "" && jobName != "" {
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
			PodName:      elh.podName,
			RepoSource:   elh.config.Git.RepoSource,
			RepoOwner:    elh.config.Git.RepoOwner,
			RepoName:     elh.config.Git.RepoName,
			RepoBranch:   elh.config.Git.RepoBranch,
			RepoRevision: elh.config.Git.RepoRevision,
			ReleaseID:    releaseID,
			BuildID:      buildID,
			BuildStatus:  buildStatus.ToStatus(),
		}
		data, err := json.Marshal(ciBuilderEvent)
		if err != nil {
			log.Error().Err(err).Msgf("Failed marshalling EstafetteCiBuilderEvent for job %v", jobName)
			return err
		}
		requestBody = bytes.NewReader(data)

		// create client, in order to add headers
		client := pester.NewExtendedClient(&http.Client{Transport: &nethttp.Transport{}})
		client.MaxRetries = 3
		client.Backoff = pester.ExponentialJitterBackoff
		client.KeepLog = true
		client.Timeout = time.Second * 10
		request, err := http.NewRequest("POST", ciServerBuilderEventsURL, requestBody)
		if err != nil {
			log.Error().Err(err).Msgf("Failed creating http client for job %v", jobName)
			return err
		}

		// add tracing context
		request = request.WithContext(opentracing.ContextWithSpan(request.Context(), span))

		// collect additional information on setting up connections
		request, ht := nethttp.TraceRequest(span.Tracer(), request)

		// add headers
		request.Header.Add("X-Estafette-Event", event)
		request.Header.Add("X-Estafette-Event-Job-Name", jobName)
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", jwt))

		// perform actual request
		response, err := client.Do(request)
		if err != nil {
			span.SetTag("error", true)
			span.LogFields(
				tracingLog.String("error", err.Error()),
			)
			log.Error().Err(err).Str("pesterLogs", client.LogString()).Msgf("Failed performing http request to %v for job %v: %v", ciServerBuilderEventsURL, jobName, client.LogString())
			return err
		}

		defer response.Body.Close()
		ht.Finish()

		log.Debug().Str("pesterLogs", client.LogString()).Str("url", ciServerBuilderEventsURL).Msgf("Succesfully sent %v event to api", event)
	}

	return nil
}
