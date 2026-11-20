package sdk

import (
	"context"
	"net/http"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

type DBConnectionResult = product.DBConnectionResult

// AccountService is a route-aligned escape hatch. Prefer typed WorkspaceHandle DB connection helpers for user-facing workflows.
type AccountService struct {
	client *Client
}

func (s *AccountService) DBConnection(ctx context.Context, workspaceID string) (*DBConnectionResult, error) {
	if workspaceID == "" {
		return nil, ErrWorkspaceRequired
	}
	out := &product.DBConnectionResult{}
	err := s.client.do(ctx, http.MethodGet, "/user/me/db-connection", workspaceID, nil, out)
	return out, err
}

// CurrentCredit reads the authenticated UC browser account's available credit.
func (s *AccountService) CurrentCredit(ctx context.Context) (*product.CurrentCreditResult, error) {
	out := &product.CurrentCreditResult{}
	err := s.client.do(ctx, http.MethodGet, "/billing/credits/current", "", nil, out)
	return out, err
}
