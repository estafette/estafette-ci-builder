package main

import (
	"os"
	"testing"

	mft "github.com/estafette/estafette-ci-manifest"
	"github.com/stretchr/testify/assert"
)

var (
	pipelineRunner = NewPipelineRunner(envvarHelper, whenEvaluator, dockerRunner)
)

func TestRunPipelines(t *testing.T) {

	t.Run("ReturnsResultWithoutErrorsWhenManifestHasNoStages", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(*manifest, dir, envvars)

		assert.Nil(t, err)
		assert.False(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithInnerResultForEachPipelineInManifest", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(*manifest, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, 1, len(result.StageResults))
	})

	t.Run("ReturnsResultWithoutErrorsWhenPipelinesSucceeded", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(*manifest, dir, envvars)

		assert.Nil(t, err)
		assert.False(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithSucceededPipelineResultWhenPipelinesSucceeded", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(*manifest, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, "SUCCEEDED", result.StageResults[0].Status)
	})

	t.Run("ReturnsResultWithErrorsWhenPipelinesFailed", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(*manifest, dir, envvars)

		assert.Nil(t, err)
		assert.True(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithFailedPipelineResultWhenPipelinesFailed", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(*manifest, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, "FAILED", result.StageResults[0].Status)
	})

	t.Run("ReturnsResultWithoutErrorsWhenPipelinesSkipped", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(*manifest, dir, envvars)

		assert.Nil(t, err)
		assert.False(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithSkippedPipelineResultWhenPipelinesSkipped", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(*manifest, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, "SKIPPED", result.StageResults[0].Status)
	})

	t.Run("ReturnsResultForAllPipelinesWhenFirstPipelineFails", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep2", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep3", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(*manifest, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, "FAILED", result.StageResults[0].Status)
		assert.Equal(t, "SKIPPED", result.StageResults[1].Status)
		assert.Equal(t, "SUCCEEDED", result.StageResults[2].Status)
	})

	t.Run("ReturnsResultWithErrorsWhenFirstPipelineFailsAndSecondSucceeds", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep2", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep3", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(*manifest, dir, envvars)

		assert.Nil(t, err)
		assert.True(t, result.HasErrors())
	})
}
