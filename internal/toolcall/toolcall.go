// Package toolcall is the single point of coupling between the CLI and the
// upstream MCP tool handlers. Every handler has the uniform signature
// func(ctx, mcp.CallToolRequest) (*mcp.CallToolResult, error). This package
// builds an in-process request from a plain arguments map, invokes the handler,
// and extracts the text payload — so command code never imports mcp-go and the
// handlers stay byte-for-byte identical to upstream (clean merges).
package toolcall

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Handler is the shared shape of every slack-mcp-server tool handler.
type Handler func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

// Result is a tool's output separated into text and any binary blobs.
//
// Text is the concatenation of every TextContent block a handler returns (a
// JSON document, a CSV table, or a status line). Images holds the base64
// ImageContent blocks: the FilesGet handler returns image attachments as a
// native MCP image block (mcp.NewToolResultImage) whose bytes never appear in
// Text, so capturing them here is what lets the CLI recover the actual image.
type Result struct {
	Text   string
	Images []Blob
}

// Blob is a base64-encoded binary content block with its MIME type.
type Blob struct {
	Data     string // base64-encoded bytes
	MIMEType string
}

// Invoke calls handler h with args delivered exactly as the MCP server would
// (a map keyed by the tool's parameter names) and returns the text result.
// A handler-reported error result (IsError) is surfaced as a Go error. Binary
// (image) content is discarded — use InvokeResult when the bytes are needed.
func Invoke(ctx context.Context, h Handler, name string, args map[string]any) (string, error) {
	res, err := InvokeResult(ctx, h, name, args)
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

// InvokeResult is Invoke that also returns any binary (image) content blocks
// instead of dropping them, so callers such as `attachments get` can write the
// actual file bytes. Text extraction and error handling match Invoke.
func InvokeResult(ctx context.Context, h Handler, name string, args map[string]any) (res *Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			res, err = nil, fmt.Errorf("tool %q failed unexpectedly: %v", name, r)
		}
	}()

	var req mcp.CallToolRequest
	req.Params.Name = name
	req.Params.Arguments = args

	out, err := h(ctx, req)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return &Result{}, nil
	}

	var b strings.Builder
	r := &Result{}
	for _, c := range out.Content {
		switch v := c.(type) {
		case mcp.TextContent:
			b.WriteString(v.Text)
		case mcp.ImageContent:
			r.Images = append(r.Images, Blob{Data: v.Data, MIMEType: v.MIMEType})
		}
	}
	r.Text = b.String()
	if out.IsError {
		msg := strings.TrimSpace(r.Text)
		if msg == "" {
			msg = "tool reported an error"
		}
		return nil, errors.New(msg)
	}
	return r, nil
}
