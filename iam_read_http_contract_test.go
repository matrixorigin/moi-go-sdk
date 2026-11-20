package sdk

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type iamReadHTTPContract struct {
	Name      string              `json:"name"`
	Operation string              `json:"operation"`
	Method    string              `json:"method"`
	Path      string              `json:"path"`
	Query     map[string][]string `json:"query"`
	Input     struct {
		UserID        string          `json:"user_id"`
		RoleID        string          `json:"role_id"`
		ResourceType  string          `json:"resource_type"`
		ResourceID    string          `json:"resource_id"`
		RequestID     string          `json:"request_id"`
		CapabilityIDs []string        `json:"capability_ids"`
		Request       json.RawMessage `json:"request"`
	} `json:"input"`
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response"`
	Status   int             `json:"status"`
}

func iamReadContractJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&value)
	return value
}

func iamReadRequest[T any](t *testing.T, scenario iamReadHTTPContract) *T {
	t.Helper()
	var request T
	if err := json.Unmarshal(scenario.Input.Request, &request); err != nil {
		t.Fatal(err)
	}
	return &request
}

func TestIAMReadSharedHTTPContracts(t *testing.T) {
	data, err := os.ReadFile("testdata/iam_read_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []iamReadHTTPContract
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			var received any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedQuery := url.Values{}
				for key, values := range scenario.Query {
					for _, value := range values {
						expectedQuery.Add(key, value)
					}
				}
				if r.Method != scenario.Method || r.URL.EscapedPath() != scenario.Path || !reflect.DeepEqual(r.URL.Query(), expectedQuery) || r.Header.Get("X-API-Key") != "user-key" || r.Header.Get("X-Workspace-ID") != "workspace-1" {
					t.Errorf("request=%s %s query=%v headers=%v", r.Method, r.URL.RequestURI(), r.URL.Query(), r.Header)
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
			case "permission-schema":
				result, err = service.PermissionSchema(t.Context())
			case "roles":
				result, err = service.Roles(t.Context())
			case "user-role-bindings":
				result, err = service.UserRoleBindings(t.Context(), input.UserID)
			case "role-permission-view":
				result, err = service.RolePermissionView(t.Context(), input.RoleID)
			case "update-role-lifecycle":
				result, err = service.UpdateRoleLifecycle(t.Context(), input.RoleID, iamReadRequest[IAMRoleLifecycleRequest](t, scenario))
			case "delete-role":
				result, err = service.DeleteRole(t.Context(), input.RoleID, iamReadRequest[IAMRoleDeleteRequest](t, scenario))
			case "transfer-object-ownership":
				result, err = service.TransferObjectOwnership(t.Context(), input.ResourceType, input.ResourceID, iamReadRequest[IAMObjectOwnershipTransferRequest](t, scenario))
			case "object-permission-view":
				result, err = service.ObjectPermissionView(t.Context(), input.ResourceType, input.ResourceID)
			case "data-privileges":
				result, err = service.DataPrivileges(t.Context(), input.RoleID)
			case "data-privilege-operation":
				result, err = service.DataPrivilegeOperation(t.Context(), input.RoleID, input.RequestID)
			case "current-principal-roles":
				result, err = service.CurrentPrincipalRoles(t.Context())
			case "current-principal-capabilities":
				result, err = service.CurrentPrincipalCapabilities(t.Context(), input.CapabilityIDs...)
			case "role-inheritance":
				result, err = service.RoleInheritance(t.Context(), input.RoleID)
			case "role-inheritance-graph":
				result, err = service.RoleInheritanceGraph(t.Context(), input.RoleID)
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
			if !reflect.DeepEqual(received, iamReadContractJSON(scenario.Request)) {
				t.Fatalf("body=%#v want=%#v", received, iamReadContractJSON(scenario.Request))
			}
		})
	}
}
