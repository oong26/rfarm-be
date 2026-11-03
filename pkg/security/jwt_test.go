package security_test

import (
	"strings"
	"testing"
	"time"

	"github.com/oong26/rfarm-be/pkg/security"
)

// ============================================================================
// JWT/TokenManager Tests
// ============================================================================

func TestNewJWTManager_EmptyKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"empty", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mgr, err := security.NewJWTManager(tc.key)
			if err == nil {
				t.Fatal("expected error for empty key")
			}
			if mgr != nil {
				t.Fatal("manager must be nil on error")
			}
		})
	}
}

func TestTokenManager_SignAndParse_Basic(t *testing.T) {
	mgr, err := security.NewJWTManager("my-very-secure-secret-key-for-jwt-testing")
	if err != nil {
		t.Fatalf("NewJWTManager failed: %v", err)
	}

	tests := []struct {
		name    string
		subject string
		ttl     time.Duration
	}{
		{"short ttl", "user-1", 1 * time.Minute},
		{"long ttl", "user-uuid-1234", 24 * time.Hour},
		{"numeric id", "123456", 1 * time.Hour},
		{"uuid-like", "550e8400-e29b-41d4-a716-446655440000", 1 * time.Hour},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, err := mgr.Sign(tc.subject, tc.ttl)
			if err != nil {
				t.Fatalf("Sign failed: %v", err)
			}
			if token == "" {
				t.Fatal("token must not be empty")
			}
			// JWT format: header.payload.signature
			parts := strings.Split(token, ".")
			if len(parts) != 3 {
				t.Fatalf("expected 3 parts, got %d: %s", len(parts), token)
			}

			claims, err := mgr.Parse(token)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			if claims.Subject != tc.subject {
				t.Fatalf("expected subject %q, got %q", tc.subject, claims.Subject)
			}
			if claims.IssuedAt == nil {
				t.Fatal("IssuedAt must be set")
			}
			if claims.ExpiresAt == nil {
				t.Fatal("ExpiresAt must be set")
			}
			if claims.NotBefore == nil {
				t.Fatal("NotBefore must be set")
			}
		})
	}
}

func TestTokenManager_Sign_InvalidInputs(t *testing.T) {
	mgr, _ := security.NewJWTManager("valid-key-1234567890-valid-key-12345678")

	tests := []struct {
		name    string
		subject string
		ttl     time.Duration
	}{
		{"empty subject", "", 1 * time.Hour},
		{"zero ttl", "user-1", 0},
		{"negative ttl", "user-1", -1 * time.Second},
		{"negative large ttl", "user-1", -24 * time.Hour},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mgr.Sign(tc.subject, tc.ttl)
			if err == nil {
				t.Fatalf("expected error for subject=%q ttl=%s", tc.subject, tc.ttl)
			}
		})
	}
}

func TestTokenManager_Parse_InvalidInputs(t *testing.T) {
	mgr, _ := security.NewJWTManager("valid-key-1234567890-valid-key-12345678")

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"random string", "not-a-jwt"},
		{"two parts", "a.b"},
		{"four parts", "a.b.c.d"},
		{"invalid base64", "!!!.!!!.!!!"},
		{"garbage with dots", "garbage.token.here"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mgr.Parse(tc.token)
			if err == nil {
				t.Fatalf("expected error for %q", tc.token)
			}
		})
	}
}

func TestTokenManager_WrongKey(t *testing.T) {
	mgrA, _ := security.NewJWTManager("secret-key-a-aaaaaaaaaaaaaaaaaaaaaaa")
	mgrB, _ := security.NewJWTManager("secret-key-b-aaaaaaaaaaaaaaaaaaaaaaa")

	token, err := mgrA.Sign("user-123", 1*time.Hour)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	_, err = mgrB.Parse(token)
	if err == nil {
		t.Fatal("expected error when parsing with different key")
	}
}

func TestTokenManager_TamperedToken(t *testing.T) {
	mgr, _ := security.NewJWTManager("secret-key-for-tamper-test-12345678")
	token, _ := mgr.Sign("user-1", 1*time.Hour)

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("bad token format")
	}

	tests := []struct {
		name   string
		tamper func([]string) string
	}{
		{"tampered payload", func(p []string) string { return p[0] + ".eHgw. " + p[2] }},
		{"tampered signature", func(p []string) string { return p[0] + "." + p[1] + ".invalidsignature" }},
		{"replaced header algo", func(p []string) string {
			// Change alg to none is not straightforward, but just corrupt
			return "eyJhbGciOiJub25lIn0." + p[1] + "."
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tampered := tc.tamper(parts)
			_, err := mgr.Parse(tampered)
			if err == nil {
				t.Fatal("expected error for tampered token")
			}
		})
	}
}

