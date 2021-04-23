package builder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"time"

	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// NewKubernetesRunner returns a new ContainerRunner to run containers using Kubernetes resources
func NewKubernetesRunner(envvarHelper EnvvarHelper, obfuscator Obfuscator, kubeClientset *kubernetes.Clientset, config contracts.BuilderConfig, tailLogsChannel chan contracts.TailLogLine) ContainerRunner {

	namespace, _ := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")

	return &kubernetesRunnerImpl{
		envvarHelper:                     envvarHelper,
		obfuscator:                       obfuscator,
		kubeClientset:                    kubeClientset,
		config:                           config,
		tailLogsChannel:                  tailLogsChannel,
		runningStagePodIDs:               make([]string, 0),
		runningSingleStageServicePodIDss: make([]string, 0),
		runningMultiStageServicePodIDss:  make([]string, 0),
		runningReadinessProbePodIDss:     make([]string, 0),
		entrypointTemplateDir:            "/entrypoint-templates",

		namespace: string(namespace),
	}
}

type kubernetesRunnerImpl struct {
	envvarHelper    EnvvarHelper
	obfuscator      Obfuscator
	kubeClientset   *kubernetes.Clientset
	config          contracts.BuilderConfig
	tailLogsChannel chan contracts.TailLogLine

	runningStagePodIDs               []string
	runningSingleStageServicePodIDss []string
	runningMultiStageServicePodIDss  []string
	runningReadinessProbePodIDss     []string
	entrypointTemplateDir            string

	namespace string
}

func (dr *kubernetesRunnerImpl) IsImagePulled(ctx context.Context, stageName string, containerImage string) bool {
	return true
}

func (dr *kubernetesRunnerImpl) PullImage(ctx context.Context, stageName string, containerImage string) (err error) {
	return
}

func (dr *kubernetesRunnerImpl) GetImageSize(containerImage string) (totalSize int64, err error) {
	return totalSize, nil
}

func (dr *kubernetesRunnerImpl) getStagePodName(stageName string, stageIndex int) (podName string) {

	podName = strings.TrimPrefix(dr.envvarHelper.GetPodName(), *dr.config.Action+"-")
	podName = fmt.Sprintf("stg-%v-%v", stageIndex, podName)

	return
}

