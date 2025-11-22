package sdk

import (
	"context"
)

func (c *RawClient) RunNL2SQL(ctx context.Context, req *NL2SQLRunSQLRequest, opts ...CallOption) (*NL2SQLRunSQLResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	var resp NL2SQLRunSQLResponse
	if err := c.postJSON(ctx, "/nl2sql/run_sql", req, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}
