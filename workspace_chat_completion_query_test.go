package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkspaceChatCompletionPreservesBillingSlotInQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/newmoi/workspaces/workspace/llm/chat/completions" || r.URL.Query().Get("service_slot") != "agent_chat" {
			t.Errorf("wrong chat completion route: %s %s", r.Method, r.URL)
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || string(body["model"]) != `"model"` || body["service_slot"] != nil {
			t.Errorf("query metadata leaked into provider body: %v, %v", body, err)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": "answer"}}}}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	result, err := client.Models("workspace").WorkspaceChatCompletion(t.Context(), &WorkspaceChatCompletionRequest{Model: "model"}, WithQueryParam("service_slot", "agent_chat"))
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result.AsMap()
	if !ok || value["choices"] == nil {
		t.Fatalf("provider response lost: %+v", result)
	}
}
