package reqctx

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestWithIP(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ip := "192.168.1.1"

	result := WithIP(ctx, ip)

	if result == ctx {
		t.Error("WithIP should return a new context")
	}

	got := IPFromContext(result)
	if got != ip {
		t.Errorf("IPFromContext() = %q; want %q", got, ip)
	}
}

func TestIPFromContext_Empty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	got := IPFromContext(ctx)

	if got != "" {
		t.Errorf("IPFromContext() = %q; want empty string", got)
	}
}

func TestIPFromContext_NilContext(t *testing.T) {
	t.Parallel()

	got := IPFromContext(nil)

	if got != "" {
		t.Errorf("IPFromContext(nil) = %q; want empty string", got)
	}
}

func TestWithUserAgent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"

	result := WithUserAgent(ctx, ua)

	if result == ctx {
		t.Error("WithUserAgent should return a new context")
	}

	got := UserAgentFromContext(result)
	if got != ua {
		t.Errorf("UserAgentFromContext() = %q; want %q", got, ua)
	}
}

func TestUserAgentFromContext_Empty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	got := UserAgentFromContext(ctx)

	if got != "" {
		t.Errorf("UserAgentFromContext() = %q; want empty string", got)
	}
}

func TestUserAgentFromContext_NilContext(t *testing.T) {
	t.Parallel()

	got := UserAgentFromContext(nil)

	if got != "" {
		t.Errorf("UserAgentFromContext(nil) = %q; want empty string", got)
	}
}

func TestWithTenantID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenantID := uuid.New()

	result := WithTenantID(ctx, tenantID)

	if result == ctx {
		t.Error("WithTenantID should return a new context")
	}

	got := TenantIDFromContext(result)
	if got != tenantID {
		t.Errorf("TenantIDFromContext() = %v; want %v", got, tenantID)
	}
}

func TestTenantIDFromContext_Nil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	got := TenantIDFromContext(ctx)

	if got != uuid.Nil {
		t.Errorf("TenantIDFromContext() = %v; want uuid.Nil", got)
	}
}

func TestTenantIDFromContext_NilContext(t *testing.T) {
	t.Parallel()

	got := TenantIDFromContext(nil)

	if got != uuid.Nil {
		t.Errorf("TenantIDFromContext(nil) = %v; want uuid.Nil", got)
	}
}

func TestTenantIDFromContext_InvalidType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxKeyTenantID{}, "not-a-uuid")

	got := TenantIDFromContext(ctx)

	if got != uuid.Nil {
		t.Errorf("TenantIDFromContext() with wrong type = %v; want uuid.Nil", got)
	}
}

func TestWithTenantID_UuidNil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	result := WithTenantID(ctx, uuid.Nil)

	got := TenantIDFromContext(result)
	if got != uuid.Nil {
		t.Errorf("TenantIDFromContext() with uuid.Nil = %v; want uuid.Nil", got)
	}
}

func TestChainedContexts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ip := "10.0.0.1"
	ua := "TestAgent/1.0"
	tenantID := uuid.New()

	ctx = WithIP(ctx, ip)
	ctx = WithUserAgent(ctx, ua)
	ctx = WithTenantID(ctx, tenantID)

	if IPFromContext(ctx) != ip {
		t.Errorf("IPFromContext() = %q; want %q", IPFromContext(ctx), ip)
	}
	if UserAgentFromContext(ctx) != ua {
		t.Errorf("UserAgentFromContext() = %q; want %q", UserAgentFromContext(ctx), ua)
	}
	if TenantIDFromContext(ctx) != tenantID {
		t.Errorf("TenantIDFromContext() = %v; want %v", TenantIDFromContext(ctx), tenantID)
	}
}

func TestOriginalContext_Unchanged(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ip := "192.168.1.1"

	_ = WithIP(ctx, ip)

	if IPFromContext(ctx) != "" {
		t.Error("original context should be unchanged")
	}
}

func TestMultipleTenantIDs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenant1 := uuid.New()
	tenant2 := uuid.New()

	ctx1 := WithTenantID(ctx, tenant1)
	ctx2 := WithTenantID(ctx1, tenant2)

	if TenantIDFromContext(ctx) != uuid.Nil {
		t.Error("original context should have no tenant")
	}
	if TenantIDFromContext(ctx1) != tenant1 {
		t.Errorf("ctx1: got %v; want %v", TenantIDFromContext(ctx1), tenant1)
	}
	if TenantIDFromContext(ctx2) != tenant2 {
		t.Errorf("ctx2: got %v; want %v", TenantIDFromContext(ctx2), tenant2)
	}
}

func TestIPTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ip   string
	}{
		{"ipv4", "192.168.1.1"},
		{"ipv6", "::1"},
		{"localhost", "127.0.0.1"},
		{"private", "10.0.0.1"},
		{"public", "8.8.8.8"},
		{"empty", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithIP(context.Background(), tc.ip)
			got := IPFromContext(ctx)
			if got != tc.ip {
				t.Errorf("IPFromContext() = %q; want %q", got, tc.ip)
			}
		})
	}
}

func TestUserAgentTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ua   string
	}{
		{"chrome", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"},
		{"curl", "curl/7.68.0"},
		{"empty", ""},
		{"custom", "MyApp/1.0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithUserAgent(context.Background(), tc.ua)
			got := UserAgentFromContext(ctx)
			if got != tc.ua {
				t.Errorf("UserAgentFromContext() = %q; want %q", got, tc.ua)
			}
		})
	}
}

func TestContextKeyIsolation(t *testing.T) {
	t.Parallel()

	ctx := WithIP(context.Background(), "1.1.1.1")
	ctx = WithUserAgent(ctx, "test")
	ctx = WithTenantID(ctx, uuid.New())

	if IPFromContext(ctx) == "" {
		t.Error("IP should be set")
	}
	if UserAgentFromContext(ctx) == "" {
		t.Error("UserAgent should be set")
	}
	if TenantIDFromContext(ctx) == uuid.Nil {
		t.Error("TenantID should be set")
	}
}

func TestIPOverwrite(t *testing.T) {
	t.Parallel()

	ctx := WithIP(context.Background(), "1.1.1.1")
	ctx = WithIP(ctx, "2.2.2.2")

	if IPFromContext(ctx) != "2.2.2.2" {
		t.Errorf("IP should be overwritten: got %s", IPFromContext(ctx))
	}
}

func TestUserAgentOverwrite(t *testing.T) {
	t.Parallel()

	ctx := WithUserAgent(context.Background(), "agent1")
	ctx = WithUserAgent(ctx, "agent2")

	if UserAgentFromContext(ctx) != "agent2" {
		t.Errorf("UserAgent should be overwritten: got %s", UserAgentFromContext(ctx))
	}
}

func TestTenantIDOverwrite(t *testing.T) {
	t.Parallel()

	id1 := uuid.New()
	id2 := uuid.New()

	ctx := WithTenantID(context.Background(), id1)
	ctx = WithTenantID(ctx, id2)

	if TenantIDFromContext(ctx) != id2 {
		t.Errorf("TenantID should be overwritten: got %v", TenantIDFromContext(ctx))
	}
}