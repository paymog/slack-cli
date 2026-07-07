package cmds

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paymog/slack-cli/internal/toolcall"
)

// pngFakeRaw is a minimal fake PNG header used by attachment tests.
var pngFakeRaw = []byte("\x89PNG\r\n\x1a\nfake")

// pngFakeB64 is the standard base64 encoding of pngFakeRaw.
var pngFakeB64 = base64.StdEncoding.EncodeToString(pngFakeRaw)

// unmarshalJSON is a test helper that asserts buf contains valid, parseable JSON
// and returns it as a string-keyed map.
func unmarshalJSON(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	trimmed := bytes.TrimSpace(buf.Bytes())
	var m map[string]any
	if err := json.Unmarshal(trimmed, &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw output: %s", err, buf.String())
	}
	return m
}

// TestUnitWriteAttachmentImageNoOutputRecoversBytes is the core bug-fix assertion:
// when a handler returns image bytes as an out-of-band MCP image block and the
// caller does not specify -o, the bytes must be folded back into the JSON
// envelope printed to stdout so they are not silently lost.
func TestUnitWriteAttachmentImageNoOutputRecoversBytes(t *testing.T) {
	metaJSON := `{"file_id":"F1","filename":"pic.png","mimetype":"image/png","size":12}`
	res := &toolcall.Result{
		Text:   metaJSON,
		Images: []toolcall.Blob{{Data: pngFakeB64, MIMEType: "image/png"}},
	}

	var buf bytes.Buffer
	if err := writeAttachment(&buf, res, "", false); err != nil {
		t.Fatalf("writeAttachment: %v", err)
	}

	m := unmarshalJSON(t, &buf)

	if m["encoding"] != "base64" {
		t.Errorf("encoding = %v, want base64", m["encoding"])
	}
	if m["content"] != pngFakeB64 {
		t.Errorf("content does not match original base64 blob")
	}
	// Metadata fields must be preserved.
	if m["file_id"] != "F1" {
		t.Errorf("file_id = %v, want F1", m["file_id"])
	}
	if m["filename"] != "pic.png" {
		t.Errorf("filename = %v, want pic.png", m["filename"])
	}
}

// TestUnitWriteAttachmentImageOutputWritesFile checks that when -o is given:
//   - the file on disk equals the original raw bytes,
//   - stdout carries only a small confirmation JSON (no base64 blob),
//   - the confirmation JSON includes the output path and key metadata.
func TestUnitWriteAttachmentImageOutputWritesFile(t *testing.T) {
	metaJSON := `{"file_id":"F1","filename":"pic.png","mimetype":"image/png","size":12}`
	res := &toolcall.Result{
		Text:   metaJSON,
		Images: []toolcall.Blob{{Data: pngFakeB64, MIMEType: "image/png"}},
	}

	outPath := filepath.Join(t.TempDir(), "pic.png")

	var buf bytes.Buffer
	if err := writeAttachment(&buf, res, outPath, false); err != nil {
		t.Fatalf("writeAttachment: %v", err)
	}

	// File bytes must equal the raw PNG bytes — not base64-encoded.
	fileBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(fileBytes, pngFakeRaw) {
		t.Errorf("file bytes mismatch: got %x, want %x", fileBytes, pngFakeRaw)
	}

	// Stdout must not contain the base64 blob (agent output should stay small).
	if strings.Contains(buf.String(), pngFakeB64) {
		t.Error("stdout contains base64 blob; it should only contain confirmation metadata")
	}

	// Stdout must be valid JSON containing path and key metadata fields.
	m := unmarshalJSON(t, &buf)
	if m["path"] != outPath {
		t.Errorf("confirmation path = %v, want %s", m["path"], outPath)
	}
	if m["file_id"] != "F1" {
		t.Errorf("confirmation file_id = %v, want F1", m["file_id"])
	}
	if m["mimetype"] != "image/png" {
		t.Errorf("confirmation mimetype = %v, want image/png", m["mimetype"])
	}
}

