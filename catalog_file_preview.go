package sdk

import (
	"context"
	"net/http"
)

// PreviewWorkspaceFile streams the inline preview of a workspace file.
func (s *CatalogService) PreviewWorkspaceFile(ctx context.Context, fileID string) (*StreamResponse, error) {
	workspaceID, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	id, err := pathID(fileID)
	if err != nil {
		return nil, err
	}
	return s.doDownload(ctx, http.MethodGet, "/workspaces/"+workspaceID+"/files/"+id+"/preview", nil)
}
