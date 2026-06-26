package cmds

import (
	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/internal/runtime"
	"github.com/paymog/slack-cli/pkg/handler"
	"github.com/spf13/cobra"
)

func newChannelsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "channels",
		Aliases: []string{"channel"},
		Short:   "List Slack channels",
	}
	cmd.AddCommand(channelsListCommand(cfg), channelsMeCommand(cfg))
	return cmd
}

func channelsListCommand(cfg *config.Config) *cobra.Command {
	var (
		types        string
		sortBy       string
		cursor       string
		query        string
		queryTargets string
		limit        int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List channels (public/private/im/mpim), as JSON",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewChannelsHandler(p, logger)
			args := map[string]any{
				"channel_types": types,
				"sort":          sortBy,
			}
			putStr(args, "cursor", cursor)
			if limit != 0 {
				args["limit"] = limit
			}
			if query != "" {
				args["query"] = query
				args["query_targets"] = queryTargets
			}
			return emitTable(cmd, cfg, "channels_list", h.ChannelsHandler, args)
		},
	}
	f := cmd.Flags()
	f.StringVar(&types, "types", "public_channel,private_channel", "comma-separated channel types: public_channel,private_channel,im,mpim")
	f.StringVar(&sortBy, "sort", "popularity", "sort order (popularity)")
	f.StringVar(&cursor, "cursor", "", "pagination cursor from a previous page")
	f.IntVar(&limit, "limit", 0, "max channels to return (default 100, max 999)")
	f.StringVar(&query, "query", "", "filter channels by keyword")
	f.StringVar(&queryTargets, "query-targets", "name", "comma-separated query targets: name,topic,purpose")
	return cmd
}

func channelsMeCommand(cfg *config.Config) *cobra.Command {
	var (
		types  string
		cursor string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "me",
		Short: "List channels the authenticated user belongs to, as JSON",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewChannelsHandler(p, logger)
			args := map[string]any{"channel_types": types}
			putStr(args, "cursor", cursor)
			if limit != 0 {
				args["limit"] = limit
			}
			return emitTable(cmd, cfg, "channels_me", h.ChannelsMeHandler, args)
		},
	}
	f := cmd.Flags()
	f.StringVar(&types, "types", "public_channel,private_channel", "comma-separated channel types")
	f.StringVar(&cursor, "cursor", "", "pagination cursor from a previous page")
	f.IntVar(&limit, "limit", 0, "max channels to return (default 100, max 999)")
	return cmd
}
