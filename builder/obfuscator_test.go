package builder

import (
	"encoding/json"
	"testing"

	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/stretchr/testify/assert"
)

var (
	obfuscator = NewObfuscator(secretHelper)
)

func TestObfuscate(t *testing.T) {

	t.Run("ObfuscatesSecretInManifest", func(t *testing.T) {

		manifest := manifest.EstafetteManifest{
			GlobalEnvVars: map[string]string{
				"MY_SECRET": "estafette.secret(deFTz5Bdjg6SUe29.oPIkXbze5G9PNEWS2-ZnArl8BCqHnx4MdTdxHg37th9u)",
			},
		}
		credentials := []*contracts.CredentialConfig{}
		pipeline := "github.com/estafette/estafette-ci-builder"
		credentialsBytes, _ := json.Marshal(credentials)

		obfuscator.CollectSecrets(manifest, credentialsBytes, pipeline)

		// act
		output := obfuscator.Obfuscate("this is my secret")

		assert.Equal(t, "***", output)
	})

	t.Run("ObfuscatesSecretInCredentials", func(t *testing.T) {

		manifest := manifest.EstafetteManifest{}
		credentials := []*contracts.CredentialConfig{
			&contracts.CredentialConfig{
				AdditionalProperties: map[string]interface{}{
					"password": "estafette.secret(deFTz5Bdjg6SUe29.oPIkXbze5G9PNEWS2-ZnArl8BCqHnx4MdTdxHg37th9u)",
				},
			},
		}
		pipeline := "github.com/estafette/estafette-ci-builder"
		credentialsBytes, _ := json.Marshal(credentials)

		obfuscator.CollectSecrets(manifest, credentialsBytes, pipeline)

		// act
		output := obfuscator.Obfuscate("this is my secret")

		assert.Equal(t, "***", output)
	})
}
