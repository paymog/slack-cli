package transport

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type captureRoundTripper struct{ req *http.Request }

func (c *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.req = req
	return &http.Response{StatusCode: 200, Body: http.NoBody, Request: req}, nil
}

func formRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "https://slack.com/api/auth.test", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func roundTrip(t *testing.T, req *http.Request) *http.Request {
	t.Helper()
	capture := &captureRoundTripper{}
	if _, err := NewAuthHeaderTransport(capture).RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	return capture.req
}

func sentBody(t *testing.T, req *http.Request) url.Values {
	t.Helper()
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	values, err := url.ParseQuery(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return values
}

func TestUnitFormTokenBecomesBearerHeader(t *testing.T) {
	sent := roundTrip(t, formRequest(t, "token=xoxb-real&channel=C1"))

	if got := sent.Header.Get("Authorization"); got != "Bearer xoxb-real" {
		t.Fatalf("Authorization = %q", got)
	}
	body := sentBody(t, sent)
	if body.Has("token") {
		t.Fatalf("token left in body: %v", body)
	}
	if body.Get("channel") != "C1" {
		t.Fatalf("other fields lost: %v", body)
	}
	// A wrong Content-Length makes Slack hang up or truncate the form.
	if want := int64(len(body.Encode())); sent.ContentLength != want {
		t.Fatalf("ContentLength = %d, want %d", sent.ContentLength, want)
	}
	// slack-go's retries replay the body.
	replay, err := sent.GetBody()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(replay)
	if strings.Contains(string(raw), "token=") {
		t.Fatalf("replayed body still carries the token: %q", raw)
	}
}

func TestUnitProxySentinelIsPromoted(t *testing.T) {
	// The whole point: an opaque broker sentinel must reach the header path.
	sent := roundTrip(t, formRequest(t, "token=sin_abc123&channel=C1"))
	if got := sent.Header.Get("Authorization"); got != "Bearer sin_abc123" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestUnitBrowserTokenStaysInBody(t *testing.T) {
	sent := roundTrip(t, formRequest(t, "token=xoxc-browser&channel=C1"))

	if got := sent.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want empty", got)
	}
	if got := sentBody(t, sent).Get("token"); got != "xoxc-browser" {
		t.Fatalf("token = %q, want xoxc-browser", got)
	}
}

func TestUnitNonFormRequestBodyIsPreserved(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://slack.com/api/files.upload", strings.NewReader("token=xoxb-real"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")

	sent := roundTrip(t, req)
	if got := sent.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want empty", got)
	}
	raw, _ := io.ReadAll(sent.Body)
	if string(raw) != "token=xoxb-real" {
		t.Fatalf("body = %q, want untouched", raw)
	}
}

func TestUnitExistingAuthorizationHeaderWins(t *testing.T) {
	req := formRequest(t, "token=xoxb-real")
	req.Header.Set("Authorization", "Bearer preset")

	sent := roundTrip(t, req)
	if got := sent.Header.Get("Authorization"); got != "Bearer preset" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := sentBody(t, sent).Get("token"); got != "xoxb-real" {
		t.Fatalf("body token = %q, want untouched", got)
	}
}
