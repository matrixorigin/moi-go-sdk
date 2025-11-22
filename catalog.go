package sdk

import (
	"context"
)

func (c *RawClient) CreateCatalog(ctx context.Context, req *CatalogCreateRequest, opts ...CallOption) (*CatalogCreateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp CatalogCreateResponse
	if err := c.postJSON(ctx, "/catalog/create", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) DeleteCatalog(ctx context.Context, req *CatalogDeleteRequest, opts ...CallOption) (*CatalogDeleteResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp CatalogDeleteResponse
	if err := c.postJSON(ctx, "/catalog/delete", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateCatalog(ctx context.Context, req *CatalogUpdateRequest, opts ...CallOption) (*CatalogUpdateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp CatalogUpdateResponse
	if err := c.postJSON(ctx, "/catalog/update", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetCatalog(ctx context.Context, req *CatalogInfoRequest, opts ...CallOption) (*CatalogInfoResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp CatalogInfoResponse
	if err := c.postJSON(ctx, "/catalog/info", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) ListCatalogs(ctx context.Context, opts ...CallOption) (*CatalogListResponse, error) {
	var resp CatalogListResponse
	if err := c.postJSON(ctx, "/catalog/list", struct{}{}, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetCatalogTree(ctx context.Context, opts ...CallOption) (*CatalogTreeResponse, error) {
	var resp CatalogTreeResponse
	if err := c.postJSON(ctx, "/catalog/tree", struct{}{}, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetCatalogRefList(ctx context.Context, req *CatalogRefListRequest, opts ...CallOption) (*CatalogRefListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp CatalogRefListResponse
	if err := c.postJSON(ctx, "/catalog/ref_list", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}
