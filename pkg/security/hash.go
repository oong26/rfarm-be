// Package security provides pure cryptographic utilities.
// It does not manage sessions, tokens in storage, or any stateful resources.
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Hasher hashes a plaintext password and verifies it against an encoded hash.
// Implementations must be pure: no I/O, no side effects beyond salt generation.
type Hasher interface {
	Hash(password string) (encoded string, err error)
	Verify(password, encoded string) (match bool, err error)
}

// Argon2idParams holds the cost parameters for Argon2id hashing.
type Argon2idParams struct {
	Memory      uint32 // KiB
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

// DefaultArgon2idParams follows OWASP recommendations for Argon2id.
var DefaultArgon2idParams = Argon2idParams{
	Memory:      64 * 1024, // 64 MiB
	Iterations:  3,
	Parallelism: 2,
	SaltLen:     16,
	KeyLen:      32,
}

type argon2idHasher struct{ p Argon2idParams }

// NewArgon2idHasher returns a Hasher backed by Argon2id with the given params.
func NewArgon2idHasher(p Argon2idParams) Hasher {
	return &argon2idHasher{p: p}
}

// NewDefaultHasher returns a Hasher using DefaultArgon2idParams.
func NewDefaultHasher() Hasher {
	return NewArgon2idHasher(DefaultArgon2idParams)
}

func (h *argon2idHasher) Hash(password string) (string, error) {
	salt := make([]byte, h.p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("security: generate salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, h.p.Iterations, h.p.Memory, h.p.Parallelism, h.p.KeyLen)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.p.Memory,
		h.p.Iterations,
		h.p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
	return encoded, nil
}

func (h *argon2idHasher) Verify(password, encoded string) (bool, error) {
	p, salt, hash, err := decodeArgon2idHash(encoded)
	if err != nil {
		return false, err
	}

	derived := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLen)
	return subtle.ConstantTimeCompare(hash, derived) == 1, nil
}

// decodeArgon2idHash parses a PHC-formatted Argon2id string.
func decodeArgon2idHash(encoded string) (p Argon2idParams, salt, hash []byte, err error) {
	parts := strings.Split(encoded, "$")
	// expected: ["", "argon2id", "v=N", "m=N,t=N,p=N", "<salt>", "<hash>"]
	if len(parts) != 6 {
		return p, nil, nil, errors.New("security: invalid argon2id hash format")
	}

	var version int
	if _, err = fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return p, nil, nil, fmt.Errorf("security: parse argon2id version: %w", err)
	}
	if version != argon2.Version {
		return p, nil, nil, fmt.Errorf("security: unsupported argon2id version %d", version)
	}

	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return p, nil, nil, fmt.Errorf("security: parse argon2id params: %w", err)
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return p, nil, nil, fmt.Errorf("security: decode argon2id salt: %w", err)
	}
	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return p, nil, nil, fmt.Errorf("security: decode argon2id hash: %w", err)
	}

	p.SaltLen = uint32(len(salt))
	p.KeyLen = uint32(len(hash))
	return p, salt, hash, nil
}
