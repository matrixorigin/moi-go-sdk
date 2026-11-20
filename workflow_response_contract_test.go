package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestWorkflowDetailAcceptsPublishedInvocationSchemas(t *testing.T) {
	for _, published := range []bool{false, true} {
		name := "schemas_absent"
		trigger := map[string]any{"mode": "api", "service_name": "published-api"}
		if published {
			name = "schemas_present"
			trigger["input_schema"] = `{"type":"object","properties":{"query":{"type":"string"}}}`
			trigger["output_schema"] = `{"type":"object","properties":{"answer":{"type":"string"}}}`
		}
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{
				"code": "OK", "msg": "OK",
				"data": map[string]any{"workflow": map[string]any{"id": "workflow-1", "trigger_summary": trigger}},
			})
			if err != nil {
				t.Fatal(err)
			}
			var envelope product.WorkflowEnvelope
			if err := parseProductResponse(raw, &envelope, ""); err != nil {
				t.Fatalf("parseProductResponse: %v", err)
			}
			got := envelope.GetWorkflow().GetTriggerSummary()
			if got.GetServiceName() != "published-api" {
				t.Fatalf("service_name=%q", got.GetServiceName())
			}
			input, _ := trigger["input_schema"].(string)
			output, _ := trigger["output_schema"].(string)
			if got.GetInputSchema() != input || got.GetOutputSchema() != output {
				t.Fatalf("invocation schemas changed: input=%q output=%q", got.GetInputSchema(), got.GetOutputSchema())
			}
		})
	}
}

func TestWorkflowDetailPreservesNotificationAndDesignGraph(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/newmoi/workflow/v2/workflow-apps/workflow-1" {
			t.Errorf("unexpected workflow request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"OK","data":{"workflow":{"id":"workflow-1","notification_config":{"type":"http","url":"https://callback.example/hook","notify_workflow_states":["COMPLETED","FAILED"]},"design_graph":{"nodes":[{"id":"start"}]}}}}`))
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := client.Workflows("workspace-1").Get(t.Context(), "workflow-1")
	if err != nil {
		t.Fatalf("read workflow detail: %v", err)
	}
	encoded, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(envelope.GetWorkflow())
	if err != nil {
		t.Fatal(err)
	}
	var detail map[string]any
	if err := json.Unmarshal(encoded, &detail); err != nil {
		t.Fatal(err)
	}
	wantNotification := map[string]any{"type": "http", "url": "https://callback.example/hook", "notify_workflow_states": []any{"COMPLETED", "FAILED"}}
	if got := detail["notification_config"]; !reflect.DeepEqual(got, wantNotification) {
		t.Fatalf("notification configuration = %#v, want %#v", got, wantNotification)
	}
	wantGraph := map[string]any{"nodes": []any{map[string]any{"id": "start"}}}
	if got := detail["design_graph"]; !reflect.DeepEqual(got, wantGraph) {
		t.Fatalf("design graph = %#v, want %#v", got, wantGraph)
	}
}

func TestWorkflowDetailAcceptsLatestVersionFields(t *testing.T) {
	raw := []byte(`{
		"code": "OK",
		"msg": "OK",
		"data": {
			"workflow": {
				"id": "workflow-1",
				"latest_version": 7,
				"latest_version_status": "published"
			}
		}
	}`)

	var envelope product.WorkflowEnvelope
	if err := parseProductResponse(raw, &envelope, ""); err != nil {
		t.Fatalf("parseProductResponse: %v", err)
	}
	if envelope.GetWorkflow().GetLatestVersion() != 7 {
		t.Fatalf("latest_version=%d", envelope.GetWorkflow().GetLatestVersion())
	}
	if envelope.GetWorkflow().GetLatestVersionStatus() != "published" {
		t.Fatalf("latest_version_status=%q", envelope.GetWorkflow().GetLatestVersionStatus())
	}
}

func TestWorkflowDetailAcceptsUIResource(t *testing.T) {
	raw := []byte(`{
		"code": "OK",
		"msg": "OK",
		"data": {
			"workflow": {
				"id": "api_x",
				"ui_resource": {
					"resource_uri": "ui://api-service/api_x/4",
					"file_id": "file-html",
					"mime_type": "text/html"
				}
			}
		}
	}`)
	var envelope product.WorkflowEnvelope
	if err := parseProductResponse(raw, &envelope, ""); err != nil {
		t.Fatalf("parseProductResponse: %v", err)
	}
	got := envelope.GetWorkflow().GetUiResource()
	if got.GetResourceUri() != "ui://api-service/api_x/4" || got.GetFileId() != "file-html" || got.GetMimeType() != "text/html" {
		t.Fatalf("ui_resource=%v", got)
	}
}

func TestWorkflowDetailAcceptsDynamicServiceSchemas(t *testing.T) {
	workflow := readContractFixture(t, "workflow_list_dynamic_service_ready.json")
	body := []byte(`{"code":"OK","msg":"OK","data":{"workflow":` + string(workflow) + `}}`)
	var envelope product.WorkflowEnvelope
	if err := parseProductResponse(body, &envelope, ""); err != nil {
		t.Fatalf("parse dynamic service detail: %v", err)
	}
	assertDynamicServiceTriggerSchemas(t, envelope.GetWorkflow().GetTriggerSummary())
}
