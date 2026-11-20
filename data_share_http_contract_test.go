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
	"sort"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type dataShareContractInput struct {
	IDs                       []string        `json:"ids"`
	SourceDatabaseID          string          `json:"source_database_id"`
	PublishID                 string          `json:"publish_id"`
	SubscriptionID            string          `json:"subscription_id"`
	Name                      string          `json:"name"`
	SubName                   string          `json:"sub_name"`
	TableScope                *DataShareScope `json:"table_scope"`
	TableScopePresent         bool            `json:"table_scope_present"`
	TargetWorkspaceIDs        []string        `json:"target_workspace_ids"`
	TargetWorkspaceIDsPresent bool            `json:"target_workspace_ids_present"`
	Remark                    string          `json:"remark"`
	RemarkPresent             bool            `json:"remark_present"`
}

type dataShareContract struct {
	Name      string                 `json:"name"`
	Operation string                 `json:"operation"`
	Method    string                 `json:"method"`
	Path      string                 `json:"path"`
	Query     map[string]string      `json:"query"`
	Input     dataShareContractInput `json:"input"`
	Request   json.RawMessage        `json:"request"`
	Response  json.RawMessage        `json:"response"`
	Status    int                    `json:"status"`
}

func dataShareContractJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&value)
	return value
}

func dataShareContractQuery(values map[string]string) []QueryOption {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	options := make([]QueryOption, 0, len(keys))
	for _, key := range keys {
		options = append(options, WithQueryParam(key, values[key]))
	}
	return options
}

func dataShareContractExpected(t *testing.T, raw json.RawMessage, message proto.Message) {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if err := protojson.Unmarshal(envelope.Data, message); err != nil {
		t.Fatal(err)
	}
}

func TestDataShareSharedHTTPContracts(t *testing.T) {
	data, err := os.ReadFile("testdata/data_share_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []dataShareContract
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
				if r.Method != scenario.Method || r.URL.EscapedPath() != scenario.Path || !reflect.DeepEqual(r.URL.Query(), expectedQuery) || r.Header.Get("X-Workspace-ID") != "workspace-1" || r.Header.Get("X-API-Key") != "user-key" {
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
			service := client.DataShare("workspace-1")
			input := scenario.Input
			var result proto.Message
			switch scenario.Operation {
			case "publishes":
				result, err = service.Publishes(t.Context(), dataShareContractQuery(scenario.Query)...)
			case "workspace-names":
				result, err = service.WorkspaceNamesResult(t.Context(), input.IDs)
			case "resolve-workspace-names":
				result, err = service.ResolveWorkspaceNames(t.Context(), input.IDs)
			case "publish-summary":
				result, err = service.PublishSummary(t.Context())
			case "source-tables":
				result, err = service.SourceTables(t.Context(), input.SourceDatabaseID, dataShareContractQuery(scenario.Query)...)
			case "check-publish-name":
				result, err = service.CheckPublishName(t.Context(), dataShareContractQuery(scenario.Query)...)
			case "create-publish":
				var options []DataSharePublishOption
				if input.RemarkPresent {
					options = append(options, WithDataShareRemark(input.Remark))
				}
				result, err = service.CreatePublish(t.Context(), input.Name, input.SourceDatabaseID, input.TableScope, input.TargetWorkspaceIDs, options...)
			case "update-publish":
				var options []DataSharePublishUpdateOption
				if input.TableScopePresent {
					options = append(options, WithDataShareUpdateTableScope(input.TableScope))
				}
				if input.TargetWorkspaceIDsPresent {
					options = append(options, WithDataShareUpdateTargetWorkspaceIDs(input.TargetWorkspaceIDs...))
				}
				if input.RemarkPresent {
					options = append(options, WithDataShareUpdateRemark(input.Remark))
				}
				result, err = service.UpdatePublish(t.Context(), input.PublishID, options...)
			case "delete-publish":
				err = service.DeletePublish(t.Context(), input.PublishID)
			case "subscriptions":
				result, err = service.Subscriptions(t.Context(), dataShareContractQuery(scenario.Query)...)
			case "subscribe":
				result, err = service.Subscribe(t.Context(), input.SubscriptionID, input.SubName)
			case "unsubscribe":
				err = service.Unsubscribe(t.Context(), input.SubscriptionID)
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
					expected := result.ProtoReflect().New().Interface()
					dataShareContractExpected(t, scenario.Response, expected)
					if !proto.Equal(result, expected) {
						t.Fatalf("result=%v want=%v", result, expected)
					}
				}
			}
			if !reflect.DeepEqual(received, dataShareContractJSON(scenario.Request)) {
				t.Fatalf("body=%#v want=%#v", received, dataShareContractJSON(scenario.Request))
			}
		})
	}
}
