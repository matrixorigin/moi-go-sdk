package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfirmSkillCandidateContract(t *testing.T) {
	for _, bind := range []bool{false, true} {
		t.Run(map[bool]string{false: "save", true: "bind"}[bind], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.EscapedPath() != "/newmoi/workspaces/workspace%201/agent-runtime-tasks/task%201/skills/confirm" {
					t.Errorf("request=%s %s", r.Method, r.URL.EscapedPath())
				}
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				wantBind := "false"
				if bind {
					wantBind = "true"
				}
				if string(body["bind_current"]) != wantBind || string(body["call_id"]) != `"call-1"` || string(body["source_digest"]) != `"digest-1"` {
					t.Errorf("body=%s", body)
				}
				if err := json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": map[string]any{"skill_id": "skill-1", "workspace_id": "workspace 1", "agent_id": "agent-1", "bound": bind}}); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.Agents("workspace 1").ConfirmSkillCandidate(t.Context(), "task 1", &ConfirmSkillCandidateRequest{CallId: "call-1", SourceDigest: "digest-1", BindCurrent: &bind})
			if err != nil || result.GetSkillId() != "skill-1" || result.GetWorkspaceId() != "workspace 1" || result.GetAgentId() != "agent-1" || result.GetBound() != bind {
				t.Fatalf("result=%v err=%v", result, err)
			}
		})
	}
}

func TestConfirmSkillCandidatePreservesBindingFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "request-1")
		w.WriteHeader(http.StatusConflict)
		_, err := w.Write([]byte(`{"code":"ErrConflict","msg":"Binding was rejected","details":{"reason":"SKILL_CREATOR_BINDING_FAILED","saved_skill_id":"skill-1"}}`))
		if err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Agents("workspace-1").ConfirmSkillCandidate(t.Context(), "task-1", &ConfirmSkillCandidateRequest{CallId: "call-1"})
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "ErrConflict" || failure.Message != "Binding was rejected" || failure.Status != http.StatusConflict || failure.RequestID != "request-1" || failure.Details["reason"] != "SKILL_CREATOR_BINDING_FAILED" || failure.Details["saved_skill_id"] != "skill-1" {
		t.Fatalf("error=%#v", err)
	}
}

func TestGetSkillCandidateConfirmationContract(t *testing.T) {
	for _, state := range []struct{ saved, bound bool }{{false, false}, {true, false}, {true, true}} {
		t.Run(fmt.Sprintf("saved=%t/bound=%t", state.saved, state.bound), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.EscapedPath() != "/newmoi/workspaces/workspace%201/agent-runtime-tasks/task%201/skills/confirmation" || r.URL.Query().Get("call_id") != "call /1" || r.URL.Query().Get("source_digest") != "sha256:a+b" {
					t.Errorf("request=%s %s", r.Method, r.URL)
				}
				if err := json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": map[string]any{"skill_id": "skill-1", "workspace_id": "workspace 1", "agent_id": "agent-1", "saved": state.saved, "bound": state.bound}}); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.Agents("workspace 1").GetSkillCandidateConfirmation(t.Context(), "task 1", "call /1", "sha256:a+b")
			if err != nil || result.GetSkillId() != "skill-1" || result.Saved == nil || result.Bound == nil || result.GetSaved() != state.saved || result.GetBound() != state.bound {
				t.Fatalf("result=%v err=%v", result, err)
			}
		})
	}
}
