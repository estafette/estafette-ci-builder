package envvar

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/estafette/estafette-ci-builder/clients/obfuscation"
	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func TestOverrideEnvvars(t *testing.T) {

	t.Run("CombinesAllEnvvarsFromPassedMaps", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		outerMap := map[string]string{
			"ENVVAR1": "value1",
		}
		innerMap := map[string]string{
			"ENVVAR2": "value2",
		}

		// act
		envvars := envvarClient.OverrideEnvvars(outerMap, innerMap)

		assert.Equal(t, 2, len(envvars))
	})

	t.Run("OverridesEnvarFromFirstMapWithSecondMap", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		outerMap := map[string]string{
			"ENVVAR1": "value1",
		}
		innerMap := map[string]string{
			"ENVVAR1": "value2",
		}

		// act
		envvars := envvarClient.OverrideEnvvars(outerMap, innerMap)

		assert.Equal(t, 1, len(envvars))
		assert.Equal(t, "value2", envvars["ENVVAR1"])
	})
}

func TestGetEstafetteEnvvarName(t *testing.T) {

	t.Run("ReturnsKeyNameWithEstafetteUnderscoreReplacedWithEstafetteEnvvarPrefixValue", func(t *testing.T) {

		envvarClient := getEnvvarClient()

		// act
		key := envvarClient.GetEstafetteEnvvarName("ESTAFETTE_KEY")

		assert.Equal(t, "TESTPREFIX_KEY", key)
	})
}

func TestCollectEstafetteEnvvarsAndLabels(t *testing.T) {

	t.Run("ReturnsEmptyMapIfManifestHasNoLabelsAndNoEnvvarsStartWithEstafette", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		manifest := manifest.EstafetteManifest{}

		// act
		envvars := envvarClient.CollectEstafetteEnvvarsAndLabels(manifest)

		assert.Equal(t, 0, len(envvars))
	})

	t.Run("ReturnsOneLabelAsEstafetteLabelLabel", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		manifest := manifest.EstafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder"}}

		// act
		envvars := envvarClient.CollectEstafetteEnvvarsAndLabels(manifest)

		_, exists := envvars["TESTPREFIX_LABEL_APP"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-builder", envvars["TESTPREFIX_LABEL_APP"])
	})

	t.Run("ReturnsOneLabelAsEstafetteLabelLabelWithSnakeCasing", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		manifest := manifest.EstafetteManifest{Labels: map[string]string{"owningTeam": "estafette-ci-team"}}

		// act
		envvars := envvarClient.CollectEstafetteEnvvarsAndLabels(manifest)

		log.Debug().Interface("envvars", envvars).Msg("")
		_, exists := envvars["TESTPREFIX_LABEL_OWNING_TEAM"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-team", envvars["TESTPREFIX_LABEL_OWNING_TEAM"])
	})

	t.Run("ReturnsTwoLabelsAsEstafetteLabelLabel", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		manifest := manifest.EstafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder", "team": "estafette-ci-team"}}

		// act
		envvars := envvarClient.CollectEstafetteEnvvarsAndLabels(manifest)

		_, exists := envvars["TESTPREFIX_LABEL_APP"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-builder", envvars["TESTPREFIX_LABEL_APP"])

		_, exists = envvars["TESTPREFIX_LABEL_TEAM"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-team", envvars["TESTPREFIX_LABEL_TEAM"])
	})

	t.Run("ReturnsOneEnvvarStartingWithEstafette", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		manifest := manifest.EstafetteManifest{}
		os.Setenv("TESTPREFIX_VERSION", "1.0.3")

		// act
		envvars := envvarClient.CollectEstafetteEnvvarsAndLabels(manifest)

		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "1.0.3", envvars["TESTPREFIX_VERSION"])
	})

	t.Run("ReturnsOneEnvvarStartingWithEstafetteIfValueContainsIsSymbol", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		manifest := manifest.EstafetteManifest{}
		os.Setenv("TESTPREFIX_VERSION", "b=c")

		// act
		envvars := envvarClient.CollectEstafetteEnvvarsAndLabels(manifest)

		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "b=c", envvars["TESTPREFIX_VERSION"])
	})

	t.Run("ReturnsTwoEnvvarsStartingWithEstafette", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		manifest := manifest.EstafetteManifest{}
		os.Setenv("TESTPREFIX_VERSION", "1.0.3")
		os.Setenv("TESTPREFIX_GIT_REPOSITORY", "git@github.com:estafette/estafette-ci-builder.git")

		// act
		envvars := envvarClient.CollectEstafetteEnvvarsAndLabels(manifest)

		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "1.0.3", envvars["TESTPREFIX_VERSION"])

		_, exists = envvars["TESTPREFIX_GIT_REPOSITORY"]
		assert.True(t, exists)
		assert.Equal(t, "git@github.com:estafette/estafette-ci-builder.git", envvars["TESTPREFIX_GIT_REPOSITORY"])
	})

	t.Run("ReturnsMixOfLabelsAndEnvvars", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		manifest := manifest.EstafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder"}}
		os.Setenv("TESTPREFIX_VERSION", "1.0.3")

		// act
		envvars := envvarClient.CollectEstafetteEnvvarsAndLabels(manifest)

		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "1.0.3", envvars["TESTPREFIX_VERSION"])

		_, exists = envvars["TESTPREFIX_LABEL_APP"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-builder", envvars["TESTPREFIX_LABEL_APP"])
	})
}

