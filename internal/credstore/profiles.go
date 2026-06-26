// Package credstore manages named credential profiles for the slack CLI.
//
// A profile bundles non-secret connection metadata (auth mode, GovSlack flag)
// in a YAML file, while the Slack tokens themselves live in the OS keyring as a
// JSON bundle. The on-disk file never contains secrets, only the profile list
// and which one is the default.
package credstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Tokens is the secret credential bundle stored in the OS keyring. At least one
// of XOXP, XOXB, or the XOXC+XOXD pair must be set.
type Tokens struct {
	XOXP string `json:"xoxp,omitempty"`
	XOXB string `json:"xoxb,omitempty"`
	XOXC string `json:"xoxc,omitempty"`
	XOXD string `json:"xoxd,omitempty"`
}

// Profile is the non-secret metadata for one named credential.
type Profile struct {
	Mode     string `yaml:"mode,omitempty"` // user | bot | session (informational)
	GovSlack bool   `yaml:"govslack,omitempty"`
}

// Store is the on-disk profile metadata. Tokens are NOT stored here; they live
// in the OS keyring keyed by profile name.
type Store struct {
	Default  string             `yaml:"default,omitempty"`
	Profiles map[string]Profile `yaml:"profiles"`
}

// Path returns the profiles file location, following the same rules Go's
// os.UserConfigDir uses (XDG_CONFIG_HOME or ~/.config on Unix, %AppData% on
// Windows). The SLACK_CLI_CONFIG_DIR env var overrides it (used by tests).
func Path() (string, error) {
	if dir := os.Getenv("SLACK_CLI_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "profiles.yaml"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determine config dir: %w", err)
	}
	return filepath.Join(dir, "slack-cli", "profiles.yaml"), nil
}

// Load reads the profiles file. A missing file yields an empty store, not an error.
func Load() (*Store, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Store{Profiles: map[string]Profile{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read profiles file %s: %w", path, err)
	}
	var s Store
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse profiles file %s (delete it and re-run `slack-cli auth login`): %w", path, err)
	}
	if s.Profiles == nil {
		s.Profiles = map[string]Profile{}
	}
	return &s, nil
}

// Save writes the profiles file, creating parent directories as needed.
func (s *Store) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal profiles: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write profiles file %s: %w", path, err)
	}
	return nil
}

// Names returns the configured profile names in sorted order.
func (s *Store) Names() []string {
	names := make([]string, 0, len(s.Profiles))
	for name := range s.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Add stores the token bundle in the keyring and records the profile metadata.
// The first profile added automatically becomes the default.
func (s *Store) Add(name string, p Profile, tok Tokens) error {
	if name == "" {
		return errors.New("profile name is required")
	}
	data, err := json.Marshal(tok)
	if err != nil {
		return fmt.Errorf("marshal token bundle: %w", err)
	}
	if err := active.Set(name, string(data)); err != nil {
		return fmt.Errorf("store tokens in keyring for profile %q: %w", name, err)
	}
	if s.Profiles == nil {
		s.Profiles = map[string]Profile{}
	}
	first := len(s.Profiles) == 0
	s.Profiles[name] = p
	if first {
		s.Default = name
	}
	return s.Save()
}

// Remove deletes the profile's keyring entry and metadata. If it was the default,
// the default is reassigned to another profile (or cleared).
func (s *Store) Remove(name string) error {
	if _, ok := s.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	if err := active.Delete(name); err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("remove tokens from keyring for profile %q: %w", name, err)
	}
	delete(s.Profiles, name)
	if s.Default == name {
		s.Default = ""
		if names := s.Names(); len(names) > 0 {
			s.Default = names[0]
		}
	}
	return s.Save()
}

// SetDefault marks an existing profile as the default.
func (s *Store) SetDefault(name string) error {
	if _, ok := s.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	s.Default = name
	return s.Save()
}

// Tokens returns the keyring-stored token bundle for a profile.
func (s *Store) Tokens(name string) (Tokens, error) {
	raw, err := active.Get(name)
	if err != nil {
		return Tokens{}, err
	}
	var t Tokens
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return Tokens{}, fmt.Errorf("parse keyring token bundle for profile %q: %w", name, err)
	}
	return t, nil
}
