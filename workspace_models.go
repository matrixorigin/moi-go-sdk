package sdk

import (
	"context"
	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"net/http"
	"strconv"
)

type WorkspaceLLMBackend = product.WorkspaceLLMBackend
type WorkspaceLLMBackendCreateRequest = product.WorkspaceLLMBackendCreateRequest
type WorkspaceLLMBackendUpdateRequest = product.WorkspaceLLMBackendUpdateRequest
type WorkspaceLLMBackendList = product.WorkspaceLLMBackendList
type WorkspaceLLMEndpoint = product.WorkspaceLLMEndpoint
type WorkspaceLLMRouterConfig = product.WorkspaceLLMRouterConfig
type WorkspaceLLMRouterConfigResult = product.WorkspaceLLMRouterConfigResult
type WorkspaceLLMRouterUpdateRequest = product.WorkspaceLLMRouterUpdateRequest
type WorkspaceChatCompletionProxy = product.WorkspaceChatCompletionProxy
type WorkspaceChatCompletionRequest = product.WorkspaceChatCompletionRequest

func (s *ModelService) CreateWorkspaceLLMBackend(ctx context.Context, request *WorkspaceLLMBackendCreateRequest) (*WorkspaceLLMBackend, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/llm"
	result := &WorkspaceLLMBackend{}
	err = s.do(ctx, http.MethodPost, prefix+"/backends", request, result)
	return result, err
}

func (s *ModelService) ListWorkspaceLLMBackends(ctx context.Context) (*WorkspaceLLMBackendList, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/llm"
	result := &WorkspaceLLMBackendList{}
	err = s.do(ctx, http.MethodGet, prefix+"/backends", nil, result)
	return result, err
}

func (s *ModelService) GetWorkspaceLLMBackend(ctx context.Context, backendID int64) (*WorkspaceLLMBackend, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/llm"
	result := &WorkspaceLLMBackend{}
	err = s.do(ctx, http.MethodGet, prefix+"/backends/"+strconv.FormatInt(backendID, 10), nil, result)
	return result, err
}

func (s *ModelService) UpdateWorkspaceLLMBackend(ctx context.Context, backendID int64, request *WorkspaceLLMBackendUpdateRequest) (*WorkspaceLLMBackend, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/llm"
	result := &WorkspaceLLMBackend{}
	err = s.do(ctx, http.MethodPut, prefix+"/backends/"+strconv.FormatInt(backendID, 10), request, result)
	return result, err
}

func (s *ModelService) DeleteWorkspaceLLMBackend(ctx context.Context, backendID int64) error {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return err
	}
	prefix := "/workspaces/" + workspace + "/llm"
	return s.do(ctx, http.MethodDelete, prefix+"/backends/"+strconv.FormatInt(backendID, 10), nil, nil)
}

func (s *ModelService) CreateWorkspaceLLMEndpoint(ctx context.Context, backendID int64, address string) (*WorkspaceLLMEndpoint, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/llm"
	result := &WorkspaceLLMEndpoint{}
	err = s.do(ctx, http.MethodPost, prefix+"/backends/"+strconv.FormatInt(backendID, 10)+"/endpoints", map[string]string{"address": address}, result)
	return result, err
}

func (s *ModelService) WorkspaceLLMRouter(ctx context.Context) (*WorkspaceLLMRouterConfigResult, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/llm"
	result := &WorkspaceLLMRouterConfigResult{}
	err = s.do(ctx, http.MethodGet, prefix+"/router-config", nil, result)
	return result, err
}

func (s *ModelService) UpdateWorkspaceLLMRouter(ctx context.Context, request *WorkspaceLLMRouterUpdateRequest) (*WorkspaceLLMRouterConfigResult, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/llm"
	result := &WorkspaceLLMRouterConfigResult{}
	err = s.do(ctx, http.MethodPut, prefix+"/router-config", request, result)
	return result, err
}

func (s *ModelService) WorkspaceChatCompletion(ctx context.Context, request *WorkspaceChatCompletionRequest, opts ...QueryOption) (*ProductData, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/llm"
	result := &ProductData{}
	err = s.do(ctx, http.MethodPost, withQuery(prefix+"/chat/completions", opts), request, result)
	return result, err
}

// WorkspaceChatCompletionStream leaves the response body with the caller so
// response-start timeouts cannot turn into a buffered, whole-stream deadline.
func (s *ModelService) WorkspaceChatCompletionStream(ctx context.Context, request *WorkspaceChatCompletionRequest, opts ...QueryOption) (*StreamResponse, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	body := struct {
		*WorkspaceChatCompletionRequest
		Stream bool `json:"stream"`
	}{request, true}
	return s.doStream(ctx, http.MethodPost, withQuery("/workspaces/"+workspace+"/llm/chat/completions", opts), body)
}

type WorkspaceEmbeddingBackend = product.WorkspaceLLMBackend
type WorkspaceEmbeddingBackendCreateRequest = product.WorkspaceLLMBackendCreateRequest
type WorkspaceEmbeddingBackendUpdateRequest = product.WorkspaceLLMBackendUpdateRequest
type WorkspaceEmbeddingBackendList = product.WorkspaceLLMBackendList
type WorkspaceEmbeddingEndpoint = product.WorkspaceLLMEndpoint
type WorkspaceEmbeddingRouterConfigResult = product.WorkspaceLLMRouterConfigResult
type WorkspaceEmbeddingRouterUpdateRequest = product.WorkspaceLLMRouterUpdateRequest

