package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
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

type workspaceHTTPContract struct {
	Name      string            `json:"name"`
	Operation string            `json:"operation"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Query     map[string]string `json:"query"`
	Input     struct {
		Name        string `json:"name"`
		WorkspaceID string `json:"workspace_id"`
	} `json:"input"`
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response"`
	Status   int             `json:"status"`
}

func workspaceContractJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&value)
	return value
}

func TestWorkspaceSharedHTTPContracts(t *testing.T) {
	data, err := os.ReadFile("testdata/workspace_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []workspaceHTTPContract
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
				if r.Method != scenario.Method || r.URL.EscapedPath() != scenario.Path || !reflect.DeepEqual(r.URL.Query(), expectedQuery) || r.Header.Get("X-API-Key") != "user-key" || r.Header.Get("X-Workspace-ID") != "" {
					t.Errorf("request=%s %s query=%v headers=%v", r.Method, r.URL.RequestURI(), r.URL.Query(), r.Header)
				}
				raw, _ := io.ReadAll(r.Body)
				if len(raw) > 0 {
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					_ = decoder.Decode(&received)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				_, _ = w.Write(scenario.Response)
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
			if err != nil {
				t.Fatal(err)
			}
			service := client.Workspaces()
			var result proto.Message
			switch scenario.Operation {
			case "list":
				var options []ListOption
				if name, ok := scenario.Query["name"]; ok {
					options = append(options, WithWorkspaceNameFilter(name))
				}
				result, err = service.List(t.Context(), options...)
			case "create":
				result, err = service.Create(t.Context(), scenario.Input.Name)
			case "claim":
				result, err = service.Claim(t.Context(), scenario.Input.WorkspaceID)
			case "delete":
				err = service.Delete(t.Context(), scenario.Input.WorkspaceID)
			case "creations":
				result, err = service.Creations(t.Context())
			case "start-creation":
				result, err = service.StartCreation(t.Context(), scenario.Input.Name)
			case "update":
				err = service.Update(t.Context(), scenario.Input.WorkspaceID, scenario.Input.Name)
			default:
				t.Fatalf("unknown operation %s", scenario.Operation)
			}
			if scenario.Status >= 400 {
				var envelope struct {
					Code string `json:"code"`
					Msg  string `json:"msg"`
				}
				if err := json.Unmarshal(scenario.Response, &envelope); err != nil {
					t.Fatal(err)
				}
				var failure *Error
				if !errors.As(err, &failure) || failure.Code != envelope.Code || failure.Message != envelope.Msg || failure.Status != scenario.Status {
					t.Fatalf("error=%#v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
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
			}
			if !reflect.DeepEqual(received, workspaceContractJSON(scenario.Request)) {
				t.Fatalf("body=%#v want=%#v", received, workspaceContractJSON(scenario.Request))
			}
		})
	}
}
