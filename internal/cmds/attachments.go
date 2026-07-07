package cmds

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/internal/output"
	"github.com/paymog/slack-cli/internal/runtime"
	"github.com/paymog/slack-cli/internal/toolcall"
	"github.com/paymog/slack-cli/pkg/handler"
	"github.com/spf13/cobra"
)

func newAttachmentsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "attachments",
		Aliases: []string{"attachment", "files"},
		Short:   "Download attachment data (requires SLACK_MCP_ATTACHMENT_TOOL)",
	}
	cmd.AddCommand(attachmentsGetCommand(cfg))
	return cmd
}

func attachmentsGetCommand(cfg *config.Config) *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "get <file_id>",
		Short: "Download an attachment by file ID (Fxxxxxxxxxx); max 5MB",
		Long: "Download an attachment by file ID (Fxxxxxxxxxx); max 5MB.\n\n" +
			"By default the file is printed as a JSON envelope with the bytes inline,\n" +
			"base64-encoded for images and other binaries (decode with\n" +
			"`jq -r .content | base64 --decode`). Pass -o/--output to write the decoded\n" +
			"bytes straight to a file and keep stdout to a small metadata confirmation —\n" +
			"recommended for images and large binaries so a multi-MB blob does not flood\n" +
			"the terminal.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewConversationsHandler(p, logger)
			res, err := toolcall.InvokeResult(cmd.Context(), h.FilesGetHandler, "attachment_get_data", map[string]any{"file_id": args[0]})
			if err != nil {
				return err
			}
			return writeAttachment(cmd.OutOrStdout(), res, outPath, cfg.Raw)
		},
	}
	cmd.Flags().StringVarP(&outPath, "output", "o", "", "write the decoded file bytes to this path instead of printing them inline")
	return cmd
}

// attachmentMeta is the JSON envelope the FilesGet handler emits for a file.
// For images the handler returns the bytes as a separate MCP image block, so
// Encoding/Content are empty in this text envelope and the bytes arrive in
// toolcall.Result.Images instead.
type attachmentMeta struct {
	FileID   string `json:"file_id"`
	Filename string `json:"filename"`
	Mimetype string `json:"mimetype"`
	Size     int    `json:"size"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

// writeAttachment renders a fetched attachment. Without -o it prints the JSON
// envelope with the bytes inline, folding image bytes (which the handler
// delivers out-of-band as a native MCP image block) back into the "content"
// field so nothing is lost. With -o it decodes the bytes to disk and prints a
// small metadata confirmation, so an agent's stdout is not flooded with a
// multi-MB base64 blob.
func writeAttachment(w io.Writer, res *toolcall.Result, outPath string, raw bool) error {
	if outPath == "" {
		text := res.Text
		if len(res.Images) > 0 {
			text = injectContent(res.Text, "base64", res.Images[0].Data)
		}
		return output.Print(w, text, raw, false)
	}

	data, meta, err := attachmentBytes(res)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return printAttachmentConfirmation(w, meta, outPath, len(data))
}

// attachmentBytes returns the decoded file bytes plus their metadata. Image
// bytes come from the out-of-band image block; every other file carries its
// bytes in the envelope's "content" field (base64 for binaries, verbatim for
// text).
func attachmentBytes(res *toolcall.Result) ([]byte, attachmentMeta, error) {
	var meta attachmentMeta
	if strings.TrimSpace(res.Text) != "" {
		if err := json.Unmarshal([]byte(res.Text), &meta); err != nil {
			return nil, meta, fmt.Errorf("parse attachment metadata: %w", err)
		}
	}
	if len(res.Images) > 0 {
		if meta.Mimetype == "" {
			meta.Mimetype = res.Images[0].MIMEType
		}
		data, err := base64.StdEncoding.DecodeString(res.Images[0].Data)
		if err != nil {
			return nil, meta, fmt.Errorf("decode image data: %w", err)
		}
		return data, meta, nil
	}
	if meta.Encoding == "base64" {
		data, err := base64.StdEncoding.DecodeString(meta.Content)
		if err != nil {
			return nil, meta, fmt.Errorf("decode base64 content: %w", err)
		}
		return data, meta, nil
	}
	return []byte(meta.Content), meta, nil
}

// printAttachmentConfirmation writes the post-download metadata as JSON (HTML
// escaping off so filenames stay legible), reporting how many bytes landed
// where. size is the number of bytes actually written.
func printAttachmentConfirmation(w io.Writer, meta attachmentMeta, path string, n int) error {
	conf := struct {
		FileID   string `json:"file_id"`
		Filename string `json:"filename"`
		Mimetype string `json:"mimetype"`
		Size     int    `json:"size"`
		Path     string `json:"path"`
	}{meta.FileID, meta.Filename, meta.Mimetype, n, path}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(conf); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// injectContent folds encoding/content fields into the handler's metadata JSON
// object, preserving field order (a struct round-trip would sort keys and
// HTML-escape Slack markup). b64 is base64 text, so it needs no escaping.
func injectContent(metaJSON, encoding, b64 string) string {
	trimmed := strings.TrimSpace(metaJSON)
	if !strings.HasSuffix(trimmed, "}") {
		return fmt.Sprintf(`{"encoding":%q,"content":%q}`, encoding, b64)
	}
	inner := strings.TrimSuffix(trimmed, "}")
	sep := ","
	if strings.TrimSpace(strings.TrimPrefix(inner, "{")) == "" {
		sep = ""
	}
	return fmt.Sprintf(`%s%s"encoding":%q,"content":%q}`, inner, sep, encoding, b64)
}
