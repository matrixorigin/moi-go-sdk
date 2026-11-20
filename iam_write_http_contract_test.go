package sdk

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type iamWriteHTTPContract struct {
	Name      string `json:"name"`
	Operation string `json:"operation"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Input     struct {
		UserID          string          `json:"user_id"`
		RoleID          string          `json:"role_id"`
		ResourceType    string          `json:"resource_type"`
		ResourceID      string          `json:"resource_id"`
		RepairOperation string          `json:"repair_operation"`
		Request         json.RawMessage `json:"request"`
	} `json:"input"`
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response"`
	Status   int             `json:"status"`
}

func iamWriteContractJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&value)
	return value
}

func iamWriteRequest[T any](t *testing.T, scenario iamWriteHTTPContract) *T {
	t.Helper()
	var request T
	if err := json.Unmarshal(scenario.Input.Request, &request); err != nil {
		t.Fatal(err)
	}
	return &request
}

func TestIAMWriteSharedHTTPContracts(t *testing.T) {
	data, err := os.ReadFile("testdata/iam_write_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []iamWriteHTTPContract
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			var received any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != scenario.Method || r.URL.EscapedPath() != scenario.Path || r.URL.RawQuery != "" || r.Header.Get("X-API-Key") != "user-key" || r.Header.Get("X-Workspace-ID") != "workspace-1" {
					t.Errorf("request=%s %s headers=%v", r.Method, r.URL.RequestURI(), r.Header)
				}
				raw, _ := io.ReadAll(r.Body)
				if len(raw) > 0 {
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					_ = decoder.Decode(&received)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				_, _ = w.Write(scenario.Response)
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
			if err != nil {
				t.Fatal(err)
			}
			service := client.IAM("workspace-1")
			input := scenario.Input
			var result proto.Message
			switch scenario.Operation {
			case "put-user-role-bindings":
				result, err = service.PutUserRoleBindings(t.Context(), input.UserID, iamWriteRequest[IAMUserRoleBindingsRequest](t, scenario))
			case "create-role":
				result, err = service.CreateRole(t.Context(), iamWriteRequest[IAMRoleCreateRequest](t, scenario))
			case "update-role":
				result, err = service.UpdateRole(t.Context(), input.RoleID, iamWriteRequest[IAMRoleUpdateRequest](t, scenario))
			case "validate-object-grants":
				result, err = service.ValidateObjectGrants(t.Context(), input.ResourceType, input.ResourceID, input.RoleID, iamWriteRequest[IAMObjectGrantsMutationRequest](t, scenario))
			case "apply-object-grants":
				result, err = service.ApplyObjectGrants(t.Context(), input.ResourceType, input.ResourceID, input.RoleID, iamWriteRequest[IAMObjectGrantsMutationRequest](t, scenario))
			case "validate-data-privileges":
				result, err = service.ValidateDataPrivileges(t.Context(), input.RoleID, iamWriteRequest[IAMDataPrivilegeMutationRequest](t, scenario))
			case "apply-data-privileges":
				result, err = service.ApplyDataPrivileges(t.Context(), input.RoleID, iamWriteRequest[IAMDataPrivilegeMutationRequest](t, scenario))
			case "repair-data-privileges":
				result, err = service.RepairDataPrivileges(t.Context(), input.RoleID, input.RepairOperation, iamWriteRequest[IAMDataPrivilegeRepairRequest](t, scenario))
			case "add-role-inheritance":
				result, err = service.AddRoleInheritance(t.Context(), input.RoleID, iamWriteRequest[IAMRoleInheritanceMutationRequest](t, scenario))
			case "remove-role-inheritance":
				result, err = service.RemoveRoleInheritance(t.Context(), input.RoleID, iamWriteRequest[IAMRoleInheritanceMutationRequest](t, scenario))
			case "validate-application-policy":
				result, err = service.ValidateApplicationPolicy(t.Context(), input.RoleID, iamWriteRequest[IAMApplicationPolicyRequest](t, scenario))
			case "apply-application-policy":
				result, err = service.ApplyApplicationPolicy(t.Context(), input.RoleID, iamWriteRequest[IAMApplicationPolicyRequest](t, scenario))
			default:
				t.Fatalf("unknown operation %s", scenario.Operation)
			}
			if err != nil {
				t.Fatal(err)
			}
			if result != nil {
				var envelope struct {
					Data json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal(scenario.Response, &envelope); err != nil {
					t.Fatal(err)
				}
				expected := result.ProtoReflect().New().Interface()
				if err := protojson.Unmarshal(envelope.Data, expected); err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(result, expected) {
					t.Fatalf("result=%v want=%v", result, expected)
				}
			}
			if !reflect.DeepEqual(received, iamWriteContractJSON(scenario.Request)) {
				t.Fatalf("body=%#v want=%#v", received, iamWriteContractJSON(scenario.Request))
			}
		})
	}
}
