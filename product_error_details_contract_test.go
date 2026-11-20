package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func TestProductErrorsMatchSharedDetailsContract(t *testing.T) {
	var cases []struct {
		Name, Operation string
		ExpectedCode    string `json:"expected_code"`
		FollowTaskID    string `json:"follow_task_id"`
		Status          int
		Input, Response json.RawMessage
	}
	data, err := os.ReadFile("testdata/product_error_details_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()
			decode := func(raw []byte) any {
				t.Helper()
				if len(raw) == 0 {
					return nil
				}
				var value any
				decoder := json.NewDecoder(bytes.NewReader(raw))
				decoder.UseNumber()
				if err := decoder.Decode(&value); err != nil {
					t.Fatal(err)
				}
				return value
			}
			compare := func(actual any, expected []byte) {
				t.Helper()
				raw, err := json.Marshal(actual)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(decode(raw), decode(expected)) {
					t.Errorf("diagnostic=%s, want=%s", raw, expected)
				}
			}
			received := make(chan []byte, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method, path := http.MethodPatch, "/newmoi/data-dashboards/dashboard-1"
				if scenario.Operation == "download-chart-result" {
					method, path = http.MethodGet, path+"/charts/chart-1/result/download"
				}
				if r.Method != method || r.URL.RequestURI() != path || r.Header.Get("X-API-Key") != "synthetic-key" || r.Header.Get("X-Workspace-ID") != "workspace-1" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				received <- body
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Request-ID", "transport-request")
				w.WriteHeader(scenario.Status)
				_, _ = w.Write(scenario.Response)
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			var callErr error
			if scenario.Operation == "download-chart-result" {
				stream, err := client.Dashboard("workspace-1").DownloadChartResult(t.Context(), "dashboard-1", "chart-1")
				callErr = err
				if stream != nil {
					_ = stream.Body.Close()
					t.Error("error response was returned as a download")
				}
			} else {
				var request DataDashboardMutationRequest
				if err := json.Unmarshal(scenario.Input, &request); err != nil {
					t.Fatal(err)
				}
				_, callErr = client.Dashboard("workspace-1").Update(t.Context(), "dashboard-1", &request)
			}
			select {
			case body := <-received:
				if !reflect.DeepEqual(decode(body), decode(scenario.Input)) {
					t.Errorf("request=%s, want=%s", body, scenario.Input)
				}
			default:
				t.Fatal("no HTTP request received")
			}
			var failure *Error
			if !errors.As(callErr, &failure) || failure.Code != scenario.ExpectedCode || failure.Status != scenario.Status || failure.RequestID != "transport-request" {
				t.Fatalf("owner error lost: %#v, error=%v; want code=%s status=%d", failure, callErr, scenario.ExpectedCode, scenario.Status)
			}
			var response struct {
				Msg, Message  string
				Details, Data json.RawMessage
			}
			if err := json.Unmarshal(scenario.Response, &response); err != nil {
				t.Fatal(err)
			}
			message := response.Message
			if response.Msg != "" {
				message = response.Msg
			}
			if scenario.ExpectedCode == "ErrResponseInvalid" {
				if failure.Message == "" || failure.Message == message || len(failure.Details) != 0 {
					t.Fatalf("invalid response exposed owner payload: %#v", failure)
				}
				return
			}
			if failure.Message != message {
				t.Errorf("message=%q, want=%q", failure.Message, message)
			}
			if expected := decode(response.Details); expected == nil {
				if len(failure.Details) != 0 {
					t.Errorf("unexpected details: %v", failure.Details)
				}
			} else {
				compare(failure.Details, response.Details)
			}
			if !reflect.DeepEqual(decode(failure.Data), decode(response.Data)) {
				t.Errorf("repair data=%s, want=%s", failure.Data, response.Data)
			}
			if scenario.FollowTaskID != "" {
				before, err := json.Marshal(failure)
				if err != nil {
					t.Fatal(err)
				}
				var followed *Error
				if err := a2aFollowError(t.Context(), scenario.FollowTaskID, failure); !errors.As(err, &followed) {
					t.Fatalf("follow error lost type: %v", err)
				}
				want := decode(response.Details).(map[string]any)
				want["task_id"] = scenario.FollowTaskID
				expected, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				compare(followed.Details, expected)
				after, err := json.Marshal(failure)
				if err != nil || !bytes.Equal(before, after) || followed.Code != failure.Code || followed.Status != failure.Status || !bytes.Equal(followed.Data, failure.Data) {
					t.Fatalf("follow changed the source error or dropped diagnostics: original=%s after=%s followed=%v", before, after, followed)
				}
			}
		})
	}
}
