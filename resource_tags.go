package sdk

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

// ResourceTagService manages workspace tags independently of the tagged objects.
type ResourceTagService struct{ workspaceService }

func (c *Client) ResourceTags(workspaceID string) *ResourceTagService {
	return &ResourceTagService{workspaceService{client: c, workspaceID: workspaceID}}
}

type ResourceTag = product.ResourceTag
type ResourceTagValue = product.ResourceTagValue
type ResourceTagCreateRequest = product.ResourceTagCreateRequest
type ResourceTagUpdateRequest = product.ResourceTagUpdateRequest
type ResourceTagListRequest = product.ResourceTagListRequest
type ResourceTagValueRequest = product.ResourceTagValueRequest
type ResourceTagQueryRequest = product.ResourceTagQueryRequest
type ResourceTagFilter = product.ResourceTagFilter
type ResourceTagRelationsRequest = product.ResourceTagRelationsRequest
type ResourceTagSet = product.ResourceTagSet
type ResourceTagRemove = product.ResourceTagRemove
type ResourceTagAssignmentsRequest = product.ResourceTagAssignmentsRequest
type ResourceTagList = product.ResourceTagList
type ResourceTagValueList = product.ResourceTagValueList
type ResourceTagGroupList = product.ResourceTagGroupList
type ResourceTagResourceList = product.ResourceTagResourceList
type ResourceTagAssignmentsList = product.ResourceTagAssignmentsList

func (s *ResourceTagService) Create(ctx context.Context, request *ResourceTagCreateRequest) (*ResourceTag, error) {
	result := &ResourceTag{}
	err := s.do(ctx, http.MethodPost, "/tags", request, result)
	return result, err
}
func (s *ResourceTagService) List(ctx context.Context, request *ResourceTagListRequest) (*ResourceTagList, error) {
	query := resourceTagListQuery(request)
	query.Set("type", request.GetType())
	query.Set("name_search", request.GetNameSearch())
	query.Set("resource_id", request.GetResourceId())
	result := &ResourceTagList{}
	err := s.do(ctx, http.MethodGet, "/tags?"+query.Encode(), nil, result)
	return result, err
}
func (s *ResourceTagService) Update(ctx context.Context, tagID string, request *ResourceTagUpdateRequest) (*ResourceTag, error) {
	id, err := pathID(tagID)
	if err != nil {
		return nil, err
	}
	result := &ResourceTag{}
	err = s.do(ctx, http.MethodPatch, "/tags/"+id, request, result)
	return result, err
}
func (s *ResourceTagService) Delete(ctx context.Context, tagID string) error {
	id, err := pathID(tagID)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodDelete, "/tags/"+id, nil, nil)
}
func (s *ResourceTagService) CreateValue(ctx context.Context, tagID string, request *ResourceTagValueRequest) (*ResourceTagValue, error) {
	id, err := pathID(tagID)
	if err != nil {
		return nil, err
	}
	result := &ResourceTagValue{}
	err = s.do(ctx, http.MethodPost, "/tags/"+id+"/values", request, result)
	return result, err
}
func (s *ResourceTagService) ListValues(ctx context.Context, tagID string, request *ResourceTagListRequest) (*ResourceTagValueList, error) {
	id, err := pathID(tagID)
	if err != nil {
		return nil, err
	}
	query := resourceTagListQuery(request)
	query.Set("value_search", request.GetValueSearch())
	result := &ResourceTagValueList{}
	err = s.do(ctx, http.MethodGet, "/tags/"+id+"/values?"+query.Encode(), nil, result)
	return result, err
}
func (s *ResourceTagService) UpdateValue(ctx context.Context, tagID, valueID string, request *ResourceTagValueRequest) (*ResourceTagValue, error) {
	tag, err := pathID(tagID)
	if err != nil {
		return nil, err
	}
	value, err := pathID(valueID)
	if err != nil {
		return nil, err
	}
	result := &ResourceTagValue{}
	err = s.do(ctx, http.MethodPatch, "/tags/"+tag+"/values/"+value, request, result)
	return result, err
}
func (s *ResourceTagService) DeleteValue(ctx context.Context, tagID, valueID string) error {
	tag, err := pathID(tagID)
	if err != nil {
		return err
	}
	value, err := pathID(valueID)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodDelete, "/tags/"+tag+"/values/"+value, nil, nil)
}
func (s *ResourceTagService) UpdateRelations(ctx context.Context, request *ResourceTagRelationsRequest) error {
	return s.do(ctx, http.MethodPost, "/tags/relations/update", request, nil)
}
func (s *ResourceTagService) Groups(ctx context.Context, request *ResourceTagQueryRequest) (*ResourceTagGroupList, error) {
	result := &ResourceTagGroupList{}
	err := s.do(ctx, http.MethodPost, "/tags/groups/query", request, result)
	return result, err
}
func (s *ResourceTagService) Resources(ctx context.Context, request *ResourceTagQueryRequest) (*ResourceTagResourceList, error) {
	result := &ResourceTagResourceList{}
	err := s.do(ctx, http.MethodPost, "/tags/resources/query", request, result)
	return result, err
}
func (s *ResourceTagService) Assignments(ctx context.Context, request *ResourceTagAssignmentsRequest) (*ResourceTagAssignmentsList, error) {
	result := &ResourceTagAssignmentsList{}
	err := s.do(ctx, http.MethodPost, "/tags/resources/tags/query", request, result)
	return result, err
}

func resourceTagListQuery(request *ResourceTagListRequest) url.Values {
	return url.Values{"offset": {strconv.Itoa(int(request.GetOffset()))}, "limit": {strconv.Itoa(int(request.GetLimit()))}}
}
