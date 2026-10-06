package auth

import (
	"errors"
	"regexp"
	"strings"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)

var ErrInvalidUsername = errors.New("username must be 3-64 characters and use lowercase letters, numbers, '.', '_' or '-'")

func NormalizeUsername(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	if !usernamePattern.MatchString(v) {
		return "", ErrInvalidUsername
	}
	return v, nil
}
