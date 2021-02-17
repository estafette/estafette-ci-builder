package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"runtime"

	"github.com/alecthomas/kingpin"
	"github.com/estafette/estafette-ci-builder/builder"
	contracts "github.com/estafette/estafette-ci-contracts"
	crypt "github.com/estafette/estafette-ci-crypt"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/rs/zerolog/log"
)

var (
	appgroup  string
	app       string
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()

	builderConfigFlag       = kingpin.Flag("builder-config", "The Estafette server passes in this json structure to parameterize the build, set trusted images and inject credentials.").Envar("BUILDER_CONFIG").String()
	builderConfigPath       = kingpin.Flag("builder-config-path", "The path to the builder config json stored in a mounted file, to parameterize the build, set trusted images and inject credentials.").Envar("BUILDER_CONFIG_PATH").String()
	secretDecryptionKey     = kingpin.Flag("secret-decryption-key", "The AES-256 key used to decrypt secrets that have been encrypted with it.").Envar("SECRET_DECRYPTION_KEY").String()
	secretDecryptionKeyPath = kingpin.Flag("secret-decryption-key-path", "The path to the AES-256 key used to decrypt secrets that have been encrypted with it.").Default("/secrets/secretDecryptionKey").OverrideDefaultFromEnvar("SECRET_DECRYPTION_KEY_PATH").String()
	runAsJob                = kingpin.Flag("run-as-job", "To run the builder as a job and prevent build failures to fail the job.").Default("false").OverrideDefaultFromEnvar("RUN_AS_JOB").Bool()
	podName                 = kingpin.Flag("pod-name", "The name of the pod.").Envar("POD_NAME").String()

	runAsReadinessProbe     = kingpin.Flag("run-as-readiness-probe", "Indicates whether the builder should run as readiness probe.").Envar("RUN_AS_READINESS_PROBE").Bool()
	readinessProtocol       = kingpin.Flag("readiness-protocol", "The protocol to use for the readiness probe.").Envar("READINESS_PROTOCOL").String()
	readinessHost           = kingpin.Flag("readiness-host", "The host to use for the readiness probe.").Envar("READINESS_HOST").String()
	readinessPort           = kingpin.Flag("readiness-port", "The port to use for the readiness probe.").Envar("READINESS_PORT").Int()
	readinessPath           = kingpin.Flag("readiness-path", "The path to use for the readiness probe.").Envar("READINESS_PATH").String()
	readinessHostname       = kingpin.Flag("readiness-hostname", "The hostname to set as host header for the readiness probe.").Envar("READINESS_HOSTNAME").String()
	readinessTimeoutSeconds = kingpin.Flag("readiness-timeout-seconds", "The timeout to use for the readiness probe.").Envar("READINESS_TIMEOUT_SECONDS").Int()
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	applicationInfo := foundation.NewApplicationInfo(appgroup, app, version, branch, revision, buildDate)

	// init log format from envvar ESTAFETTE_LOG_FORMAT
	foundation.InitLoggingFromEnv(applicationInfo)

	// handle shutdown for cancellation
	osSignals, wg := foundation.InitGracefulShutdownHandling()
	cancellationChannel := make(chan struct{})
	go foundation.HandleGracefulShutdown(osSignals, wg, func() {
		close(cancellationChannel)
	})

	ciBuilder := builder.NewCIBuilder(applicationInfo)

	// this builder binary is mounted inside a scratch container to run as a readiness probe against service containers
	if *runAsReadinessProbe {
		ciBuilder.RunReadinessProbe(*readinessProtocol, *readinessHost, *readinessPort, *readinessPath, *readinessHostname, *readinessTimeoutSeconds)
	}

	// init secret helper
	decryptionKey := getDecryptionKey()
	secretHelper := crypt.NewSecretHelper(decryptionKey, false)

	// bootstrap
	tailLogsChannel := make(chan contracts.TailLogLine, 10000)
	obfuscator := builder.NewObfuscator(secretHelper)
	envvarHelper := builder.NewEnvvarHelper("ESTAFETTE_", secretHelper, obfuscator)
	whenEvaluator := builder.NewWhenEvaluator(envvarHelper)
	builderConfig, originalEncryptedCredentials := loadBuilderConfig(secretHelper, envvarHelper)
	dockerRunner := builder.NewDockerRunner(envvarHelper, obfuscator, builderConfig, tailLogsChannel)
	pipelineRunner := builder.NewPipelineRunner(envvarHelper, whenEvaluator, dockerRunner, *runAsJob, cancellationChannel, tailLogsChannel, applicationInfo)
	endOfLifeHelper := builder.NewEndOfLifeHelper(*runAsJob, builderConfig, *podName)
	ciBuilder.RunEstafetteBuildJob(pipelineRunner, dockerRunner, envvarHelper, obfuscator, endOfLifeHelper, builderConfig, originalEncryptedCredentials, *runAsJob)
}

