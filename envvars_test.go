package main

// import (
// 	"os"
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// )

// func TestCollectEstafetteEnvvars(t *testing.T) {

// 	t.Run("ReturnsEmptyMapIfManifestHasNoLabelsAndNoEnvvarsStartWithEstafette", func(t *testing.T) {

// 		manifest := estafetteManifest{}

// 		// act
// 		envvars := collectEstafetteEnvvars(manifest)

// 		assert.Equal(t, 0, len(envvars))
// 	})

// 	t.Run("ReturnsOneLabelAsEstafetteLabelLabel", func(t *testing.T) {

// 		manifest := estafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder"}}

// 		// act
// 		envvars := collectEstafetteEnvvars(manifest)

// 		assert.Equal(t, 1, len(envvars))
// 		_, exists := envvars["ESTAFETTE_LABEL_APP"]
// 		assert.True(t, exists)
// 		assert.Equal(t, "estafette-ci-builder", envvars["ESTAFETTE_LABEL_APP"])

// 		// clean up
// 		os.Unsetenv("ESTAFETTE_LABEL_APP")
// 	})

// 	t.Run("ReturnsOneLabelAsEstafetteLabelLabelWithSnakeCasing", func(t *testing.T) {

// 		manifest := estafetteManifest{Labels: map[string]string{"owningTeam": "estafette-ci-team"}}

// 		// act
// 		envvars := collectEstafetteEnvvars(manifest)

// 		assert.Equal(t, 1, len(envvars))
// 		_, exists := envvars["ESTAFETTE_LABEL_OWNING_TEAM"]
// 		assert.True(t, exists)
// 		assert.Equal(t, "estafette-ci-team", envvars["ESTAFETTE_LABEL_OWNING_TEAM"])

// 		// clean up
// 		os.Unsetenv("ESTAFETTE_LABEL_OWNING_TEAM")
// 	})

// 	t.Run("ReturnsTwoLabelsAsEstafetteLabelLabel", func(t *testing.T) {

// 		manifest := estafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder", "team": "estafette-ci-team"}}

// 		// act
// 		envvars := collectEstafetteEnvvars(manifest)

// 		assert.Equal(t, 2, len(envvars))
// 		_, exists := envvars["ESTAFETTE_LABEL_APP"]
// 		assert.True(t, exists)
// 		assert.Equal(t, "estafette-ci-builder", envvars["ESTAFETTE_LABEL_APP"])

// 		_, exists = envvars["ESTAFETTE_LABEL_TEAM"]
// 		assert.True(t, exists)
// 		assert.Equal(t, "estafette-ci-team", envvars["ESTAFETTE_LABEL_TEAM"])

// 		// clean up
// 		os.Unsetenv("ESTAFETTE_LABEL_APP")
// 		os.Unsetenv("ESTAFETTE_LABEL_TEAM")
// 	})

// 	t.Run("ReturnsOneEnvvarStartingWithEstafette", func(t *testing.T) {

// 		manifest := estafetteManifest{}
// 		os.Setenv("ESTAFETTE_VERSION", "1.0.3")

// 		// act
// 		envvars := collectEstafetteEnvvars(manifest)

// 		assert.Equal(t, 1, len(envvars))
// 		_, exists := envvars["ESTAFETTE_VERSION"]
// 		assert.True(t, exists)
// 		assert.Equal(t, "1.0.3", envvars["ESTAFETTE_VERSION"])

// 		// clean up
// 		os.Unsetenv("ESTAFETTE_VERSION")
// 	})

// 	t.Run("ReturnsOneEnvvarStartingWithEstafetteIfValueContainsIsSymbol", func(t *testing.T) {

// 		manifest := estafetteManifest{}
// 		os.Setenv("ESTAFETTE_VERSION", "b=c")

// 		// act
// 		envvars := collectEstafetteEnvvars(manifest)

// 		assert.Equal(t, 1, len(envvars))
// 		_, exists := envvars["ESTAFETTE_VERSION"]
// 		assert.True(t, exists)
// 		assert.Equal(t, "b=c", envvars["ESTAFETTE_VERSION"])

// 		// clean up
// 		os.Unsetenv("ESTAFETTE_VERSION")
// 	})

// 	t.Run("ReturnsTwoEnvvarsStartingWithEstafette", func(t *testing.T) {

// 		manifest := estafetteManifest{}
// 		os.Setenv("ESTAFETTE_VERSION", "1.0.3")
// 		os.Setenv("ESTAFETTE_GIT_REPOSITORY", "git@github.com:estafette/estafette-ci-builder.git")

// 		// act
// 		envvars := collectEstafetteEnvvars(manifest)

// 		assert.Equal(t, 2, len(envvars))
// 		_, exists := envvars["ESTAFETTE_VERSION"]
// 		assert.True(t, exists)
// 		assert.Equal(t, "1.0.3", envvars["ESTAFETTE_VERSION"])

// 		_, exists = envvars["ESTAFETTE_GIT_REPOSITORY"]
// 		assert.True(t, exists)
// 		assert.Equal(t, "git@github.com:estafette/estafette-ci-builder.git", envvars["ESTAFETTE_GIT_REPOSITORY"])

// 		// clean up
// 		os.Unsetenv("ESTAFETTE_VERSION")
// 		os.Unsetenv("ESTAFETTE_GIT_REPOSITORY")
// 	})

// 	t.Run("ReturnsMixOfLabelsAndEnvvars", func(t *testing.T) {

// 		manifest := estafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder"}}
// 		os.Setenv("ESTAFETTE_VERSION", "1.0.3")

// 		// act
// 		envvars := collectEstafetteEnvvars(manifest)

// 		assert.Equal(t, 2, len(envvars))
// 		_, exists := envvars["ESTAFETTE_VERSION"]
// 		assert.True(t, exists)
// 		assert.Equal(t, "1.0.3", envvars["ESTAFETTE_VERSION"])

// 		_, exists = envvars["ESTAFETTE_LABEL_APP"]
// 		assert.True(t, exists)
// 		assert.Equal(t, "estafette-ci-builder", envvars["ESTAFETTE_LABEL_APP"])

// 		// clean up
// 		os.Unsetenv("ESTAFETTE_VERSION")
// 		os.Unsetenv("ESTAFETTE_LABEL_APP")
// 	})
// }

// func TestGetEstafetteEnv(t *testing.T) {

// 	t.Run("ReturnsEnvironmentVariableValueIfItStartsWithEstafetteUnderscore", func(t *testing.T) {

// 		os.Setenv("ESTAFETTE_BUILD_STATUS", "succeeded")

// 		// act
// 		result := getEstafetteEnv("ESTAFETTE_BUILD_STATUS")

// 		assert.Equal(t, "succeeded", result)

// 		// clean up
// 		os.Unsetenv("ESTAFETTE_BUILD_STATUS")
// 	})

// 	t.Run("ReturnsEnvironmentVariablePlaceholderIfItDoesNotStartWithEstafetteUnderscore", func(t *testing.T) {

// 		os.Setenv("HOME", "/root")

// 		// act
// 		result := getEstafetteEnv("HOME")

// 		assert.Equal(t, "${HOME}", result)
// 	})
// }
