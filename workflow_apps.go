package sdk

import (
	"context"
	"net/http"
)

// Apps lists workflow applications using the same product route as the UI.
func (s *WorkflowService) Apps(ctx context.Context, options ...QueryOption) (*ProductData, error) {
	result := &ProductData{}
	err := s.do(ctx, http.MethodGet, withQuery("/workflow/v2/workflow-apps", options), nil, result)
	return result, err
}
