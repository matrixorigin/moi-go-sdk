package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestIAMCurrentPrincipalCapabilitiesQueryAndResponse(t *testing.T) {
	for _, test := range []struct {
		name string
		ids  []string
	}{
		{name: "omitted"},
		{name: "empty list", ids: []string{}},
		{name: "single", ids: []string{"action.role.delete"}},
		{name: "batch", ids: []string{"action.role.delete", "action.model.update"}},
		{name: "empty value is transmitted", ids: []string{""}},
		{name: "duplicates are transmitted", ids: []string{"navigation.roles", "navigation.roles"}},
		{name: "ID is encoded without rewriting", ids: []string{"navigation.roles&capability_id=other +/"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var received []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.EscapedPath() != "/newmoi/workspaces/ws%2F1/iam/me/capabilities" || r.Header.Get("X-MOI-Role-ID") != "7" {
					t.Errorf("unexpected capability request: %s %s role=%q", r.Method, r.URL, r.Header.Get("X-MOI-Role-ID"))
				}
				received = r.URL.Query()["capability_id"]
				if len(test.ids) == 0 && r.URL.RawQuery != "" {
					t.Errorf("omitted selection has query %q", r.URL.RawQuery)
				}
				_, _ = w.Write([]byte(`{"code":"OK","msg":"OK","data":{"effective_role_id":"7","capability_version":"v1","request_id":"request-1","trace_id":"trace-1","items":[{"capability_id":"action.model.update","action_id":"model_backend.update","resource_type":"model_backend","allowed":true,"all_resources":false,"resource_ids":["llm:7"]},{"capability_id":"action.role.delete","action_id":"role.delete","resource_type":"workspace","allowed":false,"reason_code":"PERMISSION_DENIED","resource_ids":[]}]}}`))
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("key"), WithWorkspaceCurrentRoleID("ws/1", "7"))
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.IAM("ws/1").CurrentPrincipalCapabilities(t.Context(), test.ids...)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(received, test.ids) {
				t.Fatalf("capability IDs = %q, want %q", received, test.ids)
			}
			if result.GetEffectiveRoleId() != "7" || result.GetCapabilityVersion() != "v1" || result.GetRequestId() != "request-1" || result.GetTraceId() != "trace-1" {
				t.Fatalf("capability metadata = %v", result)
			}
			var ids []string
			for _, item := range result.GetItems() {
				ids = append(ids, item.GetCapabilityId())
			}
			if !slices.Equal(ids, []string{"action.model.update", "action.role.delete"}) {
				t.Fatalf("capability response IDs = %v", ids)
			}
			modelUpdate, roleDelete := result.GetItems()[0], result.GetItems()[1]
			if !modelUpdate.GetAllowed() || modelUpdate.GetAllResources() || modelUpdate.GetActionId() != "model_backend.update" || modelUpdate.GetResourceType() != "model_backend" ||
				!slices.Equal(modelUpdate.GetResourceIds(), []string{"llm:7"}) || roleDelete.GetAllowed() || roleDelete.GetReasonCode() != "PERMISSION_DENIED" {
				t.Fatalf("capability scope or denial changed: %v", result.GetItems())
			}
		})
	}
}

func TestIAMCurrentPrincipalCapabilitiesPreservesServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !slices.Equal(r.URL.Query()["capability_id"], []string{"unknown"}) {
			t.Errorf("selection = %v", r.URL.Query())
		}
		w.Header().Set("X-Request-ID", "public-request")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"IAM_ERROR_CODE_PERMISSION_DENIED","msg":"Permission denied","data":null}`))
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.IAM("ws-1").CurrentPrincipalCapabilities(t.Context(), "unknown")
	var productErr *Error
	if !errors.As(err, &productErr) || productErr.Status != http.StatusForbidden || productErr.Code != "IAM_ERROR_CODE_PERMISSION_DENIED" || productErr.Message != "Permission denied" || productErr.RequestID != "public-request" {
		t.Fatalf("server error = %#v", err)
	}
}

func TestIAMCurrentPrincipalCapabilitiesUnsupportedIDs(t *testing.T) {
	for _, test := range []struct {
		name    string
		ids     []string
		items   string
		wantIDs []string
	}{
		{name: "mixed", ids: []string{"future.id", "action.role.create"}, items: `[{"capability_id":"action.role.create","allowed":false,"reason_code":"PERMISSION_DENIED"}]`, wantIDs: []string{"action.role.create"}},
		{name: "all unsupported", ids: []string{"future.id"}, items: `[]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !slices.Equal(r.URL.Query()["capability_id"], test.ids) {
					t.Errorf("selection = %v", r.URL.Query())
				}
				fmt.Fprintf(w, `{"code":"OK","msg":"OK","data":{"effective_role_id":"7","items":%s,"unsupported_capability_ids":["future.id"]}}`, test.items)
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.IAM("ws-1").CurrentPrincipalCapabilities(t.Context(), test.ids...)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, item := range result.GetItems() {
				ids = append(ids, item.GetCapabilityId())
			}
			if !slices.Equal(ids, test.wantIDs) || !slices.Equal(result.GetUnsupportedCapabilityIds(), []string{"future.id"}) || result.GetEffectiveRoleId() != "7" {
				t.Fatalf("partial capability result = %v", result)
			}
		})
	}
}

