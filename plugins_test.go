package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestCompletePluginOAuthCallbackBrowserProof(t *testing.T) {
	for _, route := range []struct {
		name, prefix, callbackPrefix string
		product                      bool
	}{
		{"root", "/deployment/newmoi", "/deployment", false},
		{"product", "/newmoi", "/newmoi", true},
		{"deployment-product", "/deployment/newmoi", "/deployment/newmoi", true},
		{"custom-product", "/custom", "/custom", true},
	} {
		t.Run(route.name, func(t *testing.T) {
			complete := func(client *Client, registration string, request *BrowserLoginCallbackRequest, issuer string) (*StreamResponse, error) {
				if route.product {
					return client.CompletePluginOAuthProductCallback(t.Context(), registration, request, issuer)
				}
				return client.CompletePluginOAuthCallback(t.Context(), registration, request, issuer)
			}
			callbackPath := route.callbackPrefix + "/api/v1/plugins/oauth/callback/"
			request := &BrowserLoginCallbackRequest{State: "state +&%/中文", Code: "code +&%/?", Error: ""}
			issuer := "https://issuer.example.test/path?x=1&y=2"
			registrationID := "reg/ +%?中"
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == route.prefix+"/workspaces/ws/plugin-connections/pc/oauth" {
					if r.Method != http.MethodPost || r.Header.Get(headerAPIKey) != "begin-key" {
						t.Error("browser begin lost its authentication")
					}
					http.SetCookie(w, &http.Cookie{Name: "plugin-proof", Value: "bound-proof", Path: callbackPath, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
					_, _ = io.WriteString(w, `{"code":0,"data":{"attempt_id":"attempt"}}`)
					return
				}
				if r.Method != http.MethodGet || r.URL.EscapedPath() != callbackPath+url.PathEscape(registrationID) {
					t.Errorf("unexpected callback target: %s %s", r.Method, r.URL)
				}
				want := url.Values{"state": {request.State}, "code": {request.Code}, "error": {request.Error}, "iss": {issuer}}
				if !reflect.DeepEqual(r.URL.Query(), want) {
					t.Errorf("query changed: %v, want %v", r.URL.Query(), want)
				}
				for _, header := range []string{headerAPIKey, "Authorization", headerWorkspaceID, "X-MOI-Role-ID", headerRuntimeAuthorization, "X-Custom-Credential"} {
					if r.Header.Get(header) != "" {
						t.Errorf("callback forwarded %s", header)
					}
				}
				proof, err := r.Cookie("plugin-proof")
				if err != nil || proof.Value != "bound-proof" {
					w.WriteHeader(http.StatusForbidden)
					_, _ = io.WriteString(w, `{"code":"PLUGIN_OAUTH_STATE_INVALID","msg":"Browser proof is missing.","details":{"reason":"PLUGIN_OAUTH_STATE_INVALID"}}`)
					return
				}
				http.SetCookie(w, &http.Cookie{Name: "plugin-proof", Path: callbackPath, MaxAge: -1, Secure: true, HttpOnly: true})
				w.Header().Set("Content-Type", "text/html")
				w.Header().Set("X-Request-ID", "callback-request")
				_, _ = io.WriteString(w, "<html>authorized</html>")
			}))
			t.Cleanup(server.Close)
			httpClient := server.Client()
			httpClient.Jar, _ = cookiejar.New(nil)
			client, err := New(server.URL+route.prefix, WithAPIKey("begin-key"), WithToken("bearer-secret"), WithHTTPClient(httpClient),
				WithCustomHeader(headerWorkspaceID, "ws"), WithCustomHeader(headerRuntimeAuthorization, "runtime-secret"),
				WithCustomHeader("X-Custom-Credential", "custom-secret"), WithRequestModifier(func(r *http.Request) { r.Header.Set("Authorization", "modifier-secret") }))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Agents("ws").ResourcePost(t.Context(), "plugin-connections", "/pc/oauth", map[string]any{"initiator": "browser", "expected_version": 1})
			if err != nil {
				t.Fatal(err)
			}
			// The same jar must withhold the Secure proof on an HTTP callback.
			plainServer := httptest.NewServer(server.Config.Handler)
			t.Cleanup(plainServer.Close)
			plainClient, _ := New(plainServer.URL+route.prefix, WithUnauthenticated(), WithHTTPClient(httpClient))
			_, err = complete(plainClient, registrationID, request, issuer)
			var missingProof *Error
			if !errors.As(err, &missingProof) || missingProof.Code != "PLUGIN_OAUTH_STATE_INVALID" {
				t.Fatalf("Secure proof was sent over HTTP: %#v", err)
			}
			response, err := complete(client, registrationID, request, issuer)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if err != nil || response.StatusCode != 200 || response.Header.Get("X-Request-ID") != "callback-request" || string(body) != "<html>authorized</html>" {
				t.Fatalf("callback response changed: %+v body=%q err=%v", response, body, err)
			}
			_, err = complete(client, registrationID, request, issuer)
			var failure *Error
			if !errors.As(err, &failure) || failure.Code != "PLUGIN_OAUTH_STATE_INVALID" || failure.Status != 403 || failure.Details["reason"] != "PLUGIN_OAUTH_STATE_INVALID" {
				t.Fatalf("missing/consumed proof rejection changed: %#v", err)
			}
		})
	}
}

