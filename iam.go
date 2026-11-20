package sdk

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

type IAMCurrentPrincipalRolesResult = product.WorkspaceIAMPrincipalContext
type IAMCurrentPrincipalCapabilitiesResult = product.WorkspaceIAMCapabilities

type IAMRoleInheritanceEdge = product.IAMRoleInheritanceEdge
type IAMRoleInheritanceResult = product.IAMRoleInheritanceResult
type IAMRoleInheritanceMutationRequest = product.IAMRoleInheritanceMutationRequest
type IAMRoleInheritanceMutationResult = product.IAMRoleInheritanceMutationResult
type IAMApplicationPolicyRequest = product.IAMApplicationPolicyRequest
type IAMApplicationPolicyValidationResult = product.IAMApplicationPolicyValidationResult
type IAMApplicationPolicyApplyResult = product.IAMApplicationPolicyApplyResult

type IAMPermissionSchemaResult = product.IAMPermissionSchemaResult
type IAMRoleListResult = product.IAMRoleListResult
type IAMRoleSummary = product.IAMRoleSummary
type IAMUserRoleBindingItem = product.IAMUserRoleBindingItem
type IAMUserRoleBindingsResult = product.IAMUserRoleBindingsResult
type IAMUserRoleBindingsRequest = product.IAMUserRoleBindingsRequest
type IAMUserRoleBindingsMutationResult = product.IAMUserRoleBindingsMutationResult
type IAMApplicationPolicyStatement = product.IAMApplicationPolicyStatement
type IAMRoleCreateRequest = product.IAMRoleCreateRequest
type IAMRoleCreateResult = product.IAMRoleCreateResult
type IAMRoleUpdateRequest = product.IAMRoleUpdateRequest
type IAMRoleLifecycleRequest = product.IAMRoleLifecycleRequest
type IAMRoleDeleteRequest = product.IAMRoleDeleteRequest
type IAMRoleDeleteOwnershipTransfer = product.IAMRoleDeleteOwnershipTransfer
type IAMRoleDeleteOwnershipTarget = product.IAMRoleDeleteOwnershipTarget
type IAMRoleDeleteOwnershipItem = product.IAMRoleDeleteOwnershipItem
type IAMRoleDeleteOwnershipPage = product.IAMRoleDeleteOwnershipPage
type IAMRoleDeleteOwnershipList = product.IAMRoleDeleteOwnershipList
type IAMRoleRecord = product.IAMRoleRecord
type IAMRoleMutationResult = product.IAMRoleMutationResult
type IAMRoleDeleteImpactResult = product.IAMRoleDeleteImpactResult
type IAMRolePermissionViewResult = product.IAMRolePermissionViewResult
type IAMObjectGrantChange = product.IAMObjectGrantChange
type IAMObjectOwnershipTransferRequest = product.IAMObjectOwnershipTransferRequest
type IAMObjectOwnershipTransferResult = product.IAMObjectOwnershipTransferResult
type IAMObjectGrantsMutationRequest = product.IAMObjectGrantsMutationRequest
type IAMObjectGrantsMutationResult = product.IAMObjectGrantsMutationResult
type IAMObjectPermissionViewResult = product.IAMObjectPermissionViewResult
type IAMDataPrivilegeRowExpression = product.IAMDataPrivilegeRowExpression
type IAMDataPrivilegeRowRule = product.IAMDataPrivilegeRowRule
type IAMDataPrivilegeRowColumnDraft = product.IAMDataPrivilegeRowColumnDraft
type IAMDataPrivilegeRowColumnMutation = product.IAMDataPrivilegeRowColumnMutation
type IAMDataPrivilegeMutation = product.IAMDataPrivilegeMutation
type IAMDataPrivilegeMutationRequest = product.IAMDataPrivilegeMutationRequest
type IAMDataPrivilegeColumn = product.IAMDataPrivilegeColumn
type IAMDataPrivilegeItem = product.IAMDataPrivilegeItem
type IAMDataPrivilegeViewResult = product.IAMDataPrivilegeViewResult
type IAMDataPrivilegeFieldError = product.IAMDataPrivilegeFieldError
type IAMDataPrivilegeValidationResult = product.IAMDataPrivilegeValidationResult
type IAMDataPrivilegeResourceResult = product.IAMDataPrivilegeResourceResult
type IAMDataPrivilegeOperationResult = product.IAMDataPrivilegeOperationResult
type IAMDataPrivilegeRepairRequest = product.IAMDataPrivilegeRepairRequest

