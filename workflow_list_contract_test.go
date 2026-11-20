package sdk

import (
	"strings"
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/proto"
)

// GET /workflow/v2/workflow-apps -> WorkflowListEnvelope strict-decode contract.
func TestWorkflowListStrictContract(t *testing.T) {
	for _, fixture := range []string{
		"workflow_list_manual_ready.json",
		"workflow_list_cron_scheduled.json",
		"workflow_list_dynamic_service.json",
		"workflow_list_dynamic_service_ready.json",
	} {
		t.Run(fixture, func(t *testing.T) {
			itemJSON := readContractFixture(t, fixture)
			body := []byte(`{"code":"OK","msg":"OK","data":{"total":1,"workflows":[` + string(itemJSON) + `]}}`)
			var out product.WorkflowListEnvelope
			if err := parseProductResponse(body, &out, ""); err != nil {
				t.Fatalf("strict decode: %v", err)
			}
			if len(out.GetWorkflows()) != 1 || out.GetTotal() != 1 {
				t.Fatalf("workflows=%d total=%d", len(out.GetWorkflows()), out.GetTotal())
			}
			assertWorkflowFixture(t, out.GetWorkflows()[0], fixture)
		})
	}
}

func TestWorkflowListStrictContractRejectsUnknownField(t *testing.T) {
	body := []byte(`{"code":"OK","msg":"OK","data":{"total":1,"workflows":[` +
		injectUnknownContractField(t, "workflow_list_manual_ready.json") + `]}}`)
	var out product.WorkflowListEnvelope
	err := parseProductResponse(body, &out, "")
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestWorkflowResponsePermissionsStayOutsideStrictBusinessData(t *testing.T) {
	item := readContractFixture(t, "workflow_list_manual_ready.json")
	data := `{"total":37,"workflows":[` + string(item) + `]}`
	var baseline, projected product.WorkflowListEnvelope
	if err := parseProductResponse([]byte(`{"code":"OK","data":`+data+`}`), &baseline, ""); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"code":"OK","permissions":{"workspace_id":"workspace","effective_role_id":"role","resources":[{"resource_type":"workflow","resource_id":"id","allowed_actions":["workflow.run"]}]},"data":` + data + `}`)
	if err := parseProductResponse(body, &projected, ""); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(&baseline, &projected) {
		t.Fatal("response metadata changed business data or pagination")
	}
	// The additive envelope must not weaken strict decoding inside business data.
	bad := []byte(`{"code":"OK","data":{"total":37,"workflows":[{"id":"workflow","allowed_actions":["workflow.run"]}]}}`)
	if err := parseProductResponse(bad, &projected, ""); err == nil {
		t.Fatal("business allowed_actions must remain an unknown field")
	}
}

func TestWorkflowListProtoFieldNumbers(t *testing.T) {
	assertProductMessageFieldNumbers(t, &product.WorkflowListEnvelope{}, map[string]int{
		"total":     1,
		"workflows": 2,
	})
	assertProductMessageFieldNumbers(t, &product.WorkflowListItem{}, map[string]int{
		"id":                         1,
		"name":                       2,
		"description":                3,
		"source_type":                4,
		"status":                     5,
		"available_actions":          6,
		"execution_mode":             7,
		"cron_expression":            8,
		"draft_id":                   9,
		"candidate_id":               10,
		"latest_workflow_version_id": 11,
		"latest_version":             12,
		"latest_version_status":      13,
		"compute_resource_id":        14,
		"compute_resource_bindings":  15,
		"parameter_summary":          16,
		"execution_summary":          17,
		"trigger_summary":            18,
		"created_at":                 19,
		"updated_at":                 20,
	})
}

func assertWorkflowFixture(t *testing.T, workflow *product.WorkflowListItem, fixture string) {
	t.Helper()
	if workflow.GetId() == "" || workflow.GetName() == "" {
		t.Fatalf("missing identity: %+v", workflow)
	}
	if workflow.GetCreatedAt() == nil || workflow.GetUpdatedAt() == nil {
		t.Fatalf("timestamps missing: %+v", workflow)
	}
	switch fixture {
	case "workflow_list_manual_ready.json":
		if workflow.GetStatus() != "ready" || workflow.GetExecutionMode() != "one_shot" {
			t.Fatalf("status=%q mode=%q", workflow.GetStatus(), workflow.GetExecutionMode())
		}
		if workflow.GetLatestVersion() != 21 || workflow.GetLatestVersionStatus() != "published" {
			t.Fatalf("version projection: %+v", workflow)
		}
		if workflow.GetParameterSummary().GetStatus() != "complete" {
			t.Fatalf("parameter_summary: %+v", workflow.GetParameterSummary())
		}
		if len(workflow.GetAvailableActions()) != 3 {
			t.Fatalf("available_actions=%v", workflow.GetAvailableActions())
		}
		if workflow.GetTriggerSummary().GetConfigured() {
			t.Fatalf("manual workflow must not configure trigger: %+v", workflow.GetTriggerSummary())
		}
	case "workflow_list_cron_scheduled.json":
		if workflow.GetExecutionMode() != "cron" || workflow.GetCronExpression() != "0 * * * *" {
			t.Fatalf("mode=%q cron=%q", workflow.GetExecutionMode(), workflow.GetCronExpression())
		}
		if !workflow.GetTriggerSummary().GetConfigured() || workflow.GetTriggerSummary().GetEnabled() {
			t.Fatalf("trigger_summary: %+v", workflow.GetTriggerSummary())
		}
		if got := workflow.GetParameterSummary(); got.GetMissingRequiredFields() != 1 || len(got.GetMissingRequiredFieldIds()) != 1 {
			t.Fatalf("parameter_summary: %+v", got)
		}
		if got := workflow.GetExecutionSummary(); got.GetActiveExecutions() != 1 || got.GetActiveExecutionId() == "" {
			t.Fatalf("execution_summary: %+v", got)
		}
		if len(workflow.GetComputeResourceBindings()) != 1 || !workflow.GetComputeResourceBindings()[0].GetWorkflowLevel() {
			t.Fatalf("bindings: %+v", workflow.GetComputeResourceBindings())
		}
	case "workflow_list_dynamic_service.json":
		if workflow.GetExecutionMode() != "dynamic_service" {
			t.Fatalf("mode=%q", workflow.GetExecutionMode())
		}
		trigger := workflow.GetTriggerSummary()
		if !trigger.GetConfigured() || !trigger.GetEnabled() || trigger.GetServiceName() != "invoice_lookup" {
			t.Fatalf("trigger_summary: %+v", trigger)
		}
		if trigger.GetInputSchema() != `{"type":"object","required":["invoice_id"]}` {
			t.Fatalf("input_schema=%q", trigger.GetInputSchema())
		}
		if trigger.GetOutputSchema() != `{"type":"object","required":["invoice_total"]}` {
			t.Fatalf("output_schema=%q", trigger.GetOutputSchema())
		}
	case "workflow_list_dynamic_service_ready.json":
		if workflow.GetExecutionMode() != "dynamic_service" {
			t.Fatalf("mode=%q", workflow.GetExecutionMode())
		}
		assertDynamicServiceTriggerSchemas(t, workflow.GetTriggerSummary())
	}
}

func assertDynamicServiceTriggerSchemas(t *testing.T, summary *product.WorkflowTriggerSummary) {
	t.Helper()
	if !summary.GetConfigured() || !summary.GetEnabled() || summary.GetServiceName() != "published-service" {
		t.Fatalf("dynamic service trigger: %+v", summary)
	}
	if summary.GetInputSchema() != `{"type":"object","properties":{"text":{"type":"string"}}}` ||
		summary.GetOutputSchema() != `{"type":"object","properties":{"answer":{"type":"string"}}}` {
		t.Fatalf("published invocation schemas were not preserved: %+v", summary)
	}
}
