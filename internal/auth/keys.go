// Package auth handles RSA key loading and JWT token operations.
package auth

import (
	"crypto/rsa"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

// LoadPrivateKey reads and parses an RSA private key from a PEM file.
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from validated config
	if err != nil {
		return nil, fmt.Errorf("load private key from %q: %w", path, err)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("load private key from %q: %w", path, err)
	}
	return key, nil
}

// LoadPublicKey reads and parses an RSA public key from a PEM file.
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from validated config
	if err != nil {
		return nil, fmt.Errorf("load public key from %q: %w", path, err)
	}
	key, err := jwt.ParseRSAPublicKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("load public key from %q: %w", path, err)
	}
	return key, nil
}
