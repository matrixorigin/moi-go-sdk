package sdk

import (
	"context"
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTeamProductContract(t *testing.T) {
	cases := []struct {
		method, path string
		invoke       func(*AgentService) (any, error)
		response     string
	}{
		{"POST", "", func(s *AgentService) (any, error) {
			return s.CreateTeam(context.Background(), AgentTeamCreateRequest{Name: "team", Members: []*AgentTeamMemberRef{{AgentWorkspaceId: "ws/1", AgentId: "agent/1"}}})
		}, `{"id":"team/1","version":8,"membership_version":9}`},
		{"GET", "", func(s *AgentService) (any, error) { return s.ListTeams(context.Background()) }, `{"items":[{"id":"team/1"}],"total":1,"limit":50,"offset":0}`},
		{"GET", "/agent-cards?limit=20", func(s *AgentService) (any, error) {
			return s.ListAgentCatalogCards(context.Background(), WithQueryParam("limit", "20"))
		}, `{"items":[{"kind":"agent","id":"agent/1","workspace_id":"workspace","name":"Agent","status":"active","updated_at":"2026-01-01T00:00:00Z","actions":["chat"]}],"total":1,"limit":20,"offset":0}`},
		{"GET", "/team%2F1", func(s *AgentService) (any, error) { return s.GetTeam(context.Background(), "team/1") }, `{"id":"team/1","version":8,"membership_version":9}`},
		{"PATCH", "/team%2F1", func(s *AgentService) (any, error) {
			return s.UpdateTeam(context.Background(), "team/1", AgentTeamUpdateRequest{Name: teamTestPtr(""), Description: teamTestPtr(""), ExpectedVersion: 7})
		}, `{"id":"team/1","version":8,"membership_version":9}`},
		{"DELETE", "/team%2F1?expected_version=7", func(s *AgentService) (any, error) { return s.DeleteTeam(context.Background(), "team/1", 7) }, `{"id":"team/1","version":8,"membership_version":9}`},
		{"GET", "/team%2F1/members", func(s *AgentService) (any, error) { return s.ListTeamMembers(context.Background(), "team/1") }, `[{"agent_id":"agent/1","agent_workspace_id":"ws/1"}]`},
		{"PUT", "/team%2F1/members/ws%2F1/agent%2F1?expected_membership_version=7", func(s *AgentService) (any, error) {
			return s.AddTeamMember(context.Background(), "team/1", "ws/1", "agent/1", 7)
		}, `{"id":"team/1","version":8,"membership_version":9}`},
		{"DELETE", "/team%2F1/members/ws%2F1/agent%2F1?expected_membership_version=7", func(s *AgentService) (any, error) {
			return s.RemoveTeamMember(context.Background(), "team/1", "ws/1", "agent/1", 7)
		}, `{"id":"team/1","version":8,"membership_version":9}`},
		{"GET", "/ingress-capabilities", func(s *AgentService) (any, error) {
			return s.ListIngressCapabilities(context.Background())
		}, `{"items":[{"provider":"wecom","channel_type":"wecom_ai_bot","receive_modes":["managed_stream","callback"]}]}`},
		{"POST", "/ingress-connections/wecom-qr", func(s *AgentService) (any, error) {
			return s.ResolveWeComQRBinding(context.Background(), &AgentTargetRef{Kind: "agent", WorkspaceId: "workspace", Id: "agent/1"}, "sealed-session")
		}, `{"status":"authorized","expires_at":"2026-10-16T00:05:00Z","bot_id":"qr-bot","secret":"fresh-secret"}`},
		{"GET", "/ingress-connections?target_id=team%2F1&target_kind=team&target_workspace_id=workspace", func(s *AgentService) (any, error) {
			return s.ListIngressConnections(context.Background(), &AgentTargetRef{Kind: "team", WorkspaceId: "workspace", Id: "team/1"})
		}, `{"items":[{"id":"ingress/1","receive_mode":"callback","channel_instance_id":"channel/1","target":{"kind":"team","workspace_id":"workspace","id":"team/1"}}],"total":1}`},
		{"POST", "/ingress-connections", func(s *AgentService) (any, error) {
			return s.CreateIngressConnection(context.Background(), IngressConnectionCreateRequest{Target: &AgentTargetRef{Kind: "agent", WorkspaceId: "system", Id: "momo"}, Provider: "slack", ChannelType: "slack_app", ReceiveMode: "callback", AgentModel: "fast", Callback: &IngressCallbackConfig{ChannelInstanceID: "channel/1"}})
		}, `{"id":"ingress/1","receive_mode":"callback","channel_instance_id":"channel/1","target":{"kind":"agent","workspace_id":"system","id":"momo"},"agent_model":"fast"}`},
		{"DELETE", "/ingress-connections/ingress%2F1?expected_association_version=7&expected_config_version=8", func(s *AgentService) (any, error) {
			return s.DeleteIngressConnection(context.Background(), "ingress/1", 7, 8)
		}, `{"id":"ingress/1"}`},
		{"GET", "/ingress-connections/ingress%2F1", func(s *AgentService) (any, error) {
			return s.GetIngressConnection(context.Background(), "ingress/1")
		}, `{"id":"ingress/1"}`},
		{"PATCH", "/ingress-connections/ingress%2F1", func(s *AgentService) (any, error) {
			return s.UpdateIngressConnection(context.Background(), "ingress/1", IngressConnectionUpdateRequest{Enabled: teamTestPtr(false), AgentModel: teamTestPtr("fast-2"), ExpectedAssociationVersion: 7, ExpectedConfigVersion: 8})
		}, `{"id":"ingress/1"}`},
		{"POST", "/ingress-connections/ingress%2F1/test", func(s *AgentService) (any, error) {
			return s.VerifyIngressConnection(context.Background(), "ingress/1", IngressConnectionVerifyRequest{ExpectedAssociationVersion: 7, ExpectedConfigVersion: 8, CallbackURL: "https://moi.example/callback"})
		}, `{"id":"ingress/1","observed_state":"connected","enabled":true}`},
		{"GET", "/team%2F1/sessions", func(s *AgentService) (any, error) {
			return s.ListTeamSessions(context.Background(), "team/1")
		}, `{"items":[{"team_id":"team/1","astra_session_id":"astra/1","created_by_moi_user_id":"user"}],"total":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				prefix := "/newmoi/workspaces/workspace/agent-teams"
				if strings.HasPrefix(tc.path, "/agent-cards") || strings.HasPrefix(tc.path, "/ingress-capabilities") || strings.HasPrefix(tc.path, "/ingress-connections") {
					prefix = "/newmoi/workspaces/workspace"
				}
				if r.Method != tc.method || r.URL.RequestURI() != prefix+tc.path {
					t.Errorf("request %s %s", r.Method, r.URL.RequestURI())
				}
				if r.Header.Get("Authorization") != "Bearer token" || r.Header.Get("X-Workspace-ID") != "workspace" {
					t.Error("auth scope missing")
				}
				if r.Method == "POST" || r.Method == "PATCH" || r.Method == "PUT" && r.ContentLength > 0 {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if r.Method == "POST" && tc.path == "" {
						members := body["members"].([]any)
						member := members[0].(map[string]any)
						if member["agent_workspace_id"] != "ws/1" || member["agent_id"] != "agent/1" {
							t.Errorf("member: %v", member)
						}
					}
					if r.Method == "PATCH" && strings.HasPrefix(tc.path, "/team") {
						if body["name"] != "" || body["description"] != "" || body["expected_version"] != float64(7) {
							t.Errorf("patch: %v", body)
						}
					}
					if _, ok := body["expected_config_version"]; ok && r.Method == http.MethodPatch {
						if value, present := body["enabled"]; !present || value != false {
							t.Errorf("explicit disable missing: %v", body)
						}
						if value := body["agent_model"]; value != "fast-2" {
							t.Errorf("explicit model missing: %v", body)
						}
					}
					if tc.path == "/ingress-connections/ingress%2F1/test" && body["callback_url"] != "https://moi.example/callback" {
						t.Errorf("callback URL missing: %v", body)
					}
					if tc.path == "/ingress-connections/wecom-qr" {
						target, ok := body["target"].(map[string]any)
						if !ok || target["workspace_id"] != "workspace" || target["id"] != "agent/1" || body["session_token"] != "sealed-session" {
							t.Error("QR session target or token missing")
						}
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"code":0,"data":` + tc.response + "}"))
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", WithToken("token"))
			if err != nil {
				t.Fatal(err)
			}
			out, err := tc.invoke(client.Agents("workspace"))
			if err != nil {
				t.Fatal(err)
			}
			if out == nil {
				t.Fatal("missing typed response")
			}
		})
	}
}

