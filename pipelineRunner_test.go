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
		result := runPipelines(*manifest, dir, envvars)

		assert.False(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithInnerResultForEachPipelineInManifest", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result := runPipelines(*manifest, dir, envvars)

		assert.Equal(t, 1, len(result.PipelineResults))
	})

	t.Run("ReturnsResultWithoutErrorsWhenPipelinesSucceeded", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result := runPipelines(*manifest, dir, envvars)

		assert.Equal(t, "/Users/jorrit/WorkingCopies/go/src/github.com/estafette/estafette-ci-builder", dir)

		assert.False(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithSucceededPipelineResultWhenPipelinesSucceeded", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result := runPipelines(*manifest, dir, envvars)

		assert.Equal(t, "SUCCEEDED", result.PipelineResults[0].Status)
	})

	t.Run("ReturnsResultWithErrorsWhenPipelinesFailed", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result := runPipelines(*manifest, dir, envvars)

		assert.Equal(t, "/Users/jorrit/WorkingCopies/go/src/github.com/estafette/estafette-ci-builder", dir)

		assert.True(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithFailedPipelineResultWhenPipelinesFailed", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result := runPipelines(*manifest, dir, envvars)

		assert.Equal(t, "FAILED", result.PipelineResults[0].Status)
	})

	t.Run("ReturnsResultWithoutErrorsWhenPipelinesSkipped", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result := runPipelines(*manifest, dir, envvars)

		assert.False(t, result.HasErrors())
	})

	t.Run("ReturnsResultWithSkippedPipelineResultWhenPipelinesSkipped", func(t *testing.T) {

		manifest := new(estafetteManifest)
		manifest.Pipelines = append(manifest.Pipelines, &estafettePipeline{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result := runPipelines(*manifest, dir, envvars)

		assert.Equal(t, "SKIPPED", result.PipelineResults[0].Status)
	})
}
