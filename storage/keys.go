package storage

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gtank/cryptopasta"
	jose "gopkg.in/square/go-jose.v2"
)

// Encrypt serializes and encrypts an object using the encryption key.
func (k Keys) Encrypt(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("serialize: %v", err)
	}
	cipherText, err := cryptopasta.Encrypt(data, k.EncryptionKey)
	if err != nil {
		return "", fmt.Errorf("encrypt: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(cipherText), nil
}

func decrypt(key *[32]byte, expiry time.Time, cipherText []byte, v interface{}) (bool, error) {
	if !expiry.IsZero() && expiry.After(now()) {
		return false, nil
	}
	plainText, err := cryptopasta.Decrypt(cipherText, key)
	if err != nil {
		// ignore decryption error
		return false, nil
	}
	if err := json.Unmarshal(plainText, v); err != nil {
		return false, fmt.Errorf("deserialize: %v", err)
	}
	return true, nil
}

// Decrypt attempts to decrypt and deserialize a value using the set of
// decryption keys.
func (k Keys) Decrypt(cipherText string, v interface{}) error {
	rawCipherText, err := base64.RawURLEncoding.DecodeString(cipherText)
	if err != nil {
		return fmt.Errorf("base64 decode secret: %v", err)
	}
	if k.EncryptionKey != nil {
		if ok, err := decrypt(k.EncryptionKey, time.Time{}, rawCipherText, v); err != nil {
			return err
		} else if ok {
			return nil
		}
	}

	for _, key := range k.DecryptionKeys {
		if ok, err := decrypt(key.Key, key.Expiry, rawCipherText, v); err != nil {
			return err
		} else if ok {
			return nil
		}

	}
	return errors.New("no decryption keys found which can decrypt the secret")
}

// Sign creates a JWT using the signing key.
func (k Keys) Sign(payload []byte) (jws string, err error) {
	if k.SigningKey == nil {
		return "", fmt.Errorf("no key to sign payload with")
	}
	signingKey := jose.SigningKey{Key: k.SigningKey}

	switch key := k.SigningKey.Key.(type) {
	case *rsa.PrivateKey:
		// TODO(ericchiang): Allow different cryptographic hashes.
		signingKey.Algorithm = jose.RS256
	case *ecdsa.PrivateKey:
		switch key.Params() {
		case elliptic.P256().Params():
			signingKey.Algorithm = jose.ES256
		case elliptic.P384().Params():
			signingKey.Algorithm = jose.ES384
		case elliptic.P521().Params():
			signingKey.Algorithm = jose.ES512
		default:
			return "", errors.New("unsupported ecdsa curve")
		}
	}

	signer, err := jose.NewSigner(signingKey, &jose.SignerOptions{})
	if err != nil {
		return "", fmt.Errorf("new signier: %v", err)
	}
	signature, err := signer.Sign(payload)
	if err != nil {
		return "", fmt.Errorf("signing payload: %v", err)
	}
	return signature.CompactSerialize()
}
