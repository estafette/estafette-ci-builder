package main

import (
	"regexp"
	"strings"

	"github.com/estafette/estafette-ci-crypt"
	"github.com/estafette/estafette-ci-manifest"
	yaml "gopkg.in/yaml.v2"
)

// Obfuscator hides secret values and other sensitive stuff from the logs
type Obfuscator interface {
	CollectSecrets(manifest.EstafetteManifest) error
	Obfuscate(string) string
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

func (ob *obfuscatorImpl) CollectSecrets(manifest manifest.EstafetteManifest) (err error) {
	// turn manifest into string to easily scan for secrets
	manifestYamlBytes, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}
	manifestYamlString := string(manifestYamlBytes)

	r, err := regexp.Compile(`estafette\.secret\(([a-zA-Z0-9.=_-]+)\)`)
	if err != nil {
		return err
	}

	matches := r.FindAllStringSubmatch(manifestYamlString, 0)
	if matches == nil {
		return nil
	}

	replacerStrings := []string{}
	for _, m := range matches {
		if len(m) > 1 {
			decryptedValue, err := ob.secretHelper.Decrypt(m[1])
			if err != nil {
				return err
			}

			replacerStrings = append(replacerStrings, decryptedValue, "***")
		}
	}

	ob.replacer = strings.NewReplacer(replacerStrings...)

	return nil
}

func (ob *obfuscatorImpl) Obfuscate(input string) string {
	return ob.replacer.Replace(input)
}