func (s *ModelService) CreateWorkspaceEmbeddingBackend(ctx context.Context, request *WorkspaceEmbeddingBackendCreateRequest) (*WorkspaceEmbeddingBackend, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/embeddings"
	result := &WorkspaceEmbeddingBackend{}
	err = s.do(ctx, http.MethodPost, prefix+"/backends", request, result)
	return result, err
}

func (s *ModelService) ListWorkspaceEmbeddingBackends(ctx context.Context) (*WorkspaceEmbeddingBackendList, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/embeddings"
	result := &WorkspaceEmbeddingBackendList{}
	err = s.do(ctx, http.MethodGet, prefix+"/backends", nil, result)
	return result, err
}

func (s *ModelService) GetWorkspaceEmbeddingBackend(ctx context.Context, backendID int64) (*WorkspaceEmbeddingBackend, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/embeddings"
	result := &WorkspaceEmbeddingBackend{}
	err = s.do(ctx, http.MethodGet, prefix+"/backends/"+strconv.FormatInt(backendID, 10), nil, result)
	return result, err
}

func (s *ModelService) UpdateWorkspaceEmbeddingBackend(ctx context.Context, backendID int64, request *WorkspaceEmbeddingBackendUpdateRequest) (*WorkspaceEmbeddingBackend, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/embeddings"
	result := &WorkspaceEmbeddingBackend{}
	err = s.do(ctx, http.MethodPut, prefix+"/backends/"+strconv.FormatInt(backendID, 10), request, result)
	return result, err
}

func (s *ModelService) DeleteWorkspaceEmbeddingBackend(ctx context.Context, backendID int64) error {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return err
	}
	prefix := "/workspaces/" + workspace + "/embeddings"
	return s.do(ctx, http.MethodDelete, prefix+"/backends/"+strconv.FormatInt(backendID, 10), nil, nil)
}

func (s *ModelService) CreateWorkspaceEmbeddingEndpoint(ctx context.Context, backendID int64, address string) (*WorkspaceEmbeddingEndpoint, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/embeddings"
	result := &WorkspaceEmbeddingEndpoint{}
	err = s.do(ctx, http.MethodPost, prefix+"/backends/"+strconv.FormatInt(backendID, 10)+"/endpoints", map[string]string{"address": address}, result)
	return result, err
}

func (s *ModelService) WorkspaceEmbeddingRouter(ctx context.Context) (*WorkspaceEmbeddingRouterConfigResult, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/embeddings"
	result := &WorkspaceEmbeddingRouterConfigResult{}
	err = s.do(ctx, http.MethodGet, prefix+"/router-config", nil, result)
	return result, err
}

func (s *ModelService) UpdateWorkspaceEmbeddingRouter(ctx context.Context, request *WorkspaceEmbeddingRouterUpdateRequest) (*WorkspaceEmbeddingRouterConfigResult, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	prefix := "/workspaces/" + workspace + "/embeddings"
	result := &WorkspaceEmbeddingRouterConfigResult{}
	err = s.do(ctx, http.MethodPut, prefix+"/router-config", request, result)
	return result, err
}

func (s *ModelService) ListWorkspaceLLMEndpoints(ctx context.Context, backendID int64) ([]*WorkspaceLLMEndpoint, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	var out []*WorkspaceLLMEndpoint
	err = s.do(ctx, http.MethodGet, "/workspaces/"+workspace+"/llm/backends/"+strconv.FormatInt(backendID, 10)+"/endpoints", nil, &out)
	return out, err
}
func (s *ModelService) SetWorkspaceLLMEndpointStatus(ctx context.Context, backendID, endpointID int64, status int32) error {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return err
	}
	err = s.do(ctx, http.MethodPut, "/workspaces/"+workspace+"/llm/backends/"+strconv.FormatInt(backendID, 10)+"/endpoints/"+strconv.FormatInt(endpointID, 10)+"/status", map[string]int32{"status": status}, nil)
	return err
}

func (s *ModelService) ListWorkspaceEmbeddingEndpoints(ctx context.Context, backendID int64) ([]*WorkspaceLLMEndpoint, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	var out []*WorkspaceLLMEndpoint
	err = s.do(ctx, http.MethodGet, "/workspaces/"+workspace+"/embeddings/backends/"+strconv.FormatInt(backendID, 10)+"/endpoints", nil, &out)
	return out, err
}
func (s *ModelService) SetWorkspaceEmbeddingEndpointStatus(ctx context.Context, backendID, endpointID int64, status int32) (*WorkspaceEndpointMutationResult, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	out := &WorkspaceEndpointMutationResult{}
	err = s.do(ctx, http.MethodPut, "/workspaces/"+workspace+"/embeddings/backends/"+strconv.FormatInt(backendID, 10)+"/endpoints/"+strconv.FormatInt(endpointID, 10)+"/status", map[string]int32{"status": status}, out)
	return out, err
}

func (s *ModelService) ListWorkspaceLLMModels(ctx context.Context) (*ProviderModelListResult, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	out := &ProviderModelListResult{}
	err = s.do(ctx, http.MethodGet, "/workspaces/"+workspace+"/llm/models", nil, out)
	return out, err
}

type WorkspaceEndpointMutationResult = product.WorkspaceEndpointMutationResult
