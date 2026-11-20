package sdk

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestConnectorExportTaskStats(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/newmoi/export/task/stats" || r.URL.RawQuery != "" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		if r.Header.Get("X-Workspace-ID") != "ws-1" {
			t.Errorf("missing workspace header")
		}
		fmt.Fprint(w, `{"code":"OK","msg":"OK","data":{"total":22,"running":10,"pending":6,"failed":4}}`)
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Connectors("ws-1").ExportTaskStats(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.GetTotal() != 22 || got.GetRunning() != 10 || got.GetPending() != 6 || got.GetFailed() != 4 || calls != 1 {
		t.Fatalf("stats=%v calls=%d", got, calls)
	}
}

func TestConnectorExportTaskListSearchOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/newmoi/export/task/list" {
			t.Errorf("request: %s %s", r.Method, r.URL)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		want := map[string]any{"keyword": "资料", "target_category": "storage", "statuses": []any{float64(0)}, "limit": float64(10), "offset": float64(20)}
		if !reflect.DeepEqual(want, body) {
			t.Errorf("body=%v want=%v", body, want)
		}
		fmt.Fprint(w, `{"code":"OK","data":{"total":0,"tasks":[]}}`)
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Connectors("ws").ExportTasks(t.Context(), WithExportTaskKeyword("资料"), WithExportTaskTargetCategory("storage"), WithExportTaskStatuses(0), WithExportTaskLimit(10), WithExportTaskOffset(20))
	if err != nil {
		t.Fatal(err)
	}
}
