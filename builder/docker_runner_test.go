package builder

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	contracts "github.com/estafette/estafette-ci-contracts"
	crypt "github.com/estafette/estafette-ci-crypt"
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
		assert.Equal(t, "#!/bin/sh\nset -e\n\necho -e \"\\x1b[38;5;250m> exec go test ./...\\x1b[0m\"\nexec go test ./...", string(bytes))
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
		assert.Equal(t, "#!/bin/sh\nset -e\necho -e \"\\x1b[38;5;250m> go test ./... &\\x1b[0m\"\ngo test ./... &\ntrap \"kill $!; wait; exit\" 1 2 15\nwait $!\n\necho -e \"\\x1b[38;5;250m> exec go build\\x1b[0m\"\nexec go build", string(bytes))
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
		assert.Equal(t, "#!/bin/sh\nset -e\necho -e \"\\x1b[38;5;250m> go test ./... &\\x1b[0m\"\ngo test ./... &\ntrap \"kill $!; wait; exit\" 1 2 15\nwait $!\necho -e \"\\x1b[38;5;250m> export MY_TITLE_2=abc\\x1b[0m\"\nexport MY_TITLE_2=abc\necho -e \"\\x1b[38;5;250m> echo $MY_TITLE_2 &\\x1b[0m\"\necho $MY_TITLE_2 &\ntrap \"kill $!; wait; exit\" 1 2 15\nwait $!\n\necho -e \"\\x1b[38;5;250m> exec go build\\x1b[0m\"\nexec go build", string(bytes))
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
		assert.Equal(t, "#!/bin/sh\nset -e\necho -e \"\\x1b[38;5;250m> false || true\\x1b[0m\"\nfalse || true\n\necho -e \"\\x1b[38;5;250m> exec go build\\x1b[0m\"\nexec go build", string(bytes))
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
		assert.Equal(t, "#!/bin/sh\nset -e\necho -e \"\\x1b[38;5;250m> false && true\\x1b[0m\"\nfalse && true\n\necho -e \"\\x1b[38;5;250m> exec go build\\x1b[0m\"\nexec go build", string(bytes))
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
		assert.Equal(t, "#!/bin/sh\nset -e\necho -e \"\\x1b[38;5;250m> cat kubernetes.yaml | kubectl apply -f -\\x1b[0m\"\ncat kubernetes.yaml | kubectl apply -f -\n\necho -e \"\\x1b[38;5;250m> exec kubectl rollout status deploy/myapp\\x1b[0m\"\nexec kubectl rollout status deploy/myapp", string(bytes))
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
		assert.Equal(t, "#!/bin/sh\nset -e\necho -e \"\\x1b[38;5;250m> cd subdir\\x1b[0m\"\ncd subdir\n\necho -e \"\\x1b[38;5;250m> exec ls -latr\\x1b[0m\"\nexec ls -latr", string(bytes))
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
		assert.Equal(t, "#!/bin/sh\nset -e\necho -e \"\\x1b[38;5;250m> export \\$(python3 requiredenv.py)\\x1b[0m\"\nexport $(python3 requiredenv.py)\n\necho -e \"\\x1b[38;5;250m> exec ls -latr\\x1b[0m\"\nexec ls -latr", string(bytes))
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
		assert.Equal(t, "#!/bin/sh\nset -e\necho -e \"\\x1b[38;5;250m> shopt -u dotglob\\x1b[0m\"\nshopt -u dotglob\n\necho -e \"\\x1b[38;5;250m> exec ls -latr\\x1b[0m\"\nexec ls -latr", string(bytes))
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
		assert.Equal(t, "#!/bin/sh\nset -e\necho -e \"\\x1b[38;5;250m> if [ \\\"${VARIABLE}\\\" -ne \\\"\\\" ]; then echo $VARIABLE; fi\\x1b[0m\"\nif [ \"${VARIABLE}\" -ne \"\" ]; then echo $VARIABLE; fi\n\necho -e \"\\x1b[38;5;250m> exec go build\\x1b[0m\"\nexec go build", string(bytes))
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
		assert.Equal(t, "#!/bin/sh\nset -e\n\necho -e \"\\x1b[38;5;250m> exec echo \\\"<xml />\\\"\\x1b[0m\"\nexec echo \"<xml />\"", string(bytes))
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
		assert.Equal(t, "#!/bin/sh\nset -e\n\necho -e \"\\x1b[38;5;250m> exec echo '<xml />'\\x1b[0m\"\nexec echo '<xml />'", string(bytes))
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
		assert.Equal(t, "#!/bin/sh\nset -e\necho -e \"\\x1b[38;5;250m> go test ./...\\x1b[0m\"\ngo test ./...\n\necho -e \"\\x1b[38;5;250m> go build\\x1b[0m\"\ngo build", string(bytes))
	})
}

func getDockerRunnerAndMocks() (chan contracts.TailLogLine, ContainerRunner) {

	secretHelper := crypt.NewSecretHelper("SazbwMf3NZxVVbBqQHebPcXCqrVn3DDp", false)
	envvarHelper := NewEnvvarHelper("TESTPREFIX_", secretHelper, obfuscator)
	obfuscator := NewObfuscator(secretHelper)
	config := contracts.BuilderConfig{}
	tailLogsChannel := make(chan contracts.TailLogLine, 10000)

	dockerRunner := NewDockerRunner(envvarHelper, obfuscator, config, tailLogsChannel)

	return tailLogsChannel, dockerRunner
}
