package sdk

import (
	"context"
	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"net/http"
)

type WorkspaceCreationJob = product.WorkspaceCreationJob
type WorkspaceCreationsResult = product.WorkspaceCreationsResult

func (s *WorkspaceService) Creations(ctx context.Context) (*WorkspaceCreationsResult, error) {
	result := &WorkspaceCreationsResult{}
	err := s.client.do(ctx, http.MethodGet, "/workspaces/creations", "", nil, result)
	return result, err
}
func (s *WorkspaceService) StartCreation(ctx context.Context, name string) (*WorkspaceCreationJob, error) {
	result := &WorkspaceCreationJob{}
	err := s.client.do(ctx, http.MethodPost, "/workspaces/creations", "", map[string]any{"name": name}, result)
	return result, err
}
func (s *WorkspaceService) Update(ctx context.Context, workspaceID, name string) error {
	id, err := pathID(workspaceID)
	if err != nil {
		return err
	}
	return s.client.do(ctx, http.MethodPatch, "/workspaces/"+id, "", map[string]any{"name": name}, nil)
}
