package cmds

import (
	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/internal/runtime"
	"github.com/paymog/slack-cli/pkg/handler"
	"github.com/spf13/cobra"
)

func newUsersCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "users",
		Aliases: []string{"user"},
		Short:   "Search workspace users",
	}
	cmd.AddCommand(usersSearchCommand(cfg))
	return cmd
}

func usersSearchCommand(cfg *config.Config) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search users by name, email, or display name, as CSV",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewConversationsHandler(p, logger)
			a := map[string]any{"query": args[0], "limit": limit}
			return emit(cmd, cfg, "users_search", h.UsersSearchHandler, a)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "max results (1-100)")
	return cmd
}
