package builder

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateEntrypointScript(t *testing.T) {

	t.Run("ReturnsHostPathStartingWithTempDir", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, _, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"go test ./..."}, false)

		assert.Nil(t, err)
		assert.True(t, strings.HasPrefix(hostPath, os.TempDir()))
	})

	t.Run("ReturnsMountPathToEntrypoint", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		_, mountPath, _, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"go test ./..."}, false)

		assert.Nil(t, err)
		assert.Equal(t, "/entrypoint", mountPath)
	})

	t.Run("ReturnsEntrypointFileEntrypointScript", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		_, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"go test ./..."}, false)

		assert.Nil(t, err)
		assert.Equal(t, "entrypoint.sh", entrypointFile)
	})

	t.Run("ReturnsVariablesForOneCommand", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"go test ./..."}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\n\nprintf '\\033[38;5;250m> exec %s\\033[0m' \"go test ./...\"\nexec go test ./...", string(bytes))
	})

	t.Run("ReturnsVariablesForTwoOrMoreCommands", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"go test ./...", "go build"}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\nprintf '\\033[38;5;250m> %s &\\033[0m' \"go test ./...\"\ngo test ./... &\ntrap \"kill $!; wait; exit\" 1 2 15\nwait $!\n\nprintf '\\033[38;5;250m> exec %s\\033[0m' \"go build\"\nexec go build", string(bytes))
	})

	t.Run("DoesNotRunVariableAssignmentInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"go test ./...", "export MY_TITLE_2=abc", "echo $MY_TITLE_2", "go build"}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\nprintf '\\033[38;5;250m> %s &\\033[0m' \"go test ./...\"\ngo test ./... &\ntrap \"kill $!; wait; exit\" 1 2 15\nwait $!\nprintf '\\033[38;5;250m> %s\\033[0m' \"export MY_TITLE_2=abc\"\nexport MY_TITLE_2=abc\nprintf '\\033[38;5;250m> %s &\\033[0m' \"echo $MY_TITLE_2\"\necho $MY_TITLE_2 &\ntrap \"kill $!; wait; exit\" 1 2 15\nwait $!\n\nprintf '\\033[38;5;250m> exec %s\\033[0m' \"go build\"\nexec go build", string(bytes))
	})

	t.Run("DoesNotRunCommandsWithOrInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"false || true", "go build"}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\nprintf '\\033[38;5;250m> %s\\033[0m' \"false || true\"\nfalse || true\n\nprintf '\\033[38;5;250m> exec %s\\033[0m' \"go build\"\nexec go build", string(bytes))
	})

	t.Run("DoesNotRunCommandsWithAndInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"false && true", "go build"}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\nprintf '\\033[38;5;250m> %s\\033[0m' \"false && true\"\nfalse && true\n\nprintf '\\033[38;5;250m> exec %s\\033[0m' \"go build\"\nexec go build", string(bytes))
	})

	t.Run("DoesNotRunCommandsWithPipeInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"cat kubernetes.yaml | kubectl apply -f -", "kubectl rollout status deploy/myapp"}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\nprintf '\\033[38;5;250m> %s\\033[0m' \"cat kubernetes.yaml | kubectl apply -f -\"\ncat kubernetes.yaml | kubectl apply -f -\n\nprintf '\\033[38;5;250m> exec %s\\033[0m' \"kubectl rollout status deploy/myapp\"\nexec kubectl rollout status deploy/myapp", string(bytes))
	})

	t.Run("DoesNotRunCommandsWithChangeDirectoryInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"cd subdir", "ls -latr"}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\nprintf '\\033[38;5;250m> %s\\033[0m' \"cd subdir\"\ncd subdir\n\nprintf '\\033[38;5;250m> exec %s\\033[0m' \"ls -latr\"\nexec ls -latr", string(bytes))
	})

	t.Run("DoesNotRunCommandsWithExportInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"export $(python3 requiredenv.py)", "ls -latr"}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\nprintf '\\033[38;5;250m> %s\\033[0m' \"export \\$(python3 requiredenv.py)\"\nexport $(python3 requiredenv.py)\n\nprintf '\\033[38;5;250m> exec %s\\033[0m' \"ls -latr\"\nexec ls -latr", string(bytes))
	})

	t.Run("DoesNotRunCommandsWithShoptInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"shopt -u dotglob", "ls -latr"}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\nprintf '\\033[38;5;250m> %s\\033[0m' \"shopt -u dotglob\"\nshopt -u dotglob\n\nprintf '\\033[38;5;250m> exec %s\\033[0m' \"ls -latr\"\nexec ls -latr", string(bytes))
	})

	t.Run("DoesNotRunCommandsWithSemicolonInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"if [ \"${VARIABLE}\" -ne \"\" ]; then echo $VARIABLE; fi", "go build"}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\nprintf '\\033[38;5;250m> %s\\033[0m' \"if [ \\\"${VARIABLE}\\\" -ne \\\"\\\" ]; then echo $VARIABLE; fi\"\nif [ \"${VARIABLE}\" -ne \"\" ]; then echo $VARIABLE; fi\n\nprintf '\\033[38;5;250m> exec %s\\033[0m' \"go build\"\nexec go build", string(bytes))
	})

	t.Run("EscapesDoubleQuotesInEchoStatements", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"echo \"<xml />\""}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\n\nprintf '\\033[38;5;250m> exec %s\\033[0m' \"echo \\\"<xml />\\\"\"\nexec echo \"<xml />\"", string(bytes))
	})

	t.Run("DoesNotEscapeSingleQuotesInEchoStatements", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"echo '<xml />'"}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\n\nprintf '\\033[38;5;250m> exec %s\\033[0m' \"echo '<xml />'\"\nexec echo '<xml />'", string(bytes))
	})

	t.Run("DoesNotRunAnyCommandInBackgroundWhenRunCommandsInForegroundIsTrue", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"go test ./...", "go build"}, true)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, "#!/bin/sh\nset -e\nprintf '\\033[38;5;250m> %s\\033[0m' \"go test ./...\"\ngo test ./...\n\nprintf '\\033[38;5;250m> %s\\033[0m' \"go build\"\ngo build", string(bytes))
	})
}
