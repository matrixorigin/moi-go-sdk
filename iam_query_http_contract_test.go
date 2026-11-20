package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sort"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type iamQueryHTTPContract struct {
	Name      string            `json:"name"`
	Operation string            `json:"operation"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Query     map[string]string `json:"query"`
	Input     struct {
		RoleID                   string   `json:"role_id"`
		ExpectedLifecycleVersion int64    `json:"expected_lifecycle_version"`
		IncludeOwnership         bool     `json:"include_ownership"`
		OwnershipCursor          string   `json:"ownership_cursor"`
		OwnershipLimit           int      `json:"ownership_limit"`
		Cursor                   string   `json:"cursor"`
		Limit                    int      `json:"limit"`
		TransferRequested        bool     `json:"transfer_requested"`
		ResourceType             string   `json:"resource_type"`
		ActionID                 string   `json:"action_id"`
		Purpose                  string   `json:"purpose"`
		ResourceIDs              []string `json:"resource_ids"`
	} `json:"input"`
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response"`
	Status   int             `json:"status"`
}

func iamQueryContractJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&value)
	return value
}

func iamQueryContractEnvelopeData(raw json.RawMessage) any {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		panic(err)
	}
	return iamQueryContractJSON(envelope.Data)
}

func iamQueryContractOptions(query map[string]string, keys ...string) []QueryOption {
	sort.Strings(keys)
	options := make([]QueryOption, 0, len(keys))
	for _, key := range keys {
		if value, ok := query[key]; ok {
			options = append(options, WithQueryParam(key, value))
		}
	}
	return options
}

func TestIAMQuerySharedHTTPContracts(t *testing.T) {
	data, err := os.ReadFile("testdata/iam_query_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []iamQueryHTTPContract
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
				if r.Method != scenario.Method || r.URL.EscapedPath() != scenario.Path || !reflect.DeepEqual(r.URL.Query(), expectedQuery) || r.Header.Get("X-API-Key") != "user-key" || r.Header.Get("X-Workspace-ID") != "workspace-1" {
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
			service := client.IAM("workspace-1")
			input := scenario.Input
			var result proto.Message
			var rawResult *ProductData
			switch scenario.Operation {
			case "role-delete-impact":
				var options []QueryOption
				if input.IncludeOwnership {
					options = append(options, WithQueryParam("include_ownership", "true"), WithQueryParam("ownership_limit", fmt.Sprint(input.OwnershipLimit)))
					if input.OwnershipCursor != "" {
						options = append(options, WithQueryParam("ownership_cursor", input.OwnershipCursor))
					}
				}
				if input.TransferRequested {
					options = append(options, WithQueryParam("transfer_requested", "true"))
				}
				result, err = service.RoleDeleteImpact(t.Context(), input.RoleID, input.ExpectedLifecycleVersion, options...)
			case "role-delete-ownerships":
				options := iamQueryContractOptions(scenario.Query, "cursor", "limit", "transfer_requested")
				result, err = service.RoleDeleteOwnerships(t.Context(), input.RoleID, options...)
			case "role-members":
				options := iamQueryContractOptions(scenario.Query, "cursor", "limit")
				result, err = service.RoleMembers(t.Context(), input.RoleID, options...)
			case "list-permission-resources":
				options := iamQueryContractOptions(scenario.Query, "parent_id", "cursor", "q", "limit")
				rawResult, err = service.ListPermissionResources(t.Context(), input.ResourceType, input.ActionID, input.Purpose, options...)
			case "batch-describe-permission-resources":
				rawResult, err = service.BatchDescribePermissionResources(t.Context(), input.ResourceType, input.ActionID, input.Purpose, input.ResourceIDs)
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
				if rawResult != nil && !reflect.DeepEqual(rawResult.Value, iamQueryContractEnvelopeData(scenario.Response)) {
					t.Fatalf("raw result=%#v want=%#v", rawResult.Value, iamQueryContractEnvelopeData(scenario.Response))
				}
			}
			if !reflect.DeepEqual(received, iamQueryContractJSON(scenario.Request)) {
				t.Fatalf("body=%#v want=%#v", received, iamQueryContractJSON(scenario.Request))
			}
		})
	}
}
