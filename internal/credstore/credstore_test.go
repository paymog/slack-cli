package credstore_test

import (
	"testing"

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

func TestUnitStoreRoundTrip(t *testing.T) {
	t.Setenv("SLACK_CLI_CONFIG_DIR", t.TempDir())
	prev := credstore.SetBackend(&fakeKeyring{m: map[string]string{}})
	defer credstore.SetBackend(prev)

	s, err := credstore.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Names()) != 0 {
		t.Fatalf("expected empty store, got %v", s.Names())
	}

	if err := s.Add("default", credstore.Profile{Mode: "user"}, credstore.Tokens{XOXP: "x1"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// First profile becomes default.
	if s.Default != "default" {
		t.Fatalf("first profile should be default, got %q", s.Default)
	}

	// Reload from disk and confirm metadata + keyring round-trip.
	s2, err := credstore.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	tok, err := s2.Tokens("default")
	if err != nil {
		t.Fatalf("Tokens: %v", err)
	}
	if tok.XOXP != "x1" {
		t.Fatalf("token round-trip = %q", tok.XOXP)
	}

	// Adding a second profile keeps the original default.
	if err := s2.Add("prod", credstore.Profile{Mode: "bot"}, credstore.Tokens{XOXB: "b1"}); err != nil {
		t.Fatalf("Add prod: %v", err)
	}
	if s2.Default != "default" {
		t.Fatalf("default should stay 'default', got %q", s2.Default)
	}

	if err := s2.SetDefault("prod"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	// Removing the default reassigns it to a remaining profile.
	if err := s2.Remove("prod"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	s3, _ := credstore.Load()
	if s3.Default != "default" {
		t.Fatalf("default after removing prod = %q", s3.Default)
	}
	if _, err := s3.Tokens("prod"); err == nil {
		t.Fatal("expected prod keyring entry to be deleted")
	}
}
