package accounts

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordHashMemory      uint32 = 19 * 1024 // KiB
	passwordHashIterations  uint32 = 2
	passwordHashParallelism uint8  = 1
	passwordSaltLength             = 16
	passwordKeyLength              = 32
)

func isArgon2idHash(encoded string) bool {
	return strings.HasPrefix(encoded, "$argon2id$")
}

func generatePasswordHash(password string) (string, error) {
	salt := make([]byte, passwordSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, passwordHashIterations, passwordHashMemory, passwordHashParallelism, passwordKeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		passwordHashMemory, passwordHashIterations, passwordHashParallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func verifyPasswordHash(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory, iterations uint32
	var parallelism uint8
	if n, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil || n != 3 {
		return false
	}
	// Bound parameters from the stored string before allocating memory.
	if memory < 8*1024 || memory > 64*1024 || iterations < 1 || iterations > 8 || parallelism < 1 || parallelism > 4 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != passwordSaltLength {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) != passwordKeyLength {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, passwordKeyLength)
	return subtle.ConstantTimeCompare(got, want) == 1
}
