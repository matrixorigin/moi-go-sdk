package sdk

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestRuntimeMCPCancellationAcknowledgement(t *testing.T) {
	for _, scope := range []string{"runtime", "workspace"} {
		for _, tc := range []struct {
			name       string
			status     int
			body, code string
		}{
			{"accepted", 202, "", ""},
			{"unexpected-empty-200", 200, "", "ErrResponseInvalid"},
			{"unexpected-response", 202, `{}`, "ErrResponseInvalid"},
			{"denied", 401, `{"error":{"code":-32001,"message":"Denied"}}`, "-32001"},
		} {
			t.Run(scope+"/"+tc.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					wantPath := "/api/v1/mcp/http"
					if scope == "workspace" {
						wantPath = "/newmoi/workspaces/workspace/agents/agent/mcp/http"
					}
					if r.URL.Path != wantPath {
						t.Errorf("notification path = %s", r.URL.Path)
					}
					var body map[string]json.RawMessage
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if _, exists := body["id"]; exists {
						t.Error("notification contains a JSON-RPC id")
					}
					w.Header().Set(headerRequestID, "notification-case")
					w.WriteHeader(tc.status)
					io.WriteString(w, tc.body)
				}))
				t.Cleanup(server.Close)
				client, err := New(server.URL+"/newmoi", WithToken("test-task-grant"), WithRuntimeAuthorization("test-task-grant"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { client.Close() })
				params, err := structpb.NewStruct(map[string]any{"requestId": "exchange", "reason": "test"})
				if err != nil {
					t.Fatal(err)
				}
				request := &RuntimeRPCRequest{Jsonrpc: "2.0", Method: "notifications/cancelled", Params: params}
				var result *RuntimeRPCResponse
				if scope == "workspace" {
					result, err = client.Agents("workspace").WorkspaceMCP(t.Context(), "agent", request)
				} else {
					result, err = client.RuntimeGateway().MCP(t.Context(), request)
				}
				if result != nil {
					t.Fatal("notification fabricated a JSON-RPC response")
				}
				if tc.code == "" {
					if err != nil {
						t.Fatal(err)
					}
				} else {
					var failure *Error
					if !errors.As(err, &failure) || failure.Code != tc.code || failure.RequestID != "notification-case" {
						t.Fatalf("notification error = %v, want %s with request id", err, tc.code)
					}
				}
			})
		}
	}
}
