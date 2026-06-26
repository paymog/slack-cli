package cmds

import (
	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/internal/runtime"
	"github.com/paymog/slack-cli/pkg/handler"
	"github.com/spf13/cobra"
)

func newConversationsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "conversations",
		Aliases: []string{"conv"},
		Short:   "Read, search, and post conversation messages",
	}
	cmd.AddCommand(
		conversationsHistoryCommand(cfg),
		conversationsRepliesCommand(cfg),
		conversationsSearchCommand(cfg),
		conversationsAddCommand(cfg),
		conversationsMarkCommand(cfg),
		conversationsUnreadsCommand(cfg),
		conversationsJoinCommand(cfg),
		conversationsLeaveCommand(cfg),
	)
	return cmd
}

func conversationsHistoryCommand(cfg *config.Config) *cobra.Command {
	var limit, cursor string
	var activity bool
	cmd := &cobra.Command{
		Use:   "history <channel>",
		Short: "Get channel/DM messages as JSON (channel ID, #name, or @dm)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewConversationsHandler(p, logger)
			a := map[string]any{
				"channel_id":                args[0],
				"include_activity_messages": activity,
			}
			putStr(a, "limit", limit)
			putStr(a, "cursor", cursor)
			return emitTable(cmd, cfg, "conversations_history", h.ConversationsHistoryHandler, a)
		},
	}
	f := cmd.Flags()
	f.StringVar(&limit, "limit", "1d", "time window (1d,1w,30d) or message count (e.g. 50); pass --limit='' with --cursor")
	f.StringVar(&cursor, "cursor", "", "pagination cursor from a previous page")
	f.BoolVar(&activity, "activity", false, "include activity messages (joins/leaves)")
	return cmd
}

func conversationsRepliesCommand(cfg *config.Config) *cobra.Command {
	var limit, cursor string
	var activity bool
	cmd := &cobra.Command{
		Use:   "replies <channel> <thread_ts>",
		Short: "Get a thread's replies as JSON",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewConversationsHandler(p, logger)
			a := map[string]any{
				"channel_id":                args[0],
				"thread_ts":                 args[1],
				"include_activity_messages": activity,
			}
			putStr(a, "limit", limit)
			putStr(a, "cursor", cursor)
			return emitTable(cmd, cfg, "conversations_replies", h.ConversationsRepliesHandler, a)
		},
	}
	f := cmd.Flags()
	f.StringVar(&limit, "limit", "1d", "time window or message count; pass --limit='' with --cursor")
	f.StringVar(&cursor, "cursor", "", "pagination cursor from a previous page")
	f.BoolVar(&activity, "activity", false, "include activity messages (joins/leaves)")
	return cmd
}

func conversationsSearchCommand(cfg *config.Config) *cobra.Command {
	var (
		inChannel, inIMMPIM, with, from string
		before, after, on, during       string
		cursor                          string
		threadsOnly                     bool
		limit                           int
	)
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search messages with optional filters, as JSON",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewConversationsHandler(p, logger)
			a := map[string]any{
				"filter_threads_only": threadsOnly,
				"limit":               limit,
			}
			if len(args) == 1 {
				a["search_query"] = args[0]
			}
			putStr(a, "filter_in_channel", inChannel)
			putStr(a, "filter_in_im_or_mpim", inIMMPIM)
			putStr(a, "filter_users_with", with)
			putStr(a, "filter_users_from", from)
			putStr(a, "filter_date_before", before)
			putStr(a, "filter_date_after", after)
			putStr(a, "filter_date_on", on)
			putStr(a, "filter_date_during", during)
			putStr(a, "cursor", cursor)
			return emitTable(cmd, cfg, "conversations_search_messages", h.ConversationsSearchHandler, a)
		},
	}
	f := cmd.Flags()
	f.StringVar(&inChannel, "in-channel", "", "limit to a channel (ID or #name)")
	f.StringVar(&inIMMPIM, "in-dm", "", "limit to a DM/group DM (ID or @user)")
	f.StringVar(&with, "with", "", "messages with a user (ID or @user)")
	f.StringVar(&from, "from", "", "messages from a user (ID or @user)")
	f.StringVar(&before, "before", "", "before date YYYY-MM-DD (or Today/Yesterday)")
	f.StringVar(&after, "after", "", "after date YYYY-MM-DD")
	f.StringVar(&on, "on", "", "on date YYYY-MM-DD")
	f.StringVar(&during, "during", "", "during period (e.g. July)")
	f.BoolVar(&threadsOnly, "threads-only", false, "only thread messages")
	f.IntVar(&limit, "limit", 20, "max results (1-100)")
	f.StringVar(&cursor, "cursor", "", "pagination cursor from a previous page")
	return cmd
}

