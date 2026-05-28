package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/abskrj/velane/services/control-plane/internal/api/middleware"
	"github.com/abskrj/velane/services/control-plane/internal/auth"
	"github.com/abskrj/velane/services/control-plane/internal/scheduler"
	"github.com/abskrj/velane/services/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// InvocationsHandler bundles invocation-related HTTP handlers.
type InvocationsHandler struct {
	store     *postgres.Store
	scheduler *scheduler.Scheduler
	log       *zap.Logger
	provider  auth.Provider // optional; enables session JWT auth on /invoke
}

// NewInvocationsHandler constructs an InvocationsHandler.
func NewInvocationsHandler(store *postgres.Store, sched *scheduler.Scheduler, log *zap.Logger) *InvocationsHandler {
	return &InvocationsHandler{store: store, scheduler: sched, log: log}
}

// WithAuthProvider enables session JWT auth on the Invoke endpoint in addition
// to API key auth. Call this when wiring up the router.
func (h *InvocationsHandler) WithAuthProvider(p auth.Provider) *InvocationsHandler {
	h.provider = p
	return h
}

// invokeBody is the optional JSON body for invoke requests.
type invokeBody struct {
	CallbackURL string `json:"callback_url"`
}

// Invoke handles POST /v1/invoke/{tenantSlug}/{snippetSlug}.
//
// The invoke endpoint performs its own API key authentication inline rather
// than relying on the global auth middleware. This is because the tenant is
// identified via the URL path, allowing external callers to use a key scoped
// to a specific tenant without needing a separate header.
//
// Invoke mode is controlled by the X-Invoke-Mode header (default: sync):
//
//	sync   — execute immediately, return full result (200)
//	async  — enqueue for background execution, return pending info (202)
//	stream — stream execution output as text/event-stream
//
// Response headers (sync/stream):
//
//	X-Invocation-Id  — the persisted invocation ID
//	X-Duration-Ms    — execution wall-clock time in milliseconds (sync only)
func (h *InvocationsHandler) Invoke(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	snippetSlug := chi.URLParam(r, "snippetSlug")

	// --- Inline auth: session JWT first, then API key ---
	authHdr := r.Header.Get("Authorization")
	if authHdr == "" {
		writeError(w, http.StatusUnauthorized, "missing Authorization header")
		return
	}
	parts := strings.SplitN(authHdr, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		writeError(w, http.StatusUnauthorized, "malformed Authorization header")
		return
	}
	token := strings.TrimSpace(parts[1])

	// Resolve the tenant from the URL path (shared by both auth paths).
	tenant, err := h.store.GetTenantBySlug(r.Context(), tenantSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}

	// Try session JWT auth when an auth provider is configured.
	if h.provider != nil {
		if user, err := h.provider.ValidateSession(r.Context(), token); err == nil {
			// Verify the user is a member of this tenant with at least invoke role.
			role, err := h.store.GetMemberRole(r.Context(), tenant.ID, user.ID)
			if err != nil {
				writeError(w, http.StatusForbidden, "not a member of this tenant")
				return
			}
			if role != "invoke" && role != "manage" && role != "admin" {
				writeError(w, http.StatusForbidden, "insufficient role for invoke")
				return
			}
			// Session auth OK — fall through to invocation.
			goto authOK
		}
	}

	// Fall back to API key auth.
	{
		key, err := h.store.ValidateAPIKey(r.Context(), token)
		if err != nil {
			h.log.Debug("invoke: invalid api key", zap.Error(err))
			writeError(w, http.StatusUnauthorized, "invalid api key")
			return
		}
		if !key.HasScope("invoke") {
			writeError(w, http.StatusForbidden, "api key missing 'invoke' scope")
			return
		}
		if key.TenantID != tenant.ID {
			writeError(w, http.StatusForbidden, "api key does not belong to this tenant")
			return
		}
	}

authOK:

	// --- Read the input payload ---
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	// Try to unmarshal as invokeBody to extract optional fields.
	var ib invokeBody
	if len(body) > 0 {
		// Best-effort unmarshal — we'll still use the raw body as input payload.
		_ = json.Unmarshal(body, &ib)
	}

	inputPayload := string(body)
	if inputPayload == "" {
		inputPayload = "{}"
	}

	// Validate that the body is valid JSON (we store it raw).
	if !json.Valid([]byte(inputPayload)) {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON")
		return
	}

	// --- Resolve env from query param, default to prod ---
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "prod"
	}
	if env != "dev" && env != "staging" && env != "prod" {
		writeError(w, http.StatusBadRequest, "env must be 'dev', 'staging', or 'prod'")
		return
	}

	// --- Resolve pinned version from query param ---
	var pinnedVersion int
	if vStr := r.URL.Query().Get("version"); vStr != "" {
		// Accept "v3" or "3".
		vStr = strings.TrimPrefix(vStr, "v")
		n, err := strconv.Atoi(vStr)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "version must be a positive integer, e.g. ?version=3 or ?version=v3")
			return
		}
		pinnedVersion = n
	}

	invokeReq := scheduler.InvokeRequest{
		TenantID:      tenant.ID,
		SnippetSlug:   snippetSlug,
		Env:           env,
		Input:         inputPayload,
		PinnedVersion: pinnedVersion,
	}

	// --- Invoke mode ---
	mode := strings.ToLower(r.Header.Get("X-Invoke-Mode"))
	if mode == "" {
		mode = "sync"
	}

	switch mode {
	case "sync":
		h.invokeSyncMode(w, r, invokeReq)
	case "async":
		h.invokeAsyncMode(w, r, invokeReq, ib.CallbackURL)
	case "stream":
		h.invokeStreamMode(w, r, invokeReq)
	default:
		writeError(w, http.StatusBadRequest, "X-Invoke-Mode must be 'sync', 'async', or 'stream'")
	}
}

