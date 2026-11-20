package sdk

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

// GET /semantic-models -> SemanticModelListResult strict-decode contract.
func TestSemanticModelListStrictContract(t *testing.T) {
	for _, fixture := range []string{
		"semantic_model_list_knowledge.json",
		"semantic_model_list_empty.json",
	} {
		t.Run(fixture, func(t *testing.T) {
			itemJSON := readContractFixture(t, fixture)
			body := []byte(`{"code":"OK","msg":"OK","data":{"items":[` + string(itemJSON) + `],"total":1}}`)
			var out product.SemanticModelListResult
			if err := parseProductResponse(body, &out, ""); err != nil {
				t.Fatalf("strict decode: %v", err)
			}
			if len(out.GetItems()) != 1 || out.GetTotal() != 1 {
				t.Fatalf("items=%d total=%d", len(out.GetItems()), out.GetTotal())
			}
			assertSemanticModelFixture(t, out.GetItems()[0], fixture)
		})
	}
}

func TestSemanticModelListStrictContractRejectsUnknownField(t *testing.T) {
	body := []byte(`{"code":"OK","msg":"OK","data":{"items":[` +
		injectUnknownContractField(t, "semantic_model_list_knowledge.json") + `],"total":1}}`)
	var out product.SemanticModelListResult
	err := parseProductResponse(body, &out, "")
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestSemanticModelProtoFieldNumbers(t *testing.T) {
	assertProductMessageFieldNumbers(t, &product.SemanticModelListResult{}, map[string]int{
		"items":           1,
		"total":           2,
		"next_page_token": 3,
	})
	assertProductMessageFieldNumbers(t, &product.SemanticModelInfo{}, map[string]int{
		"id":                         1,
		"name":                       2,
		"description":                3,
		"tables":                     4,
		"files":                      5,
		"created_at":                 6,
		"updated_at":                 7,
		"source_counts":              8,
		"can_write":                  9,
		"allowed_actions":            10,
		"semantic_model_kind":        11,
		"drawing_structured_enabled": 12,
		"drawing_number_policy":      13,
		"can_append_sources":         14,
	})
	assertProductMessageFieldNumbers(t, &product.SemanticModelSourceCounts{}, map[string]int{
		"files":  1,
		"tables": 2,
		"total":  3,
	})
}

func assertSemanticModelFixture(t *testing.T, model *product.SemanticModelInfo, fixture string) {
	t.Helper()
	if model.GetId() == 0 || model.GetName() == "" || model.GetSourceCounts() == nil {
		t.Fatalf("missing identity: %+v", model)
	}
	switch fixture {
	case "semantic_model_list_knowledge.json":
		if !slices.Equal(model.GetAllowedActions(), []string{"semantic_model.delete", "semantic_model.update", "semantic_model.use"}) {
			t.Fatalf("allowed_actions=%v", model.GetAllowedActions())
		}
		if model.GetSemanticModelKind() != "drawing" || !model.GetDrawingStructuredEnabled() || model.GetDrawingNumberPolicy() != "keep_versions" || model.GetCanAppendSources() {
			t.Fatalf("drawing configuration lost: %v", model)
		}
		if !model.GetCanWrite() {
			t.Fatalf("can_write should be true: %+v", model)
		}
		if got := model.GetTables().GetListValue().GetValues(); len(got) != 2 {
			t.Fatalf("tables=%v", model.GetTables())
		}
		if got := model.GetFiles().GetListValue().GetValues(); len(got) != 1 {
			t.Fatalf("files=%v", model.GetFiles())
		}
		if model.GetSourceCounts().GetTotal() != 3 {
			t.Fatalf("source_counts=%+v", model.GetSourceCounts())
		}
	case "semantic_model_list_empty.json":
		if len(model.GetAllowedActions()) != 0 {
			t.Fatalf("read-only model allowed_actions=%v", model.GetAllowedActions())
		}
		if model.GetCanWrite() {
			t.Fatalf("can_write should be false: %+v", model)
		}
		if model.GetDescription() != "" {
			t.Fatalf("description must stay empty, got %q", model.GetDescription())
		}
		if model.GetSourceCounts().GetTotal() != 0 {
			t.Fatalf("source_counts=%+v", model.GetSourceCounts())
		}
	}
}

func TestKnowledgeDrawingModelHTTPContract(t *testing.T) {
	fixture := readContractFixture(t, "semantic_model_list_knowledge.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/newmoi/semantic-models" {
			_, _ = w.Write([]byte(`{"code":"OK","msg":"OK","data":{"items":[` + string(fixture) + `],"total":1}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":"OK","msg":"OK","data":` + string(fixture) + `}`))
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("test-key"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	knowledge := client.Knowledge("ws-1")
	list, err := knowledge.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(list.GetItems()) != 1 {
		t.Fatalf("list=%v", list)
	}
	assertSemanticModelFixture(t, list.GetItems()[0], "semantic_model_list_knowledge.json")
	created, err := knowledge.Create(t.Context(), "drawing-kb")
	if err != nil {
		t.Fatal(err)
	}
	assertSemanticModelFixture(t, created, "semantic_model_list_knowledge.json")
	got, err := knowledge.Get(t.Context(), "7001")
	if err != nil {
		t.Fatal(err)
	}
	assertSemanticModelFixture(t, got, "semantic_model_list_knowledge.json")
}
