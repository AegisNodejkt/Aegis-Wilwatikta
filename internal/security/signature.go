package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var (
	// ErrEmptySignature is returned when the provided signature is empty.
	ErrEmptySignature = errors.New("security: empty signature")
	// ErrEmptySecret is returned when the signing secret is empty.
	ErrEmptySecret = errors.New("security: empty secret")
	// ErrInvalidSignatureFormat is returned when the signature cannot be decoded.
	ErrInvalidSignatureFormat = errors.New("security: invalid signature format")
	// ErrSignatureMismatch is returned when signatures do not match.
	ErrSignatureMismatch = errors.New("security: signature mismatch")
)

// SignatureVerifier validates HMAC-SHA256 signatures, typically used for
// webhook payload verification from providers like GitHub.
type SignatureVerifier struct {
	secret     []byte
	headerName string
	prefix     string
}

// SignatureVerifierConfig holds configuration for creating a new SignatureVerifier.
type SignatureVerifierConfig struct {
	// Secret is the HMAC key used for signing.
	Secret string
	// HeaderName is the HTTP header containing the signature (default: "X-Hub-Signature-256").
	HeaderName string
	// Prefix is the expected prefix before the hex-encoded signature (default: "sha256=").
	Prefix string
}

// NewSignatureVerifier creates a new HMAC-SHA256 signature verifier.
func NewSignatureVerifier(cfg SignatureVerifierConfig) (*SignatureVerifier, error) {
	if cfg.Secret == "" {
		return nil, ErrEmptySecret
	}
	headerName := cfg.HeaderName
	if headerName == "" {
		headerName = "X-Hub-Signature-256"
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "sha256="
	}
	return &SignatureVerifier{
		secret:     []byte(cfg.Secret),
		headerName: headerName,
		prefix:     prefix,
	}, nil
}

// Verify checks that the payload was signed with the configured secret.
// The signature should be the hex-encoded HMAC digest, optionally prefixed
// (e.g. "sha256=abcdef...").
// It uses constant-time comparison to prevent timing attacks.
func (v *SignatureVerifier) Verify(payload []byte, signature string) error {
	if len(payload) == 0 && signature == "" {
		return ErrEmptySignature
	}
	if signature == "" {
		return ErrEmptySignature
	}

	// Strip prefix if present
	sig := signature
	if v.prefix != "" {
		sig = strings.TrimPrefix(signature, v.prefix)
	}

	// Decode the hex-encoded signature
	decoded, err := hex.DecodeString(sig)
	if err != nil {
		return ErrInvalidSignatureFormat
	}

	// Compute expected HMAC
	mac := hmac.New(sha256.New, v.secret)
	mac.Write(payload)
	expected := mac.Sum(nil)

	// Constant-time comparison
	if !hmac.Equal(decoded, expected) {
		return ErrSignatureMismatch
	}

	return nil
}

// VerifyWithHeader extracts the signature from the given HTTP-style header map
// and verifies the payload. The header map keys are looked up case-insensitively.
func (v *SignatureVerifier) VerifyWithHeader(payload []byte, headers map[string][]string) error {
	sig := getHeader(headers, v.headerName)
	if sig == "" {
		return ErrEmptySignature
	}
	return v.Verify(payload, sig)
}

// HeaderName returns the configured header name for signature extraction.
func (v *SignatureVerifier) HeaderName() string {
	return v.headerName
}

// ComputeSignature computes the HMAC-SHA256 signature for the given payload.
// This is primarily useful in tests.
func ComputeSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// getHeader retrieves a header value case-insensitively from a standard
// http.Header-style map.
func getHeader(headers map[string][]string, key string) string {
	// Try exact match first
	if vals, ok := headers[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	// Case-insensitive fallback
	lower := strings.ToLower(key)
	for k, vals := range headers {
		if strings.ToLower(k) == lower && len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}
