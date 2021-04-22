package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/template"

	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
)

// NewKubernetesRunner returns a new ContainerRunner to run containers using Kubernetes resources
func NewKubernetesRunner(envvarHelper EnvvarHelper, obfuscator Obfuscator, kubeClientset *kubernetes.Clientset, config contracts.BuilderConfig, tailLogsChannel chan contracts.TailLogLine) ContainerRunner {
	return &kubernetesRunnerImpl{
		envvarHelper:                          envvarHelper,
		obfuscator:                            obfuscator,
		kubeClientset:                         kubeClientset,
		config:                                config,
		tailLogsChannel:                       tailLogsChannel,
		runningStageContainerIDs:              make([]string, 0),
		runningSingleStageServiceContainerIDs: make([]string, 0),
		runningMultiStageServiceContainerIDs:  make([]string, 0),
		runningReadinessProbeContainerIDs:     make([]string, 0),
		entrypointTemplateDir:                 "/entrypoint-templates",
		pulledImagesMutex:                     NewMapMutex(),
	}
}

type kubernetesRunnerImpl struct {
	envvarHelper    EnvvarHelper
	obfuscator      Obfuscator
	kubeClientset   *kubernetes.Clientset
	config          contracts.BuilderConfig
	tailLogsChannel chan contracts.TailLogLine

	runningStageContainerIDs              []string
	runningSingleStageServiceContainerIDs []string
	runningMultiStageServiceContainerIDs  []string
	runningReadinessProbeContainerIDs     []string
	entrypointTemplateDir                 string

	pulledImagesMutex *MapMutex
}

func (dr *kubernetesRunnerImpl) IsImagePulled(ctx context.Context, stageName string, containerImage string) bool {

	span, ctx := opentracing.StartSpanFromContext(ctx, "IsImagePulled")
	defer span.Finish()
	span.SetTag("docker-image", containerImage)

	return false
}

func (dr *kubernetesRunnerImpl) PullImage(ctx context.Context, stageName string, containerImage string) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "PullImage")
	defer span.Finish()
	span.SetTag("docker-image", containerImage)

	return
}

func (dr *kubernetesRunnerImpl) GetImageSize(containerImage string) (totalSize int64, err error) {

	return totalSize, nil
}

