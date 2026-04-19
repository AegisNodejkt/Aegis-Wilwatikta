package security

import (
	"fmt"
	"strings"
	"testing"
)

func TestNewRedactor(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		r, err := NewRedactor(RedactorConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Mask() != "[REDACTED]" {
			t.Errorf("expected default mask [REDACTED], got %q", r.Mask())
		}
	})

	t.Run("custom mask", func(t *testing.T) {
		r, err := NewRedactor(RedactorConfig{Mask: "***"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Mask() != "***" {
			t.Errorf("expected ***, got %q", r.Mask())
		}
	})

	t.Run("invalid custom pattern returns error", func(t *testing.T) {
		_, err := NewRedactor(RedactorConfig{
			CustomPatterns: []string{"[invalid"},
		})
		if err == nil {
			t.Fatal("expected error for invalid regex")
		}
	})

	t.Run("valid custom pattern", func(t *testing.T) {
		r, err := NewRedactor(RedactorConfig{
			CustomPatterns: []string{`(?i)(custom_secret=)\S+`},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		result := r.Redact("custom_secret=myvalue123")
		if !strings.Contains(result, "[REDACTED]") {
			t.Errorf("expected redacted output, got %q", result)
		}
	})
}

func TestRedactor_Redact(t *testing.T) {
	r, _ := NewRedactor(RedactorConfig{})

	tests := []struct {
		name     string
		input    string
		contains string // result should contain this
		excludes string // result should NOT contain this
	}{
		{
			name:     "token field in JSON",
			input:    `{"token": "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ"}`,
			contains: "[REDACTED]",
			excludes: "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		},
		{
			name:     "api_key field",
			input:    `api_key=sk-1234567890abcdefghijklmnop`,
			contains: "[REDACTED]",
			excludes: "sk-1234567890",
		},
		{
			name:     "Bearer token",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.test.sig",
			contains: "[REDACTED]",
			excludes: "eyJhbGciOiJIUzI1NiJ9",
		},
		{
			name:     "GitHub PAT",
			input:    "github_pat_1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ",
			contains: "[REDACTED]",
			excludes: "1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		},
		{
			name:     "private key",
			input:    "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...\n-----END",
			contains: "[REDACTED]",
			excludes: "MIIEpAIBAAKCAQEA",
		},
		{
			name:     "password field",
			input:    `"password": "supersecretpassword123"`,
			contains: "[REDACTED]",
			excludes: "supersecretpassword123",
		},
		{
			name:     "webhook_secret field",
			input:    `"webhook_secret": "whsec_abcdef123456"`,
			contains: "[REDACTED]",
			excludes: "whsec_abcdef123456",
		},
		{
			name:     "no secrets in input",
			input:    `{"message": "hello world", "count": 42}`,
			contains: "hello world",
			excludes: "[REDACTED]",
		},
		{
			name:     "empty string",
			input:    "",
			contains: "",
			excludes: "REDACTED",
		},
		{
			name:     "Slack token",
			input:    "xoxb-AAAAAAAAAAAAAAAA-AAAAAAAAAAAA-AAAAAAAAAAAAAAAAAAAA",
			contains: "[REDACTED]",
			excludes: "AAAAAAAAAAAAAAAA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)
			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("expected result to contain %q, got %q", tt.contains, result)
			}
			if tt.excludes != "" && strings.Contains(result, tt.excludes) {
				t.Errorf("expected result to NOT contain %q, got %q", tt.excludes, result)
			}
		})
	}
}

func TestRedactor_RedactFields(t *testing.T) {
	r, _ := NewRedactor(RedactorConfig{Mask: "***"})

	t.Run("sensitive keys are redacted", func(t *testing.T) {
		fields := map[string]interface{}{
			"token":    "ghp_secret123",
			"api_key":  "sk-12345",
			"password": "hunter2",
			"safe":     "public_value",
		}
		result := r.RedactFields(fields)

		if result["token"] != "***" {
			t.Errorf("expected token to be ***, got %v", result["token"])
		}
		if result["api_key"] != "***" {
			t.Errorf("expected api_key to be ***, got %v", result["api_key"])
		}
		if result["password"] != "***" {
			t.Errorf("expected password to be ***, got %v", result["password"])
		}
		if result["safe"] != "public_value" {
			t.Errorf("expected safe to be 'public_value', got %v", result["safe"])
		}
	})

	t.Run("original map is not modified", func(t *testing.T) {
		fields := map[string]interface{}{
			"token": "original-value",
		}
		_ = r.RedactFields(fields)
		if fields["token"] != "original-value" {
			t.Error("original map should not be modified")
		}
	})

	t.Run("case-insensitive key matching", func(t *testing.T) {
		fields := map[string]interface{}{
			"Token":        "value1",
			"API_KEY":      "value2",
			"Private_Key":  "value3",
		}
		result := r.RedactFields(fields)
		if result["Token"] != "***" {
			t.Errorf("expected Token to be ***, got %v", result["Token"])
		}
		if result["API_KEY"] != "***" {
			t.Errorf("expected API_KEY to be ***, got %v", result["API_KEY"])
		}
		if result["Private_Key"] != "***" {
			t.Errorf("expected Private_Key to be ***, got %v", result["Private_Key"])
		}
	})

	t.Run("string values containing secrets are redacted inline", func(t *testing.T) {
		fields := map[string]interface{}{
			"config": `{"token": "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ"}`,
		}
		result := r.RedactFields(fields)
		configStr, ok := result["config"].(string)
		if !ok {
			t.Fatal("expected string value")
		}
		if strings.Contains(configStr, "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			t.Errorf("expected inline secret to be redacted, got %q", configStr)
		}
	})

	t.Run("non-string values are preserved", func(t *testing.T) {
		fields := map[string]interface{}{
			"count":   42,
			"enabled": true,
			"data":    []byte("bytes"),
		}
		result := r.RedactFields(fields)
		if result["count"] != 42 {
			t.Errorf("expected count to be 42, got %v", result["count"])
		}
		if result["enabled"] != true {
			t.Errorf("expected enabled to be true, got %v", result["enabled"])
		}
	})

	t.Run("empty fields map", func(t *testing.T) {
		result := r.RedactFields(map[string]interface{}{})
		if len(result) != 0 {
			t.Errorf("expected empty map, got %v", result)
		}
	})
}

func TestRedactor_LogsNeverContainPlaintextSecrets(t *testing.T) {
	r, _ := NewRedactor(RedactorConfig{})

	secrets := []string{
		"ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh",
		"sk-1234567890abcdefghijklmnopqrstuvwxyz",
		"super-secret-token-value-12345",
		"-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA\n-----END",
	}

	for _, secret := range secrets {
		fields := map[string]interface{}{
			"token":   secret,
			"message": "Processing request",
			"config":  fmt.Sprintf(`{"token": "%s"}`, secret),
		}
		result := r.RedactFields(fields)

		// Verify the secret never appears in any result field
		for k, v := range result {
			s, ok := v.(string)
			if !ok {
				continue
			}
			if strings.Contains(s, secret) {
				t.Errorf("plaintext secret found in field %q: %q", k, s)
			}
		}
	}
}
