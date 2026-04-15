package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the JWT payload for access tokens.
type Claims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

// JWTService signs and verifies RS256 access tokens and generates opaque
// refresh tokens.
type JWTService struct {
	priv        *rsa.PrivateKey
	pub         *rsa.PublicKey
	accessTTL   time.Duration
	refreshDays int
}

// NewJWTService creates a JWTService with the given RSA key pair and config.
func NewJWTService(priv *rsa.PrivateKey, pub *rsa.PublicKey, accessTTL time.Duration, refreshDays int) *JWTService {
	return &JWTService{
		priv:        priv,
		pub:         pub,
		accessTTL:   accessTTL,
		refreshDays: refreshDays,
	}
}

// IssueAccessToken signs a new RS256 JWT for the given user.
// Returns (signedToken, jti, expiresAt, error).
func (s *JWTService) IssueAccessToken(userID uuid.UUID, role string) (string, string, time.Time, error) {
	jti := uuid.New().String()
	now := time.Now()
	expiresAt := now.Add(s.accessTTL)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
			ID:        jti,
		},
		Role: role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(s.priv)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("issue access token: %w", err)
	}
	return signed, jti, expiresAt, nil
}

// IssueRefreshToken generates a cryptographically random 32-byte token
// encoded as base64url. This is the raw token returned to the client.
// Store sha256(token) in Redis — never the raw value.
func (s *JWTService) IssueRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("issue refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// RefreshTTL returns the configured refresh token lifetime as time.Duration.
func (s *JWTService) RefreshTTL() time.Duration {
	return time.Duration(s.refreshDays) * 24 * time.Hour
}

// Verify parses and validates a signed access token string.
// Returns typed *Claims on success.
// Algorithm check is mandatory: non-RSA algorithms are rejected immediately.
func (s *JWTService) Verify(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.pub, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("token expired: %w", err)
		}
		return nil, fmt.Errorf("verify token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("verify token: invalid claims")
	}
	return claims, nil
}

// HashToken returns the lowercase hex SHA-256 of the input string.
// Use this to transform a raw refresh token into its Redis storage key.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum)
}
