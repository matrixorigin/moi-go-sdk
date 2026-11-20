package sdk

import (
	"strings"
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

// GET /connectors/list -> ConnectorListResult strict-decode contract.
func TestConnectorListStrictContract(t *testing.T) {
	for _, fixture := range []string{
		"connector_list_mysql.json",
		"connector_list_s3_export.json",
	} {
		t.Run(fixture, func(t *testing.T) {
			itemJSON := readContractFixture(t, fixture)
			body := []byte(`{"code":"OK","msg":"OK","data":{"connectors":[` + string(itemJSON) + `],"total":1}}`)
			var out product.ConnectorListResult
			if err := parseProductResponse(body, &out, ""); err != nil {
				t.Fatalf("strict decode: %v", err)
			}
			if len(out.GetConnectors()) != 1 || out.GetTotal() != 1 {
				t.Fatalf("connectors=%d total=%d", len(out.GetConnectors()), out.GetTotal())
			}
			assertConnectorFixture(t, out.GetConnectors()[0], fixture)
		})
	}
}

func TestConnectorListStrictContractRejectsUnknownField(t *testing.T) {
	body := []byte(`{"code":"OK","msg":"OK","data":{"connectors":[` +
		injectUnknownContractField(t, "connector_list_mysql.json") + `],"total":1}}`)
	var out product.ConnectorListResult
	err := parseProductResponse(body, &out, "")
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestConnectorProtoFieldNumbers(t *testing.T) {
	assertProductMessageFieldNumbers(t, &product.ConnectorListResult{}, map[string]int{
		"connectors": 1,
		"total":      2,
	})
	assertProductMessageFieldNumbers(t, &product.Connector{}, map[string]int{
		"id":               1,
		"name":             2,
		"source_type":      3,
		"created_at":       4,
		"updated_at":       5,
		"status":           6,
		"username":         7,
		"related_task_ids": 8,
		"usage_type":       9,
		"config":           10,
	})
}

func assertConnectorFixture(t *testing.T, connector *product.Connector, fixture string) {
	t.Helper()
	if connector.GetId() == "" || connector.GetName() == "" {
		t.Fatalf("missing identity: %+v", connector)
	}
	switch fixture {
	case "connector_list_mysql.json":
		if connector.GetSourceType() != 9 || connector.GetStatus() != "active" {
			t.Fatalf("source_type=%d status=%q", connector.GetSourceType(), connector.GetStatus())
		}
		if len(connector.GetUsageType()) != 2 || len(connector.GetRelatedTaskIds()) != 2 {
			t.Fatalf("usage/tasks: %+v", connector)
		}
		if connector.GetConfig().GetFields()["host"].GetStringValue() != "mysql.internal" {
			t.Fatalf("config host: %v", connector.GetConfig())
		}
	case "connector_list_s3_export.json":
		if connector.GetSourceType() != 5 || connector.GetStatus() != "failed" {
			t.Fatalf("source_type=%d status=%q", connector.GetSourceType(), connector.GetStatus())
		}
		if len(connector.GetUsageType()) != 1 || connector.GetUsageType()[0] != 2 {
			t.Fatalf("usage_type=%v", connector.GetUsageType())
		}
		if len(connector.GetRelatedTaskIds()) != 0 {
			t.Fatalf("related_task_ids=%v", connector.GetRelatedTaskIds())
		}
	}
}