func (dr *kubernetesRunnerImpl) StartStageContainer(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, stage manifest.EstafetteStage) (containerID string, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "StartStageContainer")
	defer span.Finish()
	span.SetTag("docker-image", stage.ContainerImage)

	// // check if image is trusted image
	// trustedImage := dr.config.GetTrustedImage(stage.ContainerImage)

	// entrypoint, cmds, binds, err := dr.initContainerStartVariables(stage.Shell, stage.Commands, stage.RunCommandsInForeground, stage.CustomProperties, trustedImage)
	// if err != nil {
	// 	return
	// }

	// // add custom properties as ESTAFETTE_EXTENSION_... envvar
	// extensionEnvVars := dr.generateExtensionEnvvars(stage.CustomProperties, stage.EnvVars)

	// // add stage name to envvars
	// if stage.EnvVars == nil {
	// 	stage.EnvVars = map[string]string{}
	// }
	// stage.EnvVars["ESTAFETTE_STAGE_NAME"] = stage.Name

	// // combine and override estafette and global envvars with stage envvars
	// combinedEnvVars := dr.envvarHelper.OverrideEnvvars(envvars, stage.EnvVars, extensionEnvVars)

	// // decrypt secrets in all envvars
	// combinedEnvVars = dr.envvarHelper.decryptSecrets(combinedEnvVars, dr.envvarHelper.GetPipelineName())

	// // define docker envvars and expand ESTAFETTE_ variables
	// dockerEnvVars := make([]string, 0)
	// if combinedEnvVars != nil && len(combinedEnvVars) > 0 {
	// 	for k, v := range combinedEnvVars {
	// 		dockerEnvVars = append(dockerEnvVars, fmt.Sprintf("%v=%v", k, os.Expand(v, dr.envvarHelper.getEstafetteEnv)))
	// 	}
	// }

	// // define binds
	// binds = append(binds, fmt.Sprintf("%v:%v", dir, os.Expand(stage.WorkingDirectory, dr.envvarHelper.getEstafetteEnv)))

	// // check if this is a trusted image with RunDocker set to true
	// if trustedImage != nil && trustedImage.RunDocker {
	// 	if runtime.GOOS == "windows" {
	// 		if ok, _ := pathExists(`\\.\pipe\docker_engine`); ok {
	// 			binds = append(binds, `\\.\pipe\docker_engine:\\.\pipe\docker_engine`)
	// 		}
	// 		if ok, _ := pathExists("C:/Program Files/Docker"); ok {
	// 			binds = append(binds, "C:/Program Files/Docker:C:/dod")
	// 		}
	// 	} else {
	// 		if ok, _ := pathExists("/var/run/docker.sock"); ok {
	// 			binds = append(binds, "/var/run/docker.sock:/var/run/docker.sock")
	// 		}
	// 		if ok, _ := pathExists("/usr/local/bin/docker"); ok {
	// 			binds = append(binds, "/usr/local/bin/docker:/dod/docker")
	// 		}
	// 	}
	// }

	// // define config
	// config := container.Config{
	// 	AttachStdout: true,
	// 	AttachStderr: true,
	// 	Env:          dockerEnvVars,
	// 	Image:        stage.ContainerImage,
	// 	WorkingDir:   os.Expand(stage.WorkingDirectory, dr.envvarHelper.getEstafetteEnv),
	// }
	// if len(stage.Commands) > 0 {
	// 	if trustedImage != nil && !trustedImage.AllowCommands && len(trustedImage.InjectedCredentialTypes) > 0 {
	// 		// return stage as failed with error message indicating that this trusted image doesn't allow commands
	// 		err = fmt.Errorf("This trusted image does not allow for commands to be set as a protection against snooping injected credentials")
	// 		return
	// 	}

	// 	// only override entrypoint when commands are set, so extensions can work without commands
	// 	config.Entrypoint = entrypoint
	// 	// only pass commands when they are set, so extensions can work without
	// 	config.Cmd = cmds
	// }
	// if trustedImage != nil && trustedImage.RunDocker {
	// 	if runtime.GOOS != "windows" {
	// 		currentUser, err := user.Current()
	// 		if err == nil && currentUser != nil {
	// 			config.User = fmt.Sprintf("%v:%v", currentUser.Uid, currentUser.Gid)
	// 			log.Debug().Msgf("Setting docker user to %v", config.User)
	// 		} else {
	// 			log.Debug().Err(err).Msg("Can't retrieve current user")
	// 		}
	// 	} else {
	// 		log.Debug().Msg("Not setting docker user for windows")
	// 	}
	// }

	// // check if this is a trusted image with RunPrivileged or RunDocker set to true
	// privileged := false
	// if trustedImage != nil && runtime.GOOS != "windows" {
	// 	privileged = trustedImage.RunDocker || trustedImage.RunPrivileged
	// }

	// // create container
	// resp, err := dr.dockerClient.ContainerCreate(ctx, &config, &container.HostConfig{
	// 	Binds:      binds,
	// 	Privileged: privileged,
	// 	AutoRemove: true,
	// 	LogConfig: container.LogConfig{
	// 		Type: "local",
	// 		Config: map[string]string{
	// 			"max-size": "20m",
	// 			"max-file": "5",
	// 			"compress": "true",
	// 			"mode":     "non-blocking",
	// 		},
	// 	},
	// }, &network.NetworkingConfig{}, nil, "")
	// if err != nil {
	// 	return "", err
	// }

	// // connect to any configured networks
	// for networkName, networkID := range dr.networks {
	// 	err = dr.dockerClient.NetworkConnect(ctx, networkID, resp.ID, nil)
	// 	if err != nil {
	// 		log.Error().Err(err).Msgf("Failed connecting container %v to network %v with id %v", resp.ID, networkName, networkID)
	// 		return
	// 	}
	// }

	// containerID = resp.ID
	// dr.runningStageContainerIDs = dr.addRunningContainerID(dr.runningStageContainerIDs, containerID)

	// // start container
	// if err = dr.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
	// 	return
	// }

	return
}