func TestCompletePluginOAuthCallbackDenialAndRedirect(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusFound} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Cookie") != "" {
					t.Error("callback forwarded the configured session cookie")
				}
				if r.URL.Path != "/api/v1/plugins/oauth/callback/registration" || r.URL.Query().Get("error") != "access_denied" || r.URL.Query().Has("iss") {
					t.Errorf("denial target/query changed: %s", r.URL)
				}
				w.Header().Set("Location", "/must-not-follow")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "denied")
			}))
			t.Cleanup(server.Close)
			client, _ := New(server.URL+"/newmoi", WithUnauthenticated(), WithSessionCookie("must-not-forward=secret"))
			response, err := client.CompletePluginOAuthCallback(t.Context(), "registration", &BrowserLoginCallbackRequest{State: "state", Error: "access_denied"}, "")
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != status || response.Header.Get("Location") != "/must-not-follow" {
				t.Fatalf("callback redirect changed: %+v", response)
			}
		})
	}
}

func TestCompletePluginOAuthCallbackErrors(t *testing.T) {
	for _, test := range []struct{ body, code, message string }{
		{`{"code":"PLUGIN_OAUTH_ISSUER_INVALID","msg":"签发者不匹配。","details":{"reason":"PLUGIN_OAUTH_ISSUER_INVALID"}}`, "PLUGIN_OAUTH_ISSUER_INVALID", "签发者不匹配。"},
		{`RAW-SECRET`, "ErrResponseInvalid", "product error response is invalid"},
	} {
		t.Run(test.code, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("iss") != "https://wrong.example.test" {
					t.Error("issuer was not forwarded")
				}
				w.Header().Set("X-Request-ID", "issuer-request")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, test.body)
			}))
			t.Cleanup(server.Close)
			client, _ := New(server.URL, WithUnauthenticated())
			_, err := client.CompletePluginOAuthCallback(t.Context(), "registration", &BrowserLoginCallbackRequest{Code: "code", State: "state"}, "https://wrong.example.test")
			var failure *Error
			if !errors.As(err, &failure) || failure.Code != test.code || failure.Status != 400 || failure.RequestID != "issuer-request" || strings.Contains(err.Error(), "RAW-SECRET") {
				t.Fatalf("callback error changed: %#v", err)
			}
			if test.code != "ErrResponseInvalid" && (failure.Message != test.message || failure.Details["reason"] != test.code) {
				t.Fatalf("owner error changed: %+v", failure)
			}
		})
	}
	client, _ := New("https://example.test", WithUnauthenticated())
	if _, err := client.CompletePluginOAuthCallback(t.Context(), "registration", nil, ""); err == nil {
		t.Fatal("nil request accepted")
	}
	for _, id := range []string{"", ".", ".."} {
		if _, err := client.CompletePluginOAuthCallback(t.Context(), id, &BrowserLoginCallbackRequest{}, ""); err == nil {
			t.Fatalf("invalid registration ID %q accepted", id)
		}
	}
	cause := errors.New("RAW-SECRET transport diagnostic")
	client, _ = New("https://example.test", WithUnauthenticated(), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, cause })}))
	_, err := client.CompletePluginOAuthCallback(t.Context(), "registration", &BrowserLoginCallbackRequest{}, "")
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "ErrTransportUnavailable" || !errors.Is(err, cause) || strings.Contains(err.Error(), "RAW-SECRET") {
		t.Fatalf("unsafe callback transport error: %#v", err)
	}
}

