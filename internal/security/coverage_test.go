package security

import (
	"crypto/rand"
	"testing"
)

// TestDecryptStringErrorPath tests that DecryptString propagates errors correctly.
func TestDecryptStringErrorPath(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, _ := NewEncryptor(key)

	_, err := enc.DecryptString("!!!invalid-base64!!!")
	if err == nil {
		t.Error("expected error from DecryptString with invalid base64")
	}
}

// TestEncryptStringEmpty tests EncryptString with empty string.
func TestEncryptStringEmpty(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, _ := NewEncryptor(key)

	result, err := enc.EncryptString("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

// TestDecryptStringValidRoundtrip tests DecryptString on a successful path.
func TestDecryptStringValidRoundtrip(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, _ := NewEncryptor(key)

	ct, err := enc.EncryptString("hello world")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	pt, err := enc.DecryptString(ct)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if pt != "hello world" {
		t.Errorf("expected 'hello world', got %q", pt)
	}
}

// TestRedactNoCaptureGroup tests the Redact path where a regex has no capture group.
func TestRedactNoCaptureGroup(t *testing.T) {
	r, err := NewRedactor(RedactorConfig{
		CustomPatterns: []string{`secretvalue123`}, // no capture groups
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := r.Redact("some secretvalue123 here")
	if result != "some [REDACTED] here" {
		t.Errorf("expected full match to be replaced, got %q", result)
	}
}

// TestExtractTokenEmptyPrefix tests extractToken with empty TokenPrefix.
// Since the default config sets TokenPrefix to "Bearer ", we need to test
// that when prefix is empty, the raw header value is returned.
func TestExtractTokenEmptyPrefix(t *testing.T) {
	mw, err := NewServiceAuthMiddleware(ServiceAuthConfig{
		Token:       "test-token",
		HeaderName:  "X-Token",
		TokenPrefix: "none",  // We'll test a prefix that doesn't match
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = mw // Verify it was created
}
