package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type capturedRoleRequest struct {
	path            string
	workspaceIDs    []string
	roleIDs         []string
	customHeaders   []string
	modifierHeaders []string
	apiKeys         []string
	authorization   []string
	cookies         []string
}

func TestWorkspaceCurrentRoleTransportRestoresTypedScope(t *testing.T) {
	requests := make(chan capturedRoleRequest, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- capturedRoleRequest{
			path:          r.URL.Path,
			workspaceIDs:  r.Header.Values(headerWorkspaceID),
			roleIDs:       r.Header.Values(headerMOIRoleID),
			apiKeys:       r.Header.Values(headerAPIKey),
			authorization: r.Header.Values("Authorization"),
			cookies:       r.Header.Values("Cookie"),
		}
		if r.URL.Path == "/stream" {
			w.Header().Set("Content-Type", acceptEventStream)
			_, _ = io.WriteString(w, "data: {}\n\n")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client, err := NewWithPersonalAccessToken(
		server.URL,
		"opaque-pat",
		WithWorkspaceCurrentRoleID("ws-a", "12"),
		WithCustomHeader(headerWorkspaceID, "ws-other"),
		WithCustomHeader(headerMOIRoleID, "99"),
		WithRequestModifier(func(req *http.Request) {
			req.Header["X-Workspace-ID"] = []string{"ws-added"}
			req.Header["x-moi-role-id"] = []string{"98"}
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := client.do(context.Background(), http.MethodGet, "/json", "ws-a", nil, &result); err != nil {
		t.Fatal(err)
	}
	stream, err := client.doStream(context.Background(), http.MethodGet, "/stream", "ws-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if err := client.doRaw(context.Background(), http.MethodPost, "/raw", "ws-a", bytes.NewBufferString("payload"), "text/plain", contentTypeJSON, &result); err != nil {
		t.Fatal(err)
	}

	for range 3 {
		assertCapturedTypedScope(t, <-requests, "ws-a", "12")
	}

	websocketHeaders := client.SQL("ws-a").QueryWebSocketHeaders()
	assertHeaderValues(t, websocketHeaders, headerWorkspaceID, []string{"ws-a"})
	assertHeaderValues(t, websocketHeaders, headerMOIRoleID, []string{"12"})
}

func TestWorkspaceCurrentRoleTransportReservesScopeHeadersWithoutTypedMapping(t *testing.T) {
	requests := make(chan capturedRoleRequest, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- capturedRoleRequest{
			path:            r.URL.Path,
			workspaceIDs:    r.Header.Values(headerWorkspaceID),
			roleIDs:         r.Header.Values(headerMOIRoleID),
			customHeaders:   r.Header.Values("X-Custom-Header"),
			modifierHeaders: r.Header.Values("X-Modifier-Header"),
		}
		if r.URL.Path == "/stream" {
			w.Header().Set("Content-Type", acceptEventStream)
			_, _ = io.WriteString(w, "data: {}\n\n")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client, err := NewWithPersonalAccessToken(
		server.URL,
		"opaque-pat",
		WithCustomHeader(headerWorkspaceID, "custom-workspace"),
		WithCustomHeader(headerMOIRoleID, "99"),
		WithCustomHeader("X-Custom-Header", "custom-value"),
		WithRequestModifier(func(req *http.Request) {
			req.Header["x-workspace-id"] = []string{"modified-workspace"}
			req.Header["x-moi-role-id"] = []string{"98"}
			req.Header.Set("X-Modifier-Header", "modifier-value")
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := client.do(context.Background(), http.MethodGet, "/json", "ws-a", nil, &result); err != nil {
		t.Fatal(err)
	}
	stream, err := client.doStream(context.Background(), http.MethodGet, "/stream", "ws-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if err := client.doRaw(context.Background(), http.MethodPost, "/raw", "ws-a", bytes.NewBufferString("payload"), "text/plain", contentTypeJSON, &result); err != nil {
		t.Fatal(err)
	}

	for range 3 {
		captured := <-requests
		if got := captured.workspaceIDs; len(got) != 1 || got[0] != "ws-a" {
			t.Fatalf("%s workspace headers = %q, want [ws-a]", captured.path, got)
		}
		if len(captured.roleIDs) != 0 {
			t.Fatalf("%s role headers = %q, want none", captured.path, captured.roleIDs)
		}
		if got := captured.customHeaders; len(got) != 1 || got[0] != "custom-value" {
			t.Fatalf("%s custom headers = %q, want [custom-value]", captured.path, got)
		}
		if got := captured.modifierHeaders; len(got) != 1 || got[0] != "modifier-value" {
			t.Fatalf("%s modifier headers = %q, want [modifier-value]", captured.path, got)
		}
	}

	websocketHeaders := client.SQL("ws-a").QueryWebSocketHeaders()
	assertHeaderValues(t, websocketHeaders, headerWorkspaceID, []string{"ws-a"})
	assertHeaderValues(t, websocketHeaders, headerMOIRoleID, nil)
	assertHeaderValues(t, websocketHeaders, "X-Custom-Header", []string{"custom-value"})
	assertHeaderValues(t, websocketHeaders, "X-Modifier-Header", nil)
}

func TestWorkspaceCurrentRoleTransportSelectsByWorkspace(t *testing.T) {
	requests := make(chan capturedRoleRequest, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- capturedRoleRequest{
			workspaceIDs: r.Header.Values(headerWorkspaceID),
			roleIDs:      r.Header.Values(headerMOIRoleID),
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client, err := NewWithPersonalAccessToken(
		server.URL,
		"opaque-pat",
		WithWorkspaceCurrentRoleID("ws-a", "12"),
		WithWorkspaceCurrentRoleID("ws-b", "27"),
	)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := client.do(context.Background(), http.MethodGet, "/a", "ws-a", nil, &result); err != nil {
		t.Fatal(err)
	}
	if err := client.do(context.Background(), http.MethodGet, "/b", "ws-b", nil, &result); err != nil {
		t.Fatal(err)
	}
	assertCapturedTypedScope(t, <-requests, "ws-a", "12")
	assertCapturedTypedScope(t, <-requests, "ws-b", "27")
}

func TestWorkspaceCurrentRoleTransportClearsRoleOutsideTypedWorkspace(t *testing.T) {
	requests := make(chan capturedRoleRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- capturedRoleRequest{
			workspaceIDs: r.Header.Values(headerWorkspaceID),
			roleIDs:      r.Header.Values(headerMOIRoleID),
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client, err := NewWithPersonalAccessToken(
		server.URL,
		"opaque-pat",
		WithWorkspaceCurrentRoleID("ws-a", "12"),
		WithCustomHeader(headerMOIRoleID, "99"),
	)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := client.do(context.Background(), http.MethodGet, "/b", "ws-b", nil, &result); err != nil {
		t.Fatal(err)
	}
	captured := <-requests
	if got := captured.workspaceIDs; len(got) != 1 || got[0] != "ws-b" {
		t.Fatalf("workspace headers = %q, want [ws-b]", got)
	}
	if len(captured.roleIDs) != 0 {
		t.Fatalf("role headers = %q, want none", captured.roleIDs)
	}
}

func TestWorkspaceCurrentRoleTransportClearsScopeWithoutWorkspace(t *testing.T) {
	requests := make(chan capturedRoleRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- capturedRoleRequest{
			workspaceIDs: r.Header.Values(headerWorkspaceID),
			roleIDs:      r.Header.Values(headerMOIRoleID),
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client, err := NewWithPersonalAccessToken(
		server.URL,
		"opaque-pat",
		WithCustomHeader(headerWorkspaceID, "ws-other"),
		WithCustomHeader(headerMOIRoleID, "99"),
	)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := client.do(context.Background(), http.MethodGet, "/account", "", nil, &result); err != nil {
		t.Fatal(err)
	}
	captured := <-requests
	if len(captured.workspaceIDs) != 0 {
		t.Fatalf("workspace headers = %q, want none", captured.workspaceIDs)
	}
	if len(captured.roleIDs) != 0 {
		t.Fatalf("role headers = %q, want none", captured.roleIDs)
	}
}

func assertCapturedTypedScope(t *testing.T, captured capturedRoleRequest, workspaceID, roleID string) {
	t.Helper()
	if got := captured.workspaceIDs; len(got) != 1 || got[0] != workspaceID {
		t.Fatalf("%s workspace headers = %q, want [%q]", captured.path, got, workspaceID)
	}
	if got := captured.roleIDs; len(got) != 1 || got[0] != roleID {
		t.Fatalf("%s role headers = %q, want [%q]", captured.path, got, roleID)
	}
	if len(captured.apiKeys) > 0 && (len(captured.apiKeys) != 1 || captured.apiKeys[0] != "opaque-pat") {
		t.Fatalf("%s api key headers = %q", captured.path, captured.apiKeys)
	}
	if len(captured.authorization) != 0 || len(captured.cookies) != 0 {
		t.Fatalf("%s mixed credentials: authorization=%q cookie=%q", captured.path, captured.authorization, captured.cookies)
	}
}

func assertHeaderValues(t *testing.T, header http.Header, key string, want []string) {
	t.Helper()
	got := header.Values(key)
	if len(got) != len(want) {
		t.Fatalf("%s headers = %q, want %q", key, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("%s headers = %q, want %q", key, got, want)
		}
	}
}
