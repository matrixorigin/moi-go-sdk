package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 验证公开 SDK 完整传递 IAM 观测、审计和默认角色合同。
func TestIAMInsightsAndDefaultRole(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(headerWorkspaceID) != "ws/1" || r.Header.Get(headerMOIRoleID) != "7" {
			t.Errorf("request scope: %v", r.Header)
		}
		var data any
		switch r.Method + " " + r.URL.EscapedPath() {
		case "GET /newmoi/workspaces/ws%2F1/iam/roles/role%2F7/ownership-impact":
			if r.URL.Query().Get("resource_type") != "table" {
				t.Errorf("ownership filter: %s", r.URL.RawQuery)
			}
			data = map[string]any{"workspace_id": "ws/1", "role_id": "role/7", "owned_count": 1, "by_type": []any{map[string]any{"resource_type": "table", "count": 1}}, "items": []any{map[string]any{"resource_id": "table-1", "ownership_version": 9007199254740993}}, "ownership_version": "set-v3", "can_delete": false, "transfer_supported": false, "request_id": "ownership-request"}
		case "GET /newmoi/workspaces/ws%2F1/iam/roles/role%2F7/audit-events":
			if r.URL.Query().Get("cursor") != "next/1" || r.URL.Query().Get("operation") != "role.update" {
				t.Errorf("audit filters: %s", r.URL.RawQuery)
			}
			data = map[string]any{"events": []any{map[string]any{"audit_event_id": "event-1", "actor_display_name": "Alice", "diff": map[string]any{"name": map[string]any{"before": "old", "after": "new"}}, "policy_version": 3, "status": "success", "created_at": 1760000000000}}, "next_cursor": "later", "request_id": "role-audit-request", "trace_id": "role-trace"}
		case "GET /newmoi/workspaces/ws%2F1/iam/users/user%2F1/audit-events":
			data = map[string]any{"events": []any{map[string]any{"audit_event_id": "member-event", "actor_display_name": "Alice", "summary": []any{map[string]any{"key": "role", "value": "7"}}, "lifecycle_version": "member-v2"}}, "next_cursor": "member-next"}
		case "PUT /newmoi/workspaces/ws%2F1/iam/me/default-role":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["target_role_id"] != "8" || body["expected_default_role_version"] != float64(0) || body["request_id"] != "set-role-1" {
				t.Errorf("default role body: %#v", body)
			}
			data = map[string]any{"default_role_id": "8", "default_role_version": 1, "audit_event_id": "default-event", "idempotent_replay": true}
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.Error(w, "unexpected route", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": data})
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("test-key"), WithWorkspaceCurrentRoleID("ws/1", "7"))
	if err != nil {
		t.Fatal(err)
	}
	iam := client.IAM("ws/1")
	impact, err := iam.OwnershipImpact(t.Context(), "role/7", WithQueryParam("resource_type", "table"))
	if err != nil || impact.GetOwnershipVersion() != "set-v3" || impact.GetItems()[0].GetOwnershipVersion() != 9007199254740993 || impact.GetCanDelete() {
		t.Fatalf("ownership impact: %v, %v", impact, err)
	}
	roleAudit, err := iam.RoleAuditEvents(t.Context(), "role/7", WithQueryParam("cursor", "next/1"), WithQueryParam("operation", "role.update"))
	if err != nil || roleAudit.GetNextCursor() != "later" || roleAudit.GetEvents()[0].GetDiff().AsMap()["name"].(map[string]any)["after"] != "new" || roleAudit.GetTraceId() != "role-trace" {
		t.Fatalf("role audit: %v, %v", roleAudit, err)
	}
	memberAudit, err := iam.UserAuditEvents(t.Context(), "user/1")
	if err != nil || memberAudit.GetNextCursor() != "member-next" || memberAudit.GetEvents()[0].GetSummary()[0].GetValue() != "7" {
		t.Fatalf("member audit: %v, %v", memberAudit, err)
	}
	changed, err := iam.SetDefaultRole(t.Context(), &IAMSetDefaultRoleRequest{TargetRoleId: "8", ExpectedDefaultRoleVersion: 0, RequestId: "set-role-1"})
	if err != nil || changed.GetDefaultRoleId() != "8" || !changed.GetIdempotentReplay() || changed.GetAuditEventId() != "default-event" {
		t.Fatalf("default role: %v, %v", changed, err)
	}
}
