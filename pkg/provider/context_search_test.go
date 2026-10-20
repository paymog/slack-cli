package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnitContextSearchSDKAdapter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "/assistant.search.context", r.URL.Path)
		for key, value := range map[string]string{"query": "aws contract", "action_token": "secret", "limit": "3", "cursor": "page", "sort": "timestamp", "after": "1704067200", "before": "1706745600", "include_context_messages": "true"} {
			assert.Equal(t, value, r.PostForm.Get(key), key)
		}
		assert.Equal(t, []string{"public_channel", "private_channel"}, r.PostForm["channel_types"])
		assert.Equal(t, []string{"messages", "files"}, r.PostForm["content_types"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"results":{"messages":[{"channel_id":"C1","message_ts":"1.2","author_user_id":"U1","content":"message","permalink":"https://example.com","context_messages":{"before":[{"content":"context"}],"after":[]}}],"files":[{"id":"F1"}],"channels":[{"id":"C1"}],"users":[{"id":"U1"}]},"response_metadata":{"next_cursor":"next"}}`)
	}))
	defer server.Close()
	p := AssistantContextParameters{AssistantSearchContextParameters: slack.AssistantSearchContextParameters{Query: "aws contract", ActionToken: "secret", ChannelTypes: []string{"public_channel", "private_channel"}, ContentTypes: []string{"messages", "files"}, Limit: 3, Cursor: "page"}, Sort: "timestamp", After: 1704067200, Before: 1706745600, IncludeContextMessages: true}
	adapter := &contextSearchClient{base: server.Client(), params: p}
	sdk := slack.New("xoxp-test", slack.OptionAPIURL(server.URL+"/"), slack.OptionHTTPClient(adapter))
	_, err := sdk.SearchAssistantContextContext(context.Background(), p.AssistantSearchContextParameters)
	require.NoError(t, err)
	require.Len(t, adapter.result.Results.Messages, 1)
	assert.JSONEq(t, `{"before":[{"content":"context"}],"after":[]}`, string(adapter.result.Results.Messages[0].ContextMessages))
	assert.Len(t, adapter.result.Results.Files, 1)
	assert.Len(t, adapter.result.Results.Channels, 1)
	assert.Len(t, adapter.result.Results.Users, 1)
	assert.Equal(t, "next", adapter.result.ResponseMetadata.NextCursor)
}
