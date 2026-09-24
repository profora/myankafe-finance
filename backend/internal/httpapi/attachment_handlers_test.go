package httpapi

import "testing"

func TestValidatePreviewAttachmentContent(t *testing.T) {
	for _, tc := range []struct {
		name     string
		declared string
		detected string
		ok       bool
	}{
		{"jpeg", "image/jpeg", "image/jpeg", true},
		{"pdf", "application/pdf", "application/pdf", true},
		{"csv-as-text", "text/csv", "text/plain", true},
		{"spoofed-image", "image/jpeg", "text/html", false},
		{"spoofed-pdf", "application/pdf", "text/html", false},
		{"office-download", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/zip", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePreviewAttachmentContent(tc.declared, tc.detected)
			if tc.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("expected mismatch rejection")
			}
		})
	}
}

func TestSafeAttachmentFilename(t *testing.T) {
	if got := safeAttachmentFilename("../../receipt?draft.jpg"); got != "receipt_draft.jpg" {
		t.Fatalf("got %q", got)
	}
	long := ""
	for range 200 {
		long += "က"
	}
	if got := safeAttachmentFilename(long); len([]rune(got)) != 180 {
		t.Fatalf("rune length=%d want 180", len([]rune(got)))
	}
}

func TestAllowedAttachmentTypeRejectsHTML(t *testing.T) {
	if allowedAttachmentType("text/html") {
		t.Fatal("HTML must not be an allowed transaction attachment")
	}
}
