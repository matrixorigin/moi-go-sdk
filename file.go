package sdk

import (
	"context"
)

func (c *RawClient) CreateFile(ctx context.Context, req *FileCreateRequest, opts ...CallOption) (*FileCreateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FileCreateResponse
	if err := c.postJSON(ctx, "/catalog/file/create", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateFile(ctx context.Context, req *FileUpdateRequest, opts ...CallOption) (*FileUpdateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FileUpdateResponse
	if err := c.postJSON(ctx, "/catalog/file/update", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) DeleteFile(ctx context.Context, req *FileDeleteRequest, opts ...CallOption) (*FileDeleteResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FileDeleteResponse
	if err := c.postJSON(ctx, "/catalog/file/delete", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) DeleteFileRef(ctx context.Context, req *FileDeleteRefRequest, opts ...CallOption) (*FileDeleteRefResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FileDeleteRefResponse
	if err := c.postJSON(ctx, "/catalog/file/delete_ref", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetFile(ctx context.Context, req *FileInfoRequest, opts ...CallOption) (*FileInfoResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FileInfoResponse
	if err := c.postJSON(ctx, "/catalog/file/info", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) ListFiles(ctx context.Context, req *FileListRequest, opts ...CallOption) (*FileListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FileListResponse
	if err := c.postJSON(ctx, "/catalog/file/list", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UploadFile(ctx context.Context, req *FileUploadRequest, opts ...CallOption) (*FileUploadResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FileUploadResponse
	if err := c.postJSON(ctx, "/catalog/file/upload", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetFileDownloadLink(ctx context.Context, req *FileDownloadRequest, opts ...CallOption) (*FileDownloadResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FileDownloadResponse
	if err := c.postJSON(ctx, "/catalog/file/download", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetFilePreviewLink(ctx context.Context, req *FilePreviewLinkRequest, opts ...CallOption) (*FilePreviewLinkResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FilePreviewLinkResponse
	if err := c.postJSON(ctx, "/catalog/file/preview_link", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetFilePreviewStream(ctx context.Context, req *FilePreviewStreamRequest, opts ...CallOption) (*FilePreviewLinkResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp FilePreviewLinkResponse
	if err := c.postJSON(ctx, "/catalog/file/preview_stream", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}
