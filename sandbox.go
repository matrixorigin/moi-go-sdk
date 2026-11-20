package sdk

import (
	"context"
	"net/http"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

type AgentSandbox = product.AgentSandbox
type SandboxCreateRequest = product.SandboxCreateRequest
type SandboxNetworkRequest = product.SandboxNetworkRequest
type SandboxQuotaRequest = product.SandboxQuotaRequest
type SandboxEgressEndpoint = product.SandboxEgressEndpoint
type SandboxQuotaStatus = product.SandboxQuotaStatus
type SandboxQuota = product.SandboxQuota
type SandboxAdminListResult = product.SandboxAdminListResult
type SandboxTerminalToken = product.SandboxTerminalToken
type SandboxSSHSession = product.SandboxSSHSession
type SandboxAdminWorkspace = product.SandboxAdminWorkspace

// SandboxService manages workspace sandboxes through the product API.
type SandboxService struct{ workspaceService }

func (s *SandboxService) AdminGet(ctx context.Context, id string) (*AgentSandbox, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out AgentSandbox
	err = s.do(ctx, http.MethodGet, "/admin/agent-sandboxes/"+path, nil, &out)
	return &out, err
}

// AdminUpdateQuota updates the explicit target workspace under platform admission.
func (s *SandboxService) AdminUpdateQuota(ctx context.Context, workspaceID string, request *SandboxQuotaRequest) (*SandboxQuota, error) {
	path, err := pathID(workspaceID)
	if err != nil {
		return nil, err
	}
	var out SandboxQuota
	err = s.do(ctx, http.MethodPatch, "/admin/workspaces/"+path+"/sandbox-quota", request, &out)
	return &out, err
}

// CreateSSHSession obtains gateway connection information; it does not dial SSH.
func (s *SandboxService) CreateSSHSession(ctx context.Context, id string) (*SandboxSSHSession, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out SandboxSSHSession
	err = s.do(ctx, http.MethodPost, "/agent-sandboxes/"+path+"/ssh-session", nil, &out)
	return &out, err
}

// AdminList lists sandboxes across workspaces under spec-administrator admission.
func (s *SandboxService) AdminList(ctx context.Context, opts ...QueryOption) (*SandboxAdminListResult, error) {
	var out SandboxAdminListResult
	err := s.do(ctx, http.MethodGet, withQuery("/admin/agent-sandboxes", opts), nil, &out)
	return &out, err
}

func (s *SandboxService) AdminDelete(ctx context.Context, id string) error {
	path, err := pathID(id)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodDelete, "/admin/agent-sandboxes/"+path, nil, nil)
}

func (s *SandboxService) AdminUpdateNetwork(ctx context.Context, id string, request *SandboxNetworkRequest) (*AgentSandbox, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out AgentSandbox
	err = s.do(ctx, http.MethodPatch, "/admin/agent-sandboxes/"+path+"/network", request, &out)
	return &out, err
}

// TerminalToken obtains the short-lived token used by the terminal WebSocket.
func (s *SandboxService) TerminalToken(ctx context.Context, id string) (*SandboxTerminalToken, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out SandboxTerminalToken
	err = s.do(ctx, http.MethodGet, "/agent-sandboxes/"+path+"/terminal", nil, &out)
	return &out, err
}

func (s *SandboxService) Create(ctx context.Context, request *SandboxCreateRequest) (*AgentSandbox, error) {
	var out AgentSandbox
	err := s.do(ctx, http.MethodPost, "/agent-sandboxes", request, &out)
	return &out, err
}

func (s *SandboxService) List(ctx context.Context) ([]*AgentSandbox, error) {
	var out sandboxList
	err := s.do(ctx, http.MethodGet, "/agent-sandboxes", nil, &out)
	return out, err
}

func (s *SandboxService) Get(ctx context.Context, id string) (*AgentSandbox, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out AgentSandbox
	err = s.do(ctx, http.MethodGet, "/agent-sandboxes/"+path, nil, &out)
	return &out, err
}

func (s *SandboxService) Delete(ctx context.Context, id string) error {
	path, err := pathID(id)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodDelete, "/agent-sandboxes/"+path, nil, nil)
}

func (s *SandboxService) UpdateNetwork(ctx context.Context, id string, request *SandboxNetworkRequest) (*AgentSandbox, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out AgentSandbox
	err = s.do(ctx, http.MethodPatch, "/agent-sandboxes/"+path+"/network", request, &out)
	return &out, err
}

// GetQuota preserves the published response by default. Request network
// capability metadata with WithQueryParam("include_network_capabilities", "true").
func (s *SandboxService) GetQuota(ctx context.Context, opts ...QueryOption) (*SandboxQuotaStatus, error) {
	var out SandboxQuotaStatus
	err := s.do(ctx, http.MethodGet, withQuery("/agent-sandboxes/quota", opts), nil, &out)
	return &out, err
}

func (s *SandboxService) UpdateQuota(ctx context.Context, request *SandboxQuotaRequest) (*SandboxQuota, error) {
	var out SandboxQuota
	err := s.do(ctx, http.MethodPatch, "/agent-sandboxes/quota", request, &out)
	return &out, err
}

type sandboxList []*AgentSandbox

func (items *sandboxList) UnmarshalJSON(data []byte) error {
	decoded, err := decodeProductMessageList(data, func() *AgentSandbox { return &AgentSandbox{} })
	if err != nil {
		return err
	}
	*items = decoded
	return nil
}
