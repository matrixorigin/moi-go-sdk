package sdk

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFrontendMissingSDKRoutesPreserveContracts(t *testing.T) {
	for _, auth := range []struct {
		name          string
		option        Option
		header, value string
	}{
		{"api_key", WithAPIKey("key"), "X-API-Key", "key"},
		{"token", WithToken("token"), "Authorization", "Bearer token"},
	} {
		t.Run(auth.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get(auth.header) != auth.value {
					t.Errorf("authentication changed: %v", r.Header)
				}
				if r.URL.Path != "/newmoi/user/me/registration-invite" && r.URL.Path != "/newmoi/user/me/reward-center/items" && r.URL.Path != "/newmoi/user/me/reward-center/events" && r.Header.Get("X-Workspace-ID") != "workspace" {
					t.Errorf("workspace lost: %v", r.Header)
				}
				switch r.Method + " " + r.URL.Path {
				case "GET /newmoi/admin/agent-sandboxes":
					if r.URL.Query().Get("workspace_id") != "target" {
						t.Errorf("admin filter lost: %s", r.URL)
					}
					_, _ = io.WriteString(w, `{"code":"OK","data":{"items":[{"id":"sandbox","workspace":{"id":"target","owner_id":"owner","account_name":"account"}}],"limit":50,"offset":0,"has_more":true}}`)
				case "DELETE /newmoi/admin/agent-sandboxes/sandbox":
					_, _ = io.WriteString(w, `{"code":"OK","data":{}}`)
				case "PATCH /newmoi/admin/agent-sandboxes/sandbox/network":
					var request SandboxNetworkRequest
					data, err := io.ReadAll(r.Body)
					if err != nil || json.Unmarshal(data, &request) != nil || len(request.NetworkTools) != 1 || request.NetworkTools[0] != "git" {
						t.Errorf("network body changed: %s error=%v", data, err)
					}
					_, _ = io.WriteString(w, `{"code":"OK","data":{"id":"sandbox","network_tools":["git"],"workspace":{"id":"target"}}}`)
				case "GET /newmoi/agent-sandboxes/sandbox/terminal":
					_, _ = io.WriteString(w, `{"code":"OK","data":{"token":"signed-token","expires_at":"2026-10-08T01:02:03Z"}}`)
				case "GET /newmoi/agent-executors/health":
					_, _ = io.WriteString(w, `{"code":"OK","data":{"executors":[{"id":"runner","type":"runner","connected":true,"ready":true,"connected_secs":0}],"astra_reachable":true}}`)
				case "GET /newmoi/agents":
					if r.URL.Query().Get("surface") != "agent_chat" {
						t.Errorf("agent query lost: %s", r.URL)
					}
					_, _ = io.WriteString(w, `{"code":0,"data":{"items":[{"id":"agent"}],"total":1}}`)
				case "GET /newmoi/user/me/registration-invite":
					_, _ = io.WriteString(w, `{"code":"OK","data":{"invite_code":"invite"}}`)
				case "GET /newmoi/user/me/reward-center/items":
					_, _ = io.WriteString(w, `{"code":"OK","data":[{"id":"reward"}]}`)
				case "GET /newmoi/user/me/reward-center/events":
					if r.URL.Query().Get("event_type") != "account.checkin & reward" {
						t.Errorf("event query changed: %s", r.URL)
					}
					_, _ = io.WriteString(w, `{"code":"OK","data":[{"event_type":"account.checkin"}]}`)
				case "GET /newmoi/workspaces/workspace/agents/agent/query-visuals/file/preview":
					w.Header().Set("Content-Type", "image/png")
					w.Header().Set("Cache-Control", "private, no-store, max-age=0")
					_, _ = io.WriteString(w, "image-bytes")
				case "POST /newmoi/query/result/download":
					var request map[string]string
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request) != 1 || request["statement_id"] != "statement" {
						t.Errorf("CSV request changed: %v error=%v", request, err)
					}
					w.Header().Set("Content-Type", "text/csv")
					w.Header().Set("Content-Disposition", `attachment; filename="query-result.csv"`)
					_, _ = io.WriteString(w, "answer\n1\n")
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", auth.option)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			sandboxes, agents := client.Sandboxes("workspace"), client.Agents("workspace")
			list, err := sandboxes.AdminList(t.Context(), WithQueryParam("workspace_id", "target"))
			if err != nil || !list.GetHasMore() || len(list.GetItems()) != 1 || list.GetItems()[0].GetWorkspace().GetAccountName() != "account" {
				t.Fatalf("admin list=%v error=%v", list, err)
			}
			if err := sandboxes.AdminDelete(t.Context(), "sandbox"); err != nil {
				t.Fatal(err)
			}
			network, err := sandboxes.AdminUpdateNetwork(t.Context(), "sandbox", &SandboxNetworkRequest{NetworkTools: []string{"git"}})
			if err != nil || network.GetWorkspace().GetId() != "target" || len(network.GetNetworkTools()) != 1 {
				t.Fatalf("network=%v error=%v", network, err)
			}
			terminal, err := sandboxes.TerminalToken(t.Context(), "sandbox")
			if err != nil || terminal.GetToken() != "signed-token" || terminal.GetExpiresAt() != "2026-10-08T01:02:03Z" {
				t.Fatalf("terminal=%v error=%v", terminal, err)
			}
			health, err := agents.ExecutorHealth(t.Context())
			if err != nil || !health.GetAstraReachable() || len(health.GetExecutors()) != 1 || health.GetExecutors()[0].ConnectedSecs == nil || !health.GetExecutors()[0].GetReady() {
				t.Fatalf("health=%v error=%v", health, err)
			}
			if _, err := agents.List(t.Context(), WithQueryParam("surface", "agent_chat")); err != nil {
				t.Fatal(err)
			}
			if _, err := client.Account().RegistrationInvite(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := client.Account().RewardCenterItems(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := client.Account().RewardCenterEvents(t.Context(), "account.checkin & reward"); err != nil {
				t.Fatal(err)
			}
			preview, err := agents.PreviewQueryVisual(t.Context(), "agent", "file")
			if err != nil {
				t.Fatal(err)
			}
			content, err := io.ReadAll(preview.Body)
			closeErr := preview.Body.Close()
			if err != nil || closeErr != nil || string(content) != "image-bytes" || preview.Header.Get("Cache-Control") != "private, no-store, max-age=0" {
				t.Fatalf("image stream=%s error=%v close=%v", content, err, closeErr)
			}
			csv, err := client.SQL("workspace").DownloadQueryResult(t.Context(), "statement")
			if err != nil {
				t.Fatal(err)
			}
			content, err = io.ReadAll(csv.Body)
			closeErr = csv.Body.Close()
			if err != nil || closeErr != nil || string(content) != "answer\n1\n" || csv.Header.Get("Content-Disposition") == "" {
				t.Fatalf("CSV stream=%s error=%v close=%v", content, err, closeErr)
			}
		})
	}
}

func TestFrontendMissingSDKRoutesPreserveErrorEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "request-id")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"code":"SANDBOX_FORBIDDEN","msg":"无访问权限","data":null}`)
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for name, call := range map[string]func() error{
		"admin_list":   func() error { _, err := client.Sandboxes("workspace").AdminList(t.Context()); return err },
		"admin_delete": func() error { return client.Sandboxes("workspace").AdminDelete(t.Context(), "sandbox") },
		"admin_network": func() error {
			_, err := client.Sandboxes("workspace").AdminUpdateNetwork(t.Context(), "sandbox", &SandboxNetworkRequest{NetworkTools: []string{"git"}})
			return err
		},
		"terminal": func() error {
			_, err := client.Sandboxes("workspace").TerminalToken(t.Context(), "sandbox")
			return err
		},
		"health":       func() error { _, err := client.Agents("workspace").ExecutorHealth(t.Context()); return err },
		"agents":       func() error { _, err := client.Agents("workspace").List(t.Context()); return err },
		"registration": func() error { _, err := client.Account().RegistrationInvite(t.Context()); return err },
		"reward_items": func() error { _, err := client.Account().RewardCenterItems(t.Context()); return err },
		"reward_events": func() error {
			_, err := client.Account().RewardCenterEvents(t.Context(), "account.checkin")
			return err
		},
		"image": func() error {
			_, err := client.Agents("workspace").PreviewQueryVisual(t.Context(), "agent", "file")
			return err
		},
		"csv": func() error {
			_, err := client.SQL("workspace").DownloadQueryResult(t.Context(), "statement")
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := call()
			public, ok := err.(*Error)
			if !ok || public.Code != "SANDBOX_FORBIDDEN" || public.Message != "无访问权限" || public.RequestID != "request-id" || public.Status != http.StatusForbidden {
				t.Fatalf("public error changed: %v", err)
			}
		})
	}
}
