package main

import (
	"os"
	"testing"
	"time"

	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

var (
	secretHelper = crypt.NewSecretHelper("SazbwMf3NZxVVbBqQHebPcXCqrVn3DDp", false)
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

func TestCollectEstafetteEnvvarsAndLabels(t *testing.T) {

	t.Run("ReturnsEmptyMapIfManifestHasNoLabelsAndNoEnvvarsStartWithEstafette", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{}

		// act
		envvars := envvarHelper.collectEstafetteEnvvarsAndLabels(manifest)

		assert.Equal(t, 0, len(envvars))
	})

	t.Run("ReturnsOneLabelAsEstafetteLabelLabel", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder"}}

		// act
		envvars := envvarHelper.collectEstafetteEnvvarsAndLabels(manifest)

		_, exists := envvars["TESTPREFIX_LABEL_APP"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-builder", envvars["TESTPREFIX_LABEL_APP"])
	})

	t.Run("ReturnsOneLabelAsEstafetteLabelLabelWithSnakeCasing", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{Labels: map[string]string{"owningTeam": "estafette-ci-team"}}

		// act
		envvars := envvarHelper.collectEstafetteEnvvarsAndLabels(manifest)

		log.Debug().Interface("envvars", envvars).Msg("")
		_, exists := envvars["TESTPREFIX_LABEL_OWNING_TEAM"]
		assert.True(t, exists)
		assert.Equal(t, "estafette-ci-team", envvars["TESTPREFIX_LABEL_OWNING_TEAM"])
	})

	t.Run("ReturnsTwoLabelsAsEstafetteLabelLabel", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{Labels: map[string]string{"app": "estafette-ci-builder", "team": "estafette-ci-team"}}

		// act
		envvars := envvarHelper.collectEstafetteEnvvarsAndLabels(manifest)

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
		envvars := envvarHelper.collectEstafetteEnvvarsAndLabels(manifest)

		_, exists := envvars["TESTPREFIX_VERSION"]
		assert.True(t, exists)
		assert.Equal(t, "1.0.3", envvars["TESTPREFIX_VERSION"])
	})

	t.Run("ReturnsOneEnvvarStartingWithEstafetteIfValueContainsIsSymbol", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{}
		os.Setenv("TESTPREFIX_VERSION", "b=c")

		// act
		envvars := envvarHelper.collectEstafetteEnvvarsAndLabels(manifest)

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
		envvars := envvarHelper.collectEstafetteEnvvarsAndLabels(manifest)

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
		envvars := envvarHelper.collectEstafetteEnvvarsAndLabels(manifest)

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

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{}

		// act
		envvars := envvarHelper.collectGlobalEnvvars(manifest)

		assert.Equal(t, 0, len(envvars))
	})

	t.Run("ReturnsGlobalEnvvarsIfManifestHasGlobalEnvvars", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		manifest := manifest.EstafetteManifest{GlobalEnvVars: map[string]string{"VAR_A": "Greetings", "VAR_B": "World"}}

		// act
		envvars := envvarHelper.collectGlobalEnvvars(manifest)

		assert.Equal(t, 2, len(envvars))
		assert.Equal(t, "Greetings", envvars["VAR_A"])
		assert.Equal(t, "World", envvars["VAR_B"])
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

func TestToUpperSnake(t *testing.T) {

	t.Run("ReturnsLowercaseAsUppercase", func(t *testing.T) {

		// act
		snake := envvarHelper.toUpperSnake("lowercase")

		assert.Equal(t, "LOWERCASE", snake)
	})

	t.Run("ReturnsPascalCaseAsUppercaseWithUnderscoreBetweenWords", func(t *testing.T) {

		// act
		snake := envvarHelper.toUpperSnake("PascalCase")

		assert.Equal(t, "PASCAL_CASE", snake)
	})

	t.Run("ReturnsCamelCaseAsUppercaseWithUnderscoreBetweenWords", func(t *testing.T) {

		// act
		snake := envvarHelper.toUpperSnake("camelCase")

		assert.Equal(t, "CAMEL_CASE", snake)
	})

	t.Run("ReturnsHyphenSeparatedCaseAsUppercaseWithUnderscoreBetweenWords", func(t *testing.T) {

		// act
		snake := envvarHelper.toUpperSnake("kubernetes-engine")

		assert.Equal(t, "KUBERNETES_ENGINE", snake)
	})
}

func TestGetSourceFromOrigin(t *testing.T) {

	t.Run("ReturnsHostFromHttpsUrl", func(t *testing.T) {

		// act
		source := envvarHelper.getSourceFromOrigin("https://github.com/estafette/estafette-gcloud-mig-scaler.git")

		assert.Equal(t, "github.com", source)
	})

	t.Run("ReturnsHostFromGitUrl", func(t *testing.T) {

		// act
		source := envvarHelper.getSourceFromOrigin("git@github.com:estafette/estafette-ci-builder.git")

		assert.Equal(t, "github.com", source)
	})
}

func TestGetOwnerFromOrigin(t *testing.T) {

	t.Run("ReturnsOwnerFromHttpsUrl", func(t *testing.T) {

		// act
		owner := envvarHelper.getOwnerFromOrigin("https://github.com/estafette/estafette-gcloud-mig-scaler.git")

		assert.Equal(t, "estafette", owner)
	})

	t.Run("ReturnsOwnerFromGitUrl", func(t *testing.T) {

		// act
		owner := envvarHelper.getOwnerFromOrigin("git@github.com:estafette/estafette-ci-builder.git")

		assert.Equal(t, "estafette", owner)
	})
}

func TestGetNameFromOrigin(t *testing.T) {

	t.Run("ReturnsNameFromHttpsUrl", func(t *testing.T) {

		// act
		name := envvarHelper.getNameFromOrigin("https://github.com/estafette/estafette-gcloud-mig-scaler.git")

		assert.Equal(t, "estafette-gcloud-mig-scaler", name)
	})

	t.Run("ReturnsNameFromGitUrl", func(t *testing.T) {

		// act
		name := envvarHelper.getNameFromOrigin("git@github.com:estafette/estafette-ci-builder.git")

		assert.Equal(t, "estafette-ci-builder", name)
	})
}

func TestMakeDNSLabelSafe(t *testing.T) {

	t.Run("ReturnsValueIfAlreadySafeForDNSLabel", func(t *testing.T) {

		value := "dns-safe-value"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsAllLowercaseIfHasUppercase", func(t *testing.T) {

		value := "DNS-safe-value"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsAllLowercaseIfHasUppercase", func(t *testing.T) {

		value := "DNS-safe-value"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsCharactersOtherThanLettersDigitsOrHyphensAsHyphens", func(t *testing.T) {

		value := "dns-safe.value"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsMultipleHyphensAsSingleHyphen", func(t *testing.T) {

		value := "dns-safe--value"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsStartingWithLetter", func(t *testing.T) {

		value := "10-dns-safe-value"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsWithHyphensTrimmed", func(t *testing.T) {

		value := "-dns-safe-value-"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "dns-safe-value", safeValue)
	})

	t.Run("ReturnsTruncatedTo63Characters", func(t *testing.T) {

		value := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijkl"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijk", safeValue)
	})

	t.Run("ReturnsTruncatedTo63CharactersWithHyphensTrimmed", func(t *testing.T) {

		value := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghij-l"

		// act
		safeValue := envvarHelper.makeDNSLabelSafe(value)

		assert.Equal(t, "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghij", safeValue)
	})
}

