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

	t.Run("ReturnsManifestWithMappedOrderedPipelinesInSameOrderAsInTheManifest", func(t *testing.T) {

		// act
		manifest, err := readManifest("test-manifest.yaml")

		assert.Nil(t, err)

		assert.Equal(t, 5, len(manifest.Pipelines))

		assert.Equal(t, "build", manifest.Pipelines[0].Name)
		assert.Equal(t, "golang:1.8.0-alpine", manifest.Pipelines[0].ContainerImage)
		assert.Equal(t, "/go/src/github.com/estafette/estafette-ci-builder", manifest.Pipelines[0].WorkingDirectory)
		assert.Equal(t, "go test -v ./...", manifest.Pipelines[0].Commands[0])
		assert.Equal(t, "CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./publish/estafette-ci-builder .", manifest.Pipelines[0].Commands[1])

		assert.Equal(t, "bake", manifest.Pipelines[1].Name)
		assert.Equal(t, "docker:17.03.0-ce", manifest.Pipelines[1].ContainerImage)
		assert.Equal(t, "cp Dockerfile ./publish", manifest.Pipelines[1].Commands[0])
		assert.Equal(t, "docker build -t estafette-ci-builder ./publish", manifest.Pipelines[1].Commands[1])

		assert.Equal(t, "set-build-status", manifest.Pipelines[2].Name)
		assert.Equal(t, "extensions/github-status:0.0.2", manifest.Pipelines[2].ContainerImage)
		assert.Equal(t, 0, len(manifest.Pipelines[2].Commands))
		assert.Equal(t, "server == 'estafette'", manifest.Pipelines[2].When)

		assert.Equal(t, "push-to-docker-hub", manifest.Pipelines[3].Name)
		assert.Equal(t, "docker:17.03.0-ce", manifest.Pipelines[3].ContainerImage)
		assert.Equal(t, "docker login --username=${ESTAFETTE_DOCKER_HUB_USERNAME} --password='${ESTAFETTE_DOCKER_HUB_PASSWORD}'", manifest.Pipelines[3].Commands[0])
		assert.Equal(t, "docker push estafette/${ESTAFETTE_LABEL_APP}:${ESTAFETTE_BUILD_VERSION}", manifest.Pipelines[3].Commands[1])
		assert.Equal(t, "status == 'succeeded' && branch == 'master'", manifest.Pipelines[3].When)

		assert.Equal(t, "slack-notify", manifest.Pipelines[4].Name)
		assert.Equal(t, "docker:17.03.0-ce", manifest.Pipelines[4].ContainerImage)
		assert.Equal(t, "curl -X POST --data-urlencode 'payload={\"channel\": \"#build-status\", \"username\": \"estafette-ci-builder\", \"text\": \"Build ${ESTAFETTE_BUILD_VERSION} for ${ESTAFETTE_LABEL_APP} has failed!\"}' ${ESTAFETTE_SLACK_WEBHOOK}", manifest.Pipelines[4].Commands[0])
		assert.Equal(t, "status == 'failed' || branch == 'master'", manifest.Pipelines[4].When)

		assert.Equal(t, "some value with spaces", manifest.Pipelines[4].EnvVars["SOME_ENVIRONMENT_VAR"])
		assert.Equal(t, "value1", manifest.Pipelines[4].CustomProperties["unknownProperty1"])
		assert.Equal(t, "value2", manifest.Pipelines[4].CustomProperties["unknownProperty2"])

		_, unknownPropertyExist := manifest.Pipelines[4].CustomProperties["unsupportedUnknownProperty"]
		assert.False(t, unknownPropertyExist)

		_, reservedPropertyForGolangNameExist := manifest.Pipelines[4].CustomProperties["ContainerImage"]
		assert.False(t, reservedPropertyForGolangNameExist)

		_, reservedPropertyForYamlNameExist := manifest.Pipelines[4].CustomProperties["image"]
		assert.False(t, reservedPropertyForYamlNameExist)
	})

	t.Run("ReturnsWorkDirDefaultIfMissing", func(t *testing.T) {

		// act
		manifest, err := readManifest("test-manifest.yaml")

		assert.Nil(t, err)

		assert.Equal(t, "/go/src/github.com/estafette/estafette-ci-builder", manifest.Pipelines[0].WorkingDirectory)
	})

	t.Run("ReturnsWorkDirIfSet", func(t *testing.T) {

		// act
		manifest, err := readManifest("test-manifest.yaml")

		assert.Nil(t, err)

		assert.Equal(t, "/estafette-work", manifest.Pipelines[1].WorkingDirectory)
	})

	t.Run("ReturnsShellDefaultIfMissing", func(t *testing.T) {

		// act
		manifest, err := readManifest("test-manifest.yaml")

		assert.Nil(t, err)

		assert.Equal(t, "/bin/sh", manifest.Pipelines[0].Shell)
	})

	t.Run("ReturnsShellIfSet", func(t *testing.T) {

		// act
		manifest, err := readManifest("test-manifest.yaml")

		assert.Nil(t, err)

		assert.Equal(t, "/bin/bash", manifest.Pipelines[1].Shell)
	})

	t.Run("ReturnsWhenIfSet", func(t *testing.T) {

		// act
		manifest, err := readManifest("test-manifest.yaml")

		assert.Nil(t, err)

		assert.Equal(t, "status == 'succeeded' && branch == 'master'", manifest.Pipelines[3].When)
	})

	t.Run("ReturnsWhenDefaultIfMissing", func(t *testing.T) {

		// act
		manifest, err := readManifest("test-manifest.yaml")

		assert.Nil(t, err)

		assert.Equal(t, "status == 'succeeded'", manifest.Pipelines[0].When)
	})
}

func TestGetReservedPropertyNames(t *testing.T) {

	t.Run("ReturnsListWithPropertyNamesAndYamlNames", func(t *testing.T) {

		// act
		names := getReservedPropertyNames()

		// yaml names
		assert.True(t, isReservedPopertyName(names, "image"))
		assert.True(t, isReservedPopertyName(names, "shell"))
		assert.True(t, isReservedPopertyName(names, "workDir"))
		assert.True(t, isReservedPopertyName(names, "commands"))
		assert.True(t, isReservedPopertyName(names, "when"))
		assert.True(t, isReservedPopertyName(names, "env"))

		// property names
		assert.True(t, isReservedPopertyName(names, "Name"))
		assert.True(t, isReservedPopertyName(names, "ContainerImage"))
		assert.True(t, isReservedPopertyName(names, "Shell"))
		assert.True(t, isReservedPopertyName(names, "WorkingDirectory"))
		assert.True(t, isReservedPopertyName(names, "Commands"))
		assert.True(t, isReservedPopertyName(names, "When"))
		assert.True(t, isReservedPopertyName(names, "EnvVars"))
		assert.True(t, isReservedPopertyName(names, "CustomProperties"))
	})
}