// TestUnitWriteAttachmentNonImageBinaryOutput checks that a binary file
// delivered via the base64 envelope (not an MCP image block) is also
// correctly decoded to raw bytes when written to disk.
func TestUnitWriteAttachmentNonImageBinaryOutput(t *testing.T) {
	raw := []byte("\x89PNG\r\n\x1a\nfake")
	b64 := base64.StdEncoding.EncodeToString(raw)
	textJSON := `{"file_id":"F2","filename":"bin.dat","mimetype":"application/octet-stream","size":12,"encoding":"base64","content":"` + b64 + `"}`

	res := &toolcall.Result{Text: textJSON}

	outPath := filepath.Join(t.TempDir(), "bin.dat")

	var buf bytes.Buffer
	if err := writeAttachment(&buf, res, outPath, false); err != nil {
		t.Fatalf("writeAttachment: %v", err)
	}

	fileBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(fileBytes, raw) {
		t.Errorf("file bytes mismatch: got %x, want %x", fileBytes, raw)
	}
}

// TestUnitWriteAttachmentTextNoOutputUnchanged verifies that a plain-text
// attachment (encoding=="none") printed without -o keeps its content verbatim
// in the JSON envelope — no re-encoding, no data loss.
func TestUnitWriteAttachmentTextNoOutputUnchanged(t *testing.T) {
	textJSON := `{"file_id":"F3","filename":"note.txt","mimetype":"text/plain","size":5,"encoding":"none","content":"hello"}`
	res := &toolcall.Result{Text: textJSON}

	var buf bytes.Buffer
	if err := writeAttachment(&buf, res, "", false); err != nil {
		t.Fatalf("writeAttachment: %v", err)
	}

	m := unmarshalJSON(t, &buf)

	if m["content"] != "hello" {
		t.Errorf("content = %v, want hello", m["content"])
	}
	if m["encoding"] != "none" {
		t.Errorf("encoding = %v, want none", m["encoding"])
	}
}

// TestUnitInjectContentPreservesOrderAndHandlesEmpty covers two cases of
// injectContent:
//  1. A non-empty JSON object: encoding and content are appended after the
//     last existing field (order is preserved — no re-marshalling).
//  2. An empty JSON object: encoding and content are inserted without a
//     leading comma, keeping the output valid JSON.
func TestUnitInjectContentPreservesOrderAndHandlesEmpty(t *testing.T) {
	// Case 1: non-empty object — field order preserved.
	got := injectContent(`{"file_id":"F1","size":3}`, "base64", "QUJD")
	var m map[string]any
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("injectContent non-empty: not valid JSON: %v\ngot: %s", err, got)
	}
	if m["encoding"] != "base64" || m["content"] != "QUJD" {
		t.Errorf("fields missing or wrong: %v", m)
	}
	// Order: "size" must appear before "encoding" and "content".
	sizeIdx := strings.Index(got, `"size"`)
	encIdx := strings.Index(got, `"encoding"`)
	contIdx := strings.Index(got, `"content"`)
	if sizeIdx < 0 || encIdx < 0 || contIdx < 0 {
		t.Fatalf("expected fields not found in: %s", got)
	}
	if !(sizeIdx < encIdx && encIdx < contIdx) {
		t.Errorf("field order wrong in %s: size=%d enc=%d cont=%d", got, sizeIdx, encIdx, contIdx)
	}

	// Case 2: empty object — no leading comma.
	got2 := injectContent("{}", "base64", "QUJD")
	var m2 map[string]any
	if err := json.Unmarshal([]byte(got2), &m2); err != nil {
		t.Fatalf("injectContent empty: not valid JSON: %v\ngot: %s", err, got2)
	}
	if m2["encoding"] != "base64" || m2["content"] != "QUJD" {
		t.Errorf("empty-object fields missing or wrong: %v", m2)
	}
}

// TestUnitAttachmentBytesInvalidBase64Errors asserts that attachmentBytes
// returns a non-nil error when the envelope advertises base64 encoding but
// the content field is not valid base64.
func TestUnitAttachmentBytesInvalidBase64Errors(t *testing.T) {
	textJSON := `{"file_id":"F4","filename":"bad.bin","mimetype":"application/octet-stream","size":5,"encoding":"base64","content":"!!!not-base64!!!"}`
	res := &toolcall.Result{Text: textJSON}

	_, _, err := attachmentBytes(res)
	if err == nil {
		t.Fatal("want error for invalid base64 content, got nil")
	}
}
