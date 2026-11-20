package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// a2aFollowRequest records one decoded A2A JSON-RPC call the mock observed.
type a2aFollowRequest struct {
	method string
	params map[string]any
	stream bool
	accept string
}

// a2aFollowMock serves the product A2A endpoint for fold tests. The script
// function decides each response from the request sequence it observed.
type a2aFollowMock struct {
	t        *testing.T
	mu       sync.Mutex
	requests []a2aFollowRequest
	serve    func(call a2aFollowRequest, w http.ResponseWriter) bool
}

func (m *a2aFollowMock) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			JSONRPC string         `json:"jsonrpc"`
			Method  string         `json:"method"`
			Params  map[string]any `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			m.t.Errorf("decode a2a request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		call := a2aFollowRequest{
			method: body.Method,
			params: body.Params,
			accept: r.Header.Get("Accept"),
		}
		call.stream = strings.HasPrefix(call.accept, acceptEventStream)
		m.mu.Lock()
		m.requests = append(m.requests, call)
		m.mu.Unlock()
		if !m.serve(call, w) {
			m.t.Errorf("unexpected a2a call: %+v", call)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}

func (m *a2aFollowMock) calls() []a2aFollowRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]a2aFollowRequest(nil), m.requests...)
}

func writeA2AProductResult(w http.ResponseWriter, result map[string]any) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": "OK",
		"msg":  "OK",
		"data": map[string]any{"jsonrpc": "2.0", "id": "1", "result": result},
	})
}

func a2aTaskPayload(state string, cursor int64) map[string]any {
	return map[string]any{
		"kind":      "task",
		"id":        "task-1",
		"contextId": "ctx-1",
		"status":    map[string]any{"state": state, "timestamp": "2026-11-05T08:00:00Z"},
		"metadata":  map[string]any{"eventCursor": cursor},
	}
}

func writeA2ASSEFrame(w http.ResponseWriter, seq int64, result map[string]any) {
	payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "1", "result": result})
	_, _ = fmt.Fprintf(w, "id: %d\nevent: message\ndata: %s\n\n", seq, payload)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func a2aStatusUpdate(state string, seq int64, final bool) map[string]any {
	return map[string]any{
		"kind":      "status-update",
		"taskId":    "task-1",
		"contextId": "ctx-1",
		"status":    map[string]any{"state": state, "timestamp": "2026-11-05T08:00:01Z"},
		"final":     final,
		"metadata":  map[string]any{"eventCursor": seq},
	}
}

func newA2AFollowClient(t *testing.T, mock *a2aFollowMock, opts ...Option) *Client {
	t.Helper()
	server := httptest.NewServer(mock.handler())
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", append(opts, WithAPIKey("user-key"))...)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func sendA2AText(t *testing.T, client *Client, noWait bool) (*AgentA2AResult, error) {
	t.Helper()
	params, err := json.Marshal(map[string]any{"message": map[string]any{"parts": []map[string]any{{"kind": "text", "text": "hi"}}}})
	if err != nil {
		t.Fatal(err)
	}
	return client.Agents("ws-1").A2A(context.Background(), AgentA2ARequest{
		AgentSelector: AgentSelector{AgentID: "agent-1"},
		A2ARequest: A2ARequest{
			JSONRPC: "2.0",
			ID:      "req-1",
			Method:  A2AMethodSend,
			Params:  params,
			NoWait:  noWait,
		},
	})
}

func a2aResultState(t *testing.T, result *AgentA2AResult) string {
	t.Helper()
	payload, err := AgentA2APayload(result)
	if err != nil {
		t.Fatalf("payload err = %v", err)
	}
	status, _ := payload["status"].(map[string]any)
	state, _ := status["state"].(string)
	return state
}

func TestAgentA2APayloadPreservesJSONNumbers(t *testing.T) {
	t.Parallel()
	payload, err := AgentA2APayload(&AgentA2AResult{
		Json: `{"id":"task-1","status":{"state":"completed"},"cursor":9007199254740993}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := payload["cursor"].(json.Number); !ok || got.String() != "9007199254740993" {
		t.Fatalf("cursor = %#v", payload["cursor"])
	}
}

