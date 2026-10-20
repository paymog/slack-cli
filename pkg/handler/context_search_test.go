package handler

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnitResolveActionToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte(" file-token\n"), 0600))
	t.Setenv("SLACK_MCP_ACTION_TOKEN_FILE", path)
	token, err := ResolveActionToken("flag-token", true)
	require.NoError(t, err)
	assert.Equal(t, "flag-token", token)
	token, err = ResolveActionToken("", true)
	require.NoError(t, err)
	assert.Equal(t, "file-token", token)
	require.NoError(t, os.WriteFile(path, []byte("new-token\n"), 0600))
	token, err = ResolveActionToken("", true)
	require.NoError(t, err)
	assert.Equal(t, "new-token", token)
	t.Setenv("SLACK_MCP_ACTION_TOKEN_FILE", path+"missing")
	_, err = ResolveActionToken("flag-token", true)
	require.NoError(t, err)
	_, err = ResolveActionToken("", true)
	require.ErrorContains(t, err, "do not retry")
	t.Setenv("SLACK_MCP_ACTION_TOKEN_FILE", "")
	_, err = ResolveActionToken("", true)
	require.ErrorContains(t, err, "requires --action-token")
	_, err = ResolveActionToken("", false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(" \n"), 0600))
	t.Setenv("SLACK_MCP_ACTION_TOKEN_FILE", path)
	_, err = ResolveActionToken("", true)
	require.Error(t, err)
}

func TestUnitContextSearchParams(t *testing.T) {
	t.Setenv("SLACK_MCP_ACTION_TOKEN_FILE", "")
	var req mcp.CallToolRequest
	req.Params.Arguments = map[string]any{"query": "aws contract", "channel_types": "public_channel,private_channel", "content_types": "messages,files", "limit": 3, "cursor": "next", "sort": "timestamp", "after": "2024-01-01", "before": "2024-02-01", "include_context_messages": true, "action_token": "secret"}
	p, err := contextSearchParams(req, true)
	require.NoError(t, err)
	assert.Equal(t, "aws contract", p.Query)
	assert.Equal(t, []string{"public_channel", "private_channel"}, p.ChannelTypes)
	assert.Equal(t, []string{"messages", "files"}, p.ContentTypes)
	assert.Equal(t, 3, p.Limit)
	assert.Equal(t, "next", p.Cursor)
	assert.Equal(t, "timestamp", p.Sort)
	assert.Equal(t, int64(1704067200), p.After)
	assert.Equal(t, int64(1706745600), p.Before)
	assert.True(t, p.IncludeContextMessages)
	assert.Equal(t, "secret", p.ActionToken)
	for _, args := range []map[string]any{{"limit": 21}, {"limit": 0}, {"sort": "wrong"}, {"after": "yesterday"}, {"before": "2024-99-01"}} {
		args["query"] = "test"
		req.Params.Arguments = args
		_, err := contextSearchParams(req, false)
		require.Error(t, err)
	}
	req.Params.Arguments = map[string]any{"query": "test"}
	p, err = contextSearchParams(req, false)
	require.NoError(t, err)
	assert.Equal(t, 20, p.Limit)
	assert.Equal(t, "score", p.Sort)
	assert.Equal(t, []string{"public_channel"}, p.ChannelTypes)
	assert.Equal(t, []string{"messages"}, p.ContentTypes)
}

func TestUnitContextSearchError(t *testing.T) {
	for _, code := range []string{"invalid_action_token", "token_expired", "missing_scope"} {
		err := contextSearchError(errors.New(code))
		assert.Contains(t, err.Error(), "unavailable for this turn")
		assert.Contains(t, err.Error(), "do not retry")
	}
	err := errors.New("other")
	assert.Same(t, err, contextSearchError(err))
}
