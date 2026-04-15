package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"kleido/internal/auth"
)

// writeTempPEM writes a single PEM block to a temp file and returns its path.
func writeTempPEM(t *testing.T, pemType string, derBytes []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "key.pem")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("create PEM file: %v", err)
	}
	if err := pem.Encode(f, &pem.Block{Type: pemType, Bytes: derBytes}); err != nil {
		t.Fatalf("pem.Encode: %v", err)
	}
	_ = f.Close() //nolint:errcheck
	return path
}

// generateRSAKey returns a freshly generated 2048-bit RSA key pair.
func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return priv
}

// ── LoadPrivateKey ─────────────────────────────────────────────────────────

func TestLoadPrivateKey_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := auth.LoadPrivateKey(filepath.Join(t.TempDir(), "nonexistent.pem"))
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadPrivateKey_MalformedPEM(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(path, []byte("not valid PEM content at all"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := auth.LoadPrivateKey(path)
	if err == nil {
		t.Error("expected error for malformed PEM, got nil")
	}
}

func TestLoadPrivateKey_Success(t *testing.T) {
	t.Parallel()

	priv := generateRSAKey(t)
	path := writeTempPEM(t, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(priv))

	got, err := auth.LoadPrivateKey(path)
	if err != nil {
		t.Fatalf("LoadPrivateKey: %v", err)
	}
	if got == nil || got.N.Cmp(priv.N) != 0 {
		t.Error("expected non-nil private key with matching modulus")
	}
}

// ── LoadPublicKey ──────────────────────────────────────────────────────────

func TestLoadPublicKey_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := auth.LoadPublicKey(filepath.Join(t.TempDir(), "nonexistent.pem"))
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadPublicKey_MalformedPEM(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(path, []byte("not valid PEM content at all"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := auth.LoadPublicKey(path)
	if err == nil {
		t.Error("expected error for malformed PEM, got nil")
	}
}

func TestLoadPublicKey_Success(t *testing.T) {
	t.Parallel()

	priv := generateRSAKey(t)
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	path := writeTempPEM(t, "PUBLIC KEY", pubDER)

	got, err := auth.LoadPublicKey(path)
	if err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}
	if got == nil || got.N.Cmp(priv.N) != 0 {
		t.Error("expected non-nil public key with matching modulus")
	}
}
