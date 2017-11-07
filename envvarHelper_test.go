package main

import (
	"os"
	"testing"

	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

var (
	secretHelper = crypt.NewSecretHelper("SazbwMf3NZxVVbBqQHebPcXCqrVn3DDp")
	envvarHelper = NewEnvvarHelper("TESTPREFIX_", secretHelper)
)

func TestOverrideEnvvars(t *testing.T) {

	t.Run("CombinesAllEnvvarsFromPassedMaps", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
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

		envvarHelper.unsetEstafetteEnvvars()
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

		envvarHelper.unsetEstafetteEnvvars()

		// act
		key := envvarHelper.getEstafetteEnvvarName("ESTAFETTE_KEY")

		assert.Equal(t, "TESTPREFIX_KEY", key)
	})
}

func TestCollectEstafetteEnvvars(t *testing.T) {

	t.Run("ReturnsEmptyMapIfManifestHasNoLabelsAndNoEnvvarsStartWithEstafette", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{}

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		assert.Equal(t, 0, len(envvars))
	})

	t.Run("ReturnsOneLabelAsEstafetteLabelLabel", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder"}}

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		assert.Equal(t, 1, len(envvars))
		_, exists := envvars["TESTPREFIX_LABEL_APP"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-builder", envvars["TESTPREFIX_LABEL_APP"])
	})

	t.Run("ReturnsOneLabelAsEstafetteLabelLabelWithSnakeCasing", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{Labels: map[string]string{"owningTeam": "estafette-ci-team"}}

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		log.Debug().Interface("envvars", envvars).Msg("")
		assert.Equal(t, 1, len(envvars))
		_, exists := envvars["TESTPREFIX_LABEL_OWNING_TEAM"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-team", envvars["TESTPREFIX_LABEL_OWNING_TEAM"])
	})

	t.Run("ReturnsTwoLabelsAsEstafetteLabelLabel", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder", "team": "estafette-ci-team"}}

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		assert.Equal(t, 2, len(envvars))
		_, exists := envvars["TESTPREFIX_LABEL_APP"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-builder", envvars["TESTPREFIX_LABEL_APP"])

		_, exists = envvars["TESTPREFIX_LABEL_TEAM"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-team", envvars["TESTPREFIX_LABEL_TEAM"])
	})

	t.Run("ReturnsOneEnvvarStartingWithEstafette", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{}
		os.Setenv("TESTPREFIX_VERSION", "1.0.3")

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		assert.Equal(t, 1, len(envvars))
		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "1.0.3", envvars["TESTPREFIX_VERSION"])
	})

	t.Run("ReturnsOneEnvvarStartingWithEstafetteIfValueContainsIsSymbol", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{}
		os.Setenv("TESTPREFIX_VERSION", "b=c")

		// act
		envvars := envvarHelper.collectEstafetteEnvvars(manifest)

		assert.Equal(t, 1, len(envvars))
		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "b=c", envvars["TESTPREFIX_VERSION"])
	})

	t.Run("ReturnsTwoEnvvarsStartingWithEstafette", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{}
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
	})

	t.Run("ReturnsMixOfLabelsAndEnvvars", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder"}}
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
	})
}

func TestGetEstafetteEnv(t *testing.T) {

	t.Run("ReturnsEnvironmentVariableValueIfItStartsWithEstafetteUnderscore", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		os.Setenv("TESTPREFIX_BUILD_STATUS", "succeeded")

		// act
		result := envvarHelper.getEstafetteEnv("TESTPREFIX_BUILD_STATUS")

		assert.Equal(t, "succeeded", result)
	})

	t.Run("ReturnsEnvironmentVariablePlaceholderIfItDoesNotStartWithEstafetteUnderscore", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		os.Setenv("HOME", "/root")

		// act
		result := envvarHelper.getEstafetteEnv("HOME")

		assert.Equal(t, "${HOME}", result)

	})
}

func TestDecryptSecret(t *testing.T) {

	t.Run("ReturnsOriginalValueIfDoesNotMatchEstafetteSecret", func(t *testing.T) {

		// act
		result := envvarHelper.decryptSecret("not a secret")

		assert.Equal(t, "not a secret", result)
	})

	t.Run("ReturnsUnencryptedValueIfMatchesEstafetteSecret", func(t *testing.T) {

		value := "estafette.secret(deFTz5Bdjg6SUe29.oPIkXbze5G9PNEWS2-ZnArl8BCqHnx4MdTdxHg37th9u)"

		// act
		result := envvarHelper.decryptSecret(value)

		assert.Equal(t, "this is my secret", result)
	})

	t.Run("ReturnsUnencryptedValueIfMatchesEstafetteSecretWithDoubleEqualSign", func(t *testing.T) {

		value := "estafette.secret(yOQOYnIJAS1tN5eQ.Xaao3tVnwszu3OJ4XqGO0NMw8Cw0c0V3qA==)"

		// act
		result := envvarHelper.decryptSecret(value)

		assert.Equal(t, "estafette", result)
	})
}

