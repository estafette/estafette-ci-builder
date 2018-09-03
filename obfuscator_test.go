package main

import (
	"testing"

	"github.com/estafette/estafette-ci-manifest"
	"github.com/stretchr/testify/assert"
)

var (
	obfuscator = NewObfuscator(secretHelper)
)

func TestObfuscate(t *testing.T) {

	t.Run("ObfuscatesSecretGlobalEnvvar", func(t *testing.T) {

		manifest := manifest.EstafetteManifest{
			GlobalEnvVars: map[string]string{
				"MY_SECRET": "estafette.secret(deFTz5Bdjg6SUe29.oPIkXbze5G9PNEWS2-ZnArl8BCqHnx4MdTdxHg37th9u)",
			},
		}

		obfuscator.CollectSecrets(manifest)

		// act
		output := obfuscator.Obfuscate("this is my secret")

		assert.Equal(t, "***", output)
	})
}