func TestCompletePluginOAuthCallbackRejectsEndpointCredentialsAndFragment(t *testing.T) {
	for _, endpoint := range []string{
		"https://user:password@example.test/newmoi",
		"https://user@example.test/newmoi",
		"https://@example.test/newmoi",
		"https://example.test/newmoi#fragment",
	} {
		t.Run(endpoint, func(t *testing.T) {
			requested := false
			client, err := New(endpoint, WithUnauthenticated(), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				requested = true
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("unexpected request"))}, nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.CompletePluginOAuthCallback(t.Context(), "registration", &BrowserLoginCallbackRequest{State: "state", Code: "code"}, "")
			if response != nil {
				_ = response.Body.Close()
			}
			if !errors.Is(err, ErrPathInvalid) || requested {
				t.Fatalf("unsafe callback endpoint was not rejected before network: err=%v requested=%v", err, requested)
			}
		})
	}
}

func TestPluginControlPlaneTypedRoutes(t *testing.T) {
	tests := []struct {
		name, method, path, response string
		call                         func(context.Context, *AgentService) (proto.Message, error)
	}{
		{name: "SearchPlugins", method: "GET", path: "/newmoi/workspaces/ws/plugin-discovery/search", response: `{"candidates":[{"source":"system","release":{"id":"pr_fixture","workspace_id":"system","registry":"builtin","publisher":"matrixorigin","name":"example","version":"1.0.0","digest":"sha256:test","description":"Example","published_by":"system","created_at":"2026-10-08T00:00:00Z"},"capabilities":[{"capability":{"id":"operation:read","name":"read","description":"Read","kind":"http","side_effect":"read","digest":"sha256:test"},"capability_ref":"pc1.fixture"}],"capabilities_truncated":false}],"has_more":true,"next_cursor":"next"}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.SearchPlugins(ctx, WithQueryParam("availability", "market"))
		}},
		{name: "DescribePluginCapability", method: "GET", path: "/newmoi/workspaces/ws/plugin-discovery/describe", response: `{"capability":{"id":"operation:read","name":"read","description":"Read","kind":"http","side_effect":"read","digest":"sha256:test"},"capability_ref":"pc1.fixture","input_schema":{"type":"object"},"connections":[{"requirement":"account","profiles":["oauth"]}]}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.DescribePluginCapability(ctx, "pc1.fixture")
		}},
		{name: "ListPluginReleases", method: "GET", path: "/newmoi/workspaces/ws/plugin-releases", response: `{"items":[{"id":"pr_fixture","workspace_id":"system","registry":"builtin","publisher":"matrixorigin","name":"example","version":"1.0.0","digest":"sha256:test","description":"Example","published_by":"system","created_at":"2026-10-08T00:00:00Z"}],"total":1}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.ListPluginReleases(ctx, WithQueryParam("namespace", "system"))
		}},
		{name: "GetPluginRelease", method: "GET", path: "/newmoi/workspaces/ws/plugin-releases/pr_fixture", response: `{"publication":{"id":"pr_fixture","workspace_id":"system","registry":"builtin","publisher":"matrixorigin","name":"example","version":"1.0.0","digest":"sha256:test","description":"Example","published_by":"system","created_at":"2026-10-08T00:00:00Z"},"manifest":{"$schema":"plugin-v1","name":"example","version":"1.0.0"},"connections":{"requirements":{}},"servers":{"main":{"type":"http","url":"https://api.example.test/mcp"}},"operations":{},"skills":[{"id":"skill:guide","name":"guide","description":"Guide","path":"skills/guide/SKILL.md","digest":"sha256:test"}]}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.GetPluginRelease(ctx, "pr_fixture", WithQueryParam("namespace", "system"))
		}},
		{name: "InstallPlugin", method: "POST", path: "/newmoi/workspaces/ws/plugin-installations", response: `{"id":"pi_1","workspace_id":"ws","release_workspace_id":"system","release_id":"pr_fixture","release_digest":"sha256:test","release_name":"example","owner_user_id":"user","bindings":{"account":{"connection_id":"pc_1","profile":"oauth"}},"version":7,"created_at":"2026-10-08T00:00:00Z","updated_at":"2026-10-08T00:00:00Z"}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.InstallPlugin(ctx, &PluginInstallRequest{ReleaseNamespace: "system", ReleaseId: "pr_fixture", Bindings: map[string]*PluginInstallationBinding{"account": {ConnectionId: "pc_1", Profile: "oauth"}}})
		}},
		{name: "ListPluginInstallations", method: "GET", path: "/newmoi/workspaces/ws/plugin-installations", response: `{"items":[{"id":"pi_1","workspace_id":"ws","release_workspace_id":"system","release_id":"pr_fixture","release_digest":"sha256:test","release_name":"example","owner_user_id":"user","bindings":{"account":{"connection_id":"pc_1","profile":"oauth"}},"version":7,"created_at":"2026-10-08T00:00:00Z","updated_at":"2026-10-08T00:00:00Z"}],"total":1}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.ListPluginInstallations(ctx)
		}},
		{name: "GetPluginInstallation", method: "GET", path: "/newmoi/workspaces/ws/plugin-installations/pi_1", response: `{"id":"pi_1","workspace_id":"ws","release_workspace_id":"system","release_id":"pr_fixture","release_digest":"sha256:test","release_name":"example","owner_user_id":"user","bindings":{"account":{"connection_id":"pc_1","profile":"oauth"}},"version":7,"created_at":"2026-10-08T00:00:00Z","updated_at":"2026-10-08T00:00:00Z"}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.GetPluginInstallation(ctx, "pi_1")
		}},
		{name: "UpdatePluginInstallation", method: "PATCH", path: "/newmoi/workspaces/ws/plugin-installations/pi_1", response: `{"id":"pi_1","workspace_id":"ws","release_workspace_id":"system","release_id":"pr_fixture","release_digest":"sha256:test","release_name":"example","owner_user_id":"user","bindings":{"account":{"connection_id":"pc_1","profile":"oauth"}},"version":7,"created_at":"2026-10-08T00:00:00Z","updated_at":"2026-10-08T00:00:00Z"}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.UpdatePluginInstallation(ctx, "pi_1", &PluginUpdateInstallationRequest{ExpectedVersion: 7, ReleaseNamespace: "system", ReleaseId: "pr_fixture", Bindings: map[string]*PluginInstallationBinding{"account": {ConnectionId: "pc_1", Profile: "oauth"}}})
		}},
		{name: "DeletePluginInstallation", method: "DELETE", path: "/newmoi/workspaces/ws/plugin-installations/pi_1", response: `{"status":"deleted"}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.DeletePluginInstallation(ctx, "pi_1", 7)
		}},
		{name: "CreatePluginConnection", method: "POST", path: "/newmoi/workspaces/ws/plugin-connections", response: `{"id":"pc_1","workspace_id":"ws","name":"Example","status":"active","kind":"plugin","endpoint_uri":"https://api.example.test","auth_type":"oauth2","owner_user_id":"user","visibility":"private","version":3,"config":{"provider":"example"},"created_at":"2026-10-08T00:00:00Z","updated_at":"2026-10-08T00:00:00Z"}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.CreatePluginConnection(ctx, &PluginCreateConnectionRequest{ReleaseNamespace: "system", ReleaseId: "pr_fixture", Requirement: "account", Profile: "oauth", Name: "Example"})
		}},
		{name: "ListPluginConnections", method: "GET", path: "/newmoi/workspaces/ws/plugin-connections", response: `{"items":[{"id":"pc_1","workspace_id":"ws","name":"Example","status":"active","kind":"plugin","endpoint_uri":"https://api.example.test","auth_type":"oauth2","owner_user_id":"user","visibility":"private","version":3,"config":{"provider":"example"},"created_at":"2026-10-08T00:00:00Z","updated_at":"2026-10-08T00:00:00Z"}],"total":1}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) { return a.ListPluginConnections(ctx) }},
		{name: "GetPluginConnection", method: "GET", path: "/newmoi/workspaces/ws/plugin-connections/pc_1", response: `{"id":"pc_1","workspace_id":"ws","name":"Example","status":"active","kind":"plugin","endpoint_uri":"https://api.example.test","auth_type":"oauth2","owner_user_id":"user","visibility":"private","version":3,"config":{"provider":"example"},"created_at":"2026-10-08T00:00:00Z","updated_at":"2026-10-08T00:00:00Z"}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.GetPluginConnection(ctx, "pc_1")
		}},
		{name: "UpdatePluginConnection", method: "PATCH", path: "/newmoi/workspaces/ws/plugin-connections/pc_1", response: `{"id":"pc_1","workspace_id":"ws","name":"Example","status":"active","kind":"plugin","endpoint_uri":"https://api.example.test","auth_type":"oauth2","owner_user_id":"user","visibility":"private","version":3,"config":{"provider":"example"},"created_at":"2026-10-08T00:00:00Z","updated_at":"2026-10-08T00:00:00Z"}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.UpdatePluginConnection(ctx, "pc_1", &PluginUpdateConnectionRequest{ExpectedVersion: 3, Status: "disabled"})
		}},
		{name: "DeletePluginConnection", method: "DELETE", path: "/newmoi/workspaces/ws/plugin-connections/pc_1", response: `{"status":"deleted"}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.DeletePluginConnection(ctx, "pc_1", 3)
		}},
		{name: "GetPluginAuthorization", method: "GET", path: "/newmoi/workspaces/ws/plugin-connections/pc_1/authorization", response: `{"workspace_id":"ws","connection_id":"pc_1","external_subject":"external-user","account_display_name":"张小明","status":"ready","generation":2,"revision":4,"connection_version":3}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.GetPluginAuthorization(ctx, "pc_1")
		}},
		{name: "StorePluginSecret", method: "POST", path: "/newmoi/workspaces/ws/plugin-connections/pc_1/secret", response: `{"workspace_id":"ws","connection_id":"pc_1","external_subject":"external-user","status":"ready","generation":2,"revision":4,"connection_version":3}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.StorePluginSecret(ctx, "pc_1", &PluginStoreSecretRequest{ExpectedVersion: 3, Fields: map[string]string{"api_key": "test-only-secret"}})
		}},
		{name: "BeginPluginOAuth", method: "POST", path: "/newmoi/workspaces/ws/plugin-connections/pc_1/oauth", response: `{"attempt_id":"oa_1","grant":"device_code","authorization_url":"https://example.test/device","user_code":"ABCD","poll_after":5,"expires_at":"2026-10-08T00:15:00Z"}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.BeginPluginOAuth(ctx, "pc_1", 3)
		}},
		{name: "PollPluginOAuth", method: "POST", path: "/newmoi/workspaces/ws/plugin-connections/pc_1/oauth/oa_1/poll", response: `{"attempt_id":"oa_1","status":"pending","reason":"authorization_pending","poll_after":10}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.PollPluginOAuth(ctx, "pc_1", "oa_1", 3)
		}},
		{name: "PollPluginOAuthNextAction", method: "POST", path: "/newmoi/workspaces/ws/plugin-connections/pc_1/oauth/oa_1/poll", response: `{"attempt_id":"oa_1","status":"pending","poll_after":5,"next_action":{"attempt_id":"oa_1","grant":"device_code","authorization_url":"https://accounts.feishu.cn/oauth/v1/authorize?user_code=ABCD","user_code":"ABCD","poll_after":5,"expires_at":"2026-10-08T00:15:00Z"}}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.PollPluginOAuth(ctx, "pc_1", "oa_1", 3)
		}},
		{name: "DisconnectPluginConnection", method: "POST", path: "/newmoi/workspaces/ws/plugin-connections/pc_1/disconnect", response: `{"status":"disconnected"}`, call: func(ctx context.Context, a *AgentService) (proto.Message, error) {
			return a.DisconnectPluginConnection(ctx, "pc_1", 2)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != test.method || r.URL.Path != test.path {
					t.Errorf("route: %s %s, want %s %s", r.Method, r.URL.Path, test.method, test.path)
				}
				if r.Header.Get("X-Workspace-ID") != "ws" {
					t.Error("missing workspace scope")
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if len(body) > 0 {
					var payload map[string]any
					if err := json.Unmarshal(body, &payload); err != nil {
						t.Error(err)
					}
					if version, ok := payload["expected_version"]; ok {
						if _, ok := version.(float64); !ok {
							t.Errorf("version is not JSON number: %T", version)
						}
					}
					if test.name == "InstallPlugin" || test.name == "UpdatePluginInstallation" {
						bindings := payload["bindings"].(map[string]any)
						account := bindings["account"].(map[string]any)
						if account["connection_id"] != "pc_1" || account["profile"] != "oauth" {
							t.Errorf("wrong binding: %+v", account)
						}
					}
					if test.name == "BeginPluginOAuth" && payload["initiator"] != "external" {
						t.Error("SDK must request a browser handoff instead of a cookie-less code attempt")
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"code":"OK","data":`+test.response+`}`)
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("test-user-key"))
			if err != nil {
				t.Fatal(err)
			}
			result, err := test.call(t.Context(), client.Agents("ws"))
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var want, got any
			if err := json.Unmarshal([]byte(test.response), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			assertPluginWireFields(t, want, got)
			if requests != 1 {
				t.Fatalf("result changed or replayed: %+v requests=%d", got, requests)
			}
		})
	}
}

