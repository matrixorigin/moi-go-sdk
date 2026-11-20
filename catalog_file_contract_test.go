package sdk

import (
	"strings"
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

// POST /catalog/file/list -> CatalogFileListResult strict-decode contract.
func TestCatalogFileListStrictContract(t *testing.T) {
	for _, fixture := range []string{
		"catalog_file_document.json",
		"catalog_file_folder.json",
	} {
		t.Run(fixture, func(t *testing.T) {
			itemJSON := readContractFixture(t, fixture)
			body := []byte(`{"code":"OK","msg":"OK","data":{"total":1,"list":[` + string(itemJSON) + `]}}`)
			var out product.CatalogFileListResult
			if err := parseProductResponse(body, &out, ""); err != nil {
				t.Fatalf("strict decode: %v", err)
			}
			if len(out.GetList()) != 1 || out.GetTotal() != 1 {
				t.Fatalf("files=%d total=%d", len(out.GetList()), out.GetTotal())
			}
			assertCatalogFileFixture(t, out.GetList()[0], fixture)
		})
	}
}

func TestCatalogFileListStrictContractRejectsUnknownField(t *testing.T) {
	body := []byte(`{"code":"OK","msg":"OK","data":{"total":1,"list":[` +
		injectUnknownContractField(t, "catalog_file_document.json") + `]}}`)
	var out product.CatalogFileListResult
	err := parseProductResponse(body, &out, "")
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestCatalogFileProtoFieldNumbers(t *testing.T) {
	assertProductMessageFieldNumbers(t, &product.CatalogFileListResult{}, map[string]int{
		"total": 1,
		"list":  2,
	})
	assertProductMessageFieldNumbers(t, &product.CatalogFileFolder{}, map[string]int{
		"id":               1,
		"name":             2,
		"file_type":        3,
		"workflow_role":    4,
		"file_ext":         5,
		"origin_file_name": 6,
		"origin_file_ext":  7,
		"ref_file_id":      8,
		"parsed_file_id":   9,
		"size":             10,
		"parent_id":        11,
		"created_at":       12,
		"created_by":       13,
		"volume_id":        14,
		"volume_name":      15,
		"volume_reserved":  16,
		"ref_workflow_id":  17,
		"file_path":        18,
	})
}

func assertCatalogFileFixture(t *testing.T, file *product.CatalogFileFolder, fixture string) {
	t.Helper()
	if file.GetId() == "" || file.GetName() == "" || file.GetCreatedAt() == "" {
		t.Fatalf("missing identity: %+v", file)
	}
	switch fixture {
	case "catalog_file_document.json":
		if file.GetFileType() != "file" || file.GetWorkflowRole() != "input" {
			t.Fatalf("type=%q role=%q", file.GetFileType(), file.GetWorkflowRole())
		}
		if file.GetSize() != 5242880 || file.GetParsedFileId() != "file-6002" {
			t.Fatalf("parse projection: %+v", file)
		}
		if file.GetVolumeReserved() {
			t.Fatalf("tenant volume must not be reserved: %+v", file)
		}
	case "catalog_file_folder.json":
		if file.GetFileType() != "folder" || file.GetSize() != 0 {
			t.Fatalf("type=%q size=%d", file.GetFileType(), file.GetSize())
		}
		if !file.GetVolumeReserved() {
			t.Fatalf("system folder volume must be reserved: %+v", file)
		}
		if file.GetWorkflowRole() != "" || file.GetRefWorkflowId() != "" {
			t.Fatalf("folder must not carry workflow projection: %+v", file)
		}
	}
}
