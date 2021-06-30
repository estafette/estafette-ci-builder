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
		assert.Equal(t, `#!/bin/sh
set -e

printf '\033[38;5;250m> exec %s\033[0m\n' 'go test ./...'
exec go test ./...`, string(bytes))
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
		assert.Equal(t, `#!/bin/sh
set -e

printf '\033[38;5;250m> %s &\033[0m\n' 'go test ./...'
go test ./... &
trap "kill $!; wait; exit" 1 2 15
wait $!

printf '\033[38;5;250m> exec %s\033[0m\n' 'go build'
exec go build`, string(bytes))
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
		assert.Equal(t, `#!/bin/sh
set -e

printf '\033[38;5;250m> %s &\033[0m\n' 'go test ./...'
go test ./... &
trap "kill $!; wait; exit" 1 2 15
wait $!

printf '\033[38;5;250m> %s\033[0m\n' 'export MY_TITLE_2=abc'
export MY_TITLE_2=abc

printf '\033[38;5;250m> %s &\033[0m\n' 'echo $MY_TITLE_2'
echo $MY_TITLE_2 &
trap "kill $!; wait; exit" 1 2 15
wait $!

printf '\033[38;5;250m> exec %s\033[0m\n' 'go build'
exec go build`, string(bytes))
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
		assert.Equal(t, `#!/bin/sh
set -e

printf '\033[38;5;250m> %s\033[0m\n' 'false || true'
false || true

printf '\033[38;5;250m> exec %s\033[0m\n' 'go build'
exec go build`, string(bytes))
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
		assert.Equal(t, `#!/bin/sh
set -e

printf '\033[38;5;250m> %s\033[0m\n' 'false && true'
false && true

printf '\033[38;5;250m> exec %s\033[0m\n' 'go build'
exec go build`, string(bytes))
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
		assert.Equal(t, `#!/bin/sh
set -e

printf '\033[38;5;250m> %s\033[0m\n' 'cat kubernetes.yaml | kubectl apply -f -'
cat kubernetes.yaml | kubectl apply -f -

printf '\033[38;5;250m> exec %s\033[0m\n' 'kubectl rollout status deploy/myapp'
exec kubectl rollout status deploy/myapp`, string(bytes))
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
		assert.Equal(t, `#!/bin/sh
set -e

printf '\033[38;5;250m> %s\033[0m\n' 'cd subdir'
cd subdir

printf '\033[38;5;250m> exec %s\033[0m\n' 'ls -latr'
exec ls -latr`, string(bytes))
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
		assert.Equal(t, `#!/bin/sh
set -e

printf '\033[38;5;250m> %s\033[0m\n' 'export $(python3 requiredenv.py)'
export $(python3 requiredenv.py)

printf '\033[38;5;250m> exec %s\033[0m\n' 'ls -latr'
exec ls -latr`, string(bytes))
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
		assert.Equal(t, `#!/bin/sh
set -e

printf '\033[38;5;250m> %s\033[0m\n' 'shopt -u dotglob'
shopt -u dotglob

printf '\033[38;5;250m> exec %s\033[0m\n' 'ls -latr'
exec ls -latr`, string(bytes))
	})

	t.Run("DoesNotRunCommandsWithSemicolonInBackground", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{`if [ "${VARIABLE}" -ne "" ]; then echo $VARIABLE; fi`, "go build"}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, `#!/bin/sh
set -e

printf '\033[38;5;250m> %s\033[0m\n' 'if [ "${VARIABLE}" -ne "" ]; then echo $VARIABLE; fi'
if [ "${VARIABLE}" -ne "" ]; then echo $VARIABLE; fi

printf '\033[38;5;250m> exec %s\033[0m\n' 'go build'
exec go build`, string(bytes))
	})

	t.Run("DoesNotEscapeDoubleQuotesInPrintfStatements", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{`echo "<xml />"`}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, `#!/bin/sh
set -e

printf '\033[38;5;250m> exec %s\033[0m\n' 'echo "<xml />"'
exec echo "<xml />"`, string(bytes))
	})

	t.Run("EscapeSingleQuotesInPrintfStatements", func(t *testing.T) {

		dockerRunner := dockerRunnerImpl{
			entrypointTemplateDir: "../templates",
		}

		// act
		hostPath, _, entrypointFile, err := dockerRunner.generateEntrypointScript("/bin/sh", []string{"echo '<xml />'"}, false)

		assert.Nil(t, err)
		bytes, err := ioutil.ReadFile(path.Join(hostPath, entrypointFile))
		assert.Nil(t, err)
		assert.Equal(t, `#!/bin/sh
set -e

printf '\033[38;5;250m> exec %s\033[0m\n' 'echo \'<xml />\''
exec echo '<xml />'`, string(bytes))
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
		assert.Equal(t, `#!/bin/sh
set -e

printf '\033[38;5;250m> %s\033[0m\n' 'go test ./...'
go test ./...

printf '\033[38;5;250m> %s\033[0m\n' 'go build'
go build`, string(bytes))
	})
}