// IAMBindingVersionNumber converts a typed canonical role-binding read into
// the numeric CAS value required by PutUserRoleBindings. The original
// binding-vN value remains available on the result for diagnostics.
func IAMBindingVersionNumber(result *IAMUserRoleBindingsResult) (int64, error) {
	if result == nil {
		return 0, ErrIAMBindingVersionInvalid
	}
	raw, ok := strings.CutPrefix(result.GetBindingVersion(), "binding-v")
	if !ok || raw == "" {
		return 0, ErrIAMBindingVersionInvalid
	}
	if len(raw) > 1 && raw[0] == '0' {
		return 0, ErrIAMBindingVersionInvalid
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, ErrIAMBindingVersionInvalid
		}
	}
	version, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, ErrIAMBindingVersionInvalid
	}
	return version, nil
}

func (s *IAMService) PermissionSchema(ctx context.Context) (*IAMPermissionSchemaResult, error) {
	workspaceID, err := s.workspacePathID()
	if err != nil {
		return nil, err
	}
	out := &product.IAMPermissionSchemaResult{}
	if err := s.do(ctx, http.MethodGet, "/workspaces/"+workspaceID+"/iam/permissions/schema", nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *IAMService) Roles(ctx context.Context) (*IAMRoleListResult, error) {
	workspaceID, err := s.workspacePathID()
	if err != nil {
		return nil, err
	}
	out := &product.IAMRoleListResult{}
	if err := s.do(ctx, http.MethodGet, "/workspaces/"+workspaceID+"/iam/roles", nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// UserRoleBindings returns the canonical role bindings for a workspace user.
// The user ID is the canonical IAM/Core ID, not an account or legacy member ID.
func (s *IAMService) UserRoleBindings(ctx context.Context, userID string) (*IAMUserRoleBindingsResult, error) {
	workspaceID, userPath, err := s.workspaceUserPathIDs(userID)
	if err != nil {
		return nil, err
	}
	out := &product.IAMUserRoleBindingsResult{}
	path := "/workspaces/" + workspaceID + "/iam/users/" + userPath + "/role-bindings"
	if err := s.do(ctx, http.MethodGet, path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// PutUserRoleBindings replaces a user's canonical role bindings using the
// caller-provided CAS versions and request ID.
func (s *IAMService) PutUserRoleBindings(ctx context.Context, userID string, request *IAMUserRoleBindingsRequest) (*IAMUserRoleBindingsMutationResult, error) {
	if request == nil {
		return nil, iamRequestRequired()
	}
	workspaceID, userPath, err := s.workspaceUserPathIDs(userID)
	if err != nil {
		return nil, err
	}
	out := &product.IAMUserRoleBindingsMutationResult{}
	path := "/workspaces/" + workspaceID + "/iam/users/" + userPath + "/role-bindings"
	// Protobuf JSON tags use omitempty for repeated fields.  Build the wire
	// body explicitly so an empty, intentional assigned-role set remains `[]`
	// for the canonical clear operation instead of being omitted.
	body := map[string]any{
		"assigned_role_ids":             request.GetAssignedRoleIds(),
		"default_role_id":               request.GetDefaultRoleId(),
		"expected_binding_version":      request.GetExpectedBindingVersion(),
		"expected_default_role_version": request.GetExpectedDefaultRoleVersion(),
		"request_id":                    request.GetRequestId(),
		"clear_default_role":            request.GetClearDefaultRole(),
	}
	if err := s.do(ctx, http.MethodPut, path, body, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *IAMService) CreateRole(ctx context.Context, request *IAMRoleCreateRequest) (*IAMRoleCreateResult, error) {
	if request == nil {
		return nil, iamRequestRequired()
	}
	workspaceID, err := s.workspacePathID()
	if err != nil {
		return nil, err
	}
	out := &product.IAMRoleCreateResult{}
	if err := s.do(ctx, http.MethodPost, "/workspaces/"+workspaceID+"/iam/roles", request, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *IAMService) RolePermissionView(ctx context.Context, roleID string) (*IAMRolePermissionViewResult, error) {
	workspaceID, rolePath, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	out := &product.IAMRolePermissionViewResult{}
	if err := s.do(ctx, http.MethodGet, "/workspaces/"+workspaceID+"/iam/roles/"+rolePath+"/permission-view", nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *IAMService) UpdateRole(ctx context.Context, roleID string, request *IAMRoleUpdateRequest) (*IAMRoleCreateResult, error) {
	if request == nil {
		return nil, iamRequestRequired()
	}
	workspaceID, rolePath, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	out := &product.IAMRoleCreateResult{}
	if err := s.do(ctx, http.MethodPatch, "/workspaces/"+workspaceID+"/iam/roles/"+rolePath, request, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *IAMService) UpdateRoleLifecycle(ctx context.Context, roleID string, request *IAMRoleLifecycleRequest) (*IAMRoleMutationResult, error) {
	if request == nil {
		return nil, iamRequestRequired()
	}
	workspaceID, rolePath, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	out := &product.IAMRoleMutationResult{}
	if err := s.do(ctx, http.MethodPut, "/workspaces/"+workspaceID+"/iam/roles/"+rolePath+"/lifecycle", request, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *IAMService) RoleDeleteImpact(ctx context.Context, roleID string, expectedLifecycleVersion int64, opts ...QueryOption) (*IAMRoleDeleteImpactResult, error) {
	if expectedLifecycleVersion <= 0 {
		return nil, ErrVersionRequired
	}
	workspaceID, rolePath, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	out := &product.IAMRoleDeleteImpactResult{}
	path := withQuery(
		"/workspaces/"+workspaceID+"/iam/roles/"+rolePath+"/delete-impact",
		append(opts, WithQueryParam("expected_lifecycle_version", strconv.FormatInt(expectedLifecycleVersion, 10))),
	)
	if err := s.do(ctx, http.MethodGet, path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// RoleDeleteOwnerships reads one page without recomputing deletion totals.
func (s *IAMService) RoleDeleteOwnerships(ctx context.Context, roleID string, opts ...QueryOption) (*IAMRoleDeleteOwnershipList, error) {
	workspaceID, rolePath, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	out := &product.IAMRoleDeleteOwnershipList{}
	path := withQuery("/workspaces/"+workspaceID+"/iam/roles/"+rolePath+"/delete-ownerships", opts)
	if err := s.do(ctx, http.MethodGet, path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *IAMService) DeleteRole(ctx context.Context, roleID string, request *IAMRoleDeleteRequest) (*IAMRoleMutationResult, error) {
	if request == nil {
		return nil, iamRequestRequired()
	}
	workspaceID, rolePath, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	out := &product.IAMRoleMutationResult{}
	if err := s.do(ctx, http.MethodDelete, "/workspaces/"+workspaceID+"/iam/roles/"+rolePath, request, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *IAMService) ValidateObjectGrants(ctx context.Context, resourceType, resourceID, roleID string, request *IAMObjectGrantsMutationRequest) (*IAMObjectGrantsMutationResult, error) {
	workspaceID, resourceTypePath, resourceIDPath, rolePath, err := s.objectGrantPathIDs(resourceType, resourceID, roleID)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, iamRequestRequired()
	}
	out := &product.IAMObjectGrantsMutationResult{}
	path := "/workspaces/" + workspaceID + "/iam/permissions/objects/" + resourceTypePath + "/" + resourceIDPath + "/roles/" + rolePath + "/grants:validate"
	if err := s.do(ctx, http.MethodPost, path, iamObjectGrantsBody(request), out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *IAMService) ApplyObjectGrants(ctx context.Context, resourceType, resourceID, roleID string, request *IAMObjectGrantsMutationRequest) (*IAMObjectGrantsMutationResult, error) {
	workspaceID, resourceTypePath, resourceIDPath, rolePath, err := s.objectGrantPathIDs(resourceType, resourceID, roleID)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, iamRequestRequired()
	}
	out := &product.IAMObjectGrantsMutationResult{}
	path := "/workspaces/" + workspaceID + "/iam/permissions/objects/" + resourceTypePath + "/" + resourceIDPath + "/roles/" + rolePath + "/grants"
	if err := s.do(ctx, http.MethodPut, path, iamObjectGrantsBody(request), out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *IAMService) TransferObjectOwnership(ctx context.Context, resourceType, resourceID string, request *IAMObjectOwnershipTransferRequest) (*IAMObjectOwnershipTransferResult, error) {
	workspaceID, resourceTypePath, resourceIDPath, err := s.objectPathIDs(resourceType, resourceID)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, iamRequestRequired()
	}
	out := &product.IAMObjectOwnershipTransferResult{}
	path := "/workspaces/" + workspaceID + "/iam/permissions/objects/" + resourceTypePath + "/" + resourceIDPath + "/owner"
	if err := s.do(ctx, http.MethodPut, path, request, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *IAMService) ObjectPermissionView(ctx context.Context, resourceType, resourceID string) (*IAMObjectPermissionViewResult, error) {
	workspaceID, resourceTypePath, resourceIDPath, err := s.objectPathIDs(resourceType, resourceID)
	if err != nil {
		return nil, err
	}
	out := &product.IAMObjectPermissionViewResult{}
	path := "/workspaces/" + workspaceID + "/iam/permissions/objects/" + resourceTypePath + "/" + resourceIDPath + "/permission-view"
	if err := s.do(ctx, http.MethodGet, path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListPermissionResources returns the IAM resource picker page for one action.
func (s *IAMService) ListPermissionResources(
	ctx context.Context,
	resourceType, actionID, purpose string,
	opts ...QueryOption,
) (*ProductData, error) {
	if resourceType == "" {
		return nil, ErrTypeRequired
	}
	if actionID == "" {
		return nil, ErrActionRequired
	}
	workspaceID, err := s.workspacePathID()
	if err != nil {
		return nil, err
	}
	options := []QueryOption{
		WithQueryParam("resource_type", resourceType),
		WithQueryParam("action_id", actionID),
	}
	if purpose != "" {
		options = append(options, WithQueryParam("purpose", purpose))
	}
	options = append(options, opts...)
	out := &ProductData{}
	path := withQuery("/workspaces/"+workspaceID+"/iam/permissions/resources", options)
	if err := s.do(ctx, http.MethodGet, path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// BatchDescribePermissionResources resolves a closed set of IAM resources.
func (s *IAMService) BatchDescribePermissionResources(
	ctx context.Context,
	resourceType, actionID string,
	purpose string,
	resourceIDs []string,
) (*ProductData, error) {
	if resourceType == "" {
		return nil, ErrTypeRequired
	}
	if actionID == "" {
		return nil, ErrActionRequired
	}
	if len(resourceIDs) == 0 {
		return nil, ErrIDRequired
	}
	workspaceID, err := s.workspacePathID()
	if err != nil {
		return nil, err
	}
	request := struct {
		ResourceType string   `json:"resource_type"`
		ResourceIDs  []string `json:"resource_ids"`
		ActionID     string   `json:"action_id"`
		Purpose      string   `json:"purpose,omitempty"`
	}{ResourceType: resourceType, ResourceIDs: resourceIDs, ActionID: actionID, Purpose: purpose}
	out := &ProductData{}
	path := "/workspaces/" + workspaceID + "/iam/permissions/resources:batchDescribe"
	if err := s.do(ctx, http.MethodPost, path, request, out); err != nil {
		return nil, err
	}
	return out, nil
}

// DataPrivileges reads a role's current MatrixOne data-privilege projection.
func (s *IAMService) DataPrivileges(ctx context.Context, roleID string) (*IAMDataPrivilegeViewResult, error) {
	workspaceID, rolePath, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	out := &product.IAMDataPrivilegeViewResult{}
	path := "/workspaces/" + workspaceID + "/iam/roles/" + rolePath + "/data-privileges"
	if err := s.do(ctx, http.MethodGet, path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ValidateDataPrivileges validates a data-privilege mutation against the
// current role, policy, and capability state without writing MatrixOne DCL.
func (s *IAMService) ValidateDataPrivileges(ctx context.Context, roleID string, request *IAMDataPrivilegeMutationRequest) (*IAMDataPrivilegeValidationResult, error) {
	if request == nil {
		return nil, iamRequestRequired()
	}
	workspaceID, rolePath, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	out := &product.IAMDataPrivilegeValidationResult{}
	path := "/workspaces/" + workspaceID + "/iam/roles/" + rolePath + "/data-privileges:validate"
	if err := s.do(ctx, http.MethodPost, path, iamDataPrivilegeMutationBody(request), out); err != nil {
		return nil, err
	}
	return out, nil
}

// ApplyDataPrivileges applies a validated data-privilege mutation atomically.
func (s *IAMService) ApplyDataPrivileges(ctx context.Context, roleID string, request *IAMDataPrivilegeMutationRequest) (*IAMDataPrivilegeOperationResult, error) {
	if request == nil {
		return nil, iamRequestRequired()
	}
	workspaceID, rolePath, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	out := &product.IAMDataPrivilegeOperationResult{}
	path := "/workspaces/" + workspaceID + "/iam/roles/" + rolePath + "/data-privileges"
	if err := s.do(ctx, http.MethodPut, path, iamDataPrivilegeMutationBody(request), out); err != nil {
		return nil, err
	}
	return out, nil
}

// DataPrivilegeOperation returns a durable data-privilege operation by its
// request ID.
func (s *IAMService) DataPrivilegeOperation(ctx context.Context, roleID, requestID string) (*IAMDataPrivilegeOperationResult, error) {
	workspaceID, rolePath, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	requestPath, err := pathID(requestID)
	if err != nil {
		return nil, err
	}
	out := &product.IAMDataPrivilegeOperationResult{}
	path := "/workspaces/" + workspaceID + "/iam/roles/" + rolePath + "/data-privilege-operations/" + requestPath
	if err := s.do(ctx, http.MethodGet, path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// RepairDataPrivileges retries a previously recorded data-privilege operation
// through the service's durable repair contract.
func (s *IAMService) RepairDataPrivileges(ctx context.Context, roleID, repairOperation string, request *IAMDataPrivilegeRepairRequest) (*IAMDataPrivilegeOperationResult, error) {
	if request == nil {
		return nil, iamRequestRequired()
	}
	workspaceID, rolePath, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	repairPath, err := pathID(repairOperation)
	if err != nil {
		return nil, err
	}
	out := &product.IAMDataPrivilegeOperationResult{}
	path := "/workspaces/" + workspaceID + "/iam/roles/" + rolePath + "/data-privilege-operations/" + repairPath
	if err := s.do(ctx, http.MethodPost, path, map[string]any{
		"request_id": request.GetRequestId(), "expected_data_privilege_version": request.GetExpectedDataPrivilegeVersion(),
	}, out); err != nil {
		return nil, err
	}
	return out, nil
}

func iamObjectGrantsBody(request *IAMObjectGrantsMutationRequest) map[string]any {
	changes := make([]map[string]any, len(request.GetChanges()))
	for index, change := range request.GetChanges() {
		changes[index] = map[string]any{
			"remove": change.GetRemove(), "action_id": change.GetActionId(), "condition_hash": change.GetConditionHash(),
		}
	}
	return map[string]any{
		"schema_version": request.GetSchemaVersion(), "expected_policy_version": request.GetExpectedPolicyVersion(),
		"expected_role_lifecycle_version": request.GetExpectedRoleLifecycleVersion(),
		"request_id":                      request.GetRequestId(), "changes": changes,
	}
}

func iamDataPrivilegeMutationBody(request *IAMDataPrivilegeMutationRequest) map[string]any {
	mutations := make([]map[string]any, len(request.GetMutations()))
	for index, mutation := range request.GetMutations() {
		item := map[string]any{
			"operation": mutation.GetOperation(), "action_id": mutation.GetActionId(),
			"resource_type": mutation.GetResourceType(), "resource_id": mutation.GetResourceId(),
			"privilege": mutation.GetPrivilege(), "with_grant_option": mutation.GetWithGrantOption(),
		}
		if row := mutation.GetRowColumn(); row != nil {
			encodedRow := map[string]any{"mode": row.GetMode()}
			if draft := row.GetDraft(); draft != nil {
				rules := make([]map[string]any, len(draft.GetRuleList()))
				for ruleIndex, rule := range draft.GetRuleList() {
					expressions := make([]map[string]any, len(rule.GetExpressionList()))
					for expressionIndex, expression := range rule.GetExpressionList() {
						expressions[expressionIndex] = map[string]any{
							"operator": expression.GetOperator(), "expression": expression.GetExpression(), "match_type": expression.GetMatchType(),
						}
					}
					rules[ruleIndex] = map[string]any{
						"column": rule.GetColumn(), "relation": rule.GetRelation(), "expression_list": expressions,
					}
				}
				encodedRow["draft"] = map[string]any{
					"black_column_list": draft.GetBlackColumnList(), "rule_list": rules,
				}
			}
			item["row_column"] = encodedRow
		}
		mutations[index] = item
	}
	return map[string]any{
		"request_id": request.GetRequestId(), "expected_data_privilege_version": request.GetExpectedDataPrivilegeVersion(),
		"mutations": mutations,
	}
}

func (s *IAMService) workspacePathID() (string, error) {
	if s.workspaceID == "" {
		return "", ErrWorkspaceRequired
	}
	return pathID(s.workspaceID)
}

func (s *IAMService) workspaceRolePathIDs(roleID string) (string, string, error) {
	workspacePath, err := s.workspacePathID()
	if err != nil {
		return "", "", err
	}
	rolePath, err := pathID(roleID)
	if err != nil {
		return "", "", err
	}
	return workspacePath, rolePath, nil
}

func (s *IAMService) workspaceUserPathIDs(userID string) (string, string, error) {
	workspacePath, err := s.workspacePathID()
	if err != nil {
		return "", "", err
	}
	userPath, err := pathID(userID)
	if err != nil {
		return "", "", err
	}
	return workspacePath, userPath, nil
}

func (s *IAMService) objectPathIDs(resourceType, resourceID string) (string, string, string, error) {
	workspacePath, err := s.workspacePathID()
	if err != nil {
		return "", "", "", err
	}
	if resourceType == "" {
		return "", "", "", ErrTypeRequired
	}
	resourceTypePath, err := pathID(resourceType)
	if err != nil {
		return "", "", "", err
	}
	resourceIDPath, err := pathID(resourceID)
	if err != nil {
		return "", "", "", err
	}
	return workspacePath, resourceTypePath, resourceIDPath, nil
}

func (s *IAMService) objectGrantPathIDs(resourceType, resourceID, roleID string) (string, string, string, string, error) {
	workspacePath, resourceTypePath, resourceIDPath, err := s.objectPathIDs(resourceType, resourceID)
	if err != nil {
		return "", "", "", "", err
	}
	rolePath, err := pathID(roleID)
	if err != nil {
		return "", "", "", "", err
	}
	return workspacePath, resourceTypePath, resourceIDPath, rolePath, nil
}

func iamRequestRequired() error {
	return &Error{Code: "ErrRequestRequired", Message: "request is required"}
}

func (s *IAMService) RoleInheritance(ctx context.Context, roleID string) (result *IAMRoleInheritanceResult, err error) {
	workspaceID, id, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	result = &IAMRoleInheritanceResult{}
	err = s.do(ctx, http.MethodGet, "/workspaces/"+workspaceID+"/iam/roles/"+id+"/inheritance", nil, result)
	return result, err
}

func (s *IAMService) RoleInheritanceGraph(ctx context.Context, roleID string) (result *IAMRoleInheritanceResult, err error) {
	workspaceID, id, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	result = &IAMRoleInheritanceResult{}
	err = s.do(ctx, http.MethodGet, "/workspaces/"+workspaceID+"/iam/roles/"+id+"/inheritance-graph", nil, result)
	return result, err
}

func (s *IAMService) AddRoleInheritance(ctx context.Context, roleID string, request *IAMRoleInheritanceMutationRequest) (result *IAMRoleInheritanceMutationResult, err error) {
	return s.mutateRoleInheritance(ctx, roleID, request, true)
}

func (s *IAMService) RemoveRoleInheritance(ctx context.Context, roleID string, request *IAMRoleInheritanceMutationRequest) (result *IAMRoleInheritanceMutationResult, err error) {
	return s.mutateRoleInheritance(ctx, roleID, request, false)
}

func (s *IAMService) mutateRoleInheritance(ctx context.Context, roleID string, request *IAMRoleInheritanceMutationRequest, add bool) (*IAMRoleInheritanceMutationResult, error) {
	if request == nil {
		return nil, iamRequestRequired()
	}
	workspaceID, id, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	path, method := "/workspaces/"+workspaceID+"/iam/roles/"+id+"/parents", http.MethodPost
	if !add {
		parentID, err := pathID(request.GetParentRoleId())
		if err != nil {
			return nil, err
		}
		path, method = path+"/"+parentID, http.MethodDelete
	}
	body := map[string]any{
		"parent_role_id":                    request.GetParentRoleId(),
		"expected_child_lifecycle_version":  request.GetExpectedChildLifecycleVersion(),
		"expected_parent_lifecycle_version": request.GetExpectedParentLifecycleVersion(),
		"request_id":                        request.GetRequestId(),
	}
	result := &IAMRoleInheritanceMutationResult{}
	err = s.do(ctx, method, path, body, result)
	return result, err
}

func (s *IAMService) ValidateApplicationPolicy(ctx context.Context, roleID string, request *IAMApplicationPolicyRequest) (*IAMApplicationPolicyValidationResult, error) {
	if request == nil {
		return nil, iamRequestRequired()
	}
	workspaceID, id, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	result := &IAMApplicationPolicyValidationResult{}
	err = s.do(ctx, http.MethodPost, "/workspaces/"+workspaceID+"/iam/roles/"+id+"/application-policy:validate", iamApplicationPolicyBody(request), result)
	return result, err
}

func (s *IAMService) ApplyApplicationPolicy(ctx context.Context, roleID string, request *IAMApplicationPolicyRequest) (*IAMApplicationPolicyApplyResult, error) {
	if request == nil {
		return nil, iamRequestRequired()
	}
	workspaceID, id, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	result := &IAMApplicationPolicyApplyResult{}
	err = s.do(ctx, http.MethodPut, "/workspaces/"+workspaceID+"/iam/roles/"+id+"/application-policy", iamApplicationPolicyBody(request), result)
	return result, err
}

func iamApplicationPolicyBody(request *IAMApplicationPolicyRequest) map[string]any {
	statements := make([]map[string]any, len(request.GetStatements()))
	for index, statement := range request.GetStatements() {
		statements[index] = map[string]any{
			"statement_id": statement.GetStatementId(), "action_id": statement.GetActionId(),
			"resource_type": statement.GetResourceType(), "scope_mode": statement.GetScopeMode(),
			"resource_id": statement.GetResourceId(), "condition_hash": statement.GetConditionHash(),
			"target_action_id": statement.GetTargetActionId(), "grant_scope_mode": statement.GetGrantScopeMode(),
			"grant_resource_id": statement.GetGrantResourceId(), "grant_delegatable": statement.GetGrantDelegatable(),
		}
	}
	return map[string]any{
		"schema_version":                  request.GetSchemaVersion(),
		"expected_policy_version":         request.GetExpectedPolicyVersion(),
		"expected_role_lifecycle_version": request.GetExpectedRoleLifecycleVersion(),
		"request_id":                      request.GetRequestId(), "statements": statements,
	}
}

func (s *IAMService) CurrentPrincipalRoles(ctx context.Context) (*IAMCurrentPrincipalRolesResult, error) {
	workspaceID, err := s.workspacePathID()
	if err != nil {
		return nil, err
	}
	result := &IAMCurrentPrincipalRolesResult{}
	err = s.do(ctx, http.MethodGet, "/workspaces/"+workspaceID+"/iam/me/roles", nil, result)
	return result, err
}

// CurrentPrincipalCapabilities queries only the requested capability IDs. Omitting
// IDs returns the full projection. Unsupported IDs are reported separately in the
// result; supported items preserve their authorization decisions and scope.
func (s *IAMService) CurrentPrincipalCapabilities(ctx context.Context, capabilityIDs ...string) (*IAMCurrentPrincipalCapabilitiesResult, error) {
	workspaceID, err := s.workspacePathID()
	if err != nil {
		return nil, err
	}
	result := &IAMCurrentPrincipalCapabilitiesResult{}
	path := withQuery("/workspaces/"+workspaceID+"/iam/me/capabilities", []QueryOption{WithQueryValues("capability_id", capabilityIDs...)})
	err = s.do(ctx, http.MethodGet, path, nil, result)
	return result, err
}

type IAMRoleMembersResult = product.IAMRoleMembersResult

func (s *IAMService) RoleMembers(ctx context.Context, roleID string, opts ...QueryOption) (*IAMRoleMembersResult, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	role, err := pathID(roleID)
	if err != nil {
		return nil, err
	}
	out := &IAMRoleMembersResult{}
	err = s.do(ctx, http.MethodGet, withQuery("/workspaces/"+workspace+"/iam/roles/"+role+"/members", opts), nil, out)
	return out, err
}
