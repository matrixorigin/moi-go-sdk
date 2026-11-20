package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type workspaceModelContract struct {
	Name        string          `json:"name"`
	Kind        string          `json:"kind"`
	Operation   string          `json:"operation"`
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	ID          int64           `json:"id"`
	EndpointID  int64           `json:"endpoint_id"`
	Address     string          `json:"address"`
	StatusValue int32           `json:"status_value"`
	Input       json.RawMessage `json:"input"`
	Request     json.RawMessage `json:"request"`
	Response    json.RawMessage `json:"response"`
	Status      int             `json:"status"`
}

func workspaceModelContractData(t *testing.T, raw json.RawMessage) json.RawMessage {
	t.Helper()
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) > 0 {
		return envelope.Data
	}
	return raw
}

func workspaceModelExpected(t *testing.T, raw json.RawMessage, message proto.Message) {
	t.Helper()
	if err := protojson.Unmarshal(workspaceModelContractData(t, raw), message); err != nil {
		t.Fatal(err)
	}
}

func workspaceModelJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&value)
	return value
}

func TestWorkspaceModelSharedHTTPContracts(t *testing.T) {
	data, err := os.ReadFile("testdata/workspace_model_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []workspaceModelContract
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			var received any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != scenario.Method || r.URL.Path != scenario.Path || r.Header.Get("X-Workspace-ID") != "workspace-1" || r.Header.Get("X-API-Key") != "user-key" {
					t.Errorf("request=%s %s headers=%v", r.Method, r.URL.Path, r.Header)
				}
				raw, _ := io.ReadAll(r.Body)
				if len(raw) > 0 {
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					_ = decoder.Decode(&received)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				_, _ = w.Write(scenario.Response)
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
			if err != nil {
				t.Fatal(err)
			}
			service := client.Models("workspace-1")
			embedding := scenario.Kind == "embedding"
			ctx := t.Context()
			switch scenario.Operation {
			case "create-backend":
				var request WorkspaceLLMBackendCreateRequest
				if err := protojson.Unmarshal(scenario.Input, &request); err != nil {
					t.Fatal(err)
				}
				var got *WorkspaceLLMBackend
				if embedding {
					got, err = service.CreateWorkspaceEmbeddingBackend(ctx, &request)
				} else {
					got, err = service.CreateWorkspaceLLMBackend(ctx, &request)
				}
				expected := &WorkspaceLLMBackend{}
				workspaceModelExpected(t, scenario.Response, expected)
				if err != nil || !proto.Equal(got, expected) {
					t.Fatalf("backend=%v error=%v want=%v", got, err, expected)
				}
			case "list-backends":
				var got *WorkspaceLLMBackendList
				if embedding {
					got, err = service.ListWorkspaceEmbeddingBackends(ctx)
				} else {
					got, err = service.ListWorkspaceLLMBackends(ctx)
				}
				expected := &WorkspaceLLMBackendList{}
				workspaceModelExpected(t, scenario.Response, expected)
				if err != nil || !proto.Equal(got, expected) {
					t.Fatalf("list=%v error=%v", got, err)
				}
			case "get-backend":
				var got *WorkspaceLLMBackend
				if embedding {
					got, err = service.GetWorkspaceEmbeddingBackend(ctx, scenario.ID)
				} else {
					got, err = service.GetWorkspaceLLMBackend(ctx, scenario.ID)
				}
				expected := &WorkspaceLLMBackend{}
				workspaceModelExpected(t, scenario.Response, expected)
				if err != nil || !proto.Equal(got, expected) {
					t.Fatalf("backend=%v error=%v", got, err)
				}
			case "update-backend":
				var request WorkspaceLLMBackendUpdateRequest
				if err := protojson.Unmarshal(scenario.Input, &request); err != nil {
					t.Fatal(err)
				}
				var got *WorkspaceLLMBackend
				if embedding {
					got, err = service.UpdateWorkspaceEmbeddingBackend(ctx, scenario.ID, &request)
				} else {
					got, err = service.UpdateWorkspaceLLMBackend(ctx, scenario.ID, &request)
				}
				if scenario.Status >= 400 {
					var failure *Error
					if !errors.As(err, &failure) || failure.Code != "ErrConflict" || failure.Status != 409 || failure.Message != "Synthetic backend conflict" {
						t.Fatalf("error=%#v", err)
					}
				} else {
					expected := &WorkspaceLLMBackend{}
					workspaceModelExpected(t, scenario.Response, expected)
					if err != nil || !proto.Equal(got, expected) {
						t.Fatalf("backend=%v error=%v", got, err)
					}
				}
			case "delete-backend":
				if embedding {
					err = service.DeleteWorkspaceEmbeddingBackend(ctx, scenario.ID)
				} else {
					err = service.DeleteWorkspaceLLMBackend(ctx, scenario.ID)
				}
				if err != nil {
					t.Fatal(err)
				}
			case "create-endpoint":
				var got *WorkspaceLLMEndpoint
				if embedding {
					got, err = service.CreateWorkspaceEmbeddingEndpoint(ctx, scenario.ID, scenario.Address)
				} else {
					got, err = service.CreateWorkspaceLLMEndpoint(ctx, scenario.ID, scenario.Address)
				}
				expected := &WorkspaceLLMEndpoint{}
				workspaceModelExpected(t, scenario.Response, expected)
				if err != nil || !proto.Equal(got, expected) {
					t.Fatalf("endpoint=%v error=%v", got, err)
				}
			case "list-endpoints":
				var got []*WorkspaceLLMEndpoint
				if embedding {
					got, err = service.ListWorkspaceEmbeddingEndpoints(ctx, scenario.ID)
				} else {
					got, err = service.ListWorkspaceLLMEndpoints(ctx, scenario.ID)
				}
				if err != nil || len(got) != 1 || got[0].GetId() != scenario.EndpointID && scenario.EndpointID != 0 || len(got) == 1 && got[0].GetBackendId() != scenario.ID {
					t.Fatalf("endpoints=%v error=%v", got, err)
				}
			case "router":
				var got *WorkspaceLLMRouterConfigResult
				if embedding {
					got, err = service.WorkspaceEmbeddingRouter(ctx)
				} else {
					got, err = service.WorkspaceLLMRouter(ctx)
				}
				expected := &WorkspaceLLMRouterConfigResult{}
				workspaceModelExpected(t, scenario.Response, expected)
				if err != nil || !proto.Equal(got, expected) {
					t.Fatalf("router=%v error=%v", got, err)
				}
			case "update-router":
				var request WorkspaceLLMRouterUpdateRequest
				if err := protojson.Unmarshal(scenario.Input, &request); err != nil {
					t.Fatal(err)
				}
				var got *WorkspaceLLMRouterConfigResult
				if embedding {
					got, err = service.UpdateWorkspaceEmbeddingRouter(ctx, &request)
				} else {
					got, err = service.UpdateWorkspaceLLMRouter(ctx, &request)
				}
				expected := &WorkspaceLLMRouterConfigResult{}
				workspaceModelExpected(t, scenario.Response, expected)
				if err != nil || !proto.Equal(got, expected) {
					t.Fatalf("router=%v error=%v", got, err)
				}
			case "set-endpoint-status":
				if embedding {
					got, callErr := service.SetWorkspaceEmbeddingEndpointStatus(ctx, scenario.ID, scenario.EndpointID, scenario.StatusValue)
					if callErr != nil || !got.GetUpdated() {
						t.Fatalf("updated=%v error=%v", got, callErr)
					}
				} else if callErr := service.SetWorkspaceLLMEndpointStatus(ctx, scenario.ID, scenario.EndpointID, scenario.StatusValue); callErr != nil {
					t.Fatal(callErr)
				}
			case "list-models":
				got, callErr := service.ListWorkspaceLLMModels(ctx)
				expected := &ProviderModelListResult{}
				workspaceModelExpected(t, scenario.Response, expected)
				if callErr != nil || !proto.Equal(got, expected) {
					t.Fatalf("models=%v error=%v", got, callErr)
				}
			default:
				t.Fatalf("unknown operation %s", scenario.Operation)
			}
			if !reflect.DeepEqual(received, workspaceModelJSON(scenario.Request)) {
				t.Fatalf("body=%#v want=%#v", received, workspaceModelJSON(scenario.Request))
			}
		})
	}
}
