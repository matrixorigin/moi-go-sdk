package sdk

import (
	"context"
	"net/http"
)

// PreviewSourceFile streams a source file associated with a knowledge base.
func (s *KnowledgeService) PreviewSourceFile(ctx context.Context, modelID, fileID string) (*StreamResponse, error) {
	model, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	file, err := pathID(fileID)
	if err != nil {
		return nil, err
	}
	return s.doDownload(ctx, http.MethodGet, "/semantic-models/"+model+"/sources/file/"+file+"/preview", nil)
}

// PreviewArtifact streams a parsed artifact associated with a knowledge base.
func (s *KnowledgeService) PreviewArtifact(ctx context.Context, modelID, fileID string) (*StreamResponse, error) {
	model, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	file, err := pathID(fileID)
	if err != nil {
		return nil, err
	}
	return s.doDownload(ctx, http.MethodGet, "/semantic-models/"+model+"/artifacts/"+file+"/preview", nil)
}
