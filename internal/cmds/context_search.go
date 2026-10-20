package cmds

import (
	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/internal/runtime"
	"github.com/paymog/slack-cli/pkg/handler"
	"github.com/spf13/cobra"
)

func conversationsContextSearchCommand(cfg *config.Config) *cobra.Command {
	var channelTypes, contentTypes, cursor, sort, after, before, actionToken string
	var limit int
	var includeContext bool
	cmd := &cobra.Command{
		Use:   "context-search [query]",
		Short: "Search Slack with Real-time Search (bots require an action token)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewConversationsHandler(p, logger)
			return emitTable(cmd, cfg, "conversations_context_search", h.ConversationsContextSearchHandler, map[string]any{
				"query": args[0], "channel_types": channelTypes, "content_types": contentTypes,
				"limit": limit, "cursor": cursor, "sort": sort, "after": after, "before": before,
				"include_context_messages": includeContext, "action_token": actionToken,
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&channelTypes, "channel-types", "public_channel", "comma-separated channel types (public_channel,private_channel,im,mpim)")
	f.StringVar(&contentTypes, "content-types", "messages", "comma-separated content types (messages,files,channels,users)")
	f.IntVar(&limit, "limit", 20, "max results (1-20)")
	f.StringVar(&cursor, "cursor", "", "pagination cursor from a previous page")
	f.StringVar(&sort, "sort", "score", "sort by score or timestamp")
	f.StringVar(&after, "after", "", "after date YYYY-MM-DD (UTC)")
	f.StringVar(&before, "before", "", "before date YYYY-MM-DD (UTC)")
	f.BoolVar(&includeContext, "include-context-messages", false, "include surrounding messages")
	f.StringVar(&actionToken, "action-token", "", "short-lived Slack event action token (overrides SLACK_MCP_ACTION_TOKEN_FILE)")
	return cmd
}
