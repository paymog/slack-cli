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
		authListCommand(),
		authDefaultCommand(),
		authLogoutCommand(),
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

			fmt.Fprintf(cmd.OutOrStdout(), "Saved profile %q (%s, team %s)\n", name, modeLabel(tok), teamID)
			if first {
				fmt.Fprintln(cmd.OutOrStdout(), "  Set as default profile")
			}
			return nil
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

func authListCommand() *cobra.Command {
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
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No profiles configured. Run `slack-cli auth login`.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "\tPROFILE\tMODE\tGOVSLACK")
			for _, n := range names {
				marker := " "
				if n == store.Default {
					marker = "*"
				}
				p := store.Profiles[n]
				fmt.Fprintf(tw, "%s\t%s\t%s\t%t\n", marker, n, p.Mode, p.GovSlack)
			}
			return tw.Flush()
		},
	}
}

func authDefaultCommand() *cobra.Command {
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
			fmt.Fprintf(cmd.OutOrStdout(), "Default profile set to %q\n", args[0])
			return nil
		},
	}
}

func authLogoutCommand() *cobra.Command {
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
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted")
					return nil
				}
			}
			if err := store.Remove(name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed profile %q\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	return cmd
}

func authTokenCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Print the resolved tokens as SLACK_MCP_* env lines",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cfg.RequireAuth(); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			emitToken(out, "SLACK_MCP_XOXP_TOKEN", cfg.XOXP)
			emitToken(out, "SLACK_MCP_XOXB_TOKEN", cfg.XOXB)
			emitToken(out, "SLACK_MCP_XOXC_TOKEN", cfg.XOXC)
			emitToken(out, "SLACK_MCP_XOXD_TOKEN", cfg.XOXD)
			return nil
		},
	}
}

func authStatusCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the resolved credential source and auth mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			switch {
			case cfg.HasExplicitEnv():
				fmt.Fprintln(out, "Source: SLACK_MCP_* environment variables")
			case cfg.Profile != "":
				fmt.Fprintf(out, "Source: profile %q (--profile)\n", cfg.Profile)
			default:
				store, err := credstore.Load()
				if err != nil {
					return err
				}
				if store.Default != "" {
					fmt.Fprintf(out, "Source: default profile %q\n", store.Default)
				} else {
					fmt.Fprintln(out, "Source: none (no profile and no SLACK_MCP_* env)")
				}
			}
			fmt.Fprintf(out, "Auth mode: %s\n", cfg.Mode())
			fmt.Fprintf(out, "GovSlack: %t\n", cfg.GovSlack)
			if err := cfg.RequireAuth(); err != nil {
				fmt.Fprintln(out, "Credentials: (not configured)")
			} else {
				fmt.Fprintln(out, "Credentials: configured")
			}
			fmt.Fprintf(out, "Keyring available: %t\n", credstore.Available())
			return nil
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