func teamTestPtr[T any](value T) *T { return &value }

func TestSlackManagedStreamRequestSerialization(t *testing.T) {
	request := IngressConnectionCreateRequest{
		Target:   &AgentTargetRef{Kind: "agent", WorkspaceId: "system", Id: "momo"},
		Provider: "slack", ChannelType: "slack_app", ReceiveMode: "managed_stream",
		ManagedStream: &IngressManagedStreamConfig{AppID: "A1", AppToken: "xapp-test", BotToken: "xoxb-test"},
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	managed := body["managed_stream"].(map[string]any)
	if managed["app_id"] != "A1" || managed["app_token"] != "xapp-test" || managed["bot_token"] != "xoxb-test" {
		t.Fatalf("Slack managed stream credentials: %v", managed)
	}
	if _, ok := managed["bot_id"]; ok {
		t.Fatalf("unconfigured provider field serialized: %v", managed)
	}
}

func TestTeamRouteKinds(t *testing.T) {
	count := 0
	for _, route := range AgentRouteMappings() {
		if !strings.Contains(route.Path, "/agent-teams") {
			continue
		}
		count++
		want := "write"
		if route.Method == http.MethodGet {
			want = "read"
		}
		if route.Kind != want || route.Method == "ANY" {
			t.Errorf("%s %s: kind=%s, want %s", route.Method, route.Path, route.Kind, want)
		}
	}
	if count != 13 {
		t.Fatalf("Team operations: got %d, want 13", count)
	}
}

func TestTeamResponseRejectsUnknownFields(t *testing.T) {
	for _, array := range []bool{false, true} {
		var target proto.Message = &AgentTeamRecord{}
		raw := []byte(`{"UnknownField":true}`)
		if array {
			target = &AgentTeamMemberList{}
			raw = []byte(`[{"UnknownField":true}]`)
		}
		response := &teamResponse{message: target, array: array}
		if err := json.Unmarshal(raw, response); err == nil {
			t.Fatalf("unknown field accepted (array=%v)", array)
		}
	}
}
