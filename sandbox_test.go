package sdk

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestSandboxSDKQuotaPresenceAndSourceResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/newmoi/agent-sandboxes/quota" || r.Header.Get(headerWorkspaceID) != "workspace" {
			t.Errorf("request scope: %s %v", r.URL.Path, r.Header)
		}
		if r.Method == http.MethodGet {
			if r.URL.Query().Get("include_network_capabilities") != "true" {
				_, _ = w.Write([]byte(`{"code":"OK","data":{"current":2,"limit":3,"default_egress_endpoints":[{"host":"example.test","ports":[443]}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":"OK","data":{"current":2,"limit":3,"unrestricted_network_allowed":true,"default_egress_endpoints":[{"host":"example.test","ports":[443]}]}}`))
			return
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) != 2 || string(body["max_managed_count"]) != "0" || string(body["auto_ephemeral"]) != `"inherit"` {
			t.Errorf("quota update lost explicit zero or added an omitted field: %s, %v", body, err)
		}
		_, _ = w.Write([]byte(`{"code":"OK","data":{"workspace_id":"workspace","max_managed_count":0,"max_ephemeral_count":10,"auto_ephemeral_override":0,"updated_by":"user","updated_at":"2026-10-04T01:02:03Z"}}`))
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	service := client.Sandboxes("workspace")
	status, err := service.GetQuota(t.Context())
	if err != nil || status.GetCurrent() != 2 || status.GetLimit() != 3 || status.UnrestrictedNetworkAllowed != nil || len(status.GetDefaultEgressEndpoints()) != 1 || status.GetDefaultEgressEndpoints()[0].GetPorts()[0] != 443 {
		t.Fatalf("quota=%v err=%v", status, err)
	}
	status, err = service.GetQuota(t.Context(), WithQueryParam("include_network_capabilities", "true"))
	if err != nil || !status.GetUnrestrictedNetworkAllowed() {
		t.Fatalf("requested capability=%v err=%v", status, err)
	}
	quota, err := service.UpdateQuota(t.Context(), &SandboxQuotaRequest{MaxManagedCount: proto.Int32(0), AutoEphemeral: proto.String("inherit")})
	if err != nil || quota.GetMaxManagedCount() != 0 || quota.GetMaxEphemeralCount() != 10 || quota.GetUpdatedBy() != "user" {
		t.Fatalf("updated quota=%v err=%v", quota, err)
	}
}

func TestSandboxSDKAdminAndSSHContracts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /newmoi/admin/agent-sandboxes/sandbox", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":"OK","data":{"id":"sandbox","workspace":{"id":"target"}}}`))
	})
	mux.HandleFunc("PATCH /newmoi/admin/workspaces/target/sandbox-quota", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) != 1 || string(body["max_managed_count"]) != "0" {
			t.Errorf("quota body=%v error=%v", body, err)
		}
		_, _ = w.Write([]byte(`{"code":"OK","data":{"workspace_id":"target","max_managed_count":0}}`))
	})
	mux.HandleFunc("POST /newmoi/agent-sandboxes/sandbox/ssh-session", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":"OK","data":{"token":"session","gateway_host":"127.0.0.1","gateway_port":2222,"gateway_scheme":"http","host_key_fingerprint":"SHA256:abcd","expires_at_ms":1791763200000}}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	service := client.Sandboxes("workspace")
	value, err := service.AdminGet(t.Context(), "sandbox")
	if err != nil || value.GetWorkspace().GetId() != "target" {
		t.Fatalf("detail=%v error=%v", value, err)
	}
	quota, err := service.AdminUpdateQuota(t.Context(), "target", &SandboxQuotaRequest{MaxManagedCount: proto.Int32(0)})
	if err != nil || quota.GetWorkspaceId() != "target" {
		t.Fatalf("quota=%v error=%v", quota, err)
	}
	ssh, err := service.CreateSSHSession(t.Context(), "sandbox")
	if err != nil || ssh.GetToken() != "session" || ssh.GetGatewayPort() != 2222 || ssh.GetExpiresAtMs() != 1791763200000 {
		t.Fatalf("SSH=%v error=%v", ssh, err)
	}
}

func TestSandboxSDKPreservesUnknownUpdateAvailability(t *testing.T) {
	for _, test := range []struct {
		json             string
		known, available bool
	}{
		{`null`, false, false}, {`false`, true, false}, {`true`, true, true},
	} {
		var item AgentSandbox
		if err := parseProductResponse([]byte(`{"code":"OK","data":{"id":"sandbox","update_available":`+test.json+`}}`), &item, "request"); err != nil {
			t.Fatal(err)
		}
		if (item.UpdateAvailable != nil) != test.known || item.GetUpdateAvailable() != test.available {
			t.Fatalf("update availability %s changed to %v", test.json, item.UpdateAvailable)
		}
	}
	var items sandboxList
	if err := parseProductResponse([]byte(`{"code":"OK","data":[{"id":"sandbox","token_jti":"secret"}]}`), &items, "request"); err == nil {
		t.Fatal("unexpected private response field accepted")
	}
}

func TestSandboxSDKPreservesServiceError(t *testing.T) {
	for _, auth := range []Option{WithAPIKey("key"), WithToken("token")} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(headerRequestID, "request")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"code":"SANDBOX_QUOTA_EXCEEDED","msg":"工作区沙箱配额已满"}`))
		}))
		client, err := New(server.URL+"/newmoi", auth)
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Sandboxes("workspace").Create(t.Context(), &SandboxCreateRequest{Name: "sandbox", Spec: "small", DiskGi: 10, NetworkMode: "restricted"})
		var public *Error
		if !errors.As(err, &public) || public.Code != "SANDBOX_QUOTA_EXCEEDED" || public.Message != "工作区沙箱配额已满" || public.RequestID != "request" {
			t.Fatalf("service error changed: %v", err)
		}
		server.Close()
	}
}
