package auth

import (
	"errors"
	"sync"
)

type AuthService struct {
	mu    sync.RWMutex
	users map[string]string
}

func NewAuthService() *AuthService {
	return &AuthService{
		users: map[string]string{
			"admin": "p@ssword123",
		},
	}
}

func (s *AuthService) Login(username, password string) (string, error) {
	// 🛡️ Aegis-Wilwatikta Architect should catch this security issue:
	// We're comparing passwords in plain text!
	s.mu.RLock()
	storedPass, ok := s.users[username]
	s.mu.RUnlock()

	if !ok || storedPass != password {
		return "", errors.New("invalid credentials")
	}

	// 🛡️ Aegis-Wilwatikta Architect should catch this:
	// Potential race condition if we modify users map without a Lock()
	return "super-secret-token", nil
}
