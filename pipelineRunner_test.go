package main

import (
	"os"
	"testing"

	mft "github.com/estafette/estafette-ci-manifest"
	"github.com/stretchr/testify/assert"
)

var (
	pipelineRunner = NewPipelineRunner(envvarHelper, whenEvaluator, dockerRunner, true)
)

func TestRunStages(t *testing.T) {

	t.Run("ReturnsErrorWhenManifestHasNoStages", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		_, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsResultWithInnerResultForEachStageInManifest", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, 1, len(result.StageResults))
	})

	t.Run("ReturnsResultWithoutErrorsWhenStagesSucceeded", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.False(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithSucceededPipelineResultWhenStagesSucceeded", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, "SUCCEEDED", result.StageResults[0].Status)
	})

	t.Run("ReturnsResultWithErrorsWhenStagesFailed", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.True(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithFailedPipelineResultWhenStagesFailed", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, "FAILED", result.StageResults[0].Status)
	})

	t.Run("ReturnsResultWithoutErrorsWhenStagesSkipped", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.False(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithSkippedStageResultWhenStagesSkipped", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, "SKIPPED", result.StageResults[0].Status)
	})

	t.Run("ReturnsResultForAllStagesWhenFirstStageFails", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep2", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep3", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, "FAILED", result.StageResults[0].Status)
		assert.Equal(t, "SKIPPED", result.StageResults[1].Status)
		assert.Equal(t, "SUCCEEDED", result.StageResults[2].Status)
	})

	t.Run("ReturnsResultWithErrorsWhenFirstStageFailsAndSecondSucceeds", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep2", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep3", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.True(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithSucceededPipelineResultWhenStagesSucceededAfterRetrial", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		cmd := "if [ -f retried ]; then rm retried && exit 0; else touch retried && exit 1; fi;"
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestRetryStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Retries: 1, Commands: []string{cmd}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.True(t, result.HasErrors())
		assert.Equal(t, "succeeded", result.Status)
	})
}
