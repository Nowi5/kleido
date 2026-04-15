package service

// Auth event types — the complete vocabulary for structured audit logging.
// Any new event type must be added here and documented in SECURITY.md.
//
// Valid values:
//   - auth.login.success
//   - auth.login.failure
//   - auth.login.locked
//   - auth.logout
//   - auth.token.refresh
//   - auth.password.reset_requested
//   - auth.password.reset_completed
const (
	EventLoginSuccess         = "auth.login.success"
	EventLoginFailure         = "auth.login.failure"
	EventLoginLocked          = "auth.login.locked"
	EventLogout               = "auth.logout"
	EventTokenRefresh         = "auth.token.refresh"
	EventPasswordResetRequest = "auth.password.reset_requested"
	EventPasswordResetDone    = "auth.password.reset_completed"
)
