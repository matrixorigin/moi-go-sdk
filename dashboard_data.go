package sdk

import (
	"context"
	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"net/http"
)

type DataDashboardSourceInput = product.DataDashboardSourceInput
type DataDashboardMutationRequest = product.DataDashboardMutationRequest
type DataDashboardListRequest = product.DataDashboardListRequest
type DataDashboardChartMutationRequest = product.DataDashboardChartMutationRequest
type DataDashboardDrillThroughDimension = product.DataDashboardDrillThroughDimension
type DataDashboardDrillThroughRequest = product.DataDashboardDrillThroughRequest
type DataDashboardChartOrderRequest = product.DataDashboardChartOrderRequest
type DataDashboardListResult = product.DataDashboardListResult
type DataDashboardMutationResult = product.DataDashboardMutationResult
type DataDashboardGeneratedSQLResult = product.DataDashboardGeneratedSQLResult
type DataDashboardChartRun = product.DataDashboardChartRun
type DataDashboardChartRuns = product.DataDashboardChartRuns
type DataDashboardChartExecution = product.DataDashboardChartExecution
type DataDashboardDrillThroughResult = product.DataDashboardDrillThroughResult
type DataDashboard = product.DataDashboard
type DataDashboardSource = product.DataDashboardSource
type DataDashboardChartSummary = product.DataDashboardChartSummary
type DataDashboardChart = product.DataDashboardChart
type DataDashboardAlert = product.DataDashboardAlert

func (s *DashboardService) Create(ctx context.Context, request *DataDashboardMutationRequest) (*DataDashboard, error) {
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "dashboard request is required"}
	}
	path := "/data-dashboards"
	out := &DataDashboard{}
	callErr := s.do(ctx, http.MethodPost, path, request, out)
	return out, callErr
}

func (s *DashboardService) List(ctx context.Context, request *DataDashboardListRequest) (*DataDashboardListResult, error) {
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "dashboard request is required"}
	}
	path := "/data-dashboards" + "/list"
	out := &DataDashboardListResult{}
	callErr := s.do(ctx, http.MethodPost, path, request, out)
	return out, callErr
}

func (s *DashboardService) Get(ctx context.Context, dashboardID string) (*DataDashboard, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	path := "/data-dashboards" + "/" + dashboard
	out := &DataDashboard{}
	callErr := s.do(ctx, http.MethodGet, path, nil, out)
	return out, callErr
}

func (s *DashboardService) Update(ctx context.Context, dashboardID string, request *DataDashboardMutationRequest) (*DataDashboardMutationResult, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "dashboard request is required"}
	}
	path := "/data-dashboards" + "/" + dashboard
	out := &DataDashboardMutationResult{}
	callErr := s.do(ctx, http.MethodPatch, path, request, out)
	return out, callErr
}

func (s *DashboardService) Delete(ctx context.Context, dashboardID string) (*DataDashboardMutationResult, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	path := "/data-dashboards" + "/" + dashboard
	out := &DataDashboardMutationResult{}
	callErr := s.do(ctx, http.MethodDelete, path, nil, out)
	return out, callErr
}

func (s *DashboardService) CreateChart(ctx context.Context, dashboardID string, request *DataDashboardChartMutationRequest) (*DataDashboardChart, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "dashboard request is required"}
	}
	path := "/data-dashboards" + "/" + dashboard + "/charts"
	out := &DataDashboardChart{}
	callErr := s.do(ctx, http.MethodPost, path, request, out)
	return out, callErr
}

func (s *DashboardService) UpdateChart(ctx context.Context, dashboardID string, chartID string, request *DataDashboardChartMutationRequest) (*DataDashboardChart, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	chart, err := pathID(chartID)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "dashboard request is required"}
	}
	path := "/data-dashboards" + "/" + dashboard + "/charts/" + chart
	out := &DataDashboardChart{}
	callErr := s.do(ctx, http.MethodPatch, path, request, out)
	return out, callErr
}

func (s *DashboardService) DeleteChart(ctx context.Context, dashboardID string, chartID string) (*DataDashboardMutationResult, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	chart, err := pathID(chartID)
	if err != nil {
		return nil, err
	}
	path := "/data-dashboards" + "/" + dashboard + "/charts/" + chart
	out := &DataDashboardMutationResult{}
	callErr := s.do(ctx, http.MethodDelete, path, nil, out)
	return out, callErr
}