func TestIAMServiceUsesTypedWorkspaceRoutesAndCASBodies(t *testing.T) {
	type capturedRequest struct {
		method string
		path   string
		query  string
		body   map[string]any
	}
	requests := make([]capturedRequest, 0, 11)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, capturedRequest{r.Method, r.URL.Path, r.URL.RawQuery, body})
		data := iamTestResponse(r.Method, r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
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
	iam := workspace.IAM()
	ctx := t.Context()

	if _, err := iam.PermissionSchema(ctx); err != nil {
		t.Fatal(err)
	}
	roles, err := iam.Roles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles.GetRoles()) != 1 {
		t.Fatalf("roles = %#v, want one system role", roles)
	}
	if got := roles.GetRoles()[0].GetSystemRoleName(); got != "admin" {
		t.Fatalf("system role name = %q, want admin", got)
	}
	created, err := iam.CreateRole(ctx, &IAMRoleCreateRequest{
		Name: "analyst", SchemaVersion: "v1", RequestId: "create-1",
		Statements: []*IAMApplicationPolicyStatement{{ActionId: "workflow.run", ResourceType: "workflow", ScopeMode: "all"}},
		DataPrivilegeMutations: []*IAMDataPrivilegeMutation{{
			Operation: "grant", ActionId: "table.query", ResourceType: "table", ResourceId: "41", Privilege: "select",
			RowColumn: &IAMDataPrivilegeRowColumnMutation{Mode: "keep", Draft: &IAMDataPrivilegeRowColumnDraft{}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.GetRoleId() != "role-1" || created.GetDataPrivilegeVersion() != 4 || created.GetIdempotentReplay() || created.GetSessionInvalidationStatus() != "succeeded" {
		t.Fatalf("create result = %#v", created)
	}
	permissionView, err := iam.RolePermissionView(ctx, "role-1")
	if err != nil {
		t.Fatal(err)
	}
	if permissionView.GetExpectedRoleLifecycleVersion() != 3 || permissionView.GetLifecycleState() != "disabled" || permissionView.GetPolicyMutationAllowed() {
		t.Fatalf("role lifecycle CAS version = %d", permissionView.GetExpectedRoleLifecycleVersion())
	}
	updated, err := iam.UpdateRole(ctx, "role-1", &IAMRoleUpdateRequest{
		Name: "renamed", Description: "updated role", SchemaVersion: "v1", RequestId: "update-1",
		ExpectedLifecycleVersion: 3, ExpectedPolicyVersion: 7, ExpectedDataPrivilegeVersion: 4,
		Statements:             []*IAMApplicationPolicyStatement{{ActionId: "workflow.run", ResourceType: "workflow", ScopeMode: "all"}},
		DataPrivilegeMutations: iamDataPrivilegeTestMutationRequest("unused", 4).Mutations,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.GetRoleId() != "role-1" || updated.GetRoleVersion() != 4 || updated.GetPolicyVersion() != 8 || updated.GetDataPrivilegeVersion() != 5 || updated.GetProvisioningState() != "ready" || updated.GetProvisioningVersion() != 2 || updated.GetProvisioningErrorCode() != "" || !updated.GetIdempotentReplay() || updated.GetSessionInvalidationStatus() != "succeeded" || updated.GetRequestId() != "response-request" || updated.GetTraceId() != "response-trace" {
		t.Fatalf("aggregate update result = %#v", updated)
	}
	if _, err := iam.UpdateRoleLifecycle(ctx, "role-1", &IAMRoleLifecycleRequest{TargetState: "disabled", ReasonCode: "test", ExpectedLifecycleVersion: 4, RequestId: "lifecycle-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := iam.RoleDeleteImpact(ctx, "role-1", 5); err != nil {
		t.Fatal(err)
	}
	if _, err := iam.ValidateObjectGrants(ctx, "connector", "connector-1", "role-1", iamTestGrantRequest("validate-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := iam.ApplyObjectGrants(ctx, "connector", "connector-1", "role-1", iamTestGrantRequest("apply-1")); err != nil {
		t.Fatal(err)
	}
	objectView, err := iam.ObjectPermissionView(ctx, "connector", "connector-1")
	if err != nil {
		t.Fatal(err)
	}
	if objectView.GetOwner().GetRoleLifecycleState() != "disabled" || len(objectView.GetDirectGrants()) != 1 || len(objectView.GetAllScopeGrants()) != 1 {
		t.Fatalf("object view lost lifecycle metadata or sources: %v", objectView)
	}
	if objectView.GetDirectGrants()[0].GetRoleLifecycleState() != "disabled" || objectView.GetAllScopeGrants()[0].GetRoleLifecycleState() != "active" {
		t.Fatalf("object grant source lifecycle metadata: %v", objectView)
	}
	if _, err := iam.DeleteRole(ctx, "role-1", &IAMRoleDeleteRequest{ReasonCode: "cleanup", ExpectedLifecycleVersion: 5, RequestId: "delete-1"}); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 11 {
		t.Fatalf("requests = %d, want 11", len(requests))
	}
	assertIAMRequest(t, requests[0], http.MethodGet, "/newmoi/workspaces/ws-1/iam/permissions/schema")
	assertIAMRequest(t, requests[2], http.MethodPost, "/newmoi/workspaces/ws-1/iam/roles")
	if statements, ok := requests[2].body["statements"].([]any); !ok || len(statements) != 1 {
		t.Fatalf("create-role statements = %#v, want one", requests[2].body["statements"])
	}
	if mutations, ok := requests[2].body["data_privilege_mutations"].([]any); !ok || len(mutations) != 1 {
		t.Fatalf("create-role data privileges = %#v, want one", requests[2].body["data_privilege_mutations"])
	}
	assertIAMRequest(t, requests[4], http.MethodPatch, "/newmoi/workspaces/ws-1/iam/roles/role-1")
	if requests[4].body["expected_lifecycle_version"] != float64(3) || requests[4].body["expected_policy_version"] != float64(7) || requests[4].body["expected_data_privilege_version"] != float64(4) || requests[4].body["schema_version"] != "v1" || requests[4].body["description"] != "updated role" || requests[4].body["request_id"] != "update-1" {
		t.Fatalf("update body = %#v", requests[4].body)
	}
	if statements, ok := requests[4].body["statements"].([]any); !ok || len(statements) != 1 || statements[0].(map[string]any)["action_id"] != "workflow.run" {
		t.Fatalf("update statements = %#v", requests[4].body["statements"])
	}
	if mutations, ok := requests[4].body["data_privilege_mutations"].([]any); !ok || len(mutations) != 1 || mutations[0].(map[string]any)["resource_id"] != "table/1" {
		t.Fatalf("update data privileges = %#v", requests[4].body["data_privilege_mutations"])
	}
	assertIAMRequest(t, requests[6], http.MethodGet, "/newmoi/workspaces/ws-1/iam/roles/role-1/delete-impact")
	if requests[6].query != "expected_lifecycle_version=5" {
		t.Fatalf("delete impact query = %q", requests[6].query)
	}
	assertIAMRequest(t, requests[7], http.MethodPost, "/newmoi/workspaces/ws-1/iam/permissions/objects/connector/connector-1/roles/role-1/grants:validate")
	if requests[7].body["expected_policy_version"] != float64(7) || requests[7].body["expected_role_lifecycle_version"] != float64(5) {
		t.Fatalf("grant body = %#v", requests[7].body)
	}
	assertIAMRequest(t, requests[10], http.MethodDelete, "/newmoi/workspaces/ws-1/iam/roles/role-1")
}

func TestIAMServiceUserRoleBindingsUsesCanonicalRouteAndCAS(t *testing.T) {
	var method, path string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.EscapedPath()
		body = map[string]any{}
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		data := map[string]any{}
		if r.Method == http.MethodGet {
			data = map[string]any{
				"user_id":              "user/1",
				"roles":                []any{map[string]any{"role_id": "role/viewer", "role_name": "Viewer", "selectable": true}},
				"default_role_id":      "role/viewer",
				"default_role_version": 4,
				"binding_version":      "binding-v4",
				"request_id":           "read-1",
				"trace_id":             "trace-1",
			}
		} else {
			data = map[string]any{
				"user_id":              "user/1",
				"assigned_role_ids":    []string{"role/viewer", "role/editor"},
				"default_role_id":      "role/editor",
				"binding_version":      5,
				"default_role_version": 5,
				"audit_event_id":       "audit-1",
				"idempotent_replay":    false,
				"request_id":           "write-1",
				"trace_id":             "trace-2",
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	iam := client.IAM("workspace 1")
	read, err := iam.UserRoleBindings(context.Background(), "user/1")
	if err != nil {
		t.Fatal(err)
	}
	if read.GetUserId() != "user/1" || read.GetBindingVersion() != "binding-v4" || len(read.GetRoles()) != 1 {
		t.Fatalf("read role bindings = %#v", read)
	}
	expectedBindingVersion, err := IAMBindingVersionNumber(read)
	if err != nil {
		t.Fatal(err)
	}
	expectedDefaultRoleVersion := read.GetDefaultRoleVersion()
	write, err := iam.PutUserRoleBindings(context.Background(), "user/1", &IAMUserRoleBindingsRequest{
		AssignedRoleIds:            []string{"role/viewer", "role/editor"},
		DefaultRoleId:              "role/editor",
		ExpectedBindingVersion:     expectedBindingVersion,
		ExpectedDefaultRoleVersion: expectedDefaultRoleVersion,
		RequestId:                  "write-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if write.GetBindingVersion() != 5 || write.GetDefaultRoleId() != "role/editor" {
		t.Fatalf("write role bindings = %#v", write)
	}
	if method != http.MethodPut || path != "/newmoi/workspaces/workspace%201/iam/users/user%2F1/role-bindings" {
		t.Fatalf("request = %s %s", method, path)
	}
	if body["default_role_id"] != "role/editor" || body["expected_binding_version"] != float64(expectedBindingVersion) || body["expected_default_role_version"] != float64(expectedDefaultRoleVersion) || body["request_id"] != "write-1" {
		t.Fatalf("request body = %#v", body)
	}
}

func TestIAMServicePermissionResourcesUsePythonAlignedRoutes(t *testing.T) {
	type capturedRequest struct {
		method string
		path   string
		query  string
		body   map[string]any
	}
	requests := make([]capturedRequest, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, capturedRequest{r.Method, r.URL.Path, r.URL.RawQuery, body})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"items": []any{map[string]any{"resource_id": "connector/1"}}},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	iam := client.IAM("ws-1")
	list, err := iam.ListPermissionResources(
		t.Context(),
		"connector",
		"connector.read",
		"data_privilege",
		WithQueryParam("parent_id", "catalog/1"),
		WithQueryParam("cursor", "next"),
		WithQueryParam("q", "sales"),
		WithQueryParam("limit", "1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	listPage, ok := list.AsMap()
	if !ok {
		t.Fatalf("list result = %#v, want object", list)
	}
	items, _ := listPage["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("list items = %#v, want one resource", listPage["items"])
	}
	described, err := iam.BatchDescribePermissionResources(
		t.Context(),
		"connector",
		"connector.read",
		"data_privilege",
		[]string{"connector/1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	describedPage, ok := described.AsMap()
	if !ok {
		t.Fatalf("batch describe result = %#v, want object", described)
	}
	if _, ok := describedPage["items"].([]any); !ok || len(describedPage["items"].([]any)) != 1 {
		t.Fatalf("batch describe items = %#v, want one resource", describedPage["items"])
	}

	assertIAMRequest(t, requests[0], http.MethodGet, "/newmoi/workspaces/ws-1/iam/permissions/resources")
	if !strings.Contains(requests[0].query, "resource_type=connector") ||
		!strings.Contains(requests[0].query, "action_id=connector.read") ||
		!strings.Contains(requests[0].query, "purpose=data_privilege") ||
		!strings.Contains(requests[0].query, "parent_id=catalog%2F1") ||
		!strings.Contains(requests[0].query, "cursor=next") ||
		!strings.Contains(requests[0].query, "q=sales") ||
		!strings.Contains(requests[0].query, "limit=1") {
		t.Fatalf("resource query = %q", requests[0].query)
	}
	assertIAMRequest(t, requests[1], http.MethodPost, "/newmoi/workspaces/ws-1/iam/permissions/resources:batchDescribe")
	if requests[1].body["resource_type"] != "connector" ||
		requests[1].body["action_id"] != "connector.read" ||
		requests[1].body["purpose"] != "data_privilege" ||
		len(requests[1].body["resource_ids"].([]any)) != 1 {
		t.Fatalf("resource batch body = %#v", requests[1].body)
	}
}

func TestIAMBindingVersionNumber(t *testing.T) {
	tests := []struct {
		name    string
		result  *IAMUserRoleBindingsResult
		want    int64
		wantErr bool
	}{
		{name: "initial version", result: &IAMUserRoleBindingsResult{BindingVersion: "binding-v0"}, want: 0},
		{name: "normal version", result: &IAMUserRoleBindingsResult{BindingVersion: "binding-v42"}, want: 42},
		{name: "nil result", wantErr: true},
		{name: "missing suffix", result: &IAMUserRoleBindingsResult{BindingVersion: "binding-v"}, wantErr: true},
		{name: "wrong prefix", result: &IAMUserRoleBindingsResult{BindingVersion: "v42"}, wantErr: true},
		{name: "negative suffix", result: &IAMUserRoleBindingsResult{BindingVersion: "binding-v-1"}, wantErr: true},
		{name: "non numeric suffix", result: &IAMUserRoleBindingsResult{BindingVersion: "binding-vx"}, wantErr: true},
		{name: "leading zero suffix", result: &IAMUserRoleBindingsResult{BindingVersion: "binding-v01"}, wantErr: true},
		{name: "all zero suffix", result: &IAMUserRoleBindingsResult{BindingVersion: "binding-v00"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IAMBindingVersionNumber(tt.result)
			if tt.wantErr {
				if !IsCode(err, ErrIAMBindingVersionInvalid.Code) {
					t.Fatalf("error = %v, want %s", err, ErrIAMBindingVersionInvalid.Code)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("IAMBindingVersionNumber() = %d, %v; want %d, nil", got, err, tt.want)
			}
		})
	}
}

func TestIAMServiceUserRoleBindingsSendsExplicitEmptyAssignedRoles(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"data": map[string]any{
				"user_id": "user-1", "assigned_role_ids": []string{}, "binding_version": 3,
				"default_role_version": 3, "idempotent_replay": false,
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("test-key"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.IAM("ws-1").PutUserRoleBindings(context.Background(), "user-1", &IAMUserRoleBindingsRequest{
		AssignedRoleIds: []string{}, ExpectedBindingVersion: 2, ExpectedDefaultRoleVersion: 2,
		RequestId: "clear-1", ClearDefaultRole: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	assigned, ok := body["assigned_role_ids"].([]any)
	if !ok || len(assigned) != 0 {
		t.Fatalf("assigned_role_ids = %#v, want explicit empty array", body["assigned_role_ids"])
	}
}

func TestIAMServiceDataPrivilegesUsesTypedNativeRoutes(t *testing.T) {
	type capturedRequest struct {
		method string
		path   string
		body   map[string]any
	}
	requests := make([]capturedRequest, 0, 5)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		path := r.URL.EscapedPath()
		requests = append(requests, capturedRequest{method: r.Method, path: path, body: body})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK", "msg": "OK", "data": iamDataPrivilegeTestResponse(r.Method, path),
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	iam := client.IAM("workspace 1")
	ctx := context.Background()
	request := iamDataPrivilegeTestMutationRequest("request/1", 4)

	view, err := iam.DataPrivileges(ctx, "role/1")
	if err != nil {
		t.Fatal(err)
	}
	if view.GetDataPrivilegeVersion() != 4 || len(view.GetItems()) != 1 || view.GetItems()[0].GetPrivilege() != "select" {
		t.Fatalf("data privilege view = %#v", view)
	}
	validation, err := iam.ValidateDataPrivileges(ctx, "role/1", request)
	if err != nil {
		t.Fatal(err)
	}
	if !validation.GetValid() || validation.GetPayloadFingerprint() != "fingerprint-1" {
		t.Fatalf("data privilege validation = %#v", validation)
	}
	if len(validation.GetCanonicalMutations()) != 1 || validation.GetCanonicalMutations()[0].GetRowColumn().GetCanonicalRule().GetFilters()[0].GetValues()[0] != "1" {
		t.Fatalf("canonical row/column mutation = %v", validation.GetCanonicalMutations())
	}
	applied, err := iam.ApplyDataPrivileges(ctx, "role/1", request)
	if err != nil {
		t.Fatal(err)
	}
	if applied.GetState() != "succeeded" || applied.GetDataPrivilegeVersion() != 5 {
		t.Fatalf("applied data privilege operation = %#v", applied)
	}
	operation, err := iam.DataPrivilegeOperation(ctx, "role/1", "request/1")
	if err != nil {
		t.Fatal(err)
	}
	if operation.GetOperationId() != "operation-1" || len(operation.GetResourceResults()) != 1 {
		t.Fatalf("data privilege operation = %#v", operation)
	}
	repaired, err := iam.RepairDataPrivileges(ctx, "role/1", "request/1:repair", &IAMDataPrivilegeRepairRequest{
		RequestId: "repair-1", ExpectedDataPrivilegeVersion: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repaired.GetState() != "succeeded" || repaired.GetDataPrivilegeVersion() != 6 {
		t.Fatalf("repaired data privilege operation = %#v", repaired)
	}

	if len(requests) != 5 {
		t.Fatalf("requests = %d, want 5", len(requests))
	}
	base := "/newmoi/workspaces/workspace%201/iam/roles/role%2F1"
	assertIAMDataPrivilegeRequest(t, requests[0], http.MethodGet, base+"/data-privileges")
	assertIAMDataPrivilegeRequest(t, requests[1], http.MethodPost, base+"/data-privileges:validate")
	assertIAMDataPrivilegeRequest(t, requests[2], http.MethodPut, base+"/data-privileges")
	assertIAMDataPrivilegeRequest(t, requests[3], http.MethodGet, base+"/data-privilege-operations/request%2F1")
	assertIAMDataPrivilegeRequest(t, requests[4], http.MethodPost, base+"/data-privilege-operations/request%2F1:repair")
	if requests[1].body["expected_data_privilege_version"] != float64(4) {
		t.Fatalf("validate body = %#v", requests[1].body)
	}
	mutations, ok := requests[1].body["mutations"].([]any)
	if !ok || len(mutations) != 1 {
		t.Fatalf("validate mutations = %#v", requests[1].body["mutations"])
	}
	mutation, ok := mutations[0].(map[string]any)
	if !ok || mutation["operation"] != "grant" || mutation["resource_id"] != "table/1" {
		t.Fatalf("validate mutation = %#v", mutations[0])
	}
	rowColumn, ok := mutation["row_column"].(map[string]any)
	if !ok || rowColumn["mode"] != "set" {
		t.Fatalf("validate row column = %#v", mutation["row_column"])
	}
	if requests[4].body["request_id"] != "repair-1" || requests[4].body["expected_data_privilege_version"] != float64(5) {
		t.Fatalf("repair body = %#v", requests[4].body)
	}
}

func iamDataPrivilegeTestMutationRequest(requestID string, expectedVersion int64) *IAMDataPrivilegeMutationRequest {
	return &IAMDataPrivilegeMutationRequest{
		RequestId: requestID, ExpectedDataPrivilegeVersion: expectedVersion,
		Mutations: []*IAMDataPrivilegeMutation{{
			Operation: "grant", ActionId: "table.query", ResourceType: "table", ResourceId: "table/1", Privilege: "select",
			RowColumn: &IAMDataPrivilegeRowColumnMutation{
				Mode: "set", Draft: &IAMDataPrivilegeRowColumnDraft{
					BlackColumnList: []string{"secret"},
					RuleList: []*IAMDataPrivilegeRowRule{{
						Column: "id", Relation: "and", ExpressionList: []*IAMDataPrivilegeRowExpression{{
							Operator: "=", Expression: []string{"1"},
						}},
					}},
				},
			},
		}},
	}
}

func iamDataPrivilegeTestResponse(method, path string) any {
	switch {
	case method == http.MethodGet && path == "/newmoi/workspaces/workspace%201/iam/roles/role%2F1/data-privileges":
		return map[string]any{
			"target_role_id": "role/1", "data_privilege_version": 4, "matrixone_fact_version": "facts-4",
			"items": []any{map[string]any{
				"resource_type": "table", "resource_id": "table/1", "action_id": "table.query", "privilege": "select",
				"ui_config_snapshot": map[string]any{"black_column_list": []string{"secret"}, "rule_list": []any{}}, "columns": []any{},
			}},
		}
	case method == http.MethodPost && path == "/newmoi/workspaces/workspace%201/iam/roles/role%2F1/data-privileges:validate":
		return map[string]any{"valid": true, "errors": []any{}, "warnings": []any{}, "payload_fingerprint": "fingerprint-1",
			"canonical_mutations": []any{map[string]any{"operation": "grant", "resource_id": "table/1", "row_column": map[string]any{
				"mode": "set", "canonical_rule": map[string]any{"select_columns": []string{"id"}, "filters": []any{map[string]any{"column": "id", "operator": "=", "values": []string{"1"}, "match_type": ""}}, "filters_join": "and"},
			}}},
		}
	case method == http.MethodPut && path == "/newmoi/workspaces/workspace%201/iam/roles/role%2F1/data-privileges":
		return iamDataPrivilegeTestOperationResponse(5)
	case method == http.MethodGet && path == "/newmoi/workspaces/workspace%201/iam/roles/role%2F1/data-privilege-operations/request%2F1":
		return iamDataPrivilegeTestOperationResponse(5)
	case method == http.MethodPost && path == "/newmoi/workspaces/workspace%201/iam/roles/role%2F1/data-privilege-operations/request%2F1:repair":
		return iamDataPrivilegeTestOperationResponse(6)
	default:
		return map[string]any{}
	}
}

func iamDataPrivilegeTestOperationResponse(version int64) map[string]any {
	return map[string]any{
		"request_id": "request/1", "operation_id": "operation-1", "state": "succeeded", "data_privilege_version": version,
		"resource_results": []any{map[string]any{
			"resource_type": "table", "resource_id": "table/1", "action_id": "table.query", "privilege_key": "select", "operation": "grant", "state": "succeeded",
		}},
	}
}

func assertIAMDataPrivilegeRequest(t *testing.T, request struct {
	method string
	path   string
	body   map[string]any
}, method, path string) {
	t.Helper()
	if request.method != method || request.path != path {
		t.Fatalf("request = %s %s, want %s %s", request.method, request.path, method, path)
	}
}

func iamTestGrantRequest(requestID string) *IAMObjectGrantsMutationRequest {
	return &IAMObjectGrantsMutationRequest{
		SchemaVersion:                "v1",
		ExpectedPolicyVersion:        7,
		ExpectedRoleLifecycleVersion: 5,
		RequestId:                    requestID,
		Changes:                      []*IAMObjectGrantChange{{ActionId: "connector.read"}},
	}
}

func assertIAMRequest(t *testing.T, request struct {
	method string
	path   string
	query  string
	body   map[string]any
}, method, path string) {
	t.Helper()
	if request.method != method || request.path != path {
		t.Fatalf("request = %s %s, want %s %s", request.method, request.path, method, path)
	}
}

func iamTestResponse(method, path string) any {
	switch {
	case path == "/newmoi/workspaces/ws-1/iam/permissions/schema":
		return map[string]any{"schema_version": "v1", "capabilities": map[string]any{}, "resources": []any{}, "actions": []any{}, "operations": []any{}}
	case method == http.MethodGet && path == "/newmoi/workspaces/ws-1/iam/roles":
		return map[string]any{"roles": []any{map[string]any{
			"role_id":          "role-admin",
			"role_name":        "Administrator",
			"system_role":      true,
			"system_role_name": "admin",
		}}}
	case method == http.MethodPost && path == "/newmoi/workspaces/ws-1/iam/roles":
		return iamTestRoleWriteResult(1, 7, 4, false)
	case method == http.MethodPatch && path == "/newmoi/workspaces/ws-1/iam/roles/role-1":
		return iamTestRoleWriteResult(4, 8, 5, true)
	case path == "/newmoi/workspaces/ws-1/iam/roles/role-1/permission-view":
		return map[string]any{"role_id": "role-1", "schema_version": "v1", "expected_role_lifecycle_version": 3, "lifecycle_state": "disabled", "policy_mutation_allowed": false, "capabilities": map[string]any{}, "direct_grants": []any{}}
	case path == "/newmoi/workspaces/ws-1/iam/roles/role-1/delete-impact":
		return map[string]any{"role": iamTestRole(), "blocked": false}
	case path == "/newmoi/workspaces/ws-1/iam/permissions/objects/connector/connector-1/permission-view":
		return map[string]any{
			"workspace_id": "ws-1", "resource_type": "connector", "resource_id": "connector-1",
			"owner": map[string]any{"role_id": "role-1", "role_lifecycle_state": "disabled"}, "capabilities": map[string]any{},
			"direct_grants":    []any{map[string]any{"role_id": "role-1", "role_lifecycle_state": "disabled", "action_id": "connector.update", "scope_mode": "resources", "resource_id": "connector-1"}},
			"all_scope_grants": []any{map[string]any{"role_id": "role-2", "role_lifecycle_state": "active", "action_id": "connector.read", "scope_mode": "all"}},
		}
	case path == "/newmoi/workspaces/ws-1/iam/permissions/objects/connector/connector-1/roles/role-1/grants:validate":
		return map[string]any{"workspace_id": "ws-1", "resource_type": "connector", "resource_id": "connector-1", "target_role_id": "role-1", "previous_policy_version": 7, "policy_version": 7, "added_count": 1, "validate_only": true}
	case path == "/newmoi/workspaces/ws-1/iam/permissions/objects/connector/connector-1/roles/role-1/grants":
		return map[string]any{"workspace_id": "ws-1", "resource_type": "connector", "resource_id": "connector-1", "target_role_id": "role-1", "previous_policy_version": 7, "policy_version": 8, "added_count": 1}
	default:
		return map[string]any{"role": iamTestRole(), "idempotent_replay": false}
	}
}

func iamTestRoleWriteResult(roleVersion, policyVersion, dataVersion int64, replay bool) map[string]any {
	return map[string]any{
		"role_id": "role-1", "role_version": roleVersion, "policy_version": policyVersion,
		"data_privilege_version": dataVersion, "provisioning_state": "ready", "provisioning_version": 2,
		"provisioning_error_code": "", "idempotent_replay": replay, "session_invalidation_status": "succeeded",
		"request_id": "response-request", "trace_id": "response-trace",
	}
}

func iamTestRole() map[string]any {
	return map[string]any{"workspace_id": "ws-1", "role_id": "role-1", "role_name": "analyst", "lifecycle_state": "active", "lifecycle_version": 5, "system_role": false}
}

func TestIAMExactGrantResourceTransport(t *testing.T) {
	var writes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			statements, ok := body["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Errorf("statements=%#v", body["statements"])
				return
			}
			if got := statements[0].(map[string]any)["grant_resource_id"]; got != "41" {
				t.Errorf("exact grant target=%#v", got)
			}
			writes++
		}
		data := map[string]any{}
		if r.Method == http.MethodGet {
			data["direct_grants"] = []map[string]any{{"action_id": "table.grant.query", "grant_resource_id": "41"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": data})
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	service := client.IAM("ws")
	request := &IAMApplicationPolicyRequest{Statements: []*IAMApplicationPolicyStatement{{ActionId: "table.grant.query", ResourceType: "table", ResourceId: "41", ScopeMode: "resources", TargetActionId: "table.query", GrantScopeMode: "resources", GrantResourceId: "41"}}}
	if _, err = service.ValidateApplicationPolicy(t.Context(), "7", request); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ApplyApplicationPolicy(t.Context(), "7", request); err != nil {
		t.Fatal(err)
	}
	view, err := service.RolePermissionView(t.Context(), "7")
	if err != nil || len(view.GetDirectGrants()) != 1 || view.GetDirectGrants()[0].GetGrantResourceId() != "41" || writes != 2 {
		t.Fatalf("view=%v err=%v writes=%d", view, err, writes)
	}
}

func TestIAMRoleInheritanceUsesRoleVersionsAndHistoricalOutcome(t *testing.T) {
	expected := map[string]any{"parent_role_id": "7", "expected_child_lifecycle_version": float64(2), "expected_parent_lifecycle_version": float64(3), "request_id": "original-request"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(expected, body) {
			t.Errorf("inheritance body = %#v", body)
		}
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": map[string]any{"child_role_id": "8", "parent_role_id": "7", "idempotent_replay": true}})
		} else {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "ErrIAMInheritancePending", "msg": "本次修改结果尚未确认"})
		}
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	iam := client.IAM("ws-1")
	request := &IAMRoleInheritanceMutationRequest{ParentRoleId: "7", ExpectedChildLifecycleVersion: 2, ExpectedParentLifecycleVersion: 3, RequestId: "original-request"}
	result, err := iam.AddRoleInheritance(t.Context(), "8", request)
	if err != nil || result.GetChildRoleId() != "8" || result.GetParentRoleId() != "7" || !result.GetIdempotentReplay() {
		t.Fatalf("result=%v err=%v", result, err)
	}
	_, err = iam.RemoveRoleInheritance(t.Context(), "8", request)
	var failure *Error
	if !errors.As(err, &failure) || failure.Status != http.StatusConflict || failure.Code != "ErrIAMInheritancePending" || failure.Message != "本次修改结果尚未确认" {
		t.Fatalf("pending message was lost: %v", err)
	}
}

func TestIAMTransferObjectOwnership(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.EscapedPath() != "/newmoi/workspaces/ws-1/iam/permissions/objects/connector/connector-1/owner" {
			t.Errorf("unexpected ownership request: %s %s", r.Method, r.URL)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"OK","data":{"workspace_id":"ws-1","resource_type":"connector","resource_id":"connector-1","previous_owner_role_id":"5","owner_role_id":"7","ownership_version":4,"request_id":"transfer-1","idempotent_replay":true}}`))
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	result, err := client.IAM("ws-1").TransferObjectOwnership(t.Context(), "connector", "connector-1", &IAMObjectOwnershipTransferRequest{TargetRoleId: "7", ExpectedOwnershipVersion: 3, RequestId: "transfer-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(received, map[string]any{"target_role_id": "7", "expected_ownership_version": float64(3), "request_id": "transfer-1"}) {
		t.Fatalf("ownership command = %#v", received)
	}
	if result.GetOwnerRoleId() != "7" || result.GetOwnershipVersion() != 4 || !result.GetIdempotentReplay() {
		t.Fatalf("ownership result = %v", result)
	}
}

func TestIAMRoleDeleteOwnershipsUsesIndependentPageContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/newmoi/workspaces/ws-1/iam/roles/5/delete-ownerships" {
			t.Errorf("unexpected page request: %s %s", r.Method, r.URL)
		}
		want := map[string][]string{"cursor": {"next-key"}, "limit": {"10"}, "transfer_requested": {"true"}}
		if !reflect.DeepEqual(map[string][]string(r.URL.Query()), want) {
			t.Errorf("query = %v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"OK","data":{"items":[{"resource_type":"connector","resource_id":"17","display_name":"Orders","ownership_version":3}],"ownership_version":"set-v1","next_cursor":"later-key"}}`))
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	page, err := client.IAM("ws-1").RoleDeleteOwnerships(t.Context(), "5", WithQueryParam("cursor", "next-key"), WithQueryParam("limit", "10"), WithQueryParam("transfer_requested", "true"))
	if err != nil {
		t.Fatal(err)
	}
	if page.GetOwnershipVersion() != "set-v1" || page.GetNextCursor() != "later-key" {
		t.Fatalf("page = %v", page)
	}
	want := []*IAMRoleDeleteOwnershipItem{{ResourceType: "connector", ResourceId: "17", DisplayName: "Orders", OwnershipVersion: 3}}
	if len(page.GetItems()) != len(want) || page.GetItems()[0].GetResourceType() != want[0].GetResourceType() || page.GetItems()[0].GetResourceId() != want[0].GetResourceId() || page.GetItems()[0].GetDisplayName() != want[0].GetDisplayName() || page.GetItems()[0].GetOwnershipVersion() != want[0].GetOwnershipVersion() {
		t.Fatalf("items = %v", page.GetItems())
	}
}
