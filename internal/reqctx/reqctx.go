// Package reqctx provides context keys and helpers for propagating
// HTTP request metadata (IP address, user-agent, tenant) through the call stack.
// Values are set by middleware and read by the service layer
// for structured audit logging.
package reqctx

import (
	"context"

	"github.com/google/uuid"
)

type ctxKeyIP struct{}
type ctxKeyUserAgent struct{}
type ctxKeyTenantID struct{}

// WithIP stores the request IP address in ctx.
func WithIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ctxKeyIP{}, ip)
}

// IPFromContext returns the IP stored by WithIP, or "" if not set.
func IPFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	ip, _ := ctx.Value(ctxKeyIP{}).(string) //nolint:errcheck
	return ip
}

// WithUserAgent stores the request User-Agent in ctx.
func WithUserAgent(ctx context.Context, ua string) context.Context {
	return context.WithValue(ctx, ctxKeyUserAgent{}, ua)
}

// UserAgentFromContext returns the User-Agent stored by WithUserAgent, or "" if not set.
func UserAgentFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	ua, _ := ctx.Value(ctxKeyUserAgent{}).(string) //nolint:errcheck
	return ua
}

// WithTenantID stores the tenant ID in ctx.
func WithTenantID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKeyTenantID{}, id)
}

// TenantIDFromContext returns the tenant ID stored by WithTenantID, or uuid.Nil if not set.
func TenantIDFromContext(ctx context.Context) uuid.UUID {
	if ctx == nil {
		return uuid.Nil
	}
	if id, ok := ctx.Value(ctxKeyTenantID{}).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}