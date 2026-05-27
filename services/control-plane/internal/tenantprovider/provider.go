package tenantprovider

import "fmt"

// Quota captures high-level tenant resource limits.
type Quota struct {
	CPUMilli int
	MemoryMB int
}

// Policy captures tenant-level egress restrictions.
type Policy struct {
	BlockedCIDRs   []string
	BlockedDomains []string
}

// TenantProvider maps tenant IDs to execution isolation settings.
type TenantProvider interface {
	Namespace(tenantID string) string
	ResourceQuota(tenantID string) Quota
	EgressPolicy(tenantID string) Policy
}

// SharedTenantProvider runs all tenants in one shared namespace.
type SharedTenantProvider struct {
	NamespaceName string
	DefaultQuota  Quota
	DefaultPolicy Policy
}

// NewSharedTenantProvider returns a shared-namespace tenant provider.
func NewSharedTenantProvider(namespace string, quota Quota, policy Policy) *SharedTenantProvider {
	if namespace == "" {
		namespace = "shared"
	}
	return &SharedTenantProvider{
		NamespaceName: namespace,
		DefaultQuota:  quota,
		DefaultPolicy: policy,
	}
}

func (p *SharedTenantProvider) Namespace(string) string {
	return p.NamespaceName
}

func (p *SharedTenantProvider) ResourceQuota(string) Quota {
	return p.DefaultQuota
}

func (p *SharedTenantProvider) EgressPolicy(string) Policy {
	return p.DefaultPolicy
}

// NamespacedTenantProvider maps each tenant to an isolated namespace.
type NamespacedTenantProvider struct {
	Prefix        string
	DefaultQuota  Quota
	DefaultPolicy Policy
}

// NewNamespacedTenantProvider returns a namespace-per-tenant provider.
func NewNamespacedTenantProvider(prefix string, quota Quota, policy Policy) *NamespacedTenantProvider {
	if prefix == "" {
		prefix = "tenant"
	}
	return &NamespacedTenantProvider{
		Prefix:        prefix,
		DefaultQuota:  quota,
		DefaultPolicy: policy,
	}
}

func (p *NamespacedTenantProvider) Namespace(tenantID string) string {
	if tenantID == "" {
		return fmt.Sprintf("%s-unknown", p.Prefix)
	}
	return fmt.Sprintf("%s-%s", p.Prefix, tenantID)
}

func (p *NamespacedTenantProvider) ResourceQuota(string) Quota {
	return p.DefaultQuota
}

func (p *NamespacedTenantProvider) EgressPolicy(string) Policy {
	return p.DefaultPolicy
}
