package main

import (
	"os"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func TestOverrideEnvvars(t *testing.T) {

	t.Run("CombinesAllEnvvarsFromPassedMaps", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TESTPREFIX_")
		outerMap := map[string]string{
			"ENVVAR1": "value1",
		}
		innerMap := map[string]string{
			"ENVVAR2": "value2",
		}

		// act
		envvars := envvarHelper.overrideEnvvars(outerMap, innerMap)

		assert.Equal(t, 2, len(envvars))
	})

	t.Run("OverridesEnvarFromFirstMapWithSecondMap", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TESTPREFIX_")
		outerMap := map[string]string{
			"ENVVAR1": "value1",
		}
		innerMap := map[string]string{
			"ENVVAR1": "value2",
		}

		// act
		envvars := envvarHelper.overrideEnvvars(outerMap, innerMap)

		assert.Equal(t, 1, len(envvars))
		assert.Equal(t, "value2", envvars["ENVVAR1"])
	})
}

func TestGetEstafetteEnvvarName(t *testing.T) {

	t.Run("ReturnsKeyNameWithEstafetteUnderscoreReplacedWithEstafetteEnvvarPrefixValue", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TESTPREFIX_")
		envvarHelper.unsetEstafetteEnvvars()

		// act
		key := envvarHelper.getEstafetteEnvvarName("ESTAFETTE_KEY")

		assert.Equal(t, "TESTPREFIX_KEY", key)

		// clean up
		envvarHelper.unsetEstafetteEnvvars()
	})
}

func TestCollectEstafetteEnvvars(t *testing.T) {

	t.Run("ReturnsEmptyMapIfManifestHasNoLabelsAndNoEnvvarsStartWithEstafette", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TESTPREFIX_")
		envvarHelper.unsetEstafetteEnvvars()
		manifest := estafetteManifest{}

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		log.Debug().Interface("envvars", envvars).Msg("ReturnsEmptyMapIfManifestHasNoLabelsAndNoEnvvarsStartWithEstafette")
		assert.Equal(t, 0, len(envvars))

		// clean up
		envvarHelper.unsetEstafetteEnvvars()
	})

	t.Run("ReturnsOneLabelAsEstafetteLabelLabel", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TESTPREFIX_")
		envvarHelper.unsetEstafetteEnvvars()
		manifest := estafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder"}}

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		log.Debug().Interface("envvars", envvars).Msg("ReturnsOneLabelAsEstafetteLabelLabel")
		assert.Equal(t, 1, len(envvars))
		_, exists := envvars["TESTPREFIX_LABEL_APP"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-builder", envvars["TESTPREFIX_LABEL_APP"])

		// clean up
		envvarHelper.unsetEstafetteEnvvars()
	})

	t.Run("ReturnsOneLabelAsEstafetteLabelLabelWithSnakeCasing", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TESTPREFIX_")
		envvarHelper.unsetEstafetteEnvvars()
		manifest := estafetteManifest{Labels: map[string]string{"owningTeam": "estafette-ci-team"}}

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		log.Debug().Interface("envvars", envvars).Msg("ReturnsOneLabelAsEstafetteLabelLabelWithSnakeCasing")
		assert.Equal(t, 1, len(envvars))
		_, exists := envvars["TESTPREFIX_LABEL_OWNING_TEAM"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-team", envvars["TESTPREFIX_LABEL_OWNING_TEAM"])

		// clean up
		envvarHelper.unsetEstafetteEnvvars()
	})

	t.Run("ReturnsTwoLabelsAsEstafetteLabelLabel", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TESTPREFIX_")
		envvarHelper.unsetEstafetteEnvvars()
		manifest := estafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder", "team": "estafette-ci-team"}}

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		assert.Equal(t, 2, len(envvars))
		_, exists := envvars["TESTPREFIX_LABEL_APP"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-builder", envvars["TESTPREFIX_LABEL_APP"])

		_, exists = envvars["TESTPREFIX_LABEL_TEAM"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-team", envvars["TESTPREFIX_LABEL_TEAM"])

		// clean up
		envvarHelper.unsetEstafetteEnvvars()
	})

	t.Run("ReturnsOneEnvvarStartingWithEstafette", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TESTPREFIX_")
		envvarHelper.unsetEstafetteEnvvars()
		manifest := estafetteManifest{}
		os.Setenv("TESTPREFIX_VERSION", "1.0.3")

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		assert.Equal(t, 1, len(envvars))
		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "1.0.3", envvars["TESTPREFIX_VERSION"])

		// clean up
		envvarHelper.unsetEstafetteEnvvars()
	})

	t.Run("ReturnsOneEnvvarStartingWithEstafetteIfValueContainsIsSymbol", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TESTPREFIX_")
		envvarHelper.unsetEstafetteEnvvars()
		manifest := estafetteManifest{}
		os.Setenv("TESTPREFIX_VERSION", "b=c")

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		assert.Equal(t, 1, len(envvars))
		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "b=c", envvars["TESTPREFIX_VERSION"])

		// clean up
		envvarHelper.unsetEstafetteEnvvars()
	})

	t.Run("ReturnsTwoEnvvarsStartingWithEstafette", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TESTPREFIX_")
		envvarHelper.unsetEstafetteEnvvars()
		manifest := estafetteManifest{}
		os.Setenv("TESTPREFIX_VERSION", "1.0.3")
		os.Setenv("TESTPREFIX_GIT_REPOSITORY", "git@github.com:estafette/estafette-ci-builder.git")

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		assert.Equal(t, 2, len(envvars))
		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "1.0.3", envvars["TESTPREFIX_VERSION"])

		_, exists = envvars["TESTPREFIX_GIT_REPOSITORY"]
		assert.True(t, exists)
		assert.Equal(t, "git@github.com:estafette/estafette-ci-builder.git", envvars["TESTPREFIX_GIT_REPOSITORY"])

		// clean up
		envvarHelper.unsetEstafetteEnvvars()
	})

	t.Run("ReturnsMixOfLabelsAndEnvvars", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TESTPREFIX_")
		envvarHelper.unsetEstafetteEnvvars()
		manifest := estafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder"}}
		os.Setenv("TESTPREFIX_VERSION", "1.0.3")

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		assert.Equal(t, 2, len(envvars))
		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "1.0.3", envvars["TESTPREFIX_VERSION"])

		_, exists = envvars["TESTPREFIX_LABEL_APP"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-builder", envvars["TESTPREFIX_LABEL_APP"])

		// clean up
		envvarHelper.unsetEstafetteEnvvars()
	})
}

func TestGetEstafetteEnv(t *testing.T) {

	t.Run("ReturnsEnvironmentVariableValueIfItStartsWithEstafetteUnderscore", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TESTPREFIX_")
		envvarHelper.unsetEstafetteEnvvars()
		os.Setenv("TESTPREFIX_BUILD_STATUS", "succeeded")

		// act
		result := envvarHelper.getEstafetteEnv("TESTPREFIX_BUILD_STATUS")

		assert.Equal(t, "succeeded", result)

		// clean up
		envvarHelper.unsetEstafetteEnvvars()
	})

	t.Run("ReturnsEnvironmentVariablePlaceholderIfItDoesNotStartWithEstafetteUnderscore", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TESTPREFIX_")
		envvarHelper.unsetEstafetteEnvvars()
		os.Setenv("HOME", "/root")

		// act
		result := envvarHelper.getEstafetteEnv("HOME")

		assert.Equal(t, "${HOME}", result)

		// clean up
		envvarHelper.unsetEstafetteEnvvars()
	})
}
