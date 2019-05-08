package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rs/zerolog/log"

	contracts "github.com/estafette/estafette-ci-contracts"
	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
)

// EnvvarHelper is the interface for getting, setting and retrieving ESTAFETTE_ environment variables
type EnvvarHelper interface {
	toUpperSnake(string) string
	getCommandOutput(string, ...string) (string, error)
	setEstafetteGlobalEnvvars() error
	setEstafetteBuilderConfigEnvvars(builderConfig contracts.BuilderConfig) error
	setEstafetteEventEnvvars(events []*manifest.EstafetteEvent) error
	initGitSource() error
	initGitOwner() error
	initGitName() error
	initGitFullName() error
	initGitRevision() error
	initGitBranch() error
	initBuildDatetime() error
	initBuildStatus() error
	initLabels(manifest.EstafetteManifest) error
	collectEstafetteEnvvars() map[string]string
	collectEstafetteEnvvarsAndLabels(manifest.EstafetteManifest) map[string]string
	collectGlobalEnvvars(manifest.EstafetteManifest) map[string]string
	collectStagesEnvvars([]*manifest.EstafetteStage) map[string]string
	unsetEstafetteEnvvars()
	getEstafetteEnv(string) string
	setEstafetteEnv(string, string) error
	unsetEstafetteEnv(string) error
	getEstafetteEnvvarName(string) string
	overrideEnvvars(...map[string]string) map[string]string
	decryptSecret(string) string
	decryptSecrets(map[string]string) map[string]string
	getCiServer() string
	makeDNSLabelSafe(string) string

	getGitOrigin() (string, error)
	getSourceFromOrigin(string) string
	getOwnerFromOrigin(string) string
	getNameFromOrigin(string) string
}

type envvarHelperImpl struct {
	prefix       string
	ciServer     string
	secretHelper crypt.SecretHelper
}

// NewEnvvarHelper returns a new EnvvarHelper
func NewEnvvarHelper(prefix string, secretHelper crypt.SecretHelper) EnvvarHelper {
	return &envvarHelperImpl{
		prefix:       prefix,
		ciServer:     os.Getenv("ESTAFETTE_CI_SERVER"),
		secretHelper: secretHelper,
	}
}

// https://gist.github.com/elwinar/14e1e897fdbe4d3432e1
func (h *envvarHelperImpl) toUpperSnake(in string) string {
	runes := []rune(in)
	length := len(runes)

	var out []rune
	for i := 0; i < length; i++ {
		if i > 0 && unicode.IsUpper(runes[i]) && ((i+1 < length && unicode.IsLower(runes[i+1])) || unicode.IsLower(runes[i-1])) {
			out = append(out, '_')
		}
		out = append(out, unicode.ToUpper(runes[i]))
	}

	snake := string(out)

	// make sure nothing but alphanumeric characters and underscores are returned
	reg, err := regexp.Compile("[^A-Z0-9]+")
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed converting %v to upper snake case", in)
	}
	cleanSnake := reg.ReplaceAllString(snake, "_")

	return cleanSnake
}

