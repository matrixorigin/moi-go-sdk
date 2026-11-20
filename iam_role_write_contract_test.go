package sdk

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The public outcomes are migrated from abcad03c's role_write_error_test.go.
func TestIAMRoleWriteErrorsPreservePublicOutcome(t *testing.T) {
	for _, tc := range []struct {
		status        int
		code, message string
	}{
		{409, "ErrIAMRoleNameExists", "This role name is already in use. Choose another name and submit again."},
		{409, "ErrIAMRoleWriteRolledBack", "The role change was not completed. Your previous settings are unchanged. You can submit again."},
		{500, "ErrIAMRoleWriteFailed", "The role change was not completed and the operation has stopped. Contact an administrator before continuing."},
		{409, "ErrIAMRoleWriteResultUnknown", "The result of this role change is not yet confirmed. Check the current role settings. If you still cannot confirm the result, contact an administrator."},
		{422, "ErrIAMRolePermissionRequired", "Select at least one valid application or data permission before creating the role."},
	} {
		t.Run(tc.code, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Request-ID", "public-request")
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"code": tc.code, "msg": tc.message, "data": map[string]any{"request_id": "body-request"}})
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
			if err != nil {
				t.Fatal(err)
			}
			for _, operation := range []string{"create", "update"} {
				var result *IAMRoleCreateResult
				if operation == "create" {
					result, err = client.IAM("ws-1").CreateRole(t.Context(), &IAMRoleCreateRequest{Name: "analyst", SchemaVersion: "v1", RequestId: "create-1"})
				} else {
					result, err = client.IAM("ws-1").UpdateRole(t.Context(), "role-1", &IAMRoleUpdateRequest{Name: "analyst", SchemaVersion: "v1", RequestId: "update-1", ExpectedLifecycleVersion: 1, ExpectedPolicyVersion: 1})
				}
				var public *Error
				if result != nil || !errors.As(err, &public) || public.Code != tc.code || public.Message != tc.message || public.Status != tc.status || public.RequestID != "public-request" {
					t.Fatalf("%s result=%v error=%v", operation, result, err)
				}
			}
		})
	}
}

func TestIAMRoleWriteStrictlyRejectsMetadataResponseAndMalformedVersions(t *testing.T) {
	for _, data := range []any{
		map[string]any{"role": iamTestRole(), "idempotent_replay": false},
		map[string]any{"role_id": "role-1", "role_version": "invalid", "provisioning_state": "ready"},
		map[string]any{"role_id": "role-1", "data_privilege_version": "invalid", "provisioning_state": "ready"},
		map[string]any{"role_id": "role-1", "session_invalidation_status": "succeeded", "unknown_outcome": true},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
		}))
		t.Cleanup(server.Close)
		client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
		if err != nil {
			t.Fatal(err)
		}
		result, err := client.IAM("ws-1").UpdateRole(t.Context(), "role-1", &IAMRoleUpdateRequest{RequestId: "update-1"})
		if err == nil || result != nil {
			t.Fatalf("accepted incompatible response %v: result=%v err=%v", data, result, err)
		}
	}
}