func TestA2ASendFoldsNonTerminalTaskThroughResubscribe(t *testing.T) {
	t.Parallel()
	mock := &a2aFollowMock{t: t}
	mock.serve = func(call a2aFollowRequest, w http.ResponseWriter) bool {
		switch call.method {
		case A2AMethodSend:
			writeA2AProductResult(w, a2aTaskPayload("working", 3))
		case A2AMethodTasksResubscribe:
			if call.params["afterSeq"] != float64(3) {
				t.Errorf("resubscribe afterSeq = %v, want 3", call.params["afterSeq"])
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, ": heartbeat\n\n")
			writeA2ASSEFrame(w, 4, a2aStatusUpdate("working", 4, false))
			writeA2ASSEFrame(w, 5, a2aStatusUpdate("completed", 5, true))
		case A2AMethodTasksGet:
			writeA2AProductResult(w, a2aTaskPayload("completed", 5))
		default:
			return false
		}
		return true
	}
	client := newA2AFollowClient(t, mock)

	result, err := sendA2AText(t, client, false)
	if err != nil {
		t.Fatalf("send err = %v", err)
	}
	if state := a2aResultState(t, result); state != "completed" {
		t.Fatalf("state = %q, want completed", state)
	}
	if taskID, err := AgentA2ATaskID(result); err != nil || taskID != "task-1" {
		t.Fatalf("task id = %q, err = %v", taskID, err)
	}
	var methods []string
	for _, call := range mock.calls() {
		methods = append(methods, call.method)
	}
	want := []string{A2AMethodSend, A2AMethodTasksResubscribe, A2AMethodTasksGet}
	if strings.Join(methods, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %v, want %v", methods, want)
	}
}

func TestA2ASendFollowReconnectsWithEventCursor(t *testing.T) {
	t.Parallel()
	mock := &a2aFollowMock{t: t}
	var resubscribeCount int
	mock.serve = func(call a2aFollowRequest, w http.ResponseWriter) bool {
		switch call.method {
		case A2AMethodSend:
			writeA2AProductResult(w, a2aTaskPayload("submitted", 3))
		case A2AMethodTasksResubscribe:
			resubscribeCount++
			w.Header().Set("Content-Type", "text/event-stream")
			switch resubscribeCount {
			case 1:
				if call.params["afterSeq"] != float64(3) {
					t.Errorf("first resubscribe afterSeq = %v, want 3", call.params["afterSeq"])
				}
				writeA2ASSEFrame(w, 4, a2aStatusUpdate("working", 4, false))
				// Mid-stream drop: close without a terminal frame.
			case 2:
				if call.params["afterSeq"] != float64(4) {
					t.Errorf("reconnect afterSeq = %v, want resumed cursor 4", call.params["afterSeq"])
				}
				writeA2ASSEFrame(w, 5, a2aStatusUpdate("completed", 5, true))
			default:
				t.Errorf("unexpected resubscribe attempt %d", resubscribeCount)
			}
		case A2AMethodTasksGet:
			writeA2AProductResult(w, a2aTaskPayload("completed", 5))
		default:
			return false
		}
		return true
	}
	client := newA2AFollowClient(t, mock)

	result, err := sendA2AText(t, client, false)
	if err != nil {
		t.Fatalf("send err = %v", err)
	}
	if state := a2aResultState(t, result); state != "completed" {
		t.Fatalf("state = %q, want completed after reconnect", state)
	}
	if resubscribeCount != 2 {
		t.Fatalf("resubscribe attempts = %d, want 2", resubscribeCount)
	}
}

func TestA2ASendFollowDegradesToPollingWhenStreamUnavailable(t *testing.T) {
	t.Parallel()
	mock := &a2aFollowMock{t: t}
	var getCount int
	mock.serve = func(call a2aFollowRequest, w http.ResponseWriter) bool {
		switch call.method {
		case A2AMethodSend:
			writeA2AProductResult(w, a2aTaskPayload("working", 3))
		case A2AMethodTasksResubscribe:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": "ErrNotFound", "msg": "resubscribe unsupported",
			})
		case A2AMethodTasksGet:
			getCount++
			writeA2AProductResult(w, a2aTaskPayload("completed", 6))
		default:
			return false
		}
		return true
	}
	client := newA2AFollowClient(t, mock)

	result, err := sendA2AText(t, client, false)
	if err != nil {
		t.Fatalf("send err = %v", err)
	}
	if state := a2aResultState(t, result); state != "completed" {
		t.Fatalf("state = %q, want completed through polling", state)
	}
	if getCount == 0 {
		t.Fatal("polling fallback never called tasks/get")
	}
}