func (dr *kubernetesRunnerImpl) StartServiceContainer(ctx context.Context, envvars map[string]string, service manifest.EstafetteService) (containerID string, err error) {
	return containerID, fmt.Errorf("Service containers are currently not supported for builder.type: kubernetes")
}

func (dr *kubernetesRunnerImpl) RunReadinessProbeContainer(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
	return fmt.Errorf("Service containers are currently not supported for builder.type: kubernetes")
}

func (dr *kubernetesRunnerImpl) TailContainerLogs(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth, runIndex int, multiStage *bool) (err error) {

	// lineNumber := 1

	// // follow logs
	// rc, err := dr.dockerClient.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
	// 	ShowStdout: true,
	// 	ShowStderr: true,
	// 	Timestamps: false,
	// 	Follow:     true,
	// 	Details:    false,
	// })
	// if err != nil {
	// 	return err
	// }
	// defer rc.Close()

	// // stream logs to stdout with buffering
	// in := bufio.NewReader(rc)
	// var readError error
	// for {
	// 	// strip first 8 bytes, they contain docker control characters (https://github.com/docker/docker-ce/blob/v18.06.1-ce/components/engine/client/container_logs.go#L23-L32)
	// 	headers := make([]byte, 8)
	// 	n, readError := in.Read(headers)
	// 	if readError != nil {
	// 		break
	// 	}

	// 	if n < 8 {
	// 		// doesn't seem to be a valid header
	// 		continue
	// 	}

	// 	// inspect the docker log header for stream type

	// 	// first byte contains the streamType
	// 	// -   0: stdin (will be written on stdout)
	// 	// -   1: stdout
	// 	// -   2: stderr
	// 	// -   3: system error
	// 	streamType := ""
	// 	switch headers[0] {
	// 	case 1:
	// 		streamType = "stdout"
	// 	case 2:
	// 		streamType = "stderr"
	// 	default:
	// 		continue
	// 	}

	// 	// read the rest of the line until we hit end of line
	// 	logLine, readError := in.ReadBytes('\n')
	// 	if readError != nil {
	// 		break
	// 	}

	// 	// strip headers and obfuscate secret values
	// 	logLineString := dr.obfuscator.Obfuscate(string(logLine))

	// 	// create object for tailing logs and storing in the db when done
	// 	logLineObject := contracts.BuildLogLine{
	// 		LineNumber: lineNumber,
	// 		Timestamp:  time.Now().UTC(),
	// 		StreamType: streamType,
	// 		Text:       logLineString,
	// 	}
	// 	lineNumber++

	// 	// log as json, to be tailed when looking at live logs from gui
	// 	dr.tailLogsChannel <- contracts.TailLogLine{
	// 		Step:        stageName,
	// 		ParentStage: parentStageName,
	// 		Type:        stageType,
	// 		Depth:       depth,
	// 		RunIndex:    runIndex,
	// 		LogLine:     &logLineObject,
	// 	}
	// }

	// if readError != nil && readError != io.EOF {
	// 	log.Error().Msgf("[%v] Error: %v", stageName, readError)
	// 	return readError
	// }

	// // wait for container to stop running
	// resultC, errC := dr.dockerClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	// var exitCode int64
	// select {
	// case result := <-resultC:
	// 	exitCode = result.StatusCode
	// case err = <-errC:
	// 	log.Warn().Err(err).Msgf("Container %v exited with error", containerID)
	// 	return err
	// }

	// // clear container id
	// if stageType == contracts.LogTypeStage {
	// 	dr.runningStageContainerIDs = dr.removeRunningContainerID(dr.runningStageContainerIDs, containerID)
	// } else if stageType == contracts.LogTypeService && multiStage != nil {
	// 	if *multiStage {
	// 		dr.runningMultiStageServiceContainerIDs = dr.removeRunningContainerID(dr.runningMultiStageServiceContainerIDs, containerID)
	// 	} else {
	// 		dr.runningSingleStageServiceContainerIDs = dr.removeRunningContainerID(dr.runningSingleStageServiceContainerIDs, containerID)
	// 	}
	// }

	// if exitCode != 0 {
	// 	return fmt.Errorf("Failed with exit code: %v", exitCode)
	// }

	return
}

