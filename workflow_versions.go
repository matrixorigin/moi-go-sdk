package sdk

import (
	"context"
	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"net/http"
	"strconv"
)

type WorkflowVersion = product.WorkflowVersion
type WorkflowVersionList = product.WorkflowVersionList
type WorkflowVersionCreateRequest = product.WorkflowVersionCreateRequest

func (s *WorkflowService) Versions(ctx context.Context, workflowID string) (*WorkflowVersionList, error) {
	id, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	out := &WorkflowVersionList{}
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/"+id+"/versions", nil, out)
	return out, err
}
func (s *WorkflowService) Version(ctx context.Context, workflowID string, versionID string) (*WorkflowVersion, error) {
	id, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	version, err := pathID(versionID)
	if err != nil {
		return nil, err
	}
	out := &WorkflowVersion{}
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/"+id+"/versions/"+version, nil, out)
	return out, err
}
func (s *WorkflowService) VersionByNumber(ctx context.Context, workflowID string, number int) (*WorkflowVersion, error) {
	id, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	out := &WorkflowVersion{}
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/"+id+"/versions/by-number/"+strconv.Itoa(number), nil, out)
	return out, err
}
func (s *WorkflowService) LatestPublishedVersion(ctx context.Context, workflowID string) (*WorkflowVersion, error) {
	id, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	out := &WorkflowVersion{}
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/"+id+"/versions/latest-published", nil, out)
	return out, err
}
func (s *WorkflowService) CreateVersion(ctx context.Context, workflowID string, request *WorkflowVersionCreateRequest) (*WorkflowVersion, error) {
	id, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	out := &WorkflowVersion{}
	err = s.do(ctx, http.MethodPost, "/workflow/v2/workflow-apps/"+id+"/versions", request, out)
	return out, err
}
func (s *WorkflowService) PublishVersion(ctx context.Context, workflowID string, versionID string) (*WorkflowVersion, error) {
	id, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	version, err := pathID(versionID)
	if err != nil {
		return nil, err
	}
	out := &WorkflowVersion{}
	err = s.do(ctx, http.MethodPost, "/workflow/v2/workflow-apps/"+id+"/versions/"+version+"/publish", nil, out)
	return out, err
}
func (s *WorkflowService) DeprecateVersion(ctx context.Context, workflowID string, versionID string) (*WorkflowVersion, error) {
	id, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	version, err := pathID(versionID)
	if err != nil {
		return nil, err
	}
	out := &WorkflowVersion{}
	err = s.do(ctx, http.MethodPost, "/workflow/v2/workflow-apps/"+id+"/versions/"+version+"/deprecate", nil, out)
	return out, err
}
func (s *WorkflowService) DeleteVersion(ctx context.Context, workflowID string, versionID string) error {
	id, err := pathID(workflowID)
	if err != nil {
		return err
	}
	version, err := pathID(versionID)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodDelete, "/workflow/v2/workflow-apps/"+id+"/versions/"+version, nil, nil)
}
