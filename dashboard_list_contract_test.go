package sdk

import (
	"strings"
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

// POST /data-dashboards/list -> DataDashboardListResult strict-decode contract.
// The dashboard family uses PascalCase json names on the wire.
func TestDashboardListStrictContract(t *testing.T) {
	for _, fixture := range []string{
		"dashboard_list_full.json",
		"dashboard_list_bare.json",
	} {
		t.Run(fixture, func(t *testing.T) {
			itemJSON := readContractFixture(t, fixture)
			body := []byte(`{"code":"OK","msg":"OK","data":{"items":[` + string(itemJSON) + `],"total":1,"has_more":false}}`)
			var out product.DataDashboardListResult
			if err := parseProductResponse(body, &out, ""); err != nil {
				t.Fatalf("strict decode: %v", err)
			}
			if len(out.GetItems()) != 1 || out.GetTotal() != 1 {
				t.Fatalf("items=%d total=%d", len(out.GetItems()), out.GetTotal())
			}
			assertDashboardFixture(t, out.GetItems()[0], fixture)
		})
	}
}

func TestDashboardListStrictContractRejectsUnknownField(t *testing.T) {
	body := []byte(`{"code":"OK","msg":"OK","data":{"items":[` +
		injectUnknownContractField(t, "dashboard_list_bare.json") + `],"total":1,"has_more":false}}`)
	var out product.DataDashboardListResult
	err := parseProductResponse(body, &out, "")
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestDashboardProtoFieldNumbers(t *testing.T) {
	assertProductMessageFieldNumbers(t, &product.DataDashboardListResult{}, map[string]int{
		"items":           1,
		"total":           2,
		"has_more":        3,
		"next_page_token": 4,
	})
	assertProductMessageFieldNumbers(t, &product.DataDashboard{}, map[string]int{
		"id":                 1,
		"name":               2,
		"description":        3,
		"created_by_role_id": 4,
		"created_at":         5,
		"updated_at":         6,
		"sources":            7,
		"charts":             8,
		"alerts":             9,
		"active_alert_count": 10,
		"lifecycle_status":   11,
	})
	assertProductMessageFieldNumbers(t, &product.DataDashboardSource{}, map[string]int{
		"id":              1,
		"dashboard_id":    2,
		"table_id":        3,
		"database_name":   4,
		"table_name":      5,
		"status":          6,
		"affected_charts": 7,
	})
}

func assertDashboardFixture(t *testing.T, dashboard *product.DataDashboard, fixture string) {
	t.Helper()
	if dashboard.GetId() == "" || dashboard.GetName() == "" || dashboard.GetCreatedAt() == "" {
		t.Fatalf("missing identity: %+v", dashboard)
	}
	switch fixture {
	case "dashboard_list_full.json":
		if len(dashboard.GetSources()) != 1 || len(dashboard.GetCharts()) != 1 || len(dashboard.GetAlerts()) != 1 {
			t.Fatalf("associations: %+v", dashboard)
		}
		source := dashboard.GetSources()[0]
		if source.GetTableName() != "fact_sales" || source.GetStatus() != "active" {
			t.Fatalf("source: %+v", source)
		}
		if len(source.GetAffectedCharts()) != 1 || source.GetAffectedCharts()[0].GetName() != "GMV Trend" {
			t.Fatalf("affected charts: %+v", source.GetAffectedCharts())
		}
		if dashboard.GetActiveAlertCount() != 1 || !dashboard.GetCharts()[0].GetAlertEnabled() {
			t.Fatalf("alert projection: %+v", dashboard)
		}
	case "dashboard_list_bare.json":
		if dashboard.GetLifecycleStatus() != "deleting" {
			t.Fatalf("deletion status=%q", dashboard.GetLifecycleStatus())
		}
		if len(dashboard.GetSources()) != 0 || len(dashboard.GetCharts()) != 0 || len(dashboard.GetAlerts()) != 0 {
			t.Fatalf("bare board must omit associations: %+v", dashboard)
		}
		if dashboard.GetActiveAlertCount() != 0 {
			t.Fatalf("active_alert_count=%d", dashboard.GetActiveAlertCount())
		}
	}
}
