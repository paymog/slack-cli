package transport

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// AuthHeaderTransport moves the Slack `token` form field into an
// `Authorization: Bearer` header.
//
// slack-go's postForm sends every Web API call as x-www-form-urlencoded with
// the token in a `token` field and no Authorization header. Slack accepts
// either, but body-carried credentials are invisible to the credential
// brokers, MITM proxies, and audit tooling that sit in front of this CLI —
// they only inspect headers, so a body token is passed through verbatim and
// Slack answers `invalid_auth`.
//
// Browser-session credentials (`xoxc-`, paired with the `d` cookie) stay in
// the body: they are not bearer tokens and the edge API expects them there.
type AuthHeaderTransport struct {
	roundTripper http.RoundTripper
}

// NewAuthHeaderTransport wraps rt so form-carried tokens become bearer headers.
func NewAuthHeaderTransport(rt http.RoundTripper) *AuthHeaderTransport {
	return &AuthHeaderTransport{roundTripper: rt}
}

// RoundTrip implements the http.RoundTripper interface.
func (t *AuthHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rewritten, err := bearerizeFormToken(req)
	if err != nil {
		return nil, err
	}
	return t.roundTripper.RoundTrip(rewritten)
}

// bearerizeFormToken returns req with its `token` form field promoted to an
// Authorization header, or req untouched when there is nothing to promote.
func bearerizeFormToken(req *http.Request) (*http.Request, error) {
	if req.Body == nil || req.Header.Get("Authorization") != "" {
		return req, nil
	}
	if !strings.HasPrefix(req.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		return req, nil
	}

	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	// Any bail-out below must hand the consumed body back to the caller.
	restore := func() *http.Request {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return req
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		return restore(), nil
	}
	token := values.Get("token")
	if token == "" || strings.HasPrefix(token, "xoxc-") || strings.HasPrefix(token, "xoxd-") {
		return restore(), nil
	}

	values.Del("token")
	encoded := values.Encode()

	out := req.Clone(req.Context())
	out.Header.Set("Authorization", "Bearer "+token)
	out.Body = io.NopCloser(strings.NewReader(encoded))
	out.ContentLength = int64(len(encoded))
	out.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(encoded)), nil
	}
	return out, nil
}
