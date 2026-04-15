// Package reqctx provides context keys and helpers for propagating
// HTTP request metadata (IP address, user-agent) through the call stack.
// Values are set by the RequestLogger middleware and read by the service layer
// for structured audit logging.
package reqctx

import "context"

type ctxKeyIP struct{}
type ctxKeyUserAgent struct{}

// WithIP stores the request IP address in ctx.
func WithIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ctxKeyIP{}, ip)
}

// IPFromContext returns the IP stored by WithIP, or "" if not set.
func IPFromContext(ctx context.Context) string {
	ip, _ := ctx.Value(ctxKeyIP{}).(string) //nolint:errcheck
	return ip
}

// WithUserAgent stores the request User-Agent in ctx.
func WithUserAgent(ctx context.Context, ua string) context.Context {
	return context.WithValue(ctx, ctxKeyUserAgent{}, ua)
}

// UserAgentFromContext returns the User-Agent stored by WithUserAgent, or "" if not set.
func UserAgentFromContext(ctx context.Context) string {
	ua, _ := ctx.Value(ctxKeyUserAgent{}).(string) //nolint:errcheck
	return ua
}
