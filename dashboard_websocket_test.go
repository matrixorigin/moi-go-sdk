package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
)

// 从客户端公开入口验证看板协议、身份绑定及消息级拒绝。
func TestDashboardWebSocketProductProtocol(t *testing.T) {
	message := "localized refusal from fixture"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/newmoi/data-dashboards/ws" || r.URL.Query().Get("workspace_id") != "ws/1" || r.Header.Get(headerWorkspaceID) != "ws/1" || r.Header.Get(headerMOIRoleID) != "7" {
			t.Errorf("handshake scope: %s %v", r.URL, r.Header)
		}
		upgrader := websocket.Upgrader{Subprotocols: []string{"moi-api-key"}}
		connection, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()
		for {
			var frame struct {
				ID   string         `json:"id"`
				Type string         `json:"message_type"`
				Data map[string]any `json:"data"`
			}
			if err := connection.ReadJSON(&frame); err != nil {
				return
			}
			var kind string
			var data any
			switch frame.Type {
			case "ping":
				kind = "pong"
			case "dashboard_preview_execute":
				if frame.Data["dashboard_id"] != "dashboard/1" || frame.Data["sql_text"] != "SELECT 1" || frame.Data["chart_id"] != "chart/1" {
					t.Errorf("preview payload: %#v", frame.Data)
				}
				kind, data = "dashboard_preview_result", map[string]any{"status": "success", "total": 1}
			case "dashboard_chart_create":
				if frame.Data["name"] != "report" || frame.Data["dashboard_id"] != "dashboard/1" {
					t.Errorf("create payload: %#v", frame.Data)
				}
				kind, data = "dashboard_chart_create_result", map[string]any{"ID": "chart/1", "Name": "report"}
			case "dashboard_chart_update":
				if frame.Data["chart_id"] != "chart/1" {
					t.Errorf("update payload: %#v", frame.Data)
				}
				kind, data = "error", map[string]any{"status": 403, "code": "ErrAccessDenied", "msg": message}
			case "preclose":
				return
			default:
				t.Errorf("unexpected frame: %+v", frame)
				return
			}
			if err := connection.WriteJSON(map[string]any{"id": frame.ID, "message_type": kind, "data": data}); err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("test-key"), WithWorkspaceCurrentRoleID("ws/1", "7"), WithRequestModifier(func(r *http.Request) {
		r.Header.Set(headerWorkspaceID, "other")
		r.Header.Set(headerMOIRoleID, "99")
		query := r.URL.Query()
		query.Set("workspace_id", "other")
		r.URL.RawQuery = query.Encode()
	}))
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.Dashboard("ws/1").ConnectWebSocket(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if err := session.Ping(t.Context(), "heartbeat"); err != nil {
		t.Fatal(err)
	}
	frame, err := session.Receive(t.Context())
	if err != nil || frame.GetId() != "heartbeat" || frame.GetMessageType() != DashboardMessagePong {
		t.Fatalf("pong: %v, %v", frame, err)
	}
	if err := session.PreviewSQL(t.Context(), "preview-1", "dashboard/1", "chart/1", "SELECT 1"); err != nil {
		t.Fatal(err)
	}
	frame, err = session.Receive(t.Context())
	if err != nil || frame.GetId() != "preview-1" || frame.GetData().AsMap()["status"] != "success" {
		t.Fatalf("preview: %v, %v", frame, err)
	}
	request := &DataDashboardChartMutationRequest{Name: "report", ChartType: "table", SqlText: "SELECT 1"}
	if err := session.CreateChart(t.Context(), "create-1", "dashboard/1", request); err != nil {
		t.Fatal(err)
	}
	frame, err = session.Receive(t.Context())
	if err != nil || frame.GetData().AsMap()["ID"] != "chart/1" {
		t.Fatalf("create: %v, %v", frame, err)
	}
	if err := session.UpdateChart(t.Context(), "update-1", "dashboard/1", "chart/1", request); err != nil {
		t.Fatal(err)
	}
	frame, err = session.Receive(t.Context())
	var refusal *Error
	if !errors.As(err, &refusal) || refusal.Status != 403 || refusal.Code != "ErrAccessDenied" || refusal.Message != message || frame.GetId() != "update-1" {
		t.Fatalf("refusal: %v, %v", frame, err)
	}
	if err := session.Preclose(t.Context(), "finish"); err != nil {
		t.Fatal(err)
	}
}

func TestDashboardWebSocketInvalidResponsesAndDisconnect(t *testing.T) {
	for _, test := range []struct {
		name, payload, code string
	}{
		{"invalid JSON", `not-json`, "ErrResponseInvalid"},
		{"invalid error data", `{"id":"1","message_type":"error","data":{"status":"broken","code":"ErrForbidden","msg":"fixture"}}`, "ErrResponseInvalid"},
		{"empty error code", `{"id":"1","message_type":"error","data":{"status":403,"msg":"fixture"}}`, "ErrResponseInvalid"},
		{"unknown type", `{"id":"1","message_type":"unknown"}`, "ErrResponseInvalid"},
		{"disconnected", "", "ErrTransportUnavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				connection, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer connection.Close()
				if test.payload != "" {
					_ = connection.WriteMessage(websocket.TextMessage, []byte(test.payload))
				}
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("fixture"))
			if err != nil {
				t.Fatal(err)
			}
			session, err := client.Dashboard("ws").ConnectWebSocket(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = session.Close() })
			_, err = session.Receive(t.Context())
			if !IsCode(err, test.code) {
				t.Fatalf("response error = %v, want %s", err, test.code)
			}
		})
	}
}

func TestDashboardWebSocketHandshakeErrorAndCancellation(t *testing.T) {
	t.Run("handshake refusal", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(headerRequestID, "handshake-request")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "ErrAccessDenied", "msg": "fixture message"})
		}))
		t.Cleanup(server.Close)
		client, _ := New(server.URL+"/newmoi", WithAPIKey("test-key"))
		_, err := client.Dashboard("ws-1").ConnectWebSocket(t.Context())
		var failure *Error
		if !errors.As(err, &failure) || failure.Status != 403 || failure.Code != "ErrAccessDenied" || failure.RequestID != "handshake-request" {
			t.Fatalf("handshake error: %v", err)
		}
	})
	t.Run("read cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			connection, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer connection.Close()
			_, _, _ = connection.ReadMessage()
		}))
		t.Cleanup(server.Close)
		client, _ := New(server.URL+"/newmoi", WithAPIKey("test-key"))
		session, err := client.Dashboard("ws-1").ConnectWebSocket(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = session.Close() })
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		started := make(chan struct{})
		finished := make(chan error, 1)
		go func() {
			close(started)
			_, err := session.Receive(ctx)
			finished <- err
		}()
		<-started
		cancel()
		if err := <-finished; !errors.Is(err, context.Canceled) {
			t.Fatalf("receive cancellation: %v", err)
		}
	})
}
