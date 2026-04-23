package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/stackpr/stackpr/internal/db"
	"github.com/stackpr/stackpr/internal/engine"
	ghclient "github.com/stackpr/stackpr/internal/github"
)

// Server holds dependencies for the webhook HTTP server.
type Server struct {
	eng           *engine.Engine
	gh            *ghclient.Client
	webhookSecret string
	mux           *http.ServeMux
}

// New creates a configured Server.
func New(eng *engine.Engine, gh *ghclient.Client, webhookSecret string) *Server {
	s := &Server{
		eng:           eng,
		gh:            gh,
		webhookSecret: webhookSecret,
		mux:           http.NewServeMux(),
	}

	s.mux.HandleFunc("POST /webhook", s.handleWebhook)
	s.mux.HandleFunc("GET /healthz", s.handleHealth)

	s.mux.HandleFunc("GET /api/stacks", s.handleAPIListStacks)
	s.mux.HandleFunc("GET /api/stacks/{stackID}", s.handleAPIGetStack)
	s.mux.HandleFunc("GET /api/stacks/{stackID}/entries", s.handleAPIGetEntries)
	s.mux.HandleFunc("POST /api/stacks/{stackID}/sync", s.handleAPISync)
	s.mux.HandleFunc("GET /api/stacks/{stackID}/events", s.handleAPIGetSyncEvents)

	s.mux.HandleFunc("POST /api/stacks", s.handleAPICreateStack)
	s.mux.HandleFunc("DELETE /api/stacks/{stackID}", s.handleAPIDeleteStack)
	s.mux.HandleFunc("POST /api/stacks/{stackID}/entries", s.handleAPIAddEntry)
	s.mux.HandleFunc("DELETE /api/stacks/{stackID}/entries/{prNumber}", s.handleAPIRemoveEntry)
	s.mux.HandleFunc("POST /api/stacks/{stackID}/entries/{prNumber}/merged", s.handleAPIMarkMerged)

	return s
}

// Handler returns the http.Handler for use with http.ListenAndServe.
func (s *Server) Handler() http.Handler {
	return corsMiddleware(s.mux)
}

// corsMiddleware adds CORS headers to all responses and handles preflight requests.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}

// apiError is the standard JSON error body for API responses.
type apiError struct {
	Error string `json:"error"`
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

// --- API handlers ---

func (s *Server) handleAPIListStacks(w http.ResponseWriter, r *http.Request) {
	stacks, err := db.ListStacks(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	if stacks == nil {
		stacks = []*db.Stack{}
	}
	writeJSON(w, http.StatusOK, stacks)
}

func (s *Server) handleAPIGetStack(w http.ResponseWriter, r *http.Request) {
	stackID := r.PathValue("stackID")
	stack, err := db.GetStackByID(r.Context(), stackID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	if stack == nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "stack not found"})
		return
	}
	writeJSON(w, http.StatusOK, stack)
}

func (s *Server) handleAPIGetEntries(w http.ResponseWriter, r *http.Request) {
	stackID := r.PathValue("stackID")
	entries, err := db.GetAllEntries(r.Context(), stackID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	if entries == nil {
		entries = []*db.StackEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleAPISync(w http.ResponseWriter, r *http.Request) {
	stackID := r.PathValue("stackID")
	stack, err := db.GetStackByID(r.Context(), stackID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	if stack == nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "stack not found"})
		return
	}
	if err := s.eng.CascadeSync(r.Context(), stack, -1, 0); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "sync triggered"})
}

