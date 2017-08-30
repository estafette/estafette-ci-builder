package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncrypt(t *testing.T) {

	t.Run("ReturnsEncryptedValueWithNonceDotEncryptedString", func(t *testing.T) {

		secretHelper := NewSecretHelper("SazbwMf3NZxVVbBqQHebPcXCqrVn3DDp")
		originalText := "this is my secret"

		// act
		encryptedTextPlusNonce, err := secretHelper.Encrypt(originalText)

		assert.Nil(t, err)
		splittedStrings := strings.Split(encryptedTextPlusNonce, ".")
		assert.Equal(t, 2, len(splittedStrings))
		assert.Equal(t, 16, len(splittedStrings[0]))
		fmt.Println(encryptedTextPlusNonce)
	})
}

func TestDecrypt(t *testing.T) {

	t.Run("ReturnsOriginalValue", func(t *testing.T) {

		secretHelper := NewSecretHelper("SazbwMf3NZxVVbBqQHebPcXCqrVn3DDp")
		originalText := "this is my secret"
		encryptedTextPlusNonce := "deFTz5Bdjg6SUe29.oPIkXbze5G9PNEWS2-ZnArl8BCqHnx4MdTdxHg37th9u"

		// act
		decryptedText, err := secretHelper.Decrypt(encryptedTextPlusNonce)

		assert.Nil(t, err)
		assert.Equal(t, originalText, decryptedText)
	})

	t.Run("ReturnsErrorIfStringDoesNotContainDot", func(t *testing.T) {

		secretHelper := NewSecretHelper("SazbwMf3NZxVVbBqQHebPcXCqrVn3DDp")
		encryptedTextPlusNonce := "deFTz5Bdjg6SUe29oPIkXbze5G9PNEWS2-ZnArl8BCqHnx4MdTdxHg37th9u"

		// act
		_, err := secretHelper.Decrypt(encryptedTextPlusNonce)

		assert.NotNil(t, err)
	})
}