func TestCollectGlobalEnvvars(t *testing.T) {

	t.Run("ReturnsEmptyMapIfManifestHasNoGlobalEnvvars", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		manifest := manifest.EstafetteManifest{}

		// act
		envvars := envvarClient.CollectGlobalEnvvars(manifest)

		assert.Equal(t, 0, len(envvars))
	})

	t.Run("ReturnsGlobalEnvvarsIfManifestHasGlobalEnvvars", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		manifest := manifest.EstafetteManifest{GlobalEnvVars: map[string]string{"VAR_A": "Greetings", "VAR_B": "World"}}

		// act
		envvars := envvarClient.CollectGlobalEnvvars(manifest)

		assert.Equal(t, 2, len(envvars))
		assert.Equal(t, "Greetings", envvars["VAR_A"])
		assert.Equal(t, "World", envvars["VAR_B"])
	})
}

func TestGetEstafetteEnv(t *testing.T) {

	t.Run("ReturnsEnvironmentVariableValueIfItStartsWithEstafetteUnderscore", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		os.Setenv("TESTPREFIX_BUILD_STATUS", "succeeded")

		// act
		result := envvarClient.GetEstafetteEnv("TESTPREFIX_BUILD_STATUS")

		assert.Equal(t, "succeeded", result)
	})

	t.Run("ReturnsEnvironmentVariablePlaceholderIfItDoesNotStartWithEstafetteUnderscore", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		os.Setenv("HOME", "/root")

		// act
		result := envvarClient.GetEstafetteEnv("HOME")

		assert.Equal(t, "${HOME}", result)

	})
}

func TestDecryptSecret(t *testing.T) {

	t.Run("ReturnsOriginalValueIfDoesNotMatchEstafetteSecret", func(t *testing.T) {

		envvarClient := getEnvvarClient()

		// act
		result := envvarClient.DecryptSecret("not a secret", "github.com/estafette/estafette-ci-builder")

		assert.Equal(t, "not a secret", result)
	})

	t.Run("ReturnsUnencryptedValueIfMatchesEstafetteSecret", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		value := "estafette.secret(deFTz5Bdjg6SUe29.oPIkXbze5G9PNEWS2-ZnArl8BCqHnx4MdTdxHg37th9u)"

		// act
		result := envvarClient.DecryptSecret(value, "github.com/estafette/estafette-ci-builder")

		assert.Equal(t, "this is my secret", result)
	})

	t.Run("ReturnsUnencryptedValueIfMatchesEstafetteSecretWithDoubleEqualSign", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		value := "estafette.secret(yOQOYnIJAS1tN5eQ.Xaao3tVnwszu3OJ4XqGO0NMw8Cw0c0V3qA==)"

		// act
		result := envvarClient.DecryptSecret(value, "github.com/estafette/estafette-ci-builder")

		assert.Equal(t, "estafette", result)
	})
}

func TestDecryptSecrets(t *testing.T) {

	t.Run("ReturnsOriginalValueIfDoesNotMatchEstafetteSecret", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		envvars := map[string]string{
			"SOME_PLAIN_ENVVAR": "not a secret",
		}

		// act
		result := envvarClient.DecryptSecrets(envvars, "github.com/estafette/estafette-ci-builder")

		assert.Equal(t, 1, len(result))
		assert.Equal(t, "not a secret", result["SOME_PLAIN_ENVVAR"])
	})

	t.Run("ReturnsUnencryptedValueIfMatchesEstafetteSecret", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		envvars := map[string]string{
			"SOME_SECRET": "estafette.secret(deFTz5Bdjg6SUe29.oPIkXbze5G9PNEWS2-ZnArl8BCqHnx4MdTdxHg37th9u)",
		}

		// act
		result := envvarClient.DecryptSecrets(envvars, "github.com/estafette/estafette-ci-builder")

		assert.Equal(t, 1, len(result))
		assert.Equal(t, "this is my secret", result["SOME_SECRET"])
	})
}

