package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      uint32 = 19 * 1024
	argonIterations  uint32 = 2
	argonParallelism uint8  = 1
	argonSaltLength         = 16
	argonKeyLength   uint32 = 32
)

var (
	ErrInvalidPasswordHash = errors.New("invalid password hash")
	ErrPasswordTooShort    = errors.New("password must be at least 12 characters")
	ErrPasswordTooLong     = errors.New("password must be at most 128 characters")
)

func ValidatePassword(password string) error {
	count := utf8.RuneCountInString(password)
	if count < 12 {
		return ErrPasswordTooShort
	}
	if count > 128 || len(password) > 512 {
		return ErrPasswordTooLong
	}
	return nil
}

func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func VerifyPassword(password, encoded string) (match bool, needsRehash bool, err error) {
	memory, iterations, parallelism, salt, expected, err := decodePasswordHash(encoded)
	if err != nil {
		return false, false, err
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	match = subtle.ConstantTimeCompare(actual, expected) == 1
	needsRehash = memory != argonMemory ||
		iterations != argonIterations ||
		parallelism != argonParallelism ||
		len(expected) != int(argonKeyLength)
	return match, needsRehash, nil
}

func decodePasswordHash(encoded string) (memory uint32, iterations uint32, parallelism uint8, salt, hash []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	m, err := parseUintParam(params[0], "m=", 32)
	if err != nil || m == 0 {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	it, err := parseUintParam(params[1], "t=", 32)
	if err != nil || it == 0 {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	p, err := parseUintParam(params[2], "p=", 8)
	if err != nil || p == 0 {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) < 16 {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	return uint32(m), uint32(it), uint8(p), salt, hash, nil
}

func parseUintParam(v, prefix string, bits int) (uint64, error) {
	if !strings.HasPrefix(v, prefix) {
		return 0, ErrInvalidPasswordHash
	}
	return strconv.ParseUint(strings.TrimPrefix(v, prefix), 10, bits)
}
