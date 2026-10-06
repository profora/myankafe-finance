package postgres

import "testing"

func TestAuditPayloadDropsSensitiveKeys(t *testing.T) {
	out := auditPayload(map[string]any{
		"password":      "not-stored",
		"user_agent":    "browser",
		"authorization": "Bearer secret",
		"cookie":        "session",
		"role":          "OWNER",
		"journal_id":    "journal",
		"ip":            "127.0.0.1",
	})
	for _, key := range []string{"password", "user_agent", "authorization", "cookie"} {
		if _, ok := out[key]; ok {
			t.Fatalf("sensitive key %s was kept", key)
		}
	}
	if out["role"] != "OWNER" || out["journal_id"] != "journal" || out["ip"] != "127.0.0.1" {
		t.Fatalf("business facts were dropped: %+v", out)
	}
}