func (s *DashboardService) UpdateChartOrder(ctx context.Context, dashboardID string, request *DataDashboardChartOrderRequest) (*DataDashboardChartOrderRequest, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "dashboard request is required"}
	}
	path := "/data-dashboards" + "/" + dashboard + "/chart-order"
	out := &DataDashboardChartOrderRequest{}
	// The owner requires an array, including [] when the dashboard has no
	// charts. A protobuf repeated field's empty value must not disappear.
	body := map[string][]string{"chart_ids": append([]string{}, request.ChartIds...)}
	callErr := s.do(ctx, http.MethodPatch, path, body, out)
	return out, callErr
}

type dashboardPreviewOptions struct{ chartID *string }

// DashboardPreviewOption configures the existing SQL preview operation.
type DashboardPreviewOption func(*dashboardPreviewOptions)

// WithDashboardPreviewChartID selects the chart's persisted SQL/database scope.
func WithDashboardPreviewChartID(chartID string) DashboardPreviewOption {
	return func(options *dashboardPreviewOptions) { options.chartID = &chartID }
}

func (s *DashboardService) PreviewSQL(ctx context.Context, dashboardID string, sqlText string, options ...DashboardPreviewOption) (*SQLQueryResult, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	path := "/data-dashboards" + "/" + dashboard + "/preview"
	out := &SQLQueryResult{}
	var config dashboardPreviewOptions
	for _, option := range options {
		option(&config)
	}
	body := map[string]string{"sql_text": sqlText}
	if config.chartID != nil {
		body["chart_id"] = *config.chartID
	}
	callErr := s.do(ctx, http.MethodPost, path, body, out)
	return out, callErr
}

func (s *DashboardService) GenerateSQL(ctx context.Context, dashboardID string, question string, title string) (*DataDashboardGeneratedSQLResult, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	path := "/data-dashboards" + "/" + dashboard + "/generate-sql"
	out := &DataDashboardGeneratedSQLResult{}
	callErr := s.do(ctx, http.MethodPost, path, map[string]string{"question": question, "title": title}, out)
	return out, callErr
}

func (s *DashboardService) Refresh(ctx context.Context, dashboardID string) (*DataDashboardChartRuns, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	path := "/data-dashboards" + "/" + dashboard + "/refresh"
	out := &DataDashboardChartRuns{}
	callErr := s.do(ctx, http.MethodPost, path, nil, out)
	return out, callErr
}

func (s *DashboardService) RunChart(ctx context.Context, dashboardID string, chartID string) (*DataDashboardChartRun, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	chart, err := pathID(chartID)
	if err != nil {
		return nil, err
	}
	path := "/data-dashboards" + "/" + dashboard + "/charts/" + chart + "/run"
	out := &DataDashboardChartRun{}
	callErr := s.do(ctx, http.MethodPost, path, nil, out)
	return out, callErr
}

func (s *DashboardService) ChartResult(ctx context.Context, dashboardID string, chartID string, options ...QueryOption) (*DataDashboardChartExecution, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	chart, err := pathID(chartID)
	if err != nil {
		return nil, err
	}
	path := "/data-dashboards" + "/" + dashboard + "/charts/" + chart + "/result"
	path = withQuery(path, options)
	out := &DataDashboardChartExecution{}
	callErr := s.do(ctx, http.MethodGet, path, nil, out)
	return out, callErr
}

func (s *DashboardService) DownloadChartResult(ctx context.Context, dashboardID string, chartID string, options ...QueryOption) (*StreamResponse, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	chart, err := pathID(chartID)
	if err != nil {
		return nil, err
	}
	path := "/data-dashboards" + "/" + dashboard + "/charts/" + chart + "/result/download"
	path = withQuery(path, options)
	return s.doDownload(ctx, http.MethodGet, path, nil)
}

func (s *DashboardService) DrillThrough(ctx context.Context, dashboardID string, chartID string, request *DataDashboardDrillThroughRequest) (*DataDashboardDrillThroughResult, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	chart, err := pathID(chartID)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "dashboard request is required"}
	}
	path := "/data-dashboards" + "/" + dashboard + "/charts/" + chart + "/drill-through"
	out := &DataDashboardDrillThroughResult{}
	callErr := s.do(ctx, http.MethodPost, path, request, out)
	return out, callErr
}

func (s *DashboardService) DownloadDrillThrough(ctx context.Context, dashboardID string, chartID string, request *DataDashboardDrillThroughRequest) (*StreamResponse, error) {
	dashboard, err := pathID(dashboardID)
	if err != nil {
		return nil, err
	}
	chart, err := pathID(chartID)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "dashboard request is required"}
	}
	path := "/data-dashboards" + "/" + dashboard + "/charts/" + chart + "/drill-through/download"
	return s.doDownload(ctx, http.MethodPost, path, request)
}
