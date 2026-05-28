package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/runeforge/control-plane/internal/api/middleware"
)

// writeJSON serialises v as JSON and writes it with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error body.  Internal error details are NOT included
// to avoid leaking implementation details to callers.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// resolveActor returns the actor ID and type for audit logging.
// Prefers session user (JWT/session auth), falls back to API key.
func resolveActor(r *http.Request) (actorID, actorType string) {
	if u := middleware.SessionUserFromContext(r.Context()); u != nil {
		return u.ID, "user"
	}
	if k := middleware.APIKeyFromContext(r.Context()); k != nil {
		return k.ID, "api_key"
	}
	return "", "api_key"
}

// auditMeta marshals a map into a json.RawMessage for audit log metadata.
// Returns nil if marshalling fails (the DB column defaults to '{}').
func auditMeta(m map[string]any) json.RawMessage {
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return b
}
