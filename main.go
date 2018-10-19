package main

import (
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"runtime"
	"strconv"

	"github.com/alecthomas/kingpin"
	"github.com/estafette/estafette-ci-contracts"
	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()

	builderConfigFlag   = kingpin.Flag("builder-config", "The Estafette server passes in this json structure to parameterize the build, set trusted images and inject credentials.").Envar("BUILDER_CONFIG").String()
	secretDecryptionKey = kingpin.Flag("secret-decryption-key", "The AES-256 key used to decrypt secrets that have been encrypted with it.").Envar("SECRET_DECRYPTION_KEY").String()
	runAsJob            = kingpin.Flag("run-as-job", "To run the builder as a job and prevent build failures to fail the job.").Default("false").OverrideDefaultFromEnvar("RUN_AS_JOB").Bool()
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	secretHelper := crypt.NewSecretHelper(*secretDecryptionKey)

	// read builder config from envvar and unset envar; will replace parameterizing the job via separate envvars
	var builderConfig contracts.BuilderConfig
	builderConfigJSON := *builderConfigFlag
	if builderConfigJSON == "" {
		log.Fatal().Msg("BUILDER_CONFIG envvar is not set")
	}
	builderConfigJSONDecrypted, err := secretHelper.DecryptAllEnvelopes(builderConfigJSON)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed decrypting secrets in BUILDER_CONFIG: %v", builderConfigJSON)
	}

	os.Unsetenv("BUILDER_CONFIG")

	// unmarshal builder config
	err = json.Unmarshal([]byte(builderConfigJSONDecrypted), &builderConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to unmarshal BUILDER_CONFIG")
	}
	log.Debug().Interface("builderConfig", builderConfig).Msg("")

	// bootstrap
	envvarHelper := NewEnvvarHelper("ESTAFETTE_", secretHelper)
	whenEvaluator := NewWhenEvaluator(envvarHelper)
	obfuscator := NewObfuscator(secretHelper)
	dockerRunner := NewDockerRunner(envvarHelper, obfuscator, *runAsJob, builderConfig)
	pipelineRunner := NewPipelineRunner(envvarHelper, whenEvaluator, dockerRunner, *runAsJob)
	endOfLifeHelper := NewEndOfLifeHelper(*runAsJob, builderConfig)

	// set ESTAFETTE_CI_REPOSITORY_CREDENTIALS_JSON for backwards compatibility until extensions/docker supports generic credential injection
	var credentials []*contracts.ContainerRepositoryCredentialConfig
	containerRegistryCredentials := builderConfig.GetCredentialsByType("container-registry")
	for _, cred := range containerRegistryCredentials {
		credentials = append(credentials, &contracts.ContainerRepositoryCredentialConfig{
			Repository: cred.AdditionalProperties["repository"].(string),
			Username:   cred.AdditionalProperties["username"].(string),
			Password:   cred.AdditionalProperties["password"].(string),
		})
	}
	credentialsBytes, err := json.Marshal(credentials)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to marshal credentials for backwards compatibility")
	}
	os.Setenv("ESTAFETTE_CI_REPOSITORY_CREDENTIALS_JSON", string(credentialsBytes))

	// detect controlling server
	ciServer := envvarHelper.getEstafetteEnv("ESTAFETTE_CI_SERVER")

	if ciServer == "gocd" {

		fatalHandler := NewGocdFatalHandler()

		// pretty print for go.cd integration
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().
			Timestamp().
			Logger()

		stdlog.SetFlags(0)
		stdlog.SetOutput(log.Logger)

		// log startup message
		log.Info().
			Str("branch", branch).
			Str("revision", revision).
			Str("buildDate", buildDate).
			Str("goVersion", goVersion).
			Msg("Starting estafette-ci-builder...")

		// create docker client
		_, err := dockerRunner.createDockerClient()
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Failed creating a docker client")
		}

		// read yaml
		manifest, err := manifest.ReadManifestFromFile(".estafette.yaml")
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Reading .estafette.yaml manifest failed")
		}

		// initialize obfuscator
		err = obfuscator.CollectSecrets(manifest)
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Collecting secrets to obfuscate failed")
		}

		// get current working directory
		dir, err := os.Getwd()
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Getting current working directory failed")
		}

		log.Info().Msgf("Running %v stages", len(manifest.Stages))

		err = envvarHelper.setEstafetteGlobalEnvvars()
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Setting global environment variables failed")
		}

		// collect estafette and 'global' envvars from manifest
		estafetteEnvvars := envvarHelper.collectEstafetteEnvvarsAndLabels(manifest)
		globalEnvvars := envvarHelper.collectGlobalEnvvars(manifest)

		// merge estafette and global envvars
		envvars := envvarHelper.overrideEnvvars(estafetteEnvvars, globalEnvvars)

		result, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)
		if err != nil {
			fatalHandler.handleGocdFatal(err, "Executing stages from manifest failed")
		}

		renderStats(result)

		handleExit(result)

	} else if ciServer == "estafette" {

		// log as severity for stackdriver logging to recognize the level
		zerolog.LevelFieldName = "severity"

		// todo unset all ESTAFETTE_ envvars so they don't get abused by non-estafette components
		envvarsToUnset := envvarHelper.collectEstafetteEnvvars()
		for key := range envvarsToUnset {
			os.Unsetenv(key)
		}

		// set envvars that can be used by any container
		os.Setenv("ESTAFETTE_GIT_BRANCH", builderConfig.Git.RepoBranch)
		os.Setenv("ESTAFETTE_GIT_REVISION", builderConfig.Git.RepoRevision)
		os.Setenv("ESTAFETTE_BUILD_VERSION", builderConfig.BuildVersion.Version)
		if builderConfig.BuildVersion.Major != nil {
			os.Setenv("ESTAFETTE_BUILD_VERSION_MAJOR", strconv.Itoa(*builderConfig.BuildVersion.Major))
		}
		if builderConfig.BuildVersion.Minor != nil {
			os.Setenv("ESTAFETTE_BUILD_VERSION_MINOR", strconv.Itoa(*builderConfig.BuildVersion.Minor))
		}
		if builderConfig.BuildVersion.Patch != nil {
			os.Setenv("ESTAFETTE_BUILD_VERSION_PATCH", *builderConfig.BuildVersion.Patch)
		}
		if builderConfig.ReleaseParams != nil {
			os.Setenv("ESTAFETTE_RELEASE_NAME", builderConfig.ReleaseParams.ReleaseName)
		}

		// set ESTAFETTE_BITBUCKET_API_TOKEN and ESTAFETTE_GIT_URL for backward compatibility with extensions/bitbucket-status and extensions/git-clone until it supports generic credential injection
		bitbucketAPICredentials := builderConfig.GetCredentialsByType("bitbucket-api-token")
		if len(bitbucketAPICredentials) > 0 {
			token := bitbucketAPICredentials[0].AdditionalProperties["token"].(string)
			os.Setenv("ESTAFETTE_BITBUCKET_API_TOKEN", token)
			log.Debug().Msgf("Set ESTAFETTE_BITBUCKET_API_TOKEN=%v", os.Getenv("ESTAFETTE_BITBUCKET_API_TOKEN"))
			os.Setenv("ESTAFETTE_GIT_URL", fmt.Sprintf("https://x-token-auth:%v@%v/%v/%v", token, builderConfig.Git.RepoSource, builderConfig.Git.RepoOwner, builderConfig.Git.RepoName))
			log.Debug().Msgf("Set ESTAFETTE_GIT_URL=%v", os.Getenv("ESTAFETTE_GIT_URL"))
		} else {
			log.Debug().Msg("Failed getting bitbucket-api-token credentials ")
		}

		// set ESTAFETTE_GITHUB_API_TOKEN and ESTAFETTE_GIT_URL for backward compatibility with extensions/github-status and extensions/git-clone until it supports generic credential injection
		githubAPICredentials := builderConfig.GetCredentialsByType("github-api-token")
		if len(githubAPICredentials) > 0 {
			token := githubAPICredentials[0].AdditionalProperties["token"].(string)
			os.Setenv("ESTAFETTE_GITHUB_API_TOKEN", token)
			log.Debug().Msgf("Set ESTAFETTE_GITHUB_API_TOKEN=%v", os.Getenv("ESTAFETTE_GITHUB_API_TOKEN"))
			os.Setenv("ESTAFETTE_GIT_URL", fmt.Sprintf("https://x-access-token:%v@%v/%v/%v", token, builderConfig.Git.RepoSource, builderConfig.Git.RepoOwner, builderConfig.Git.RepoName))
			log.Debug().Msgf("Set ESTAFETTE_GIT_URL=%v", os.Getenv("ESTAFETTE_GIT_URL"))
		} else {
			log.Debug().Msg("Failed getting github-api-token credentials ")
		}

		// set ESTAFETTE_GIT_NAME for backwards compatibility with extensions/git-clone
		os.Setenv("ESTAFETTE_GIT_NAME", fmt.Sprintf("%v/%v", builderConfig.Git.RepoOwner, builderConfig.Git.RepoName))

		buildLog := contracts.BuildLog{
			RepoSource:   builderConfig.Git.RepoSource,
			RepoOwner:    builderConfig.Git.RepoOwner,
			RepoName:     builderConfig.Git.RepoName,
			RepoBranch:   builderConfig.Git.RepoBranch,
			RepoRevision: builderConfig.Git.RepoRevision,
			Steps:        make([]contracts.BuildLogStep, 0),
		}

		// log to file and stdout
		logFile, err := os.OpenFile("/log.txt", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create log file log.txt")
		}
		defer logFile.Close()
		multiLogWriter := io.MultiWriter(os.Stdout, logFile)

		// set some default fields added to all logs
		log.Logger = zerolog.New(multiLogWriter).With().
			Timestamp().
			Str("app", "estafette-ci-builder").
			Str("version", version).
			Str("jobName", *builderConfig.JobName).
			Interface("git", builderConfig.Git).
			Logger()

		stdlog.SetFlags(0)
		stdlog.SetOutput(log.Logger)

		// log startup message
		log.Info().
			Str("branch", branch).
			Str("revision", revision).
			Str("buildDate", buildDate).
			Str("goVersion", goVersion).
			Msg("Starting estafette-ci-builder...")

		// start docker daemon
		err = dockerRunner.startDockerDaemon()
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Error starting docker daemon")
		}

		// wait for docker daemon to be ready for usage
		dockerRunner.waitForDockerDaemon()

		// get current working directory
		dir, err := os.Getwd()
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Getting current working directory failed")
		}

		// set some envvars
		err = envvarHelper.setEstafetteGlobalEnvvars()
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Setting global environment variables failed")
		}

		// initialize obfuscator
		err = obfuscator.CollectSecrets(*builderConfig.Manifest)
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Collecting secrets to obfuscate failed")
		}

		// check whether this is a regular build or a release
		stages := builderConfig.Manifest.Stages
		if *builderConfig.Action == "release" {
			// check if the release is defined
			releaseExists := false
			for _, r := range builderConfig.Manifest.Releases {
				if r.Name == builderConfig.ReleaseParams.ReleaseName {
					releaseExists = true
					stages = r.Stages
				}
			}
			if !releaseExists {
				endOfLifeHelper.handleFatal(buildLog, err, fmt.Sprintf("Release %v does not exist", builderConfig.ReleaseParams.ReleaseName))
			}
			log.Info().Msgf("Starting release %v at version %v...", builderConfig.ReleaseParams.ReleaseName, builderConfig.BuildVersion.Version)
		} else {
			log.Info().Msgf("Starting build version %v...", builderConfig.BuildVersion.Version)
		}

		// create docker client
		_, err = dockerRunner.createDockerClient()
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Failed creating a docker client")
		}

		// collect estafette envvars and run stages from manifest
		log.Info().Msgf("Running %v stages", len(stages))
		estafetteEnvvars := envvarHelper.collectEstafetteEnvvarsAndLabels(*builderConfig.Manifest)
		globalEnvvars := envvarHelper.collectGlobalEnvvars(*builderConfig.Manifest)
		envvars := envvarHelper.overrideEnvvars(estafetteEnvvars, globalEnvvars)
		result, err := pipelineRunner.runStages(stages, dir, envvars)
		if err != nil {
			endOfLifeHelper.handleFatal(buildLog, err, "Executing stages from manifest failed")
		}

		// send result to ci-api
		log.Info().Interface("result", result).Msg("Finished running stages")
		buildLog.Steps = transformPipelineRunResultToBuildLogSteps(result)
		endOfLifeHelper.sendBuildJobLogEvent(buildLog)
		buildStatus := "succeeded"
		if result.HasErrors() {
			buildStatus = "failed"
		}
		endOfLifeHelper.sendBuildFinishedEvent(buildStatus)

		if *runAsJob {
			os.Exit(0)
		} else {
			handleExit(result)
		}

	} else {
		// Set up a simple console logger
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().
			Timestamp().
			Logger()

		log.Warn().Msgf("The CI Server (\"%s\") is not recognized, exiting.", ciServer)
	}
}
