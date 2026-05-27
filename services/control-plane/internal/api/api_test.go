// Package api_test contains end-to-end HTTP tests for the full chi router.
// Tests connect to a real Postgres instance (TEST_DATABASE_URL) and use a
// mock executor httptest server — no external executor container is needed.
package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	api "github.com/runeforge/control-plane/internal/api"
	"github.com/runeforge/control-plane/internal/executor/remote"
	"github.com/runeforge/control-plane/internal/models"
	"github.com/runeforge/control-plane/internal/scheduler"
	"github.com/runeforge/control-plane/internal/store/postgres"
	redisstore "github.com/runeforge/control-plane/internal/store/redis"
	"go.uber.org/zap"
)

// --- Test harness ---

type testEnv struct {
	router    http.Handler
	store     *postgres.Store
	tenant    *models.Tenant
	manageKey string // plain key with manage+invoke+admin scopes
	invokeKey string // plain key with invoke scope only
}

// setup wires the full stack against a real test Postgres and a mock executor.
func setup(t *testing.T) *testEnv {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test postgres: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Mock executor: always succeeds with {"ok":true}.
	mockExec := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"output":"{\"ok\":true}","stderr":"","duration_ms":10,"peak_memory_mb":8,"exit_code":0,"error":""}`))
	}))
	t.Cleanup(mockExec.Close)

	exec := remote.New(mockExec.URL, mockExec.URL)
	sched := scheduler.New(store, exec)
	log := zap.NewNop()
	router := api.NewRouter(store, sched, log)

	// Bootstrap tenant.
	slug := fmt.Sprintf("test-%d", time.Now().UnixNano())
	tenant, err := store.CreateTenant(context.Background(), "Test Tenant", slug)
	if err != nil {
		t.Fatalf("create test tenant: %v", err)
	}

	// Admin/manage key.
	_, manageKey, err := store.CreateAPIKeyWithPlain(context.Background(), tenant.ID, "manage-key",
		[]string{"admin", "manage", "invoke"})
	if err != nil {
		t.Fatalf("create manage key: %v", err)
	}

	// Invoke-only key.
	_, invokeKey, err := store.CreateAPIKeyWithPlain(context.Background(), tenant.ID, "invoke-key",
		[]string{"invoke"})
	if err != nil {
		t.Fatalf("create invoke key: %v", err)
	}

	return &testEnv{
		router:    router,
		store:     store,
		tenant:    tenant,
		manageKey: manageKey,
		invokeKey: invokeKey,
	}
}

func (e *testEnv) do(t *testing.T, method, path, key string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatalf("decode JSON body (status %d): %v\nbody: %s", rec.Code, err, rec.Body.String())
	}
	return m
}

// --- Health ---

func TestHealthz(t *testing.T) {
	env := setup(t)
	rec := env.do(t, http.MethodGet, "/healthz", "", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rec.Code)
	}
}

// --- Tenants ---

func TestCreateTenant(t *testing.T) {
	env := setup(t)
	slug := fmt.Sprintf("new-tenant-%d", time.Now().UnixNano())
	rec := env.do(t, http.MethodPost, "/v1/tenants", "", map[string]any{
		"name": "New Tenant",
		"slug": slug,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d; want 201\nbody: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec)
	if body["slug"] != slug {
		t.Errorf("slug = %q; want %q", body["slug"], slug)
	}
}

func TestCreateAPIKey(t *testing.T) {
	env := setup(t)
	path := fmt.Sprintf("/v1/tenants/%s/api-keys", env.tenant.Slug)
	rec := env.do(t, http.MethodPost, path, env.manageKey, map[string]any{
		"name":   "new-key",
		"scopes": []string{"invoke"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d; want 201\nbody: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec)
	if _, ok := body["plain_key"]; !ok {
		t.Error("response must include plain_key field")
	}
}

func TestCreateAPIKey_RequiresAdminScope(t *testing.T) {
	env := setup(t)
	path := fmt.Sprintf("/v1/tenants/%s/api-keys", env.tenant.Slug)
	rec := env.do(t, http.MethodPost, path, env.invokeKey, map[string]any{
		"name":   "bad-key",
		"scopes": []string{"invoke"},
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 403", rec.Code)
	}
}

// --- Snippets ---

func TestCreateSnippet(t *testing.T) {
	env := setup(t)
	slug := fmt.Sprintf("my-snippet-%d", time.Now().UnixNano())
	rec := env.do(t, http.MethodPost, "/v1/snippets", env.manageKey, map[string]any{
		"name":     "My Snippet",
		"slug":     slug,
		"language": "bun",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d; want 201\nbody: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec)
	if body["slug"] != slug {
		t.Errorf("slug = %q; want %q", body["slug"], slug)
	}
}

func TestCreateSnippet_RequiresManageScope(t *testing.T) {
	env := setup(t)
	rec := env.do(t, http.MethodPost, "/v1/snippets", env.invokeKey, map[string]any{
		"name": "Bad", "slug": "bad", "language": "bun",
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 403", rec.Code)
	}
}

func TestListSnippets(t *testing.T) {
	env := setup(t)

	// Create two snippets.
	for i := 0; i < 2; i++ {
		env.do(t, http.MethodPost, "/v1/snippets", env.manageKey, map[string]any{
			"name": "Snippet", "slug": fmt.Sprintf("list-sn-%d-%d", time.Now().UnixNano(), i), "language": "bun",
		})
	}

	rec := env.do(t, http.MethodGet, "/v1/snippets", env.manageKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}

	var snippets []any
	if err := json.NewDecoder(rec.Body).Decode(&snippets); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(snippets) < 2 {
		t.Errorf("expected at least 2 snippets, got %d", len(snippets))
	}
}

func TestGetSnippet_TenantIsolation(t *testing.T) {
	env := setup(t)

	// Create a snippet under env's tenant.
	slug := fmt.Sprintf("isolation-sn-%d", time.Now().UnixNano())
	rec := env.do(t, http.MethodPost, "/v1/snippets", env.manageKey, map[string]any{
		"name": "Isolation", "slug": slug, "language": "bun",
	})
	body := decodeJSON(t, rec)
	snippetID, _ := body["id"].(string)

	// Second tenant tries to read it.
	slug2 := fmt.Sprintf("tenant2-%d", time.Now().UnixNano())
	tenant2, _ := env.store.CreateTenant(context.Background(), "T2", slug2)
	_, key2, _ := env.store.CreateAPIKeyWithPlain(context.Background(), tenant2.ID, "k2", []string{"invoke", "manage"})

	rec2 := env.do(t, http.MethodGet, "/v1/snippets/"+snippetID, key2, nil)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404 (cross-tenant access must be blocked)", rec2.Code)
	}
}

func TestDeleteSnippet(t *testing.T) {
	env := setup(t)

	slug := fmt.Sprintf("del-sn-%d", time.Now().UnixNano())
	rec := env.do(t, http.MethodPost, "/v1/snippets", env.manageKey, map[string]any{
		"name": "Delete Me", "slug": slug, "language": "python",
	})
	body := decodeJSON(t, rec)
	snippetID, _ := body["id"].(string)

	rec2 := env.do(t, http.MethodDelete, "/v1/snippets/"+snippetID, env.manageKey, nil)
	if rec2.Code != http.StatusNoContent {
		t.Errorf("status = %d; want 204", rec2.Code)
	}

	rec3 := env.do(t, http.MethodGet, "/v1/snippets/"+snippetID, env.manageKey, nil)
	if rec3.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404 after deletion", rec3.Code)
	}
}

// --- Versions ---

func createTestSnippet(t *testing.T, env *testEnv) (snippetID string) {
	t.Helper()
	slug := fmt.Sprintf("vsn-sn-%d", time.Now().UnixNano())
	rec := env.do(t, http.MethodPost, "/v1/snippets", env.manageKey, map[string]any{
		"name": "VSN", "slug": slug, "language": "bun",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create snippet: status %d", rec.Code)
	}
	body := decodeJSON(t, rec)
	return body["id"].(string)
}

func TestCreateVersion(t *testing.T) {
	env := setup(t)
	snippetID := createTestSnippet(t, env)

	rec := env.do(t, http.MethodPost, "/v1/snippets/"+snippetID+"/versions", env.manageKey, map[string]any{
		"code":           "export async function handler() { return {ok: true} }",
		"input_schema":   "{}",
		"output_schema":  "{}",
		"timeout_ms":     5000,
		"max_memory_mb":  128,
		"max_cpu_percent": 100,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d; want 201\nbody: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec)
	if body["status"] != "draft" {
		t.Errorf("status = %q; want %q", body["status"], "draft")
	}
}

func TestPublishVersion(t *testing.T) {
	env := setup(t)
	snippetID := createTestSnippet(t, env)

	// Create a version.
	rec := env.do(t, http.MethodPost, "/v1/snippets/"+snippetID+"/versions", env.manageKey, map[string]any{
		"code": "export async function handler() { return {ok: true} }",
	})
	body := decodeJSON(t, rec)
	versionNum := int(body["version_number"].(float64))

	// Publish it to prod.
	path := fmt.Sprintf("/v1/snippets/%s/versions/%d/publish?env=prod", snippetID, versionNum)
	rec2 := env.do(t, http.MethodPost, path, env.manageKey, nil)
	if rec2.Code != http.StatusOK {
		t.Fatalf("publish status = %d; want 200\nbody: %s", rec2.Code, rec2.Body.String())
	}
	body2 := decodeJSON(t, rec2)
	if body2["status"] != "published" {
		t.Errorf("status = %q; want %q", body2["status"], "published")
	}
}

func TestListVersions(t *testing.T) {
	env := setup(t)
	snippetID := createTestSnippet(t, env)

	for i := 0; i < 3; i++ {
		env.do(t, http.MethodPost, "/v1/snippets/"+snippetID+"/versions", env.manageKey, map[string]any{
			"code": fmt.Sprintf("// version %d", i),
		})
	}

	rec := env.do(t, http.MethodGet, "/v1/snippets/"+snippetID+"/versions", env.manageKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	var versions []any
	if err := json.NewDecoder(rec.Body).Decode(&versions); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(versions) != 3 {
		t.Errorf("got %d versions; want 3", len(versions))
	}
}

// --- Invocations ---

// publishSnippetForInvoke creates a snippet, creates a version, and publishes
// it to prod. Returns the tenant slug and snippet slug for the invoke URL.
func publishSnippetForInvoke(t *testing.T, env *testEnv) (tenantSlug, snippetSlug string) {
	t.Helper()
	snippetSlug = fmt.Sprintf("invoke-sn-%d", time.Now().UnixNano())
	rec := env.do(t, http.MethodPost, "/v1/snippets", env.manageKey, map[string]any{
		"name": "Invoke Snippet", "slug": snippetSlug, "language": "bun",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create snippet: %d", rec.Code)
	}
	body := decodeJSON(t, rec)
	snippetID := body["id"].(string)

	rec2 := env.do(t, http.MethodPost, "/v1/snippets/"+snippetID+"/versions", env.manageKey, map[string]any{
		"code": "export async function handler(req) { return {ok: true} }",
	})
	if rec2.Code != http.StatusCreated {
		t.Fatalf("create version: %d", rec2.Code)
	}
	body2 := decodeJSON(t, rec2)
	versionNum := int(body2["version_number"].(float64))

	path := fmt.Sprintf("/v1/snippets/%s/versions/%d/publish?env=prod", snippetID, versionNum)
	rec3 := env.do(t, http.MethodPost, path, env.manageKey, nil)
	if rec3.Code != http.StatusOK {
		t.Fatalf("publish: %d\nbody: %s", rec3.Code, rec3.Body.String())
	}

	return env.tenant.Slug, snippetSlug
}

func TestInvoke_Success(t *testing.T) {
	env := setup(t)
	tenantSlug, snippetSlug := publishSnippetForInvoke(t, env)

	path := fmt.Sprintf("/v1/invoke/%s/%s?env=prod", tenantSlug, snippetSlug)

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"prompt":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.invokeKey)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	body := decodeJSON(t, rec)
	if body["invocation_id"] == "" {
		t.Error("response must include invocation_id")
	}
	if body["status"] != string(models.InvocationCompleted) {
		t.Errorf("status = %q; want %q", body["status"], models.InvocationCompleted)
	}

	if rec.Header().Get("X-Invocation-Id") == "" {
		t.Error("X-Invocation-Id header must be set")
	}
	if rec.Header().Get("X-Duration-Ms") == "" {
		t.Error("X-Duration-Ms header must be set")
	}
}

func TestInvoke_RequiresInvokeScope(t *testing.T) {
	env := setup(t)
	tenantSlug, snippetSlug := publishSnippetForInvoke(t, env)

	// Create a manage-only key (no invoke scope).
	_, manageOnly, _ := env.store.CreateAPIKeyWithPlain(context.Background(), env.tenant.ID, "manage-only", []string{"manage"})

	path := fmt.Sprintf("/v1/invoke/%s/%s", tenantSlug, snippetSlug)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+manageOnly)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 403", rec.Code)
	}
}

func TestInvoke_InvalidJSON(t *testing.T) {
	env := setup(t)
	tenantSlug, snippetSlug := publishSnippetForInvoke(t, env)

	path := fmt.Sprintf("/v1/invoke/%s/%s", tenantSlug, snippetSlug)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`not json`))
	req.Header.Set("Authorization", "Bearer "+env.invokeKey)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", rec.Code)
	}
}

func TestInvoke_NoPublishedVersion(t *testing.T) {
	env := setup(t)
	slug := fmt.Sprintf("no-pub-%d", time.Now().UnixNano())
	env.do(t, http.MethodPost, "/v1/snippets", env.manageKey, map[string]any{
		"name": "No Pub", "slug": slug, "language": "bun",
	})

	path := fmt.Sprintf("/v1/invoke/%s/%s", env.tenant.Slug, slug)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.invokeKey)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400 (no published version)", rec.Code)
	}
}

func TestInvoke_CrossTenantKeyRejected(t *testing.T) {
	env := setup(t)
	tenantSlug, snippetSlug := publishSnippetForInvoke(t, env)

	// A key from a different tenant.
	slug2 := fmt.Sprintf("tenant2-%d", time.Now().UnixNano())
	tenant2, _ := env.store.CreateTenant(context.Background(), "T2", slug2)
	_, key2, _ := env.store.CreateAPIKeyWithPlain(context.Background(), tenant2.ID, "k2", []string{"invoke"})

	path := fmt.Sprintf("/v1/invoke/%s/%s", tenantSlug, snippetSlug)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key2)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 403 (cross-tenant key)", rec.Code)
	}
}

func TestGetInvocation(t *testing.T) {
	env := setup(t)
	tenantSlug, snippetSlug := publishSnippetForInvoke(t, env)

	// Invoke to get an invocation ID.
	path := fmt.Sprintf("/v1/invoke/%s/%s", tenantSlug, snippetSlug)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.invokeKey)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	invocationID := rec.Header().Get("X-Invocation-Id")
	if invocationID == "" {
		t.Fatal("X-Invocation-Id header not set")
	}

	// Fetch the invocation.
	rec2 := env.do(t, http.MethodGet, "/v1/invocations/"+invocationID, env.manageKey, nil)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GetInvocation status = %d; want 200\nbody: %s", rec2.Code, rec2.Body.String())
	}
	body := decodeJSON(t, rec2)
	if body["id"] != invocationID {
		t.Errorf("id = %q; want %q", body["id"], invocationID)
	}
}

func TestGetInvocation_TenantIsolation(t *testing.T) {
	env := setup(t)
	tenantSlug, snippetSlug := publishSnippetForInvoke(t, env)

	path := fmt.Sprintf("/v1/invoke/%s/%s", tenantSlug, snippetSlug)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.invokeKey)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	invocationID := rec.Header().Get("X-Invocation-Id")

	// Tenant 2 tries to read it.
	slug2 := fmt.Sprintf("t2iso-%d", time.Now().UnixNano())
	tenant2, _ := env.store.CreateTenant(context.Background(), "T2", slug2)
	_, key2, _ := env.store.CreateAPIKeyWithPlain(context.Background(), tenant2.ID, "k2", []string{"manage"})

	rec2 := env.do(t, http.MethodGet, "/v1/invocations/"+invocationID, key2, nil)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404 (cross-tenant invocation access)", rec2.Code)
	}
}

// --- Auth edge cases ---

func TestUnauthenticated_Returns401(t *testing.T) {
	env := setup(t)
	for _, path := range []string{"/v1/snippets", "/v1/invocations/some-id"} {
		rec := env.do(t, http.MethodGet, path, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s: status = %d; want 401", path, rec.Code)
		}
	}
}

// --- Phase 2: Async, Stream, Version Pinning ---

// setupWithStreaming creates a testEnv whose mock executor also handles
// POST /run/stream by returning a single SSE chunk with done:true.
func setupWithStreaming(t *testing.T) *testEnv {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test postgres: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Mock executor that handles both /run and /run/stream.
	mockExec := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/run/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			flusher, _ := w.(http.Flusher)
			chunk := `{"data":"{\"ok\":true}","done":true}`
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
			if flusher != nil {
				flusher.Flush()
			}
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"output":"{\"ok\":true}","stderr":"","duration_ms":10,"peak_memory_mb":8,"exit_code":0,"error":""}`))
		}
	}))
	t.Cleanup(mockExec.Close)

	exec := remote.New(mockExec.URL, mockExec.URL)
	sched := scheduler.New(store, exec)
	log := zap.NewNop()
	router := api.NewRouter(store, sched, log)

	slug := fmt.Sprintf("test-stream-%d", time.Now().UnixNano())
	tenant, err := store.CreateTenant(context.Background(), "Stream Tenant", slug)
	if err != nil {
		t.Fatalf("create test tenant: %v", err)
	}

	_, manageKey, err := store.CreateAPIKeyWithPlain(context.Background(), tenant.ID, "manage-key",
		[]string{"admin", "manage", "invoke"})
	if err != nil {
		t.Fatalf("create manage key: %v", err)
	}
	_, invokeKey, err := store.CreateAPIKeyWithPlain(context.Background(), tenant.ID, "invoke-key",
		[]string{"invoke"})
	if err != nil {
		t.Fatalf("create invoke key: %v", err)
	}

	return &testEnv{
		router:    router,
		store:     store,
		tenant:    tenant,
		manageKey: manageKey,
		invokeKey: invokeKey,
	}
}

