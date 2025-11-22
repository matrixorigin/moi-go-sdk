package sdk

import (
	"context"
)

func (c *RawClient) CreateDatabase(ctx context.Context, req *DatabaseCreateRequest, opts ...CallOption) (*DatabaseCreateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp DatabaseCreateResponse
	if err := c.postJSON(ctx, "/catalog/database/create", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) DeleteDatabase(ctx context.Context, req *DatabaseDeleteRequest, opts ...CallOption) (*DatabaseDeleteResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp DatabaseDeleteResponse
	if err := c.postJSON(ctx, "/catalog/database/delete", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateDatabase(ctx context.Context, req *DatabaseUpdateRequest, opts ...CallOption) (*DatabaseUpdateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp DatabaseUpdateResponse
	if err := c.postJSON(ctx, "/catalog/database/update", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetDatabase(ctx context.Context, req *DatabaseInfoRequest, opts ...CallOption) (*DatabaseInfoResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp DatabaseInfoResponse
	if err := c.postJSON(ctx, "/catalog/database/info", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) ListDatabases(ctx context.Context, req *DatabaseListRequest, opts ...CallOption) (*DatabaseListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp DatabaseListResponse
	if err := c.postJSON(ctx, "/catalog/database/list", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetDatabaseChildren(ctx context.Context, req *DatabaseChildrenRequest, opts ...CallOption) (*DatabaseChildrenResponseData, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp DatabaseChildrenResponseData
	if err := c.postJSON(ctx, "/catalog/database/children", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetDatabaseRefList(ctx context.Context, req *DatabaseRefListRequest, opts ...CallOption) (*DatabaseRefListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp DatabaseRefListResponse
	if err := c.postJSON(ctx, "/catalog/database/ref_list", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}
