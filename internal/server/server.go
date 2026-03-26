package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/stackpr/stackpr/internal/db"
	"github.com/stackpr/stackpr/internal/engine"
)

// Server holds dependencies for the webhook HTTP server.
type Server struct {
	eng           *engine.Engine
	webhookSecret string
	repoOwner     string
	repoName      string
	mux           *http.ServeMux
}

// New creates a configured Server.
func New(eng *engine.Engine, webhookSecret, repoOwner, repoName string) *Server {
	s := &Server{
		eng:           eng,
		webhookSecret: webhookSecret,
		repoOwner:     repoOwner,
		repoName:      repoName,
		mux:           http.NewServeMux(),
	}

	s.mux.HandleFunc("POST /webhook", s.handleWebhook)
	s.mux.HandleFunc("GET /healthz", s.handleHealth)

	return s
}

// Handler returns the http.Handler for use with http.ListenAndServe.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// handleHealth is a simple liveness endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleWebhook processes incoming GitHub webhook events.
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Verify HMAC-SHA256 signature.
	if err := s.verifySignature(r.Header.Get("X-Hub-Signature-256"), body); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	if eventType != "pull_request" {
		// We only process pull_request events.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ignored"))
		return
	}

	var payload prWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "failed to parse payload", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	switch payload.Action {
	case "synchronize":
		s.handleSynchronize(ctx, w, payload)
	case "closed":
		if payload.PullRequest.Merged {
			s.handleMerged(ctx, w, payload)
		} else {
			s.handleClosedWithoutMerge(ctx, w, payload)
		}
	default:
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ignored"))
	}
}

func (s *Server) handleSynchronize(ctx context.Context, w http.ResponseWriter, payload prWebhookPayload) {
	prNumber := payload.PullRequest.Number
	owner := payload.Repository.Owner.Login
	repo := payload.Repository.Name

	entry, stack, err := db.GetEntryByPRNumber(ctx, prNumber, owner, repo)
	if err != nil {
		http.Error(w, fmt.Sprintf("db error: %v", err), http.StatusInternalServerError)
		return
	}

	if entry == nil {
		// PR is not part of any tracked stack; nothing to do.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pr not tracked"))
		return
	}

	if err := s.eng.CascadeSync(ctx, stack, entry.Position, prNumber); err != nil {
		http.Error(w, fmt.Sprintf("cascade sync failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("sync triggered"))
}

func (s *Server) handleMerged(ctx context.Context, w http.ResponseWriter, payload prWebhookPayload) {
	prNumber := payload.PullRequest.Number
	owner := payload.Repository.Owner.Login
	repo := payload.Repository.Name
	baseBranch := payload.PullRequest.Base.Ref

	entry, stack, err := db.GetEntryByPRNumber(ctx, prNumber, owner, repo)
	if err != nil {
		http.Error(w, fmt.Sprintf("db error: %v", err), http.StatusInternalServerError)
		return
	}

	if entry == nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pr not tracked"))
		return
	}

	// Mark merged entry.
	if err := db.MarkEntryMerged(ctx, entry.ID); err != nil {
		http.Error(w, fmt.Sprintf("mark merged failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Retarget the child PR's base and cascade sync.
	if err := s.eng.RetargetBase(ctx, stack, entry, baseBranch); err != nil {
		http.Error(w, fmt.Sprintf("retarget base failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("retarget and sync triggered"))
}

func (s *Server) handleClosedWithoutMerge(ctx context.Context, w http.ResponseWriter, payload prWebhookPayload) {
	prNumber := payload.PullRequest.Number
	owner := payload.Repository.Owner.Login
	repo := payload.Repository.Name

	entry, _, err := db.GetEntryByPRNumber(ctx, prNumber, owner, repo)
	if err != nil {
		http.Error(w, fmt.Sprintf("db error: %v", err), http.StatusInternalServerError)
		return
	}

	if entry == nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pr not tracked"))
		return
	}

	if err := s.eng.MarkStackEntryBroken(ctx, entry); err != nil {
		http.Error(w, fmt.Sprintf("mark broken failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("entry marked broken"))
}

// verifySignature checks the X-Hub-Signature-256 header against the body and secret.
func (s *Server) verifySignature(signatureHeader string, body []byte) error {
	if s.webhookSecret == "" {
		// No secret configured; skip verification (useful for local testing).
		return nil
	}

	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return fmt.Errorf("missing or malformed signature header")
	}

	gotHex := strings.TrimPrefix(signatureHeader, prefix)
	gotBytes, err := hex.DecodeString(gotHex)
	if err != nil {
		return fmt.Errorf("failed to decode signature hex: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)

	if !hmac.Equal(expected, gotBytes) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// --- Payload types ---

type prWebhookPayload struct {
	Action      string           `json:"action"`
	PullRequest prPayload        `json:"pull_request"`
	Repository  repoPayload      `json:"repository"`
}

type prPayload struct {
	Number int         `json:"number"`
	Merged bool        `json:"merged"`
	Base   branchPayload `json:"base"`
	Head   branchPayload `json:"head"`
}

type branchPayload struct {
	Ref string `json:"ref"`
}

type repoPayload struct {
	Name  string      `json:"name"`
	Owner ownerPayload `json:"owner"`
}

type ownerPayload struct {
	Login string `json:"login"`
}
