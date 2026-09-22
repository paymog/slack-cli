package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Print writes a tool's text result as JSON by default.
//
// Upstream tool handlers return one of three shapes: a JSON document, a CSV
// table (gocsv), or a plain-text status line ("Successfully posted ..."). The
// CLI normalizes every non-empty result to valid JSON:
//   - Valid JSON is re-indented (field order preserved).
//   - When tabular is set, CSV is parsed into a JSON array of row objects.
//   - Everything else becomes {"message":"..."}.
//
// raw bypasses all of this and writes the handler output byte-for-byte.
// Empty output prints nothing.
//
// tabular is opt-in per command (see internal/cmds): only handlers known to
// emit a CSV table set it, so plain-text and binary handler outputs are never
// fed to the CSV parser.
func Print(w io.Writer, text string, raw, tabular bool) error {
	if raw {
		_, err := io.WriteString(w, text)
		return err
	}

	trimmed := strings.TrimRight(text, "\n")
	if trimmed == "" {
		return nil
	}

	// Already JSON (e.g. usergroups create/update, me join/leave): re-indent.
	if json.Valid([]byte(trimmed)) {
		return printJSON(w, []byte(trimmed))
	}

	// Tabular handlers emit CSV; convert to a JSON array of objects. Output
	// that is not a real table (a single-column status line) falls through.
	if tabular {
		if jsonBytes, ok := csvToJSON(trimmed); ok {
			return printJSON(w, jsonBytes)
		}
	}

	return WriteJSON(w, struct {
		Message string `json:"message"`
	}{Message: trimmed})
}

// WriteJSON encodes a successful command result using the CLI's default
// machine-readable output contract.
func WriteJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

// printJSON writes jsonBytes re-indented with two spaces and a trailing
// newline. Indentation preserves the source field order (unlike a
// decode/re-marshal round-trip, which would sort object keys).
func printJSON(w io.Writer, jsonBytes []byte) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, jsonBytes, "", "  "); err != nil {
		buf.Reset()
		buf.Write(jsonBytes)
	}
	_, err := fmt.Fprintln(w, buf.String())
	return err
}

// csvToJSON parses gocsv-style CSV (a header row followed by data rows) into a
// JSON array of string-keyed objects, preserving column order. It reports
// ok=false when text is not a real table so the caller can fall back to
// printing it verbatim:
//   - unparseable as CSV, or
//   - a single-column header — gocsv always emits a multi-column header for a
//     struct slice, so one column means a plain-text line (e.g. an empty-result
//     message), not a table.
//
// Values stay strings: CSV carries no type information, and naive coercion
// would corrupt Slack data (message timestamps like "1772680334.954409" must
// not become floats). Consumers can `tonumber` in jq when they need numbers.
func csvToJSON(text string) ([]byte, bool) {
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1 // tolerate ragged rows; gocsv won't produce them
	records, err := r.ReadAll()
	if err != nil || len(records) == 0 {
		return nil, false
	}

	header := records[0]
	if len(header) < 2 {
		return nil, false
	}

	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, row := range records[1:] {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('{')
		for j, key := range header {
			if j > 0 {
				buf.WriteByte(',')
			}
			writeJSONString(&buf, key)
			buf.WriteByte(':')
			val := ""
			if j < len(row) {
				val = row[j]
			}
			writeJSONString(&buf, val)
		}
		buf.WriteByte('}')
	}
	buf.WriteByte(']')
	return buf.Bytes(), true
}

// writeJSONString appends s as a JSON-encoded string with HTML escaping off, so
// the angle brackets and ampersands in Slack links/mentions (<@U123>, <http..>,
// &amp;) stay legible instead of becoming \u003c/\u0026. Encoding a string never
// fails, so the error is unreachable; Encode appends a newline we trim.
func writeJSONString(buf *bytes.Buffer, s string) {
	var tmp bytes.Buffer
	enc := json.NewEncoder(&tmp)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	buf.Write(bytes.TrimRight(tmp.Bytes(), "\n"))
}