func (dr *kubernetesRunnerImpl) StopSingleStageServiceContainers(ctx context.Context, parentStage manifest.EstafetteStage) {

	// log.Info().Msgf("[%v] Stopping single-stage service containers...", parentStage.Name)

	// // the service containers should be the only ones running, so just stop all containers
	// dr.stopContainers(dr.runningSingleStageServiceContainerIDs)

	// log.Info().Msgf("[%v] Stopped single-stage service containers...", parentStage.Name)
}

func (dr *kubernetesRunnerImpl) StopMultiStageServiceContainers(ctx context.Context) {

	// log.Info().Msg("Stopping multi-stage service containers...")

	// // the service containers should be the only ones running, so just stop all containers
	// dr.stopContainers(dr.runningMultiStageServiceContainerIDs)

	// log.Info().Msg("Stopped multi-stage service containers...")
}

func (dr *kubernetesRunnerImpl) StartDockerDaemon() error {
	return fmt.Errorf("Docker daemon should not be started for builder.type: kubernetes")
}

func (dr *kubernetesRunnerImpl) WaitForDockerDaemon() {
}

func (dr *kubernetesRunnerImpl) CreateDockerClient() error {
	return fmt.Errorf("Docker daemon should not be started for builder.type: kubernetes")
}

func (dr *kubernetesRunnerImpl) IsTrustedImage(stageName string, containerImage string) bool {

	log.Info().Msgf("[%v] Checking if docker image '%v' is trusted...", stageName, containerImage)

	// check if image is trusted image
	trustedImage := dr.config.GetTrustedImage(containerImage)

	return trustedImage != nil
}

func (dr *kubernetesRunnerImpl) HasInjectedCredentials(stageName string, containerImage string) bool {

	log.Info().Msgf("[%v] Checking if docker image '%v' has injected credentials...", stageName, containerImage)

	// check if image has injected credentials
	trustedImage := dr.config.GetTrustedImage(containerImage)
	if trustedImage == nil {
		return false
	}

	credentialMap := dr.config.GetCredentialsForTrustedImage(*trustedImage)

	return len(credentialMap) > 0
}

func (dr *kubernetesRunnerImpl) stopContainer(containerID string) error {

	// log.Debug().Msgf("Stopping container with id %v", containerID)

	// timeout := 20 * time.Second
	// err := dr.dockerClient.ContainerStop(context.Background(), containerID, &timeout)
	// if err != nil {
	// 	log.Warn().Err(err).Msgf("Failed stopping container with id %v", containerID)
	// 	return err
	// }

	// log.Info().Msgf("Stopped container with id %v", containerID)
	return nil
}

func (dr *kubernetesRunnerImpl) stopContainers(containerIDs []string) {

	if len(containerIDs) > 0 {
		log.Info().Msgf("Stopping %v containers", len(containerIDs))

		var wg sync.WaitGroup
		wg.Add(len(containerIDs))

		for _, id := range containerIDs {
			go func(id string) {
				defer wg.Done()
				dr.stopContainer(id)
			}(id)
		}

		wg.Wait()

		log.Info().Msgf("Stopped %v containers", len(containerIDs))
	} else {
		log.Info().Msg("No containers to stop")
	}
}

