package config_test

import (
	"os"
	"testing"

	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/internal/credstore"
)

type fakeKeyring struct{ m map[string]string }

func (f *fakeKeyring) Get(a string) (string, error) {
	v, ok := f.m[a]
	if !ok {
		return "", credstore.ErrNotFound
	}
	return v, nil
}
func (f *fakeKeyring) Set(a, s string) error { f.m[a] = s; return nil }
func (f *fakeKeyring) Delete(a string) error { delete(f.m, a); return nil }

// clearTokenEnv blanks every credential env var so a test starts from a known
// state (firstEnv treats "" as unset).
func clearTokenEnv(t *testing.T) {
	for _, k := range []string{
		"SLACK_MCP_XOXP_TOKEN", "SLACK_CLI_XOXP_TOKEN",
		"SLACK_MCP_XOXB_TOKEN", "SLACK_CLI_XOXB_TOKEN",
		"SLACK_MCP_XOXC_TOKEN", "SLACK_CLI_XOXC_TOKEN",
		"SLACK_MCP_XOXD_TOKEN", "SLACK_CLI_XOXD_TOKEN",
		"SLACK_MCP_GOVSLACK", "SLACK_CLI_GOVSLACK",
	} {
		t.Setenv(k, "")
	}
}

func TestUnitResolveExplicitEnvWins(t *testing.T) {
	clearTokenEnv(t)
	t.Setenv("SLACK_MCP_XOXP_TOKEN", "xoxp-env")

	store := &credstore.Store{Default: "default", Profiles: map[string]credstore.Profile{
		"default": {Mode: "user"},
	}}
	var c config.Config
	if err := c.Resolve(store); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if c.XOXP != "xoxp-env" {
		t.Fatalf("want env token, got %q", c.XOXP)
	}
}

func TestUnitResolveExplicitWithProfileIsAmbiguous(t *testing.T) {
	clearTokenEnv(t)
	t.Setenv("SLACK_MCP_XOXP_TOKEN", "xoxp-env")

	store := &credstore.Store{Profiles: map[string]credstore.Profile{}}
	c := config.Config{Profile: "prod"}
	if err := c.Resolve(store); err == nil {
		t.Fatal("expected ambiguity error combining env token with --profile")
	}
}

func TestUnitResolveDefaultProfileFromKeyring(t *testing.T) {
	clearTokenEnv(t)
	prev := credstore.SetBackend(&fakeKeyring{m: map[string]string{
		"default": `{"xoxc":"xoxc-1","xoxd":"xoxd-1"}`,
	}})
	defer credstore.SetBackend(prev)

	store := &credstore.Store{Default: "default", Profiles: map[string]credstore.Profile{
		"default": {Mode: "session", GovSlack: true},
	}}
	var c config.Config
	if err := c.Resolve(store); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if c.XOXC != "xoxc-1" || c.XOXD != "xoxd-1" {
		t.Fatalf("tokens not loaded from keyring: %+v", c)
	}
	if !c.GovSlack {
		t.Fatal("GovSlack should be inherited from profile metadata")
	}
}

func TestUnitResolveMissingProfileErrors(t *testing.T) {
	clearTokenEnv(t)
	store := &credstore.Store{Profiles: map[string]credstore.Profile{}}
	c := config.Config{Profile: "nope"}
	if err := c.Resolve(store); err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestUnitRequireAuthCombos(t *testing.T) {
	cases := []struct {
		name string
		c    config.Config
		ok   bool
	}{
		{"xoxp", config.Config{XOXP: "p"}, true},
		{"xoxb", config.Config{XOXB: "b"}, true},
		{"session", config.Config{XOXC: "c", XOXD: "d"}, true},
		{"xoxc only", config.Config{XOXC: "c"}, false},
		{"empty", config.Config{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.RequireAuth()
			if tc.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestUnitApplyToEnvSetsCanonicalVars(t *testing.T) {
	clearTokenEnv(t)
	c := config.Config{XOXP: "p1", GovSlack: true}
	c.ApplyToEnv()
	if got := os.Getenv("SLACK_MCP_XOXP_TOKEN"); got != "p1" {
		t.Fatalf("xoxp env = %q", got)
	}
	if got := os.Getenv("SLACK_MCP_GOVSLACK"); got != "true" {
		t.Fatalf("govslack env = %q", got)
	}
}
