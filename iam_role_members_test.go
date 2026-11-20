package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIAMRoleMembersPreservesUserDisplayAndPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/newmoi/workspaces/ws-1/iam/roles/role-1/members" || request.URL.Query().Get("cursor") != "user-0" || request.URL.Query().Get("limit") != "1" {
			t.Errorf("unexpected role members request: %s %s", request.Method, request.URL)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":"OK","data":{"role_id":"role-1","members":[{"user_id":"user-1","display_name":"Alice Chen","email":"alice@example.com","default_role":true,"default_role_version":4,"binding_created_at":1700000000}],"next_cursor":"user-1","request_id":"members-request","trace_id":"members-trace"}}`))
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	page, err := client.IAM("ws-1").RoleMembers(t.Context(), "role-1", WithQueryParam("cursor", "user-0"), WithQueryParam("limit", "1"))
	if err != nil {
		t.Fatal(err)
	}
	if page.GetRoleId() != "role-1" || page.GetNextCursor() != "user-1" || page.GetRequestId() != "members-request" || page.GetTraceId() != "members-trace" {
		t.Fatalf("role members page lost metadata: %v", page)
	}
	member := page.GetMembers()[0]
	if member.GetUserId() != "user-1" || member.GetDisplayName() != "Alice Chen" || member.GetEmail() != "alice@example.com" || !member.GetDefaultRole() || member.GetDefaultRoleVersion() != 4 || member.GetBindingCreatedAt() != 1700000000 {
		t.Fatalf("role member lost display or binding fields: %v", member)
	}
}
