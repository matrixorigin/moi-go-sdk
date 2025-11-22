package sdk

import (
	"context"
)

func (c *RawClient) ListUserLogs(ctx context.Context, req *LogLogListRequest, opts ...CallOption) (*LogLogListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp LogLogListResponse
	if err := c.postJSON(ctx, "/log/user", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) ListRoleLogs(ctx context.Context, req *LogLogListRequest, opts ...CallOption) (*LogLogListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp LogLogListResponse
	if err := c.postJSON(ctx, "/log/role", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}
