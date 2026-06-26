// Package cmds wires Slack tool handlers as cobra subcommands. Each command
// resolves credentials, builds the reused ApiProvider, invokes the upstream
// handler in-process via internal/toolcall, and prints the result.
package cmds

import (
	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/internal/output"
	"github.com/paymog/slack-cli/internal/toolcall"
	"github.com/spf13/cobra"
)

// AddCommands registers every tool command group on the root command.
func AddCommands(root *cobra.Command, cfg *config.Config) {
	root.AddCommand(
		newChannelsCommand(cfg),
		newConversationsCommand(cfg),
		newUsersCommand(cfg),
		newUsergroupsCommand(cfg),
		newSavedCommand(cfg),
		newReactionsCommand(cfg),
		newAttachmentsCommand(cfg),
		newCacheCommand(cfg),
	)
}

// emit invokes a tool handler with args and prints its text result. Use it for
// handlers that return plain text or JSON; for handlers that return a CSV table
// use emitTable so the output is converted to JSON.
func emit(cmd *cobra.Command, cfg *config.Config, name string, h toolcall.Handler, args map[string]any) error {
	return emitFormat(cmd, cfg, name, h, args, false)
}

// emitTable is emit for handlers that return a CSV table; output.Print converts
// it to a JSON array of row objects so callers can pipe to jq.
func emitTable(cmd *cobra.Command, cfg *config.Config, name string, h toolcall.Handler, args map[string]any) error {
	return emitFormat(cmd, cfg, name, h, args, true)
}

func emitFormat(cmd *cobra.Command, cfg *config.Config, name string, h toolcall.Handler, args map[string]any, tabular bool) error {
	out, err := toolcall.Invoke(cmd.Context(), h, name, args)
	if err != nil {
		return err
	}
	return output.Print(cmd.OutOrStdout(), out, cfg.Raw, tabular)
}

// putIf adds key=val to args only when val is non-empty (string) — keeps the
// arguments map free of zero-value noise the handlers would otherwise read.
func putStr(args map[string]any, key, val string) {
	if val != "" {
		args[key] = val
	}
}
