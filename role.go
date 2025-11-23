package sdk

import (
	"context"
)

func (c *RawClient) CreateRole(ctx context.Context, req *RoleCreateRequest, opts ...CallOption) (*RoleCreateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp RoleCreateResponse
	if err := c.postJSON(ctx, "/role/create", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) DeleteRole(ctx context.Context, req *RoleDeleteRequest, opts ...CallOption) (*RoleDeleteResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp RoleDeleteResponse
	if err := c.postJSON(ctx, "/role/delete", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetRole(ctx context.Context, req *RoleInfoRequest, opts ...CallOption) (*RoleInfoResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp RoleInfoResponse
	if err := c.postJSON(ctx, "/role/info", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) ListRoles(ctx context.Context, req *RoleListRequest, opts ...CallOption) (*RoleListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp RoleListResponse
	if err := c.postJSON(ctx, "/role/list", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) ListRolesByCategoryAndObject(ctx context.Context, req *RoleListByCategoryAndObjectRequest, opts ...CallOption) (*RoleListByCategoryAndObjectResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp RoleListByCategoryAndObjectResponse
	if err := c.postJSON(ctx, "/role/list_by_category_and_obj", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateRoleCodeList(ctx context.Context, req *RoleUpdateCodeListRequest, opts ...CallOption) (*RoleUpdateCodeListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp RoleUpdateCodeListResponse
	if err := c.postJSON(ctx, "/role/update_code_list", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateRoleInfo updates role information including privileges.
// The request can include:
//   - PrivList: global privilege codes (string array)
//   - ObjPrivList: object privileges with optional rules (ObjPrivResponse array)
//     Each ObjPrivResponse contains AuthorityCodeList which is []*AuthorityCodeAndRule,
//     where each AuthorityCodeAndRule can have RuleList for row/column level permissions.
func (c *RawClient) UpdateRoleInfo(ctx context.Context, req *RoleUpdateInfoRequest, opts ...CallOption) (*RoleUpdateInfoResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp RoleUpdateInfoResponse
	if err := c.postJSON(ctx, "/role/update_info", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateRolesByObject(ctx context.Context, req *RoleUpdateRolesByObjectRequest, opts ...CallOption) (*RoleUpdateRolesByObjectResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp RoleUpdateRolesByObjectResponse
	if err := c.postJSON(ctx, "/role/update_roles_by_obj", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateRoleStatus(ctx context.Context, req *RoleUpdateStatusRequest, opts ...CallOption) (*RoleUpdateStatusResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp RoleUpdateStatusResponse
	if err := c.postJSON(ctx, "/role/update_status", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}
