package security

import (
	"encoding/hex"
	"testing"
)

func TestNewSignatureVerifier(t *testing.T) {
	t.Run("valid config with defaults", func(t *testing.T) {
		v, err := NewSignatureVerifier(SignatureVerifierConfig{Secret: "my-secret"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.HeaderName() != "X-Hub-Signature-256" {
			t.Errorf("expected default header X-Hub-Signature-256, got %s", v.HeaderName())
		}
	})

	t.Run("empty secret returns error", func(t *testing.T) {
		_, err := NewSignatureVerifier(SignatureVerifierConfig{Secret: ""})
		if err != ErrEmptySecret {
			t.Errorf("expected ErrEmptySecret, got %v", err)
		}
	})

	t.Run("custom header name", func(t *testing.T) {
		v, err := NewSignatureVerifier(SignatureVerifierConfig{
			Secret:     "secret",
			HeaderName: "X-Signature",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.HeaderName() != "X-Signature" {
			t.Errorf("expected X-Signature, got %s", v.HeaderName())
		}
	})
}

func TestSignatureVerifier_Verify(t *testing.T) {
	secret := "test-webhook-secret"
	v, err := NewSignatureVerifier(SignatureVerifierConfig{Secret: secret})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("valid signature", func(t *testing.T) {
		payload := []byte(`{"action":"push","ref":"refs/heads/main"}`)
		sig := "sha256=" + ComputeSignature(secret, payload)
		if err := v.Verify(payload, sig); err != nil {
			t.Errorf("expected valid signature, got error: %v", err)
		}
	})

	t.Run("valid signature without prefix", func(t *testing.T) {
		payload := []byte(`{"action":"push"}`)
		sig := ComputeSignature(secret, payload)
		if err := v.Verify(payload, sig); err != nil {
			t.Errorf("expected valid signature without prefix, got error: %v", err)
		}
	})

	t.Run("empty signature returns error", func(t *testing.T) {
		err := v.Verify([]byte("payload"), "")
		if err != ErrEmptySignature {
			t.Errorf("expected ErrEmptySignature, got %v", err)
		}
	})

	t.Run("empty payload and empty signature returns error", func(t *testing.T) {
		err := v.Verify([]byte{}, "")
		if err != ErrEmptySignature {
			t.Errorf("expected ErrEmptySignature, got %v", err)
		}
	})

	t.Run("tampered payload returns mismatch", func(t *testing.T) {
		payload := []byte(`{"action":"push"}`)
		sig := "sha256=" + ComputeSignature(secret, payload)
		err := v.Verify([]byte(`{"action":"push","extra":"tampered"}`), sig)
		if err != ErrSignatureMismatch {
			t.Errorf("expected ErrSignatureMismatch for tampered payload, got %v", err)
		}
	})

	t.Run("wrong secret produces mismatch", func(t *testing.T) {
		payload := []byte(`{"action":"push"}`)
		sig := ComputeSignature("wrong-secret", payload)
		err := v.Verify(payload, sig)
		if err != ErrSignatureMismatch {
			t.Errorf("expected ErrSignatureMismatch for wrong secret, got %v", err)
		}
	})

	t.Run("invalid hex signature returns format error", func(t *testing.T) {
		err := v.Verify([]byte("payload"), "not-hex-at-all")
		if err != ErrInvalidSignatureFormat {
			t.Errorf("expected ErrInvalidSignatureFormat, got %v", err)
		}
	})

	t.Run("empty payload with valid signature still works", func(t *testing.T) {
		payload := []byte{}
		sig := "sha256=" + ComputeSignature(secret, payload)
		if err := v.Verify(payload, sig); err != nil {
			t.Errorf("expected valid for empty payload, got error: %v", err)
		}
	})

	t.Run("signature with different prefix is stripped", func(t *testing.T) {
		v2, _ := NewSignatureVerifier(SignatureVerifierConfig{
			Secret: secret,
			Prefix: "sha256=",
		})
		payload := []byte("test")
		sig := "sha256=" + ComputeSignature(secret, payload)
		if err := v2.Verify(payload, sig); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})
}

func TestSignatureVerifier_VerifyWithHeader(t *testing.T) {
	secret := "header-test-secret"
	v, _ := NewSignatureVerifier(SignatureVerifierConfig{Secret: secret})

	t.Run("valid header", func(t *testing.T) {
		payload := []byte(`{"test":true}`)
		sig := "sha256=" + ComputeSignature(secret, payload)
		headers := map[string][]string{
			"X-Hub-Signature-256": {sig},
		}
		if err := v.VerifyWithHeader(payload, headers); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("missing header returns error", func(t *testing.T) {
		headers := map[string][]string{}
		err := v.VerifyWithHeader([]byte("payload"), headers)
		if err != ErrEmptySignature {
			t.Errorf("expected ErrEmptySignature, got %v", err)
		}
	})

	t.Run("case-insensitive header lookup", func(t *testing.T) {
		payload := []byte(`case-test`)
		sig := "sha256=" + ComputeSignature(secret, payload)
		headers := map[string][]string{
			"x-hub-signature-256": {sig},
		}
		if err := v.VerifyWithHeader(payload, headers); err != nil {
			t.Errorf("expected valid with case-insensitive lookup, got: %v", err)
		}
	})
}

func TestComputeSignature(t *testing.T) {
	t.Run("produces correct length hex string", func(t *testing.T) {
		sig := ComputeSignature("secret", []byte("payload"))
		decoded, err := hex.DecodeString(sig)
		if err != nil {
			t.Fatalf("signature is not valid hex: %v", err)
		}
		if len(decoded) != 32 { // SHA-256 = 32 bytes
			t.Errorf("expected 32 bytes, got %d", len(decoded))
		}
	})

	t.Run("deterministic output", func(t *testing.T) {
		sig1 := ComputeSignature("secret", []byte("payload"))
		sig2 := ComputeSignature("secret", []byte("payload"))
		if sig1 != sig2 {
			t.Errorf("same inputs produced different outputs: %s vs %s", sig1, sig2)
		}
	})

	t.Run("different inputs produce different outputs", func(t *testing.T) {
		sig1 := ComputeSignature("secret", []byte("payload1"))
		sig2 := ComputeSignature("secret", []byte("payload2"))
		if sig1 == sig2 {
			t.Error("different inputs should produce different signatures")
		}
	})
}

func TestGetHeader(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		headers := map[string][]string{
			"X-Test": {"value"},
		}
		if got := getHeader(headers, "X-Test"); got != "value" {
			t.Errorf("expected 'value', got '%s'", got)
		}
	})

	t.Run("case-insensitive match", func(t *testing.T) {
		headers := map[string][]string{
			"x-test": {"value"},
		}
		if got := getHeader(headers, "X-TEST"); got != "value" {
			t.Errorf("expected 'value' with case-insensitive match, got '%s'", got)
		}
	})

	t.Run("missing key returns empty", func(t *testing.T) {
		headers := map[string][]string{
			"Other": {"val"},
		}
		if got := getHeader(headers, "X-Missing"); got != "" {
			t.Errorf("expected empty for missing key, got '%s'", got)
		}
	})

	t.Run("empty values returns empty", func(t *testing.T) {
		headers := map[string][]string{
			"X-Empty": {},
		}
		if got := getHeader(headers, "X-Empty"); got != "" {
			t.Errorf("expected empty for empty values, got '%s'", got)
		}
	})
}
