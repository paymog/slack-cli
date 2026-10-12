package handler

import (
	"errors"
	"testing"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type repliesPage struct {
	msgs []string
	next string
	err  error
}

// pagedFetch serves pages keyed by the cursor that requests them and records
// the cursors it was called with.
func pagedFetch(pages map[string]repliesPage, calls *[]string) func(string) ([]slack.Message, bool, string, error) {
	return func(cursor string) ([]slack.Message, bool, string, error) {
		*calls = append(*calls, cursor)
		p := pages[cursor]
		if p.err != nil {
			return nil, false, "", p.err
		}
		out := make([]slack.Message, 0, len(p.msgs))
		for _, ts := range p.msgs {
			out = append(out, slack.Message{Msg: slack.Msg{Timestamp: ts}})
		}
		return out, p.next != "", p.next, nil
	}
}

func timestamps(msgs []slack.Message) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.Timestamp)
	}
	return out
}

func TestUnitCollectThreadReplies_FollowsCursorsAndDedupesParent(t *testing.T) {
	var calls []string
	fetch := pagedFetch(map[string]repliesPage{
		"":   {msgs: []string{"1.0", "2.0"}, next: "c1"},
		"c1": {msgs: []string{"1.0", "3.0"}, next: "c2"}, // parent repeated on later pages
		"c2": {msgs: []string{"1.0", "4.0"}},
	}, &calls)

	msgs, next, err := collectThreadReplies(fetch, 100)
	require.NoError(t, err)
	assert.Equal(t, []string{"1.0", "2.0", "3.0", "4.0"}, timestamps(msgs))
	assert.Empty(t, next)
	assert.Equal(t, []string{"", "c1", "c2"}, calls)
}

func TestUnitCollectThreadReplies_StopsAtCapWithResumeCursor(t *testing.T) {
	var calls []string
	fetch := pagedFetch(map[string]repliesPage{
		"":   {msgs: []string{"1.0", "2.0"}, next: "c1"},
		"c1": {msgs: []string{"3.0", "4.0"}, next: "c2"},
	}, &calls)

	msgs, next, err := collectThreadReplies(fetch, 3)
	require.NoError(t, err)
	assert.Equal(t, []string{"1.0", "2.0", "3.0", "4.0"}, timestamps(msgs))
	assert.Equal(t, "c2", next)
	assert.Equal(t, []string{"", "c1"}, calls)
}

func TestUnitCollectThreadReplies_LaterPageFailureReturnsPartialWithCursor(t *testing.T) {
	var calls []string
	fetch := pagedFetch(map[string]repliesPage{
		"":   {msgs: []string{"1.0", "2.0"}, next: "c1"},
		"c1": {err: errors.New("rate limited")},
	}, &calls)

	msgs, next, err := collectThreadReplies(fetch, 100)
	require.NoError(t, err)
	assert.Equal(t, []string{"1.0", "2.0"}, timestamps(msgs))
	assert.Equal(t, "c1", next, "cursor must resume at the page that failed")
}

func TestUnitCollectThreadReplies_FirstPageFailureIsAnError(t *testing.T) {
	var calls []string
	boom := errors.New("channel_not_found")
	fetch := pagedFetch(map[string]repliesPage{"": {err: boom}}, &calls)

	msgs, next, err := collectThreadReplies(fetch, 100)
	assert.ErrorIs(t, err, boom)
	assert.Nil(t, msgs)
	assert.Empty(t, next)
}
