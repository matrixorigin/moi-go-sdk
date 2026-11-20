package sdk

import (
	"context"
	"net/http"
)

// DownloadQueryResult streams the complete persisted statement result as CSV.
// The caller owns the returned response body.
func (s *SQLService) DownloadQueryResult(ctx context.Context, statementID string) (*StreamResponse, error) {
	if statementID == "" {
		return nil, ErrStatementIDRequired
	}
	return s.doDownload(ctx, http.MethodPost, "/query/result/download", map[string]string{"statement_id": statementID})
}
