package sdk

import (
	"encoding/json"
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/encoding/protojson"
)

// Product SDK uses strict protojson (DiscardUnknown=false). Optional embedding
// subtype "type" must be on ProviderModel or /embedding/models responses fail.
func TestProviderModelListAcceptsOptionalEmbeddingSubtype(t *testing.T) {
	raw := []byte(`{
		"code": "OK",
		"msg": "OK",
		"data": {
			"models": [
				{
					"model": "efficientnet-b3",
					"backend_id": -900002,
					"backend_name": "TaaS Embedding",
					"model_type": "embedding",
					"type": "embedding_multimodal",
					"source": "system",
					"system_default": true
				},
				{
					"model": "legacy-embed",
					"backend_id": 12,
					"backend_name": "workspace-embedding",
					"model_type": "embedding",
					"source": "workspace",
					"system_default": false
				}
			]
		}
	}`)

	var list product.ProviderModelListResult
	if err := parseProductResponse(raw, &list, ""); err != nil {
		t.Fatalf("parseProductResponse: %v", err)
	}
	if len(list.Models) != 2 {
		t.Fatalf("models len=%d", len(list.Models))
	}
	if list.Models[0].GetType() != "embedding_multimodal" {
		t.Fatalf("multimodal type=%q", list.Models[0].GetType())
	}
	if list.Models[0].GetModelType() != "embedding" {
		t.Fatalf("model_type=%q", list.Models[0].GetModelType())
	}
	if list.Models[1].GetType() != "" {
		t.Fatalf("legacy type should be empty, got %q", list.Models[1].GetType())
	}

	// Round-trip strict decode without envelope.
	options := protojson.UnmarshalOptions{DiscardUnknown: false}
	var one product.ProviderModel
	item, _ := json.Marshal(map[string]any{
		"model": "bge-m3", "backend_id": -900002, "backend_name": "TaaS Embedding",
		"model_type": "embedding", "type": "embedding_text",
	})
	if err := options.Unmarshal(item, &one); err != nil {
		t.Fatalf("strict unmarshal with type: %v", err)
	}
	if one.GetType() != "embedding_text" {
		t.Fatalf("type=%q", one.GetType())
	}
}

func TestParserBackendListAcceptsEmptyBackendSuccess(t *testing.T) {
	raw := []byte(`{"code":"OK","msg":"OK","data":null}`)
	var backends parserBackendList
	if err := parseProductResponse(raw, &backends, ""); err != nil {
		t.Fatalf("parseProductResponse: %v", err)
	}
	if len(backends.backends) != 0 {
		t.Fatalf("parser backend count=%d, want 0", len(backends.backends))
	}
}

func TestParserEndpointListAcceptsEmptyEndpointSuccess(t *testing.T) {
	raw := []byte(`{"code":"OK","msg":"OK","data":null}`)
	var endpoints parserEndpointList
	if err := parseProductResponse(raw, &endpoints, ""); err != nil {
		t.Fatalf("parseProductResponse: %v", err)
	}
	if len(endpoints.endpoints) != 0 {
		t.Fatalf("parser endpoint count=%d, want 0", len(endpoints.endpoints))
	}
}
