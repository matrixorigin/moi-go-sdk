package sdk

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMysqlImportCapabilityPreservesProofAndFailsClosedWhenAbsent(t *testing.T) {
	for _, payload := range []struct {
		data      string
		available bool
	}{
		{`{"mysql_cdc_available":true,"capability_profile_hash":"proof","doris_cdc_available":false}`, true},
		{`{"capability_profile_hash":"proof","doris_cdc_available":true}`, false},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/connectors/structured/v1/source/capabilities" {
				t.Errorf("route = %s %s", r.Method, r.URL.Path)
			}
			if r.Header.Get("X-Workspace-ID") != "workspace" {
				t.Errorf("workspace lost")
			}
			_, _ = fmt.Fprintf(w, `{"code":"OK","data":%s}`, payload.data)
		}))
		client, err := New(server.URL, WithAPIKey("key"))
		if err != nil {
			server.Close()
			t.Fatal(err)
		}
		result, err := client.Connectors("workspace").StructuredSourceCapability(t.Context(), "connector", "mysql", "source", "items", "")
		server.Close()
		if err != nil {
			t.Fatal(err)
		}
		if result.MysqlCDCAvailable != payload.available || result.CapabilityProfileHash != "proof" {
			t.Fatalf("capability = %#v", result)
		}
	}
}
