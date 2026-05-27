package tenantprovider

import "testing"

func TestSharedTenantProvider(t *testing.T) {
	p := NewSharedTenantProvider("global", Quota{CPUMilli: 500, MemoryMB: 256}, Policy{
		BlockedDomains: []string{"example.com"},
	})

	if got := p.Namespace("tenant-1"); got != "global" {
		t.Fatalf("namespace = %q; want %q", got, "global")
	}
	if p.ResourceQuota("tenant-1").CPUMilli != 500 {
		t.Fatalf("cpu quota mismatch")
	}
	if len(p.EgressPolicy("tenant-1").BlockedDomains) != 1 {
		t.Fatalf("egress policy mismatch")
	}
}

func TestNamespacedTenantProvider(t *testing.T) {
	p := NewNamespacedTenantProvider("rf", Quota{CPUMilli: 1000, MemoryMB: 512}, Policy{})

	if got := p.Namespace("tenant-abc"); got != "rf-tenant-abc" {
		t.Fatalf("namespace = %q; want %q", got, "rf-tenant-abc")
	}
	if got := p.Namespace(""); got != "rf-unknown" {
		t.Fatalf("empty tenant namespace = %q; want %q", got, "rf-unknown")
	}
}
