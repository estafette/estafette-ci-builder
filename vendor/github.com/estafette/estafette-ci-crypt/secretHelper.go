package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// SecretHelper is the interface for encrypting and decrypting secrets
type SecretHelper interface {
	Encrypt(unencryptedText string) (encryptedTextPlusNonce string, err error)
	Decrypt(encryptedTextPlusNonce string) (decryptedText string, err error)
	EncryptEnvelope(unencryptedText string) (encryptedTextInEnvelope string, err error)
	DecryptEnvelope(encryptedTextInEnvelope string) (decryptedText string, err error)
	DecryptAllEnvelopes(encryptedTextWithEnvelopes string) (decryptedText string, err error)
	ReencryptAllEnvelopes(encryptedTextWithEnvelopes string, base64encodedKey bool) (reencryptedText string, key string, err error)
	GenerateKey(numberOfBytes int, base64encodedKey bool) (key string, err error)
}

type secretHelperImpl struct {
	key              string
	base64encodedKey bool
}

// NewSecretHelper returns a new SecretHelper
func NewSecretHelper(key string, base64encodedKey bool) SecretHelper {

	return &secretHelperImpl{
		key:              key,
		base64encodedKey: base64encodedKey,
	}
}

func (sh *secretHelperImpl) Encrypt(unencryptedText string) (encryptedTextPlusNonce string, err error) {
	return sh.encryptWithKey(unencryptedText, sh.key, sh.base64encodedKey)
}

func (sh *secretHelperImpl) encryptWithKey(unencryptedText, key string, base64encodedKey bool) (encryptedTextPlusNonce string, err error) {

	// The key argument should be the AES key, either 16 or 32 bytes to select AES-128 or AES-256.
	keyBytes := []byte(key)
	if base64encodedKey {
		keyBytes, err = base64.StdEncoding.DecodeString(key)
		if err != nil {
			return
		}
	}
	plaintext := []byte(unencryptedText)

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return
	}

	// Never use more than 2^32 random nonces with a given key because of the risk of a repeat.
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return encryptedTextPlusNonce, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}

	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)

	encryptedTextPlusNonce = fmt.Sprintf("%v.%v", base64.URLEncoding.EncodeToString(nonce), base64.URLEncoding.EncodeToString(ciphertext))

	return
}

func (sh *secretHelperImpl) Decrypt(encryptedTextPlusNonce string) (decryptedText string, err error) {

	splittedStrings := strings.Split(encryptedTextPlusNonce, ".")
	if splittedStrings == nil || len(splittedStrings) != 2 {
		err = errors.New("The encrypted text plus nonce doesn't split correctly")
		return
	}

	usedNonce := splittedStrings[0]
	encryptedText := splittedStrings[1]

	// The key argument should be the AES key, either 16 or 32 bytes to select AES-128 or AES-256.
	key := []byte(sh.key)
	ciphertext, _ := base64.URLEncoding.DecodeString(encryptedText)

	nonce, _ := base64.URLEncoding.DecodeString(usedNonce)

	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return
	}

	decryptedText = string(plaintext)

	return
}

func (sh *secretHelperImpl) EncryptEnvelope(unencryptedText string) (encryptedTextInEnvelope string, err error) {

	return sh.encryptEnvelopeWithKey(unencryptedText, sh.key, sh.base64encodedKey)
}

func (sh *secretHelperImpl) encryptEnvelopeWithKey(unencryptedText, key string, base64encodedKey bool) (encryptedTextInEnvelope string, err error) {

	encryptedText, err := sh.encryptWithKey(unencryptedText, key, base64encodedKey)
	if err != nil {
		return
	}
	encryptedTextInEnvelope = fmt.Sprintf("estafette.secret(%v)", encryptedText)

	return
}

func (sh *secretHelperImpl) DecryptEnvelope(encryptedTextInEnvelope string) (decryptedText string, err error) {

	r, err := regexp.Compile(`^estafette\.secret\(([a-zA-Z0-9.=_-]+)\)$`)
	if err != nil {
		return
	}

	matches := r.FindStringSubmatch(encryptedTextInEnvelope)
	if matches == nil {
		return encryptedTextInEnvelope, nil
	}

	decryptedText, err = sh.Decrypt(matches[1])
	if err != nil {
		return
	}

	return
}

func (sh *secretHelperImpl) decryptEnvelopeInBytes(encryptedTextInEnvelope []byte) []byte {

	decryptedText, err := sh.DecryptEnvelope(string(encryptedTextInEnvelope))
	if err != nil {
		return nil
	}

	return []byte(decryptedText)
}

func (sh *secretHelperImpl) DecryptAllEnvelopes(encryptedTextWithEnvelopes string) (decryptedText string, err error) {

	r, err := regexp.Compile(`estafette\.secret\([a-zA-Z0-9.=_-]+\)`)
	if err != nil {
		return
	}

	decryptedText = string(r.ReplaceAllFunc([]byte(encryptedTextWithEnvelopes), sh.decryptEnvelopeInBytes))

	return
}

func (sh *secretHelperImpl) GenerateKey(numberOfBytes int, base64encodedKey bool) (string, error) {

	key := make([]byte, numberOfBytes)

	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}

	keyString := string(key)
	if base64encodedKey {
		keyString = base64.StdEncoding.EncodeToString(key)
	}

	return keyString, nil
}

func (sh *secretHelperImpl) ReencryptAllEnvelopes(encryptedTextWithEnvelopes string, base64encodedKey bool) (reencryptedText string, key string, err error) {

	// generate 32 bytes key
	key, err = sh.GenerateKey(32, base64encodedKey)
	if err != nil {
		return encryptedTextWithEnvelopes, key, err
	}

	// scan for all secrets and replace them with new secret
	r, err := regexp.Compile(`estafette\.secret\([a-zA-Z0-9.=_-]+\)`)
	if err != nil {
		return
	}

	reencryptedText = string(r.ReplaceAllFunc([]byte(encryptedTextWithEnvelopes), func(encryptedTextInEnvelope []byte) []byte {

		decryptedText, err := sh.DecryptEnvelope(string(encryptedTextInEnvelope))
		if err != nil {
			return nil
		}

		reencryptedTextInEnvelope, err := sh.encryptEnvelopeWithKey(decryptedText, key, base64encodedKey)
		if err != nil {
			return nil
		}

		return []byte(reencryptedTextInEnvelope)
	}))

	return reencryptedText, key, nil
}
