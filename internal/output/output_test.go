package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func run(t *testing.T, text string, raw, tabular bool) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Print(&buf, text, raw, tabular); err != nil {
		t.Fatalf("Print: %v", err)
	}
	return buf.String()
}

func TestUnitPrintConvertsCSVToJSON(t *testing.T) {
	csv := "ID,Name,MemberCount\nC123,#general,42\nC456,#random,7\n"
	got := run(t, csv, false, true)

	var rows []map[string]string
	if err := json.Unmarshal([]byte(got), &rows); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, got)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d: %s", len(rows), got)
	}
	if rows[0]["ID"] != "C123" || rows[0]["Name"] != "#general" || rows[0]["MemberCount"] != "42" {
		t.Fatalf("row 0 wrong: %v", rows[0])
	}
	if rows[1]["Name"] != "#random" {
		t.Fatalf("row 1 wrong: %v", rows[1])
	}
}

func TestUnitPrintPreservesColumnOrder(t *testing.T) {
	got := run(t, "ID,Name,Topic\nC1,#g,hi\n", false, true)
	iID := strings.Index(got, `"ID"`)
	iName := strings.Index(got, `"Name"`)
	iTopic := strings.Index(got, `"Topic"`)
	if iID < 0 || !(iID < iName && iName < iTopic) {
		t.Fatalf("column order not preserved: %s", got)
	}
}

func TestUnitPrintCSVQuotedFields(t *testing.T) {
	// gocsv quotes fields containing commas / newlines; they must survive the
	// round-trip into a single JSON string value.
	csv := "MsgID,Text\n1.0,\"hello, world\nsecond line\"\n"
	got := run(t, csv, false, true)

	var rows []map[string]string
	if err := json.Unmarshal([]byte(got), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, got)
	}
	if rows[0]["Text"] != "hello, world\nsecond line" {
		t.Fatalf("quoted field mangled: %q", rows[0]["Text"])
	}
}

func TestUnitPrintEmptyTableIsEmptyArray(t *testing.T) {
	// A handler returning zero rows still emits a gocsv header; that is a real
	// (empty) table, not a status line, so it becomes an empty JSON array.
	if got := strings.TrimSpace(run(t, "ID,Name\n", false, true)); got != "[]" {
		t.Fatalf("want [], got %q", got)
	}
}

func TestUnitPrintSingleColumnIsVerbatim(t *testing.T) {
	// "No users found." is a one-column line, not a gocsv table; even with the
	// tabular flag set it must pass through untouched.
	msg := "No users found matching the query."
	if got := strings.TrimSpace(run(t, msg, false, true)); got != msg {
		t.Fatalf("status line altered: %q", got)
	}
}

func TestUnitPrintPlainTextVerbatim(t *testing.T) {
	msg := "Successfully posted message to channel C123 (ts=1.2)"
	if got := strings.TrimSpace(run(t, msg, false, false)); got != msg {
		t.Fatalf("plain text altered: %q", got)
	}
}

func TestUnitPrintJSONReindented(t *testing.T) {
	// Compact JSON from a handler is pretty-printed with field order preserved
	// and numbers left as numbers.
	got := run(t, `{"id":"S1","name":"eng","count":3}`, false, false)
	if !strings.Contains(got, "\n  \"id\": \"S1\"") {
		t.Fatalf("not indented: %s", got)
	}
	if !strings.Contains(got, "\"count\": 3") {
		t.Fatalf("number not preserved: %s", got)
	}
	if iID, iName := strings.Index(got, `"id"`), strings.Index(got, `"name"`); iID < 0 || iID > iName {
		t.Fatalf("JSON field order not preserved: %s", got)
	}
}

func TestUnitPrintRawIsVerbatim(t *testing.T) {
	csv := "ID,Name\nC1,#g\n"
	if got := run(t, csv, true, true); got != csv {
		t.Fatalf("raw should be byte-for-byte, got %q", got)
	}
}

func TestUnitPrintCSVNotConvertedWithoutTabular(t *testing.T) {
	// Defensive: a non-tabular handler's output is never reinterpreted as a
	// table, even if it happens to be comma-separated.
	csv := "ID,Name\nC1,#g"
	if got := strings.TrimSpace(run(t, csv, false, false)); got != csv {
		t.Fatalf("CSV converted without tabular flag: %q", got)
	}
}

func TestUnitPrintEmptyWritesNothing(t *testing.T) {
	if got := run(t, "\n\n", false, true); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}