// setupWithRedis creates a testEnv wired to a real Redis (TEST_REDIS_URL).
func setupWithRedis(t *testing.T) (*testEnv, *redisstore.Client) {
	t.Helper()
	redisAddr := os.Getenv("TEST_REDIS_URL")
	if redisAddr == "" {
		t.Skip("TEST_REDIS_URL not set — skipping async integration test")
	}

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test postgres: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	rc, err := redisstore.New(redisAddr)
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	t.Cleanup(func() { rc.Close() })

	mockExec := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"output":"{\"ok\":true}","stderr":"","duration_ms":10,"peak_memory_mb":8,"exit_code":0,"error":""}`))
	}))
	t.Cleanup(mockExec.Close)

	exec := remote.New(mockExec.URL, mockExec.URL)
	sched := scheduler.NewWithQueue(store, exec, rc)
	log := zap.NewNop()
	router := api.NewRouter(store, sched, log)

	slug := fmt.Sprintf("test-async-%d", time.Now().UnixNano())
	tenant, err := store.CreateTenant(context.Background(), "Async Tenant", slug)
	if err != nil {
		t.Fatalf("create test tenant: %v", err)
	}

	_, manageKey, err := store.CreateAPIKeyWithPlain(context.Background(), tenant.ID, "manage-key",
		[]string{"admin", "manage", "invoke"})
	if err != nil {
		t.Fatalf("create manage key: %v", err)
	}
	_, invokeKey, err := store.CreateAPIKeyWithPlain(context.Background(), tenant.ID, "invoke-key",
		[]string{"invoke"})
	if err != nil {
		t.Fatalf("create invoke key: %v", err)
	}

	return &testEnv{
		router:    router,
		store:     store,
		tenant:    tenant,
		manageKey: manageKey,
		invokeKey: invokeKey,
	}, rc
}

func TestInvoke_AsyncMode(t *testing.T) {
	env, _ := setupWithRedis(t)
	tenantSlug, snippetSlug := publishSnippetForInvoke(t, env)

	path := fmt.Sprintf("/v1/invoke/%s/%s?env=prod", tenantSlug, snippetSlug)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.invokeKey)
	req.Header.Set("X-Invoke-Mode", "async")
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d; want 202\nbody: %s", rec.Code, rec.Body.String())
	}

	body := decodeJSON(t, rec)
	invocationID, _ := body["invocation_id"].(string)
	if invocationID == "" {
		t.Fatal("response must include invocation_id")
	}

	statusVal, _ := body["status"].(string)
	if statusVal != string(models.InvocationPending) {
		t.Errorf("status = %q; want %q", statusVal, models.InvocationPending)
	}

	statusURL, _ := body["status_url"].(string)
	if statusURL == "" {
		t.Error("response must include status_url")
	}

	// Poll GET /v1/invocations/{id} — the invocation should exist in the DB.
	rec2 := env.do(t, http.MethodGet, "/v1/invocations/"+invocationID, env.manageKey, nil)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET invocation status = %d; want 200\nbody: %s", rec2.Code, rec2.Body.String())
	}
	body2 := decodeJSON(t, rec2)
	if body2["id"] != invocationID {
		t.Errorf("invocation id = %q; want %q", body2["id"], invocationID)
	}
}

func TestInvoke_VersionPinning(t *testing.T) {
	env := setup(t)
	snippetSlug := fmt.Sprintf("pin-sn-%d", time.Now().UnixNano())

	// Create snippet.
	rec := env.do(t, http.MethodPost, "/v1/snippets", env.manageKey, map[string]any{
		"name": "Pin Snippet", "slug": snippetSlug, "language": "bun",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create snippet: %d", rec.Code)
	}
	snippetID := decodeJSON(t, rec)["id"].(string)

	// Create version 1 with distinctive code.
	rec2 := env.do(t, http.MethodPost, "/v1/snippets/"+snippetID+"/versions", env.manageKey, map[string]any{
		"code": "export async function handler() { return {version: 1} }",
	})
	if rec2.Code != http.StatusCreated {
		t.Fatalf("create version 1: %d", rec2.Code)
	}
	v1Num := int(decodeJSON(t, rec2)["version_number"].(float64))

	// Publish version 1.
	pub1Path := fmt.Sprintf("/v1/snippets/%s/versions/%d/publish?env=prod", snippetID, v1Num)
	env.do(t, http.MethodPost, pub1Path, env.manageKey, nil)

	// Create version 2.
	rec3 := env.do(t, http.MethodPost, "/v1/snippets/"+snippetID+"/versions", env.manageKey, map[string]any{
		"code": "export async function handler() { return {version: 2} }",
	})
	if rec3.Code != http.StatusCreated {
		t.Fatalf("create version 2: %d", rec3.Code)
	}
	v2Num := int(decodeJSON(t, rec3)["version_number"].(float64))

	// Publish version 2 (now active).
	pub2Path := fmt.Sprintf("/v1/snippets/%s/versions/%d/publish?env=prod", snippetID, v2Num)
	env.do(t, http.MethodPost, pub2Path, env.manageKey, nil)

	// Invoke with ?version=v1 — should use version 1 (pinned).
	invPath := fmt.Sprintf("/v1/invoke/%s/%s?env=prod&version=v%d", env.tenant.Slug, snippetSlug, v1Num)
	req := httptest.NewRequest(http.MethodPost, invPath, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.invokeKey)
	rec4 := httptest.NewRecorder()
	env.router.ServeHTTP(rec4, req)

	if rec4.Code != http.StatusOK {
		t.Fatalf("pin invoke status = %d; want 200\nbody: %s", rec4.Code, rec4.Body.String())
	}

	// The invocation should record version 1's ID.
	invocationID := rec4.Header().Get("X-Invocation-Id")
	if invocationID == "" {
		t.Fatal("X-Invocation-Id not set")
	}

	inv, err := env.store.GetInvocation(context.Background(), invocationID)
	if err != nil {
		t.Fatalf("GetInvocation: %v", err)
	}

	// Verify the version recorded matches v1, not v2.
	v1, _ := env.store.GetVersionByNumber(context.Background(), snippetID, v1Num)
	if inv.VersionID != v1.ID {
		t.Errorf("VersionID = %q; want v1 ID %q (pinned)", inv.VersionID, v1.ID)
	}
}

func TestInvoke_StreamMode(t *testing.T) {
	env := setupWithStreaming(t)
	tenantSlug, snippetSlug := publishSnippetForInvoke(t, env)

	path := fmt.Sprintf("/v1/invoke/%s/%s?env=prod", tenantSlug, snippetSlug)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.invokeKey)
	req.Header.Set("X-Invoke-Mode", "stream")
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q; want text/event-stream", ct)
	}

	if rec.Header().Get("X-Invocation-Id") == "" {
		t.Error("X-Invocation-Id header must be set for stream response")
	}

	// Parse SSE events from the body.
	scanner := bufio.NewScanner(rec.Body)
	var events []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			events = append(events, strings.TrimPrefix(line, "data: "))
		}
	}

	if len(events) == 0 {
		t.Fatal("expected at least one SSE data event, got none")
	}

	// The last event should be the final done event.
	lastEvent := events[len(events)-1]
	var chunk map[string]any
	if err := json.Unmarshal([]byte(lastEvent), &chunk); err != nil {
		t.Fatalf("unmarshal last SSE event %q: %v", lastEvent, err)
	}
	if done, _ := chunk["done"].(bool); !done {
		t.Errorf("last SSE event done = %v; want true. event: %s", done, lastEvent)
	}
}
