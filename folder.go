package sdk

import (
	"context"
)

func (c *RawClient) CreateFolder(ctx context.Context, req *FolderCreateRequest, opts ...CallOption) (*FolderCreateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FolderCreateResponse
	if err := c.postJSON(ctx, "/catalog/folder/create", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateFolder(ctx context.Context, req *FolderUpdateRequest, opts ...CallOption) (*FolderUpdateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FolderUpdateResponse
	if err := c.postJSON(ctx, "/catalog/folder/update", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) DeleteFolder(ctx context.Context, req *FolderDeleteRequest, opts ...CallOption) (*FolderDeleteResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FolderDeleteResponse
	if err := c.postJSON(ctx, "/catalog/folder/delete", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) CleanFolder(ctx context.Context, req *FolderCleanRequest, opts ...CallOption) (*FolderCleanResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FolderCleanResponse
	if err := c.postJSON(ctx, "/catalog/folder/clean", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetFolderRefList(ctx context.Context, req *FolderRefListRequest, opts ...CallOption) (*FolderRefListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FolderRefListResponse
	if err := c.postJSON(ctx, "/catalog/folder/ref_list", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}
