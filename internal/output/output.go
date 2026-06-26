package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Print writes a tool's text result. Tool handlers return CSV or JSON strings.
// Unless raw is set, valid JSON is pretty-printed; everything else (CSV/text)
// is written verbatim with a single trailing newline. Empty output prints
// nothing.
func Print(w io.Writer, text string, raw bool) error {
	if raw {
		_, err := io.WriteString(w, text)
		return err
	}

	trimmed := strings.TrimRight(text, "\n")
	if trimmed == "" {
		return nil
	}

	var decoded any
	if json.Unmarshal([]byte(trimmed), &decoded) == nil {
		pretty, err := json.MarshalIndent(decoded, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(pretty))
		return err
	}

	_, err := fmt.Fprintln(w, trimmed)
	return err
}
