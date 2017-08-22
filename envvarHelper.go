package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"

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
	collectEstafetteEnvvars(manifest.EstafetteManifest) map[string]string
	unsetEstafetteEnvvars()
	getEstafetteEnv(string) string
	setEstafetteEnv(string, string) error
	unsetEstafetteEnv(string) error
	getEstafetteEnvvarName(string) string
	overrideEnvvars(...map[string]string) map[string]string
}

type envvarHelperImpl struct {
	prefix string
}

// NewEnvvarHelper returns a new EnvvarHelper
func NewEnvvarHelper(prefix string) EnvvarHelper {
	return &envvarHelperImpl{
		prefix: prefix,
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

	return string(out)
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

func (h *envvarHelperImpl) collectEstafetteEnvvars(m manifest.EstafetteManifest) (envvars map[string]string) {

	// set labels as envvars
	if m.Labels != nil && len(m.Labels) > 0 {
		for key, value := range m.Labels {
			envvarName := "ESTAFETTE_LABEL_" + h.toUpperSnake(key)
			h.setEstafetteEnv(envvarName, value)
		}
	}

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

// only to be used from unit tests
func (h *envvarHelperImpl) unsetEstafetteEnvvars() {

	for _, e := range os.Environ() {
		kvPair := strings.SplitN(e, "=", 2)

		if len(kvPair) == 2 {
			envvarName := kvPair[0]

			if strings.HasPrefix(envvarName, h.prefix) {
				h.unsetEstafetteEnv(envvarName)
			}
		}
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