func (dr *kubernetesRunnerImpl) StartStageContainer(ctx context.Context, depth int, runIndex int, dir string, envvars map[string]string, stage manifest.EstafetteStage, stageIndex int) (podID string, err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "StartStageContainer")
	defer span.Finish()
	span.SetTag("docker-image", stage.ContainerImage)

	podName := dr.getStagePodName(stage.Name, stageIndex)

	// check if image is trusted image
	trustedImage := dr.config.GetTrustedImage(stage.ContainerImage)

	cmd, args, binds, err := dr.initContainerStartVariables(stage.Shell, stage.Commands, stage.RunCommandsInForeground, stage.CustomProperties, trustedImage)
	if err != nil {
		return
	}

	if len(stage.Commands) > 0 && trustedImage != nil && !trustedImage.AllowCommands && len(trustedImage.InjectedCredentialTypes) > 0 {
		// return stage as failed with error message indicating that this trusted image doesn't allow commands
		err = fmt.Errorf("This trusted image does not allow for commands to be set as a protection against snooping injected credentials")
		return
	}

	// add custom properties as ESTAFETTE_EXTENSION_... envvar
	extensionEnvVars := dr.generateExtensionEnvvars(stage.CustomProperties, stage.EnvVars)

	// add stage name to envvars
	if stage.EnvVars == nil {
		stage.EnvVars = map[string]string{}
	}
	stage.EnvVars["ESTAFETTE_STAGE_NAME"] = stage.Name

	// combine and override estafette and global envvars with stage envvars
	combinedEnvVars := dr.envvarHelper.OverrideEnvvars(envvars, stage.EnvVars, extensionEnvVars)

	// decrypt secrets in all envvars
	combinedEnvVars = dr.envvarHelper.decryptSecrets(combinedEnvVars, dr.envvarHelper.GetPipelineName())

	// define docker envvars and expand ESTAFETTE_ variables
	kubernetesEnvVars := make([]v1.EnvVar, 0)
	if len(combinedEnvVars) > 0 {
		for k, v := range combinedEnvVars {
			kubernetesEnvVars = append(kubernetesEnvVars, v1.EnvVar{Name: k, Value: os.Expand(v, dr.envvarHelper.getEstafetteEnv)})
		}
	}

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

	// define binds
	binds = append(binds, fmt.Sprintf("%v:%v", dir, os.Expand(stage.WorkingDirectory, dr.envvarHelper.getEstafetteEnv)))

	volumes := []v1.Volume{}
	volumeMounts := []v1.VolumeMount{}

	for i, b := range binds {
		bindparts := strings.Split(b, ":")
		if len(bindparts) == 2 {
			volumeName := fmt.Sprintf("vol-%v", i)
			hostPath := bindparts[0]
			mountPath := bindparts[1]

			volumes = append(volumes, v1.Volume{
				Name: volumeName,
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: hostPath,
					},
				},
			})

			volumeMounts = append(volumeMounts, v1.VolumeMount{
				Name:      volumeName,
				MountPath: mountPath,
			})
		}
	}

	// check if this is a trusted image with RunPrivileged or RunDocker set to true
	privileged := false
	if trustedImage != nil && runtime.GOOS != "windows" {
		privileged = trustedImage.RunDocker || trustedImage.RunPrivileged
	}

	labels := map[string]string{
		"pod": podName,
	}

	terminationGracePeriodSeconds := int64(300)

	affinity := &v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchFields: []v1.NodeSelectorRequirement{
							{
								Key:      "metadata.name",
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{dr.envvarHelper.GetPodNodeName()},
							},
						},
					},
				},
			},
		},
	}

	tolerations := []v1.Toleration{{
		Effect:   v1.TaintEffectNoSchedule,
		Operator: v1.TolerationOpExists,
	}}

	// TODO in the future use https://kubernetes.io/docs/concepts/workloads/pods/ephemeral-containers/ instead
	// TODO combine with https://kubernetes.io/docs/concepts/services-networking/add-entries-to-pod-etc-hosts-with-host-aliases/ for service containers

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: dr.namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"cluster-autoscaler.kubernetes.io/safe-to-evict": "false",
			},
		},
		Spec: v1.PodSpec{
			ServiceAccountName:            "estafette-ci-builder",
			TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			Containers: []v1.Container{
				{
					Name:            "estafette-ci-stage-builder",
					Image:           stage.ContainerImage,
					ImagePullPolicy: v1.PullAlways,
					Command:         cmd,
					Args:            args,
					WorkingDir:      os.Expand(stage.WorkingDirectory, dr.envvarHelper.getEstafetteEnv),
					Env:             kubernetesEnvVars,
					SecurityContext: &v1.SecurityContext{
						Privileged: &privileged,
					},
					// no resources, so it can be scheduled on the same node
					// Resources:    ,
					VolumeMounts: volumeMounts,
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes:       volumes,
			// needs affinity to land on the same node as the current pod and tolerations to be allowed on the node
			Affinity:    affinity,
			Tolerations: tolerations,
		},
	}

	pod, err = dr.kubeClientset.CoreV1().Pods(dr.namespace).Create(pod)
	if err != nil {
		return
	}

	// // connect to any configured networks
	// for networkName, networkID := range dr.networks {
	// 	err = dr.dockerClient.NetworkConnect(ctx, networkID, resp.ID, nil)
	// 	if err != nil {
	// 		log.Error().Err(err).Msgf("Failed connecting container %v to network %v with id %v", resp.ID, networkName, networkID)
	// 		return
	// 	}
	// }

	podID = podName
	dr.runningStagePodIDs = dr.addRunningPodID(dr.runningStagePodIDs, podName)

	return
}

func (dr *kubernetesRunnerImpl) StartServiceContainer(ctx context.Context, envvars map[string]string, service manifest.EstafetteService) (podID string, err error) {
	return podID, fmt.Errorf("Service containers are currently not supported for builder.type: kubernetes")
}

func (dr *kubernetesRunnerImpl) RunReadinessProbeContainer(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) (err error) {
	return fmt.Errorf("Service containers are currently not supported for builder.type: kubernetes")
}

