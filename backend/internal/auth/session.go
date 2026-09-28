package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
)

const sessionTokenBytes = 32

var ErrInvalidSessionToken = errors.New("invalid session token")

func NewSessionToken() (raw string, hash []byte, err error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256(buf)
	return raw, sum[:], nil
}

func HashSessionToken(raw string) ([]byte, error) {
	buf, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(buf) != sessionTokenBytes {
		return nil, ErrInvalidSessionToken
	}
	sum := sha256.Sum256(buf)
	return sum[:], nil
}

func SessionTokenFromRequest(r *http.Request, cookieName string) (string, bool) {
	if authz := strings.TrimSpace(r.Header.Get("Authorization")); authz != "" {
		const prefix = "Bearer "
		if len(authz) > len(prefix) && strings.EqualFold(authz[:len(prefix)], prefix) {
			token := strings.TrimSpace(authz[len(prefix):])
			if token != "" {
				return token, true
			}
		}
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return "", false
	}
	return cookie.Value, true
}
