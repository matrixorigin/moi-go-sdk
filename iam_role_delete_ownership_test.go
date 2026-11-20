package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestIAMRoleDeleteOwnershipContract(t *testing.T) {
	for _, transfer := range []*IAMRoleDeleteOwnershipTransfer{
		{ExpectedOwnershipVersion: "ownership-v1", TargetRoleId: "role-target"},
		{ExpectedOwnershipVersion: "ownership-v1", Targets: []*IAMRoleDeleteOwnershipTarget{
			{ResourceType: "workflow", ResourceId: "workflow-1", TargetRoleId: "role-a"},
			{ResourceType: "table", ResourceId: "table-1", TargetRoleId: "role-b"},
		}},
	} {
		t.Run(transfer.GetTargetRoleId(), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/newmoi/workspaces/ws-1/iam/roles/source" && r.URL.Path != "/newmoi/workspaces/ws-1/iam/roles/source/delete-impact" {
					t.Fatalf("unexpected route: %s", r.URL.Path)
				}
				var data any
				switch r.Method {
				case http.MethodGet:
					if r.URL.Query().Get("expected_lifecycle_version") != "5" || r.URL.Query().Get("include_ownership") != "true" || r.URL.Query().Get("ownership_cursor") != "cursor-1" || r.URL.Query().Get("ownership_limit") != "1" || r.URL.Query().Get("transfer_requested") != "true" {
						t.Errorf("ownership preview query: %v", r.URL.Query())
					}
					data = map[string]any{"active_ownership_count": 2, "blocked": true, "ownership": map[string]any{
						"owned_count": 2, "ownership_version": "ownership-v1", "next_cursor": "cursor-2",
						"items": []any{map[string]any{"resource_type": "workflow", "resource_id": "workflow-1", "display_name": "Order cleaning", "ownership_version": 7}},
					}}
				case http.MethodDelete:
					var body struct {
						ReasonCode      string                          `json:"reason_code"`
						RequestID       string                          `json:"request_id"`
						ExpectedVersion int64                           `json:"expected_lifecycle_version"`
						Transfer        *IAMRoleDeleteOwnershipTransfer `json:"ownership_transfer"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if body.ReasonCode != "transfer-delete" || body.RequestID != "delete-1" || body.ExpectedVersion != 5 || !reflect.DeepEqual(body.Transfer, transfer) {
						t.Errorf("delete ownership plan changed: %+v", body)
					}
					data = map[string]any{"role": map[string]any{"role_id": "source", "lifecycle_state": "deleted"}}
				default:
					t.Fatalf("unexpected method: %s", r.Method)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": data})
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			iam := client.IAM("ws-1")
			preview, err := iam.RoleDeleteImpact(t.Context(), "source", 5,
				WithQueryParam("include_ownership", "true"), WithQueryParam("ownership_cursor", "cursor-1"),
				WithQueryParam("ownership_limit", "1"), WithQueryParam("transfer_requested", "true"))
			if err != nil {
				t.Fatal(err)
			}
			page := preview.GetOwnership()
			if page.GetOwnedCount() != 2 || page.GetOwnershipVersion() != "ownership-v1" || page.GetNextCursor() != "cursor-2" || len(page.GetItems()) != 1 {
				t.Fatalf("ownership page: %v", page)
			}
			item := page.GetItems()[0]
			if item.GetResourceType() != "workflow" || item.GetResourceId() != "workflow-1" || item.GetDisplayName() != "Order cleaning" || item.GetOwnershipVersion() != 7 {
				t.Fatalf("ownership object: %v", item)
			}
			deleted, err := iam.DeleteRole(t.Context(), "source", &IAMRoleDeleteRequest{ReasonCode: "transfer-delete", RequestId: "delete-1", ExpectedLifecycleVersion: 5, OwnershipTransfer: transfer})
			if err != nil || deleted.GetRole().GetLifecycleState() != "deleted" {
				t.Fatalf("delete=%v error=%v", deleted, err)
			}
		})
	}
}
