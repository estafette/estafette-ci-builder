package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/rs/zerolog/log"

	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
)

// EnvvarHelper is the interface for getting, setting and retrieving ESTAFETTE_ environment variables
type EnvvarHelper interface {
	toUpperSnake(string) string
	getCommandOutput(string, ...string) (string, error)
	setEstafetteGlobalEnvvars() error
	initGitRevision() error
	initGitBranch() error
	initBuildDatetime() error
	initBuildStatus() error
	initLabels(manifest.EstafetteManifest) error
	collectEstafetteEnvvars() map[string]string
	collectEstafetteEnvvarsAndLabels(manifest.EstafetteManifest) map[string]string
	collectGlobalEnvvars(manifest.EstafetteManifest) map[string]string
	unsetEstafetteEnvvars()
	getEstafetteEnv(string) string
	setEstafetteEnv(string, string) error
	unsetEstafetteEnv(string) error
	getEstafetteEnvvarName(string) string
	overrideEnvvars(...map[string]string) map[string]string
	decryptSecret(string) string
	decryptSecrets(map[string]string) map[string]string
	getCiServer() string
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

	return os.Setenv(key, value)
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

	r, err := regexp.Compile("^estafette\\.secret\\(([a-zA-Z0-9.=_-]+)\\)$")
	if err != nil {
		log.Warn().Err(err).Msg("Failed compiling regexp")
		return encryptedValue
	}

	matches := r.FindStringSubmatch(encryptedValue)
	if matches == nil {
		return encryptedValue
	}

	decryptedValue, err = h.secretHelper.Decrypt(matches[1])
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
