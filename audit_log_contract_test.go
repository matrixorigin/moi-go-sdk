package sdk

import (
	"strings"
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

// GET /audit/logs -> AuditLogListResult strict-decode contract.
func TestAuditLogListStrictContract(t *testing.T) {
	for _, fixture := range []string{
		"audit_log_success.json",
		"audit_log_failed.json",
	} {
		t.Run(fixture, func(t *testing.T) {
			itemJSON := readContractFixture(t, fixture)
			body := []byte(`{"code":"OK","msg":"OK","data":{"items":[` + string(itemJSON) + `],"total":1,"page":1,"page_size":20}}`)
			var out product.AuditLogListResult
			if err := parseProductResponse(body, &out, ""); err != nil {
				t.Fatalf("strict decode: %v", err)
			}
			if len(out.GetItems()) != 1 || out.GetTotal() != 1 {
				t.Fatalf("items=%d total=%d", len(out.GetItems()), out.GetTotal())
			}
			assertAuditLogFixture(t, out.GetItems()[0], fixture)
		})
	}
}

func TestAuditLogListStrictContractRejectsUnknownField(t *testing.T) {
	body := []byte(`{"code":"OK","msg":"OK","data":{"items":[` +
		injectUnknownContractField(t, "audit_log_success.json") + `],"total":1,"page":1,"page_size":20}}`)
	var out product.AuditLogListResult
	err := parseProductResponse(body, &out, "")
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestAuditLogProtoFieldNumbers(t *testing.T) {
	assertProductMessageFieldNumbers(t, &product.AuditLogListResult{}, map[string]int{
		"items":     1,
		"total":     2,
		"page":      3,
		"page_size": 4,
	})
	assertProductMessageFieldNumbers(t, &product.AuditOperationLog{}, map[string]int{
		"id":                     1,
		"workspace_id":           2,
		"operator_user_id":       3,
		"operator_name_snapshot": 4,
		"method":                 5,
		"path":                   6,
		"route_pattern":          7,
		"query_json":             8,
		"request_body_json":      9,
		"response_body_json":     10,
		"status_code":            11,
		"result":                 12,
		"error_code":             13,
		"error_message":          14,
		"duration_ms":            15,
		"client_ip":              16,
		"user_agent":             17,
		"request_id":             18,
		"action_type":            19,
		"resource_type":          20,
		"resource_id":            21,
		"resource_name":          22,
		"operation_summary":      23,
		"metadata_json":          24,
		"created_at":             25,
	})
}

func assertAuditLogFixture(t *testing.T, entry *product.AuditOperationLog, fixture string) {
	t.Helper()
	if entry.GetId() == "" || entry.GetCreatedAt() == "" || entry.GetWorkspaceId() == "" {
		t.Fatalf("missing identity: %+v", entry)
	}
	switch fixture {
	case "audit_log_success.json":
		if entry.GetResult() != "success" || entry.GetStatusCode() != 200 {
			t.Fatalf("result=%q status=%d", entry.GetResult(), entry.GetStatusCode())
		}
		if entry.GetRequestId() == "" || entry.GetRoutePattern() == "" {
			t.Fatalf("trace projection missing: %+v", entry)
		}
		if entry.GetErrorCode() != "" {
			t.Fatalf("success entry must not carry error_code, got %q", entry.GetErrorCode())
		}
	case "audit_log_failed.json":
		if entry.GetResult() != "failed" || entry.GetStatusCode() != 403 {
			t.Fatalf("result=%q status=%d", entry.GetResult(), entry.GetStatusCode())
		}
		if entry.GetErrorCode() != "ErrAccessDenied" || entry.GetErrorMessage() == "" {
			t.Fatalf("error projection: %+v", entry)
		}
		if entry.GetRequestBodyJson() != "" || entry.GetQueryJson() != "" {
			t.Fatalf("omitted payload fields must stay empty: %+v", entry)
		}
	}
}