func loadBuilderConfig(secretHelper crypt.SecretHelper, envvarHelper builder.EnvvarHelper) (builderConfig contracts.BuilderConfig, credentialsBytes []byte) {
	// read builder config either from file or envvar
	var builderConfigJSON []byte
	if *builderConfigPath != "" {

		log.Info().Msgf("Reading builder config from file %v...", *builderConfigPath)

		var err error
		builderConfigJSON, err = ioutil.ReadFile(*builderConfigPath)
		if err != nil {
			log.Fatal().Err(err).Interface("builderConfigJSON", builderConfigJSON).Msgf("Failed to read builder config file at %v", *builderConfigPath)
		}

	} else if *builderConfigFlag != "" {

		log.Info().Msg("Reading builder config from envvar BUILDER_CONFIG...")

		builderConfigJSON = []byte(*builderConfigFlag)
		os.Unsetenv("BUILDER_CONFIG")

	} else {

		log.Fatal().Msg("Neither BUILDER_CONFIG_PATH nor BUILDER_CONFIG envvar is set; one of them is required")

	}

	// unmarshal builder config
	err := json.Unmarshal(builderConfigJSON, &builderConfig)
	if err != nil {
		log.Fatal().Err(err).Interface("builderConfigJSON", builderConfigJSON).Msg("Failed to unmarshal builder config")
	}

	// unmarshal a second time to be able to return the original unaltered credentials for the obfuscator to extract secrets from it
	credentialsBytes, err = json.Marshal(builderConfig.Credentials)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to marshal credentials")
	}

	// ensure GetPipelineName does not fail below
	err = envvarHelper.SetPipelineName(builderConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to set pipeline name")
	}

	// decrypt all credentials
	decryptedCredentials := []*contracts.CredentialConfig{}
	for _, c := range builderConfig.Credentials {

		// loop all additional properties and decrypt
		decryptedAdditionalProperties := map[string]interface{}{}
		for key, value := range c.AdditionalProperties {
			if s, isString := value.(string); isString {
				decryptedAdditionalProperties[key], err = secretHelper.DecryptAllEnvelopes(s, envvarHelper.GetPipelineName())
				if err != nil {
					log.Fatal().Err(err).Msgf("Failed decrypting credential %v property %v", c.Name, key)
				}
			} else {
				decryptedAdditionalProperties[key] = value
			}
		}
		c.AdditionalProperties = decryptedAdditionalProperties

		decryptedCredentials = append(decryptedCredentials, c)
	}
	builderConfig.Credentials = decryptedCredentials

	return
}

func getDecryptionKey() string {
	// support both base64 encoded decryption key and non-encoded or mounted as secret
	decryptionKey := *secretDecryptionKey
	if *secretDecryptionKeyPath != "" && foundation.FileExists(*secretDecryptionKeyPath) {
		secretDecryptionKeyBytes, err := ioutil.ReadFile(*secretDecryptionKeyPath)
		if err != nil {
			log.Fatal().Err(err).Msgf("Failed reading secret decryption key from path %v", *secretDecryptionKeyPath)
		}

		decryptionKey = string(secretDecryptionKeyBytes)
	}

	return decryptionKey
}
