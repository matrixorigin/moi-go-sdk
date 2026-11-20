package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkspaceWorkflowDeploymentIdentity(t *testing.T) {
	accepted := acceptedWorkflowCandidateForTest(t, WorkflowCandidateRef{TaskID: "task-123", ArtifactID: "workflow_candidate_0123456789abcdef0123456789abcdef"})
	for _, candidate := range []bool{false, true} {
		kind := "manual"
		if candidate {
			kind = "candidate"
		}
		for _, test := range []struct {
			name, suppliedID, responseID, code string
			status                             int
			opts                               []WorkflowDeployOption
		}{
			{name: "create", responseID: "server-created"},
			{name: "update", suppliedID: "existing", responseID: "existing"},
			{name: "option_selects_existing", suppliedID: "original", responseID: "selected", opts: []WorkflowDeployOption{WithWorkflowID("selected")}},
			{name: "missing_target", suppliedID: "missing", code: "ErrWorkflowDeploymentTargetNotFound", status: http.StatusNotFound},
			{name: "missing_response_id", suppliedID: "existing", code: "ErrResponseInvalid"},
		} {
			t.Run(kind+"/"+test.name, func(t *testing.T) {
				var requestBody map[string]any
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get(headerWorkspaceID) != "workspace-1" {
						t.Error("deployment lost workspace scope")
					}
					if r.Method == http.MethodGet && r.URL.Path == "/newmoi/workflow/v2/workflow-apps/"+test.responseID {
						_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": map[string]any{"workflow": map[string]any{"id": test.responseID}}})
						return
					}
					if r.Method != http.MethodPost || r.URL.Path != "/newmoi/workflow/v2/workflow-deployments" {
						t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					}
					if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
						t.Error(err)
					}
					if test.status != 0 {
						w.WriteHeader(test.status)
						_ = json.NewEncoder(w).Encode(map[string]any{"code": test.code, "msg": "Workflow does not exist", "details": map[string]string{"workflow_id": test.suppliedID}})
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": map[string]any{"workflow": map[string]any{"id": test.responseID}}})
				}))
				t.Cleanup(server.Close)
				client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = client.Close() })
				workspace, err := client.Workspace("workspace-1")
				if err != nil {
					t.Fatal(err)
				}
				deploy := func() (*WorkflowHandle, *WorkflowDeploymentEnvelope, error) {
					if candidate {
						return workspace.DeployWorkflowCandidate(t.Context(), "workflow", accepted, test.suppliedID, test.opts...)
					}
					return workspace.DeployWorkflow(t.Context(), "workflow", "workflow:\n  name: workflow", test.suppliedID, test.opts...)
				}
				handle, result, err := deploy()
				if test.code != "" {
					var apiErr *Error
					if !errors.As(err, &apiErr) || apiErr.Code != test.code || handle != nil {
						t.Fatalf("handle=%v error=%v, want %s without a handle", handle, err, test.code)
					}
					if test.status != 0 && (apiErr.Status != test.status || apiErr.Message != "Workflow does not exist" || apiErr.Details["workflow_id"] != test.suppliedID) {
						t.Fatalf("backend refusal changed: %+v", apiErr)
					}
					if test.code == "ErrResponseInvalid" && apiErr.Details["kind"] != "workflow" {
						t.Fatalf("invalid response lost the resource kind: %+v", apiErr)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					if handle.ID() != test.responseID || result.GetWorkflow().GetId() != handle.ID() {
						t.Fatalf("handle=%q response=%v, want %q", handle.ID(), result, test.responseID)
					}
					info, err := handle.Info(context.Background())
					if err != nil || info.GetWorkflow().GetId() != handle.ID() {
						t.Fatalf("returned handle cannot read its workflow: %v %v", info, err)
					}
				}
				if requestBody == nil {
					t.Fatal("deployment did not reach the product endpoint")
				}
				wantID := test.suppliedID
				if test.name == "option_selects_existing" {
					wantID = "selected"
				}
				id, supplied := requestBody["workflow_id"]
				if (wantID == "" && supplied) || (wantID != "" && id != wantID) {
					t.Fatalf("workflow_id=%v present=%t, want %q (omitted on create)", id, supplied, wantID)
				}
			})
		}
	}
}
