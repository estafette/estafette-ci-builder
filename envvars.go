package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"
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

	cmd := exec.Command(name, arg...)
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	bytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func setEstafetteGlobalEnvvars() error {

	// set git revision
	revision, err := getCommandOutput("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	err = os.Setenv("ESTAFETTE_GIT_REVISION", revision)
	if err != nil {
		return err
	}

	// set git branch
	branch, err := getCommandOutput("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	err = os.Setenv("ESTAFETTE_GIT_BRANCH", branch)
	if err != nil {
		return err
	}

	// set build datetime
	err = os.Setenv("ESTAFETTE_BUILD_DATETIME", time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return err
	}

	return nil
}

func collectEstafetteEnvvars(m estafetteManifest) (envvars map[string]string) {

	envvars = map[string]string{}

	for _, e := range os.Environ() {
		kvPair := strings.Split(e, "=")
		if len(kvPair) == 2 {
			envvarName := kvPair[0]
			envvarValue := kvPair[1]

			if strings.HasPrefix(envvarName, "ESTAFETTE_") {
				envvars[envvarName] = envvarValue
			}
		}
	}

	// add the labels as envvars
	if m.Labels != nil && len(m.Labels) > 0 {
		for key, value := range m.Labels {

			envvarName := "ESTAFETTE_LABEL_" + toUpperSnake(key)
			envvars[envvarName] = value

			os.Setenv(envvarName, value)
		}
	}

	return
}
