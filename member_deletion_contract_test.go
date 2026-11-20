package sdk

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/matrixorigin/moi-go-sdk/v5/product"
)

func TestWorkspaceMemberListPreservesIncompleteDeletionDiagnostics(t *testing.T) {
	raw, err := os.ReadFile("testdata/workspace_member_deletion_failed.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture map[string]any
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	want := &product.WorkspaceMember{}
	if err := protojson.Unmarshal(raw, want); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"failed", "pending", "enabled"} {
		t.Run(state, func(t *testing.T) {
			row := maps.Clone(fixture)
			expected := proto.Clone(want).(*product.WorkspaceMember)
			switch state {
			case "pending":
				expected.Deletion.State = "pending"
				expected.Deletion.Phase = ""
				expected.Deletion.Code = "WORKSPACE_MEMBER_DELETE_PENDING"
				expected.Deletion.Message = "Deletion has not finished. Continue deletion to check the result."
				row["deletion"] = expected.Deletion
			case "enabled":
				expected.Status = "enabled"
				expected.Deletion = nil
				row["status"] = "enabled"
				delete(row, "deletion")
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/newmoi/user/list" || r.Header.Get("X-Workspace-ID") != "workspace-1" {
					t.Errorf("member request = %s %s workspace=%s", r.Method, r.URL.Path, r.Header.Get("X-Workspace-ID"))
				}
				if err := json.NewEncoder(w).Encode(map[string]any{
					"code": "OK", "msg": "OK", "data": map[string]any{"total": 1, "user_list": []any{row}},
				}); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("admin-key"))
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.WorkspaceMembers("workspace-1").Members(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if result.GetTotal() != 1 || len(result.GetUserList()) != 1 || !proto.Equal(result.GetUserList()[0], expected) {
				t.Fatalf("members = %v, want member %v", result, expected)
			}
		})
	}
}
