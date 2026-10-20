package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/paymog/slack-cli/pkg/transport"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

// AssistantContextParameters includes current RTS fields missing in slack-go v0.19.
type AssistantContextParameters struct {
	Query                  string
	ActionToken            string
	ChannelTypes           []string
	ContentTypes           []string
	Limit                  int
	Cursor                 string
	Sort                   string
	After                  int64
	Before                 int64
	IncludeContextMessages bool
}

type AssistantContextMessage struct {
	slack.AssistantSearchContextMessage
	ContextMessages json.RawMessage `json:"context_messages,omitempty"`
}

type AssistantContextResponse struct {
	slack.SlackResponse
	Results struct {
		Messages []AssistantContextMessage `json:"messages"`
		Files    []json.RawMessage         `json:"files"`
		Channels []json.RawMessage         `json:"channels"`
		Users    []json.RawMessage         `json:"users"`
	} `json:"results"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
	NextCursor string `json:"next_cursor"`
}

func searchAssistantContext(ctx context.Context, client *http.Client, base, token string, p AssistantContextParameters) (*AssistantContextResponse, error) {
	form := url.Values{
		"token": {token}, "query": {p.Query},
		"channel_types": {strings.Join(p.ChannelTypes, ",")},
		"content_types": {strings.Join(p.ContentTypes, ",")},
		"limit":         {strconv.Itoa(p.Limit)}, "sort": {p.Sort},
		"include_context_messages": {strconv.FormatBool(p.IncludeContextMessages)},
	}
	if p.ActionToken != "" {
		form.Set("action_token", p.ActionToken)
	}
	if p.Cursor != "" {
		form.Set("cursor", p.Cursor)
	}
	if p.After != 0 {
		form.Set("after", strconv.FormatInt(p.After, 10))
	}
	if p.Before != 0 {
		form.Set("before", strconv.FormatInt(p.Before, 10))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"assistant.search.context", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// The shared transport moves OAuth tokens from the form to a Bearer header,
	// while preserving browser-token authentication and cookies.
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("assistant.search.context HTTP status %d", res.StatusCode)
	}
	var result AssistantContextResponse
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}
	if err := result.Err(); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *MCPSlackClient) SearchAssistantContextContext(ctx context.Context, params AssistantContextParameters) (*AssistantContextResponse, error) {
	base := "https://slack.com/api/"
	if !c.isOAuth {
		base = c.teamEndpoint + "api/"
	} else if os.Getenv("SLACK_MCP_GOVSLACK") == "true" {
		base = "https://slack-gov.com/api/"
	}
	client := transport.ProvideHTTPClient(c.authProvider.Cookies(), zap.NewNop())
	return searchAssistantContext(ctx, client, base, c.authProvider.SlackToken(), params)
}
