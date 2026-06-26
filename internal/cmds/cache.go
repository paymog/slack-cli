package cmds

import (
	"errors"
	"fmt"

	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/internal/runtime"
	"github.com/paymog/slack-cli/pkg/provider"
	"github.com/spf13/cobra"
)

func newCacheCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the on-disk user/channel cache shared across invocations",
	}
	cmd.AddCommand(cacheRefreshCommand(cfg))
	return cmd
}

func cacheRefreshCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Fetch users and channels from Slack and write the cache to disk",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, _, err := runtime.Provider(cfg)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if err := p.ForceRefreshUsers(ctx); err != nil && !errors.Is(err, provider.ErrRefreshRateLimited) {
				return fmt.Errorf("refresh users: %w", err)
			}
			if err := p.ForceRefreshChannels(ctx); err != nil && !errors.Is(err, provider.ErrRefreshRateLimited) {
				return fmt.Errorf("refresh channels: %w", err)
			}
			users := p.ProvideUsersMap()
			channels := p.ProvideChannelsMaps()
			fmt.Fprintf(cmd.OutOrStdout(), "Cache refreshed: %d users, %d channels\n", len(users.Users), len(channels.Channels))
			return nil
		},
	}
}
