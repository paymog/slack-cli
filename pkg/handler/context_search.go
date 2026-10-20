package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gocarina/gocsv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/paymog/slack-cli/pkg/provider"
)

// ResolveActionToken reads the file afresh, allowing rotation between invocations.
// Errors deliberately omit file contents and token values.
func ResolveActionToken(flag string, bot bool) (string, error) {
	token := strings.TrimSpace(flag)
	if token == "" {
		if path := os.Getenv("SLACK_MCP_ACTION_TOKEN_FILE"); path != "" {
			data, err := os.ReadFile(path)
			if err != nil {
				return "", errors.New("cannot read SLACK_MCP_ACTION_TOKEN_FILE; search is unavailable for this turn; do not retry")
			}
			token = strings.TrimSpace(string(data))
		}
	}
	if bot && token == "" {
		return "", errors.New("bot context-search requires --action-token or SLACK_MCP_ACTION_TOKEN_FILE; search is unavailable for this turn; do not retry")
	}
	return token, nil
}

func contextSearchParams(req mcp.CallToolRequest, bot bool) (provider.AssistantContextParameters, error) {
	p := provider.AssistantContextParameters{
		Query: req.GetString("query", ""), ChannelTypes: strings.Split(req.GetString("channel_types", "public_channel"), ","),
		ContentTypes: strings.Split(req.GetString("content_types", "messages"), ","), Limit: req.GetInt("limit", 20), Cursor: req.GetString("cursor", ""),
		Sort: req.GetString("sort", "score"), IncludeContextMessages: req.GetBool("include_context_messages", false)}
	if strings.TrimSpace(p.Query) == "" {
		return p, errors.New("context-search requires a query")
	}
	if p.Limit < 1 || p.Limit > 20 {
		return p, errors.New("--limit must be between 1 and 20")
	}
	if p.Sort != "score" && p.Sort != "timestamp" {
		return p, errors.New("--sort must be score or timestamp")
	}
	for _, filter := range []struct {
		name   string
		target *int64
	}{{"after", &p.After}, {"before", &p.Before}} {
		if value := req.GetString(filter.name, ""); value != "" {
			date, err := time.Parse("2006-01-02", value)
			if err != nil {
				return p, fmt.Errorf("--%s must be YYYY-MM-DD", filter.name)
			}
			*filter.target = date.Unix()
		}
	}
	var err error
	p.ActionToken, err = ResolveActionToken(req.GetString("action_token", ""), bot)
	return p, err
}

func contextSearchError(err error) error {
	switch err.Error() {
	case "invalid_action_token", "token_expired":
		return errors.New("action token is missing, invalid, or expired; search is unavailable for this turn; do not retry")
	case "missing_scope":
		return errors.New("Slack token lacks the required Real-time Search scopes (search:read.public for public messages); search is unavailable for this turn; do not retry")
	default:
		return err
	}
}

type contextSearchRow struct {
	ChannelID       string
	ChannelName     string
	Ts              string
	Permalink       string
	Author          string
	Text            string
	ContextMessages string
	ContentType     string
	Data            string
	Cursor          string
}

func (ch *ConversationsHandler) ConversationsContextSearchHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Never log request arguments: they contain the short-lived action token.
	params, err := contextSearchParams(req, ch.apiProvider.IsBotToken())
	if err != nil {
		return nil, err
	}
	response, err := ch.apiProvider.Slack().SearchAssistantContextContext(ctx, params)
	if err != nil {
		return nil, contextSearchError(err)
	}
	rows := make([]contextSearchRow, 0, len(response.Results.Messages))
	channels := ch.apiProvider.ProvideChannelsMaps().Channels
	for _, msg := range response.Results.Messages {
		row := contextSearchRow{ChannelID: msg.ChannelID, ChannelName: channels[msg.ChannelID].Name, Ts: msg.MessageTS, Permalink: msg.Permalink, Author: msg.AuthorUserID, Text: msg.Content, ContentType: "messages"}
		if params.IncludeContextMessages {
			row.ContextMessages = string(msg.ContextMessages)
		}
		rows = append(rows, row)
	}
	for _, group := range []struct {
		kind  string
		items []json.RawMessage
	}{{"files", response.Results.Files}, {"channels", response.Results.Channels}, {"users", response.Results.Users}} {
		for _, item := range group.items {
			rows = append(rows, contextSearchRow{ContentType: group.kind, Data: string(item)})
		}
	}
	cursor := response.ResponseMetadata.NextCursor
	if cursor == "" {
		cursor = response.NextCursor
	}
	if cursor != "" {
		if len(rows) == 0 {
			rows = append(rows, contextSearchRow{})
		}
		rows[len(rows)-1].Cursor = cursor
	}
	data, err := gocsv.MarshalBytes(&rows)
	if err != nil {
		return nil, err
	}
	return mcp.NewToolResultText(string(data)), nil
}