func (h *envvarHelperImpl) getCommandOutput(name string, arg ...string) (string, error) {

	out, err := exec.Command(name, arg...).Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

func (h *envvarHelperImpl) setEstafetteGlobalEnvvars() (err error) {

	// initialize git source envvar
	err = h.initGitSource()
	if err != nil {
		return err
	}

	// initialize git owner envvar
	err = h.initGitOwner()
	if err != nil {
		return err
	}

	// initialize git name envvar
	err = h.initGitName()
	if err != nil {
		return err
	}

	// initialize git full name envvar
	err = h.initGitFullName()
	if err != nil {
		return err
	}

	// initialize git revision envvar
	err = h.initGitRevision()
	if err != nil {
		return err
	}

	// initialize git branch envvar
	err = h.initGitBranch()
	if err != nil {
		return err
	}

	// initialize build datetime envvar
	err = h.initBuildDatetime()
	if err != nil {
		return err
	}

	// initialize build status envvar
	err = h.initBuildStatus()
	if err != nil {
		return err
	}

	return
}

func (h *envvarHelperImpl) setEstafetteBuilderConfigEnvvars(builderConfig contracts.BuilderConfig) (err error) {
	// set envvars that can be used by any container
	h.setEstafetteEnv("ESTAFETTE_GIT_SOURCE", builderConfig.Git.RepoSource)
	h.setEstafetteEnv("ESTAFETTE_GIT_OWNER", builderConfig.Git.RepoOwner)
	h.setEstafetteEnv("ESTAFETTE_GIT_NAME", builderConfig.Git.RepoName)
	h.setEstafetteEnv("ESTAFETTE_GIT_FULLNAME", fmt.Sprintf("%v/%v", builderConfig.Git.RepoOwner, builderConfig.Git.RepoName))

	h.setEstafetteEnv("ESTAFETTE_GIT_BRANCH", builderConfig.Git.RepoBranch)
	h.setEstafetteEnv("ESTAFETTE_GIT_REVISION", builderConfig.Git.RepoRevision)
	h.setEstafetteEnv("ESTAFETTE_BUILD_VERSION", builderConfig.BuildVersion.Version)
	if builderConfig.BuildVersion.Major != nil {
		h.setEstafetteEnv("ESTAFETTE_BUILD_VERSION_MAJOR", strconv.Itoa(*builderConfig.BuildVersion.Major))
	}
	if builderConfig.BuildVersion.Minor != nil {
		h.setEstafetteEnv("ESTAFETTE_BUILD_VERSION_MINOR", strconv.Itoa(*builderConfig.BuildVersion.Minor))
	}
	if builderConfig.BuildVersion.AutoIncrement != nil {
		h.setEstafetteEnv("ESTAFETTE_BUILD_VERSION_PATCH", strconv.Itoa(*builderConfig.BuildVersion.AutoIncrement))
	}
	if builderConfig.BuildVersion.Label != nil {
		h.setEstafetteEnv("ESTAFETTE_BUILD_VERSION_LABEL", *builderConfig.BuildVersion.Label)
	}
	if builderConfig.ReleaseParams != nil {
		h.setEstafetteEnv("ESTAFETTE_RELEASE_NAME", builderConfig.ReleaseParams.ReleaseName)
		h.setEstafetteEnv("ESTAFETTE_RELEASE_ACTION", builderConfig.ReleaseParams.ReleaseAction)
		h.setEstafetteEnv("ESTAFETTE_RELEASE_TRIGGERED_BY", builderConfig.ReleaseParams.TriggeredBy)
		// set ESTAFETTE_RELEASE_ID for backwards compatibility with extensions/slack-build-status
		h.setEstafetteEnv("ESTAFETTE_RELEASE_ID", strconv.Itoa(builderConfig.ReleaseParams.ReleaseID))
	}
	if builderConfig.BuildParams != nil {
		// set ESTAFETTE_BUILD_ID for backwards compatibility with extensions/github-status and extensions/bitbucket-status and extensions/slack-build-status
		h.setEstafetteEnv("ESTAFETTE_BUILD_ID", strconv.Itoa(builderConfig.BuildParams.BuildID))
	}

	// set ESTAFETTE_CI_SERVER_BASE_URL for backwards compatibility with extensions/github-status and extensions/bitbucket-status and extensions/slack-build-status
	if builderConfig.CIServer != nil {
		h.setEstafetteEnv("ESTAFETTE_CI_SERVER_BASE_URL", builderConfig.CIServer.BaseURL)
	}

	return h.setEstafetteEventEnvvars(builderConfig.Events)
}

func (h *envvarHelperImpl) setEstafetteEventEnvvars(events []*manifest.EstafetteEvent) (err error) {

	if events != nil {
		for _, e := range events {

			if e == nil {
				continue
			}

			triggerFields := reflect.TypeOf(*e)
			triggerValues := reflect.ValueOf(*e)

			for i := 0; i < triggerFields.NumField(); i++ {

				triggerField := triggerFields.Field(i).Name
				triggerValue := triggerValues.Field(i)

				if triggerValue.Kind() != reflect.Ptr || triggerValue.IsNil() {
					continue
				}

				// dereference the pointer
				derefencedPointerValue := reflect.Indirect(triggerValue)

				triggerPropertyFields := derefencedPointerValue.Type()
				triggerPropertyValues := derefencedPointerValue

				for j := 0; j < triggerPropertyFields.NumField(); j++ {

					triggerPropertyField := triggerPropertyFields.Field(j).Name
					triggerPropertyValue := triggerPropertyValues.Field(j)

					envvarName := "ESTAFETTE_TRIGGER_" + h.toUpperSnake(triggerField) + "_" + h.toUpperSnake(triggerPropertyField)
					envvarValue := ""

					switch triggerPropertyValue.Kind() {
					case reflect.String:
						envvarValue = triggerPropertyValue.String()
					default:
						jsonValue, _ := json.Marshal(triggerPropertyValue.Interface())
						envvarValue = string(jsonValue)

						envvarValue = strings.Trim(envvarValue, "\"")
					}

					h.setEstafetteEnv(envvarName, envvarValue)
				}
			}
		}
	}

	return nil
}

func (h *envvarHelperImpl) getGitOrigin() (string, error) {
	return h.getCommandOutput("git", "config", "--get", "remote.origin.url")
}

func (h *envvarHelperImpl) initGitSource() (err error) {
	if h.getEstafetteEnv("ESTAFETTE_GIT_SOURCE") == "" {
		origin, err := h.getGitOrigin()
		if err != nil {
			return err
		}
		source := h.getSourceFromOrigin(origin)
		return h.setEstafetteEnv("ESTAFETTE_GIT_SOURCE", source)
	}
	return
}

func (h *envvarHelperImpl) initGitOwner() (err error) {
	if h.getEstafetteEnv("ESTAFETTE_GIT_OWNER") == "" {
		origin, err := h.getGitOrigin()
		if err != nil {
			return err
		}
		owner := h.getOwnerFromOrigin(origin)
		return h.setEstafetteEnv("ESTAFETTE_GIT_OWNER", owner)
	}
	return
}

func (h *envvarHelperImpl) initGitName() (err error) {
	if h.getEstafetteEnv("ESTAFETTE_GIT_NAME") == "" {
		origin, err := h.getGitOrigin()
		if err != nil {
			return err
		}
		name := h.getNameFromOrigin(origin)
		return h.setEstafetteEnv("ESTAFETTE_GIT_NAME", name)
	}
	return
}

func (h *envvarHelperImpl) initGitFullName() (err error) {
	if h.getEstafetteEnv("ESTAFETTE_GIT_FULLNAME") == "" {
		origin, err := h.getGitOrigin()
		if err != nil {
			return err
		}
		owner := h.getOwnerFromOrigin(origin)
		name := h.getNameFromOrigin(origin)
		return h.setEstafetteEnv("ESTAFETTE_GIT_FULLNAME", fmt.Sprintf("%v/%v", owner, name))
	}
	return
}

func (h *envvarHelperImpl) getSourceFromOrigin(origin string) string {

	re := regexp.MustCompile(`^(git@|https://)([^:\/]+)(:|/)([^\/]+)/([^\/]+)\.git`)
	match := re.FindStringSubmatch(origin)

	if len(match) < 6 {
		return ""
	}

	return match[2]
}

func (h *envvarHelperImpl) getOwnerFromOrigin(origin string) string {

	re := regexp.MustCompile(`^(git@|https://)([^:\/]+)(:|/)([^\/]+)/([^\/]+)\.git`)
	match := re.FindStringSubmatch(origin)

	if len(match) < 6 {
		return ""
	}

	return match[4]
}

func (h *envvarHelperImpl) getNameFromOrigin(origin string) string {

	re := regexp.MustCompile(`^(git@|https://)([^:\/]+)(:|/)([^\/]+)/([^\/]+)\.git`)
	match := re.FindStringSubmatch(origin)

	if len(match) < 6 {
		return ""
	}

	return match[5]
}

func (h *envvarHelperImpl) initGitRevision() (err error) {
	if h.getEstafetteEnv("ESTAFETTE_GIT_REVISION") == "" {
		revision, err := h.getCommandOutput("git", "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		return h.setEstafetteEnv("ESTAFETTE_GIT_REVISION", revision)
	}
	return
}

func (h *envvarHelperImpl) initGitBranch() (err error) {
	if h.getEstafetteEnv("ESTAFETTE_GIT_BRANCH") == "" {
		branch, err := h.getCommandOutput("git", "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return err
		}
		return h.setEstafetteEnv("ESTAFETTE_GIT_BRANCH", branch)
	}
	return
}

func (h *envvarHelperImpl) initBuildDatetime() (err error) {
	if h.getEstafetteEnv("ESTAFETTE_BUILD_DATETIME") == "" {
		return h.setEstafetteEnv("ESTAFETTE_BUILD_DATETIME", time.Now().UTC().Format(time.RFC3339))
	}
	return
}

func (h *envvarHelperImpl) initBuildStatus() (err error) {
	if h.getEstafetteEnv("ESTAFETTE_BUILD_STATUS") == "" {
		return h.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	}
	return
}

func (h *envvarHelperImpl) initLabels(m manifest.EstafetteManifest) (err error) {

	// set labels as envvars
	if m.Labels != nil && len(m.Labels) > 0 {
		for key, value := range m.Labels {
			envvarName := "ESTAFETTE_LABEL_" + h.toUpperSnake(key)
			err = h.setEstafetteEnv(envvarName, value)
			if err != nil {
				return
			}
		}
	}

	return
}

func (h *envvarHelperImpl) collectEstafetteEnvvarsAndLabels(m manifest.EstafetteManifest) (envvars map[string]string) {

	// set labels as envvars
	h.initLabels(m)

	// return all envvars starting with ESTAFETTE_
	return h.collectEstafetteEnvvars()
}

func (h *envvarHelperImpl) collectEstafetteEnvvars() (envvars map[string]string) {

	// return all envvars starting with ESTAFETTE_
	envvars = map[string]string{}

	for _, e := range os.Environ() {
		kvPair := strings.SplitN(e, "=", 2)

		if len(kvPair) == 2 {
			envvarName := kvPair[0]
			envvarValue := kvPair[1]

			if strings.HasPrefix(envvarName, h.prefix) {
				envvars[envvarName] = envvarValue
			}
		}
	}

	return
}

func (h *envvarHelperImpl) collectGlobalEnvvars(m manifest.EstafetteManifest) (envvars map[string]string) {

	envvars = map[string]string{}

	if m.GlobalEnvVars != nil {
		envvars = m.GlobalEnvVars
	}

	return
}

func (h *envvarHelperImpl) collectStagesEnvvars(stages []*manifest.EstafetteStage) (envvars map[string]string) {

	envvars = map[string]string{}

	stagesJSONBytes, err := json.Marshal(stages)
	if err != nil {
		key := h.getEstafetteEnvvarName("ESTAFETTE_STAGES")
		envvars[key] = string(stagesJSONBytes)
	}

	return
}

// only to be used from unit tests
func (h *envvarHelperImpl) unsetEstafetteEnvvars() {

	envvarsToUnset := h.collectEstafetteEnvvars()
	for key := range envvarsToUnset {
		h.unsetEstafetteEnv(key)
	}
}

func (h *envvarHelperImpl) getEstafetteEnv(key string) string {

	key = h.getEstafetteEnvvarName(key)

	if strings.HasPrefix(key, h.prefix) {
		return os.Getenv(key)
	}

	return fmt.Sprintf("${%v}", key)
}

func (h *envvarHelperImpl) setEstafetteEnv(key, value string) error {

	key = h.getEstafetteEnvvarName(key)

	err := os.Setenv(key, value)
	if err != nil {
		return err
	}

	// set dns safe version
	err = os.Setenv(fmt.Sprintf("%v_DNS_SAFE", key), h.makeDNSLabelSafe(value))
	if err != nil {
		return err
	}

	return nil
}

func (h *envvarHelperImpl) unsetEstafetteEnv(key string) error {

	key = h.getEstafetteEnvvarName(key)

	return os.Unsetenv(key)
}

func (h *envvarHelperImpl) getEstafetteEnvvarName(key string) string {
	return strings.Replace(key, "ESTAFETTE_", h.prefix, -1)
}

func (h *envvarHelperImpl) overrideEnvvars(envvarMaps ...map[string]string) (envvars map[string]string) {

	envvars = make(map[string]string)
	for _, envvarMap := range envvarMaps {
		if envvarMap != nil && len(envvarMap) > 0 {
			for k, v := range envvarMap {
				envvars[k] = v
			}
		}
	}

	return
}

func (h *envvarHelperImpl) decryptSecret(encryptedValue string) (decryptedValue string) {

	decryptedValue, err := h.secretHelper.DecryptAllEnvelopes(encryptedValue)

	if err != nil {
		log.Warn().Err(err).Msg("Failed decrypting secret")
		return encryptedValue
	}

	return
}

func (h *envvarHelperImpl) decryptSecrets(encryptedEnvvars map[string]string) (envvars map[string]string) {

	if encryptedEnvvars == nil || len(encryptedEnvvars) == 0 {
		return encryptedEnvvars
	}

	envvars = make(map[string]string)
	for k, v := range encryptedEnvvars {
		envvars[k] = h.decryptSecret(v)
	}

	return
}

func (h *envvarHelperImpl) getCiServer() string {
	return h.ciServer
}

func (h *envvarHelperImpl) makeDNSLabelSafe(value string) string {
	// in order for the label to be used as a dns label (part between dots) it should only use
	// lowercase letters, digits and hyphens and have a max length of 63 characters;
	// also it should start with a letter and not end in a hyphen

	// ensure the label is lowercase
	value = strings.ToLower(value)

	// replace all invalid characters with a hyphen
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	value = reg.ReplaceAllString(value, "-")

	// replace double hyphens with a single one
	value = strings.Replace(value, "--", "-", -1)

	// trim hyphens from start and end
	value = strings.Trim(value, "-")

	// ensure it starts with a letter, not a digit or hyphen
	reg = regexp.MustCompile(`^[0-9-]+`)
	value = reg.ReplaceAllString(value, "")

	if len(value) > 63 {
		value = value[:63]
	}

	// trim hyphens from start and end
	value = strings.Trim(value, "-")

	return value
}
