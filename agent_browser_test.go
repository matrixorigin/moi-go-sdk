package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	gorillawebsocket "github.com/gorilla/websocket"
	"golang.org/x/net/websocket"
	"maps"
	"slices"
)

func TestAgentBrowserTokenContract(t *testing.T) {
	const token = "moi-browser-token-v1.signed"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/newmoi/workspaces/workspace%20%2F1/agent-runtime-tasks/task%2F1%20%3F/browser/token" {
			t.Errorf("request=%s %s", r.Method, r.URL.EscapedPath())
		}
		if r.Header.Get("X-API-Key") != "synthetic-key" || r.Header.Get("X-Workspace-ID") != "workspace /1" {
			t.Errorf("headers=%v", r.Header)
		}
		for _, name := range []string{"Authorization", "Cookie", "X-MOI-Runtime-Authorization"} {
			if value := r.Header.Get(name); value != "" {
				t.Errorf("unexpected %s: %q", name, value)
			}
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != "{}" {
			t.Errorf("body=%q error=%v", body, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": map[string]any{"token": token, "expires_at": "2026-11-10T12:00:00Z"}})
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Agents("workspace /1").RuntimeBrowserToken(t.Context(), "task/1 ?")
	if err != nil {
		t.Fatal(err)
	}
	if result.GetToken() != token || result.GetExpiresAt() != "2026-11-10T12:00:00Z" {
		t.Fatalf("token=%v", result)
	}
}

func TestAgentBrowserWebSocketProtocol(t *testing.T) {
	const (
		token  = "moi-browser-token-v1.signed"
		origin = "https://console.example.com"
	)
	handshake := make(chan *http.Request, 1)
	input := make(chan string, 1)
	serverErr := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handshake <- r.Clone(r.Context())
		websocket.Server{
			Handshake: func(config *websocket.Config, _ *http.Request) error {
				config.Protocol = []string{runtimeBrowserProtocol}
				return nil
			},
			Handler: func(connection *websocket.Conn) {
				defer connection.Close()
				var event string
				if err := websocket.Message.Receive(connection, &event); err != nil {
					serverErr <- err
					return
				}
				input <- event
				if err := websocket.Message.Send(connection, `{"type":"frame","seq":9007199254740993,"metadata":{"deviceWidth":0}}`); err != nil {
					serverErr <- err
					return
				}
				if err := websocket.Message.Send(connection, []byte{0, 1, 0xff, 0xfe}); err != nil {
					serverErr <- err
					return
				}
				var ignored []byte
				if err := websocket.Message.Receive(connection, &ignored); err == nil {
					serverErr <- fmt.Errorf("browser close was not observed")
					return
				}
				serverErr <- nil
			},
		}.ServeHTTP(w, r)
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
	if err != nil {
		t.Fatal(err)
	}
	browser, err := client.Agents("workspace /1").ConnectRuntimeBrowser(t.Context(), "task/1 ?", token, origin)
	if err != nil {
		t.Fatal(err)
	}
	event := map[string]any{"type": "input_mouse", "eventType": "mousePressed", "x": 0, "button": "", "modifiers": 0, "optional": nil}
	if err := browser.WriteInput(t.Context(), event); err != nil {
		t.Fatal(err)
	}
	metadata, err := browser.Receive(t.Context())
	if err != nil || !metadata.Text || string(metadata.Data) != `{"type":"frame","seq":9007199254740993,"metadata":{"deviceWidth":0}}` {
		t.Fatalf("metadata=%#v error=%v", metadata, err)
	}
	image, err := browser.Receive(t.Context())
	if err != nil || image.Text || !bytes.Equal(image.Data, []byte{0, 1, 0xff, 0xfe}) {
		t.Fatalf("image=%#v error=%v", image, err)
	}
	if err := browser.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case request := <-handshake:
		if request.URL.EscapedPath() != "/newmoi/workspaces/workspace%20%2F1/agent-runtime-tasks/task%2F1%20%3F/browser/stream" || request.URL.RawQuery != "" {
			t.Fatalf("handshake URL=%s", request.URL.String())
		}
		protocols := agentBrowserRequestedProtocols(request)
		if request.Header.Get("Origin") != origin || !reflect.DeepEqual(protocols, []string{runtimeBrowserProtocol, token}) {
			t.Fatalf("origin=%q protocols=%v", request.Header.Get("Origin"), protocols)
		}
		for _, name := range []string{"X-API-Key", "X-Workspace-ID", "Authorization", "Cookie", "X-MOI-Runtime-Authorization"} {
			if value := request.Header.Get(name); value != "" {
				t.Fatalf("unexpected %s: %q", name, value)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no browser WebSocket handshake")
	}
	select {
	case raw := <-input:
		decoder := json.NewDecoder(bytes.NewBufferString(raw))
		decoder.UseNumber()
		var decoded map[string]any
		if err := decoder.Decode(&decoded); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"type": "input_mouse", "eventType": "mousePressed", "x": json.Number("0"), "button": "", "modifiers": json.Number("0"), "optional": nil}
		if !reflect.DeepEqual(decoded, want) {
			t.Fatalf("input=%#v want=%#v", decoded, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no browser input")
	}
	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not observe browser close")
	}
}

func agentBrowserRequestedProtocols(request *http.Request) []string {
	parts := strings.Split(request.Header.Get("Sec-WebSocket-Protocol"), ",")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

func TestAgentBrowserWebSocketValidation(t *testing.T) {
	client, err := New("http://127.0.0.1:1/newmoi", WithAPIKey("synthetic-key"))
	if err != nil {
		t.Fatal(err)
	}
	service := client.Agents("workspace-1")
	if browser, err := service.ConnectRuntimeBrowser(t.Context(), "task-1", "", "https://console.example.com"); err != ErrTokenRequired {
		if browser != nil {
			_ = browser.Close()
		}
		t.Fatalf("empty token error=%v", err)
	}
	if browser, err := service.ConnectRuntimeBrowser(t.Context(), "task-1", "moi-browser-token-v1.signed", ""); err != ErrOriginRequired {
		if browser != nil {
			_ = browser.Close()
		}
		t.Fatalf("empty origin error=%v", err)
	}
}
func TestAgentBrowserTransport(t *testing.T) {
	for _, scenario := range []struct{ compression, view bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		t.Run(fmt.Sprint(scenario), func(t *testing.T) {
			compression := scenario.compression
			const path = "/newmoi/workspaces/ws/agent-runtime-tasks/task/browser"
			const proof = "moi-browser-token-v1.test"
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case path + "/token":
					if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer product-identity" {
						http.Error(w, "product identity missing", http.StatusUnauthorized)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": map[string]string{"token": proof}})
				case path + "/stream":
					if (r.URL.Query().Get("mode") == "view") != scenario.view {
						http.Error(w, "unexpected browser mode", http.StatusBadRequest)
						return
					}
					if protocols := gorillawebsocket.Subprotocols(r); !slices.Equal(protocols, []string{"moi-browser.v1", proof}) {
						http.Error(w, "task proof missing", http.StatusUnauthorized)
						return
					}
					upgrader := gorillawebsocket.Upgrader{EnableCompression: true, Subprotocols: []string{"moi-browser.v1"}}
					connection, err := upgrader.Upgrade(w, r, nil)
					if err != nil {
						return
					}
					defer connection.Close()
					if connection.WriteJSON(map[string]any{"type": "frame", "seq": 7}) != nil {
						return
					}
					if connection.WriteMessage(gorillawebsocket.BinaryMessage, []byte("image\x00\xff")) != nil {
						return
					}
					var ack map[string]any
					if connection.ReadJSON(&ack) != nil || !maps.Equal(ack, map[string]any{"type": "ack", "seq": float64(7)}) {
						return
					}
					if connection.WriteJSON(map[string]any{"type": "frame", "seq": 8}) != nil || connection.WriteMessage(gorillawebsocket.BinaryMessage, []byte("next-image")) != nil {
						return
					}
					var input map[string]string
					if connection.ReadJSON(&input) != nil {
						return
					}
					if !scenario.view {
						if connection.WriteJSON(map[string]any{"type": "input_echo", "event": input}) != nil {
							return
						}
						if connection.ReadJSON(&input) != nil {
							return
						}
					}
					if input["type"] == "release" {
						_ = connection.WriteJSON(map[string]string{"type": "released"})
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", WithToken("product-identity"), WithHTTPClient(server.Client()))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			open := client.Agents("ws").ConnectBrowser
			if scenario.view {
				open = client.Agents("ws").WatchBrowser
			}
			browser, err := open(ctx, "task", compression)
			if err != nil {
				t.Fatal(err)
			}
			defer browser.Close()
			if browser.CompressionEnabled() != compression {
				t.Fatalf("compression=%v", browser.CompressionEnabled())
			}
			event, image, err := browser.Receive(ctx)
			if err != nil {
				t.Fatal(err)
			}
			metadata, ok := event.AsMap()
			if !ok || metadata["type"] != "frame" || metadata["seq"] != json.Number("7") || string(image) != "image\x00\xff" {
				t.Fatalf("frame=%v image=%q", event, image)
			}
			if err := browser.Acknowledge(ctx, 7); err != nil {
				t.Fatal(err)
			}
			event, image, err = browser.Receive(ctx)
			if err != nil {
				t.Fatal(err)
			}
			metadata, _ = event.AsMap()
			if metadata["seq"] != json.Number("8") || string(image) != "next-image" {
				t.Fatalf("next frame=%v image=%q", event, image)
			}
			if !scenario.view {
				if err := browser.InputKeyboard(ctx, "char", "验收 input"); err != nil {
					t.Fatal(err)
				}
				event, _, err = browser.Receive(ctx)
				if err != nil {
					t.Fatal(err)
				}
				echo, _ := event.AsMap()
				input, _ := echo["event"].(map[string]any)
				if input["type"] != "input_keyboard" || input["eventType"] != "char" || input["text"] != "验收 input" {
					t.Fatalf("echo=%v", event)
				}
			}
			if err := browser.Release(ctx); err != nil {
				t.Fatal(err)
			}
			event, _, err = browser.Receive(ctx)
			if err != nil {
				t.Fatal(err)
			}
			released, _ := event.AsMap()
			if released["type"] != "released" {
				t.Fatalf("release=%v", event)
			}
		})
	}
}