func TestPluginMutationsRequireValidSuccessEnvelope(t *testing.T) {
	for _, body := range []string{"", `null`, `RAW-SECRET`, `{"status":"disconnected"}`,
		`{"code":null,"data":{}}`, `{"code":true,"data":{}}`, `{"code":"","data":{}}`,
		`{"code":200.5,"data":{}}`, `{"code":0,"msg":null,"data":{}}`,
		`{"code":0,"data":null}`, `{"code":0,"data":[]}`, `{"code":0,"data":{"unknown":true}}`,
	} {
		for _, operation := range []string{"disconnect", "create"} {
			t.Run(operation+body, func(t *testing.T) {
				requests := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					requests++
					w.Header().Set("X-Request-ID", "plugin-request")
					_, _ = io.WriteString(w, body)
				}))
				t.Cleanup(server.Close)
				client, err := New(server.URL+"/newmoi", WithAPIKey("test-user-key"))
				if err != nil {
					t.Fatal(err)
				}
				agents := client.Agents("ws")
				switch operation {
				case "disconnect":
					_, err = agents.DisconnectPluginConnection(t.Context(), "connection", 1)
				case "create":
					_, err = agents.CreatePluginConnection(t.Context(), &PluginCreateConnectionRequest{})
				}
				var failure *Error
				if !errors.As(err, &failure) || failure.Code != "ErrResponseInvalid" || failure.Status != 200 || failure.RequestID != "plugin-request" {
					t.Fatalf("invalid response classification: %#v", err)
				}
				if strings.Contains(failure.Error(), "RAW-SECRET") {
					t.Fatal("response diagnostic leaked")
				}
				if requests != 1 {
					t.Fatalf("mutation replayed %d times", requests)
				}
			})
		}
	}
}

