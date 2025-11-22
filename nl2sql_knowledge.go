package sdk

import (
	"context"
)

func (c *RawClient) CreateKnowledge(ctx context.Context, req *NL2SQLKnowledgeCreateRequest, opts ...CallOption) (*NL2SQLKnowledgeCreateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp NL2SQLKnowledgeCreateResponse
	if err := c.postJSON(ctx, "/catalog/nl2sql_knowledge/create", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) UpdateKnowledge(ctx context.Context, req *NL2SQLKnowledgeUpdateRequest, opts ...CallOption) (*NL2SQLKnowledgeUpdateResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp NL2SQLKnowledgeUpdateResponse
	if err := c.postJSON(ctx, "/catalog/nl2sql_knowledge/update", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) DeleteKnowledge(ctx context.Context, req *NL2SQLKnowledgeDeleteRequest, opts ...CallOption) (*NL2SQLKnowledgeDeleteResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp NL2SQLKnowledgeDeleteResponse
	if err := c.postJSON(ctx, "/catalog/nl2sql_knowledge/delete", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) GetKnowledge(ctx context.Context, req *NL2SQLKnowledgeGetRequest, opts ...CallOption) (*NL2SQLKnowledgeGetResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp NL2SQLKnowledgeGetResponse
	if err := c.postJSON(ctx, "/catalog/nl2sql_knowledge/get", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) ListKnowledge(ctx context.Context, req *NL2SQLKnowledgeListRequest, opts ...CallOption) (*NL2SQLKnowledgeListResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp NL2SQLKnowledgeListResponse
	if err := c.postJSON(ctx, "/catalog/nl2sql_knowledge/list", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) SearchKnowledge(ctx context.Context, req *NL2SQLKnowledgeSearchRequest, opts ...CallOption) (*NL2SQLKnowledgeSearchResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp NL2SQLKnowledgeSearchResponse
	if err := c.postJSON(ctx, "/catalog/nl2sql_knowledge/search", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}
