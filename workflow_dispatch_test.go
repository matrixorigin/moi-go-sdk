package sdk

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkflowDispatchReceipt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Workspace-ID") != "workspace-1" {
			t.Error("dispatch observation lost workspace")
		}
		switch r.Method + " " + r.URL.Path {
		case "POST /newmoi/workflow/v2/workflow-apps/workflow-1/executions":
			fmt.Fprint(w, `{"code":"OK","data":{"workflow_run":{"execution_id":"dispatch-1","dispatch_job_id":"dispatch-1","workflow_id":"workflow-1","status":"waiting","available_actions":[]}}}`)
		case "GET /newmoi/workflow/v2/workflow-apps/workflow-1/dispatch-jobs/dispatch-1":
			fmt.Fprint(w, `{"code":"OK","data":{"dispatch_job":{"dispatch_job_id":"dispatch-1","workflow_id":"workflow-1","status":"completed","total_deliveries":2,"pending_deliveries":0,"failed_deliveries":0}}}`)
		default:
			t.Errorf("unexpected dispatch request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("fixture-key"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	workflows := client.Workflows("workspace-1")
	run, err := workflows.Run(t.Context(), "workflow-1")
	if err != nil {
		t.Fatal(err)
	}
	if run.GetWorkflowRun().GetDispatchJobId() != "dispatch-1" {
		t.Fatalf("dispatch receipt = %v", run)
	}
	job, err := workflows.GetDispatchJob(t.Context(), "workflow-1", run.GetWorkflowRun().GetDispatchJobId())
	if err != nil {
		t.Fatal(err)
	}
	observed := job.GetDispatchJob()
	if observed.GetDispatchJobId() != "dispatch-1" || observed.GetWorkflowId() != "workflow-1" || observed.GetStatus() != "completed" || observed.GetTotalDeliveries() != 2 || observed.GetPendingDeliveries() != 0 || observed.GetFailedDeliveries() != 0 {
		t.Fatalf("dispatch observation = %v", observed)
	}
}
