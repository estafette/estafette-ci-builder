package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadManifest(t *testing.T) {

	t.Run("ReturnsManifestWithoutErrors", func(t *testing.T) {

		// act
		_, err := readManifest("test-manifest.yaml")

		assert.Nil(t, err)
	})

	t.Run("ReturnsManifestWithMappedLabels", func(t *testing.T) {

		// act
		manifest, err := readManifest("test-manifest.yaml")

		assert.Nil(t, err)
		assert.Equal(t, "estafette-ci-builder", manifest.Labels["app"])
		assert.Equal(t, "estafette-team", manifest.Labels["team"])
		assert.Equal(t, "golang", manifest.Labels["language"])
	})

	t.Run("ReturnsManifestWithMappedPipelines", func(t *testing.T) {

		// act
		manifest, err := readManifest("test-manifest.yaml")

		assert.Nil(t, err)

		assert.Equal(t, "golang:1.8.0-alpine", manifest.Pipelines["build"].ContainerImage)
		assert.Equal(t, "/go/src/github.com/estafette/estafette-ci-builder", manifest.Pipelines["build"].WorkingDirectory)
		assert.Equal(t, "go test -v ./...", manifest.Pipelines["build"].Commands[0])
		assert.Equal(t, "CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./publish/estafette-ci-builder .", manifest.Pipelines["build"].Commands[1])

		assert.Equal(t, "docker:17.03.0-ce", manifest.Pipelines["bake"].ContainerImage)
		assert.Equal(t, "cp Dockerfile ./publish", manifest.Pipelines["bake"].Commands[0])
		assert.Equal(t, "docker build -t estafette-ci-builder ./publish", manifest.Pipelines["bake"].Commands[1])
	})
}
