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
	"sort"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
)

type agentAccountResourceContract struct {
	Name      string              `json:"name"`
	Operation string              `json:"operation"`
	Method    string              `json:"method"`
	Path      string              `json:"path"`
	Auth      string              `json:"auth"`
	Query     map[string][]string `json:"query"`
	Input     json.RawMessage     `json:"input"`
	Request   json.RawMessage     `json:"request"`
	Response  json.RawMessage     `json:"response"`
	Status    int                 `json:"status"`
	Resource  string              `json:"resource"`
	TailPath  string              `json:"tail_path"`
	TicketID  string              `json:"ticket_id"`
	Kind      string              `json:"kind"`
	Title     string              `json:"title"`
	BodyText  string              `json:"body_text"`
	Retry     bool                `json:"retry"`
}

func agentAccountResourceJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		panic(err)
	}
	return value
}

func agentAccountResourceData(raw json.RawMessage) any {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		panic(err)
	}
	return agentAccountResourceJSON(envelope.Data)
}

func agentAccountResourceQuery(values map[string][]string) []QueryOption {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var options []QueryOption
	for _, key := range keys {
		for _, value := range values[key] {
			options = append(options, WithQueryParam(key, value))
		}
	}
	return options
}

func TestAgentAccountResourceSharedHTTPContracts(t *testing.T) {
	data, err := os.ReadFile("testdata/agent_account_resource_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []agentAccountResourceContract
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
				if r.Method != scenario.Method || r.URL.EscapedPath() != scenario.Path || !reflect.DeepEqual(r.URL.Query(), expectedQuery) || r.Header.Get("X-Workspace-ID") != "workspace-1" || r.Header.Get("Cookie") != "" {
					t.Errorf("request=%s %s query=%v headers=%v", r.Method, r.URL.RequestURI(), r.URL.Query(), r.Header)
				}
				if scenario.Auth == "token" {
					if r.Header.Get("Authorization") != "Bearer admin-token" || r.Header.Get("X-API-Key") != "" {
						t.Errorf("token headers=%v", r.Header)
					}
				} else if r.Header.Get("X-API-Key") != "user-key" || r.Header.Get("Authorization") != "" {
					t.Errorf("API key headers=%v", r.Header)
				}
				raw, _ := io.ReadAll(r.Body)
				if len(raw) > 0 {
					received = agentAccountResourceJSON(raw)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				_, _ = w.Write(scenario.Response)
			}))
			defer server.Close()

			var client *Client
			if scenario.Auth == "token" {
				client, err = New(server.URL+"/newmoi", WithToken("admin-token"))
			} else {
				client, err = New(server.URL+"/newmoi", WithAPIKey("user-key"))
			}
			if err != nil {
				t.Fatal(err)
			}
			service := client.Agents("workspace-1")
			options := agentAccountResourceQuery(scenario.Query)
			var result *ProductData
			switch scenario.Operation {
			case "admin-feedback-tickets":
				result, err = service.AdminFeedbackTickets(t.Context(), options...)
			case "admin-feedback-ticket":
				result, err = service.AdminFeedbackTicket(t.Context(), scenario.TicketID)
			case "update-admin-feedback-ticket":
				request := &FeedbackTicketAdminUpdateRequest{}
				if err = protojson.Unmarshal(scenario.Input, request); err == nil {
					result, err = service.UpdateAdminFeedbackTicket(t.Context(), scenario.TicketID, request)
				}
			case "create-feedback-github-issue":
				result, err = service.CreateFeedbackGithubIssue(t.Context(), scenario.TicketID, scenario.Kind, scenario.Title, scenario.BodyText, scenario.Retry)
			case "build-feedback-github-issue-draft":
				result, err = service.BuildFeedbackGithubIssueDraft(t.Context(), scenario.TicketID, scenario.Kind)
			case "approve-feedback-reward":
				request := &FeedbackTicketAdminUpdateRequest{}
				if err = protojson.Unmarshal(scenario.Input, request); err == nil {
					result, err = service.ApproveFeedbackReward(t.Context(), scenario.TicketID, request)
				}
			case "create-feedback-ticket":
				request := &FeedbackTicketCreateRequest{}
				if err = protojson.Unmarshal(scenario.Input, request); err == nil {
					result, err = service.CreateFeedbackTicket(t.Context(), request)
				}
			case "my-feedback-tickets":
				result, err = service.MyFeedbackTickets(t.Context(), options...)
			case "resource-any":
				result, err = service.ResourceAny(t.Context(), scenario.Method, scenario.Resource, scenario.TailPath, agentAccountResourceJSON(scenario.Input), options...)
			default:
				t.Fatalf("unknown operation %q", scenario.Operation)
			}
			if err != nil {
				t.Fatal(err)
			}
			if result == nil || !reflect.DeepEqual(result.Value, agentAccountResourceData(scenario.Response)) {
				t.Fatalf("result=%#v want=%#v", result, agentAccountResourceData(scenario.Response))
			}
			if !reflect.DeepEqual(received, agentAccountResourceJSON(scenario.Request)) {
				t.Fatalf("body=%#v want=%#v", received, agentAccountResourceJSON(scenario.Request))
			}
		})
	}
}