func TestDecryptSecrets(t *testing.T) {

	t.Run("ReturnsOriginalValueIfDoesNotMatchEstafetteSecret", func(t *testing.T) {

		envvars := map[string]string{
			"SOME_PLAIN_ENVVAR": "not a secret",
		}

		// act
		result := envvarHelper.decryptSecrets(envvars)

		assert.Equal(t, 1, len(result))
		assert.Equal(t, "not a secret", result["SOME_PLAIN_ENVVAR"])
	})

	t.Run("ReturnsUnencryptedValueIfMatchesEstafetteSecret", func(t *testing.T) {

		envvars := map[string]string{
			"SOME_SECRET": "estafette.secret(deFTz5Bdjg6SUe29.oPIkXbze5G9PNEWS2-ZnArl8BCqHnx4MdTdxHg37th9u)",
		}

		// act
		result := envvarHelper.decryptSecrets(envvars)

		assert.Equal(t, 1, len(result))
		assert.Equal(t, "this is my secret", result["SOME_SECRET"])
	})
}

func TestInterpolateEnvVars(t *testing.T) {
	getLookupFunc := func(envvars map[string]string) func(string) string {
		return func(key string) string {
			return envvars[key]
		}
	}

	t.Run("HandlesNilCollection", func(t *testing.T) {
		result := envvarHelper.interpolateEnvVars(nil, getLookupFunc(make(map[string]string)))
		assert.Nil(t, result)
	})

	t.Run("HandlesNilENvVars", func(t *testing.T) {
		result := envvarHelper.interpolateEnvVars(make(map[string]string), nil)
		assert.Nil(t, result)
	})

	t.Run("HandlesNormalStringAndDoesNotReplaceIt", func(t *testing.T) {
		envVars := map[string]string{
			"A": "replacedAValue",
		}
		collectedStrings := make(map[string]string)
		collectedStrings["VAR_A"] = "normalAValue"

		result := envvarHelper.interpolateEnvVars(collectedStrings, getLookupFunc(envVars))
		assert.Nil(t, result)
		assert.Equal(t, "normalAValue", collectedStrings["VAR_A"])
	})

	t.Run("HandlesPosixReplacement", func(t *testing.T) {
		envVars := map[string]string{
			"A": "replacedAValue",
		}
		collectedStrings := make(map[string]string)
		collectedStrings["VAR_A"] = "This is ${A}"

		result := envvarHelper.interpolateEnvVars(collectedStrings, getLookupFunc(envVars))
		assert.Nil(t, result)
		assert.Equal(t, "This is replacedAValue", collectedStrings["VAR_A"])
	})

	t.Run("HandlesBrokenValuesMissingEndTag", func(t *testing.T) {
		envVars := map[string]string{}
		collectedStrings := make(map[string]string)
		collectedStrings["VAR_A"] = "This is ${A-replacedValue"

		result := envvarHelper.interpolateEnvVars(collectedStrings, getLookupFunc(envVars))
		assert.Nil(t, result)
		assert.Equal(t, "This is A-replacedValue", collectedStrings["VAR_A"])
	})

	t.Run("HandlesBrokenValuesMissingStartTag", func(t *testing.T) {
		envVars := map[string]string{}
		collectedStrings := make(map[string]string)
		collectedStrings["VAR_A"] = "This is $A-replacedValue}"

		result := envvarHelper.interpolateEnvVars(collectedStrings, getLookupFunc(envVars))
		assert.Nil(t, result)
		assert.Equal(t, "This is -replacedValue}", collectedStrings["VAR_A"])
	})

	t.Run("HandlesSingleValue", func(t *testing.T) {
		envVars := map[string]string{
			"VARA": "valueA",
		}
		collectedStrings := make(map[string]string)
		collectedStrings["VAR_A"] = "$VARA"

		result := envvarHelper.interpolateEnvVars(collectedStrings, getLookupFunc(envVars))
		assert.Nil(t, result)
		assert.Equal(t, "valueA", collectedStrings["VAR_A"])
	})

	t.Run("HandlesEnvVarsWithNewlines", func(t *testing.T) {
		envVars := map[string]string{
			"VARA": "valueA\nvalueB",
		}
		collectedStrings := make(map[string]string)
		collectedStrings["VAR_A"] = "$VARA"

		result := envvarHelper.interpolateEnvVars(collectedStrings, getLookupFunc(envVars))
		assert.Nil(t, result)
		assert.Equal(t, "valueA\nvalueB", collectedStrings["VAR_A"])
	})

	t.Run("IgnoresSecrets", func(t *testing.T) {
		envVars := map[string]string{
			"VARA": "valueA\nvalueB",
		}
		collectedStrings := make(map[string]string)
		collectedStrings["VAR_A"] = "estafette.secret(IXFQ9igip3IH0KVY.N6RTT4RB9dz15UGKHQUBctAf5QNI8G8QYg==)"

		result := envvarHelper.interpolateEnvVars(collectedStrings, getLookupFunc(envVars))
		assert.Nil(t, result)
		assert.Equal(t, "estafette.secret(IXFQ9igip3IH0KVY.N6RTT4RB9dz15UGKHQUBctAf5QNI8G8QYg==)", collectedStrings["VAR_A"])
	})
}
