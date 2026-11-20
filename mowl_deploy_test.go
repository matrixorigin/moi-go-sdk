package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func mowlTestGraph() map[string]any {
	return map[string]any{
		"name": "probe", "root": "root",
		"nets": map[string]any{"root": map[string]any{"nodes": map[string]any{
			"__start": map[string]any{"id": "mowl:start"},
			"step":    map[string]any{"id": "moi:custom.operator:ws:probe", "version": "v1"},
			"__end":   map[string]any{"id": "mowl:end"},
		}}},
	}
}

func TestWorkflowMOWLDeployAndTemplate(t *testing.T) {
	var requests []struct {
		method string
		path   string
		body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		if r.Body != nil {
			defer r.Body.Close()
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		requests = append(requests, struct {
			method string
			path   string
			body   map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), body: body})
		data := any(nil)
		switch r.URL.EscapedPath() {
		case "/newmoi/workflow/v2/workflow-deployments":
			data = map[string]any{"workflow": map[string]any{"id": "wf-mowl", "name": "mowl-wf", "source_type": "manual_mowl", "status": "ready"}}
		case "/newmoi/workflow-templates":
			data = map[string]any{"id": 11, "workspace_id": "ws-1", "name": "mowl-tpl", "definition_format": "mowl", "is_builtin": false}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workflows := client.Workflows("ws-1")
	ctx := context.Background()

	envelope, err := workflows.DeployMOWL(ctx, "mowl-wf", mowlTestGraph(), WithWorkflowDescription("mowl deploy"))
	if err != nil {
		t.Fatal(err)
	}
	if envelope.GetWorkflow().GetId() != "wf-mowl" || envelope.GetWorkflow().GetSourceType() != "manual_mowl" {
		t.Fatalf("deployed workflow = %#v", envelope.GetWorkflow())
	}
	if _, err := workflows.DeployMOWL(ctx, "", mowlTestGraph()); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("empty name error = %v", err)
	}
	if _, err := workflows.DeployMOWL(ctx, "mowl-wf", nil); err == nil {
		t.Fatal("empty graph must fail")
	}
	if _, err := workflows.DeployMOWL(ctx, "mowl-wf", mowlTestGraph(), WithWorkflowSourceType("nl2dsl")); err == nil {
		t.Fatal("non-manual_mowl source_type must fail")
	}

	template, err := workflows.CreateMOWLWorkflowTemplate(ctx, "mowl-tpl", mowlTestGraph(), WithWorkflowTemplateDescription("mowl template"))
	if err != nil {
		t.Fatal(err)
	}
	if template.GetId() != 11 || template.GetDefinitionFormat() != "mowl" {
		t.Fatalf("template = %#v", template)
	}
	if _, err := workflows.CreateMOWLWorkflowTemplate(ctx, "", mowlTestGraph()); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("empty name error = %v", err)
	}
	if _, err := workflows.CreateMOWLWorkflowTemplate(ctx, "mowl-tpl", nil); err == nil {
		t.Fatal("empty graph must fail")
	}

	if len(requests) != 2 {
		t.Fatalf("request count = %d", len(requests))
	}
	deployRequest := requests[0]
	if deployRequest.method != http.MethodPost || deployRequest.path != "/newmoi/workflow/v2/workflow-deployments" ||
		deployRequest.body["name"] != "mowl-wf" ||
		deployRequest.body["source_type"] != "manual_mowl" ||
		deployRequest.body["description"] != "mowl deploy" {
		t.Fatalf("deploy request = %#v", deployRequest)
	}
	if _, hasDSL := deployRequest.body["dsl_yaml"]; hasDSL {
		t.Fatalf("MOWL deploy must not send dsl_yaml: %#v", deployRequest.body)
	}
	deployGraph, ok := deployRequest.body["workflow"].(map[string]any)
	if !ok || deployGraph["root"] != "root" {
		t.Fatalf("deploy workflow graph = %#v", deployRequest.body["workflow"])
	}
	templateRequest := requests[1]
	if templateRequest.method != http.MethodPost || templateRequest.path != "/newmoi/workflow-templates" ||
		templateRequest.body["name"] != "mowl-tpl" ||
		templateRequest.body["definition_format"] != "mowl" ||
		templateRequest.body["description"] != "mowl template" {
		t.Fatalf("template request = %#v", templateRequest)
	}
	if _, hasDSL := templateRequest.body["dsl_yaml"]; hasDSL {
		t.Fatalf("MOWL template must not send dsl_yaml: %#v", templateRequest.body)
	}
	templateGraph, ok := templateRequest.body["workflow"].(map[string]any)
	if !ok || templateGraph["root"] != "root" {
		t.Fatalf("template workflow graph = %#v", templateRequest.body["workflow"])
	}
}