func TestGetSourceFromOrigin(t *testing.T) {

	t.Run("ReturnsHostFromHttpsUrl", func(t *testing.T) {

		envvarClient := getEnvvarClient()

		// act
		source := envvarClient.GetSourceFromOrigin("https://github.com/estafette/estafette-gcloud-mig-scaler.git")

		assert.Equal(t, "github.com", source)
	})

	t.Run("ReturnsHostFromGitUrl", func(t *testing.T) {

		envvarClient := getEnvvarClient()

		// act
		source := envvarClient.GetSourceFromOrigin("git@github.com:estafette/estafette-ci-builder.git")

		assert.Equal(t, "github.com", source)
	})
}

func TestGetOwnerFromOrigin(t *testing.T) {

	t.Run("ReturnsOwnerFromHttpsUrl", func(t *testing.T) {

		envvarClient := getEnvvarClient()

		// act
		owner := envvarClient.GetOwnerFromOrigin("https://github.com/estafette/estafette-gcloud-mig-scaler.git")

		assert.Equal(t, "estafette", owner)
	})

	t.Run("ReturnsOwnerFromGitUrl", func(t *testing.T) {

		envvarClient := getEnvvarClient()

		// act
		owner := envvarClient.GetOwnerFromOrigin("git@github.com:estafette/estafette-ci-builder.git")

		assert.Equal(t, "estafette", owner)
	})
}

func TestGetNameFromOrigin(t *testing.T) {

	t.Run("ReturnsNameFromHttpsUrl", func(t *testing.T) {

		envvarClient := getEnvvarClient()

		// act
		name := envvarClient.GetNameFromOrigin("https://github.com/estafette/estafette-gcloud-mig-scaler.git")

		assert.Equal(t, "estafette-gcloud-mig-scaler", name)
	})

	t.Run("ReturnsNameFromGitUrl", func(t *testing.T) {

		envvarClient := getEnvvarClient()

		// act
		name := envvarClient.GetNameFromOrigin("git@github.com:estafette/estafette-ci-builder.git")

		assert.Equal(t, "estafette-ci-builder", name)
	})
}

func TestMakeDNSLabelSafe(t *testing.T) {

	t.Run("ReturnsValueIfAlreadySafeForDNSLabel", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		value := "dns-safe-value"

		// act
		safeValue := envvarClient.MakeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsAllLowercaseIfHasUppercase", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		value := "DNS-safe-value"

		// act
		safeValue := envvarClient.MakeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsAllLowercaseIfHasUppercase", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		value := "DNS-safe-value"

		// act
		safeValue := envvarClient.MakeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsCharactersOtherThanLettersDigitsOrHyphensAsHyphens", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		value := "dns-safe.value"

		// act
		safeValue := envvarClient.MakeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsMultipleHyphensAsSingleHyphen", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		value := "dns-safe--value"

		// act
		safeValue := envvarClient.MakeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsStartingWithLetter", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		value := "10-dns-safe-value"

		// act
		safeValue := envvarClient.MakeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsWithHyphensTrimmed", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		value := "-dns-safe-value-"

		// act
		safeValue := envvarClient.MakeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsTruncatedTo63Characters", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		value := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijkl"

		// act
		safeValue := envvarClient.MakeDNSLabelSafe(value)

		assert.Equal(t, "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijk", safeValue)
	})

	t.Run("ReturnsTruncatedTo63CharactersWithHyphensTrimmed", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		value := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghij-l"

		// act
		safeValue := envvarClient.MakeDNSLabelSafe(value)

		assert.Equal(t, "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghij", safeValue)
	})
}

