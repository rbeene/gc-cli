package drive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportTargetFor(t *testing.T) {
	cases := []struct {
		mime       string
		wantNative bool
		wantOK     bool
		wantExt    string
	}{
		{"application/pdf", false, false, ""},
		{"image/jpeg", false, false, ""},
		{googleAppsPrefix + "document", true, true, ".pdf"},
		{googleAppsPrefix + "spreadsheet", true, true, ".pdf"},
		// A Form quiz is native but has no export format, so it must be
		// reported as undownloadable rather than attempted.
		{googleAppsPrefix + "form", true, false, ""},
		{googleAppsPrefix + "folder", true, false, ""},
	}
	for _, c := range cases {
		_, ext, native, ok := exportTargetFor(c.mime)
		if native != c.wantNative || ok != c.wantOK || ext != c.wantExt {
			t.Errorf("exportTargetFor(%q) = ext %q native %v ok %v; want ext %q native %v ok %v",
				c.mime, ext, native, ok, c.wantExt, c.wantNative, c.wantOK)
		}
	}
}

func TestSafeFileName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Worksheet 4.pdf", "Worksheet 4.pdf"},
		// Teacher-authored names routinely contain slashes; they must not be
		// able to escape the destination directory.
		{"Unit 3/Week 2 homework.pdf", "Unit 3-Week 2 homework.pdf"},
		{"  spaced  ", "spaced"},
		{"", "drive-abc123"},
		{"...", "drive-abc123"},
	}
	for _, c := range cases {
		if got := SafeFileName(c.in, "abc123"); got != c.want {
			t.Errorf("SafeFileName(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

// TestSafeFileNameContainsNoTraversal asserts the property that matters rather
// than an exact string: whatever a teacher named the file, the result must be a
// single path element that cannot walk out of the destination directory.
func TestSafeFileNameContainsNoTraversal(t *testing.T) {
	hostile := []string{
		"../../etc/passwd",
		"..",
		".",
		"/absolute/path.pdf",
		"a\\b\\c.pdf",
		"nul\x00byte.pdf",
	}
	for _, in := range hostile {
		got := SafeFileName(in, "abc123")
		if got == "" || got == "." || got == ".." {
			t.Errorf("SafeFileName(%q) = %q; must not be empty or a dot entry", in, got)
		}
		if strings.ContainsRune(got, os.PathSeparator) || strings.ContainsRune(got, '/') || strings.ContainsRune(got, '\\') {
			t.Errorf("SafeFileName(%q) = %q; must not contain a path separator", in, got)
		}
		if filepath.Base(got) != got {
			t.Errorf("SafeFileName(%q) = %q; must be a single path element", in, got)
		}
	}
}

func TestEnsureExtension(t *testing.T) {
	if got := ensureExtension("Notes", ".pdf"); got != "Notes.pdf" {
		t.Errorf("got %q", got)
	}
	if got := ensureExtension("Notes.pdf", ".pdf"); got != "Notes.pdf" {
		t.Errorf("double extension: got %q", got)
	}
	if got := ensureExtension("Notes.PDF", ".pdf"); got != "Notes.PDF" {
		t.Errorf("case-insensitive match failed: got %q", got)
	}
}
