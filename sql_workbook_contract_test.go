package sdk

import (
	"strings"
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

// POST /workbook/list -> SQLWorkbookListResult strict-decode contract.
func TestSQLWorkbookListStrictContract(t *testing.T) {
	for _, fixture := range []string{
		"sql_workbook_list_report.json",
		"sql_workbook_list_adhoc.json",
	} {
		t.Run(fixture, func(t *testing.T) {
			itemJSON := readContractFixture(t, fixture)
			body := []byte(`{"code":"OK","msg":"OK","data":{"total":1,"result":[` + string(itemJSON) + `],"next_page_token":""}}`)
			var out product.SQLWorkbookListResult
			if err := parseProductResponse(body, &out, ""); err != nil {
				t.Fatalf("strict decode: %v", err)
			}
			if len(out.GetResult()) != 1 || out.GetTotal() != 1 {
				t.Fatalf("result=%d total=%d", len(out.GetResult()), out.GetTotal())
			}
			item := out.GetResult()[0]
			if item.GetWorkbookId() == "" || item.GetName() == "" {
				t.Fatalf("missing identity: %+v", item)
			}
		})
	}
}

func TestSQLWorkbookListStrictContractRejectsUnknownField(t *testing.T) {
	body := []byte(`{"code":"OK","msg":"OK","data":{"total":1,"result":[` +
		injectUnknownContractField(t, "sql_workbook_list_report.json") + `],"next_page_token":""}}`)
	var out product.SQLWorkbookListResult
	err := parseProductResponse(body, &out, "")
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestSQLWorkbookProtoFieldNumbers(t *testing.T) {
	assertProductMessageFieldNumbers(t, &product.SQLWorkbookListResult{}, map[string]int{
		"total":           1,
		"result":          2,
		"next_page_token": 3,
	})
	assertProductMessageFieldNumbers(t, &product.SQLWorkbook{}, map[string]int{
		"workbook_id": 1,
		"name":        2,
	})
}
