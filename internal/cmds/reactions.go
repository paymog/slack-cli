package cmds

import (
	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/internal/runtime"
	"github.com/paymog/slack-cli/internal/toolcall"
	"github.com/paymog/slack-cli/pkg/handler"
	"github.com/spf13/cobra"
)

func newReactionsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reactions",
		Aliases: []string{"reaction"},
		Short:   "Add or remove emoji reactions (requires SLACK_MCP_REACTION_TOOL)",
	}
	cmd.AddCommand(
		reactionCommand(cfg, "add", "Add an emoji reaction to a message", "reactions_add",
			func(h *handler.ConversationsHandler) toolcall.Handler { return h.ReactionsAddHandler }),
		reactionCommand(cfg, "remove", "Remove an emoji reaction from a message", "reactions_remove",
			func(h *handler.ConversationsHandler) toolcall.Handler { return h.ReactionsRemoveHandler }),
	)
	return cmd
}

func reactionCommand(cfg *config.Config, use, short, tool string, pick func(*handler.ConversationsHandler) toolcall.Handler) *cobra.Command {
	var emoji string
	cmd := &cobra.Command{
		Use:   use + " <channel> <timestamp>",
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewConversationsHandler(p, logger)
			a := map[string]any{
				"channel_id": args[0],
				"timestamp":  args[1],
				"emoji":      emoji,
			}
			return emit(cmd, cfg, tool, pick(h), a)
		},
	}
	cmd.Flags().StringVar(&emoji, "emoji", "", "emoji name without colons (e.g. thumbsup) (required)")
	_ = cmd.MarkFlagRequired("emoji")
	return cmd
}
