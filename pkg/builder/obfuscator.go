package builder

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"

	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
	"github.com/rs/zerolog/log"
)

const maxLengthToSkipObfuscation = 3

// Obfuscator hides secret values and other sensitive stuff from the logs
type Obfuscator interface {
	CollectSecrets(manifest manifest.EstafetteManifest, credentialsBytes []byte, pipeline string) (err error)
	Obfuscate(input string) string
	ObfuscateSecrets(input string) string
}

type obfuscator struct {
	secretHelper crypt.SecretHelper
	replacer     *strings.Replacer
}

// NewObfuscator returns a new Obfuscator
func NewObfuscator(secretHelper crypt.SecretHelper) Obfuscator {
	return &obfuscator{
		secretHelper: secretHelper,
		replacer:     strings.NewReplacer(),
	}
}

func (ob *obfuscator) CollectSecrets(manifest manifest.EstafetteManifest, credentialsBytes []byte, pipeline string) (err error) {

	log.Debug().Msgf("Collecting secrets and checking if they're valid for pipeline %v...", pipeline)

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

	log.Debug().Msgf("Collected %v manifest secrets for pipeline %v...", len(values), pipeline)

	replacerStrings = append(replacerStrings, ob.getReplacerStrings(values)...)

	// collect all secrets from injected credentials
	values, err = ob.secretHelper.GetAllSecretValues(string(credentialsBytes), pipeline)
	if err != nil {
		return err
	}

	log.Debug().Msgf("Collected %v credentials secrets for pipeline %v...", len(values), pipeline)

	replacerStrings = append(replacerStrings, ob.getReplacerStrings(values)...)

	// replace all secret values with obfuscated string
	ob.replacer = strings.NewReplacer(replacerStrings...)

	return nil
}

func (ob *obfuscator) getReplacerStrings(values []string) (replacerStrings []string) {

	replacerStrings = []string{}

	for _, v := range values {
		valueLines := strings.Split(v, "\n")
		for _, l := range valueLines {
			if len(l) > maxLengthToSkipObfuscation {
				// obfuscate plain secret value
				replacerStrings = append(replacerStrings, l, "***")

				// obfuscate secret value in base64 encoding
				replacerStrings = append(replacerStrings, base64.StdEncoding.EncodeToString([]byte(l)), "***")

				// split further if line contains \n (encoded newline) and obfuscate each line
				valueLineLines := strings.Split(l, "\\n")
				for _, ll := range valueLineLines {
					if len(ll) > maxLengthToSkipObfuscation {
						replacerStrings = append(replacerStrings, ll, "***")
					}
				}
			}
		}

		// if value looks like base64 decode it
		decodedValue, err := base64.StdEncoding.DecodeString(v)
		if err == nil {
			// split decoded value on newlines and add individual lines to replacerStrings
			decodedValueString := string(decodedValue)
			decodedValueLines := strings.Split(decodedValueString, "\n")
			for _, l := range decodedValueLines {
				if len(l) > maxLengthToSkipObfuscation {
					replacerStrings = append(replacerStrings, l, "***")

					// split further if line contains \n (encoded newline)
					valueLineLines := strings.Split(l, "\\n")
					for _, ll := range valueLineLines {
						if len(ll) > maxLengthToSkipObfuscation {
							replacerStrings = append(replacerStrings, ll, "***")
						}
					}
				}
			}
		}
	}

	return replacerStrings
}

func (ob *obfuscator) Obfuscate(input string) string {
	return ob.replacer.Replace(input)
}

func (ob *obfuscator) ObfuscateSecrets(input string) string {

	r, err := regexp.Compile(`estafette\.secret\(([a-zA-Z0-9.=_-]+)\)`)
	if err != nil {
		return input
	}

	return r.ReplaceAllString(input, "***")
}
