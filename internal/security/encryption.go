package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	// ErrInvalidKeySize is returned when the encryption key is not 32 bytes.
	ErrInvalidKeySize = errors.New("security: encryption key must be 32 bytes")
	// ErrCiphertextTooShort is returned when the ciphertext is too short to contain a nonce.
	ErrCiphertextTooShort = errors.New("security: ciphertext too short")
	// ErrDecryptionFailed is returned when decryption fails (tampered data, wrong key).
	ErrDecryptionFailed = errors.New("security: decryption failed")
)

// Encryptor provides AES-256-GCM encryption and decryption for sensitive data
// such as provider tokens stored at-rest.
type Encryptor struct {
	aead cipher.AEAD
}

// NewEncryptor creates a new AES-256-GCM encryptor from a 32-byte key.
// The key should be sourced from an environment variable or secret manager.
func NewEncryptor(key []byte) (*Encryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("%w: got %d bytes", ErrInvalidKeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("security: creating AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("security: creating GCM: %w", err)
	}
	return &Encryptor{aead: aead}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns a base64-encoded
// string containing the nonce prepended to the ciphertext.
func (e *Encryptor) Encrypt(plaintext []byte) (string, error) {
	if len(plaintext) == 0 {
		return "", nil
	}

	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("security: generating nonce: %w", err)
	}

	// nonce is prepended to the ciphertext
	ciphertext := e.aead.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decodes a base64-encoded string and decrypts it using AES-256-GCM.
// Returns the original plaintext.
func (e *Encryptor) Decrypt(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, nil
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode: %v", ErrDecryptionFailed, err)
	}

	nonceSize := e.aead.NonceSize()
	if len(data) < nonceSize {
		return nil, ErrCiphertextTooShort
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := e.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptString is a convenience wrapper for Encrypt that accepts and returns strings.
func (e *Encryptor) EncryptString(plaintext string) (string, error) {
	return e.Encrypt([]byte(plaintext))
}

// DecryptString is a convenience wrapper for Decrypt that returns a string.
func (e *Encryptor) DecryptString(encoded string) (string, error) {
	plaintext, err := e.Decrypt(encoded)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