func (dr *kubernetesRunnerImpl) StopAllContainers() {

	allRunningContainerIDs := append(dr.runningStageContainerIDs, dr.runningSingleStageServiceContainerIDs...)
	allRunningContainerIDs = append(allRunningContainerIDs, dr.runningMultiStageServiceContainerIDs...)
	allRunningContainerIDs = append(allRunningContainerIDs, dr.runningReadinessProbeContainerIDs...)

	dr.stopContainers(allRunningContainerIDs)
}

func (dr *kubernetesRunnerImpl) addRunningContainerID(containerIDs []string, containerID string) []string {

	log.Debug().Msgf("Adding container id %v to containerIDs", containerID)

	return append(containerIDs, containerID)
}

func (dr *kubernetesRunnerImpl) removeRunningContainerID(containerIDs []string, containerID string) []string {

	log.Debug().Msgf("Removing container id %v from containerIDs", containerID)

	purgedContainerIDs := []string{}

	for _, id := range containerIDs {
		if id != containerID {
			purgedContainerIDs = append(purgedContainerIDs, id)
		}
	}

	return purgedContainerIDs
}

func (dr *kubernetesRunnerImpl) CreateNetworks(ctx context.Context) error {
	return fmt.Errorf("Networks are not supported for builder.type: kubernetes")
}

func (dr *kubernetesRunnerImpl) DeleteNetworks(ctx context.Context) error {
	return fmt.Errorf("Networks are not supported for builder.type: kubernetes")
}

func (dr *kubernetesRunnerImpl) generateEntrypointScript(shell string, commands []string, runCommandsInForeground bool) (hostPath, mountPath, entrypointFile string, err error) {

	r, _ := regexp.Compile("[a-zA-Z0-9_]+=|export|shopt|;|cd |\\||&&|\\|\\|")

	firstCommands := []struct {
		Command         string
		EscapedCommand  string
		RunInBackground bool
	}{}
	for _, c := range commands[:len(commands)-1] {
		// check if the command is assigning a value, in which case it shouldn't be run in the background
		match := r.MatchString(c)
		runInBackground := !runCommandsInForeground && !match

		firstCommands = append(firstCommands, struct {
			Command         string
			EscapedCommand  string
			RunInBackground bool
		}{c, escapeCharsInCommand(c), runInBackground})
	}

	lastCommand := commands[len(commands)-1]
	match := r.MatchString(lastCommand)
	runFinalCommandWithExec := !runCommandsInForeground && !match

	data := struct {
		Shell    string
		Commands []struct {
			Command         string
			EscapedCommand  string
			RunInBackground bool
		}
		FinalCommand            string
		EscapedFinalCommand     string
		RunFinalCommandWithExec bool
	}{
		shell,
		firstCommands,
		lastCommand,
		escapeCharsInCommand(lastCommand),
		runFinalCommandWithExec,
	}

	entrypointFile = "entrypoint.sh"
	if runtime.GOOS == "windows" && shell == "powershell" {
		entrypointFile = "entrypoint.ps1"
	} else if runtime.GOOS == "windows" && shell == "cmd" {
		entrypointFile = "entrypoint.bat"
	}

	entrypointdir, err := ioutil.TempDir("", "*-entrypoint")
	if err != nil {
		return
	}

	// set permissions on directory to avoid non-root containers not to be able to read from the mounted directory
	err = os.Chmod(entrypointdir, 0777)
	if err != nil {
		return
	}

	entrypointPath := path.Join(entrypointdir, entrypointFile)

	// read and parse template
	templatePath := path.Join(dr.entrypointTemplateDir, entrypointFile)
	entrypointTemplate, err := template.ParseFiles(templatePath)
	if err != nil {
		return
	}

	targetFile, err := os.Create(entrypointPath)
	if err != nil {
		return
	}
	defer targetFile.Close()

	err = entrypointTemplate.Execute(targetFile, data)
	if err != nil {
		return
	}

	err = os.Chmod(entrypointPath, 0777)
	if err != nil {
		return
	}

	entryPointBytes, innerErr := ioutil.ReadFile(entrypointPath)
	if innerErr == nil {
		log.Debug().Str("entrypoint", string(entryPointBytes)).Msgf("Inspecting entrypoint script at %v", entrypointPath)
	}

	hostPath = entrypointdir
	mountPath = "/entrypoint"
	if runtime.GOOS == "windows" {
		hostPath = filepath.Join(dr.envvarHelper.GetTempDir(), strings.TrimPrefix(hostPath, "C:\\Windows\\TEMP"))
		mountPath = "C:" + mountPath
	}

	return
}

