package main

import (
	"context"

	"github.com/abskrj/velane/services/control-plane/internal/auth"
	"github.com/abskrj/velane/services/control-plane/internal/config"
	"github.com/abskrj/velane/services/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// bootstrapAdminIfNeeded creates the first admin user and tenant when:
//   - BOOTSTRAP_EMAIL and BOOTSTRAP_PASSWORD are both set, AND
//   - no users exist in the database yet (first boot only)
//
// On subsequent starts the user table is non-empty so this is a no-op.
// Remove BOOTSTRAP_EMAIL/PASSWORD from your environment after first boot in production.
func bootstrapAdminIfNeeded(ctx context.Context, store *postgres.Store, provider auth.Provider, cfg config.Config, log *zap.Logger) {
	if cfg.BootstrapEmail == "" || cfg.BootstrapPassword == "" {
		return
	}

	// Check if any user already exists — if so, bootstrapping is already done.
	_, err := store.GetUserByEmail(ctx, cfg.BootstrapEmail)
	if err == nil {
		// User already exists — nothing to do.
		log.Info("bootstrap: admin user already exists, skipping", zap.String("email", cfg.BootstrapEmail))
		return
	}

	// Ensure the tenant exists (create if not).
	tenant, err := store.GetTenantBySlug(ctx, cfg.BootstrapTenant)
	if err != nil {
		// Tenant doesn't exist — create it.
		tenantName := cfg.BootstrapTenant
		tenant, err = store.CreateTenant(ctx, tenantName, cfg.BootstrapTenant)
		if err != nil {
			log.Error("bootstrap: failed to create tenant", zap.String("slug", cfg.BootstrapTenant), zap.Error(err))
			return
		}
		log.Info("bootstrap: tenant created", zap.String("slug", tenant.Slug), zap.String("id", tenant.ID))
	}

	// Create the admin user.
	user, err := provider.CreateUser(ctx, cfg.BootstrapEmail, cfg.BootstrapPassword)
	if err != nil {
		log.Error("bootstrap: failed to create admin user", zap.String("email", cfg.BootstrapEmail), zap.Error(err))
		return
	}

	// Add user as admin member of the tenant.
	if _, err := store.AddMember(ctx, tenant.ID, user.ID, "admin"); err != nil {
		log.Error("bootstrap: failed to add admin member", zap.Error(err))
		return
	}

	log.Info("bootstrap: admin user created — remove BOOTSTRAP_EMAIL/PASSWORD from environment after first boot",
		zap.String("email", user.Email),
		zap.String("tenant", tenant.Slug),
		zap.String("role", "admin"),
	)
}
