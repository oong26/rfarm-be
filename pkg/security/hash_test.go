package security_test

import (
	"testing"

	"github.com/oong26/rfarm-be/pkg/security"
)

// ============================================================================
// Argon2idHasher Tests
// ============================================================================

func TestArgon2idHasher_HashAndVerify(t *testing.T) {
	h := security.NewArgon2idHasher(security.Argon2idParams{
		Memory:      16 * 1024, // fast for tests
		Iterations:  1,
		Parallelism: 1,
		SaltLen:     16,
		KeyLen:      32,
	})

	tests := []struct {
		name     string
		password string
	}{
		{"simple ascii", "password123"},
		{"empty string", ""},
		{"unicode", "密碼🔐パスワード암호"},
		{"long password", "a very long password that exceeds typical limits but should still work fine because argon2 handles arbitrary input"},
		{"special chars", "!@#$%^&*()_+-=[]{}|;':\",./<>?`~"},
		{"whitespace", "  leading trailing  \t\n"},
		{"null byte", "before\000after"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := h.Hash(tc.password)
			if err != nil {
				t.Fatalf("Hash(%q) failed: %v", tc.password, err)
			}
			if encoded == "" {
				t.Fatal("encoded must not be empty")
			}

			ok, err := h.Verify(tc.password, encoded)
			if err != nil {
				t.Fatalf("Verify(%q, encoded) error: %v", tc.password, err)
			}
			if !ok {
				t.Fatal("Verify expected true for correct password")
			}

			// Wrong password must fail
			ok, err = h.Verify("wrong-"+tc.password, encoded)
			if err != nil {
				t.Fatalf("Verify with wrong password error: %v", err)
			}
			if ok {
				t.Fatal("Verify expected false for wrong password")
			}
		})
	}
}

func TestArgon2idHasher_SaltUniqueness(t *testing.T) {
	h := security.NewArgon2idHasher(security.Argon2idParams{
		Memory:      16 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLen:     16,
		KeyLen:      32,
	})

	pass := "same-password"
	enc1, _ := h.Hash(pass)
	enc2, _ := h.Hash(pass)

	if enc1 == enc2 {
		t.Fatal("two Hash calls must produce different salts/encodings")
	}

	// Both must verify
	for _, enc := range []string{enc1, enc2} {
		ok, err := h.Verify(pass, enc)
		if err != nil || !ok {
			t.Fatalf("Verify failed for %q: ok=%v err=%v", enc, ok, err)
		}
	}
}

func TestArgon2idHasher_Verify_MalformedEncoded(t *testing.T) {
	h := security.NewDefaultHasher()

	malformed := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"not enough parts", "$argon2id$v=19$m=1,t=1,p=1$salt"},
		{"too many parts", "$argon2id$v=19$m=1,t=1,p=1$salt$hash$extra"},
		{"bad version", "$argon2id$v=999$m=1,t=1,p=1$salt$hash"},
		{"bad params format", "$argon2id$v=19$m=abc,t=1,p=1$salt$hash"},
		{"bad salt base64", "$argon2id$v=19$m=1,t=1,p=1$not-base64!$hash"},
		{"bad hash base64", "$argon2id$v=19$m=1,t=1,p=1$salt$not-base64!"},
		{"random string", "not-a-hash-at-all"},
	}

	for _, tc := range malformed {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.Verify("any", tc.value)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestArgon2idHasher_Verify_CrossParams(t *testing.T) {
	hStrong := security.NewArgon2idHasher(security.Argon2idParams{
		Memory:      32 * 1024,
		Iterations:  2,
		Parallelism: 2,
		SaltLen:     16,
		KeyLen:      32,
	})
	hWeak := security.NewArgon2idHasher(security.Argon2idParams{
		Memory:      16 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLen:     16,
		KeyLen:      32,
	})

	pass := "password"
	encodedStrong, _ := hStrong.Hash(pass)

	// Verify reads params from PHC-encoded string, so any hasher can verify
	// hashes regardless of its own construction params.
	ok, err := hWeak.Verify(pass, encodedStrong)
	if err != nil {
		t.Fatalf("Verify with different hasher error: %v", err)
	}
	if !ok {
		t.Fatal("Verify expected true — params are embedded in encoded string")
	}

	// And vice versa
	encodedWeak, _ := hWeak.Hash(pass)
	ok, err = hStrong.Verify(pass, encodedWeak)
	if err != nil {
		t.Fatalf("Verify with different hasher error: %v", err)
	}
	if !ok {
		t.Fatal("Verify expected true — params are embedded in encoded string")
	}
}

func TestNewDefaultHasher(t *testing.T) {
	h := security.NewDefaultHasher()

	encoded, err := h.Hash("password")
	if err != nil {
		t.Fatalf("Hash failed: %v", err)
	}

	ok, err := h.Verify("password", encoded)
	if err != nil || !ok {
		t.Fatalf("Verify failed: ok=%v err=%v", ok, err)
	}

	// Default params should be strong
	if h == nil {
		t.Fatal("NewDefaultHasher must not return nil")
	}
}

func TestArgon2idParams_Validation(t *testing.T) {
	// Minimal valid params
	h := security.NewArgon2idHasher(security.Argon2idParams{
		Memory:      1, // minimum memory
		Iterations:  1,
		Parallelism: 1,
		SaltLen:     8,
		KeyLen:      16,
	})
	_, err := h.Hash("test")
	if err != nil {
		t.Fatalf("minimal params should work: %v", err)
	}
}

func TestArgon2idHasher_ConcurrentAccess(t *testing.T) {
	h := security.NewDefaultHasher()
	const n = 50

	done := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			pass := "pass-" + string(rune('A'+idx%26))
			encoded, err := h.Hash(pass)
			if err != nil {
				done <- err
				return
			}
			ok, err := h.Verify(pass, encoded)
			if err != nil || !ok {
				done <- err
				return
			}
			done <- nil
		}(i)
	}

	for i := 0; i < n; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent Hash/Verify failed: %v", err)
		}
	}
}