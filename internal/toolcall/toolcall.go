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

// Invoke calls handler h with args delivered exactly as the MCP server would
// (a map keyed by the tool's parameter names) and returns the text result.
// A handler-reported error result (IsError) is surfaced as a Go error.
func Invoke(ctx context.Context, h Handler, name string, args map[string]any) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = "", fmt.Errorf("tool %q failed unexpectedly: %v", name, r)
		}
	}()

	var req mcp.CallToolRequest
	req.Params.Name = name
	req.Params.Arguments = args

	res, err := h(ctx, req)
	if err != nil {
		return "", err
	}
	if res == nil {
		return "", nil
	}

	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	out = b.String()
	if res.IsError {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = "tool reported an error"
		}
		return "", errors.New(msg)
	}
	return out, nil
}