func TestSetEstafetteEventEnvvars(t *testing.T) {

	t.Run("ReturnsPipelineEventPropertiesAsEnvvars", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		event := manifest.EstafetteEvent{
			Fired: true,
			Pipeline: &manifest.EstafettePipelineEvent{
				BuildVersion: "1.0.50-some-branch",
				RepoSource:   "github.com",
				RepoOwner:    "estafette",
				RepoName:     "estafette-ci-api",
				Branch:       "master",
				Status:       "succeeded",
				Event:        "finished",
			},
		}

		// act
		envvarClient.SetEstafetteEventEnvvars([]*manifest.EstafetteEvent{&event})

		envvars := envvarClient.CollectEstafetteEnvvars()
		assert.Equal(t, 14, len(envvars))
		assert.Equal(t, "1.0.50-some-branch", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_BUILD_VERSION"))
		assert.Equal(t, "github.com", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_REPO_SOURCE"))
		assert.Equal(t, "estafette", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_REPO_OWNER"))
		assert.Equal(t, "estafette-ci-api", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_REPO_NAME"))
		assert.Equal(t, "master", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_BRANCH"))
		assert.Equal(t, "succeeded", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_STATUS"))
		assert.Equal(t, "finished", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_EVENT"))
	})

	t.Run("ReturnsNamedPipelineEventPropertiesAsEnvvars", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		event := manifest.EstafetteEvent{
			Name:  "upstream",
			Fired: false,
			Pipeline: &manifest.EstafettePipelineEvent{
				BuildVersion: "1.0.50-some-branch",
				RepoSource:   "github.com",
				RepoOwner:    "estafette",
				RepoName:     "estafette-ci-api",
				Branch:       "master",
				Status:       "succeeded",
				Event:        "finished",
			},
		}

		// act
		envvarClient.SetEstafetteEventEnvvars([]*manifest.EstafetteEvent{&event})

		_ = envvarClient.CollectEstafetteEnvvars()
		assert.Equal(t, "1.0.50-some-branch", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_UPSTREAM_BUILD_VERSION"))
		assert.Equal(t, "github.com", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_UPSTREAM_REPO_SOURCE"))
		assert.Equal(t, "estafette", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_UPSTREAM_REPO_OWNER"))
		assert.Equal(t, "estafette-ci-api", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_UPSTREAM_REPO_NAME"))
		assert.Equal(t, "master", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_UPSTREAM_BRANCH"))
		assert.Equal(t, "succeeded", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_UPSTREAM_STATUS"))
		assert.Equal(t, "finished", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_UPSTREAM_EVENT"))
	})

	t.Run("ReturnsReleaseEventPropertiesAsEnvvars", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		event := manifest.EstafetteEvent{
			Fired: true,
			Release: &manifest.EstafetteReleaseEvent{
				ReleaseVersion: "1.0.50-some-branch",
				RepoSource:     "github.com",
				RepoOwner:      "estafette",
				RepoName:       "estafette-ci-api",
				Target:         "development",
				Status:         "succeeded",
				Event:          "finished",
			},
		}

		// act
		envvarClient.SetEstafetteEventEnvvars([]*manifest.EstafetteEvent{&event})

		envvars := envvarClient.CollectEstafetteEnvvars()
		assert.Equal(t, 14, len(envvars))
		assert.Equal(t, "1.0.50-some-branch", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_RELEASE_VERSION"))
		assert.Equal(t, "github.com", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_REPO_SOURCE"))
		assert.Equal(t, "estafette", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_REPO_OWNER"))
		assert.Equal(t, "estafette-ci-api", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_REPO_NAME"))
		assert.Equal(t, "development", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_TARGET"))
		assert.Equal(t, "succeeded", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_STATUS"))
		assert.Equal(t, "finished", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_EVENT"))
	})

	t.Run("ReturnsGitEventPropertiesAsEnvvars", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		event := manifest.EstafetteEvent{
			Fired: true,
			Git: &manifest.EstafetteGitEvent{
				Event:      "push",
				Repository: "github.com/estafette/estafette-ci-api",
				Branch:     "master",
			},
		}

		// act
		envvarClient.SetEstafetteEventEnvvars([]*manifest.EstafetteEvent{&event})

		envvars := envvarClient.CollectEstafetteEnvvars()
		assert.Equal(t, 6, len(envvars))
		assert.Equal(t, "push", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_GIT_EVENT"))
		assert.Equal(t, "github.com/estafette/estafette-ci-api", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_GIT_REPOSITORY"))
		assert.Equal(t, "master", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_GIT_BRANCH"))
	})

	t.Run("ReturnsCronEventPropertiesAsEnvvars", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		event := manifest.EstafetteEvent{
			Fired: true,
			Cron: &manifest.EstafetteCronEvent{
				Time: time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC),
			},
		}

		// act
		envvarClient.SetEstafetteEventEnvvars([]*manifest.EstafetteEvent{&event})

		envvars := envvarClient.CollectEstafetteEnvvars()
		assert.Equal(t, 2, len(envvars))
		assert.Equal(t, "2009-11-17T20:34:58.651387237Z", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_CRON_TIME"))
	})

	t.Run("ReturnsManualEventPropertiesAsEnvvars", func(t *testing.T) {

		envvarClient := getEnvvarClient()
		event := manifest.EstafetteEvent{
			Fired: true,
			Manual: &manifest.EstafetteManualEvent{
				UserID: "user@server.com",
			},
		}

		// act
		envvarClient.SetEstafetteEventEnvvars([]*manifest.EstafetteEvent{&event})

		envvars := envvarClient.CollectEstafetteEnvvars()
		assert.Equal(t, 2, len(envvars))
		assert.Equal(t, "user@server.com", envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER_MANUAL_USER_ID"))
	})
}

func getEnvvarClient() Client {
	ctx := context.Background()
	secretHelper := crypt.NewSecretHelper("SazbwMf3NZxVVbBqQHebPcXCqrVn3DDp", false)
	obfuscationClient, _ := obfuscation.NewClient(ctx, secretHelper)
	envvarClient, _ := NewClient(ctx, "TESTPREFIX_", secretHelper, obfuscationClient)
	envvarClient.UnsetEstafetteEnvvars()

	return envvarClient
}
