package sdk

import (
	"context"
	"net/http"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

// TaskExecutionOverviewResult is the dashboard task execution summary for the recent window.
type TaskExecutionOverviewResult = product.TaskExecutionOverviewResult

// TaskExecutionOverview returns the recent task execution status summary.
func (s *DashboardService) TaskExecutionOverview(ctx context.Context) (*TaskExecutionOverviewResult, error) {
	out := &TaskExecutionOverviewResult{}
	err := s.do(ctx, http.MethodGet, "/dashboard/task-execution-overview", nil, out)
	return out, err
}
