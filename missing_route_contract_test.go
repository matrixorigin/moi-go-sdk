package sdk

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAddedRoutesPreserveWireContracts(t *testing.T) {
	for _, auth := range []Option{WithAPIKey("key"), WithToken("token")} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.Method + " " + r.URL.Path {
			case "GET /newmoi/workflow/v2/workflow-apps/workflow/versions/version":
				io.WriteString(w, `{"code":0,"data":{"ID":"version","WorkflowDefinitionID":"workflow","Version":3,"WorkflowJSON":"{}","Status":"draft","ResultMode":1}}`)
			case "PUT /newmoi/workspaces/workspace/llm/backends/1/endpoints/2/status":
				w.WriteHeader(http.StatusNoContent)
			case "GET /api/v1/service-accounts/workspaces":
				io.WriteString(w, `{"items":[{"workspace_id":"workspace","display_name":"Workspace","remote_version":"v1"}]}`)
			case "POST /api/v1/mcp/http":
				io.WriteString(w, `{"jsonrpc":"2.0","id":"rpc","result":{"tools":[]}}`)
			default:
				t.Errorf("unexpected SDK request %s %s", r.Method, r.URL)
				w.WriteHeader(404)
			}
		}))
		client, err := New(server.URL+"/newmoi", auth)
		if err != nil {
			t.Fatal(err)
		}
		got, err := client.Workflows("workspace").Version(t.Context(), "workflow", "version")
		if err != nil || got.GetId() != "version" || got.GetVersion() != 3 || got.GetResultMode() != 1 {
			t.Fatalf("workflow version=%v err=%v", got, err)
		}
		if err := client.Models("workspace").SetWorkspaceLLMEndpointStatus(t.Context(), 1, 2, 1); err != nil {
			t.Fatal(err)
		}
		list, err := client.System().ServiceAccountWorkspaces(t.Context())
		if err != nil || len(list.GetItems()) != 1 || list.GetItems()[0].GetRemoteVersion() != "v1" {
			t.Fatalf("provisioner=%v err=%v", list, err)
		}
		rpc, err := client.RuntimeGateway().MCP(t.Context(), &RuntimeRPCRequest{Jsonrpc: "2.0", Method: "tools/list"})
		if err != nil || rpc.GetId().GetStringValue() != "rpc" || rpc.GetResult().GetFields()["tools"].GetListValue() == nil {
			t.Fatalf("RPC=%v err=%v", rpc, err)
		}
		client.Close()
		server.Close()
	}
}
func TestNestedCallbackErrorsPreserveOwnerFields(t *testing.T) {
	for _, test := range []struct{ body, code, message, id, retryable string }{
		{`{"error":{"code":"actor_not_linked","message":"账号尚未关联","request_id":"owner-request","retryable":false}}`, "actor_not_linked", "账号尚未关联", "owner-request", "false"},
		{`{"jsonrpc":"2.0","id":"rpc","error":{"code":-32001,"message":"Access denied"}}`, "-32001", "Access denied", "", ""},
	} {
		err := parseHTTPError(403, []byte(test.body), "")
		var got *Error
		if !errors.As(err, &got) || got.Code != test.code || got.Message != test.message || got.RequestID != test.id {
			t.Fatalf("error=%+v", got)
		}
		value, present := got.Details["retryable"]
		if present != (test.retryable != "") || present && value != test.retryable {
			t.Fatalf("retryable=%v present=%t, want %q", value, present, test.retryable)
		}
	}
}
