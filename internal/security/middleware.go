package security

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
)

var (
	// ErrMissingServiceToken is returned when the request has no service token.
	ErrMissingServiceToken = errors.New("security: missing service token")
	// ErrInvalidServiceToken is returned when the service token does not match.
	ErrInvalidServiceToken = errors.New("security: invalid service token")
)

// ServiceAuthMiddleware validates the DASHBOARD_SERVICE_TOKEN on internal
// endpoints using constant-time comparison. Requests without a valid token
// are rejected with HTTP 401.
type ServiceAuthMiddleware struct {
	expectedToken string
	headerName    string
	TokenPrefix   string
}

// ServiceAuthConfig configures the service-to-service auth middleware.
type ServiceAuthConfig struct {
	// Token is the expected shared secret for service-to-service auth.
	Token string
	// HeaderName is the header to check for the token (default: "Authorization").
	HeaderName string
	// TokenPrefix is the expected prefix before the token (default: "Bearer ").
	TokenPrefix string
}

// NewServiceAuthMiddleware creates a new service auth middleware.
func NewServiceAuthMiddleware(cfg ServiceAuthConfig) (*ServiceAuthMiddleware, error) {
	if cfg.Token == "" {
		return nil, errors.New("security: service token must not be empty")
	}
	headerName := cfg.HeaderName
	if headerName == "" {
		headerName = "Authorization"
	}
	prefix := cfg.TokenPrefix
	if prefix == "" {
		prefix = "Bearer "
	}
	return &ServiceAuthMiddleware{
		expectedToken: cfg.Token,
		headerName:    headerName,
		TokenPrefix:   prefix,
	}, nil
}

// Middleware returns an http.Handler middleware that validates the service token.
func (m *ServiceAuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := m.extractToken(r)
		if token == "" {
			http.Error(w, ErrMissingServiceToken.Error(), http.StatusUnauthorized)
			return
		}
		// Constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(token), []byte(m.expectedToken)) != 1 {
			http.Error(w, ErrInvalidServiceToken.Error(), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ValidateRequest directly validates the token from an HTTP request.
// Returns nil on success, or an error describing the failure.
func (m *ServiceAuthMiddleware) ValidateRequest(r *http.Request) error {
	token := m.extractToken(r)
	if token == "" {
		return ErrMissingServiceToken
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(m.expectedToken)) != 1 {
		return ErrInvalidServiceToken
	}
	return nil
}

// extractToken retrieves the token from the configured header.
func (m *ServiceAuthMiddleware) extractToken(r *http.Request) string {
	header := r.Header.Get(m.headerName)
	if header == "" {
		return ""
	}
	if m.TokenPrefix != "" {
		if !strings.HasPrefix(header, m.TokenPrefix) {
			return ""
		}
		return strings.TrimPrefix(header, m.TokenPrefix)
	}
	return header
}
