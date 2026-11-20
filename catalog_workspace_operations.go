package sdk

import (
	"context"
	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"net/http"
	"net/url"
)

type CatalogGarbageCollectionRequest = product.CatalogGarbageCollectionRequest
type CatalogGarbageCollectionResult = product.CatalogGarbageCollectionResult
type WorkspaceTableColumnsRequest = product.WorkspaceTableColumnsRequest
type WorkspaceTableOverviewItem = product.WorkspaceTableOverviewItem
type WorkspaceTableOverviewResult = product.WorkspaceTableOverviewResult
type RefListResult = product.RefListResult

func (s *CatalogService) CatalogReferences(ctx context.Context, id int64) (*RefListResult, error) {
	result := &RefListResult{}
	err := s.do(ctx, http.MethodPost, "/catalog/ref_list", map[string]int64{"id": id}, result)
	return result, err
}

func (s *CatalogService) DatabaseReferences(ctx context.Context, id int64) (*RefListResult, error) {
	result := &RefListResult{}
	err := s.do(ctx, http.MethodPost, "/catalog/database/ref_list", map[string]int64{"id": id}, result)
	return result, err
}

func (s *CatalogService) VolumeReferences(ctx context.Context, id int64) (*RefListResult, error) {
	result := &RefListResult{}
	err := s.do(ctx, http.MethodPost, "/catalog/volume/ref_list", map[string]int64{"id": id}, result)
	return result, err
}

func (s *CatalogService) FolderReferences(ctx context.Context, id int64) (*RefListResult, error) {
	result := &RefListResult{}
	err := s.do(ctx, http.MethodPost, "/catalog/folder/ref_list", map[string]int64{"id": id}, result)
	return result, err
}

func (s *CatalogService) TableReferences(ctx context.Context, id int64) (*RefListResult, error) {
	result := &RefListResult{}
	err := s.do(ctx, http.MethodPost, "/catalog/table/ref_list", map[string]int64{"id": id}, result)
	return result, err
}

func (s *CatalogService) WorkspaceFileModelURL(ctx context.Context, fileID string) (*URLResult, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	file, err := pathID(fileID)
	if err != nil {
		return nil, err
	}
	result := &URLResult{}
	err = s.do(ctx, http.MethodPost, "/workspaces/"+workspace+"/files/"+file+"/model-url", nil, result)
	return result, err
}
func (s *CatalogService) GarbageCollection(ctx context.Context, request *CatalogGarbageCollectionRequest) (*CatalogGarbageCollectionResult, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	result := &CatalogGarbageCollectionResult{}
	err = s.do(ctx, http.MethodPost, "/workspaces/"+workspace+"/garbage-collection", request, result)
	return result, err
}
func (s *CatalogService) WorkspaceTableColumns(ctx context.Context, request *WorkspaceTableColumnsRequest) (*WorkspaceTableOverviewItem, error) {
	result := &WorkspaceTableOverviewItem{}
	err := s.do(ctx, http.MethodPost, "/table/columns", request, result)
	return result, err
}
func (s *CatalogService) WorkspaceDatabaseTables(ctx context.Context, databaseID int64) (*WorkspaceTableOverviewResult, error) {
	result := &WorkspaceTableOverviewResult{}
	err := s.do(ctx, http.MethodPost, "/table/database/tables", map[string]int64{"database_id": databaseID}, result)
	return result, err
}
func (s *CatalogService) WorkspaceTableOverview(ctx context.Context) (*WorkspaceTableOverviewResult, error) {
	result := &WorkspaceTableOverviewResult{}
	err := s.do(ctx, http.MethodPost, "/table/overview", struct{}{}, result)
	return result, err
}

// CatalogFileContent reads content with a model-URL capability through the product route.
func (s *CatalogService) CatalogFileContent(ctx context.Context, fileID, capability string) (*StreamResponse, error) {
	file, err := pathID(fileID)
	if err != nil {
		return nil, err
	}
	return s.doDownload(ctx, http.MethodGet, "/catalog-files/"+file+"/content?"+url.Values{"capability": {capability}}.Encode(), nil)
}
