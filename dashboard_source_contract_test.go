package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDashboardGetPreservesSourceAvailabilityAndAffectedCharts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/newmoi/data-dashboards/dashboard-1" || r.Header.Get(headerWorkspaceID) != "ws-1" {
			t.Errorf("request = %s %s, workspace = %q", r.Method, r.URL.Path, r.Header.Get(headerWorkspaceID))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"OK","msg":"OK","data":{"ID":"dashboard-1","sources":[{"ID":"source-1","DashboardID":"dashboard-1","TableID":42,"DatabaseName":"sales","TableName":"orders","Status":"invalid","AffectedCharts":[{"ID":"chart-1","Name":"Revenue"}]}]}}`))
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	dashboard, err := client.Dashboard("ws-1").Get(t.Context(), "dashboard-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(dashboard.Sources) != 1 {
		t.Fatalf("sources = %v", dashboard.Sources)
	}
	source := dashboard.Sources[0]
	if source.Status != "invalid" || len(source.AffectedCharts) != 1 || source.AffectedCharts[0].Id != "chart-1" || source.AffectedCharts[0].Name != "Revenue" {
		t.Fatalf("source = %v", source)
	}
}