func TestTokenManager_ExpiredToken(t *testing.T) {
	mgr, _ := security.NewJWTManager("secret-key-for-expiry-test-12345678")
	token, err := mgr.Sign("user-1", 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	time.Sleep(15 * time.Millisecond)

	_, err = mgr.Parse(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestTokenManager_NotBefore(t *testing.T) {
	mgr, _ := security.NewJWTManager("secret-key-for-nbf-test-1234567890")

	// Create token with nbf in future (1 hour from now) - we can't control nbf via API
	// but we can sign normally and verify nbf is set; for future nbf we'd need custom claims
	// Here just verify nbf is set
	token, err := mgr.Sign("user-1", 1*time.Hour)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	claims, err := mgr.Parse(token)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if claims.NotBefore == nil {
		t.Fatal("NotBefore must be set")
	}
	if claims.NotBefore.After(time.Now().Add(time.Second)) {
		t.Fatal("NotBefore should not be in the future")
	}
}

func TestTokenManager_ClaimsStructure(t *testing.T) {
	mgr, _ := security.NewJWTManager("secret-key-for-claims-test-12345678")
	sub := "test-subject-123"
	ttl := 30 * time.Minute

	token, _ := mgr.Sign(sub, ttl)
	claims, err := mgr.Parse(token)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if claims.Subject != sub {
		t.Fatalf("Subject mismatch: got %q", claims.Subject)
	}

	// Check expiration is roughly correct (within 1 second tolerance)
	expectedExp := time.Now().Add(ttl)
	diff := claims.ExpiresAt.Time.Sub(expectedExp)
	if diff < -time.Second || diff > time.Second {
		t.Fatalf("ExpiresAt not within tolerance: diff=%s", diff)
	}

	// Check issued at is recent
	diffIat := time.Since(claims.IssuedAt.Time)
	if diffIat < 0 || diffIat > time.Second {
		t.Fatalf("IssuedAt not recent: diff=%s", diffIat)
	}
}

func TestTokenManager_InterfaceCompliance(t *testing.T) {
	mgr, _ := security.NewJWTManager("secret-key-for-interface-test-123456")

	var _ security.TokenSigner = mgr
	var _ security.TokenParser = mgr
	var _ security.TokenManager = mgr

	signer := mgr.(security.TokenSigner)
	parser := mgr.(security.TokenParser)

	token, err := signer.Sign("user-x", 10*time.Minute)
	if err != nil {
		t.Fatalf("signer.Sign failed: %v", err)
	}

	claims, err := parser.Parse(token)
	if err != nil {
		t.Fatalf("parser.Parse failed: %v", err)
	}
	if claims.Subject != "user-x" {
		t.Fatalf("unexpected subject: %q", claims.Subject)
	}
}

func TestTokenManager_ConcurrentAccess(t *testing.T) {
	mgr, _ := security.NewJWTManager("secret-key-for-concurrent-test-12345678")
	const n = 50

	done := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			sub := string(rune('a' + idx%26))
			token, err := mgr.Sign(sub, time.Hour)
			if err != nil {
				done <- err
				return
			}
			claims, err := mgr.Parse(token)
			if err != nil {
				done <- err
				return
			}
			if claims.Subject != sub {
				done <- err
				return
			}
			done <- nil
		}(i)
	}

	for i := 0; i < n; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent Sign/Parse failed: %v", err)
		}
	}
}

func TestTokenManager_MultipleManagersIndependent(t *testing.T) {
	mgr1, _ := security.NewJWTManager("key-one-aaaaaaaaaaaaaaaaaaaaaaaaaaa")
	mgr2, _ := security.NewJWTManager("key-two-aaaaaaaaaaaaaaaaaaaaaaaaaaa")

	token1, _ := mgr1.Sign("user-1", 1*time.Hour)
	token2, _ := mgr2.Sign("user-2", 1*time.Hour)

	// Each manager can parse its own token
	if _, err := mgr1.Parse(token1); err != nil {
		t.Fatalf("mgr1 cannot parse own token: %v", err)
	}
	if _, err := mgr2.Parse(token2); err != nil {
		t.Fatalf("mgr2 cannot parse own token: %v", err)
	}

	// Cross-verify must fail
	if _, err := mgr1.Parse(token2); err == nil {
		t.Fatal("mgr1 must not parse mgr2 token")
	}
	if _, err := mgr2.Parse(token1); err == nil {
		t.Fatal("mgr2 must not parse mgr1 token")
	}
}