func TestA2ASendFollowTimeoutCarriesTaskID(t *testing.T) {
	t.Parallel()
	mock := &a2aFollowMock{t: t}
	mock.serve = func(call a2aFollowRequest, w http.ResponseWriter) bool {
		switch call.method {
		case A2AMethodSend:
			writeA2AProductResult(w, a2aTaskPayload("working", 3))
		case A2AMethodTasksResubscribe:
			w.WriteHeader(http.StatusNotFound)
		case A2AMethodTasksGet:
			writeA2AProductResult(w, a2aTaskPayload("working", 4))
		default:
			return false
		}
		return true
	}
	client := newA2AFollowClient(t, mock, WithA2AWaitTimeout(300*time.Millisecond))

	_, err := sendA2AText(t, client, false)
	if err == nil {
		t.Fatal("send succeeded despite wait budget exhaustion")
	}
	if !IsCode(err, "ErrTimeout") {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	var sdkErr *Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("err type = %T, want *Error", err)
	}
	if sdkErr.Details["task_id"] != "task-1" {
		t.Fatalf("timeout details = %v, want task_id task-1", sdkErr.Details)
	}
}

func TestA2ASendFollowSurfacesStreamErrorFrameWithTaskID(t *testing.T) {
	t.Parallel()
	mock := &a2aFollowMock{t: t}
	mock.serve = func(call a2aFollowRequest, w http.ResponseWriter) bool {
		switch call.method {
		case A2AMethodSend:
			writeA2AProductResult(w, a2aTaskPayload("working", 3))
		case A2AMethodTasksResubscribe:
			w.Header().Set("Content-Type", "text/event-stream")
			payload, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"error": map[string]any{"code": -32001, "message": "task projection lost"},
			})
			_, _ = fmt.Fprintf(w, "id: 4\nevent: message\ndata: %s\n\n", payload)
		default:
			return false
		}
		return true
	}
	client := newA2AFollowClient(t, mock)

	_, err := sendA2AText(t, client, false)
	if err == nil {
		t.Fatal("send succeeded despite stream error frame")
	}
	if !IsCode(err, "-32001") {
		t.Fatalf("err = %v, want JSON-RPC code -32001", err)
	}
	var sdkErr *Error
	if !errors.As(err, &sdkErr) || sdkErr.Details["task_id"] != "task-1" {
		t.Fatalf("stream error = %#v, want task_id detail", err)
	}
}

func TestA2ASendPassesThroughTerminalAndMessageResults(t *testing.T) {
	t.Parallel()
	mock := &a2aFollowMock{t: t}
	mock.serve = func(call a2aFollowRequest, w http.ResponseWriter) bool {
		if call.method != A2AMethodSend {
			return false
		}
		text, _ := call.params["message"].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"].(string)
		switch text {
		case "terminal":
			writeA2AProductResult(w, a2aTaskPayload("completed", 9))
		case "ack":
			writeA2AProductResult(w, map[string]any{
				"kind":      "message",
				"messageId": "msg-1",
				"role":      "agent",
				"metadata":  map[string]any{"matrixflow_type": "moi.feedback.received"},
			})
		case "nowait":
			writeA2AProductResult(w, a2aTaskPayload("working", 3))
		default:
			return false
		}
		return true
	}
	client := newA2AFollowClient(t, mock)

	send := func(text string, noWait bool) (*AgentA2AResult, error) {
		params, _ := json.Marshal(map[string]any{"message": map[string]any{"parts": []map[string]any{{"kind": "text", "text": text}}}})
		return client.Agents("ws-1").A2A(context.Background(), AgentA2ARequest{
			AgentSelector: AgentSelector{AgentID: "agent-1"},
			A2ARequest:    A2ARequest{JSONRPC: "2.0", ID: text, Method: A2AMethodSend, Params: params, NoWait: noWait},
		})
	}

	terminal, err := send("terminal", false)
	if err != nil || a2aResultState(t, terminal) != "completed" {
		t.Fatalf("terminal passthrough state = %q, err = %v", a2aResultState(t, terminal), err)
	}
	ack, err := send("ack", false)
	if err != nil {
		t.Fatalf("ack passthrough err = %v", err)
	}
	ackPayload, err := AgentA2APayload(ack)
	if err != nil || ackPayload["kind"] != "message" {
		t.Fatalf("ack payload = %v, err = %v", ackPayload, err)
	}
	admission, err := send("nowait", true)
	if err != nil || a2aResultState(t, admission) != "working" {
		t.Fatalf("no-wait admission state = %q, err = %v", a2aResultState(t, admission), err)
	}
	if calls := mock.calls(); len(calls) != 3 {
		t.Fatalf("calls = %v, want exactly 3 sends without follow", calls)
	}
}
