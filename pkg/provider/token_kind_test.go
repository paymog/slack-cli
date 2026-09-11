package provider

import "testing"

func TestUnitTokenKindFromEnvSource(t *testing.T) {
	// An egress proxy can hand the CLI an opaque sentinel and swap the real
	// credential in on the wire, so the prefix says nothing about the kind.
	t.Setenv("SLACK_MCP_XOXP_TOKEN", "")
	t.Setenv("SLACK_MCP_XOXC_TOKEN", "")
	t.Setenv("SLACK_MCP_XOXB_TOKEN", "sin_opaque_sentinel")

	isOAuth, isBot := tokenKind("sin_opaque_sentinel")
	if !isOAuth || !isBot {
		t.Fatalf("sentinel from XOXB env: got isOAuth=%v isBot=%v, want true/true", isOAuth, isBot)
	}
}

func TestUnitTokenKindUserBeatsBot(t *testing.T) {
	t.Setenv("SLACK_MCP_XOXP_TOKEN", "sin_same")
	t.Setenv("SLACK_MCP_XOXB_TOKEN", "sin_same")

	isOAuth, isBot := tokenKind("sin_same")
	if !isOAuth || isBot {
		t.Fatalf("got isOAuth=%v isBot=%v, want true/false", isOAuth, isBot)
	}
}

func TestUnitTokenKindSessionToken(t *testing.T) {
	t.Setenv("SLACK_MCP_XOXP_TOKEN", "")
	t.Setenv("SLACK_MCP_XOXB_TOKEN", "")
	t.Setenv("SLACK_MCP_XOXC_TOKEN", "sin_session_sentinel")

	isOAuth, isBot := tokenKind("sin_session_sentinel")
	if isOAuth || isBot {
		t.Fatalf("got isOAuth=%v isBot=%v, want false/false", isOAuth, isBot)
	}
}

func TestUnitTokenKindPrefixFallback(t *testing.T) {
	t.Setenv("SLACK_MCP_XOXP_TOKEN", "")
	t.Setenv("SLACK_MCP_XOXB_TOKEN", "")
	t.Setenv("SLACK_MCP_XOXC_TOKEN", "")

	for _, tc := range []struct {
		token   string
		isOAuth bool
		isBot   bool
	}{
		{"xoxb-1-2-3", true, true},
		{"xoxe.xoxb-1-2-3", true, true},
		{"xoxp-1-2-3", true, false},
		{"xoxe.xoxp-1-2-3", true, false},
		{"xoxc-1-2-3", false, false},
		{"", false, false},
	} {
		isOAuth, isBot := tokenKind(tc.token)
		if isOAuth != tc.isOAuth || isBot != tc.isBot {
			t.Errorf("tokenKind(%q) = %v/%v, want %v/%v", tc.token, isOAuth, isBot, tc.isOAuth, tc.isBot)
		}
	}
}
