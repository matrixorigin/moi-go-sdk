package sdk

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSQLDatabasePaginationOptionsAndCursor(t *testing.T) {
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/newmoi/meta/db/info" || r.Header.Get(headerWorkspaceID) != "ws-1" {
			t.Errorf("unexpected request %s workspace=%s", r.URL.Path, r.Header.Get(headerWorkspaceID))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
			t.Error(err)
		}
		bodies = append(bodies, body)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": map[string]any{"result": []any{map[string]any{"db_name": "sales.data`2026"}}, "total": 20, "next_page_token": "sales.data`2026"}})
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	metadata := workspace.SQLMetadata()
	if _, err := metadata.Databases(t.Context()); err != nil {
		t.Fatal(err)
	}
	page, err := metadata.Databases(t.Context(), WithSQLDatabasePageSize(10), WithSQLDatabasePageToken("previous"), WithSQLDatabaseSearch("sales"), WithSQLDatabaseName("sales.data`2026"))
	if err != nil {
		t.Fatal(err)
	}
	if page.NextPageToken != "sales.data`2026" || page.Total != 20 || len(page.Result) != 1 {
		t.Fatalf("page=%+v", page)
	}
	want := []map[string]any{nil, {"page_size": float64(10), "page_token": "previous", "search": "sales", "name": "sales.data`2026"}}
	if !reflect.DeepEqual(bodies, want) {
		t.Fatalf("bodies=%#v want=%#v", bodies, want)
	}
}
