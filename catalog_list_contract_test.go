package sdk

import (
	"strings"
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

// POST /catalog/list -> CatalogListResult strict-decode contract.
func TestCatalogListStrictContract(t *testing.T) {
	for _, fixture := range []string{
		"catalog_list_full.json",
		"catalog_list_system_reserved.json",
	} {
		t.Run(fixture, func(t *testing.T) {
			itemJSON := readContractFixture(t, fixture)
			body := []byte(`{"code":"OK","msg":"OK","data":{"list":[` + string(itemJSON) + `]}}`)
			var out product.CatalogListResult
			if err := parseProductResponse(body, &out, ""); err != nil {
				t.Fatalf("strict decode: %v", err)
			}
			if len(out.GetList()) != 1 {
				t.Fatalf("catalogs len=%d", len(out.GetList()))
			}
			assertCatalogFixture(t, out.GetList()[0], fixture)
		})
	}
}

func TestCatalogListStrictContractRejectsUnknownField(t *testing.T) {
	body := []byte(`{"code":"OK","msg":"OK","data":{"list":[` +
		injectUnknownContractField(t, "catalog_list_full.json") + `]}}`)
	var out product.CatalogListResult
	err := parseProductResponse(body, &out, "")
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestCatalogProtoFieldNumbers(t *testing.T) {
	assertProductMessageFieldNumbers(t, &product.CatalogListResult{}, map[string]int{
		"list":            1,
		"next_page_token": 2,
	})
	assertProductMessageFieldNumbers(t, &product.CatalogDTO{}, map[string]int{
		"id":                  1,
		"name":                2,
		"description":         3,
		"database_count":      4,
		"volume_count":        5,
		"file_count":          6,
		"table_count":         7,
		"created_at":          8,
		"created_by":          9,
		"updated_at":          10,
		"updated_by":          11,
		"reserved":            12,
		"display_name":        14,
		"display_description": 15,
		"allowed_actions":     16,
	})
}

func assertCatalogFixture(t *testing.T, catalog *product.CatalogDTO, fixture string) {
	t.Helper()
	if catalog.GetId() == 0 || catalog.GetName() == "" {
		t.Fatalf("missing identity: %+v", catalog)
	}
	switch fixture {
	case "catalog_list_full.json":
		if catalog.GetReserved() {
			t.Fatalf("tenant catalog must not be reserved: %+v", catalog)
		}
		if catalog.GetDisplayName() == "" || catalog.GetDisplayDescription() == "" {
			t.Fatalf("display projection missing: %+v", catalog)
		}
		if catalog.GetTableCount() != 56 || len(catalog.GetAllowedActions()) != 2 {
			t.Fatalf("counts/actions: %+v", catalog)
		}
		if catalog.GetCreatedBy() == "" || catalog.GetUpdatedBy() == "" {
			t.Fatalf("audit projection missing: %+v", catalog)
		}
	case "catalog_list_system_reserved.json":
		if !catalog.GetReserved() {
			t.Fatalf("system catalog must be reserved: %+v", catalog)
		}
		if catalog.GetDisplayName() != "" || catalog.GetVolumeCount() != 0 {
			t.Fatalf("omitted fields must stay zero: %+v", catalog)
		}
		if len(catalog.GetAllowedActions()) != 1 {
			t.Fatalf("allowed_actions=%v", catalog.GetAllowedActions())
		}
	}
}
