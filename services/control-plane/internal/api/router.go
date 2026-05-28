package api

import (
	"crypto/rsa"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/abskrj/velane/services/control-plane/internal/api/handlers"
	"github.com/abskrj/velane/services/control-plane/internal/api/middleware"
	"github.com/abskrj/velane/services/control-plane/internal/audit"
	"github.com/abskrj/velane/services/control-plane/internal/auth"
	"github.com/abskrj/velane/services/control-plane/internal/platformlibs"
	"github.com/abskrj/velane/services/control-plane/internal/scheduler"
	"github.com/abskrj/velane/services/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// NewRouter builds and returns the fully configured chi router.
func NewRouter(store *postgres.Store, sched *scheduler.Scheduler, log *zap.Logger, encKey []byte, authProvider auth.Provider, platLibs []platformlibs.PlatformLib) http.Handler {
	return newRouter(store, sched, log, encKey, authProvider, nil, platLibs)
}

// NewRouterWithJWT builds the router and wires the RSA public key for the JWKS endpoint.
func NewRouterWithJWT(store *postgres.Store, sched *scheduler.Scheduler, log *zap.Logger, encKey []byte, authProvider auth.Provider, pubKey *rsa.PublicKey, platLibs []platformlibs.PlatformLib) http.Handler {
	return newRouter(store, sched, log, encKey, authProvider, pubKey, platLibs)
}

func newRouter(store *postgres.Store, sched *scheduler.Scheduler, log *zap.Logger, encKey []byte, authProvider auth.Provider, pubKey *rsa.PublicKey, platLibs []platformlibs.PlatformLib) http.Handler {
	r := chi.NewRouter()

	// Global middleware.
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger(log))

	// Audit logger (shared across handlers).
	auditor := audit.New(store, log)

	// Instantiate handlers.
	tenantsH := handlers.NewTenantsHandler(store, log).WithAuditor(auditor)
	snippetsH := handlers.NewSnippetsHandler(store, log)
	versionsH := handlers.NewVersionsHandler(store, log).WithAuditor(auditor)
	invocationsH := handlers.NewInvocationsHandler(store, sched, log).WithAuthProvider(authProvider)
	secretsH := handlers.NewSecretsHandler(store, log, encKey).WithAuditor(auditor)
	gitIntH := handlers.NewGitIntegrationHandler(store, log)
	webhookH := handlers.NewWebhookHandler(store, sched, log)
	logsH := handlers.NewLogsHandler(store, log)
	metricsH := handlers.NewMetricsHandler(store, log)
	replayH := handlers.NewReplayHandler(store, sched, log)
	embedH := handlers.NewEmbedHandler(store, log)
	librariesH := handlers.NewLibrariesHandler(store, log, platLibs)
	adminAuthH := handlers.NewAdminAuthHandler(authProvider, store, log)
	if pubKey != nil {
		adminAuthH = adminAuthH.WithPublicKey(pubKey)
	}
	brandingH := handlers.NewBrandingHandler(store, log).WithAuditor(auditor)
	membersH := handlers.NewMembersHandler(store, log).WithAuditor(auditor)
	usageH := handlers.NewUsageHandler(store, log)
	apikeysH := handlers.NewAPIKeysHandler(store, log).WithAuditor(auditor)
	auditH := handlers.NewAuditHandler(store, log)

	// Health check — no auth.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Bootstrap endpoint — intentionally unauthenticated.
	// See handler comment for production guidance.
	r.Post("/v1/tenants", tenantsH.CreateTenant)

	// JWKS — used by third parties to verify Velane JWTs (no auth).
	r.Get("/.well-known/jwks.json", adminAuthH.JWKS)

	// Admin auth — no API key required, session-based.
	r.Post("/v1/admin/auth/register", adminAuthH.Register)
	r.Post("/v1/admin/auth/login", adminAuthH.Login)
	r.Post("/v1/admin/auth/logout", adminAuthH.Logout)
	r.Post("/v1/admin/auth/refresh", adminAuthH.RefreshToken)
	r.With(middleware.SessionAuth(authProvider, store, log)).
		Get("/v1/admin/auth/me", adminAuthH.Me)

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		// SessionAuth runs first: if the Bearer token is a valid JWT it sets the session user
		// and (when X-Tenant is present) their tenant membership role in context.
		// Auth then attempts API key validation — if the token was a JWT, ValidateAPIKey will
		// fail harmlessly and the session context values are used by RequireScope instead.
		r.Use(middleware.SessionAuth(authProvider, store, log))
		authMw := middleware.Auth(store, log)
		r.Use(authMw)

		// API key management — requires admin scope.
		r.With(middleware.RequireScope("admin", log)).
			Post("/v1/tenants/{tenantSlug}/api-keys", tenantsH.CreateAPIKey)
		r.With(middleware.RequireScope("admin", log)).
			Get("/v1/tenants/{tenantSlug}/api-keys", apikeysH.ListAPIKeys)
		r.With(middleware.RequireScope("admin", log)).
			Delete("/v1/tenants/{tenantSlug}/api-keys/{keyID}", apikeysH.DeleteAPIKey)

		// Egress policy.
		r.Get("/v1/tenants/{tenantSlug}/egress", tenantsH.GetEgressPolicy)
		r.With(middleware.RequireScope("admin", log)).
			Put("/v1/tenants/{tenantSlug}/egress", tenantsH.UpdateEgressPolicy)

		// Branding.
		r.With(middleware.RequireScope("invoke", log)).
			Get("/v1/tenants/{tenantSlug}/branding", brandingH.GetBranding)
		r.With(middleware.RequireScope("admin", log)).
			Put("/v1/tenants/{tenantSlug}/branding", brandingH.UpdateBranding)

		// Team members and invites.
		r.With(middleware.RequireScope("admin", log)).
			Get("/v1/tenants/{tenantSlug}/members", membersH.ListMembers)
		r.With(middleware.RequireScope("admin", log)).
			Post("/v1/tenants/{tenantSlug}/members/invite", membersH.InviteMember)
		r.With(middleware.RequireScope("admin", log)).
			Delete("/v1/tenants/{tenantSlug}/members/{userID}", membersH.RemoveMember)
		r.With(middleware.RequireScope("admin", log)).
			Get("/v1/tenants/{tenantSlug}/members/invites", membersH.ListInvites)

		// Usage aggregation.
		r.With(middleware.RequireScope("admin", log)).
			Get("/v1/tenants/{tenantSlug}/usage", usageH.GetUsage)

		// Audit log.
		r.With(middleware.RequireScope("admin", log)).
			Get("/v1/tenants/{slug}/audit-log", auditH.ListAuditLog)

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

		// Secrets / Variables.
		r.Get("/v1/secrets", secretsH.ListSecrets)
		r.With(middleware.RequireScope("manage", log)).
			Post("/v1/secrets", secretsH.CreateSecret)
		r.With(middleware.RequireScope("manage", log)).
			Patch("/v1/secrets/{secretID}", secretsH.UpdateSecret)
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

		// Libraries.
		r.Get("/v1/libraries", librariesH.ListAll)
		r.With(middleware.RequireScope("manage", log)).
			Post("/v1/libraries", librariesH.Create)
		r.With(middleware.RequireScope("manage", log)).
			Delete("/v1/libraries/{libraryID}", librariesH.Delete)
		r.Get("/v1/libraries/{libraryID}/versions", librariesH.ListVersions)
		r.With(middleware.RequireScope("manage", log)).
			Post("/v1/libraries/{libraryID}/versions", librariesH.CreateVersion)
		r.With(middleware.RequireScope("manage", log)).
			Post("/v1/libraries/{libraryID}/versions/{num}/publish", librariesH.PublishVersion)

		// Embed token management.
		r.With(middleware.RequireScope("manage", log)).
			Get("/v1/embed/tokens", embedH.ListTokens)
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
