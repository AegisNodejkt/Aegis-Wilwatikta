package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubWebhookHandler_ValidateSignature(t *testing.T) {
	secret := "test-secret"
	handler := NewGitHubWebhookHandler(secret, func(owner, repo string, prNumber int, action string, installationID int64) error {
		return nil
	})

	payload := []byte(`{"action":"opened"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !handler.ValidateSignature(payload, sig) {
		t.Error("expected valid signature to pass validation")
	}

	wrongSig := "sha256=" + hex.EncodeToString([]byte("wrong"))
	if handler.ValidateSignature(payload, wrongSig) {
		t.Error("expected invalid signature to fail validation")
	}

	if handler.ValidateSignature(payload, "invalid-format") {
		t.Error("expected malformed signature to fail validation")
	}

	if handler.ValidateSignature(payload, "") {
		t.Error("expected empty signature to fail validation")
	}
}

func TestGitHubWebhookHandler_Ping(t *testing.T) {
	handler := NewGitHubWebhookHandler("secret", func(owner, repo string, prNumber int, action string, installationID int64) error {
		t.Error("triggerReview should not be called for ping events")
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-GitHub-Event", "ping")

	w := httptest.NewRecorder()
	handler.HandleWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["msg"] != "pong" {
		t.Errorf("expected pong, got %s", resp["msg"])
	}
}

func TestGitHubWebhookHandler_MissingSignature(t *testing.T) {
	handler := NewGitHubWebhookHandler("secret", func(owner, repo string, prNumber int, action string, installationID int64) error {
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-GitHub-Event", "pull_request")

	w := httptest.NewRecorder()
	handler.HandleWebhook(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestGitHubWebhookHandler_ValidPullRequest(t *testing.T) {
	var triggeredOwner, triggeredRepo string
	var triggeredPR int
	var triggeredAction string

	handler := NewGitHubWebhookHandler("secret", func(owner, repo string, prNumber int, action string, installationID int64) error {
		triggeredOwner = owner
		triggeredRepo = repo
		triggeredPR = prNumber
		triggeredAction = action
		return nil
	})

	payload := `{
		"action": "opened",
		"number": 42,
		"repository": {"owner": {"login": "testowner"}, "name": "testrepo"},
		"pull_request": {"number": 42, "title": "Test PR"}
	}`

	body := []byte(payload)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "test-delivery-id")
	req.Header.Set("X-Hub-Signature-256", sig)

	w := httptest.NewRecorder()
	handler.HandleWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if triggeredOwner != "testowner" {
		t.Errorf("expected owner 'testowner', got '%s'", triggeredOwner)
	}
	if triggeredRepo != "testrepo" {
		t.Errorf("expected repo 'testrepo', got '%s'", triggeredRepo)
	}
	if triggeredPR != 42 {
		t.Errorf("expected PR number 42, got %d", triggeredPR)
	}
	if triggeredAction != "opened" {
		t.Errorf("expected action 'opened', got '%s'", triggeredAction)
	}
}

func TestGitHubWebhookHandler_IgnoresClosedAction(t *testing.T) {
	called := false
	handler := NewGitHubWebhookHandler("secret", func(owner, repo string, prNumber int, action string, installationID int64) error {
		called = true
		return nil
	})

	payload := `{
		"action": "closed",
		"number": 42,
		"repository": {"owner": {"login": "testowner"}, "name": "testrepo"},
		"pull_request": {"number": 42}
	}`

	body := []byte(payload)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", sig)

	w := httptest.NewRecorder()
	handler.HandleWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if called {
		t.Error("triggerReview should not be called for closed action")
	}
}
