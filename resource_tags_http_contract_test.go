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

func TestResourceTagsMatchSharedHTTPContract(t *testing.T) {
	var cases []struct {
		Name, Operation, Method, Path string
		TagID                         string `json:"tag_id"`
		ValueID                       string `json:"value_id"`
		Input, Request, Response      json.RawMessage
		Query                         map[string]string
		Status                        int
	}
	data, err := os.ReadFile("testdata/resource_tags_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()
			decodeJSON := func(data []byte) any {
				t.Helper()
				var value any
				decoder := json.NewDecoder(bytes.NewReader(data))
				decoder.UseNumber()
				if err := decoder.Decode(&value); err != nil {
					t.Fatal(err)
				}
				return value
			}
			query := url.Values{}
			for key, value := range scenario.Query {
				query.Set(key, value)
			}
			received := make(chan any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != scenario.Method || r.URL.Path != scenario.Path || !reflect.DeepEqual(r.URL.Query(), query) || r.Header.Get("X-API-Key") != "synthetic-key" || r.Header.Get("X-Workspace-ID") != "workspace-1" {
					t.Errorf("unexpected scoped request: %s %s", r.Method, r.URL.RequestURI())
				}
				raw, err := io.ReadAll(r.Body)
				var body any
				if err != nil {
					t.Error(err)
				} else if len(raw) > 0 {
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					if err := decoder.Decode(&body); err != nil {
						t.Error(err)
					}
				}
				received <- body
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				_, _ = w.Write(scenario.Response)
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			service := client.ResourceTags("workspace-1")
			decode := func(value proto.Message) {
				t.Helper()
				// Protobuf construction preserves the Value(null) selector used by
				// real Go callers with structpb.NewNullValue().
				if err := protojson.Unmarshal(scenario.Input, value); err != nil {
					t.Fatal(err)
				}
			}
			var result proto.Message
			var callErr error
			switch scenario.Operation {
			case "create":
				request := &ResourceTagCreateRequest{}
				decode(request)
				result, callErr = service.Create(t.Context(), request)
			case "update":
				request := &ResourceTagUpdateRequest{}
				decode(request)
				result, callErr = service.Update(t.Context(), scenario.TagID, request)
			case "delete":
				callErr = service.Delete(t.Context(), scenario.TagID)
			case "list", "list-values":
				request := &ResourceTagListRequest{}
				decode(request)
				if scenario.Operation == "list" {
					result, callErr = service.List(t.Context(), request)
				} else {
					result, callErr = service.ListValues(t.Context(), scenario.TagID, request)
				}
			case "create-value", "update-value":
				request := &ResourceTagValueRequest{}
				decode(request)
				if scenario.Operation == "create-value" {
					result, callErr = service.CreateValue(t.Context(), scenario.TagID, request)
				} else {
					result, callErr = service.UpdateValue(t.Context(), scenario.TagID, scenario.ValueID, request)
				}
			case "delete-value":
				callErr = service.DeleteValue(t.Context(), scenario.TagID, scenario.ValueID)
			case "update-relations":
				request := &ResourceTagRelationsRequest{}
				decode(request)
				callErr = service.UpdateRelations(t.Context(), request)
			case "groups", "resources":
				request := &ResourceTagQueryRequest{}
				decode(request)
				if scenario.Operation == "groups" {
					result, callErr = service.Groups(t.Context(), request)
				} else {
					result, callErr = service.Resources(t.Context(), request)
				}
			case "assignments":
				request := &ResourceTagAssignmentsRequest{}
				decode(request)
				result, callErr = service.Assignments(t.Context(), request)
			default:
				t.Fatalf("unsupported fixture operation: %s", scenario.Operation)
			}
			select {
			case body := <-received:
				if want := decodeJSON(scenario.Request); !reflect.DeepEqual(body, want) {
					t.Errorf("HTTP body=%v, want=%v", body, want)
				}
			default:
				t.Fatal("no HTTP request received")
			}
			var response struct {
				Code, Msg string
				Data      json.RawMessage
			}
			if err := json.Unmarshal(scenario.Response, &response); err != nil {
				t.Fatal(err)
			}
			if scenario.Status >= 400 {
				var failure *Error
				if !errors.As(callErr, &failure) || failure.Code != response.Code || failure.Message != response.Msg || failure.Status != scenario.Status {
					t.Fatalf("owner error changed: %v", callErr)
				}
				return
			}
			if callErr != nil {
				t.Fatal(callErr)
			}
			if result != nil {
				want := result.ProtoReflect().New().Interface()
				if err := protojson.Unmarshal(response.Data, want); err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(result, want) {
					t.Errorf("response=%v, want=%v", result, want)
				}
			}
		})
	}
}
