package service

import (
	"context"
	"log/slog"

	"kleido/internal/logger"
)

// EmailSender is the interface for sending transactional emails.
// In this sprint, the implementation is a stub that logs the email.
// In production, replace with SendGrid, SES, Postmark, or similar.
type EmailSender interface {
	SendPasswordReset(ctx context.Context, toEmail, resetURL string) error
}

// StubEmailSender logs the email instead of sending it.
// Suitable for development and testing — replace with a real implementation
// before enabling password reset in production.
type StubEmailSender struct{}

// SendPasswordReset logs the reset URL to stdout. In development this allows
// developers to copy the reset link from the server log without needing a
// real email provider configured.
func (s *StubEmailSender) SendPasswordReset(ctx context.Context, toEmail, resetURL string) error {
	logger.FromContext(ctx).InfoContext(ctx, "password reset email (stub)",
		slog.String("to", toEmail),
		slog.String("reset_url", resetURL),
	)
	return nil
}
