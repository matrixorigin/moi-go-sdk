package sdk

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConnectorTaskStats(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/newmoi/task/stats" || r.URL.RawQuery != "" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		if r.Header.Get("X-Workspace-ID") != "ws-1" {
			t.Errorf("missing workspace header")
		}
		fmt.Fprint(w, `{"code":"OK","msg":"OK","data":{"total":22,"running":10,"waiting":8,"failed":4}}`)
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Connectors("ws-1").TaskStats(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.GetTotal() != 22 || got.GetRunning() != 10 || got.GetWaiting() != 8 || got.GetFailed() != 4 || calls != 1 {
		t.Fatalf("stats=%v calls=%d", got, calls)
	}
}

func TestConnectorTaskListConfigTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/newmoi/task/list" || r.URL.RawQuery != "config_type=1&config_types=3&config_types=2&page=2&page_size=10&structured=true" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		fmt.Fprint(w, `{"code":"OK","data":{"total":0,"tasks":[]}}`)
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Connectors("ws-1").Tasks(t.Context(), WithQueryParam("config_type", "1"), WithQueryValues("config_types", "3", "2"), WithPage(2), WithPageSize(10), WithQueryParam("structured", "true"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestConnectorTasksPreservesParameterError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Language") != "en-US" {
			t.Errorf("missing locale")
		}
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"code":"ErrParamInvalid","msg":"Invalid parameters. Please check and try again."}`)
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"), WithCustomHeader("Accept-Language", "en-US"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Connectors("ws").Tasks(t.Context(), WithQueryValues("config_types", "0"))
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "ErrParamInvalid" || failure.Status != 400 || failure.Message != "Invalid parameters. Please check and try again." {
		t.Fatalf("public parameter error changed: %#v", err)
	}
}
