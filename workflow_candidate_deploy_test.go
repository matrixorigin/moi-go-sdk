package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/types/known/structpb"
)

func acceptedWorkflowCandidateForTest(t *testing.T, ref WorkflowCandidateRef) *AcceptedWorkflowCandidate {
	t.Helper()
	accepted, err := AcceptedWorkflowCandidateFromAcceptance(&WorkflowCandidateAcceptance{
		CandidateRef: &product.WorkflowCandidateRef{TaskId: ref.TaskID, ArtifactId: ref.ArtifactID},
		AcceptedAt:   "2026-08-31T00:00:00Z",
		Candidate: &WorkflowCandidateBundle{
			Ok:          true,
			CandidateId: ref.ArtifactID,
			Workflow: structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
				"name": structpb.NewStringValue("accepted"),
			}}).GetStructValue(),
			EnvelopeDigest: "sha256:accepted",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return accepted
}

func TestWorkflowCandidateAcceptThenTypedDeploy(t *testing.T) {
	ref := WorkflowCandidateRef{TaskID: "task-123", ArtifactID: "workflow_candidate_0123456789abcdef0123456789abcdef"}
	var requests []struct {
		method string
		path   string
		body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, struct {
			method string
			path   string
			body   map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), body: body})

		var data any
		switch r.URL.EscapedPath() {
		case "/newmoi/workflow/v2/workflow-candidates/accept":
			data = map[string]any{
				"candidate_ref": map[string]any{"task_id": ref.TaskID, "artifact_id": ref.ArtifactID},
				"accepted_at":   "2026-08-31T00:00:00Z",
				"candidate": map[string]any{
					"ok": true, "candidate_id": ref.ArtifactID,
					"workflow": map[string]any{"name": "accepted"}, "envelope_digest": "sha256:accepted",
				},
			}
		case "/newmoi/workflow/v2/workflow-deployments":
			data = map[string]any{"workflow": map[string]any{"id": "workflow-1", "source_type": "nl2dsl"}}
		default:
			t.Fatalf("unexpected path %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	service := client.Workflows("workspace-1")
	accepted, err := service.AcceptCandidate(context.Background(), ref, &WorkflowCandidateConfirmation{ProposalId: "proposal-1", IdempotencyKey: "confirm-1"})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Ref() != ref {
		t.Fatalf("accepted ref = %#v, want %#v", accepted.Ref(), ref)
	}
	if _, err := service.DeployCandidate(context.Background(), "accepted workflow", accepted, WithWorkflowID("workflow-1")); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 2 {
		t.Fatalf("request count = %d, want accept then deploy", len(requests))
	}
	if requests[0].method != http.MethodPost || requests[0].path != "/newmoi/workflow/v2/workflow-candidates/accept" {
		t.Fatalf("accept request = %#v", requests[0])
	}
	if !reflect.DeepEqual(requests[0].body, map[string]any{"candidate_ref": map[string]any{"task_id": ref.TaskID, "artifact_id": ref.ArtifactID}, "proposal_id": "proposal-1", "idempotency_key": "confirm-1"}) {
		t.Fatalf("accept body = %#v", requests[0].body)
	}
	if requests[1].path != "/newmoi/workflow/v2/workflow-deployments" {
		t.Fatalf("deploy request = %#v", requests[1])
	}
	for _, forbidden := range []string{"dsl_yaml", "runtime_fields", "runtime_layout", "envelope_digest", "source_code"} {
		if _, ok := requests[1].body[forbidden]; ok {
			t.Fatalf("typed deploy leaked %q: %#v", forbidden, requests[1].body)
		}
	}
}

func TestWorkflowCandidateDeployRejectsPendingOrClientAuthoredFacts(t *testing.T) {
	ref := WorkflowCandidateRef{TaskID: "task-123", ArtifactID: "workflow_candidate_0123456789abcdef0123456789abcdef"}
	accepted := acceptedWorkflowCandidateForTest(t, ref)
	if _, err := workflowCandidateDeployBody("candidate", nil); err == nil {
		t.Fatal("pending candidate was accepted")
	}
	if _, err := workflowCandidateDeployBody("candidate", accepted, WithWorkflowInputForm(map[string]any{"fields": []any{}})); err == nil {
		t.Fatal("candidate deploy accepted client-authored runtime form")
	}
	if _, err := workflowDeployBody("manual", "workflow:\n  name: manual\n", WithWorkflowSourceType("nl2dsl")); err == nil {
		t.Fatal("manual deploy accepted nl2dsl source type")
	}
}

func TestAgentTextMessageParamsIncludesModel(t *testing.T) {
	params, err := (AgentTextMessage{MessageID: "msg-1", Text: "create a workflow", Model: "deepseek-v4-flash"}).Params()
	if err != nil {
		t.Fatal(err)
	}
	if params["model"] != "deepseek-v4-flash" {
		t.Fatalf("model = %#v", params["model"])
	}
}

func TestWorkflowCandidateHistoryRestoreAndRejectTypedRoutes(t *testing.T) {
	var requests []struct {
		method string
		path   string
		body   map[string]any
		query  string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Method != http.MethodGet {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method string
			path   string
			body   map[string]any
			query  string
		}{r.Method, r.URL.EscapedPath(), body, r.URL.RawQuery})
		var data any
		switch r.URL.EscapedPath() {
		case "/newmoi/workflow/v2/workflow-candidates/history":
			data = map[string]any{"items": []any{}, "total_tasks": 0, "limit": 20, "offset": 0}
		case "/newmoi/workflow/v2/workflow-candidates/restore":
			data = map[string]any{"id": "restore-1", "candidate_ref": map[string]any{"task_id": "task-1", "artifact_id": "candidate-1"}, "candidate": map[string]any{"ok": true}, "status": "pending", "created_at": "2026-08-31T00:00:00Z"}
		case "/newmoi/workflow/v2/workflow-candidates/reject-proposal":
			data = map[string]any{"proposal_id": "proposal-1", "status": "rejected"}
		default:
			t.Fatalf("unexpected path %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	service := client.Workflows("workspace-1")
	page, err := service.History(context.Background(), "context-1", 20, 0)
	if err != nil || page.GetLimit() != 20 || page.GetOffset() != 0 {
		t.Fatalf("history = %#v err=%v", page, err)
	}
	entry, err := service.RestoreHistory(context.Background(), WorkflowHistoryRestoreRequest{ContextID: "context-1", RequestID: "restore-1", CandidateRef: WorkflowCandidateRef{TaskID: "task-1", ArtifactID: "candidate-1"}})
	if err != nil || entry.GetId() != "restore-1" {
		t.Fatalf("restore = %#v err=%v", entry, err)
	}
	rejected, err := service.RejectCandidateProposal(context.Background(), "proposal-1")
	if err != nil || rejected.GetStatus() != "rejected" {
		t.Fatalf("reject = %#v err=%v", rejected, err)
	}
	if len(requests) != 3 || requests[0].query != "context_id=context-1&limit=20&offset=0" ||
		!reflect.DeepEqual(requests[1].body, map[string]any{"context_id": "context-1", "request_id": "restore-1", "candidate_ref": map[string]any{"task_id": "task-1", "artifact_id": "candidate-1"}}) ||
		!reflect.DeepEqual(requests[2].body, map[string]any{"proposal_id": "proposal-1"}) {
		t.Fatalf("candidate route requests = %#v", requests)
	}
}
