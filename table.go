package sdk

import (
	"context"
)

func (c *RawClient) CreateTable(ctx context.Context, req *TableCreateRequest, opts ...CallOption) (*TableCreateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp TableCreateResponse
	if err := c.postJSON(ctx, "/catalog/table/create", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetTable(ctx context.Context, req *TableInfoRequest, opts ...CallOption) (*TableInfoResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp TableInfoResponse
	if err := c.postJSON(ctx, "/catalog/table/info", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetTableOverview(ctx context.Context, opts ...CallOption) ([]TableOverview, error) {
	var resp []TableOverview
	if err := c.postJSON(ctx, "/catalog/table/overview", struct{}{}, &resp, opts...); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *RawClient) CheckTableExists(ctx context.Context, req *TableExistRequest, opts ...CallOption) (bool, error) {
	if req == nil {
		return false, ErrNilRequest
	}
	var exists bool
	if err := c.postJSON(ctx, "/catalog/table/exist", req, &exists, opts...); err != nil {
		return false, err
	}
	return exists, nil
}

func (c *RawClient) PreviewTable(ctx context.Context, req *TablePreviewRequest, opts ...CallOption) (*TablePreviewResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp TablePreviewResponse
	if err := c.postJSON(ctx, "/catalog/table/preview", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) LoadTable(ctx context.Context, req *TableLoadRequest, opts ...CallOption) (*TableLoadResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp TableLoadResponse
	if err := c.postJSON(ctx, "/catalog/table/load", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetTableDownloadLink(ctx context.Context, req *TableDownloadRequest, opts ...CallOption) (*TableDownloadResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp TableDownloadResponse
	if err := c.postJSON(ctx, "/catalog/table/download", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) TruncateTable(ctx context.Context, req *TableTruncateRequest, opts ...CallOption) (*TableTruncateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp TableTruncateResponse
	if err := c.postJSON(ctx, "/catalog/table/truncate", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) DeleteTable(ctx context.Context, req *TableDeleteRequest, opts ...CallOption) (*TableDeleteResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp TableDeleteResponse
	if err := c.postJSON(ctx, "/catalog/table/delete", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetTableFullPath(ctx context.Context, req *TableFullPathRequest, opts ...CallOption) (*TableFullPathResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp TableFullPathResponse
	if err := c.postJSON(ctx, "/catalog/table/full_path", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetTableRefList(ctx context.Context, req *TableRefListRequest, opts ...CallOption) (*TableRefListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp TableRefListResponse
	if err := c.postJSON(ctx, "/catalog/table/ref_list", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}
