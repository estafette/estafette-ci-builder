package builder

import (
	"encoding/json"
	"regexp"
	"strings"

	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
)

// Obfuscator hides secret values and other sensitive stuff from the logs
type Obfuscator interface {
	CollectSecrets(manifest manifest.EstafetteManifest, credentialsBytes []byte, pipeline string) (err error)
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

func (ob *obfuscatorImpl) CollectSecrets(manifest manifest.EstafetteManifest, credentialsBytes []byte, pipeline string) (err error) {

	replacerStrings := []string{}

	// collect all secrets from manifest
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	values, err := ob.secretHelper.GetAllSecretValues(string(manifestBytes), pipeline)
	if err != nil {
		return err
	}
	for _, v := range values {
		replacerStrings = append(replacerStrings, v, "***")
	}

	// collect all secrets from injected credentials
	values, err = ob.secretHelper.GetAllSecretValues(string(credentialsBytes), pipeline)
	if err != nil {
		return err
	}

	for _, v := range values {
		replacerStrings = append(replacerStrings, v, "***")
	}

	// replace all secret values with obfuscated string
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
