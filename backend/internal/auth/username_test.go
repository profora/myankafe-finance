package auth

import "testing"

func TestNormalizeUsername(t *testing.T) {
	got, err := NormalizeUsername("  Owner.Admin ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "owner.admin" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeUsernameRejectsInvalid(t *testing.T) {
	for _, v := range []string{"ab", "UPPER SPACE", "bad/user"} {
		if _, err := NormalizeUsername(v); err == nil {
			t.Fatalf("expected %q invalid", v)
		}
	}
}
