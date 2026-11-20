package sdk

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Product SDK uses strict protojson (DiscardUnknown=false). GET /models and
// GET /embedding/models share ProviderModel on the wire; every field the
// handler emits must be declared in the proto or selector List calls fail
// with an unknown field error (issue #19157).
func TestProviderModelListStrictContract(t *testing.T) {
	for _, fixture := range []string{
		"models_list_chat_with_reasoning.json",
		"models_list_workspace_plain.json",
		"models_list_embedding_subtype.json",
	} {
		t.Run(fixture, func(t *testing.T) {
			modelJSON := readProviderModelFixture(t, fixture)
			body := []byte(`{"code":"OK","msg":"OK","data":{"models":[` + string(modelJSON) + `]}}`)
			var out product.ProviderModelListResult
			if err := parseProductResponse(body, &out, ""); err != nil {
				t.Fatalf("strict decode: %v", err)
			}
			if len(out.GetModels()) != 1 {
				t.Fatalf("models len=%d", len(out.GetModels()))
			}
			assertProviderModelFixture(t, out.GetModels()[0], fixture)
		})
	}
}

func TestProviderModelListStrictContractRejectsUnknownField(t *testing.T) {
	raw := strings.TrimSpace(string(readProviderModelFixture(t, "models_list_chat_with_reasoning.json")))
	raw = strings.TrimSuffix(raw, "}") + `,"unregistered_field":true}`
	body := []byte(`{"code":"OK","msg":"OK","data":{"models":[` + raw + `]}}`)
	var out product.ProviderModelListResult
	err := parseProductResponse(body, &out, "")
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestProviderModelProtoFieldNumbers(t *testing.T) {
	expected := map[string]int{
		"model":             1,
		"backend_id":        2,
		"backend_name":      3,
		"model_type":        4,
		"source":            5,
		"system_default":    6,
		"dim":               7,
		"type":              8,
		"reasoning_efforts": 9,
	}
	fields := (&product.ProviderModel{}).ProtoReflect().Descriptor().Fields()
	for name, number := range expected {
		field := fields.ByName(protoreflect.Name(name))
		if field == nil || int(field.Number()) != number {
			t.Fatalf("field %s number = %v, want %d", name, field, number)
		}
	}
}

func readProviderModelFixture(t *testing.T, name string) json.RawMessage {
	t.Helper()
	value, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return value
}

func assertProviderModelFixture(t *testing.T, model *product.ProviderModel, fixture string) {
	t.Helper()
	if model.GetModel() == "" || model.GetBackendName() == "" {
		t.Fatalf("missing identity: %+v", model)
	}
	switch fixture {
	case "models_list_chat_with_reasoning.json":
		efforts := model.GetReasoningEfforts()
		if len(efforts) != 5 || efforts[0] != "none" || efforts[4] != "max" {
			t.Fatalf("reasoning_efforts=%v", efforts)
		}
		if model.GetSource() != "system" || !model.GetSystemDefault() {
			t.Fatalf("source/default: %+v", model)
		}
	case "models_list_workspace_plain.json":
		if len(model.GetReasoningEfforts()) != 0 {
			t.Fatalf("plain workspace model must omit efforts, got %v", model.GetReasoningEfforts())
		}
		if model.GetSource() != "workspace" || model.GetSystemDefault() {
			t.Fatalf("source/default: %+v", model)
		}
	case "models_list_embedding_subtype.json":
		if model.GetModelType() != "embedding" || model.GetType() != "embedding_multimodal" {
			t.Fatalf("embedding fields: %+v", model)
		}
		if model.GetDim() != 1024 {
			t.Fatalf("dim=%d", model.GetDim())
		}
	}
}
