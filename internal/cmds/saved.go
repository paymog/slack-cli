package cmds

import (
	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/internal/runtime"
	"github.com/paymog/slack-cli/pkg/handler"
	"github.com/spf13/cobra"
)

func newSavedCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "saved",
		Short: "Manage Slack's \"Save for Later\" items (browser tokens only)",
	}
	cmd.AddCommand(
		savedListCommand(cfg),
		savedUpdateCommand(cfg),
		savedClearCompletedCommand(cfg),
	)
	return cmd
}

// savedHandler builds the SavedHandler, which needs a ConversationsHandler to
// hydrate message bodies.
func savedHandler(cfg *config.Config, cmd *cobra.Command) (*handler.SavedHandler, error) {
	p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
	if err != nil {
		return nil, err
	}
	conv := handler.NewConversationsHandler(p, logger)
	return handler.NewSavedHandler(p, logger, conv), nil
}

func savedListCommand(cfg *config.Config) *cobra.Command {
	var (
		filter          string
		limit           int
		includeMessages bool
		maxPerItem      int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved items",
		RunE: func(cmd *cobra.Command, _ []string) error {
			h, err := savedHandler(cfg, cmd)
			if err != nil {
				return err
			}
			a := map[string]any{
				"filter":                filter,
				"limit":                 limit,
				"include_messages":      includeMessages,
				"max_messages_per_item": maxPerItem,
			}
			return emitTable(cmd, cfg, "saved_list", h.SavedListHandler, a)
		},
	}
	f := cmd.Flags()
	f.StringVar(&filter, "filter", "saved", "saved|completed|archived")
	f.IntVar(&limit, "limit", 50, "max items (auto-paginates)")
	f.BoolVar(&includeMessages, "include-messages", true, "fetch saved message content")
	f.IntVar(&maxPerItem, "max-messages-per-item", 5, "max messages per saved item")
	return cmd
}

func savedUpdateCommand(cfg *config.Config) *cobra.Command {
	var (
		mark    string
		dateDue int
	)
	cmd := &cobra.Command{
		Use:   "update <item_id> <ts>",
		Short: "Mark a saved item complete and/or set a due date",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := savedHandler(cfg, cmd)
			if err != nil {
				return err
			}
			a := map[string]any{"item_id": args[0], "ts": args[1]}
			putStr(a, "mark", mark)
			if cmd.Flags().Changed("date-due") {
				a["date_due"] = dateDue
			}
			return emit(cmd, cfg, "saved_update", h.SavedUpdateHandler, a)
		},
	}
	f := cmd.Flags()
	f.StringVar(&mark, "mark", "", "set to \"completed\" to mark done")
	f.IntVar(&dateDue, "date-due", 0, "due date as unix timestamp (0 clears)")
	return cmd
}

func savedClearCompletedCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "clear-completed",
		Short: "Remove all completed saved items",
		RunE: func(cmd *cobra.Command, _ []string) error {
			h, err := savedHandler(cfg, cmd)
			if err != nil {
				return err
			}
			return emit(cmd, cfg, "saved_clear_completed", h.SavedClearCompletedHandler, map[string]any{})
		},
	}
}
