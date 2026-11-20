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

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type accountHTTPContract struct {
	Name           string            `json:"name"`
	Operation      string            `json:"operation"`
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	Query          map[string]string `json:"query"`
	WorkspaceScope string            `json:"workspace_scope"`
	Input          struct {
		WorkspaceID   string          `json:"workspace_id"`
		ReturnTo      string          `json:"return_to"`
		State         string          `json:"state"`
		Source        string          `json:"source"`
		EventType     string          `json:"event_type"`
		Page          string          `json:"page"`
		PageSize      string          `json:"page_size"`
		BrowserLogin  json.RawMessage `json:"browser_login"`
		BrowserLogout json.RawMessage `json:"browser_logout"`
		Preferences   json.RawMessage `json:"preferences"`
	} `json:"input"`
	Request         json.RawMessage   `json:"request"`
	Response        json.RawMessage   `json:"response"`
	ResponseHeaders map[string]string `json:"response_headers"`
	Stream          bool              `json:"stream"`
	Status          int               `json:"status"`
}

func accountContractJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&value)
	return value
}

func accountContractEnvelopeData(raw json.RawMessage) any {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		panic(err)
	}
	return accountContractJSON(envelope.Data)
}

func TestAccountSharedHTTPContracts(t *testing.T) {
	data, err := os.ReadFile("testdata/account_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []accountHTTPContract
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			var received any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedQuery := url.Values{}
				for key, value := range scenario.Query {
					expectedQuery.Set(key, value)
				}
				if r.Method != scenario.Method || r.URL.EscapedPath() != scenario.Path || !reflect.DeepEqual(r.URL.Query(), expectedQuery) || r.Header.Get("X-API-Key") != "user-key" || r.Header.Get("X-Workspace-ID") != scenario.WorkspaceScope || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Errorf("request=%s %s query=%v headers=%v", r.Method, r.URL.RequestURI(), r.URL.Query(), r.Header)
				}
				if scenario.Stream && r.Header.Get("Accept") != "*/*" {
					t.Errorf("stream Accept=%q", r.Header.Get("Accept"))
				}
				raw, _ := io.ReadAll(r.Body)
				if len(raw) > 0 {
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					_ = decoder.Decode(&received)
				}
				for key, value := range scenario.ResponseHeaders {
					w.Header().Set(key, value)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				if !scenario.Stream {
					_, _ = w.Write(scenario.Response)
				}
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
			if err != nil {
				t.Fatal(err)
			}
			service := client.Account()
			var result proto.Message
			var rawResult *ProductData
			var stream *StreamResponse
			switch scenario.Operation {
			case "current-credit":
				result, err = service.CurrentCredit(t.Context())
			case "db-connection":
				result, err = service.DBConnection(t.Context(), scenario.Input.WorkspaceID)
			case "begin-browser-login":
				stream, err = service.BeginBrowserLogin(t.Context(), scenario.Input.ReturnTo)
			case "complete-browser-login":
				var request BrowserLoginCallbackRequest
				if err = json.Unmarshal(scenario.Input.BrowserLogin, &request); err == nil {
					stream, err = service.CompleteBrowserLogin(t.Context(), &request)
				}
			case "complete-browser-logout":
				stream, err = service.CompleteBrowserLogout(t.Context(), scenario.Input.State)
			case "browser-session":
				result, err = service.BrowserSession(t.Context())
			case "logout-browser-session":
				var request BrowserLogoutRequest
				if err = json.Unmarshal(scenario.Input.BrowserLogout, &request); err == nil {
					result, err = service.LogoutBrowserSession(t.Context(), &request)
				}
			case "preferences":
				result, err = service.Preferences(t.Context())
			case "update-preferences":
				var request AuthPreferencesRequest
				if err = json.Unmarshal(scenario.Input.Preferences, &request); err == nil {
					result, err = service.UpdatePreferences(t.Context(), &request)
				}
			case "checkin-status":
				rawResult, err = service.CheckinStatus(t.Context())
			case "registration-invite":
				rawResult, err = service.RegistrationInvite(t.Context())
			case "reward-center-items":
				rawResult, err = service.RewardCenterItems(t.Context())
			case "reward-center-events":
				rawResult, err = service.RewardCenterEvents(t.Context(), scenario.Input.EventType)
			case "reward-center-invitations":
				rawResult, err = service.RewardCenterInvitations(t.Context(), scenario.Input.Page, scenario.Input.PageSize)
			case "checkin":
				rawResult, err = service.Checkin(t.Context(), scenario.Input.Source)
			default:
				t.Fatalf("unknown operation %s", scenario.Operation)
			}
			if err != nil {
				t.Fatal(err)
			}
			if stream != nil {
				if stream.StatusCode != scenario.Status || stream.Header.Get("Location") != scenario.ResponseHeaders["Location"] {
					t.Fatalf("stream status=%d headers=%v", stream.StatusCode, stream.Header)
				}
				if err := stream.Body.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if result != nil {
				var envelope struct {
					Data json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal(scenario.Response, &envelope); err != nil {
					t.Fatal(err)
				}
				expected := result.ProtoReflect().New().Interface()
				if err := protojson.Unmarshal(envelope.Data, expected); err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(result, expected) {
					t.Fatalf("result=%v want=%v", result, expected)
				}
			}
			if rawResult != nil && !reflect.DeepEqual(rawResult.Value, accountContractEnvelopeData(scenario.Response)) {
				t.Fatalf("raw result=%#v want=%#v", rawResult.Value, accountContractEnvelopeData(scenario.Response))
			}
			if !reflect.DeepEqual(received, accountContractJSON(scenario.Request)) {
				t.Fatalf("body=%#v want=%#v", received, accountContractJSON(scenario.Request))
			}
		})
	}
}
