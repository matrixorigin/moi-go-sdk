package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatalogAllowedActionsContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/newmoi/catalog/allowed-actions" || r.Header.Get(headerWorkspaceID) != "ws-1" {
			t.Errorf("request = %s / %s", r.URL.Path, r.Header.Get(headerWorkspaceID))
		}
		var body struct {
			ResourceType string   `json:"resource_type"`
			ResourceIDs  []string `json:"resource_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.ResourceType != "catalog" || len(body.ResourceIDs) != 1 || body.ResourceIDs[0] != "10" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": map[string]any{"items": []any{map[string]any{"resource_id": "10", "allowed_actions": []string{}}}}})
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Catalog("ws-1").AllowedActions(t.Context(), "catalog", []string{"10"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].ResourceId != "10" || len(result.Items[0].AllowedActions) != 0 {
		t.Fatalf("result = %v", result)
	}
}
