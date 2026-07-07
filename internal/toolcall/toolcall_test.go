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

func TestUnitInvokeResultCapturesImage(t *testing.T) {
	const metaText = `{"file_id":"F1"}`
	const b64Data = "aVZCT1J3MEtHZ289"
	h := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultImage(metaText, b64Data, "image/png"), nil
	}
	res, err := InvokeResult(context.Background(), h, "files_get", nil)
	if err != nil {
		t.Fatalf("InvokeResult: %v", err)
	}
	if res.Text != metaText {
		t.Errorf("Text = %q, want %q", res.Text, metaText)
	}
	if len(res.Images) != 1 {
		t.Fatalf("Images len = %d, want 1", len(res.Images))
	}
	if res.Images[0].Data != b64Data {
		t.Errorf("Images[0].Data = %q, want %q", res.Images[0].Data, b64Data)
	}
	if res.Images[0].MIMEType != "image/png" {
		t.Errorf("Images[0].MIMEType = %q, want image/png", res.Images[0].MIMEType)
	}
}

func TestUnitInvokeDropsImageContent(t *testing.T) {
	const metaText = `{"file_id":"F1"}`
	const b64Data = "aVZCT1J3MEtHZ289"
	h := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultImage(metaText, b64Data, "image/png"), nil
	}
	out, err := Invoke(context.Background(), h, "files_get", nil)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out != metaText {
		t.Errorf("out = %q, want %q", out, metaText)
	}
	if strings.Contains(out, b64Data) {
		t.Error("Invoke leaked image bytes into text output — regression")
	}
}

func TestUnitInvokeResultSurfacesErrorAndNil(t *testing.T) {
	// IsError result must surface the message as a Go error.
	hErr := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		r := mcp.NewToolResultText("something went wrong")
		r.IsError = true
		return r, nil
	}
	_, err := InvokeResult(context.Background(), hErr, "t", nil)
	if err == nil || !strings.Contains(err.Error(), "something went wrong") {
		t.Fatalf("want IsError surfaced as error, got %v", err)
	}

	// Nil *mcp.CallToolResult must yield a non-nil *Result with no error.
	hNil := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, nil
	}
	res, err2 := InvokeResult(context.Background(), hNil, "t", nil)
	if err2 != nil {
		t.Fatalf("want nil error for nil handler result, got %v", err2)
	}
	if res == nil {
		t.Fatal("want non-nil *Result for nil handler result")
	}
	if res.Text != "" || len(res.Images) != 0 {
		t.Errorf("want empty Result{}, got Text=%q Images=%d", res.Text, len(res.Images))
	}
}
