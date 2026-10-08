package integration

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	SystemMyanKafePlatform = "MYANKAFE_PLATFORM"
	SecretReference        = "env:INTEGRATION_SECRET_MYANKAFE_PLATFORM"
	ReplayWindow           = 5 * time.Minute
	HeaderSystem           = "X-Integration-System"
	HeaderTimestamp        = "X-Integration-Timestamp"
	HeaderEventID          = "X-Integration-Event-ID"
	HeaderSignature        = "X-Integration-Signature"
	ReadinessEventID       = "READINESS"
)

func Canonical(method, path, timestamp string, body []byte) string {
	sum := sha256.Sum256(body)
	return strings.ToUpper(method) + "\n" + path + "\n" + timestamp + "\n" + hex.EncodeToString(sum[:])
}

func Sign(secret, canonical string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

func BodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func VerifySignature(secret, method, path, timestamp string, body []byte, presented string) error {
	presented = strings.TrimSpace(presented)
	if secret == "" || !strings.HasPrefix(presented, "v1=") {
		return errors.New("bad_signature")
	}
	got, err := hex.DecodeString(strings.TrimPrefix(presented, "v1="))
	if err != nil {
		return errors.New("bad_signature")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(Canonical(method, path, timestamp, body)))
	expected := mac.Sum(nil)
	if len(got) != len(expected) || subtle.ConstantTimeCompare(got, expected) != 1 {
		return errors.New("bad_signature")
	}
	return nil
}

func VerifyTimestamp(raw string, now time.Time) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("missing_timestamp")
	}
	sec, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return errors.New("missing_timestamp")
	}
	when := time.Unix(sec, 0)
	if when.Before(now.Add(-ReplayWindow)) || when.After(now.Add(ReplayWindow)) {
		return errors.New("stale_timestamp")
	}
	return nil
}
