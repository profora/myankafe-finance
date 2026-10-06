package ids

import (
	"regexp"
	"testing"
)

func TestULIDCanonicalShape(t *testing.T) {
	id, err := ULID()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 26 {
		t.Fatalf("length=%d id=%q", len(id), id)
	}
	if !regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`).MatchString(id) {
		t.Fatalf("not canonical ULID: %q", id)
	}
}

func TestUUIDv7Shape(t *testing.T) {
	id, err := UUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(id) {
		t.Fatalf("not UUIDv7-shaped: %q", id)
	}
}
