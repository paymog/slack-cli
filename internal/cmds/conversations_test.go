package cmds

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/paymog/slack-cli/internal/config"
)

func TestUnitAddFileAsNativeSlackText(t *testing.T) {
	body := "*Findings*\n\n• Keep `code` and $HOME literal.\n• Do not decode \\n inside examples.\n"
	path := filepath.Join(t.TempDir(), "reply.txt")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	posts := make(chan url.Values, 1)
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/auth.test":
			io.WriteString(w, `{"ok":true,"url":"https://example.slack.com/","team_id":"T123","user_id":"U123"}`)
		case "/api/chat.postMessage":
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			posts <- r.PostForm
			io.WriteString(w, `{"ok":true,"channel":"C123","ts":"1700.2"}`)
		default:
			t.Errorf("unexpected API call: %s", r.URL.Path)
			http.Error(w, "unexpected API call", http.StatusNotFound)
		}
	}))
	defer api.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			t.Errorf("unexpected proxy method: %s", r.Method)
			http.Error(w, "CONNECT required", http.StatusBadRequest)
			return
		}
		upstream, err := net.Dial("tcp", api.Listener.Addr().String())
		if err != nil {
			t.Error(err)
			http.Error(w, "dial failed", http.StatusBadGateway)
			return
		}
		defer upstream.Close()
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		go io.Copy(upstream, conn)
		io.Copy(conn, upstream)
	}))
	defer proxy.Close()
	t.Setenv("SLACK_MCP_PROXY", proxy.URL)
	t.Setenv("SLACK_MCP_CUSTOM_TLS", "")
	t.Setenv("SLACK_MCP_SERVER_CA", "")
	t.Setenv("SLACK_MCP_SERVER_CA_INSECURE", "1")
	t.Setenv("SLACK_MCP_ADD_MESSAGE_TOOL", "C123")
	t.Setenv("SLACK_MCP_ADD_MESSAGE_MARK", "")
	t.Setenv("SLACK_MCP_GOVSLACK", "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("SLACK_MCP_XOXP_TOKEN", "")
	t.Setenv("SLACK_MCP_XOXB_TOKEN", "xoxb-test")
	t.Setenv("SLACK_MCP_XOXC_TOKEN", "")
	t.Setenv("SLACK_MCP_XOXD_TOKEN", "")
	cmd := conversationsAddCommand(&config.Config{XOXB: "xoxb-test", NoCache: true})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"C123", "--thread-ts", "1700.1", "--text-file", path, "--content-type", "text/mrkdwn"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var posted url.Values
	select {
	case posted = <-posts:
	default:
		t.Fatal("no Slack message was posted")
	}
	if posted.Get("text") != body || posted.Get("blocks") != "" || posted.Get("mrkdwn") == "false" {
		t.Fatalf("expected native Slack text without blocks, got %v", posted)
	}
	if posted.Get("thread_ts") != "1700.1" {
		t.Fatalf("wrong thread: %v", posted)
	}
	var result map[string]string
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["ts"] != "1700.2" {
		t.Fatalf("posted timestamp missing: %v", result)
	}
}

func TestUnitAddMissingTextFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	cmd := conversationsAddCommand(&config.Config{})
	cmd.SetArgs([]string{"C123", "--text-file", path})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file-not-found error, got %v", err)
	}
}