func (dr *kubernetesRunnerImpl) initContainerStartVariables(shell string, commands []string, runCommandsInForeground bool, customProperties map[string]interface{}, trustedImage *contracts.TrustedImageConfig) (entrypoint []string, cmds []string, binds []string, err error) {
	entrypoint = make([]string, 0)
	cmds = make([]string, 0)
	binds = make([]string, 0)

	if len(commands) > 0 {
		// generate entrypoint script
		entrypointHostPath, entrypointMountPath, entrypointFile, innerErr := dr.generateEntrypointScript(shell, commands, runCommandsInForeground)
		if innerErr != nil {
			return entrypoint, cmds, binds, innerErr
		}

		// use generated entrypoint script for executing commands
		entrypointFilePath := path.Join(entrypointMountPath, entrypointFile)
		if runtime.GOOS == "windows" && shell == "powershell" {
			entrypointFilePath = fmt.Sprintf("C:\\entrypoint\\%v", entrypointFile)
			entrypoint = []string{"powershell.exe", entrypointFilePath}
		} else if runtime.GOOS == "windows" && shell == "cmd" {
			entrypointFilePath = fmt.Sprintf("C:\\entrypoint\\%v", entrypointFile)
			entrypoint = []string{"cmd.exe", "/C", entrypointFilePath}
		} else {
			entrypoint = []string{entrypointFilePath}
		}

		log.Debug().Interface("entrypoint", entrypoint).Msg("Inspecting entrypoint array")

		binds = append(binds, fmt.Sprintf("%v:%v", entrypointHostPath, entrypointMountPath))
	}

	// mount injected credentials as files
	credentialsHostPath, credentialsMountPath, err := dr.generateCredentialsFiles(trustedImage)
	if err != nil {
		return
	}
	if credentialsHostPath != "" && credentialsMountPath != "" {
		binds = append(binds, fmt.Sprintf("%v:%v", credentialsHostPath, credentialsMountPath))
	}

	return
}

