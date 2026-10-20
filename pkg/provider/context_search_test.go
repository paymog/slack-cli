package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnitContextSearchHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "/assistant.search.context", r.URL.Path)
		for key, value := range map[string]string{"query": "aws contract", "action_token": "secret", "limit": "3", "cursor": "page", "sort": "timestamp", "after": "1704067200", "before": "1706745600", "include_context_messages": "true"} {
			assert.Equal(t, value, r.PostForm.Get(key), key)
		}
		assert.Equal(t, "xoxp-test", r.PostForm.Get("token"))
		assert.Equal(t, "public_channel,private_channel", r.PostForm.Get("channel_types"))
		assert.Equal(t, "messages,files", r.PostForm.Get("content_types"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"results":{"messages":[{"channel_id":"C1","message_ts":"1.2","author_user_id":"U1","content":"message","permalink":"https://example.com","context_messages":{"before":[{"content":"context"}],"after":[]}}],"files":[{"id":"F1"}],"channels":[{"id":"C1"}],"users":[{"id":"U1"}]},"response_metadata":{"next_cursor":"next"}}`)
	}))
	defer server.Close()
	p := AssistantContextParameters{Query: "aws contract", ActionToken: "secret", ChannelTypes: []string{"public_channel", "private_channel"}, ContentTypes: []string{"messages", "files"}, Limit: 3, Cursor: "page", Sort: "timestamp", After: 1704067200, Before: 1706745600, IncludeContextMessages: true}
	result, err := searchAssistantContext(context.Background(), server.Client(), server.URL+"/", "xoxp-test", p)
	require.NoError(t, err)
	require.Len(t, result.Results.Messages, 1)
	assert.JSONEq(t, `{"before":[{"content":"context"}],"after":[]}`, string(result.Results.Messages[0].ContextMessages))
	assert.Len(t, result.Results.Files, 1)
	assert.Len(t, result.Results.Channels, 1)
	assert.Len(t, result.Results.Users, 1)
	assert.Equal(t, "next", result.ResponseMetadata.NextCursor)
}

func TestUnitContextSearchHTTPFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"invalid token", 200, `{"ok":false,"error":"invalid_action_token"}`, "invalid_action_token"},
		{"expired token", 200, `{"ok":false,"error":"token_expired"}`, "token_expired"},
		{"missing scope", 200, `{"ok":false,"error":"missing_scope"}`, "missing_scope"},
		{"http failure", 503, `unavailable`, "HTTP status 503"},
		{"malformed response", 200, `{`, "unexpected EOF"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			_, err := searchAssistantContext(context.Background(), server.Client(), server.URL+"/", "token", AssistantContextParameters{Query: "test"})
			require.ErrorContains(t, err, tc.want)
		})
	}
}
