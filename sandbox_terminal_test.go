package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestSandboxTerminalProtocol(t *testing.T) {
	type frame struct {
		Text bool
		Data string
	}
	frames := make(chan frame, 2)
	serverErr := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/newmoi/agent-sandboxes/sandbox-1/terminal/ws" || r.Header.Get("Origin") != "http://"+r.Host {
			serverErr <- &Error{Code: "unexpected-handshake", Message: r.URL.String()}
			return
		}
		wantQuery := url.Values{"token": {"signed token"}, "cols": {"100"}, "rows": {"30"}}
		if !reflect.DeepEqual(r.URL.Query(), wantQuery) {
			serverErr <- &Error{Code: "unexpected-query", Message: r.URL.RawQuery}
			return
		}
		websocket.Handler(func(connection *websocket.Conn) {
			defer connection.Close()
			var input []byte
			if err := websocket.Message.Receive(connection, &input); err != nil {
				serverErr <- err
				return
			}
			frames <- frame{Data: string(input)}
			var resize string
			if err := websocket.Message.Receive(connection, &resize); err != nil {
				serverErr <- err
				return
			}
			frames <- frame{Text: true, Data: resize}
			if err := websocket.Message.Send(connection, []byte{0, 1, 0xff}); err != nil {
				serverErr <- err
				return
			}
			if err := websocket.Message.Send(connection, `{"type":"exit","code":0}`); err != nil {
				serverErr <- err
				return
			}
			var ignored []byte
			if err := websocket.Message.Receive(connection, &ignored); err == nil {
				serverErr <- &Error{Code: "close-missing"}
				return
			}
			serverErr <- nil
		}).ServeHTTP(w, r)
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := client.Sandboxes("workspace-1").ConnectTerminal(t.Context(), "sandbox-1", "signed token", 100, 30)
	if err != nil {
		t.Fatal(err)
	}
	if err := terminal.WriteInput(t.Context(), []byte{0, 2, 0xfe}); err != nil {
		t.Fatal(err)
	}
	if err := terminal.Resize(t.Context(), 120, 40); err != nil {
		t.Fatal(err)
	}
	output, err := terminal.Receive(t.Context())
	if err != nil || !reflect.DeepEqual(output, []byte{0, 1, 0xff}) {
		t.Fatalf("binary output=%v error=%v", output, err)
	}
	control, err := terminal.Receive(t.Context())
	if err != nil || string(control) != `{"type":"exit","code":0}` {
		t.Fatalf("control output=%s error=%v", control, err)
	}
	if err := terminal.Close(); err != nil {
		t.Fatal(err)
	}
	first := <-frames
	if first.Text || first.Data != string([]byte{0, 2, 0xfe}) {
		t.Fatalf("input frame=%#v", first)
	}
	second := <-frames
	var resize map[string]any
	if !second.Text || json.Unmarshal([]byte(second.Data), &resize) != nil || !reflect.DeepEqual(resize, map[string]any{"type": "resize", "cols": float64(120), "rows": float64(40)}) {
		t.Fatalf("resize frame=%#v decoded=%v", second, resize)
	}
	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not observe terminal close")
	}
}

func TestSandboxTerminalRejectsInvalidCapability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "invalid terminal capability", http.StatusUnauthorized)
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	terminal, err := client.Sandboxes("workspace-1").ConnectTerminal(ctx, "sandbox-1", "invalid", 80, 24)
	if err == nil {
		terminal.Close()
		t.Fatal("invalid terminal capability connected")
	}
}
