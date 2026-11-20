package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatalogBrowseCanSkipChildStatistics(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["include_stats"] != false || r.Header.Get(headerWorkspaceID) != "ws-1" {
			t.Errorf("body = %+v, workspace = %s", body, r.Header.Get(headerWorkspaceID))
		}
		var items any
		switch r.URL.Path {
		case "/newmoi/catalog/list":
			items = []map[string]any{{"id": 10, "name": "default"}}
		case "/newmoi/catalog/database/list":
			if body["id"] != float64(10) {
				t.Errorf("catalog id = %v", body["id"])
			}
			items = []map[string]any{{"id": 20, "name": "testdb"}}
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": map[string]any{"list": items}})
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
	catalogs, _, err := workspace.Catalogs(t.Context(), WithCatalogIncludeStats(false))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalogs) != 1 {
		t.Fatalf("catalogs = %+v", catalogs)
	}
	databases, _, err := catalogs[0].Databases(t.Context(), WithCatalogIncludeStats(false))
	if err != nil {
		t.Fatal(err)
	}
	if len(databases) != 1 || requests != 2 {
		t.Fatalf("databases = %+v, requests = %d", databases, requests)
	}
}

func TestSystemCatalogTreeCanLoadOneLevel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/newmoi/catalog/system/tree" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["include_table_leaves"] != false || body["database_name"] != "system" || body["table_name"] != "statement_info" {
			t.Errorf("unexpected body: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": map[string]any{"tree": []any{}}})
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Catalog("ws-1").SystemTree(t.Context(), WithSystemCatalogDatabase("system"), WithSystemCatalogTableLeaves(false), WithSystemCatalogTableName("statement_info"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestCatalogNavigationPaginationContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["page_size"] != float64(20) || body["page_token"] != "cursor" || r.Header.Get(headerWorkspaceID) != "ws-1" {
			t.Errorf("unexpected pagination request: %+v", body)
		}
		key := "list"
		if r.URL.Path == "/newmoi/catalog/tree" || r.URL.Path == "/newmoi/catalog/system/tree" {
			key = "tree"
			if body["search"] != "report" {
				t.Errorf("missing search: %+v", body)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": map[string]any{key: []any{}, "next_page_token": "next"}})
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	service := client.Catalog("ws-1")
	list, err := service.List(t.Context(), WithCatalogPageSize(20), WithCatalogPageToken("cursor"))
	if err != nil || list.GetNextPageToken() != "next" {
		t.Fatalf("list: %+v, %v", list, err)
	}
	databases, err := service.ListDatabases(t.Context(), 10, WithCatalogPageSize(20), WithCatalogPageToken("cursor"))
	if err != nil || databases.GetNextPageToken() != "next" {
		t.Fatalf("databases: %+v, %v", databases, err)
	}
	children, err := service.DatabaseChildren(t.Context(), 20, WithCatalogPageSize(20), WithCatalogPageToken("cursor"))
	if err != nil || children.GetNextPageToken() != "next" {
		t.Fatalf("children: %+v, %v", children, err)
	}
	tree, err := service.Tree(t.Context(), WithCatalogTreePageSize(20), WithCatalogTreePageToken("cursor"), WithCatalogTreeSearch("report"))
	if err != nil || tree.GetNextPageToken() != "next" {
		t.Fatalf("tree: %+v, %v", tree, err)
	}
	system, err := service.SystemTree(t.Context(), WithSystemCatalogPageSize(20), WithSystemCatalogPageToken("cursor"), WithSystemCatalogSearch("report"))
	if err != nil || system.GetNextPageToken() != "next" {
		t.Fatalf("system: %+v, %v", system, err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	database := &DatabaseHandle{catalog: &CatalogHandle{workspace: workspace, id: 10}, id: 20}
	page, err := database.Children(t.Context(), WithCatalogPageSize(20), WithCatalogPageToken("cursor"))
	if err != nil || page.GetNextPageToken() != "next" {
		t.Fatalf("scoped children: %+v, %v", page, err)
	}

}
