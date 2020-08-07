package builder

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
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

	t.Run("ObfuscatesBase64EncodedSecretInManifest", func(t *testing.T) {

		pipeline := "github.com/estafette/estafette-ci-builder"
		unencryptedValue := "{\n  \"type\": \"service_account\",\n  \"project_id\": \"my-project-id\",\n  \"private_key_id\": \"sf8re97843hewrhuifsdidf\",\n  \"private_key\": \"-----BEGIN PRIVATE KEY-----\\n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\\n-----END PRIVATE KEY-----\\n\",\n  \"client_email\": \"my-sa@my-project-id.iam.gserviceaccount.com\",\n  \"client_id\": \"34598543985498345\",\n  \"auth_uri\": \"https://accounts.google.com/o/oauth2/auth\",\n  \"token_uri\": \"https://accounts.google.com/o/oauth2/token\",\n  \"auth_provider_x509_cert_url\": \"https://www.googleapis.com/oauth2/v1/certs\",\n  \"client_x509_cert_url\": \"https://www.googleapis.com/robot/v1/metadata/x509/my-sa%40my-project-id.iam.gserviceaccount.com\"\n}"
		base64UnencryptedValue := base64.StdEncoding.EncodeToString([]byte(unencryptedValue))
		encryptedTextInEnvelope, err := secretHelper.EncryptEnvelope(base64UnencryptedValue, pipeline)
		assert.Nil(t, err)

		manifest := manifest.EstafetteManifest{
			GlobalEnvVars: map[string]string{
				"MY_SA": encryptedTextInEnvelope,
			},
		}
		credentials := []*contracts.CredentialConfig{}
		credentialsBytes, _ := json.Marshal(credentials)

		obfuscator.CollectSecrets(manifest, credentialsBytes, pipeline)

		unencryptedValueLines := strings.Split(strings.ReplaceAll(unencryptedValue, "\\n", "\n"), "\n")
		assert.Equal(t, 50, len(unencryptedValueLines))
		for _, l := range unencryptedValueLines {
			// act
			output := obfuscator.Obfuscate(fmt.Sprintf("%v", l))
			assert.Equal(t, "***", output)
		}
	})

	t.Run("ObfuscatesBase64EncodedSecretInManifestWhenEncodedNewlineIsNotOutputedAsNewline", func(t *testing.T) {

		pipeline := "github.com/estafette/estafette-ci-builder"
		unencryptedValue := "{\n  \"type\": \"service_account\",\n  \"project_id\": \"my-project-id\",\n  \"private_key_id\": \"sf8re97843hewrhuifsdidf\",\n  \"private_key\": \"-----BEGIN PRIVATE KEY-----\\n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\\n-----END PRIVATE KEY-----\\n\",\n  \"client_email\": \"my-sa@my-project-id.iam.gserviceaccount.com\",\n  \"client_id\": \"34598543985498345\",\n  \"auth_uri\": \"https://accounts.google.com/o/oauth2/auth\",\n  \"token_uri\": \"https://accounts.google.com/o/oauth2/token\",\n  \"auth_provider_x509_cert_url\": \"https://www.googleapis.com/oauth2/v1/certs\",\n  \"client_x509_cert_url\": \"https://www.googleapis.com/robot/v1/metadata/x509/my-sa%40my-project-id.iam.gserviceaccount.com\"\n}"
		base64UnencryptedValue := base64.StdEncoding.EncodeToString([]byte(unencryptedValue))
		encryptedTextInEnvelope, err := secretHelper.EncryptEnvelope(base64UnencryptedValue, pipeline)
		assert.Nil(t, err)

		manifest := manifest.EstafetteManifest{
			GlobalEnvVars: map[string]string{
				"MY_SA": encryptedTextInEnvelope,
			},
		}
		credentials := []*contracts.CredentialConfig{}
		credentialsBytes, _ := json.Marshal(credentials)

		obfuscator.CollectSecrets(manifest, credentialsBytes, pipeline)

		unencryptedValueLines := strings.Split(unencryptedValue, "\n")
		assert.Equal(t, 12, len(unencryptedValueLines))
		for _, l := range unencryptedValueLines {
			// act
			output := obfuscator.Obfuscate(fmt.Sprintf("%v", l))
			assert.Equal(t, "***", output)
		}
	})

	t.Run("ObfuscatesBase64EncodedSecretInManifestInLogLineFollowedByNewline", func(t *testing.T) {

		pipeline := "github.com/estafette/estafette-ci-builder"
		unencryptedValue := "{\n  \"type\": \"service_account\",\n  \"project_id\": \"my-project-id\",\n  \"private_key_id\": \"sf8re97843hewrhuifsdidf\",\n  \"private_key\": \"-----BEGIN PRIVATE KEY-----\\n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\\n-----END PRIVATE KEY-----\\n\",\n  \"client_email\": \"my-sa@my-project-id.iam.gserviceaccount.com\",\n  \"client_id\": \"34598543985498345\",\n  \"auth_uri\": \"https://accounts.google.com/o/oauth2/auth\",\n  \"token_uri\": \"https://accounts.google.com/o/oauth2/token\",\n  \"auth_provider_x509_cert_url\": \"https://www.googleapis.com/oauth2/v1/certs\",\n  \"client_x509_cert_url\": \"https://www.googleapis.com/robot/v1/metadata/x509/my-sa%40my-project-id.iam.gserviceaccount.com\"\n}"
		base64UnencryptedValue := base64.StdEncoding.EncodeToString([]byte(unencryptedValue))
		encryptedTextInEnvelope, err := secretHelper.EncryptEnvelope(base64UnencryptedValue, pipeline)
		assert.Nil(t, err)

		manifest := manifest.EstafetteManifest{
			GlobalEnvVars: map[string]string{
				"MY_SA": encryptedTextInEnvelope,
			},
		}
		credentials := []*contracts.CredentialConfig{}
		credentialsBytes, _ := json.Marshal(credentials)

		obfuscator.CollectSecrets(manifest, credentialsBytes, pipeline)

		unencryptedValueLines := strings.Split(strings.ReplaceAll(unencryptedValue, "\\n", "\n"), "\n")
		assert.Equal(t, 50, len(unencryptedValueLines))
		for _, l := range unencryptedValueLines {
			// act
			output := obfuscator.Obfuscate(fmt.Sprintf("%v\n", l))
			assert.Equal(t, "***\n", output)
		}
	})

	t.Run("ObfuscatesMultilineSecretInManifest", func(t *testing.T) {

		pipeline := "github.com/estafette/estafette-ci-builder"
		unencryptedValue := "{\n  \"type\": \"service_account\",\n  \"project_id\": \"my-project-id\",\n  \"private_key_id\": \"sf8re97843hewrhuifsdidf\",\n  \"private_key\": \"-----BEGIN PRIVATE KEY-----\\n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\\n-----END PRIVATE KEY-----\\n\",\n  \"client_email\": \"my-sa@my-project-id.iam.gserviceaccount.com\",\n  \"client_id\": \"34598543985498345\",\n  \"auth_uri\": \"https://accounts.google.com/o/oauth2/auth\",\n  \"token_uri\": \"https://accounts.google.com/o/oauth2/token\",\n  \"auth_provider_x509_cert_url\": \"https://www.googleapis.com/oauth2/v1/certs\",\n  \"client_x509_cert_url\": \"https://www.googleapis.com/robot/v1/metadata/x509/my-sa%40my-project-id.iam.gserviceaccount.com\"\n}"
		encryptedTextInEnvelope, err := secretHelper.EncryptEnvelope(unencryptedValue, pipeline)
		assert.Nil(t, err)

		manifest := manifest.EstafetteManifest{
			GlobalEnvVars: map[string]string{
				"MY_SA": encryptedTextInEnvelope,
			},
		}
		credentials := []*contracts.CredentialConfig{}
		credentialsBytes, _ := json.Marshal(credentials)

		obfuscator.CollectSecrets(manifest, credentialsBytes, pipeline)

		unencryptedValueLines := strings.Split(strings.ReplaceAll(unencryptedValue, "\\n", "\n"), "\n")
		assert.Equal(t, 50, len(unencryptedValueLines))
		for _, l := range unencryptedValueLines {
			// act
			output := obfuscator.Obfuscate(fmt.Sprintf("%v", l))
			assert.Equal(t, "***", output)
		}
	})

	t.Run("ObfuscatesMultilineSecretInManifestWhenEncodedNewlineIsNotOutputedAsNewline", func(t *testing.T) {

		pipeline := "github.com/estafette/estafette-ci-builder"
		unencryptedValue := "{\n  \"type\": \"service_account\",\n  \"project_id\": \"my-project-id\",\n  \"private_key_id\": \"sf8re97843hewrhuifsdidf\",\n  \"private_key\": \"-----BEGIN PRIVATE KEY-----\\n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\\n-----END PRIVATE KEY-----\\n\",\n  \"client_email\": \"my-sa@my-project-id.iam.gserviceaccount.com\",\n  \"client_id\": \"34598543985498345\",\n  \"auth_uri\": \"https://accounts.google.com/o/oauth2/auth\",\n  \"token_uri\": \"https://accounts.google.com/o/oauth2/token\",\n  \"auth_provider_x509_cert_url\": \"https://www.googleapis.com/oauth2/v1/certs\",\n  \"client_x509_cert_url\": \"https://www.googleapis.com/robot/v1/metadata/x509/my-sa%40my-project-id.iam.gserviceaccount.com\"\n}"
		encryptedTextInEnvelope, err := secretHelper.EncryptEnvelope(unencryptedValue, pipeline)
		assert.Nil(t, err)

		manifest := manifest.EstafetteManifest{
			GlobalEnvVars: map[string]string{
				"MY_SA": encryptedTextInEnvelope,
			},
		}
		credentials := []*contracts.CredentialConfig{}
		credentialsBytes, _ := json.Marshal(credentials)

		obfuscator.CollectSecrets(manifest, credentialsBytes, pipeline)

		unencryptedValueLines := strings.Split(unencryptedValue, "\n")
		assert.Equal(t, 12, len(unencryptedValueLines))
		for _, l := range unencryptedValueLines {
			// act
			output := obfuscator.Obfuscate(fmt.Sprintf("%v", l))
			assert.Equal(t, "***", output)
		}
	})

	t.Run("ObfuscatesMultilineSecretInManifestInLogLineFollowedByNewline", func(t *testing.T) {

		pipeline := "github.com/estafette/estafette-ci-builder"
		unencryptedValue := "{\n  \"type\": \"service_account\",\n  \"project_id\": \"my-project-id\",\n  \"private_key_id\": \"sf8re97843hewrhuifsdidf\",\n  \"private_key\": \"-----BEGIN PRIVATE KEY-----\\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAABlwAAAAdzc2gtcn\\nNhAAAAAwEAAQAAAYEArt0UJjY9XbFQi9XCqTpGh5tu5DwRJV4vG47GtpQj4sIDlWOfNnxB\\nh4CdDjVJNdDl+cIrZ24J4C+DtZWRQrlUnSp8wBSv2N5Dt8Wrbsd3sg6rrt0U5YN7Ve5+je\\nxAhPCS1UHTAWz8+NIvfp0WkoZSkvnn2Ic7GqPkOQIXGqJ4FcKtpJzissBdwA0YVZ0BmjOk\\nnibuIlm/XYm14TgZovQ6Q3RsqsQQHGP3CYhV4srsG/m5NGoGLE4+aJK0L/cI36TxLnRBy9\\nRZk9Z01af1Tak/l6xGnlOksC9/3kcuRfwpfTBi5UB0t5QWbQrIqGnvNT9S8VGGfGkw+FjL\\nzFIqZf0+bPEIKU16f1A+CiYGl19xVblUXJyu2VOjVCBggnGf985bEOToEXA1bap/fAOhsT\\nNmokTlg93mSzwOeecEg7hQjHAQCaDaEEfEbDo/P3NXSt+a/SnMVqT+xZlsllrbg58eRjMQ\\n6kXX4r2r8xlx7ZKagTVupJellbucUHssyy9yHhUFAAAFmCUlgdAlJYHQAAAAB3NzaC1yc2\\nEAAAGBAK7dFCY2PV2xUIvVwqk6RoebbuQ8ESVeLxuOxraUI+LCA5VjnzZ8QYeAnQ41STXQ\\n5fnCK2duCeAvg7WVkUK5VJ0qfMAUr9jeQ7fFq27Hd7IOq67dFOWDe1Xufo3sQITwktVB0w\\nFs/PjSL36dFpKGUpL559iHOxqj5DkCFxqieBXCraSc4rLAXcANGFWdAZozpJ4m7iJZv12J\\nteE4GaL0OkN0bKrEEBxj9wmIVeLK7Bv5uTRqBixOPmiStC/3CN+k8S50QcvUWZPWdNWn9U\\n2pP5esRp5TpLAvf95HLkX8KX0wYuVAdLeUFm0KyKhp7zU/UvFRhnxpMPhYy8xSKmX9Pmzx\\nCClNen9QPgomBpdfcVW5VFycrtlTo1QgYIJxn/fOWxDk6BFwNW2qf3wDobEzZqJE5YPd5k\\ns8DnnnBIO4UIxwEAmg2hBHxGw6Pz9zV0rfmv0pzFak/sWZbJZa24OfHkYzEOpF1+K9q/MZ\\nce2SmoE1bqSXpZW7nFB7LMsvch4VBQAAAAMBAAEAAAGBAJUART8aUMgZY20ERM82nQrIY4\\nGPvXx9+N4elyzUpo9+itctAGnJD32LFkkZFr0IuC5OSfXkSf4B/tUoEZMtoPAbWBnEhuLg\\n4gsiIKZQyamr3pcuQ7QeiWX7x1Lf0Up2RGf7ovVADX9oepgE+0r3sj0TPX/AG5jjtoDtSw\\nqjDnhcXuI53OI8EKapgebR1p+zCb7JpXkXyHzH73dt+kpkmZEJD9+jGadXdxVkWurZxr8/\\n15TWE1SFh6BMAcYtVh5byM8abgWkkDTjXpVO1tq0eWzxQE62ZNDq+/XNd+N31q2H3zzQRC\\nsuugHIk/GAHbu+S3FMphp0gI3Z7hoMRTsUKNBKNtHmi78ELMeZ1W4VzqKItk0TvgsaikGs\\nGMpP/b7gIg1WY2hfai1JfPViYCFpgVNwrdl/azBhzBhDk7maO1iezBGISuDYXP2lgtfmnG\\nh+AD4IqYW9nhHC4bBrd8FVTKeyOPE80EIgJ/qtuiJ0MvHcyPgpDcFiI9zz/PMfJl3rAQAA\\nAMEAg3mR8OD0CipKok+431pg6pRfWFlQaXZF/PCg58ZholLXCFsUn7w4kaDZ/WlpnwHJUJ\\nz0D75GURL1dOhAZhstUvSUqd6RMF0KbYkSOznFMCFvdkkVYUA6AXuaUsp+Q+HSvvUPfEjx\\nUuXvlyMaV+CKuq+M2AS6fso+uzanbZChIzwgCR7WQfDnIMPbcpg57QrJ2/ll5nRCYJxGKx\\nHT1QkEtAK/Y7SQpuCTSbLeD4g2iU8St1qN2QjlPjUQLYgJx7k6AAAAwQDVRxjKgb+Mr6ay\\nvXik/5GFU0Vi86KjX5gYYApJ/t7n5NFeYNnPJ28se3AMc0J0Dk0H8o4Lal4ScQAlF6Xr2Q\\n1qU5L8Nvp0T0pT/OIDdtkYOq9YMcO+dEewsrR2abvo+nss7r6futvxanssjL7wx/VWfV4C\\nm1LvX/GuoHLidWtfc+yfvxsnMhcJTS5bFQfqEmCFCfinZEuJlqQyd2L546RAf79YzigHB9\\n3TdUoj4HZwI8Y7L/9fJijm5/WDWRBKonEAAADBANHkGenlDJburcAhDndI7zc/U9uSPBIh\\ndqGk/fMUOIiTe1rB5R6T+vUS55u+Rjv5fBNmA7j99265JZ0Yhj2M2733cMGldvDWZKrRDF\\nY+1E9blLB2YgY9rRuY01mcN4g+ysviIntb5BcIUVSCk+LUkrztNJzVMYhumlrfY0Tj0Iik\\nBuFKFInBUdJUNU1ue1E0UIKJFQahqn2Y4+q7Z3Z5TvHPaF0EKITrk8yyMpsSsi+LxJdPmd\\nfR2LWCJ2xh2FA91QAAAB1qb3JyaXRASm9ycml0cy1pTWFjLmZyaXR6LmJveAECAwQ=\\n-----END PRIVATE KEY-----\\n\",\n  \"client_email\": \"my-sa@my-project-id.iam.gserviceaccount.com\",\n  \"client_id\": \"34598543985498345\",\n  \"auth_uri\": \"https://accounts.google.com/o/oauth2/auth\",\n  \"token_uri\": \"https://accounts.google.com/o/oauth2/token\",\n  \"auth_provider_x509_cert_url\": \"https://www.googleapis.com/oauth2/v1/certs\",\n  \"client_x509_cert_url\": \"https://www.googleapis.com/robot/v1/metadata/x509/my-sa%40my-project-id.iam.gserviceaccount.com\"\n}"
		encryptedTextInEnvelope, err := secretHelper.EncryptEnvelope(unencryptedValue, pipeline)
		assert.Nil(t, err)

		manifest := manifest.EstafetteManifest{
			GlobalEnvVars: map[string]string{
				"MY_SA": encryptedTextInEnvelope,
			},
		}
		credentials := []*contracts.CredentialConfig{}
		credentialsBytes, _ := json.Marshal(credentials)

		obfuscator.CollectSecrets(manifest, credentialsBytes, pipeline)

		unencryptedValueLines := strings.Split(strings.ReplaceAll(unencryptedValue, "\\n", "\n"), "\n")
		assert.Equal(t, 50, len(unencryptedValueLines))
		for _, l := range unencryptedValueLines {
			// act
			output := obfuscator.Obfuscate(fmt.Sprintf("%v\n", l))
			assert.Equal(t, "***\n", output)
		}
	})
}
