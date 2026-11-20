package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNativeBootstrapUsesBearerAndTypedProductContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/newmoi/auth/cli/bootstrap" || request.Header.Get("Authorization") != "Bearer native-token" || request.Header.Get("X-API-Key") != "" || request.ContentLength != 0 {
			t.Errorf("incorrect native bootstrap request")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":"OK","data":{"issuer":"https://uc.example.test","subject":"uc-user","session_id":"session","principal_id":"product-user","catalog_user_id":"catalog-user","workspaces":[{"id":"workspace","status":"ACTIVE","access_status":"accepted"}],"workspace_initialization":{"status":"ready"}}}`))
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithToken("native-token"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	result, err := client.Account().BootstrapNative(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if result.GetPrincipalId() != "product-user" || result.GetCatalogUserId() != "catalog-user" || result.GetWorkspaceInitialization().GetStatus() != "ready" || result.GetWorkspaces()[0].GetAccessStatus() != "accepted" {
		t.Fatal("native bootstrap contract lost fields")
	}
}
