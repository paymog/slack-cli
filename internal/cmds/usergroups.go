package cmds

import (
	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/internal/runtime"
	"github.com/paymog/slack-cli/pkg/handler"
	"github.com/spf13/cobra"
)

func newUsergroupsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "usergroups",
		Aliases: []string{"usergroup"},
		Short:   "Manage user groups (subteams)",
	}
	cmd.AddCommand(
		usergroupsListCommand(cfg),
		usergroupsMeCommand(cfg),
		usergroupsCreateCommand(cfg),
		usergroupsUpdateCommand(cfg),
		usergroupsUsersUpdateCommand(cfg),
	)
	return cmd
}

func usergroupsListCommand(cfg *config.Config) *cobra.Command {
	var includeUsers, includeCount, includeDisabled bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List user groups as JSON",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewUsergroupsHandler(p, logger)
			a := map[string]any{
				"include_users":    includeUsers,
				"include_count":    includeCount,
				"include_disabled": includeDisabled,
			}
			return emitTable(cmd, cfg, "usergroups_list", h.UsergroupsListHandler, a)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&includeUsers, "include-users", false, "include member user IDs per group")
	f.BoolVar(&includeCount, "include-count", true, "include member count per group")
	f.BoolVar(&includeDisabled, "include-disabled", false, "include disabled/archived groups")
	return cmd
}

func usergroupsMeCommand(cfg *config.Config) *cobra.Command {
	var usergroupID string
	cmd := &cobra.Command{
		Use:   "me <list|join|leave>",
		Short: "List your groups, or join/leave a group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewUsergroupsHandler(p, logger)
			a := map[string]any{"action": args[0]}
			putStr(a, "usergroup_id", usergroupID)
			return emitTable(cmd, cfg, "usergroups_me", h.UsergroupsMeHandler, a)
		},
	}
	cmd.Flags().StringVar(&usergroupID, "usergroup-id", "", "group ID (required for join/leave)")
	return cmd
}

func usergroupsCreateCommand(cfg *config.Config) *cobra.Command {
	var name, handleFlag, description, channels string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a user group",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewUsergroupsHandler(p, logger)
			a := map[string]any{"name": name}
			putStr(a, "handle", handleFlag)
			putStr(a, "description", description)
			putStr(a, "channels", channels)
			return emit(cmd, cfg, "usergroups_create", h.UsergroupsCreateHandler, a)
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "group name (required)")
	f.StringVar(&handleFlag, "handle", "", "mention handle without @ (auto-generated if empty)")
	f.StringVar(&description, "description", "", "group description")
	f.StringVar(&channels, "channels", "", "comma-separated default channel IDs")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func usergroupsUpdateCommand(cfg *config.Config) *cobra.Command {
	var name, handleFlag, description, channels string
	cmd := &cobra.Command{
		Use:   "update <usergroup_id>",
		Short: "Update a user group's metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewUsergroupsHandler(p, logger)
			a := map[string]any{"usergroup_id": args[0]}
			putStr(a, "name", name)
			putStr(a, "handle", handleFlag)
			putStr(a, "description", description)
			putStr(a, "channels", channels)
			return emit(cmd, cfg, "usergroups_update", h.UsergroupsUpdateHandler, a)
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "new group name")
	f.StringVar(&handleFlag, "handle", "", "new mention handle")
	f.StringVar(&description, "description", "", "new description")
	f.StringVar(&channels, "channels", "", "new default channels (comma-separated IDs, replaces existing)")
	return cmd
}

func usergroupsUsersUpdateCommand(cfg *config.Config) *cobra.Command {
	var users string
	cmd := &cobra.Command{
		Use:   "users-update <usergroup_id>",
		Short: "Replace a user group's members",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewUsergroupsHandler(p, logger)
			a := map[string]any{"usergroup_id": args[0], "users": users}
			return emit(cmd, cfg, "usergroups_users_update", h.UsergroupsUsersUpdateHandler, a)
		},
	}
	cmd.Flags().StringVar(&users, "users", "", "comma-separated user IDs (required, e.g. U123,U456)")
	_ = cmd.MarkFlagRequired("users")
	return cmd
}
