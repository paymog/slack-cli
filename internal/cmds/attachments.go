package cmds

import (
	"github.com/paymog/slack-cli/internal/config"
	"github.com/paymog/slack-cli/internal/runtime"
	"github.com/paymog/slack-cli/pkg/handler"
	"github.com/spf13/cobra"
)

func newAttachmentsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "attachments",
		Aliases: []string{"attachment", "files"},
		Short:   "Download attachment data (requires SLACK_MCP_ATTACHMENT_TOOL)",
	}
	cmd.AddCommand(attachmentsGetCommand(cfg))
	return cmd
}

func attachmentsGetCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "get <file_id>",
		Short: "Download an attachment by file ID (Fxxxxxxxxxx); max 5MB",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, logger, err := runtime.PrepareRead(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			h := handler.NewConversationsHandler(p, logger)
			return emit(cmd, cfg, "attachment_get_data", h.FilesGetHandler, map[string]any{"file_id": args[0]})
		},
	}
}