func (s *Server) handleAPIGetSyncEvents(w http.ResponseWriter, r *http.Request) {
	stackID := r.PathValue("stackID")
	events, err := db.ListSyncEvents(r.Context(), stackID, 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	if events == nil {
		events = []*db.SyncEvent{}
	}
	writeJSON(w, http.StatusOK, events)
}

// --- Management API handlers (Phase 2.5) ---

type createStackRequest struct {
	Name      string `json:"name"`
	RepoOwner string `json:"repo_owner"`
	RepoName  string `json:"repo_name"`
}

type addEntryRequest struct {
	PRNumber int `json:"pr_number"`
}

func (s *Server) handleAPICreateStack(w http.ResponseWriter, r *http.Request) {
	var req createStackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if req.Name == "" || req.RepoOwner == "" || req.RepoName == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "name, repo_owner, and repo_name are required"})
		return
	}

	ctx := r.Context()
	existing, err := db.GetStackByName(ctx, req.Name, req.RepoOwner, req.RepoName)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, apiError{Error: "stack already exists"})
		return
	}

	stack, err := db.CreateStack(ctx, req.Name, req.RepoOwner, req.RepoName)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, stack)
}

func (s *Server) handleAPIDeleteStack(w http.ResponseWriter, r *http.Request) {
	stackID := r.PathValue("stackID")
	ctx := r.Context()

	stack, err := db.GetStackByID(ctx, stackID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	if stack == nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "stack not found"})
		return
	}

	if err := db.DeleteStack(ctx, stackID); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAPIAddEntry(w http.ResponseWriter, r *http.Request) {
	stackID := r.PathValue("stackID")
	ctx := r.Context()

	var req addEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if req.PRNumber <= 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "pr_number must be a positive integer"})
		return
	}

	stack, err := db.GetStackByID(ctx, stackID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	if stack == nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "stack not found"})
		return
	}

	pr, err := s.gh.GetPR(ctx, stack.RepoOwner, stack.RepoName, req.PRNumber)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: fmt.Sprintf("failed to fetch PR #%d from GitHub: %v", req.PRNumber, err)})
		return
	}
	branchName := pr.GetHead().GetRef()
	if branchName == "" {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: fmt.Sprintf("could not determine head branch for PR #%d", req.PRNumber)})
		return
	}

	maxPos, err := db.GetMaxPosition(ctx, stack.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}

	entry, err := db.AddStackEntry(ctx, stack.ID, req.PRNumber, branchName, maxPos+1)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleAPIRemoveEntry(w http.ResponseWriter, r *http.Request) {
	stackID := r.PathValue("stackID")
	prNumberStr := r.PathValue("prNumber")
	ctx := r.Context()

	prNumber, err := strconv.Atoi(prNumberStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid PR number"})
		return
	}

	stack, err := db.GetStackByID(ctx, stackID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	if stack == nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "stack not found"})
		return
	}

	if err := db.RemoveStackEntry(ctx, stack.ID, prNumber); err != nil {
		if errors.Is(err, db.ErrEntryNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Error: "PR not in this stack"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAPIMarkMerged(w http.ResponseWriter, r *http.Request) {
	stackID := r.PathValue("stackID")
	prNumberStr := r.PathValue("prNumber")
	ctx := r.Context()

	prNumber, err := strconv.Atoi(prNumberStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid PR number"})
		return
	}

	stack, err := db.GetStackByID(ctx, stackID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	if stack == nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "stack not found"})
		return
	}

	entries, err := db.GetAllEntries(ctx, stack.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}

	var mergedEntry *db.StackEntry
	for _, e := range entries {
		if e.PRNumber == prNumber {
			mergedEntry = e
			break
		}
	}
	if mergedEntry == nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: fmt.Sprintf("PR #%d is not in this stack", prNumber)})
		return
	}

	pr, err := s.gh.GetPR(ctx, stack.RepoOwner, stack.RepoName, prNumber)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: fmt.Sprintf("failed to fetch PR #%d from GitHub: %v", prNumber, err)})
		return
	}
	baseBranch := pr.GetBase().GetRef()

	if err := db.MarkEntryMerged(ctx, mergedEntry.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}

	if err := s.eng.RetargetBase(ctx, stack, mergedEntry, baseBranch); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "retarget and sync triggered"})
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
