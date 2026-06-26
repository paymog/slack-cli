package toolcall

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestUnitInvokePassesArgsAndExtractsText(t *testing.T) {
	var gotName string
	var gotArg string
	h := func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		gotName = req.Params.Name
		gotArg = req.GetString("channel_id", "")
		return mcp.NewToolResultText("hello,world"), nil
	}
	out, err := Invoke(context.Background(), h, "channels_list", map[string]any{"channel_id": "C123"})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out != "hello,world" {
		t.Fatalf("out = %q", out)
	}
	if gotName != "channels_list" {
		t.Fatalf("name = %q", gotName)
	}
	if gotArg != "C123" {
		t.Fatalf("arg not delivered, got %q", gotArg)
	}
}

func TestUnitInvokeSurfacesIsError(t *testing.T) {
	h := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		r := mcp.NewToolResultText("boom")
		r.IsError = true
		return r, nil
	}
	_, err := Invoke(context.Background(), h, "t", nil)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want IsError surfaced, got %v", err)
	}
}

func TestUnitInvokeReturnsHandlerError(t *testing.T) {
	h := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, errors.New("nope")
	}
	_, err := Invoke(context.Background(), h, "t", nil)
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("want handler error, got %v", err)
	}
}

func TestUnitInvokeRecoversPanic(t *testing.T) {
	h := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		panic("kaboom")
	}
	_, err := Invoke(context.Background(), h, "t", nil)
	if err == nil || !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("want recovered panic as error, got %v", err)
	}
}