func conversationsAddCommand(cfg *config.Config) *cobra.Command {
	var text, threadTS, contentType, blocks string
	cmd := &cobra.Command{
		Use:   "add <channel>",
		Short: "Post a message (requires SLACK_MCP_ADD_MESSAGE_TOOL)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewConversationsHandler(p, logger)
			a := map[string]any{
				"channel_id":   args[0],
				"content_type": contentType,
			}
			putStr(a, "text", text)
			putStr(a, "thread_ts", threadTS)
			putStr(a, "blocks", blocks)
			return emit(cmd, cfg, "conversations_add_message", h.ConversationsAddMessageHandler, a)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&text, "text", "t", "", "message text (text/markdown or text/plain)")
	f.StringVar(&threadTS, "thread-ts", "", "post into this thread (timestamp 1234567890.123456)")
	f.StringVar(&contentType, "content-type", "text/markdown", "text/markdown or text/plain")
	f.StringVar(&blocks, "blocks", "", "raw Slack Block Kit JSON array (overrides text rendering)")
	return cmd
}

func conversationsMarkCommand(cfg *config.Config) *cobra.Command {
	var ts string
	cmd := &cobra.Command{
		Use:   "mark <channel>",
		Short: "Mark a channel/DM as read (requires SLACK_MCP_MARK_TOOL)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewConversationsHandler(p, logger)
			a := map[string]any{"channel_id": args[0]}
			putStr(a, "ts", ts)
			return emit(cmd, cfg, "conversations_mark", h.ConversationsMarkHandler, a)
		},
	}
	cmd.Flags().StringVar(&ts, "ts", "", "mark read up to this message timestamp (default: all)")
	return cmd
}

func conversationsUnreadsCommand(cfg *config.Config) *cobra.Command {
	var (
		types                    string
		maxChannels, maxPerChan  int
		includeMessages          bool
		mentionsOnly, includeMtd bool
	)
	cmd := &cobra.Command{
		Use:   "unreads",
		Short: "Get unread messages across channels, prioritized",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewConversationsHandler(p, logger)
			a := map[string]any{
				"include_messages":         includeMessages,
				"channel_types":            types,
				"max_channels":             maxChannels,
				"max_messages_per_channel": maxPerChan,
				"mentions_only":            mentionsOnly,
				"include_muted":            includeMtd,
			}
			return emitTable(cmd, cfg, "conversations_unreads", h.ConversationsUnreadsHandler, a)
		},
	}
	f := cmd.Flags()
	f.StringVar(&types, "types", "all", "all|dm|group_dm|partner|internal")
	f.IntVar(&maxChannels, "max-channels", 50, "max channels to scan")
	f.IntVar(&maxPerChan, "max-messages-per-channel", 10, "max messages per channel")
	f.BoolVar(&includeMessages, "include-messages", true, "include message bodies")
	f.BoolVar(&mentionsOnly, "mentions-only", false, "only channels with @mentions (browser tokens)")
	f.BoolVar(&includeMtd, "include-muted", false, "include muted channels")
	return cmd
}

func conversationsJoinCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "join <channel>",
		Short: "Join a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewConversationsHandler(p, logger)
			return emit(cmd, cfg, "conversations_join", h.ConversationsJoinHandler, map[string]any{"channel_id": args[0]})
		},
	}
}

func conversationsLeaveCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "leave <channel>",
		Short: "Leave a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewConversationsHandler(p, logger)
			return emit(cmd, cfg, "conversations_leave", h.ConversationsLeaveHandler, map[string]any{"channel_id": args[0]})
		},
	}
}
