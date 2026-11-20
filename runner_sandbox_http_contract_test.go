package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestRunnerSandboxMatchSharedHTTPContract(t *testing.T) {
	var cases []struct {
		Name, Domain, Operation, Method, Path, ID string
		TargetWorkspaceID                         string `json:"target_workspace_id"`
		Input, Request, Response                  json.RawMessage
		Query                                     map[string]string
		Status                                    int
	}
	data, err := os.ReadFile("testdata/runner_sandbox_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()
			decode := func(raw []byte) any {
				t.Helper()
				if len(raw) == 0 {
					return nil
				}
				var value any
				decoder := json.NewDecoder(bytes.NewReader(raw))
				decoder.UseNumber()
				if err := decoder.Decode(&value); err != nil {
					t.Fatal(err)
				}
				return value
			}
			query := url.Values{}
			var options []QueryOption
			for key, value := range scenario.Query {
				query.Set(key, value)
				options = append(options, WithQueryParam(key, value))
			}
			received := make(chan []byte, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != scenario.Method || r.URL.Path != scenario.Path || !reflect.DeepEqual(r.URL.Query(), query) || r.Header.Get("X-API-Key") != "synthetic-key" || r.Header.Get("X-Workspace-ID") != "workspace-1" {
					t.Errorf("unexpected scoped request: %s %s", r.Method, r.URL.RequestURI())
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				received <- raw
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				_, _ = w.Write(scenario.Response)
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			parse := func(value proto.Message) {
				t.Helper()
				if err := protojson.Unmarshal(scenario.Input, value); err != nil {
					t.Fatal(err)
				}
			}
			var actual any
			var callErr error
			if scenario.Domain == "runner" {
				service := client.Runners("workspace-1")
				switch scenario.Operation {
				case "create":
					var input struct{ Name string }
					if err := json.Unmarshal(scenario.Input, &input); err != nil {
						t.Fatal(err)
					}
					actual, callErr = service.Create(t.Context(), input.Name)
				case "list":
					actual, callErr = service.List(t.Context())
				case "get":
					actual, callErr = service.Get(t.Context(), scenario.ID)
				case "refresh":
					actual, callErr = service.Refresh(t.Context(), scenario.ID)
				case "delete":
					callErr = service.Delete(t.Context(), scenario.ID)
				default:
					t.Fatalf("unknown runner fixture operation: %s", scenario.Operation)
				}
			} else {
				service := client.Sandboxes("workspace-1")
				switch scenario.Operation {
				case "create":
					request := &SandboxCreateRequest{}
					parse(request)
					actual, callErr = service.Create(t.Context(), request)
				case "list":
					actual, callErr = service.List(t.Context())
				case "get":
					actual, callErr = service.Get(t.Context(), scenario.ID)
				case "delete":
					callErr = service.Delete(t.Context(), scenario.ID)
				case "update-network", "admin-update-network":
					request := &SandboxNetworkRequest{}
					parse(request)
					if scenario.Operation == "update-network" {
						actual, callErr = service.UpdateNetwork(t.Context(), scenario.ID, request)
					} else {
						actual, callErr = service.AdminUpdateNetwork(t.Context(), scenario.ID, request)
					}
				case "get-quota":
					actual, callErr = service.GetQuota(t.Context(), options...)
				case "update-quota", "admin-update-quota":
					request := &SandboxQuotaRequest{}
					parse(request)
					if scenario.Operation == "update-quota" {
						actual, callErr = service.UpdateQuota(t.Context(), request)
					} else {
						actual, callErr = service.AdminUpdateQuota(t.Context(), scenario.TargetWorkspaceID, request)
					}
				case "terminal-token":
					actual, callErr = service.TerminalToken(t.Context(), scenario.ID)
				case "create-ssh-session":
					actual, callErr = service.CreateSSHSession(t.Context(), scenario.ID)
				case "admin-list":
					actual, callErr = service.AdminList(t.Context(), options...)
				case "admin-get":
					actual, callErr = service.AdminGet(t.Context(), scenario.ID)
				case "admin-delete":
					callErr = service.AdminDelete(t.Context(), scenario.ID)
				default:
					t.Fatalf("unknown sandbox fixture operation: %s", scenario.Operation)
				}
			}
			select {
			case raw := <-received:
				if !reflect.DeepEqual(decode(raw), decode(scenario.Request)) {
					t.Errorf("HTTP body=%s, want=%s", raw, scenario.Request)
				}
			default:
				t.Fatalf("no HTTP request received: %v", callErr)
			}
			var response struct {
				Code, Msg, Message string
				Data               json.RawMessage
			}
			if err := json.Unmarshal(scenario.Response, &response); err != nil {
				t.Fatal(err)
			}
			if scenario.Status >= 400 {
				var failure *Error
				message := response.Msg
				if message == "" {
					message = response.Message
				}
				if !errors.As(callErr, &failure) || failure.Code != response.Code || failure.Message != message || failure.Status != scenario.Status {
					t.Fatalf("owner error changed: %v", callErr)
				}
				return
			}
			if callErr != nil {
				t.Fatal(callErr)
			}
			checkMessage := func(actual proto.Message, raw []byte) {
				t.Helper()
				want := actual.ProtoReflect().New().Interface()
				if err := protojson.Unmarshal(raw, want); err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(actual, want) {
					t.Errorf("result=%v, want=%v", actual, want)
				}
			}
			switch actual := actual.(type) {
			case proto.Message:
				checkMessage(actual, response.Data)
			case []*AgentRunner:
				var items []json.RawMessage
				if err := json.Unmarshal(response.Data, &items); err != nil {
					t.Fatal(err)
				}
				if len(actual) != len(items) {
					t.Fatalf("runner results=%v, want=%s", actual, response.Data)
				}
				for i, item := range items {
					checkMessage(actual[i], item)
				}
			case []*AgentSandbox:
				var items []json.RawMessage
				if err := json.Unmarshal(response.Data, &items); err != nil {
					t.Fatal(err)
				}
				if len(actual) != len(items) {
					t.Fatalf("sandbox results=%v, want=%s", actual, response.Data)
				}
				for i, item := range items {
					checkMessage(actual[i], item)
				}
			case nil:
			default:
				t.Fatalf("unhandled public result: %T", actual)
			}
		})
	}
}
