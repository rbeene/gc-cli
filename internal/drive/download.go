package drive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ErrNotDownloadable marks a Drive item that has no byte stream to fetch:
// folders, shortcuts and Google Forms. Callers report these rather than
// failing the whole run, because a single assignment often mixes a PDF
// worksheet with a Form quiz.
var ErrNotDownloadable = errors.New("drive item has no downloadable content")

const googleAppsPrefix = "application/vnd.google-apps."

// DownloadResult describes one file written to disk.
type DownloadResult struct {
	FileID   string `json:"file_id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	MimeType string `json:"mime_type"`
	Bytes    int64  `json:"bytes"`
	// Exported is true when the source was a Google-native document that had
	// to be converted (to PDF) rather than downloaded byte-for-byte.
	Exported bool `json:"exported"`
}

// exportTargets maps Google-native types to the format we convert them to.
// Everything printable becomes PDF: this tool exists to put paper in a kid's
// hand, not to round-trip editable documents.
var exportTargets = map[string]struct {
	MimeType  string
	Extension string
}{
	googleAppsPrefix + "document":     {"application/pdf", ".pdf"},
	googleAppsPrefix + "presentation": {"application/pdf", ".pdf"},
	googleAppsPrefix + "drawing":      {"application/pdf", ".pdf"},
	googleAppsPrefix + "spreadsheet":  {"application/pdf", ".pdf"},
	googleAppsPrefix + "script":       {"application/json", ".json"},
}

// exportTargetFor reports how a Google-native mime type should be converted.
// A native type with no export target (folder, form, shortcut) is not
// downloadable at all.
func exportTargetFor(mimeType string) (mime string, ext string, native bool, ok bool) {
	if !strings.HasPrefix(mimeType, googleAppsPrefix) {
		return "", "", false, false
	}
	target, found := exportTargets[mimeType]
	if !found {
		return "", "", true, false
	}
	return target.MimeType, target.Extension, true, true
}

// DownloadFile writes the contents of a Drive file into destDir and returns
// where it landed. Google-native documents are exported to PDF; binary files
// (the attached worksheet PDFs this tool is built for) are streamed verbatim.
func (c *Client) DownloadFile(ctx context.Context, fileID, destDir string) (*DownloadResult, error) {
	meta, err := c.svc.Files.Get(fileID).SupportsAllDrives(true).Fields("id,name,mimeType,size").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get file metadata %s: %w", fileID, err)
	}

	exportMime, exportExt, native, exportable := exportTargetFor(meta.MimeType)
	if native && !exportable {
		return nil, fmt.Errorf("%w: %s is %s", ErrNotDownloadable, meta.Name, meta.MimeType)
	}

	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return nil, fmt.Errorf("create destination %s: %w", destDir, err)
	}

	var body io.ReadCloser
	if native {
		resp, err := c.svc.Files.Export(fileID, exportMime).Context(ctx).Download()
		if err != nil {
			return nil, fmt.Errorf("export file %s as %s: %w", fileID, exportMime, err)
		}
		body = resp.Body
	} else {
		resp, err := c.svc.Files.Get(fileID).SupportsAllDrives(true).Context(ctx).Download()
		if err != nil {
			return nil, fmt.Errorf("download file %s: %w", fileID, err)
		}
		body = resp.Body
	}
	defer body.Close()

	name := SafeFileName(meta.Name, fileID)
	if native {
		name = ensureExtension(name, exportExt)
	}
	path := filepath.Join(destDir, name)

	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", path, err)
	}
	written, copyErr := io.Copy(out, body)
	closeErr := out.Close()
	if copyErr != nil {
		return nil, fmt.Errorf("write %s: %w", path, copyErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close %s: %w", path, closeErr)
	}

	result := &DownloadResult{
		FileID:   meta.Id,
		Name:     meta.Name,
		Path:     path,
		MimeType: meta.MimeType,
		Bytes:    written,
		Exported: native,
	}
	if native {
		result.MimeType = exportMime
	}
	return result, nil
}

// SafeFileName strips anything that could escape destDir or confuse a shell.
// Drive names are teacher-authored free text and routinely contain slashes.
func SafeFileName(name, fallback string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r == os.PathSeparator:
			b.WriteRune('-')
		case r < 0x20 || r == 0x7f:
			// drop control characters
		case unicode.IsPrint(r):
			b.WriteRune(r)
		}
	}
	cleaned := strings.Trim(strings.TrimSpace(b.String()), ".")
	if cleaned == "" {
		return "drive-" + fallback
	}
	if len(cleaned) > 120 {
		cleaned = strings.TrimSpace(cleaned[:120])
	}
	return cleaned
}

func ensureExtension(name, ext string) string {
	if ext == "" || strings.EqualFold(filepath.Ext(name), ext) {
		return name
	}
	return name + ext
}
