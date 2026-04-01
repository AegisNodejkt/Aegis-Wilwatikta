package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

const (
	headerGitHubEvent    = "X-GitHub-Event"
	headerGitHubDelivery = "X-GitHub-Delivery"
	headerGitHubSig256   = "X-Hub-Signature-256"
)

type GitHubWebhookHandler struct {
	secret        []byte
	triggerReview func(owner, repo string, prNumber int, action string, installationID int64) error
}

type GitHubWebhookPayload struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		State   string `json:"state"`
		HTMLURL string `json:"html_url"`
		Head    struct {
			SHA string `json:"sha"`
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref  string `json:"ref"`
			SHA  string `json:"sha"`
			Repo struct {
				Owner struct {
					Login string `json:"login"`
				} `json:"owner"`
				Name string `json:"name"`
			} `json:"repo"`
		} `json:"base"`
	} `json:"pull_request"`
	Repository struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	} `json:"repository"`
	Installation *struct {
		ID int64 `json:"id"`
	} `json:"installation"`
}

func NewGitHubWebhookHandler(secret string, triggerReview func(owner, repo string, prNumber int, action string, installationID int64) error) *GitHubWebhookHandler {
	return &GitHubWebhookHandler{
		secret:        []byte(secret),
		triggerReview: triggerReview,
	}
}

func (h *GitHubWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(headerGitHubEvent) == "ping" {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"msg":"pong"}`)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get(headerGitHubSig256)
	if sig == "" {
		http.Error(w, "missing signature", http.StatusUnauthorized)
		return
	}

	if !h.validateSignature(body, sig) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	event := r.Header.Get(headerGitHubEvent)
	deliveryID := r.Header.Get(headerGitHubDelivery)

	switch event {
	case "pull_request":
		h.handlePullRequest(w, body, deliveryID)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (h *GitHubWebhookHandler) validateSignature(payload []byte, signature string) bool {
	if len(signature) < 7 || signature[:7] != "sha256=" {
		return false
	}
	sig := signature[7:]
	mac := hmac.New(sha256.New, h.secret)
	mac.Write(payload)
	expected := mac.Sum(nil)
	actual, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	return hmac.Equal(actual, expected)
}

func (h *GitHubWebhookHandler) handlePullRequest(w http.ResponseWriter, body []byte, deliveryID string) {
	var payload GitHubWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	owner := payload.Repository.Owner.Login
	repo := payload.Repository.Name
	prNumber := payload.PullRequest.Number
	action := payload.Action

	validActions := map[string]bool{
		"opened":      true,
		"synchronize": true,
		"reopened":    true,
	}

	if !validActions[action] {
		w.WriteHeader(http.StatusOK)
		return
	}

	var installationID int64
	if payload.Installation != nil {
		installationID = payload.Installation.ID
	}

	if err := h.triggerReview(owner, repo, prNumber, action, installationID); err != nil {
		http.Error(w, fmt.Sprintf("failed to trigger review: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"queued","delivery_id":"%s"}`, deliveryID)
}

func (h *GitHubWebhookHandler) ValidateSignature(payload []byte, signature string) bool {
	return h.validateSignature(payload, signature)
}

func GitHubWebhookRouter(secret string, triggerReview func(owner, repo string, prNumber int, action string, installationID int64) error) func(r chi.Router) {
	handler := NewGitHubWebhookHandler(secret, triggerReview)
	return func(r chi.Router) {
		r.Post("/", handler.HandleWebhook)
	}
}