func (dr *kubernetesRunnerImpl) TailContainerLogs(ctx context.Context, podID, parentStageName, stageName string, stageType contracts.LogType, depth, runIndex int, multiStage *bool) (err error) {

	namespace := dr.envvarHelper.GetPodNamespace()

	log.Debug().Msgf("TailContainerLogs - getting pod=%v namespace=%v", podID, namespace)

	pod, err := dr.kubeClientset.CoreV1().Pods(namespace).Get(podID, metav1.GetOptions{})
	if err != nil {
		return
	}

	err = dr.waitWhilePodLeavesState(ctx, pod, v1.PodPending)
	if err != nil {
		return
	}

	if pod.Status.Phase != v1.PodRunning {
		return fmt.Errorf("TailContainerLogs - pod %v has unsupported phase %v", pod.Name, pod.Status.Phase)
	}

	err = dr.followPodLogs(ctx, pod, parentStageName, stageName, stageType, depth, runIndex)
	if err != nil {
		return
	}

	// refresh pod status
	pod, err = dr.kubeClientset.CoreV1().Pods(namespace).Get(podID, metav1.GetOptions{})
	if err != nil {
		return
	}

	err = dr.waitWhilePodLeavesState(ctx, pod, v1.PodRunning)
	if err != nil {
		return
	}

	if pod.Status.Phase != v1.PodSucceeded && pod.Status.Phase != v1.PodFailed {
		return fmt.Errorf("TailContainerLogs - pod %v has unsupported phase %v", pod.Name, pod.Status.Phase)
	}

	// check exit code
	var exitCode int32
	if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].LastTerminationState.Terminated != nil {
		exitCode = pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.ExitCode
	} else {
		return fmt.Errorf("Container %v exited with error", podID)
	}

	log.Debug().Msgf("TailContainerLogs - done following logs stream for pod %v", pod.Name)

	// clear container id
	if stageType == contracts.LogTypeStage {
		dr.runningStagePodIDs = dr.removeRunningPodIDs(dr.runningStagePodIDs, podID)
	} else if stageType == contracts.LogTypeService && multiStage != nil {
		if *multiStage {
			dr.runningMultiStageServicePodIDss = dr.removeRunningPodIDs(dr.runningMultiStageServicePodIDss, podID)
		} else {
			dr.runningSingleStageServicePodIDss = dr.removeRunningPodIDs(dr.runningSingleStageServicePodIDss, podID)
		}
	}

	if exitCode != 0 {
		return fmt.Errorf("Failed with exit code: %v", exitCode)
	}

	return
}

func (dr *kubernetesRunnerImpl) waitWhilePodLeavesState(ctx context.Context, pod *v1.Pod, phase v1.PodPhase) (err error) {

	namespace := dr.envvarHelper.GetPodNamespace()

	labelSelector := labels.Set{
		"pod": pod.Name,
	}

	if pod.Status.Phase == phase {

		log.Debug().Msgf("waitWhilePodLeavesState - pod %v, waiting to leave phase %v...", pod.Name, phase)

		// watch for pod to go into out of specified state
		timeoutSeconds := int64(300)

		watcher, err := dr.kubeClientset.CoreV1().Pods(namespace).Watch(metav1.ListOptions{
			LabelSelector:  labelSelector.String(),
			TimeoutSeconds: &timeoutSeconds,
		})
		if err != nil {
			return err
		}

		for {
			event, ok := <-watcher.ResultChan()
			if !ok {
				log.Warn().Msgf("Watcher for pod %v is closed", pod.Name)
				break
			}
			if event.Type == watch.Modified {
				modifiedPod, ok := event.Object.(*v1.Pod)
				if !ok {
					log.Warn().Msgf("Watcher for pod %v returns event object of incorrect type", pod.Name)
					break
				}

				if modifiedPod.Status.Phase != phase {
					*pod = *modifiedPod
					break
				}
			}
		}
	}

	return nil
}