func (dr *kubernetesRunnerImpl) generateExtensionEnvvars(customProperties map[string]interface{}, envvars map[string]string) (extensionEnvVars map[string]string) {
	extensionEnvVars = map[string]string{}
	if customProperties != nil && len(customProperties) > 0 {
		for k, v := range customProperties {
			extensionkey := dr.envvarHelper.getEstafetteEnvvarName(fmt.Sprintf("ESTAFETTE_EXTENSION_%v", foundation.ToUpperSnakeCase(k)))

			if s, isString := v.(string); isString {
				// if custom property is of type string add the envvar
				extensionEnvVars[extensionkey] = s
			} else if s, isBool := v.(bool); isBool {
				// if custom property is of type bool add the envvar
				extensionEnvVars[extensionkey] = strconv.FormatBool(s)
			} else if s, isInt := v.(int); isInt {
				// if custom property is of type bool add the envvar
				extensionEnvVars[extensionkey] = strconv.FormatInt(int64(s), 10)
			} else if s, isFloat := v.(float64); isFloat {
				// if custom property is of type bool add the envvar
				extensionEnvVars[extensionkey] = strconv.FormatFloat(float64(s), 'f', -1, 64)

			} else if i, isInterfaceArray := v.([]interface{}); isInterfaceArray {
				// check whether all array items are of type string
				valid := true
				stringValues := []string{}
				for _, iv := range i {
					if s, isString := iv.(string); isString {
						stringValues = append(stringValues, s)
					} else {
						valid = false
						break
					}
				}

				if valid {
					// if all array items are string, pass as comma-separated list to extension
					extensionEnvVars[extensionkey] = strings.Join(stringValues, ",")
				} else {
					log.Warn().Interface("customProperty", v).Msgf("Cannot turn custom property %v into extension envvar", k)
				}
			} else {
				log.Warn().Interface("customProperty", v).Msgf("Cannot turn custom property %v of type %v into extension envvar", k, reflect.TypeOf(v))
			}
		}

		// add envvar to custom properties
		customProperties := customProperties
		customProperties["env"] = envvars
	}

	// also add add custom properties as json object in ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES envvar
	customPropertiesBytes, err := json.Marshal(customProperties)
	if err == nil {
		extensionEnvVars["ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES"] = string(customPropertiesBytes)
	} else {
		log.Warn().Err(err).Interface("customProperty", customProperties).Msg("Cannot marshal custom properties for ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES envvar")
	}

	// also add add custom properties as json object in ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES_YAML envvar
	customPropertiesYamlBytes, err := yaml.Marshal(customProperties)
	if err == nil {
		extensionEnvVars["ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES_YAML"] = string(customPropertiesYamlBytes)
	} else {
		log.Warn().Err(err).Interface("customProperty", customProperties).Msg("Cannot marshal custom properties for ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES_YAML envvar")
	}

	return
}

func (dr *kubernetesRunnerImpl) generateCredentialsFiles(trustedImage *contracts.TrustedImageConfig) (hostPath, mountPath string, err error) {

	if trustedImage != nil {
		// create a tempdir to store credential files in and mount into container
		credentialsdir, innerErr := ioutil.TempDir("", "*-credentials")
		if innerErr != nil {
			return hostPath, mountPath, innerErr
		}

		// set permissions on directory to avoid non-root containers not to be able to read from the mounted directory
		err = os.Chmod(credentialsdir, 0777)
		if err != nil {
			return
		}

		credentialMap := dr.config.GetCredentialsForTrustedImage(*trustedImage)
		if len(credentialMap) == 0 {
			credentialsdir = ""
			return
		}
		for credentialType, credentialsForType := range credentialMap {

			filename := fmt.Sprintf("%v.json", foundation.ToLowerSnakeCase(credentialType))
			filepath := path.Join(credentialsdir, filename)

			// convert credentialsForType to json string
			credentialsForTypeBytes, innerErr := json.Marshal(credentialsForType)
			if innerErr != nil {
				return hostPath, mountPath, innerErr
			}

			// expand estafette variables in json file
			credentialsForTypeString := string(credentialsForTypeBytes)
			credentialsForTypeString = os.Expand(credentialsForTypeString, dr.envvarHelper.getEstafetteEnv)

			// write to file
			err = ioutil.WriteFile(filepath, []byte(credentialsForTypeString), 0666)
			if err != nil {
				return
			}

			log.Debug().Msgf("Stored credentials of type %v in file %v", credentialType, filepath)
		}

		hostPath = credentialsdir
		mountPath = "/credentials"
		if runtime.GOOS == "windows" {
			hostPath = filepath.Join(dr.envvarHelper.GetTempDir(), strings.TrimPrefix(hostPath, "C:\\Windows\\TEMP"))
			mountPath = "C:" + mountPath
		}
	}

	return
}
