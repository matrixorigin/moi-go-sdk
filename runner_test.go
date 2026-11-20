package sdk

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const runnerContractJSON = `{"id":"runner-id","workspace_id":"workspace-id","user_id":"user-id","name":"local runner","runner_type":"custom","edge_agent_id":"runner-runner-id","connect_cmd":"astra-edge --token signed-token","phase":"ready","token_expires_at":"2026-11-04T01:02:03Z","created_at":"2026-10-04T01:02:03Z","updated_at":"2026-10-04T01:02:04Z"}`

func TestRunnerSDKRoutesAndPublicFields(t *testing.T) {
	for _, auth := range []struct {
		name, header, value string
		option              Option
	}{
		{"api-key", "X-API-Key", "user-key", WithAPIKey("user-key")},
		{"bearer", "Authorization", "Bearer user-token", WithToken("user-token")},
	} {
		t.Run(auth.name, func(t *testing.T) {
			var calls []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.Method+" "+r.URL.Path)
				if r.Header.Get(auth.header) != auth.value || r.Header.Get(headerWorkspaceID) != "workspace-id" || r.Header.Get(headerRequestID) != "request-id" {
					t.Errorf("request headers=%v", r.Header)
				}
				if r.Method == http.MethodPost && r.URL.Path == "/newmoi/agent-runners/custom" {
					var body map[string]string
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) != 1 || body["name"] != "local runner" {
						t.Errorf("create body=%v err=%v", body, err)
					}
				}
				data := runnerContractJSON
				if r.Method == http.MethodGet && r.URL.Path == "/newmoi/agent-runners" {
					data = "[" + data + "]"
				}
				if r.Method == http.MethodDelete {
					data = "{}"
				}
				_, _ = w.Write([]byte(`{"code":"OK","msg":"OK","data":` + data + `}`))
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", auth.option, WithCustomHeader(headerRequestID, "request-id"))
			if err != nil {
				t.Fatal(err)
			}
			service := client.Runners("workspace-id")
			created, err := service.Create(t.Context(), "local runner")
			if err != nil {
				t.Fatal(err)
			}
			assertRunnerPublicFields(t, created)
			items, err := service.List(t.Context())
			if err != nil || len(items) != 1 {
				t.Fatalf("list=%v err=%v", items, err)
			}
			assertRunnerPublicFields(t, items[0])
			item, err := service.Get(t.Context(), "runner-id")
			if err != nil {
				t.Fatal(err)
			}
			assertRunnerPublicFields(t, item)
			item, err = service.Refresh(t.Context(), "runner-id")
			if err != nil {
				t.Fatal(err)
			}
			assertRunnerPublicFields(t, item)
			if err := service.Delete(t.Context(), "runner-id"); err != nil {
				t.Fatal(err)
			}
			want := "POST /newmoi/agent-runners/custom\nGET /newmoi/agent-runners\nGET /newmoi/agent-runners/runner-id\nPOST /newmoi/agent-runners/runner-id/refresh-token\nDELETE /newmoi/agent-runners/runner-id"
			if strings.Join(calls, "\n") != want {
				t.Fatalf("requests=%v", calls)
			}
		})
	}
}

func assertRunnerPublicFields(t *testing.T, item *AgentRunner) {
	t.Helper()
	if item.GetId() != "runner-id" || item.GetWorkspaceId() != "workspace-id" || item.GetUserId() != "user-id" || item.GetName() != "local runner" || item.GetRunnerType() != "custom" || item.GetEdgeAgentId() != "runner-runner-id" || item.GetConnectCmd() != "astra-edge --token signed-token" || item.GetPhase() != "ready" || item.GetTokenExpiresAt() != "2026-11-04T01:02:03Z" || item.GetCreatedAt() != "2026-10-04T01:02:03Z" || item.GetUpdatedAt() != "2026-10-04T01:02:04Z" {
		t.Fatalf("runner response=%v", item)
	}
}

func TestRunnerSDKStrictResponseContract(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
		out  any
	}{
		{"get-unknown-field", `{"id":"r","token_jti":"private"}`, &AgentRunner{}},
		{"list-unknown-field", `[{"id":"r","token_jti":"private"}]`, new(runnerList)},
		{"list-object", `{"runners":[]}`, new(runnerList)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := parseProductResponse([]byte(`{"code":"OK","data":`+test.data+`}`), test.out, ""); err == nil {
				t.Fatal("invalid Runner response accepted")
			}
		})
	}
}

func TestRunnerSDKPreservesPublicError(t *testing.T) {
	for _, test := range []struct {
		status int
		code   string
	}{
		{400, "ErrParamInvalid"}, {404, "ErrRunnerNotFound"}, {409, "ErrRunnerNameConflict"}, {409, "ErrRunnerIamRequestConflict"}, {503, "ErrRunnerIamUnavailable"},
	} {
		t.Run(test.code, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(headerRequestID, "request-id")
				w.WriteHeader(test.status)
				_ = json.NewEncoder(w).Encode(map[string]string{"code": test.code, "msg": "原始本地化错误"})
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Runners("workspace-id").Create(t.Context(), "name")
			var failure *Error
			if !errors.As(err, &failure) || failure.Code != test.code || failure.Message != "原始本地化错误" || failure.Status != test.status || failure.RequestID != "request-id" {
				t.Fatalf("Runner error=%#v", err)
			}
		})
	}
}
