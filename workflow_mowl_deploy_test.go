package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestWorkflowDeployAuthoredMOWL(t *testing.T) {
	definition := json.RawMessage(`{"name":"authored","root":"root","nets":{"root":{"nodes":{}}}}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		var workflow map[string]any
		if err := json.Unmarshal(definition, &workflow); err != nil {
			t.Error(err)
		}
		want := map[string]any{"name": "authored", "source_type": "manual_mowl", "workflow": workflow,
			"execution_mode": "dynamic_service", "dynamic_service": map[string]any{"service_name": "authored-api", "result_mode": "oneshot"}}
		if r.URL.Path != "/newmoi/workflow/v2/workflow-deployments" || r.Header.Get("X-Workspace-ID") != "workspace" || !reflect.DeepEqual(got, want) {
			t.Errorf("deployment changed: %s %+v", r.URL.Path, got)
		}
		_, _ = w.Write([]byte(`{"code":"OK","data":{"workflow":{"id":"saved","source_type":"manual_mowl"}}}`))
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Workflows("workspace").Deploy(t.Context(), "authored", "", WithWorkflowMOWL(definition),
		WithWorkflowExecutionMode("dynamic_service"), WithWorkflowDynamicService(WorkflowDynamicServiceParam{ServiceName: "authored-api", ResultMode: "oneshot"}))
	if err != nil || result.GetWorkflow().GetId() != "saved" || result.GetWorkflow().GetSourceType() != "manual_mowl" {
		t.Fatalf("deployed result: %v, %v", result, err)
	}
}

func TestWorkflowDeployRejectsAmbiguousOrInvalidAuthoredInputBeforeMutation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid authored workflow reached publication owner")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		dsl  string
		opts []WorkflowDeployOption
	}{
		{name: "missing source"},
		{name: "mixed DSL and MOWL", dsl: "workflow: {}", opts: []WorkflowDeployOption{WithWorkflowMOWL(json.RawMessage(`{}`))}},
		{name: "malformed JSON", opts: []WorkflowDeployOption{WithWorkflowMOWL(json.RawMessage(`{`))}},
		{name: "null JSON", opts: []WorkflowDeployOption{WithWorkflowMOWL(json.RawMessage(`null`))}},
		{name: "array JSON", opts: []WorkflowDeployOption{WithWorkflowMOWL(json.RawMessage(`[]`))}},
		{name: "empty JSON", opts: []WorkflowDeployOption{WithWorkflowMOWL(nil)}},
		{name: "candidate source", opts: []WorkflowDeployOption{WithWorkflowMOWL(json.RawMessage(`{}`)), WithWorkflowSourceType("nl2dsl")}},
		{name: "missing MOWL", opts: []WorkflowDeployOption{WithWorkflowSourceType("manual_mowl")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := client.Workflows("workspace").Deploy(context.Background(), "authored", test.dsl, test.opts...); err == nil {
				t.Fatal("invalid deployment accepted")
			}
		})
	}
	accepted := acceptedWorkflowCandidateForTest(t, WorkflowCandidateRef{TaskID: "task", ArtifactID: "candidate"})
	if _, err := client.Workflows("workspace").DeployCandidate(t.Context(), "candidate", accepted, WithWorkflowMOWL(json.RawMessage(`{}`))); err == nil {
		t.Fatal("client-authored workflow replaced accepted candidate")
	}
}
