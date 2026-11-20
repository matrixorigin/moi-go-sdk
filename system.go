package sdk

import (
	"context"
	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"net/http"
)

type SystemService struct{ client *Client }
type SystemComponentVersion = product.SystemComponentVersion
type SystemVersionsResult = product.SystemVersionsResult
type WebappConfigResult = product.WebappConfigResult
type WebappLogo = product.WebappLogo
type SystemRouteCoverageResult = product.SystemRouteCoverageResult
type BillingMeterRolloutResult = product.BillingMeterRolloutResult

// EnableComputeBilling activates the existing audited Compute meter rollout.
func (s *SystemService) EnableComputeBilling(ctx context.Context) (*BillingMeterRolloutResult, error) {
	return s.enableBillingMeter(ctx, "/api/v1/system/billing/compute-rollout:enable")
}

// EnableAIServiceBilling activates the existing built-in AI service rollout.
func (s *SystemService) EnableAIServiceBilling(ctx context.Context) (*BillingMeterRolloutResult, error) {
	return s.enableBillingMeter(ctx, "/api/v1/system/billing/ai-service-rollout:enable")
}

func (s *SystemService) enableBillingMeter(ctx context.Context, path string) (*BillingMeterRolloutResult, error) {
	endpoint, err := s.client.nativeURL(path)
	if err != nil {
		return nil, err
	}
	result := &BillingMeterRolloutResult{}
	err = s.client.doURL(ctx, http.MethodPost, endpoint, "", nil, result)
	return result, err
}

func (c *Client) System() *SystemService { return &SystemService{client: c} }
func (s *SystemService) Health(ctx context.Context) (*HealthResult, error) {
	endpoint, err := s.client.nativeURL("/healthcheck")
	if err != nil {
		return nil, err
	}
	result := &HealthResult{}
	err = s.client.doURL(ctx, http.MethodGet, endpoint, "", nil, result)
	return result, err
}
func (s *SystemService) Metrics(ctx context.Context) (*StreamResponse, error) {
	endpoint, err := s.client.nativeURL("/metrics")
	if err != nil {
		return nil, err
	}
	return s.client.doStreamURL(ctx, http.MethodGet, endpoint, "", nil, acceptAny, false)
}
func (s *SystemService) Versions(ctx context.Context) (*SystemVersionsResult, error) {
	result := &SystemVersionsResult{}
	err := s.client.do(ctx, http.MethodGet, "/system/versions", "", nil, result)
	return result, err
}
func (s *SystemService) WebappConfig(ctx context.Context) (*WebappConfigResult, error) {
	result := &WebappConfigResult{}
	err := s.client.do(ctx, http.MethodGet, "/webapp/config", "", nil, result)
	return result, err
}

// RouteCoverage reads the diagnostic route-registration and hit snapshot.
func (s *SystemService) RouteCoverage(ctx context.Context) (*SystemRouteCoverageResult, error) {
	endpoint, err := s.client.nativeURL("/__moi_test/route-coverage")
	if err != nil {
		return nil, err
	}
	result := &SystemRouteCoverageResult{}
	err = s.client.doURL(ctx, http.MethodGet, endpoint, "", nil, result)
	return result, err
}

type ServiceAccountWorkspaceList = product.ServiceAccountWorkspaceList
type ServiceAccountRoleList = product.ServiceAccountRoleList

// ServiceAccountWorkspaces requires provisioner bearer, actor assertion and
// contract-version headers; ordinary product identities cannot use this API.
func (s *SystemService) ServiceAccountWorkspaces(ctx context.Context) (*ServiceAccountWorkspaceList, error) {
	endpoint, err := s.client.nativeURL("/api/v1/service-accounts/workspaces")
	if err != nil {
		return nil, err
	}
	out := &ServiceAccountWorkspaceList{}
	err = s.client.doURL(ctx, http.MethodGet, endpoint, "", nil, out)
	return out, err
}
func (s *SystemService) ServiceAccountAssignableRoles(ctx context.Context, workspaceID string) (*ServiceAccountRoleList, error) {
	workspace, err := pathID(workspaceID)
	if err != nil {
		return nil, err
	}
	endpoint, err := s.client.nativeURL("/api/v1/service-accounts/workspaces/" + workspace + "/assignable-roles")
	if err != nil {
		return nil, err
	}
	out := &ServiceAccountRoleList{}
	err = s.client.doURL(ctx, http.MethodGet, endpoint, "", nil, out)
	return out, err
}
