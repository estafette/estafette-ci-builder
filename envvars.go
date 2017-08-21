package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"
)

var (
	// the default prefix for Estafette envvars, can be overridden for testing
	estafetteEnvvarPrefix = "ESTAFETTE_"
)

// https://gist.github.com/elwinar/14e1e897fdbe4d3432e1
func toUpperSnake(in string) string {
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

func getCommandOutput(name string, arg ...string) (string, error) {

	out, err := exec.Command(name, arg...).Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

func setEstafetteGlobalEnvvars() (err error) {

	// initialize git revision envvar
	err = initGitRevision()
	if err != nil {
		return err
	}

	// initialize git branch envvar
	err = initGitBranch()
	if err != nil {
		return err
	}

	// initialize build datetime envvar
	err = initBuildDatetime()
	if err != nil {
		return err
	}

	// initialize build status envvar
	err = initBuildStatus()
	if err != nil {
		return err
	}

	return
}

func initGitRevision() (err error) {
	if getEstafetteEnv("ESTAFETTE_GIT_REVISION") == "" {
		revision, err := getCommandOutput("git", "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		return setEstafetteEnv("ESTAFETTE_GIT_REVISION", revision)
	}
	return
}

func initGitBranch() (err error) {
	if getEstafetteEnv("ESTAFETTE_GIT_BRANCH") == "" {
		branch, err := getCommandOutput("git", "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return err
		}
		return setEstafetteEnv("ESTAFETTE_GIT_BRANCH", branch)
	}
	return
}

func initBuildDatetime() (err error) {
	if getEstafetteEnv("ESTAFETTE_BUILD_DATETIME") == "" {
		return setEstafetteEnv("ESTAFETTE_BUILD_DATETIME", time.Now().UTC().Format(time.RFC3339))
	}
	return
}

func initBuildStatus() (err error) {
	if getEstafetteEnv("ESTAFETTE_BUILD_STATUS") == "" {
		return setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
	}
	return
}

func collectEstafetteEnvvars(m estafetteManifest) (envvars map[string]string) {

	// set labels as envvars
	if m.Labels != nil && len(m.Labels) > 0 {
		for key, value := range m.Labels {
			envvarName := "ESTAFETTE_LABEL_" + toUpperSnake(key)
			setEstafetteEnv(envvarName, value)
		}
	}

	// return all envvars starting with ESTAFETTE_
	envvars = map[string]string{}

	for _, e := range os.Environ() {
		kvPair := strings.SplitN(e, "=", 2)

		if len(kvPair) == 2 {
			envvarName := getEstafetteEnvvarName(kvPair[0])
			envvarValue := kvPair[1]

			if strings.HasPrefix(envvarName, estafetteEnvvarPrefix) {
				envvars[envvarName] = envvarValue
			}
		}
	}

	return
}

func getEstafetteEnv(key string) string {

	key = getEstafetteEnvvarName(key)

	if strings.HasPrefix(key, estafetteEnvvarPrefix) {
		return os.Getenv(key)
	}

	return fmt.Sprintf("${%v}", key)
}

func setEstafetteEnv(key, value string) error {

	key = getEstafetteEnvvarName(key)

	return os.Setenv(key, value)
}

func unsetEstafetteEnv(key string) error {

	key = getEstafetteEnvvarName(key)

	return os.Unsetenv(key)
}

func getEstafetteEnvvarName(key string) string {
	return strings.Replace(key, "ESTAFETTE_", estafetteEnvvarPrefix, -1)
}

func overrideEnvvars(envvarMaps ...map[string]string) (envvars map[string]string) {

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
