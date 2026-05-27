package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/runeforge/control-plane/internal/api/handlers"
	"github.com/runeforge/control-plane/internal/api/middleware"
	"github.com/runeforge/control-plane/internal/scheduler"
	"github.com/runeforge/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// NewRouter builds and returns the fully configured chi router.
func NewRouter(store *postgres.Store, sched *scheduler.Scheduler, log *zap.Logger, encKey []byte) http.Handler {
	r := chi.NewRouter()

	// Global middleware.
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger(log))

	// Instantiate handlers.
	tenantsH := handlers.NewTenantsHandler(store, log)
	snippetsH := handlers.NewSnippetsHandler(store, log)
	versionsH := handlers.NewVersionsHandler(store, log)
	invocationsH := handlers.NewInvocationsHandler(store, sched, log)
	secretsH := handlers.NewSecretsHandler(store, log, encKey)
	gitIntH := handlers.NewGitIntegrationHandler(store, log)
	webhookH := handlers.NewWebhookHandler(store, sched, log)
	logsH := handlers.NewLogsHandler(store, log)
	metricsH := handlers.NewMetricsHandler(store, log)
	replayH := handlers.NewReplayHandler(store, sched, log)
	embedH := handlers.NewEmbedHandler(store, log)

	// Health check — no auth.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Bootstrap endpoint — intentionally unauthenticated.
	// See handler comment for production guidance.
	r.Post("/v1/tenants", tenantsH.CreateTenant)

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		authMw := middleware.Auth(store, log)
		r.Use(authMw)

		// API key management — requires admin scope.
		r.With(middleware.RequireScope("admin", log)).
			Post("/v1/tenants/{tenantSlug}/api-keys", tenantsH.CreateAPIKey)

		// Egress policy.
		r.Get("/v1/tenants/{tenantSlug}/egress", tenantsH.GetEgressPolicy)
		r.With(middleware.RequireScope("admin", log)).
			Put("/v1/tenants/{tenantSlug}/egress", tenantsH.UpdateEgressPolicy)

		// Snippets.
		r.Get("/v1/snippets", snippetsH.ListSnippets)
		r.With(middleware.RequireScope("manage", log)).
			Post("/v1/snippets", snippetsH.CreateSnippet)
		r.Get("/v1/snippets/{snippetID}", snippetsH.GetSnippet)
		r.With(middleware.RequireScope("manage", log)).
			Delete("/v1/snippets/{snippetID}", snippetsH.DeleteSnippet)

		// Versions.
		r.Get("/v1/snippets/{snippetID}/versions", versionsH.ListVersions)
		r.With(middleware.RequireScope("manage", log)).
			Post("/v1/snippets/{snippetID}/versions", versionsH.CreateVersion)
		r.Get("/v1/snippets/{snippetID}/versions/{num}", versionsH.GetVersion)
		r.With(middleware.RequireScope("manage", log)).
			Post("/v1/snippets/{snippetID}/versions/{num}/publish", versionsH.PublishVersion)

		// Canary traffic splitting.
		r.With(middleware.RequireScope("manage", log)).
			Post("/v1/snippets/{snippetID}/canary", versionsH.SetCanary)
		r.With(middleware.RequireScope("manage", log)).
			Delete("/v1/snippets/{snippetID}/canary", versionsH.ClearCanary)

		// Secrets.
		r.Get("/v1/secrets", secretsH.ListSecrets)
		r.With(middleware.RequireScope("manage", log)).
			Post("/v1/secrets", secretsH.CreateSecret)
		r.With(middleware.RequireScope("manage", log)).
			Delete("/v1/secrets/{secretID}", secretsH.DeleteSecret)

		// Git integrations.
		r.With(middleware.RequireScope("manage", log)).
			Post("/v1/snippets/{snippetID}/git-integration", gitIntH.Create)
		r.With(middleware.RequireScope("manage", log)).
			Get("/v1/snippets/{snippetID}/git-integration", gitIntH.Get)
		r.With(middleware.RequireScope("manage", log)).
			Delete("/v1/snippets/{snippetID}/git-integration", gitIntH.Delete)

		// Invocation detail lookup.
		r.Get("/v1/invocations/{id}", invocationsH.GetInvocation)

		// Observability endpoints.
		r.Get("/v1/logs/snippets/{snippetID}", logsH.GetSnippetLogs)
		r.Get("/v1/metrics/snippets/{snippetID}", metricsH.GetSnippetMetrics)
		r.With(middleware.RequireScope("manage", log)).
			Post("/v1/invocations/{id}/replay", replayH.ReplayInvocation)

		// Embed token management.
		r.With(middleware.RequireScope("manage", log)).
			Post("/v1/embed/tokens", embedH.CreateToken)
		r.With(middleware.RequireScope("manage", log)).
			Delete("/v1/embed/tokens/{tokenID}", embedH.RevokeToken)
	})

	// The invoke endpoint performs its own auth inline (see handler comment).
	// It is registered outside the auth middleware group because it uses
	// path-based tenant routing that differs from the standard Bearer flow.
	r.Post("/v1/invoke/{tenantSlug}/{snippetSlug}", invocationsH.Invoke)

	// Git webhook endpoint — no auth middleware; HMAC signature is verified inline.
	r.Post("/v1/webhooks/git/{snippetID}", webhookH.GitWebhook)

	// Embed read routes authenticated by opaque embed token.
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthEmbed(store, log))
		r.Get("/v1/embed/bootstrap", embedH.Bootstrap)
		r.Get("/v1/embed/snippets", embedH.ListSnippets)
		r.Get("/v1/embed/snippets/{snippetID}", embedH.GetSnippet)
		r.Get("/v1/embed/snippets/{snippetID}/metrics", embedH.GetSnippetMetrics)
		r.Get("/v1/embed/snippets/{snippetID}/logs", embedH.GetSnippetLogs)
	})

	return r
}