func (h *InvocationsHandler) invokeSyncMode(w http.ResponseWriter, r *http.Request, req scheduler.InvokeRequest) {
	invocation, err := h.scheduler.Invoke(r.Context(), req)
	if err != nil {
		h.log.Error("invoke failed", zap.String("slug", req.SnippetSlug), zap.Error(err))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("X-Invocation-Id", invocation.ID)
	w.Header().Set("X-Duration-Ms", fmt.Sprintf("%d", invocation.DurationMs))

	// Parse the output JSON so it embeds naturally in the response object.
	var outputVal any
	if err := json.Unmarshal([]byte(invocation.Output), &outputVal); err != nil {
		outputVal = invocation.Output
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"output":        outputVal,
		"invocation_id": invocation.ID,
		"duration_ms":   invocation.DurationMs,
		"status":        invocation.Status,
		"error":         invocation.Error,
		"stderr":        invocation.Stderr,
	})
}

func (h *InvocationsHandler) invokeAsyncMode(w http.ResponseWriter, r *http.Request, req scheduler.InvokeRequest, callbackURL string) {
	invocation, err := h.scheduler.InvokeAsync(r.Context(), req, callbackURL)
	if err != nil {
		h.log.Error("invoke async failed", zap.String("slug", req.SnippetSlug), zap.Error(err))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"invocation_id": invocation.ID,
		"status":        invocation.Status,
		"status_url":    "/v1/invocations/" + invocation.ID,
	})
}

func (h *InvocationsHandler) invokeStreamMode(w http.ResponseWriter, r *http.Request, req scheduler.InvokeRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported by this server")
		return
	}

	ch, invocation, err := h.scheduler.InvokeStream(r.Context(), req)
	if err != nil {
		h.log.Error("invoke stream failed", zap.String("slug", req.SnippetSlug), zap.Error(err))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-Invocation-Id", invocation.ID)
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for chunk := range ch {
		data, err := json.Marshal(chunk)
		if err != nil {
			h.log.Error("stream: marshal chunk", zap.Error(err))
			continue
		}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		if chunk.Done {
			break
		}
	}

	// Drain any remaining chunks in case we broke early.
	for range ch {
	}

	// Final done event.
	_, _ = fmt.Fprintf(w, "data: {\"done\":true}\n\n")
	flusher.Flush()
}

// GetInvocation handles GET /v1/invocations/{id}.
func (h *InvocationsHandler) GetInvocation(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	id := chi.URLParam(r, "id")
	invocation, err := h.store.GetInvocation(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "invocation not found")
		return
	}

	// Enforce tenant isolation.
	if invocation.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "invocation not found")
		return
	}

	writeJSON(w, http.StatusOK, invocation)
}
