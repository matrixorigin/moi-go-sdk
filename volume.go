package sdk

import (
	"context"
)

func (c *RawClient) CreateVolume(ctx context.Context, req *VolumeCreateRequest, opts ...CallOption) (*VolumeCreateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp VolumeCreateResponse
	if err := c.postJSON(ctx, "/catalog/volume/create", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) DeleteVolume(ctx context.Context, req *VolumeDeleteRequest, opts ...CallOption) (*VolumeDeleteResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp VolumeDeleteResponse
	if err := c.postJSON(ctx, "/catalog/volume/delete", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateVolume(ctx context.Context, req *VolumeUpdateRequest, opts ...CallOption) (*VolumeUpdateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp VolumeUpdateResponse
	if err := c.postJSON(ctx, "/catalog/volume/update", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetVolume(ctx context.Context, req *VolumeInfoRequest, opts ...CallOption) (*VolumeInfoResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp VolumeInfoResponse
	if err := c.postJSON(ctx, "/catalog/volume/info", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetVolumeRefList(ctx context.Context, req *VolumeRefListRequest, opts ...CallOption) (*VolumeRefListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp VolumeRefListResponse
	if err := c.postJSON(ctx, "/catalog/volume/ref_list", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetVolumeFullPath(ctx context.Context, req *VolumeFullPathRequest, opts ...CallOption) (*VolumeFullPathResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp VolumeFullPathResponse
	if err := c.postJSON(ctx, "/catalog/volume/full_path", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) AddVolumeWorkflowRef(ctx context.Context, req *VolumeAddRefWorkflowRequest, opts ...CallOption) (*VolumeAddRefWorkflowResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp VolumeAddRefWorkflowResponse
	if err := c.postJSON(ctx, "/catalog/volume/add_ref_workflow", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) RemoveVolumeWorkflowRef(ctx context.Context, req *VolumeRemoveRefWorkflowRequest, opts ...CallOption) (*VolumeRemoveRefWorkflowResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp VolumeRemoveRefWorkflowResponse
	if err := c.postJSON(ctx, "/catalog/volume/remove_ref_workflow", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}