func (dr *kubernetesRunnerImpl) followPodLogs(ctx context.Context, pod *v1.Pod, parentStageName, stageName string, stageType contracts.LogType, depth, runIndex int) (err error) {
	log.Debug().Msg("TailContainerLogs - pod has running state...")

	namespace := dr.envvarHelper.GetPodNamespace()
	lineNumber := 1

	req := dr.kubeClientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &v1.PodLogOptions{
		Follow: true,
	})
	logsStream, err := req.Stream()
	if err != nil {
		return errors.Wrapf(err, "Failed opening logs stream for pod %v", pod.Name)
	}
	defer logsStream.Close()

	reader := bufio.NewReader(logsStream)
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			log.Debug().Msgf("EOF in logs stream for pod %v, exiting tailing", pod.Name)
			break
		}
		if err != nil {
			return err
		}

		// first byte contains the streamType
		// -   0: stdin (will be written on stdout)
		// -   1: stdout
		// -   2: stderr
		// -   3: system error
		streamType := "stdout"
		// switch headers[0] {
		// case 1:
		// 	streamType = "stdout"
		// case 2:
		// 	streamType = "stderr"
		// default:
		// 	continue
		// }

		// strip headers and obfuscate secret values
		logLineString := dr.obfuscator.Obfuscate(string(line))

		// create object for tailing logs and storing in the db when done
		logLineObject := contracts.BuildLogLine{
			LineNumber: lineNumber,
			Timestamp:  time.Now().UTC(),
			StreamType: streamType,
			Text:       logLineString,
		}
		lineNumber++

		// log as json, to be tailed when looking at live logs from gui
		dr.tailLogsChannel <- contracts.TailLogLine{
			Step:        stageName,
			ParentStage: parentStageName,
			Type:        stageType,
			Depth:       depth,
			RunIndex:    runIndex,
			LogLine:     &logLineObject,
		}
	}

	log.Debug().Msgf("Done following logs stream for pod %v", pod.Name)

	return nil
}

func (dr *kubernetesRunnerImpl) StopSingleStageServiceContainers(ctx context.Context, parentStage manifest.EstafetteStage) {
	log.Info().Msgf("[%v] Stopping single-stage service containers...", parentStage.Name)

	// the service containers should be the only ones running, so just stop all containers
	dr.stopContainers(dr.runningSingleStageServicePodIDss)

	log.Info().Msgf("[%v] Stopped single-stage service containers...", parentStage.Name)
}

func (dr *kubernetesRunnerImpl) StopMultiStageServiceContainers(ctx context.Context) {
	log.Info().Msg("Stopping multi-stage service containers...")

	// the service containers should be the only ones running, so just stop all containers
	dr.stopContainers(dr.runningMultiStageServicePodIDss)

	log.Info().Msg("Stopped multi-stage service containers...")
}

func (dr *kubernetesRunnerImpl) StartDockerDaemon() error {
	// return fmt.Errorf("Docker daemon should not be started for builder.type: kubernetes")
	return nil
}

func (dr *kubernetesRunnerImpl) WaitForDockerDaemon() {
}

