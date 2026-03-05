package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

const appleJWKSURL = "https://appleid.apple.com/auth/keys"

// AppleVerifier validates an Apple identity token and returns the subject and email.
type AppleVerifier interface {
	Verify(ctx context.Context, identityToken string) (appleSub, email string, err error)
}

// ── Real verifier ─────────────────────────────────────────────────────────────

type realAppleVerifier struct {
	cache *jwk.Cache
}

func NewAppleVerifier(ctx context.Context) (AppleVerifier, error) {
	cache := jwk.NewCache(ctx)
	if err := cache.Register(appleJWKSURL); err != nil {
		return nil, fmt.Errorf("register apple jwks: %w", err)
	}
	// Warm the cache.
	if _, err := cache.Refresh(ctx, appleJWKSURL); err != nil {
		return nil, fmt.Errorf("warm apple jwks cache: %w", err)
	}
	return &realAppleVerifier{cache: cache}, nil
}

func (v *realAppleVerifier) Verify(ctx context.Context, identityToken string) (string, string, error) {
	keySet, err := v.cache.Get(ctx, appleJWKSURL)
	if err != nil {
		return "", "", fmt.Errorf("get apple jwks: %w", err)
	}

	token, err := jwt.Parse([]byte(identityToken),
		jwt.WithKeySet(keySet),
		jwt.WithValidate(true),
		jwt.WithIssuer("https://appleid.apple.com"),
	)
	if err != nil {
		return "", "", fmt.Errorf("verify apple token: %w", err)
	}

	sub := token.Subject()
	email, _ := token.Get("email")
	emailStr, _ := email.(string)
	return sub, emailStr, nil
}

// ── Mock verifier (development/testing) ──────────────────────────────────────

// MockAppleVerifier accepts any token of the form "mock:<apple_sub>:<email>"
// and returns the embedded sub and email. Useful in development without
// real Apple credentials.
type MockAppleVerifier struct{}

func NewMockAppleVerifier() AppleVerifier {
	return &MockAppleVerifier{}
}

func (m *MockAppleVerifier) Verify(_ context.Context, identityToken string) (string, string, error) {
	// Expected format: "mock:<apple_sub>:<email>"
	parts := strings.SplitN(identityToken, ":", 3)
	if len(parts) != 3 || parts[0] != "mock" {
		return "", "", fmt.Errorf("mock verifier expects token format 'mock:<sub>:<email>', got %q", identityToken)
	}
	return parts[1], parts[2], nil
}
