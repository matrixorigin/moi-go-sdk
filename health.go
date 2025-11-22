package sdk

import (
	"context"
	"encoding/json"
	"net/http"
)

// HealthStatus mirrors the response from /healthz.
type HealthStatus struct {
	Status string `json:"status"`
}

// HealthCheck queries the /healthz endpoint.
func (c *RawClient) HealthCheck(ctx context.Context, opts ...CallOption) (*HealthStatus, error) {
	callOpts := newCallOptions(opts...)
	resp, err := c.doRaw(ctx, http.MethodGet, "/healthz", nil, callOpts, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var status HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}
