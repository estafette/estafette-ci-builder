package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunPipelines(t *testing.T) {

	t.Run("ReturnsResultWithoutErrorsWhenManifestHasNoPipelines", func(t *testing.T) {

		manifest := new(estafetteManifest)
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := runPipelines(*manifest, dir, envvars)

		assert.NotNil(t, err)
		assert.False(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithInnerResultForEachPipelineInManifest", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := runPipelines(*manifest, dir, envvars)

		assert.NotNil(t, err)
		assert.Equal(t, 1, len(result.PipelineResults))
	})

	t.Run("ReturnsResultWithoutErrorsWhenPipelinesSucceeded", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := runPipelines(*manifest, dir, envvars)

		assert.NotNil(t, err)
		assert.False(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithSucceededPipelineResultWhenPipelinesSucceeded", func(t *testing.T) {

		os.Setenv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := runPipelines(*manifest, dir, envvars)

		assert.NotNil(t, err)
		assert.Equal(t, "SUCCEEDED", result.PipelineResults[0].Status)
	})

	t.Run("ReturnsResultWithErrorsWhenPipelinesFailed", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := runPipelines(*manifest, dir, envvars)

		assert.NotNil(t, err)
		assert.True(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithFailedPipelineResultWhenPipelinesFailed", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := runPipelines(*manifest, dir, envvars)

		assert.NotNil(t, err)
		assert.Equal(t, "FAILED", result.PipelineResults[0].Status)
	})

	t.Run("ReturnsResultWithoutErrorsWhenPipelinesSkipped", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := runPipelines(*manifest, dir, envvars)

		assert.NotNil(t, err)
		assert.False(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithSkippedPipelineResultWhenPipelinesSkipped", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := runPipelines(*manifest, dir, envvars)

		assert.NotNil(t, err)
		assert.Equal(t, "SKIPPED", result.PipelineResults[0].Status)
	})

	t.Run("ReturnsResultForAllPipelinesWhenFirstPipelineFails", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep2", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep3", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := runPipelines(*manifest, dir, envvars)

		assert.NotNil(t, err)
		assert.Equal(t, "FAILED", result.PipelineResults[0].Status)
		assert.Equal(t, "SKIPPED", result.PipelineResults[1].Status)
		assert.Equal(t, "SUCCEEDED", result.PipelineResults[2].Status)
	})

	t.Run("ReturnsResultWithErrorsWhenFirstPipelineFailsAndSecondSucceeds", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep2", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep3", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := runPipelines(*manifest, dir, envvars)

		assert.NotNil(t, err)
		assert.True(t, result.HasErrors())
	})
}
