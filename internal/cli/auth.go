package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/internal/credstore"
	"github.com/paymog/slack-cli/internal/output"
	"github.com/paymog/slack-cli/internal/runtime"
	"github.com/paymog/slack-cli/pkg/provider"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// skipResolveAnnotation marks commands that must run before credentials are
// resolved (e.g. `auth login`, which has no stored profile yet). The root
// PersistentPreRunE skips config.Resolve for any command carrying it.
const skipResolveAnnotation = "skipAuthResolve"

func skipResolve() map[string]string {
	return map[string]string{skipResolveAnnotation: "true"}
}

func newAuthCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage stored credential profiles",
		Long: "Manage named credential profiles. Slack tokens are stored in the OS keyring;\n" +
			"profile metadata (auth mode, GovSlack) lives in a config file.\n\n" +
			"For CI or one-off use, set SLACK_MCP_XOXP_TOKEN (or xoxb, or xoxc+xoxd) instead —\n" +
			"env tokens take precedence over stored profiles.",
		Annotations: skipResolve(),
		RunE:        func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(
		authLoginCommand(cfg),
		authListCommand(cfg),
		authDefaultCommand(cfg),
		authLogoutCommand(cfg),
		authTokenCommand(cfg),
		authStatusCommand(cfg),
	)
	return cmd
}

func authLoginCommand(cfg *config.Config) *cobra.Command {
	var (
		name     string
		xoxp     string
		xoxb     string
		xoxc     string
		xoxd     string
		govslack bool
	)
	cmd := &cobra.Command{
		Use:         "login [name]",
		Short:       "Add or update a credential profile",
		Args:        cobra.MaximumNArgs(1),
		Annotations: skipResolve(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				name = args[0]
			}
			if name == "" {
				name = "default"
			}
			if !credstore.Available() {
				return fmt.Errorf("no usable OS keyring found; set SLACK_MCP_XOXP_TOKEN (or xoxb, or xoxc+xoxd) instead of using profiles")
			}

			tok := credstore.Tokens{XOXP: xoxp, XOXB: xoxb, XOXC: xoxc, XOXD: xoxd}
			if modeLabel(tok) == "" {
				var err error
				if tok, err = promptForTokens(cmd); err != nil {
					return err
				}
			}
			if modeLabel(tok) == "" {
				return fmt.Errorf("no credentials provided")
			}

			// GovSlack must be visible to the validator via env.
			if govslack {
				_ = os.Setenv("SLACK_MCP_GOVSLACK", "true")
			}
			logger := runtime.Logger(cfg.Verbose)
			teamID, err := provider.ValidateTokens(tok.XOXP, tok.XOXB, tok.XOXC, tok.XOXD, logger)
			if err != nil {
				return fmt.Errorf("credential check failed (verify your tokens): %w", err)
			}

			store, err := credstore.Load()
			if err != nil {
				return err
			}
			first := len(store.Profiles) == 0
			if err := store.Add(name, credstore.Profile{Mode: modeLabel(tok), GovSlack: govslack}, tok); err != nil {
				return err
			}

			if cfg.Raw {
				fmt.Fprintf(cmd.OutOrStdout(), "Saved profile %q (%s, team %s)\n", name, modeLabel(tok), teamID)
				if first {
					fmt.Fprintln(cmd.OutOrStdout(), "  Set as default profile")
				}
				return nil
			}
			return output.WriteJSON(cmd.OutOrStdout(), struct {
				Profile string `json:"profile"`
				Mode    string `json:"mode"`
				TeamID  string `json:"team_id"`
				Default bool   `json:"default"`
			}{Profile: name, Mode: modeLabel(tok), TeamID: teamID, Default: first})
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "Profile name (defaults to the positional arg, then \"default\")")
	f.StringVar(&xoxp, "xoxp", "", "User OAuth token (xoxp-...)")
	f.StringVar(&xoxb, "xoxb", "", "Bot token (xoxb-...)")
	f.StringVar(&xoxc, "xoxc", "", "Browser token (xoxc-...); requires --xoxd")
	f.StringVar(&xoxd, "xoxd", "", "Browser cookie d (xoxd-...); requires --xoxc")
	f.BoolVar(&govslack, "govslack", false, "Route API calls to slack-gov.com")
	return cmd
}

func authListCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List configured profiles",
		Annotations: skipResolve(),
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := credstore.Load()
			if err != nil {
				return err
			}
			names := store.Names()
			if cfg.Raw {
				if len(names) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No profiles configured. Run `slack-cli auth login`.")
					return nil
				}
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "\tPROFILE\tMODE\tGOVSLACK")
				for _, name := range names {
					marker := " "
					if name == store.Default {
						marker = "*"
					}
					profile := store.Profiles[name]
					fmt.Fprintf(tw, "%s\t%s\t%s\t%t\n", marker, name, profile.Mode, profile.GovSlack)
				}
				return tw.Flush()
			}
			type profileResult struct {
				Profile  string `json:"profile"`
				Mode     string `json:"mode"`
				GovSlack bool   `json:"govslack"`
				Default  bool   `json:"default"`
			}
			results := make([]profileResult, 0, len(names))
			for _, name := range names {
				profile := store.Profiles[name]
				results = append(results, profileResult{
					Profile: name, Mode: profile.Mode, GovSlack: profile.GovSlack, Default: name == store.Default,
				})
			}
			return output.WriteJSON(cmd.OutOrStdout(), results)
		},
	}
}

func authDefaultCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:         "default <name>",
		Short:       "Set the default profile",
		Args:        cobra.ExactArgs(1),
		Annotations: skipResolve(),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := credstore.Load()
			if err != nil {
				return err
			}
			if err := store.SetDefault(args[0]); err != nil {
				return err
			}
			if cfg.Raw {
				fmt.Fprintf(cmd.OutOrStdout(), "Default profile set to %q\n", args[0])
				return nil
			}
			return output.WriteJSON(cmd.OutOrStdout(), struct {
				DefaultProfile string `json:"default_profile"`
			}{DefaultProfile: args[0]})
		},
	}
}

func authLogoutCommand(cfg *config.Config) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:         "logout <name>",
		Short:       "Remove a credential profile",
		Args:        cobra.ExactArgs(1),
		Annotations: skipResolve(),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			store, err := credstore.Load()
			if err != nil {
				return err
			}
			if _, ok := store.Profiles[name]; !ok {
				return fmt.Errorf("profile %q not found", name)
			}
			if !force {
				ok, err := confirm(cmd, fmt.Sprintf("Remove profile %q? [y/N] ", name))
				if err != nil {
					return err
				}
				if !ok {
					if cfg.Raw {
						fmt.Fprintln(cmd.OutOrStdout(), "Aborted")
						return nil
					}
					return output.WriteJSON(cmd.OutOrStdout(), struct {
						Profile string `json:"profile"`
						Removed bool   `json:"removed"`
					}{Profile: name})
				}
			}
			if err := store.Remove(name); err != nil {
				return err
			}
			if cfg.Raw {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed profile %q\n", name)
				return nil
			}
			return output.WriteJSON(cmd.OutOrStdout(), struct {
				Profile string `json:"profile"`
				Removed bool   `json:"removed"`
			}{Profile: name, Removed: true})
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	return cmd
}

func authTokenCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Print the resolved tokens as a JSON object",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cfg.RequireAuth(); err != nil {
				return err
			}
			if cfg.Raw {
				out := cmd.OutOrStdout()
				emitToken(out, "SLACK_MCP_XOXP_TOKEN", cfg.XOXP)
				emitToken(out, "SLACK_MCP_XOXB_TOKEN", cfg.XOXB)
				emitToken(out, "SLACK_MCP_XOXC_TOKEN", cfg.XOXC)
				emitToken(out, "SLACK_MCP_XOXD_TOKEN", cfg.XOXD)
				return nil
			}
			tokens := map[string]string{}
			if cfg.XOXP != "" {
				tokens["SLACK_MCP_XOXP_TOKEN"] = cfg.XOXP
			}
			if cfg.XOXB != "" {
				tokens["SLACK_MCP_XOXB_TOKEN"] = cfg.XOXB
			}
			if cfg.XOXC != "" {
				tokens["SLACK_MCP_XOXC_TOKEN"] = cfg.XOXC
			}
			if cfg.XOXD != "" {
				tokens["SLACK_MCP_XOXD_TOKEN"] = cfg.XOXD
			}
			return output.WriteJSON(cmd.OutOrStdout(), tokens)
		},
	}
}

func authStatusCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the resolved credential source and auth mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			source := "none"
			profile := ""
			switch {
			case cfg.HasExplicitEnv():
				source = "environment"
			case cfg.Profile != "":
				source = "profile"
				profile = cfg.Profile
			default:
				store, err := credstore.Load()
				if err != nil {
					return err
				}
				if store.Default != "" {
					source = "default_profile"
					profile = store.Default
				}
			}
			configured := cfg.RequireAuth() == nil
			if cfg.Raw {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "Source: %s", source)
				if profile != "" {
					fmt.Fprintf(out, " %q", profile)
				}
				fmt.Fprintln(out)
				fmt.Fprintf(out, "Auth mode: %s\n", cfg.Mode())
				fmt.Fprintf(out, "GovSlack: %t\n", cfg.GovSlack)
				fmt.Fprintf(out, "Credentials configured: %t\n", configured)
				fmt.Fprintf(out, "Keyring available: %t\n", credstore.Available())
				return nil
			}
			return output.WriteJSON(cmd.OutOrStdout(), struct {
				Source                string `json:"source"`
				Profile               string `json:"profile,omitempty"`
				Mode                  string `json:"mode"`
				GovSlack              bool   `json:"govslack"`
				CredentialsConfigured bool   `json:"credentials_configured"`
				KeyringAvailable      bool   `json:"keyring_available"`
			}{
				Source: source, Profile: profile, Mode: cfg.Mode(), GovSlack: cfg.GovSlack,
				CredentialsConfigured: configured, KeyringAvailable: credstore.Available(),
			})
		},
	}
}

func modeLabel(t credstore.Tokens) string {
	switch {
	case t.XOXP != "":
		return "user"
	case t.XOXB != "":
		return "bot"
	case t.XOXC != "" && t.XOXD != "":
		return "session"
	default:
		return ""
	}
}

func emitToken(w io.Writer, key, val string) {
	if val != "" {
		fmt.Fprintf(w, "%s=%s\n", key, val)
	}
}

func promptForTokens(cmd *cobra.Command) (credstore.Tokens, error) {
	mode, err := promptLine(cmd, "Auth mode [user/bot/session]: ")
	if err != nil {
		return credstore.Tokens{}, err
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "user":
		t, err := promptSecret(cmd, "User OAuth token (xoxp-...): ")
		return credstore.Tokens{XOXP: t}, err
	case "bot":
		t, err := promptSecret(cmd, "Bot token (xoxb-...): ")
		return credstore.Tokens{XOXB: t}, err
	case "session":
		c, err := promptSecret(cmd, "Browser token (xoxc-...): ")
		if err != nil {
			return credstore.Tokens{}, err
		}
		d, err := promptSecret(cmd, "Browser cookie d (xoxd-...): ")
		return credstore.Tokens{XOXC: c, XOXD: d}, err
	default:
		return credstore.Tokens{}, fmt.Errorf("unknown mode %q (expected user, bot, or session)", mode)
	}
}

func promptLine(cmd *cobra.Command, prompt string) (string, error) {
	fmt.Fprint(cmd.ErrOrStderr(), prompt)
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func promptSecret(cmd *cobra.Command, prompt string) (string, error) {
	fmt.Fprint(cmd.ErrOrStderr(), prompt)
	if f, ok := cmd.InOrStdin().(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(cmd.ErrOrStderr())
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	return promptLine(cmd, "")
}

func confirm(cmd *cobra.Command, prompt string) (bool, error) {
	line, err := promptLine(cmd, prompt)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
