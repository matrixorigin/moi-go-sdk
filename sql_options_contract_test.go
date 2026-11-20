package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestSQLOptionsMatchSharedHandlerContract(t *testing.T) {
	var cases []struct {
		Name, Command, Path string
		Input               map[string]json.RawMessage
		Request, Response   json.RawMessage
		Envelope            json.RawMessage
		Status              int
		ErrorCode           string `json:"error_code"`
	}
	data, err := os.ReadFile("testdata/sql_options_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()
			var want any
			decoder := json.NewDecoder(bytes.NewReader(scenario.Request))
			decoder.UseNumber()
			if err := decoder.Decode(&want); err != nil {
				t.Fatal(err)
			}
			received := make(chan any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/newmoi"+scenario.Path || r.URL.RawQuery != "" || r.Header.Get("X-Workspace-ID") != "workspace-1" || r.Header.Get("X-API-Key") != "synthetic-key" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
				}
				var got any
				decoder := json.NewDecoder(r.Body)
				decoder.UseNumber()
				if err := decoder.Decode(&got); err != nil {
					t.Error(err)
				}
				received <- got
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				_, _ = w.Write(scenario.Envelope)
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			service := client.SQL("workspace-1")
			var result proto.Message
			i := scenario.Input
			switch scenario.Command {
			case "workbooks":
				var opts []WorkbookListOption
				for key := range i {
					switch key {
					case "search":
						opts = append(opts, WithWorkbookSearch(sqlContractValue[string](t, i, key)))
					case "page_size":
						opts = append(opts, WithWorkbookPageSize(sqlContractValue[int](t, i, key)))
					case "page_token":
						opts = append(opts, WithWorkbookPageToken(sqlContractValue[string](t, i, key)))
					}
				}
				result, err = service.Workbooks(t.Context(), opts...)
			case "workbook-versions":
				var opts []WorkbookVersionListOption
				if _, ok := i["page_size"]; ok {
					opts = append(opts, WithWorkbookVersionPageSize(sqlContractValue[int](t, i, "page_size")))
				}
				if _, ok := i["page_token"]; ok {
					opts = append(opts, WithWorkbookVersionPageToken(sqlContractValue[string](t, i, "page_token")))
				}
				result, err = service.WorkbookVersions(t.Context(), sqlContractValue[string](t, i, "workbook_id"), opts...)
			case "query-history-overview":
				var opts []QueryHistoryOverviewOption
				for key := range i {
					switch key {
					case "scope":
						opts = append(opts, WithQueryHistoryOverviewScope(QueryHistoryScope(sqlContractValue[string](t, i, key))))
					case "non_user":
						opts = append(opts, WithQueryHistoryOverviewNonUser(sqlContractValue[bool](t, i, key)))
					case "start":
						opts = append(opts, WithQueryHistoryOverviewStart(sqlContractTime(t, i, key)))
					case "end":
						opts = append(opts, WithQueryHistoryOverviewEnd(sqlContractTime(t, i, key)))
					case "sql_source_type":
						opts = append(opts, WithQueryHistoryOverviewSQLSourceTypes(sqlContractValue[[]string](t, i, key)...))
					}
				}
				result, err = service.QueryHistoryOverview(t.Context(), opts...)
			case "query-history":
				result, err = service.QueryHistory(t.Context(), sqlContractHistoryOptions(t, i)...)
			case "query-detail", "query-profile":
				var opts []QueryRecordOption
				for key := range i {
					switch key {
					case "scope":
						opts = append(opts, WithQueryRecordScope(QueryHistoryScope(sqlContractValue[string](t, i, key))))
					case "start":
						opts = append(opts, WithQueryRecordStart(sqlContractValue[string](t, i, key)))
					case "end":
						opts = append(opts, WithQueryRecordEnd(sqlContractValue[string](t, i, key)))
					}
				}
				id := sqlContractValue[string](t, i, "statement_id")
				if scenario.Command == "query-detail" {
					result, err = service.QueryDetail(t.Context(), id, opts...)
				} else {
					result, err = service.QueryProfile(t.Context(), id, opts...)
				}
			case "execute-query", "execute", "run":
				var opts []SQLQueryOption
				for key := range i {
					switch key {
					case "db_name":
						opts = append(opts, WithSQLQueryDBName(sqlContractValue[string](t, i, key)))
					case "sql_type":
						opts = append(opts, WithSQLQueryType(sqlContractValue[string](t, i, key)))
					case "offset":
						opts = append(opts, WithSQLQueryOffset(sqlContractValue[int](t, i, key)))
					case "limit":
						opts = append(opts, WithSQLQueryLimit(sqlContractValue[int](t, i, key)))
					}
				}
				if scenario.Command == "run" {
					result, err = service.RunQuery(t.Context(), sqlContractValue[string](t, i, "query"), opts...)
				} else {
					result, err = service.ExecuteQuery(t.Context(), sqlContractValue[string](t, i, "query"), opts...)
				}
			case "query-result":
				result, err = service.QueryResult(t.Context(), sqlContractValue[string](t, i, "statement_id"), WithSQLResultOffset(sqlContractValue[int](t, i, "offset")), WithSQLResultLimit(sqlContractValue[int](t, i, "limit")))
			default:
				t.Fatalf("unknown fixture command %s", scenario.Command)
			}
			select {
			case got := <-received:
				if !reflect.DeepEqual(got, want) {
					t.Errorf("request=%v, want=%v", got, want)
				}
			default:
				t.Fatal("no HTTP request received")
			}
			if scenario.ErrorCode != "" {
				var failure *Error
				if !errors.As(err, &failure) {
					t.Fatalf("expected owner rejection, got %v", err)
				}
				var envelope struct{ Msg, Message string }
				if err := json.Unmarshal(scenario.Envelope, &envelope); err != nil {
					t.Fatal(err)
				}
				message := envelope.Msg
				if message == "" {
					message = envelope.Message
				}
				if failure.Code != scenario.ErrorCode || failure.Status != scenario.Status || failure.Message != message {
					t.Fatalf("owner failure=%+v", failure)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			wantResponse := result.ProtoReflect().New().Interface()
			if err := protojson.Unmarshal(scenario.Response, wantResponse); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(result, wantResponse) {
				t.Errorf("response=%v, want=%v", result, wantResponse)
			}
		})
	}
}

