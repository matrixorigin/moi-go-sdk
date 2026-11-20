package sdk

import (
	"strings"
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

// GET /workspaces -> WorkspaceListResult strict-decode contract.
func TestWorkspaceListStrictContract(t *testing.T) {
	for _, fixture := range []string{
		"workspace_list_owner.json",
		"workspace_list_pending_invite.json",
	} {
		t.Run(fixture, func(t *testing.T) {
			itemJSON := readContractFixture(t, fixture)
			body := []byte(`{"code":"OK","msg":"OK","data":{"workspaces":[` + string(itemJSON) + `],"total":1}}`)
			var out product.WorkspaceListResult
			if err := parseProductResponse(body, &out, ""); err != nil {
				t.Fatalf("strict decode: %v", err)
			}
			if len(out.GetWorkspaces()) != 1 || out.GetTotal() != 1 {
				t.Fatalf("workspaces=%d total=%d", len(out.GetWorkspaces()), out.GetTotal())
			}
			assertWorkspaceFixture(t, out.GetWorkspaces()[0], fixture)
		})
	}
}

func TestWorkspaceListStrictContractRejectsUnknownField(t *testing.T) {
	body := []byte(`{"code":"OK","msg":"OK","data":{"workspaces":[` +
		injectUnknownContractField(t, "workspace_list_owner.json") + `],"total":1}}`)
	var out product.WorkspaceListResult
	err := parseProductResponse(body, &out, "")
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestWorkspaceProtoFieldNumbers(t *testing.T) {
	assertProductMessageFieldNumbers(t, &product.WorkspaceListResult{}, map[string]int{
		"workspaces": 1,
		"total":      2,
	})
	assertProductMessageFieldNumbers(t, &product.Workspace{}, map[string]int{
		"id":            1,
		"name":          2,
		"status":        3,
		"owner_id":      4,
		"created_at":    5,
		"access_status": 6,
		"invitation_id": 7,
		"owner_name":    8,
		"owner_email":   9,
	})
}

func assertWorkspaceFixture(t *testing.T, workspace *product.Workspace, fixture string) {
	t.Helper()
	if workspace.GetId() == "" || workspace.GetName() == "" || workspace.GetCreatedAt() == "" {
		t.Fatalf("missing identity: %+v", workspace)
	}
	switch fixture {
	case "workspace_list_owner.json":
		if workspace.GetStatus() != "ACTIVE" || workspace.GetAccessStatus() != "accepted" {
			t.Fatalf("status=%q access=%q", workspace.GetStatus(), workspace.GetAccessStatus())
		}
		if workspace.GetOwnerName() == "" || workspace.GetOwnerEmail() == "" {
			t.Fatalf("owner projection missing: %+v", workspace)
		}
		if workspace.GetInvitationId() != "" {
			t.Fatalf("owned workspace must not carry invitation_id, got %q", workspace.GetInvitationId())
		}
	case "workspace_list_pending_invite.json":
		if workspace.GetStatus() != "PENDING_INVITATION" || workspace.GetAccessStatus() != "pending" {
			t.Fatalf("status=%q access=%q", workspace.GetStatus(), workspace.GetAccessStatus())
		}
		if workspace.GetInvitationId() == "" {
			t.Fatalf("pending workspace must carry invitation_id: %+v", workspace)
		}
		if workspace.GetOwnerName() != "" || workspace.GetOwnerEmail() != "" {
			t.Fatalf("pending invite must omit owner projection: %+v", workspace)
		}
	}
}
