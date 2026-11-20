package sdk

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestKnowledgeCreateOptionsContract(t *testing.T) {
	var cases []struct {
		Name     string          `json:"name"`
		Request  json.RawMessage `json:"request"`
		Status   int             `json:"status"`
		Response struct {
			Code    string          `json:"code"`
			Message string          `json:"msg"`
			Data    json.RawMessage `json:"data"`
		} `json:"response"`
	}
	value, err := os.ReadFile("testdata/knowledge_create_options_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(value, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			var input struct {
				Name            string `json:"name"`
				TargetCatalogID *int64 `json:"target_catalog_id"`
				Sources         []struct {
					SourceType string `json:"source_type"`
					FileID     string `json:"file_id"`
				} `json:"sources"`
				Selections *[]struct {
					Kind        string `json:"kind"`
					VolumeID    int64  `json:"volume_id"`
					AllSelected bool   `json:"all_selected"`
				} `json:"source_selections"`
			}
			if err := json.Unmarshal(scenario.Request, &input); err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/newmoi/semantic-models/create-with-sources" || r.Header.Get("X-Workspace-ID") != "workspace-1" {
					t.Errorf("unexpected target: %s %s", r.Method, r.URL.Path)
				}
				var got, want map[string]any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Error(err)
				}
				if err := json.Unmarshal(scenario.Request, &want); err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("create body=%#v, want=%#v", got, want)
				}
				w.WriteHeader(scenario.Status)
				_ = json.NewEncoder(w).Encode(scenario.Response)
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			var opts []SemanticModelWithSourcesOption
			if input.TargetCatalogID != nil {
				opts = append(opts, WithSemanticModelWithSourcesTargetCatalogID(*input.TargetCatalogID))
			}
			if input.Selections != nil {
				selections := make([]SemanticModelSourceSelectionInput, 0, len(*input.Selections))
				for _, selection := range *input.Selections {
					selections = append(selections, SemanticModelSourceSelectionInput{Kind: selection.Kind, VolumeID: selection.VolumeID, AllSelected: selection.AllSelected})
				}
				opts = append(opts, WithSemanticModelWithSourcesSourceSelections(selections...))
			}
			sources := make([]SemanticModelSourceInput, 0, len(input.Sources))
			for _, source := range input.Sources {
				sources = append(sources, SemanticModelSourceInput{SourceType: source.SourceType, FileID: source.FileID})
			}
			result, err := client.Knowledge("workspace-1").CreateWithSources(t.Context(), input.Name, sources, opts...)
			if scenario.Status >= 400 {
				var ownerError *Error
				if !errors.As(err, &ownerError) || ownerError.Code != scenario.Response.Code || ownerError.Message != scenario.Response.Message || ownerError.Status != scenario.Status {
					t.Fatalf("owner error changed: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			} else if result.GetModel().GetId() != 101 || result.GetDataDomain().GetCatalogId() != 17 {
				t.Fatalf("create result=%v", result)
			}
			if calls.Load() != 1 {
				t.Errorf("create requests=%d, want one", calls.Load())
			}
		})
	}
}
