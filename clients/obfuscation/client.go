package obfuscation

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"

	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
)

const maxLengthToSkipObfuscation = 3

// Client hides secret values and other sensitive stuff from the logs
//go:generate mockgen -package=obfuscation -destination ./mock.go -source=client.go
type Client interface {
	CollectSecrets(manifest manifest.EstafetteManifest, credentialsBytes []byte, pipeline string) (err error)
	Obfuscate(input string) string
	ObfuscateSecrets(input string) string
}

// NewClient returns a new Client
func NewClient(secretHelper crypt.SecretHelper) (Client, error) {
	return &client{
		secretHelper: secretHelper,
	}, nil
}

type client struct {
	secretHelper crypt.SecretHelper
	replacer     *strings.Replacer
}

func (ob *client) CollectSecrets(manifest manifest.EstafetteManifest, credentialsBytes []byte, pipeline string) (err error) {

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

	replacerStrings = append(replacerStrings, ob.getReplacerStrings(values)...)

	// collect all secrets from injected credentials
	values, err = ob.secretHelper.GetAllSecretValues(string(credentialsBytes), pipeline)
	if err != nil {
		return err
	}

	replacerStrings = append(replacerStrings, ob.getReplacerStrings(values)...)

	// replace all secret values with obfuscated string
	ob.replacer = strings.NewReplacer(replacerStrings...)

	return nil
}

func (ob *client) getReplacerStrings(values []string) (replacerStrings []string) {

	replacerStrings = []string{}

	for _, v := range values {
		valueLines := strings.Split(v, "\n")
		for _, l := range valueLines {
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

func (ob *client) Obfuscate(input string) string {
	return ob.replacer.Replace(input)
}

func (ob *client) ObfuscateSecrets(input string) string {

	r, err := regexp.Compile(`estafette\.secret\(([a-zA-Z0-9.=_-]+)\)`)
	if err != nil {
		return input
	}

	return r.ReplaceAllString(input, "***")
}
