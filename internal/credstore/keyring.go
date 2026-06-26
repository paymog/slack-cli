package credstore

import "github.com/zalando/go-keyring"

// Service is the keyring service name under which all profile token bundles are
// stored. The keyring "account" is the profile name.
const Service = "slack-cli"

// ErrNotFound is returned by the keyring when an account has no entry.
var ErrNotFound = keyring.ErrNotFound

// Backend abstracts the OS keyring so tests can inject an in-memory fake.
type Backend interface {
	Get(account string) (string, error)
	Set(account, secret string) error
	Delete(account string) error
}

type osKeyring struct{}

func (osKeyring) Get(account string) (string, error) { return keyring.Get(Service, account) }
func (osKeyring) Set(account, secret string) error   { return keyring.Set(Service, account, secret) }
func (osKeyring) Delete(account string) error        { return keyring.Delete(Service, account) }

var active Backend = osKeyring{}

// SetBackend swaps the keyring backend. Tests use a map-backed fake; production
// code never calls this. Returns the previous backend so callers can restore it.
func SetBackend(b Backend) Backend {
	prev := active
	active = b
	return prev
}

// Available reports whether the OS keyring can be written to and read from.
// When false, callers should fall back to the SLACK_MCP_XOX*_TOKEN env vars.
func Available() bool {
	const probe = "__slack_cli_probe__"
	if err := active.Set(probe, "x"); err != nil {
		return false
	}
	_ = active.Delete(probe)
	return true
}
