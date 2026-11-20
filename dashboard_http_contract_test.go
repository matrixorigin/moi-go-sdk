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

func TestDashboardMatchesSharedHTTPContract(t *testing.T) {
	var cases []struct {
		Name, Operation, Method, Path, CSV string
		DashboardID                        string `json:"dashboard_id"`
		ChartID                            string `json:"chart_id"`
		Input, Request, Response           json.RawMessage
		Query                              map[string]string
		Status                             int
	}
	data, err := os.ReadFile("testdata/dashboard_http_contract.json")
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
				var result any
				decoder := json.NewDecoder(bytes.NewReader(data))
				decoder.UseNumber()
				if err := decoder.Decode(&result); err != nil {
					t.Fatal(err)
				}
				return result
			}
			query := url.Values{}
			var options []QueryOption
			for key, value := range scenario.Query {
				query.Set(key, value)
				options = append(options, WithQueryParam(key, value))
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
				if scenario.CSV != "" {
					w.Header().Set("Content-Type", "text/csv; charset=utf-8")
					w.WriteHeader(scenario.Status)
					_, _ = io.WriteString(w, scenario.CSV)
				} else {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(scenario.Status)
					_, _ = w.Write(scenario.Response)
				}
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			service := client.Dashboard("workspace-1")
			decode := func(request proto.Message) {
				t.Helper()
				if err := protojson.Unmarshal(scenario.Input, request); err != nil {
					t.Fatal(err)
				}
			}
			var result any
			var callErr error
			switch scenario.Operation {
			case "task-execution-overview":
				result, callErr = service.TaskExecutionOverview(t.Context())
			case "create", "update":
				request := &DataDashboardMutationRequest{}
				decode(request)
				if scenario.Operation == "create" {
					result, callErr = service.Create(t.Context(), request)
				} else {
					result, callErr = service.Update(t.Context(), scenario.DashboardID, request)
				}
			case "list":
				request := &DataDashboardListRequest{}
				decode(request)
				result, callErr = service.List(t.Context(), request)
			case "get":
				result, callErr = service.Get(t.Context(), scenario.DashboardID)
			case "delete":
				result, callErr = service.Delete(t.Context(), scenario.DashboardID)
			case "create-chart", "update-chart":
				request := &DataDashboardChartMutationRequest{}
				decode(request)
				if scenario.Operation == "create-chart" {
					result, callErr = service.CreateChart(t.Context(), scenario.DashboardID, request)
				} else {
					result, callErr = service.UpdateChart(t.Context(), scenario.DashboardID, scenario.ChartID, request)
				}
			case "delete-chart":
				result, callErr = service.DeleteChart(t.Context(), scenario.DashboardID, scenario.ChartID)
			case "update-chart-order":
				request := &DataDashboardChartOrderRequest{}
				decode(request)
				result, callErr = service.UpdateChartOrder(t.Context(), scenario.DashboardID, request)
			case "preview-sql":
				var input struct {
					SQLText string  `json:"sql_text"`
					ChartID *string `json:"chart_id"`
				}
				if err := json.Unmarshal(scenario.Input, &input); err != nil {
					t.Fatal(err)
				}
				var options []DashboardPreviewOption
				if input.ChartID != nil {
					options = append(options, WithDashboardPreviewChartID(*input.ChartID))
				}
				result, callErr = service.PreviewSQL(t.Context(), scenario.DashboardID, input.SQLText, options...)
			case "generate-sql":
				var input struct{ Question, Title string }
				if err := json.Unmarshal(scenario.Input, &input); err != nil {
					t.Fatal(err)
				}
				result, callErr = service.GenerateSQL(t.Context(), scenario.DashboardID, input.Question, input.Title)
			case "refresh":
				result, callErr = service.Refresh(t.Context(), scenario.DashboardID)
			case "run-chart":
				result, callErr = service.RunChart(t.Context(), scenario.DashboardID, scenario.ChartID)
			case "chart-result":
				result, callErr = service.ChartResult(t.Context(), scenario.DashboardID, scenario.ChartID, options...)
			case "download-chart-result":
				result, callErr = service.DownloadChartResult(t.Context(), scenario.DashboardID, scenario.ChartID, options...)
			case "drill-through", "download-drill-through":
				request := &DataDashboardDrillThroughRequest{}
				decode(request)
				if scenario.Operation == "drill-through" {
					result, callErr = service.DrillThrough(t.Context(), scenario.DashboardID, scenario.ChartID, request)
				} else {
					result, callErr = service.DownloadDrillThrough(t.Context(), scenario.DashboardID, scenario.ChartID, request)
				}
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
			switch value := result.(type) {
			case *StreamResponse:
				t.Cleanup(func() { _ = value.Body.Close() })
				body, err := io.ReadAll(value.Body)
				if err != nil || string(body) != scenario.CSV {
					t.Fatalf("CSV=%q, want=%q, error=%v", body, scenario.CSV, err)
				}
			case proto.Message:
				want := value.ProtoReflect().New().Interface()
				if err := protojson.Unmarshal(response.Data, want); err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(value, want) {
					t.Errorf("response=%v, want=%v", value, want)
				}
			default:
				encoded, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(decodeJSON(encoded), decodeJSON(response.Data)) {
					t.Errorf("response=%s, want=%s", encoded, response.Data)
				}
			}
		})
	}
}
