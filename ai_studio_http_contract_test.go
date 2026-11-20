package sdk

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"
)

type aiStudioHTTPContract struct {
	Name      string              `json:"name"`
	Operation string              `json:"operation"`
	Method    string              `json:"method"`
	Path      string              `json:"path"`
	Query     map[string][]string `json:"query"`
	Input     json.RawMessage     `json:"input"`
	Request   json.RawMessage     `json:"request"`
	Response  json.RawMessage     `json:"response"`
	Status    int                 `json:"status"`
	SlotID    string              `json:"slot_id"`
}

func aiStudioContractJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&value)
	return value
}

func TestAIStudioSharedHTTPContracts(t *testing.T) {
	data, err := os.ReadFile("testdata/ai_studio_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []aiStudioHTTPContract
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			var received any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedQuery := url.Values{}
				for key, values := range scenario.Query {
					for _, value := range values {
						expectedQuery.Add(key, value)
					}
				}
				if r.Method != scenario.Method || r.URL.EscapedPath() != scenario.Path || !reflect.DeepEqual(r.URL.Query(), expectedQuery) || r.Header.Get("Authorization") != "Bearer admin-token" || r.Header.Get("X-API-Key") != "" || r.Header.Get("X-Workspace-ID") != "" {
					t.Errorf("request=%s %s query=%v headers=%v", r.Method, r.URL.RequestURI(), r.URL.Query(), r.Header)
				}
				raw, _ := io.ReadAll(r.Body)
				if len(raw) > 0 {
					received = aiStudioContractJSON(raw)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				_, _ = w.Write(scenario.Response)
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", WithToken("admin-token"))
			if err != nil {
				t.Fatal(err)
			}
			service := client.AIStudio()
			var result *ProductData
			switch scenario.Operation {
			case "chat-model-policy":
				result, err = service.ChatModelPolicy(t.Context())
			case "service-slots":
				result, err = service.ServiceSlots(t.Context())
			case "service-slot-candidates":
				result, err = service.ServiceSlotCandidates(t.Context())
			case "service-slot-logs":
				result, err = service.ServiceSlotLogs(t.Context(), WithServiceSlotLogSlotID(""), WithServiceSlotLogPageSize(0))
			case "put-service-slot":
				var request PutAIStudioServiceSlotRequest
				if err = json.Unmarshal(scenario.Input, &request); err == nil {
					result, err = service.PutServiceSlot(t.Context(), scenario.SlotID, &request)
				}
			default:
				t.Fatalf("unknown operation %q", scenario.Operation)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(received, aiStudioContractJSON(scenario.Request)) {
				t.Fatalf("body=%#v want=%#v", received, aiStudioContractJSON(scenario.Request))
			}
			var envelope struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(scenario.Response, &envelope); err != nil {
				t.Fatal(err)
			}
			if result == nil || !reflect.DeepEqual(result.Value, aiStudioContractJSON(envelope.Data)) {
				t.Fatalf("result=%#v", result)
			}
		})
	}
}
