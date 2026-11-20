package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestSQLQueryWebSocketSharedProtocol(t *testing.T) {
	const secret = "synthetic-key"
	handshake := make(chan *http.Request, 1)
	messages := make(chan map[string]any, 3)
	serverErr := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handshake <- r.Clone(r.Context())
		websocket.Server{
			Handshake: func(config *websocket.Config, _ *http.Request) error {
				config.Protocol = []string{"moi-api-key"}
				return nil
			},
			Handler: func(connection *websocket.Conn) {
				defer connection.Close()
				received := 0
				for {
					var raw string
					if err := websocket.Message.Receive(connection, &raw); err != nil {
						if received == 3 {
							serverErr <- nil
						} else {
							serverErr <- err
						}
						return
					}
					var message map[string]any
					decoder := json.NewDecoder(bytes.NewBufferString(raw))
					decoder.UseNumber()
					if err := decoder.Decode(&message); err != nil {
						serverErr <- err
						return
					}
					messages <- message
					received++
					id, _ := message["id"].(string)
					switch message["message_type"] {
					case "ping":
						err := websocket.Message.Send(connection, fmt.Sprintf(`{"id":%q,"message_type":"pong"}`, id))
						if err != nil {
							serverErr <- err
							return
						}
					case "query_execute":
						first := fmt.Sprintf(`{"id":%q,"message_type":"query_id","data":{"query_id":"query/1"}}`, id)
						second := fmt.Sprintf(`{"id":%q,"message_type":"query_result","data":{"query_id":"query/1","status":"success","total":1,"nullable":null}}`, id)
						if err := websocket.Message.Send(connection, first); err != nil {
							serverErr <- err
							return
						}
						if err := websocket.Message.Send(connection, second); err != nil {
							serverErr <- err
							return
						}
					case "query_kill":
						if err := websocket.Message.Send(connection, fmt.Sprintf(`{"id":%q,"message_type":"query_kill"}`, id)); err != nil {
							serverErr <- err
							return
						}
					}
				}
			},
		}.ServeHTTP(w, r)
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey(secret))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	session, err := client.SQL("workspace-1").ConnectQueryWebSocket(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if got := session.Protocols(); !reflect.DeepEqual(got, []string{"moi-api-key"}) {
		t.Fatalf("protocols=%v", got)
	}
	idle, cancelIdle := context.WithTimeout(ctx, 20*time.Millisecond)
	_, err = session.Receive(idle)
	cancelIdle()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("idle receive=%v", err)
	}
	if err := session.Ping(ctx, "ping-1"); err != nil {
		t.Fatal(err)
	}
	pong, err := session.Receive(ctx)
	if err != nil || pong.GetId() != "ping-1" || pong.GetMessageType() != SQLQueryMessagePong {
		t.Fatalf("pong=%v error=%v", pong, err)
	}
	if err := session.ExecuteQuery(ctx, "execute-1", "SELECT 1", WithSQLQueryDBName("db/1"), WithSQLQueryType("sql"), WithSQLQueryOffset(0), WithSQLQueryLimit(0)); err != nil {
		t.Fatal(err)
	}
	started, err := session.Receive(ctx)
	if err != nil || started.GetId() != "execute-1" || started.GetMessageType() != SQLQueryMessageQueryID || started.GetData().AsMap()["query_id"] != "query/1" {
		t.Fatalf("started=%v error=%v", started, err)
	}
	finished, err := session.Receive(ctx)
	if err != nil || finished.GetMessageType() != SQLQueryMessageResult || finished.GetData().AsMap()["status"] != "success" || finished.GetData().AsMap()["nullable"] != nil {
		t.Fatalf("finished=%v error=%v", finished, err)
	}
	if err := session.KillQuery(ctx, "kill-1", "query/1"); err != nil {
		t.Fatal(err)
	}
	killed, err := session.Receive(ctx)
	if err != nil || killed.GetId() != "kill-1" || killed.GetMessageType() != SQLQueryMessageKill {
		t.Fatalf("killed=%v error=%v", killed, err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case request := <-handshake:
		if request.URL.Path != "/newmoi/query/ws" || !reflect.DeepEqual(request.URL.Query(), url.Values{"workspace_id": {"workspace-1"}}) || request.Header.Get("X-API-Key") != secret || request.Header.Get("X-Workspace-ID") != "workspace-1" {
			t.Fatalf("handshake=%s headers=%v", request.URL, request.Header)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no handshake")
	}
	want := []map[string]any{
		{"id": "ping-1", "message_type": "ping"},
		{"id": "execute-1", "message_type": "query_execute", "data": map[string]any{"query": "SELECT 1", "db_name": "db/1", "sql_type": "sql", "offset": json.Number("0"), "limit": json.Number("0")}},
		{"id": "kill-1", "message_type": "query_kill", "data": map[string]any{"query_id": "query/1"}},
	}
	for index, expected := range want {
		select {
		case got := <-messages:
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("message[%d]=%#v want=%#v", index, got, expected)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("missing message %d", index)
		}
	}
	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not observe close")
	}
}
