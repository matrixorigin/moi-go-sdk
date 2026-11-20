package sdk

import (
	"context"
	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"net/http"
)

type SemanticModelImportEntry = product.SemanticModelImportEntry
type SemanticModelImportRequest = product.SemanticModelImportRequest

// ImportModel imports one complete semantic model, including its source declarations and entries.
func (s *KnowledgeService) ImportModel(ctx context.Context, request *SemanticModelImportRequest) (*SemanticModelInfo, error) {
	result := &SemanticModelInfo{}
	err := s.do(ctx, http.MethodPost, "/semantic-models/import", request, result)
	return result, err
}

// ImportModels imports the published array payload and returns all imported models.
func (s *KnowledgeService) ImportModels(ctx context.Context, requests []*SemanticModelImportRequest) (*SemanticModelListResult, error) {
	result := &SemanticModelListResult{}
	err := s.do(ctx, http.MethodPost, "/semantic-models/import", requests, result)
	return result, err
}