func sqlContractValue[T any](t *testing.T, input map[string]json.RawMessage, key string) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(input[key], &value); err != nil {
		t.Fatalf("%s: %v", key, err)
	}
	return value
}

func sqlContractTime(t *testing.T, input map[string]json.RawMessage, key string) time.Time {
	t.Helper()
	value, err := time.Parse(time.RFC3339Nano, sqlContractValue[string](t, input, key))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func sqlContractHistoryOptions(t *testing.T, input map[string]json.RawMessage) []QueryHistoryOption {
	t.Helper()
	var opts []QueryHistoryOption
	for key := range input {
		switch key {
		case "scope":
			opts = append(opts, WithQueryHistoryScope(QueryHistoryScope(sqlContractValue[string](t, input, key))))
		case "non_user":
			opts = append(opts, WithQueryHistoryNonUser(sqlContractValue[bool](t, input, key)))
		case "status":
			opts = append(opts, WithQueryHistoryStatus(sqlContractValue[string](t, input, key)))
		case "user":
			opts = append(opts, WithQueryHistoryUsers(sqlContractValue[[]string](t, input, key)...))
		case "databases":
			opts = append(opts, WithQueryHistoryDatabases(sqlContractValue[[]string](t, input, key)...))
		case "statement":
			opts = append(opts, WithQueryHistoryStatement(sqlContractValue[string](t, input, key)))
		case "query_type":
			opts = append(opts, WithQueryHistoryTypes(sqlContractValue[[]string](t, input, key)...))
		case "duration":
			opts = append(opts, WithQueryHistoryMinDuration(sqlContractValue[uint64](t, input, key)))
		case "start":
			opts = append(opts, WithQueryHistoryStart(sqlContractTime(t, input, key)))
		case "end":
			opts = append(opts, WithQueryHistoryEnd(sqlContractTime(t, input, key)))
		case "statement_id":
			opts = append(opts, WithQueryHistoryStatementID(sqlContractValue[string](t, input, key)))
		case "transaction_id":
			opts = append(opts, WithQueryHistoryTransactionID(sqlContractValue[string](t, input, key)))
		case "session_id":
			opts = append(opts, WithQueryHistorySessionID(sqlContractValue[string](t, input, key)))
		case "selected_field":
			opts = append(opts, WithQueryHistorySelectedFields(sqlContractValue[[]string](t, input, key)...))
		case "sql_source_type":
			opts = append(opts, WithQueryHistorySQLSourceTypes(sqlContractValue[[]string](t, input, key)...))
		case "offset":
			opts = append(opts, WithQueryHistoryOffset(sqlContractValue[uint](t, input, key)))
		case "limit":
			opts = append(opts, WithQueryHistoryLimit(sqlContractValue[uint](t, input, key)))
		case "order_by":
			opts = append(opts, WithQueryHistoryOrderBy(sqlContractValue[[]string](t, input, key)...))
		case "order":
			opts = append(opts, WithQueryHistoryOrder(sqlContractValue[string](t, input, key)))
		case "cu":
			opts = append(opts, WithQueryHistoryCU(sqlContractValue[uint](t, input, key)))
		}
	}
	return opts
}
