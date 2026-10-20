package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/paymog/slack-cli/pkg/transport"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

// AssistantContextParameters extends slack-go v0.19's incomplete RTS parameters.
type AssistantContextParameters struct {
	slack.AssistantSearchContextParameters
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

// contextSearchClient supplies fields not yet exposed by slack-go and preserves
// response fields its decoder would otherwise discard. It is private to one call.
type contextSearchClient struct {
	base interface {
		Do(*http.Request) (*http.Response, error)
	}
	params AssistantContextParameters
	result AssistantContextResponse
}

func (c *contextSearchClient) Do(req *http.Request) (*http.Response, error) {
	if err := req.ParseForm(); err != nil {
		return nil, err
	}
	req.PostForm.Set("sort", c.params.Sort)
	if c.params.After != 0 {
		req.PostForm.Set("after", strconv.FormatInt(c.params.After, 10))
	}
	if c.params.Before != 0 {
		req.PostForm.Set("before", strconv.FormatInt(c.params.Before, 10))
	}
	req.PostForm.Set("include_context_messages", strconv.FormatBool(c.params.IncludeContextMessages))
	body := req.PostForm.Encode()
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
	req.GetBody = nil
	res, err := c.base.Do(req)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	res.Body = io.NopCloser(bytes.NewReader(data))
	if res.StatusCode == http.StatusOK {
		if err := json.Unmarshal(data, &c.result); err != nil {
			res.Body.Close()
			return nil, err
		}
	}
	return res, nil
}

func (c *MCPSlackClient) SearchAssistantContextContext(ctx context.Context, params AssistantContextParameters) (*AssistantContextResponse, error) {
	client := &contextSearchClient{base: transport.ProvideHTTPClient(c.authProvider.Cookies(), zap.NewNop()), params: params}
	opts := []slack.Option{slack.OptionHTTPClient(client)}
	if !c.isOAuth {
		opts = append(opts, slack.OptionAPIURL(c.teamEndpoint+"api/"))
	}
	if c.isOAuth && os.Getenv("SLACK_MCP_GOVSLACK") == "true" {
		opts = append(opts, slack.OptionAPIURL("https://slack-gov.com/api/"))
	}
	_, err := slack.New(c.authProvider.SlackToken(), opts...).SearchAssistantContextContext(ctx, params.AssistantSearchContextParameters)
	if err != nil {
		return nil, err
	}
	return &client.result, nil
}