func TestPluginPublicationPreservesLocalizedDisplayAndCanonicalName(t *testing.T) {
	for _, name := range []string{"企业微信", "WeCom"} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"publication": map[string]string{
					"name": "wecom", "description": "package metadata", "display_name": name, "display_description": "localized description",
				}}})
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL, WithAPIKey("test"))
			if err != nil {
				t.Fatal(err)
			}
			release, err := client.Agents("ws").GetPluginRelease(t.Context(), "pr_fixture", WithQueryParam("namespace", "system"))
			if err != nil || release.GetPublication().GetName() != "wecom" || release.GetPublication().GetDescription() != "package metadata" ||
				release.GetPublication().GetDisplayName() != name || release.GetPublication().GetDisplayDescription() != "localized description" {
				t.Fatalf("publication display contract changed: %+v, %v", release, err)
			}
		})
	}
}

func TestPluginErrorsPreserveOwnerContractAndDistinguishInvalidResponse(t *testing.T) {
	for _, locale := range []string{"zh-CN", "en-US"} {
		t.Run(locale, func(t *testing.T) {
			message := "The connection changed. Refresh and try again."
			if locale == "zh-CN" {
				message = "连接已更新，请刷新后再试。"
			}
			body, _ := json.Marshal(map[string]any{"code": 4, "msg": message, "details": map[string]string{"reason": "PLUGIN_STATE_CONFLICT", "domain": "moi-core.agent-plugin"}})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Accept-Language") != locale {
					t.Error("locale was not propagated")
				}
				w.Header().Set("X-Request-ID", "plugin-error-request")
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write(body)
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL, WithAPIKey("test"), WithCustomHeader("Accept-Language", locale))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Agents("ws").GetPluginInstallation(t.Context(), "installation")
			var failure *Error
			if !errors.As(err, &failure) || failure.Code != "4" || failure.Message != message || failure.Status != 409 || failure.RequestID != "plugin-error-request" || failure.Details["reason"] != "PLUGIN_STATE_CONFLICT" {
				t.Fatalf("owner error was changed: %+v", err)
			}
		})
	}
	for _, body := range []string{"", `not-json RAW-SECRET`, `null`, `[]`, `{"msg":"RAW-SECRET"}`, `{"code":4,"msg":"safe","details":{"reason":false}}`, `{"code":0,"msg":"safe"}`, `{"code":4.5,"msg":"safe"}`} {
		failure := parseHTTPError(502, []byte(body), "request-1")
		var typed *Error
		if !errors.As(failure, &typed) || typed.Code != "ErrResponseInvalid" || typed.Status != 502 || typed.RequestID != "request-1" || strings.Contains(typed.Message, "RAW-SECRET") {
			t.Fatalf("malformed error must not become business failure: %+v", failure)
		}
	}
	cause := errors.New("private transport diagnostic RAW-SECRET")
	client, err := New("https://example.test", WithAPIKey("test"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, cause })}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Agents("ws").GetPluginInstallation(t.Context(), "installation")
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "ErrTransportUnavailable" || failure.Status != 0 || !errors.Is(err, cause) || strings.Contains(err.Error(), "RAW-SECRET") {
		t.Fatalf("transport must be explicit and diagnostic retained privately: %+v", err)
	}
}

