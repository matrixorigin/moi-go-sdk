package sdk

import (
	"context"
)

func (c *RawClient) CreateUser(ctx context.Context, req *UserCreateRequest, opts ...CallOption) (*UserCreateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp UserCreateResponse
	if err := c.postJSON(ctx, "/user/create", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) DeleteUser(ctx context.Context, req *UserDeleteUserRequest, opts ...CallOption) (*UserDeleteUserResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp UserDeleteUserResponse
	if err := c.postJSON(ctx, "/user/delete", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetUserDetail(ctx context.Context, req *UserDetailInfoRequest, opts ...CallOption) (*UserDetailInfoResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp UserDetailInfoResponse
	if err := c.postJSON(ctx, "/user/detail_info", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) ListUsers(ctx context.Context, req *UserListRequest, opts ...CallOption) (*UserListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp UserListResponse
	if err := c.postJSON(ctx, "/user/list", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateUserPassword(ctx context.Context, req *UserUpdatePasswordRequest, opts ...CallOption) (*UserUpdatePasswordResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp UserUpdatePasswordResponse
	if err := c.postJSON(ctx, "/user/update_password", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateUserInfo(ctx context.Context, req *UserUpdateInfoRequest, opts ...CallOption) (*UserUpdateInfoResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp UserUpdateInfoResponse
	if err := c.postJSON(ctx, "/user/update_info", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateUserRoles(ctx context.Context, req *UserUpdateRoleListRequest, opts ...CallOption) (*UserUpdateRoleListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp UserUpdateRoleListResponse
	if err := c.postJSON(ctx, "/user/update_role_list", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateUserStatus(ctx context.Context, req *UserUpdateStatusRequest, opts ...CallOption) (*UserUpdateStatusResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp UserUpdateStatusResponse
	if err := c.postJSON(ctx, "/user/update_status", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetMyAPIKey(ctx context.Context, opts ...CallOption) (*UserApiKeyResponse, error) {
	var resp UserApiKeyResponse
	if err := c.postJSON(ctx, "/user/me/api-key", nil, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) RefreshMyAPIKey(ctx context.Context, opts ...CallOption) (*UserApiKeyRefreshResonse, error) {
	var resp UserApiKeyRefreshResonse
	if err := c.postJSON(ctx, "/user/me/api-key/refresh", nil, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetMyInfo(ctx context.Context, opts ...CallOption) (*UserMeInfoResponse, error) {
	var resp UserMeInfoResponse
	if err := c.postJSON(ctx, "/user/me/info", nil, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateMyInfo(ctx context.Context, req *UserMeUpdateInfoRequest, opts ...CallOption) (*UserMeUpdateInfoResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp UserMeUpdateInfoResponse
	if err := c.postJSON(ctx, "/user/me/update_info", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateMyPassword(ctx context.Context, req *UserMeUpdatePasswordRequest, opts ...CallOption) (*UserMeUpdatePasswordResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp UserMeUpdatePasswordResponse
	if err := c.postJSON(ctx, "/user/me/update_password", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}
