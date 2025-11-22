package sdk

import (
	"context"
)

func (c *RawClient) ListObjectsByCategory(ctx context.Context, req *PrivListObjByCategoryRequest, opts ...CallOption) (*PrivListObjByCategoryResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp PrivListObjByCategoryResponse
	if err := c.postJSON(ctx, "/rbac/priv/list_obj_by_category", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}
