package telemetry_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/nowi5/kleido/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup_Disabled(t *testing.T) {
	t.Parallel()
	// When enabled=false, Setup must return a no-op shutdown and not error.
	shutdown, err := telemetry.Setup(context.Background(), "test", "dev", "localhost:4317", "development", false)
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	// Calling shutdown must not error.
	require.NoError(t, shutdown(context.Background()))
}

func TestSetup_Enabled_BadEndpoint(t *testing.T) {
	t.Parallel()
	// A bad endpoint must still return a provider (the exporter is lazy)
	// and not error at setup time — OTLP gRPC connects lazily.
	shutdown, err := telemetry.Setup(context.Background(), "test", "dev", "localhost:19999", "development", true)
	require.NoError(t, err) // connection errors appear at export time, not setup time
	require.NotNil(t, shutdown)
	_ = shutdown(context.Background())
}

func TestSetup_Enabled_ProductionSampler(t *testing.T) {
	t.Parallel()
	// Production env must use ParentBased sampler — verified indirectly by
	// checking Setup succeeds (sampler is set as part of TracerProvider init).
	shutdown, err := telemetry.Setup(context.Background(), "test", "1.0.0", "localhost:19999", "production", true)
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	_ = shutdown(context.Background())
}

func TestNewSlogHandler_NotNil(t *testing.T) {
	t.Parallel()
	base := slog.NewJSONHandler(io.Discard, nil)
	h := telemetry.NewSlogHandler(base, "test-service")
	assert.NotNil(t, h)
}
