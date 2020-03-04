package builder

import (
	"encoding/json"
	"regexp"
	"strings"

	contracts "github.com/estafette/estafette-ci-contracts"
	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
)

// Obfuscator hides secret values and other sensitive stuff from the logs
type Obfuscator interface {
	CollectSecrets(manifest manifest.EstafetteManifest, builderConfig contracts.BuilderConfig, pipeline string) (err error)
	Obfuscate(input string) string
	ObfuscateSecrets(input string) string
}

type obfuscatorImpl struct {
	secretHelper crypt.SecretHelper
	replacer     *strings.Replacer
}

// NewObfuscator returns a new Obfuscator
func NewObfuscator(secretHelper crypt.SecretHelper) Obfuscator {
	return &obfuscatorImpl{
		secretHelper: secretHelper,
	}
}

func (ob *obfuscatorImpl) CollectSecrets(manifest manifest.EstafetteManifest, builderConfig contracts.BuilderConfig, pipeline string) (err error) {
	// turn manifest into string to easily scan for secrets
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	r, err := regexp.Compile(`estafette\.secret\(([a-zA-Z0-9.=_-]+)\)`)
	if err != nil {
		return err
	}

	replacerStrings := []string{}

	matches := r.FindAllStringSubmatch(string(manifestBytes), -1)
	if matches != nil {
		for _, m := range matches {
			if len(m) > 1 {
				decryptedValue, _, err := ob.secretHelper.Decrypt(m[1], pipeline)
				if err != nil {
					return err
				}

				replacerStrings = append(replacerStrings, decryptedValue, "***")
			}
		}
	}

	if builderConfig.Credentials != nil && len(builderConfig.Credentials) > 0 {
		credentialsBytes, err := json.Marshal(builderConfig.Credentials)
		if err != nil {
			return err
		}

		matches := r.FindAllStringSubmatch(string(credentialsBytes), -1)
		if matches != nil {
			for _, m := range matches {
				if len(m) > 1 {
					decryptedValue, _, err := ob.secretHelper.Decrypt(m[1], pipeline)
					if err != nil {
						return err
					}

					replacerStrings = append(replacerStrings, decryptedValue, "***")
				}
			}
		}
	}

	ob.replacer = strings.NewReplacer(replacerStrings...)

	return nil
}

func (ob *obfuscatorImpl) Obfuscate(input string) string {
	return ob.replacer.Replace(input)
}

func (ob *obfuscatorImpl) ObfuscateSecrets(input string) string {

	r, err := regexp.Compile(`estafette\.secret\(([a-zA-Z0-9.=_-]+)\)`)
	if err != nil {
		return input
	}

	return r.ReplaceAllString(input, "***")
}
