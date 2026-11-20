package sdk

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestRuntimeGatewayComputeEventAuthorizationAndExecutorHealth(t *testing.T) {
	type observedRequest struct {
		path, authorization, apiKey, workspace string
		body                                   map[string]any
	}
	var requests []observedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		var body map[string]any
		if len(raw) > 0 {
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			if err := decoder.Decode(&body); err != nil {
				t.Error(err)
			}
		}
		requests = append(requests, observedRequest{r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("X-API-Key"), r.Header.Get("X-Workspace-ID"), body})
		switch r.URL.Path {
		case "/api/v1/internal/compute-bindings/events":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"binding":{"compute_binding_id":"binding-1"},"duplicate":false}`))
		case "/api/v1/runtime-executors/authorize":
			w.WriteHeader(http.StatusNoContent)
		case "/newmoi/agent-executors/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":"OK","data":{"executors":[{"id":"runner-1","connected":true,"connected_secs":9007199254740993,"ready":true}],"astra_reachable":true}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	runtime, err := New(server.URL+"/newmoi", WithToken("runtime-grant"))
	if err != nil {
		t.Fatal(err)
	}
	event := &ComputeBindingEvent{ComputeBindingId: "binding-1", EventType: "activation_requested", WorkspaceId: "workspace-1", SubjectType: "agent", SubjectId: "agent-1", EdgeAgentId: "edge-1", ComputeResourceId: "compute-1", SpecId: "spec-1", PlacementProfile: "default", Units: 2, LifecycleInstanceId: "lifecycle-1", FencingToken: 9007199254740993, Kind: "sandbox", IdempotencyKey: "event-1", OccurredAt: "2026-11-10T01:02:03Z"}
	result, err := runtime.RuntimeGateway().ApplyComputeBindingEvent(t.Context(), event)
	if err != nil {
		t.Fatal(err)
	}
	if result.Value.(map[string]any)["duplicate"] != false {
		t.Fatalf("result=%v", result.Value)
	}
	wantEvent := map[string]any{"compute_binding_id": "binding-1", "event_type": "activation_requested", "workspace_id": "workspace-1", "subject_type": "agent", "subject_id": "agent-1", "edge_agent_id": "edge-1", "compute_resource_id": "compute-1", "spec_id": "spec-1", "placement_profile": "default", "units": json.Number("2"), "lifecycle_instance_id": "lifecycle-1", "fencing_token": json.Number("9007199254740993"), "kind": "sandbox", "idempotency_key": "event-1", "occurred_at": "2026-11-10T01:02:03Z"}
	if !reflect.DeepEqual(requests[0].body, wantEvent) {
		t.Fatalf("event body=%#v", requests[0].body)
	}
	if got, ok := requests[0].body["fencing_token"].(json.Number); !ok || got.String() != "9007199254740993" {
		t.Fatalf("fencing_token=%#v", requests[0].body["fencing_token"])
	}
	authorization := &RuntimeExecutorAuthorizationRequest{TaskId: "task-1", ExecutorId: "executor-1", RunId: "run-1", TurnChainId: "turn-1", ToolCallId: "tool-1"}
	if err := runtime.RuntimeGateway().AuthorizeExecutor(t.Context(), authorization); err != nil {
		t.Fatal(err)
	}
	product, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	health, err := product.Agents("workspace-1").ExecutorHealth(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if health.Executors[0].GetConnectedSecs() != 9007199254740993 {
		t.Fatalf("health=%v", health)
	}
	if requests[0].authorization != "Bearer runtime-grant" || requests[1].authorization != "Bearer runtime-grant" || requests[2].apiKey != "user-key" || requests[2].workspace != "workspace-1" {
		t.Fatalf("requests=%#v", requests)
	}
}

func TestRuntimeGatewayEdgeModelRPCAndStream(t *testing.T) {
	type observed struct {
		path, authorization string
		body                map[string]any
	}
	var requests []observed
	stream := []byte("data: {\"choices\":[]}\n\ndata: [DONE]\n\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		_ = decoder.Decode(&body)
		requests = append(requests, observed{r.URL.Path, r.Header.Get("Authorization"), body})
		switch r.URL.Path {
		case "/api/v1/astra/edge-tokens/check":
			_, _ = w.Write([]byte(`{"code":0,"data":{"ok":true,"subject":"subject-1","workspace_id":"workspace-1","edge_agent_id":"edge-1","expires_at":"2026-11-10T02:00:00Z"}}`))
		case "/newmoi/edge-tokens/renew":
			_, _ = w.Write([]byte(`{"code":0,"data":{"token":"fresh-edge","expires_at":"2026-11-10T03:00:00Z"}}`))
		case "/api/v1/models/resolve":
			_, _ = w.Write([]byte(`{"model":"model-1","status":"ready","protocol":"openai","invoke":{"url":"http://model.test/v1"}}`))
		case "/api/v1/mcp/http":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"rpc-1","result":{"tools":[]}}`))
		case "/api/v1/models/openai/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write(stream)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithToken("runtime-grant"))
	if err != nil {
		t.Fatal(err)
	}
	gateway := client.RuntimeGateway()
	checked, err := gateway.CheckEdgeToken(t.Context(), "edge-registration")
	if err != nil {
		t.Fatal(err)
	}
	if !checked.GetOk() || checked.GetSubject() != "subject-1" || checked.GetWorkspaceId() != "workspace-1" || checked.GetEdgeAgentId() != "edge-1" {
		t.Fatalf("checked=%v", checked)
	}
	renewed, err := gateway.RenewEdgeToken(t.Context(), "edge-registration")
	if err != nil || renewed.GetToken() != "fresh-edge" {
		t.Fatalf("renewed=%v error=%v", renewed, err)
	}
	resolved, err := gateway.ResolveModel(t.Context(), &RuntimeModelResolveRequest{Model: "model-1", Gateway: "control-plane"})
	if err != nil || resolved.GetInvoke().GetUrl() != "http://model.test/v1" {
		t.Fatalf("resolved=%v error=%v", resolved, err)
	}
	mcpParams, _ := structpb.NewStruct(map[string]any{"cursor": "next"})
	rpc, err := gateway.MCP(t.Context(), &RuntimeRPCRequest{Jsonrpc: "2.0", Id: structpb.NewStringValue("rpc-1"), Method: "tools/list", Params: mcpParams})
	if err != nil || rpc.GetResult().GetFields()["tools"] == nil {
		t.Fatalf("rpc=%v error=%v", rpc, err)
	}
	invoke, _ := structpb.NewStruct(map[string]any{"model": "model-1", "messages": []any{map[string]any{"role": "user", "content": "hello"}}, "stream": true})
	response, err := gateway.InvokeModel(t.Context(), invoke)
	if err != nil {
		t.Fatal(err)
	}
	content, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || !bytes.Equal(content, stream) {
		t.Fatalf("stream=%q read=%v close=%v", content, readErr, closeErr)
	}
	wantPaths := []string{"/api/v1/astra/edge-tokens/check", "/newmoi/edge-tokens/renew", "/api/v1/models/resolve", "/api/v1/mcp/http", "/api/v1/models/openai/chat/completions"}
	for index, path := range wantPaths {
		if requests[index].path != path || requests[index].authorization != "Bearer runtime-grant" {
			t.Fatalf("request[%d]=%#v", index, requests[index])
		}
	}
	wantBodies := []map[string]any{
		{"token": "edge-registration"}, {"token": "edge-registration"}, {"model": "model-1", "gateway": "control-plane"},
		{"jsonrpc": "2.0", "id": "rpc-1", "method": "tools/list", "params": map[string]any{"cursor": "next"}},
		{"model": "model-1", "messages": []any{map[string]any{"role": "user", "content": "hello"}}, "stream": true},
	}
	for index, want := range wantBodies {
		if !reflect.DeepEqual(requests[index].body, want) {
			t.Fatalf("body[%d]=%#v want=%#v", index, requests[index].body, want)
		}
	}
}
