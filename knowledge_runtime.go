package sdk

import (
	"context"
	"net/http"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

type KnowledgeRuntimeResult = product.KnowledgeRuntimeResult

type KnowledgeFindFilesRequest = product.KnowledgeFindFilesRequest

// FindFiles operates only within the authenticated live task's knowledge scope.
func (s *KnowledgeService) FindFiles(ctx context.Context, request *KnowledgeFindFilesRequest) (*KnowledgeRuntimeResult, error) {
	result := &KnowledgeRuntimeResult{}
	err := s.do(ctx, http.MethodPost, "/semantic-models/runtime/find-files", request, result)
	return result, err
}

type KnowledgeSearchRequest = product.KnowledgeSearchRequest
type KnowledgeSourcePage = product.KnowledgeSourcePage

// Search operates only within the authenticated live task's knowledge scope.
func (s *KnowledgeService) Search(ctx context.Context, request *KnowledgeSearchRequest) (*KnowledgeRuntimeResult, error) {
	result := &KnowledgeRuntimeResult{}
	err := s.do(ctx, http.MethodPost, "/semantic-models/runtime/search", request, result)
	return result, err
}

type KnowledgeReadDocumentRequest = product.KnowledgeReadDocumentRequest

// ReadDocument operates only within the authenticated live task's knowledge scope.
func (s *KnowledgeService) ReadDocument(ctx context.Context, request *KnowledgeReadDocumentRequest) (*KnowledgeRuntimeResult, error) {
	result := &KnowledgeRuntimeResult{}
	err := s.do(ctx, http.MethodPost, "/semantic-models/runtime/read-document", request, result)
	return result, err
}

type KnowledgeSearchDocumentRequest = product.KnowledgeSearchDocumentRequest

// SearchDocument operates only within the authenticated live task's knowledge scope.
func (s *KnowledgeService) SearchDocument(ctx context.Context, request *KnowledgeSearchDocumentRequest) (*KnowledgeRuntimeResult, error) {
	result := &KnowledgeRuntimeResult{}
	err := s.do(ctx, http.MethodPost, "/semantic-models/runtime/search-document", request, result)
	return result, err
}

type KnowledgeSearchImagesRequest = product.KnowledgeSearchImagesRequest

// SearchImages operates only within the authenticated live task's knowledge scope.
func (s *KnowledgeService) SearchImages(ctx context.Context, request *KnowledgeSearchImagesRequest) (*KnowledgeRuntimeResult, error) {
	result := &KnowledgeRuntimeResult{}
	err := s.do(ctx, http.MethodPost, "/semantic-models/runtime/search-images", request, result)
	return result, err
}

type KnowledgeReadEvidenceRequest = product.KnowledgeReadEvidenceRequest
type KnowledgeLocalEvidenceRead = product.KnowledgeLocalEvidenceRead

// ReadEvidence operates only within the authenticated live task's knowledge scope.
func (s *KnowledgeService) ReadEvidence(ctx context.Context, request *KnowledgeReadEvidenceRequest) (*KnowledgeRuntimeResult, error) {
	result := &KnowledgeRuntimeResult{}
	err := s.do(ctx, http.MethodPost, "/semantic-models/runtime/read-evidence", request, result)
	return result, err
}

type KnowledgeDescribeTablesRequest = product.KnowledgeDescribeTablesRequest

// DescribeTables operates only within the authenticated live task's knowledge scope.
func (s *KnowledgeService) DescribeTables(ctx context.Context, request *KnowledgeDescribeTablesRequest) (*KnowledgeRuntimeResult, error) {
	result := &KnowledgeRuntimeResult{}
	err := s.do(ctx, http.MethodPost, "/semantic-models/runtime/describe-tables", request, result)
	return result, err
}

type KnowledgeQueryRequest = product.KnowledgeQueryRequest

// Query operates only within the authenticated live task's knowledge scope.
func (s *KnowledgeService) Query(ctx context.Context, request *KnowledgeQueryRequest) (*KnowledgeRuntimeResult, error) {
	result := &KnowledgeRuntimeResult{}
	err := s.do(ctx, http.MethodPost, "/semantic-models/runtime/query", request, result)
	return result, err
}

type KnowledgeCiteRequest = product.KnowledgeCiteRequest

// Cite operates only within the authenticated live task's knowledge scope.
func (s *KnowledgeService) Cite(ctx context.Context, request *KnowledgeCiteRequest) (*KnowledgeRuntimeResult, error) {
	result := &KnowledgeRuntimeResult{}
	err := s.do(ctx, http.MethodPost, "/semantic-models/runtime/cite", request, result)
	return result, err
}