func (dr *kubernetesRunnerImpl) CreateDockerClient() error {
	// return fmt.Errorf("Docker client should not be created for builder.type: kubernetes")
	return nil
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

func (dr *kubernetesRunnerImpl) stopContainer(podID string) error {

	log.Debug().Msgf("Stopping pod with id %v", podID)

	gracePeriodSeconds := int64(20)

	err := dr.kubeClientset.CoreV1().Pods(dr.namespace).Delete(podID, &metav1.DeleteOptions{GracePeriodSeconds: &gracePeriodSeconds})
	if err != nil {
		log.Warn().Err(err).Msgf("Failed stopping pod with id %v", podID)
		return err
	}

	log.Info().Msgf("Stopped pod with id %v", podID)

	return nil
}

func (dr *kubernetesRunnerImpl) stopContainers(podIDs []string) {

	if len(podIDs) > 0 {
		log.Info().Msgf("Stopping %v pods", len(podIDs))

		var wg sync.WaitGroup
		wg.Add(len(podIDs))

		for _, id := range podIDs {
			go func(id string) {
				defer wg.Done()
				dr.stopContainer(id)
			}(id)
		}

		wg.Wait()

		log.Info().Msgf("Stopped %v pods", len(podIDs))
	} else {
		log.Info().Msg("No pods to stop")
	}
}

func (dr *kubernetesRunnerImpl) StopAllContainers() {

	allRunningPodIDss := append(dr.runningStagePodIDs, dr.runningSingleStageServicePodIDss...)
	allRunningPodIDss = append(allRunningPodIDss, dr.runningMultiStageServicePodIDss...)
	allRunningPodIDss = append(allRunningPodIDss, dr.runningReadinessProbePodIDss...)

	dr.stopContainers(allRunningPodIDss)
}

func (dr *kubernetesRunnerImpl) addRunningPodID(podIDs []string, podID string) []string {

	log.Debug().Msgf("Adding pod id %v to podIDs", podID)

	return append(podIDs, podID)
}

func (dr *kubernetesRunnerImpl) removeRunningPodIDs(podIDs []string, podID string) []string {

	log.Debug().Msgf("Removing pod id %v from podIDs", podID)

	purgedPodIDss := []string{}
	for _, id := range podIDs {
		if id != podID {
			purgedPodIDss = append(purgedPodIDss, id)
		} else {
			// remove the pod
			err := dr.kubeClientset.CoreV1().Pods(dr.envvarHelper.GetPodNamespace()).Delete(podID, &metav1.DeleteOptions{})
			if err != nil {
				log.Warn().Err(err).Msgf("Failed deleting pod %v", podID)
			}
		}
	}

	return purgedPodIDss
}

func (dr *kubernetesRunnerImpl) CreateNetworks(ctx context.Context) error {
	// return fmt.Errorf("Networks are not supported for builder.type: kubernetes")
	return nil
}

func (dr *kubernetesRunnerImpl) DeleteNetworks(ctx context.Context) error {
	// return fmt.Errorf("Networks are not supported for builder.type: kubernetes")
	return nil
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

	hostPath = filepath.Join(dr.envvarHelper.GetTempDir(), strings.TrimPrefix(entrypointdir, "/tmp"))
	mountPath = "/entrypoint"
	if runtime.GOOS == "windows" {
		hostPath = filepath.Join(dr.envvarHelper.GetTempDir(), strings.TrimPrefix(entrypointdir, "C:\\Windows\\TEMP"))
		mountPath = "C:" + mountPath
	}

	return
}

func (dr *kubernetesRunnerImpl) initContainerStartVariables(shell string, commands []string, runCommandsInForeground bool, customProperties map[string]interface{}, trustedImage *contracts.TrustedImageConfig) (cmd []string, args []string, binds []string, err error) {
	cmd = make([]string, 0)
	args = make([]string, 0)
	binds = make([]string, 0)

	if len(commands) > 0 {
		// generate entrypoint script
		entrypointHostPath, entrypointMountPath, entrypointFile, innerErr := dr.generateEntrypointScript(shell, commands, runCommandsInForeground)
		if innerErr != nil {
			return cmd, args, binds, innerErr
		}

		// use generated entrypoint script for executing commands
		entrypointFilePath := path.Join(entrypointMountPath, entrypointFile)
		if runtime.GOOS == "windows" && shell == "powershell" {
			entrypointFilePath = fmt.Sprintf("C:\\entrypoint\\%v", entrypointFile)
			cmd = []string{"powershell.exe", entrypointFilePath}
		} else if runtime.GOOS == "windows" && shell == "cmd" {
			entrypointFilePath = fmt.Sprintf("C:\\entrypoint\\%v", entrypointFile)
			cmd = []string{"cmd.exe", "/C", entrypointFilePath}
		} else {
			cmd = []string{entrypointFilePath}
		}

		log.Debug().Interface("cmd", cmd).Msg("Inspecting cmd array")

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

		hostPath = filepath.Join(dr.envvarHelper.GetTempDir(), strings.TrimPrefix(credentialsdir, "/tmp"))
		mountPath = "/credentials"
		if runtime.GOOS == "windows" {
			hostPath = filepath.Join(dr.envvarHelper.GetTempDir(), strings.TrimPrefix(credentialsdir, "C:\\Windows\\TEMP"))
			mountPath = "C:" + mountPath
		}
	}

	return
}
