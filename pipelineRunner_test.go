package main

import (
	"context"
	"os"
	"testing"

	mft "github.com/estafette/estafette-ci-manifest"
	"github.com/stretchr/testify/assert"
)

var (
	pipelineRunner = NewPipelineRunner(envvarHelper, whenEvaluator, dockerRunner, true, make(chan struct{}))
)

func TestRunStages(t *testing.T) {

	t.Run("ReturnsErrorWhenManifestHasNoStages", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		_, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.NotNil(t, err, "Error: %v", err)
	})

	t.Run("ReturnsResultWithInnerResultForEachStageInManifest", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

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
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.False(t, result.HasAggregatedErrors())
	})

	t.Run("ReturnsResultWithSucceededPipelineResultWhenStagesSucceeded", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

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
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.True(t, result.HasAggregatedErrors())
	})

	t.Run("ReturnsResultWithFailedPipelineResultWhenStagesFailed", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

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
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.False(t, result.HasAggregatedErrors())
	})

	t.Run("ReturnsResultWithSkippedStageResultWhenStagesSkipped", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

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
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

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
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.True(t, result.HasAggregatedErrors())
	})

	t.Run("ReturnsResultWithoutErrorsWhenStagesSucceededAfterRetrial", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		cmd := "if [ -f retried ]; then rm retried && exit 0; else touch retried && exit 1; fi;"
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestRetryStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Retries: 1, Commands: []string{cmd}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.False(t, result.HasAggregatedErrors())
	})

	t.Run("ReturnsResultWithErrorsWhenStagesFailedAfterRetrial", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		cmd := "exit 1"
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestRetryStep2", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Retries: 1, Commands: []string{cmd}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.True(t, result.HasAggregatedErrors())
	})
}
