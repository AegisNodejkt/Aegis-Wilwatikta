package security

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
)

func TestNewEncryptor(t *testing.T) {
	t.Run("valid 32-byte key", func(t *testing.T) {
		key := make([]byte, 32)
		_, err := NewEncryptor(key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("key too short", func(t *testing.T) {
		_, err := NewEncryptor([]byte("short"))
		if err == nil {
			t.Fatal("expected error for short key")
		}
		if err.Error() != "security: encryption key must be 32 bytes: got 5 bytes" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("key too long", func(t *testing.T) {
		_, err := NewEncryptor(make([]byte, 64))
		if err == nil {
			t.Fatal("expected error for long key")
		}
	})

	t.Run("nil key", func(t *testing.T) {
		_, err := NewEncryptor(nil)
		if err == nil {
			t.Fatal("expected error for nil key")
		}
	})
}

func TestEncryptor_EncryptDecrypt_Roundtrip(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("string round-trip", func(t *testing.T) {
		plaintext := "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh"
		ciphertext, err := enc.EncryptString(plaintext)
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}
		if ciphertext == plaintext {
			t.Error("ciphertext should not equal plaintext")
		}
		decrypted, err := enc.DecryptString(ciphertext)
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		if decrypted != plaintext {
			t.Errorf("round-trip failed: expected %q, got %q", plaintext, decrypted)
		}
	})

	t.Run("bytes round-trip", func(t *testing.T) {
		plaintext := []byte("sensitive-provider-token-12345")
		ciphertext, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}
		decrypted, err := enc.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		if string(decrypted) != string(plaintext) {
			t.Errorf("round-trip failed: expected %q, got %q", plaintext, decrypted)
		}
	})

	t.Run("empty plaintext returns empty", func(t *testing.T) {
		ciphertext, err := enc.Encrypt([]byte{})
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}
		if ciphertext != "" {
			t.Errorf("expected empty ciphertext for empty plaintext, got %q", ciphertext)
		}
	})

	t.Run("empty ciphertext decrypts to nil", func(t *testing.T) {
		plaintext, err := enc.Decrypt("")
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		if plaintext != nil {
			t.Errorf("expected nil for empty ciphertext, got %v", plaintext)
		}
	})

	t.Run("empty string encrypt/decrypt", func(t *testing.T) {
		ciphertext, err := enc.EncryptString("")
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}
		if ciphertext != "" {
			t.Errorf("expected empty for empty string")
		}
		decrypted, err := enc.DecryptString("")
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		if decrypted != "" {
			t.Errorf("expected empty string, got %q", decrypted)
		}
	})
}

func TestEncryptor_TamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, _ := NewEncryptor(key)

	t.Run("tampered ciphertext fails decryption", func(t *testing.T) {
		ciphertext, err := enc.EncryptString("secret-data")
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}

		// Tamper with the ciphertext
		decoded, _ := base64.StdEncoding.DecodeString(ciphertext)
		if len(decoded) > 0 {
			decoded[len(decoded)-1] ^= 0xFF // flip last byte
		}
		tampered := base64.StdEncoding.EncodeToString(decoded)

		_, err = enc.Decrypt(tampered)
		if err != ErrDecryptionFailed {
			t.Errorf("expected ErrDecryptionFailed for tampered ciphertext, got %v", err)
		}
	})

	t.Run("invalid base64 fails decryption", func(t *testing.T) {
		_, err := enc.Decrypt("not-valid-base64!!!")
		if !errors.Is(err, ErrDecryptionFailed) {
			t.Errorf("expected ErrDecryptionFailed for invalid base64, got %v", err)
		}
	})

	t.Run("too-short ciphertext fails", func(t *testing.T) {
		short := base64.StdEncoding.EncodeToString([]byte("short"))
		_, err := enc.Decrypt(short)
		if err != ErrCiphertextTooShort {
			t.Errorf("expected ErrCiphertextTooShort, got %v", err)
		}
	})
}

func TestEncryptor_DifferentKeysProduceDifferentCiphertexts(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)

	enc1, _ := NewEncryptor(key1)
	enc2, _ := NewEncryptor(key2)

	plaintext := "same-plaintext"
	ct1, _ := enc1.EncryptString(plaintext)
	ct2, _ := enc2.EncryptString(plaintext)

	if ct1 == ct2 {
		t.Error("different keys should produce different ciphertexts")
	}
}

func TestEncryptor_WrongKeyFailsDecryption(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)

	enc1, _ := NewEncryptor(key1)
	enc2, _ := NewEncryptor(key2)

	ciphertext, _ := enc1.EncryptString("secret")
	_, err := enc2.Decrypt(ciphertext)
	if err != ErrDecryptionFailed {
		t.Errorf("expected ErrDecryptionFailed with wrong key, got %v", err)
	}
}

func TestEncryptor_NonceUniqueness(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, _ := NewEncryptor(key)

	plaintext := "same-data"
	ct1, _ := enc.EncryptString(plaintext)
	ct2, _ := enc.EncryptString(plaintext)

	// Same plaintext encrypted twice should produce different ciphertexts
	// (different nonces)
	if ct1 == ct2 {
		t.Error("encrypting same data twice should produce different ciphertexts due to random nonce")
	}

	// Both should decrypt correctly
	d1, _ := enc.DecryptString(ct1)
	d2, _ := enc.DecryptString(ct2)
	if d1 != plaintext || d2 != plaintext {
		t.Error("both should decrypt to original plaintext")
	}
}
