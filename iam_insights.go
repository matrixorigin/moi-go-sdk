package sdk

import (
	"context"
	"net/http"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

type IAMSetDefaultRoleRequest = product.IAMSetDefaultRoleRequest
type IAMSetDefaultRoleResult = product.IAMSetDefaultRoleResult
type IAMOwnershipImpactResult = product.IAMOwnershipImpactResult
type IAMOwnershipImpactItem = product.IAMOwnershipImpactItem
type IAMOwnershipImpactTypeCount = product.IAMOwnershipImpactTypeCount
type IAMRoleAuditEvent = product.IAMRoleAuditEvent
type IAMRoleAuditEventsResult = product.IAMRoleAuditEventsResult
type IAMUserAuditEvent = product.IAMUserAuditEvent
type IAMUserAuditSummary = product.IAMUserAuditSummary
type IAMUserAuditEventsResult = product.IAMUserAuditEventsResult

// OwnershipImpact 查询归属观测信息，不执行角色删除预检。
func (s *IAMService) OwnershipImpact(ctx context.Context, roleID string, opts ...QueryOption) (*IAMOwnershipImpactResult, error) {
	workspaceID, rolePath, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	out := &IAMOwnershipImpactResult{}
	err = s.do(ctx, http.MethodGet, withQuery("/workspaces/"+workspaceID+"/iam/roles/"+rolePath+"/ownership-impact", opts), nil, out)
	return out, err
}

// RoleAuditEvents 保留审计 diff、版本和下一页游标。
func (s *IAMService) RoleAuditEvents(ctx context.Context, roleID string, opts ...QueryOption) (*IAMRoleAuditEventsResult, error) {
	workspaceID, rolePath, err := s.workspaceRolePathIDs(roleID)
	if err != nil {
		return nil, err
	}
	out := &IAMRoleAuditEventsResult{}
	err = s.do(ctx, http.MethodGet, withQuery("/workspaces/"+workspaceID+"/iam/roles/"+rolePath+"/audit-events", opts), nil, out)
	return out, err
}

// UserAuditEvents 查询 canonical 用户的工作区成员审计。
func (s *IAMService) UserAuditEvents(ctx context.Context, userID string, opts ...QueryOption) (*IAMUserAuditEventsResult, error) {
	workspaceID, userPath, err := s.workspaceUserPathIDs(userID)
	if err != nil {
		return nil, err
	}
	out := &IAMUserAuditEventsResult{}
	err = s.do(ctx, http.MethodGet, withQuery("/workspaces/"+workspaceID+"/iam/users/"+userPath+"/audit-events", opts), nil, out)
	return out, err
}

// SetDefaultRole 使用调用者提供的版本与请求 ID 修改自身默认角色。
func (s *IAMService) SetDefaultRole(ctx context.Context, request *IAMSetDefaultRoleRequest) (*IAMSetDefaultRoleResult, error) {
	if request == nil {
		return nil, iamRequestRequired()
	}
	workspaceID, err := s.workspacePathID()
	if err != nil {
		return nil, err
	}
	body := map[string]any{"target_role_id": request.GetTargetRoleId(), "expected_default_role_version": request.GetExpectedDefaultRoleVersion(), "request_id": request.GetRequestId()}
	out := &IAMSetDefaultRoleResult{}
	err = s.do(ctx, http.MethodPut, "/workspaces/"+workspaceID+"/iam/me/default-role", body, out)
	return out, err
}