func assertPluginWireFields(t *testing.T, expected, actual any) {
	t.Helper()
	switch expected := expected.(type) {
	case map[string]any:
		got, ok := actual.(map[string]any)
		if !ok {
			t.Fatalf("expected object, got %T", actual)
		}
		for key, value := range expected {
			field, exists := got[key]
			if !exists {
				t.Fatalf("response field %s disappeared", key)
			}
			assertPluginWireFields(t, value, field)
		}
	case []any:
		got, ok := actual.([]any)
		if !ok || len(expected) != len(got) {
			t.Fatalf("array changed: %v -> %v", expected, actual)
		}
		for i, value := range expected {
			assertPluginWireFields(t, value, got[i])
		}
	case float64:
		if text, ok := actual.(string); ok {
			if text != strconv.FormatFloat(expected, 'f', -1, 64) {
				t.Fatalf("number changed: %v -> %v", expected, actual)
			}
		} else if actual != expected {
			t.Fatalf("number changed: %v -> %v", expected, actual)
		}
	default:
		if !reflect.DeepEqual(expected, actual) {
			t.Fatalf("field changed: %v -> %v", expected, actual)
		}
	}
}

func TestPluginOAuthPreservesErrorWithoutReplay(t *testing.T) {
	for _, message := range []string{"Connection is busy. Wait and try again.", "连接正在授权中，请稍后再试。"} {
		t.Run(message, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("X-Request-ID", "plugin-request")
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{"code": "PLUGIN_AUTH_IN_PROGRESS", "msg": message, "details": map[string]string{"reason": "PLUGIN_AUTH_IN_PROGRESS"}})
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("test-user-key"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Agents("ws").PollPluginOAuth(t.Context(), "pc_1", "oa_1", 3)
			var failure *Error
			if !errors.As(err, &failure) || failure.Code != "PLUGIN_AUTH_IN_PROGRESS" || failure.Message != message || failure.RequestID != "plugin-request" || failure.Details["reason"] != "PLUGIN_AUTH_IN_PROGRESS" || requests != 1 {
				t.Fatalf("error contract changed: %+v requests=%d", failure, requests)
			}
		})
	}
}