func TestSetEstafetteEventEnvvars(t *testing.T) {

	t.Run("ReturnsPipelineEventPropertiesAsEnvvars", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		event := manifest.EstafetteEvent{
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
		envvarHelper.setEstafetteEventEnvvars([]*manifest.EstafetteEvent{&event})

		envvars := envvarHelper.collectEstafetteEnvvars()
		assert.Equal(t, 14, len(envvars))
		assert.Equal(t, "1.0.50-some-branch", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_BUILD_VERSION"))
		assert.Equal(t, "github.com", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_REPO_SOURCE"))
		assert.Equal(t, "estafette", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_REPO_OWNER"))
		assert.Equal(t, "estafette-ci-api", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_REPO_NAME"))
		assert.Equal(t, "master", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_BRANCH"))
		assert.Equal(t, "succeeded", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_STATUS"))
		assert.Equal(t, "finished", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_PIPELINE_EVENT"))
	})

	t.Run("ReturnsReleaseEventPropertiesAsEnvvars", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		event := manifest.EstafetteEvent{
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
		envvarHelper.setEstafetteEventEnvvars([]*manifest.EstafetteEvent{&event})

		envvars := envvarHelper.collectEstafetteEnvvars()
		assert.Equal(t, 14, len(envvars))
		assert.Equal(t, "1.0.50-some-branch", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_RELEASE_VERSION"))
		assert.Equal(t, "github.com", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_REPO_SOURCE"))
		assert.Equal(t, "estafette", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_REPO_OWNER"))
		assert.Equal(t, "estafette-ci-api", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_REPO_NAME"))
		assert.Equal(t, "development", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_TARGET"))
		assert.Equal(t, "succeeded", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_STATUS"))
		assert.Equal(t, "finished", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_RELEASE_EVENT"))
	})

	t.Run("ReturnsGitEventPropertiesAsEnvvars", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		event := manifest.EstafetteEvent{
			Git: &manifest.EstafetteGitEvent{
				Event:      "push",
				Repository: "github.com/estafette/estafette-ci-api",
				Branch:     "master",
			},
		}

		// act
		envvarHelper.setEstafetteEventEnvvars([]*manifest.EstafetteEvent{&event})

		envvars := envvarHelper.collectEstafetteEnvvars()
		assert.Equal(t, 6, len(envvars))
		assert.Equal(t, "push", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_GIT_EVENT"))
		assert.Equal(t, "github.com/estafette/estafette-ci-api", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_GIT_REPOSITORY"))
		assert.Equal(t, "master", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_GIT_BRANCH"))
	})

	t.Run("ReturnsCronEventPropertiesAsEnvvars", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		event := manifest.EstafetteEvent{
			Cron: &manifest.EstafetteCronEvent{
				Time: time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC),
			},
		}

		// act
		envvarHelper.setEstafetteEventEnvvars([]*manifest.EstafetteEvent{&event})

		envvars := envvarHelper.collectEstafetteEnvvars()
		assert.Equal(t, 2, len(envvars))
		assert.Equal(t, "2009-11-17T20:34:58.651387237Z", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_CRON_TIME"))
	})

	t.Run("ReturnsManualEventPropertiesAsEnvvars", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		event := manifest.EstafetteEvent{
			Manual: &manifest.EstafetteManualEvent{
				UserID: "user@server.com",
			},
		}

		// act
		envvarHelper.setEstafetteEventEnvvars([]*manifest.EstafetteEvent{&event})

		envvars := envvarHelper.collectEstafetteEnvvars()
		assert.Equal(t, 2, len(envvars))
		assert.Equal(t, "user@server.com", envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER_MANUAL_USER_ID"))
	})
}

func TestSetEstafetteStagesEnvvar(t *testing.T) {

	t.Run("ReturnsJsonSerializedStages", func(t *testing.T) {

		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				ContainerImage: "extensions/git-clone:stable",
			},
			&manifest.EstafetteStage{
				ContainerImage: "extensions/github-build-status:stable",
			},
			&manifest.EstafetteStage{
				ContainerImage: "extensions/docker:stable",
			},
		}

		// act
		err := envvarHelper.setEstafetteStagesEnvvar(stages)

		assert.Nil(t, err)
		assert.Equal(t, "[{\"Name\":\"\",\"ContainerImage\":\"extensions/git-clone:stable\",\"Shell\":\"\",\"WorkingDirectory\":\"\",\"Commands\":null,\"When\":\"\",\"EnvVars\":null,\"AutoInjected\":false,\"Retries\":0,\"CustomProperties\":null},{\"Name\":\"\",\"ContainerImage\":\"extensions/github-build-status:stable\",\"Shell\":\"\",\"WorkingDirectory\":\"\",\"Commands\":null,\"When\":\"\",\"EnvVars\":null,\"AutoInjected\":false,\"Retries\":0,\"CustomProperties\":null},{\"Name\":\"\",\"ContainerImage\":\"extensions/docker:stable\",\"Shell\":\"\",\"WorkingDirectory\":\"\",\"Commands\":null,\"When\":\"\",\"EnvVars\":null,\"AutoInjected\":false,\"Retries\":0,\"CustomProperties\":null}]", envvarHelper.getEstafetteEnv("ESTAFETTE_STAGES"))
	})
}

func TestSetEstafetteStageImagesEnvvar(t *testing.T) {

	t.Run("ReturnsJsonSerializedStageImages", func(t *testing.T) {

		stages := []*manifest.EstafetteStage{
			&manifest.EstafetteStage{
				ContainerImage: "extensions/git-clone:stable",
			},
			&manifest.EstafetteStage{
				ContainerImage: "extensions/github-build-status:stable",
			},
			&manifest.EstafetteStage{
				ContainerImage: "extensions/docker:stable",
			},
		}

		// act
		err := envvarHelper.setEstafetteStageImagesEnvvar(stages)

		assert.Nil(t, err)
		assert.Equal(t, "[\"extensions/git-clone:stable\",\"extensions/github-build-status:stable\",\"extensions/docker:stable\"]", envvarHelper.getEstafetteEnv("ESTAFETTE_STAGE_IMAGES"))
	})
}
