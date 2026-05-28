package handlers_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/abskrj/velane/services/control-plane/internal/api/handlers"
	"github.com/abskrj/velane/services/control-plane/internal/models"
	"go.uber.org/zap"
)

// --- Mock store ---

type mockWebhookStore struct {
	gitIntegration *models.GitIntegration
	gitErr         error
	snippet        *models.Snippet
	snippetErr     error
	createdVersion *models.SnippetVersion
	createErr      error
	publishedVersion *models.SnippetVersion
	publishErr     error
	publishedEnv   string
}

func (m *mockWebhookStore) GetGitIntegrationBySnippetID(_ context.Context, _ string) (*models.GitIntegration, error) {
	return m.gitIntegration, m.gitErr
}

func (m *mockWebhookStore) GetSnippetByID(_ context.Context, _ string) (*models.Snippet, error) {
	return m.snippet, m.snippetErr
}

func (m *mockWebhookStore) CreateVersion(_ context.Context, _, _, _, _, _ string, _, _, _ int) (*models.SnippetVersion, error) {
	return m.createdVersion, m.createErr
}

func (m *mockWebhookStore) PublishVersion(_ context.Context, _ string, env string) (*models.SnippetVersion, error) {
	m.publishedEnv = env
	return m.publishedVersion, m.publishErr
}

// --- Helpers ---

const testSecret = "webhook-secret-abc123"

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func webhookRequest(t *testing.T, snippetID string, payload map[string]any, secret string) *http.Request {
	t.Helper()
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/git/"+snippetID, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sign(b, secret))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("snippetID", snippetID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	return req
}

func defaultStore() *mockWebhookStore {
	return &mockWebhookStore{
		gitIntegration: &models.GitIntegration{
			ID:        "gi-1",
			TenantID:  "t-1",
			SnippetID: "sn-1",
			Provider:  "github",
			RepoURL:   "https://github.com/example/repo",
			Secret:    testSecret,
		},
		snippet: &models.Snippet{
			ID:       "sn-1",
			TenantID: "t-1",
			Name:     "My Snippet",
			Slug:     "my-snippet",
			Language: models.LanguageBun,
		},
		createdVersion: &models.SnippetVersion{
			ID:            "v-1",
			SnippetID:     "sn-1",
			VersionNumber: 1,
			Code:          "// code",
			Status:        models.StatusDraft,
		},
		publishedVersion: &models.SnippetVersion{
			ID:            "v-1",
			SnippetID:     "sn-1",
			VersionNumber: 1,
			Code:          "// code",
			Status:        models.StatusPublished,
		},
	}
}

// --- Tests ---

func TestGitWebhook_InvalidSignature(t *testing.T) {
	store := defaultStore()
	h := handlers.NewWebhookHandler(store, nil, zapNop())

	payload := map[string]any{"ref": "refs/heads/main", "snippet_code": "// code"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/git/sn-1", bytes.NewReader(b))
	req.Header.Set("X-Hub-Signature-256", "sha256=badhash")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("snippetID", "sn-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.GitWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rec.Code)
	}
}

func TestGitWebhook_MainBranchDeploysToStaging(t *testing.T) {
	store := defaultStore()
	h := handlers.NewWebhookHandler(store, nil, zapNop())

	req := webhookRequest(t, "sn-1", map[string]any{
		"ref":          "refs/heads/main",
		"snippet_code": "export default function handler() {}",
	}, testSecret)

	rec := httptest.NewRecorder()
	h.GitWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	if store.publishedEnv != "staging" {
		t.Errorf("deployed to env %q; want staging", store.publishedEnv)
	}

	var body map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["env"] != "staging" {
		t.Errorf("response env = %q; want staging", body["env"])
	}
}

func TestGitWebhook_TagDeploysToProd(t *testing.T) {
	store := defaultStore()
	h := handlers.NewWebhookHandler(store, nil, zapNop())

	req := webhookRequest(t, "sn-1", map[string]any{
		"ref":          "refs/tags/v1.2.3",
		"snippet_code": "export default function handler() {}",
	}, testSecret)

	rec := httptest.NewRecorder()
	h.GitWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	if store.publishedEnv != "prod" {
		t.Errorf("deployed to env %q; want prod", store.publishedEnv)
	}

	var body map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["env"] != "prod" {
		t.Errorf("response env = %q; want prod", body["env"])
	}
}

func TestGitWebhook_FeatureBranchDeploysToDev(t *testing.T) {
	store := defaultStore()
	h := handlers.NewWebhookHandler(store, nil, zapNop())

	req := webhookRequest(t, "sn-1", map[string]any{
		"ref":          "refs/heads/feature/cool-new-thing",
		"snippet_code": "export default function handler() {}",
	}, testSecret)

	rec := httptest.NewRecorder()
	h.GitWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	if store.publishedEnv != "dev" {
		t.Errorf("deployed to env %q; want dev", store.publishedEnv)
	}

	var body map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["env"] != "dev" {
		t.Errorf("response env = %q; want dev", body["env"])
	}
}

func zapNop() *zap.Logger {
	return zap.NewNop()
}
