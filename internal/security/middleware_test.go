package security

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewServiceAuthMiddleware(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		m, err := NewServiceAuthMiddleware(ServiceAuthConfig{Token: "my-service-token"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m == nil {
			t.Fatal("expected non-nil middleware")
		}
	})

	t.Run("empty token returns error", func(t *testing.T) {
		_, err := NewServiceAuthMiddleware(ServiceAuthConfig{Token: ""})
		if err == nil {
			t.Fatal("expected error for empty token")
		}
	})

	t.Run("custom header config", func(t *testing.T) {
		m, err := NewServiceAuthMiddleware(ServiceAuthConfig{
			Token:      "token",
			HeaderName: "X-Service-Token",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m.headerName != "X-Service-Token" {
			t.Errorf("expected X-Service-Token, got %s", m.headerName)
		}
	})

	t.Run("custom prefix config", func(t *testing.T) {
		m, err := NewServiceAuthMiddleware(ServiceAuthConfig{
			Token:       "token",
			TokenPrefix: "Token ",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m.TokenPrefix != "Token " {
			t.Errorf("expected 'Token ', got %q", m.TokenPrefix)
		}
	})
}

func TestServiceAuthMiddleware_Middleware(t *testing.T) {
	validToken := "valid-service-token-12345"
	mw, _ := NewServiceAuthMiddleware(ServiceAuthConfig{Token: validToken})

	// Create a simple handler that returns 200 OK
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	handler := mw.Middleware(nextHandler)

	t.Run("valid token passes through", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/internal/data", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("missing token returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/internal/data", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
		if body := rec.Body.String(); body == "" {
			t.Error("expected error message in body")
		}
	})

	t.Run("wrong token returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/internal/data", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("token without Bearer prefix returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/internal/data", nil)
		req.Header.Set("Authorization", validToken) // no "Bearer " prefix
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for missing prefix, got %d", rec.Code)
		}
	})

	t.Run("empty Authorization header returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/internal/data", nil)
		req.Header.Set("Authorization", "")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})
}

func TestServiceAuthMiddleware_ValidateRequest(t *testing.T) {
	token := "test-token-xyz"
	mw, _ := NewServiceAuthMiddleware(ServiceAuthConfig{Token: token})

	t.Run("valid request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		if err := mw.ValidateRequest(req); err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("missing header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
		if err := mw.ValidateRequest(req); err != ErrMissingServiceToken {
			t.Errorf("expected ErrMissingServiceToken, got %v", err)
		}
	})

	t.Run("wrong token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		if err := mw.ValidateRequest(req); err != ErrInvalidServiceToken {
			t.Errorf("expected ErrInvalidServiceToken, got %v", err)
		}
	})

	t.Run("case sensitive token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
		req.Header.Set("Authorization", "Bearer "+strings.ToUpper(token))
		if err := mw.ValidateRequest(req); err != ErrInvalidServiceToken {
			t.Errorf("expected ErrInvalidServiceToken for case change, got %v", err)
		}
	})
}

func TestServiceAuthMiddleware_CustomConfig(t *testing.T) {
	t.Run("custom header name", func(t *testing.T) {
		mw, _ := NewServiceAuthMiddleware(ServiceAuthConfig{
			Token:       "my-token",
			HeaderName:  "X-Service-Token",
			TokenPrefix: "Token ",
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Service-Token", "Token my-token")
		if err := mw.ValidateRequest(req); err != nil {
			t.Errorf("expected valid with custom header, got %v", err)
		}
	})

	t.Run("custom token prefix", func(t *testing.T) {
		mw, _ := NewServiceAuthMiddleware(ServiceAuthConfig{
			Token:       "my-token",
			HeaderName:  "X-Auth",
			TokenPrefix: "Token ",
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Auth", "Token my-token")
		if err := mw.ValidateRequest(req); err != nil {
			t.Errorf("expected valid with custom prefix, got %v", err)
		}
	})
}

func TestServiceAuthMiddleware_ConstantTime(t *testing.T) {
	// This test verifies that the middleware uses constant-time comparison.
	// We can't directly test timing, but we verify correct behavior with tokens
	// of different lengths and partial matches.
	token := "exact-32-char-token-aaaaaaaaaaaaa"
	mw, _ := NewServiceAuthMiddleware(ServiceAuthConfig{Token: token})

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"exact match", token, true},
		{"prefix match only", token[:16], false},
		{"suffix match only", token[16:], false},
		{"one char different", token[:30] + "Z" + token[31:], false},
		{"empty string", "", false},
		{"double length", token + token, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer "+tt.input)
			err := mw.ValidateRequest(req)
			if tt.valid && err != nil {
				t.Errorf("expected valid, got %v", err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected invalid for input %q", tt.input)
			}
		})
	}
}
