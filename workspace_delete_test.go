package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkspaceDeleteWaitQuery(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		options []QueryOption
		query   string
	}{
		{name: "default"},
		{name: "wait for account", options: []QueryOption{WithQueryParam("wait", "true")}, query: "wait=true"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/newmoi/workspaces/workspace" || r.URL.RawQuery != scenario.query {
					t.Errorf("unexpected deletion request: %s %s", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"code":"OK","msg":"OK","data":null}`))
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", WithAPIKey("test-key"))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if err := client.Workspaces().Delete(t.Context(), "workspace", scenario.options...); err != nil {
				t.Fatal(err)
			}
			handle, err := client.Workspace("workspace")
			if err != nil {
				t.Fatal(err)
			}
			if err := handle.Delete(t.Context(), scenario.options...); err != nil {
				t.Fatal(err)
			}
		})
	}
}
