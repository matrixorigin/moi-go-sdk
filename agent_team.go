package sdk

import (
	"context"
	"net/http"
	"strconv"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type AgentTeamRecord = product.AgentTeamRecord
type AgentCatalogCard = product.AgentCatalogCard
type AgentCatalogCardListResult = product.AgentCatalogCardListResult
type AgentTeamMemberRef = product.AgentTeamMemberRef
type AgentTeamMember = product.AgentTeamMember
type AgentTargetRef = product.AgentTargetRef
type IngressConnection = product.IngressConnection
type WeComQRBindingResult = product.WeComQRBindingResult

type AgentTeamSession = product.AgentTeamSession
type AgentTeamListResult = product.AgentTeamListResult
type AgentTeamSessionListResult = product.AgentTeamSessionListResult
type AgentTeamMemberList = product.AgentTeamMemberList
type IngressConnectionList = product.IngressConnectionList
type IngressCapability = product.IngressCapability
type IngressCapabilityList = product.IngressCapabilityList
type AgentTeamSubmitResult = product.AgentTeamSubmitResult
type AgentTeamMessageListResult = product.AgentTeamMessageListResult
type AgentTeamCreateRequest struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Members     []*AgentTeamMemberRef `json:"members"`
}

// ListAgentCatalogCards returns the authorized Agent and Team cards shown in
// the MOI catalog.
func (s *AgentService) ListAgentCatalogCards(ctx context.Context, opts ...QueryOption) (*AgentCatalogCardListResult, error) {
	ids, err := s.teamPathIDs()
	if err != nil {
		return nil, err
	}
	out := &AgentCatalogCardListResult{}
	if err := s.do(ctx, http.MethodGet, withQuery("/workspaces/"+ids[0]+"/agent-cards", opts), nil, &teamResponse{message: out}); err != nil {
		return nil, err
	}
	return out, nil
}

type AgentTeamUpdateRequest struct {
	Name            *string `json:"name,omitempty"`
	Description     *string `json:"description,omitempty"`
	ExpectedVersion int64   `json:"expected_version"`
}
type IngressCallbackConfig struct {
	ChannelInstanceID string `json:"channel_instance_id"`
}

type IngressMailPollingConfig struct {
	ChannelInstanceID string `json:"channel_instance_id"`
}

type IngressManagedStreamConfig struct {
	BotID     string `json:"bot_id,omitempty"`
	Secret    string `json:"secret,omitempty"`
	AppID     string `json:"app_id,omitempty"`
	AppSecret string `json:"app_secret,omitempty"`
	AppToken  string `json:"app_token,omitempty"`
	BotToken  string `json:"bot_token,omitempty"`
}

type IngressConnectionCreateRequest struct {
	Target        *AgentTargetRef             `json:"target"`
	Provider      string                      `json:"provider"`
	ChannelType   string                      `json:"channel_type"`
	ReceiveMode   string                      `json:"receive_mode"`
	AgentModel    string                      `json:"agent_model,omitempty"`
	Callback      *IngressCallbackConfig      `json:"callback,omitempty"`
	MailPolling   *IngressMailPollingConfig   `json:"mail_polling,omitempty"`
	ManagedStream *IngressManagedStreamConfig `json:"managed_stream,omitempty"`
}

type IngressConnectionUpdateRequest struct {
	Enabled                    *bool                       `json:"enabled,omitempty"`
	AgentModel                 *string                     `json:"agent_model,omitempty"`
	ManagedStream              *IngressManagedStreamConfig `json:"managed_stream,omitempty"`
	ExpectedAssociationVersion int64                       `json:"expected_association_version"`
	ExpectedConfigVersion      int64                       `json:"expected_config_version"`
}
type IngressConnectionVerifyRequest struct {
	ExpectedAssociationVersion int64  `json:"expected_association_version"`
	ExpectedConfigVersion      int64  `json:"expected_config_version"`
	CallbackURL                string `json:"callback_url,omitempty"`
}
type AgentTeamSessionCreateRequest struct {
	Name string `json:"name,omitempty"`
}
type AgentTeamSessionRenameRequest struct {
	Name            string `json:"name"`
	ExpectedVersion int64  `json:"expected_version"`
}
type AgentTeamMessageRequest struct {
	MessageID       string              `json:"message_id"`
	Parts           []map[string]any    `json:"parts"`
	RequestedMember *AgentTeamMemberRef `json:"requested_member,omitempty"`
	IdempotencyKey  string              `json:"idempotency_key"`
	Metadata        map[string]any      `json:"metadata,omitempty"`
}

// teamPathIDs validates and escapes identifiers once before composing explicit routes.
func (s *AgentService) teamPathIDs(ids ...string) ([]string, error) {
	if s.workspaceID == "" {
		return nil, ErrWorkspaceRequired
	}
	ids = append([]string{s.workspaceID}, ids...)
	for i, id := range ids {
		encoded, err := pathID(id)
		if err != nil {
			return nil, err
		}
		ids[i] = encoded
	}
	return ids, nil
}

// teamResponse preserves strict generated decoding for both object and array endpoints.
type teamResponse struct {
	message proto.Message
	array   bool
}

func (r *teamResponse) UnmarshalJSON(raw []byte) error {
	if r.array {
		raw = append(append([]byte("{\"items\":"), raw...), '}')
	}
	return (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(raw, r.message)
}

// CreateTeam calls the corresponding Catalog-owned Team product operation.
func (s *AgentService) CreateTeam(ctx context.Context, request AgentTeamCreateRequest) (*AgentTeamRecord, error) {
	ids, err := s.teamPathIDs()
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/agent-teams"
	members := make([]map[string]string, 0, len(request.Members))
	for _, member := range request.Members {
		if member == nil {
			return nil, ErrIDRequired
		}
		members = append(members, map[string]string{"agent_workspace_id": member.GetAgentWorkspaceId(), "agent_id": member.GetAgentId()})
	}
	body := map[string]any{"name": request.Name, "description": request.Description, "members": members}

	out := &AgentTeamRecord{}
	if err := s.do(ctx, http.MethodPost, withQuery(path, nil), body, &teamResponse{message: out, array: false}); err != nil {
		return nil, err
	}
	return out, nil
}

// ListTeams calls the corresponding Catalog-owned Team product operation.
func (s *AgentService) ListTeams(ctx context.Context, opts ...QueryOption) (*AgentTeamListResult, error) {
	ids, err := s.teamPathIDs()
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/agent-teams"

	out := &AgentTeamListResult{}
	if err := s.do(ctx, http.MethodGet, withQuery(path, opts), nil, &teamResponse{message: out, array: false}); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTeam calls the corresponding Catalog-owned Team product operation.
func (s *AgentService) GetTeam(ctx context.Context, teamId string, opts ...QueryOption) (*AgentTeamRecord, error) {
	ids, err := s.teamPathIDs(teamId)
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/agent-teams" + "/" + ids[1]

	out := &AgentTeamRecord{}
	if err := s.do(ctx, http.MethodGet, withQuery(path, opts), nil, &teamResponse{message: out, array: false}); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateTeam calls the corresponding Catalog-owned Team product operation.
func (s *AgentService) UpdateTeam(ctx context.Context, teamId string, request AgentTeamUpdateRequest) (*AgentTeamRecord, error) {
	ids, err := s.teamPathIDs(teamId)
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/agent-teams" + "/" + ids[1]

	out := &AgentTeamRecord{}
	if err := s.do(ctx, http.MethodPatch, withQuery(path, nil), request, &teamResponse{message: out, array: false}); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteTeam calls the corresponding Catalog-owned Team product operation.
func (s *AgentService) DeleteTeam(ctx context.Context, teamId string, expectedVersion int64) (*AgentTeamRecord, error) {
	ids, err := s.teamPathIDs(teamId)
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/agent-teams" + "/" + ids[1]

	out := &AgentTeamRecord{}
	if err := s.do(ctx, http.MethodDelete, withQuery(path, []QueryOption{WithQueryParam("expected_version", strconv.FormatInt(expectedVersion, 10))}), nil, &teamResponse{message: out, array: false}); err != nil {
		return nil, err
	}
	return out, nil
}

// ListTeamMembers calls the corresponding Catalog-owned Team product operation.
func (s *AgentService) ListTeamMembers(ctx context.Context, teamId string, opts ...QueryOption) (*AgentTeamMemberList, error) {
	ids, err := s.teamPathIDs(teamId)
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/agent-teams" + "/" + ids[1] + "/members"

	out := &AgentTeamMemberList{}
	if err := s.do(ctx, http.MethodGet, withQuery(path, opts), nil, &teamResponse{message: out, array: true}); err != nil {
		return nil, err
	}
	return out, nil
}

// AddTeamMember calls the corresponding Catalog-owned Team product operation.
func (s *AgentService) AddTeamMember(ctx context.Context, teamId string, agentWorkspaceId string, agentId string, expectedMembershipVersion int64) (*AgentTeamRecord, error) {
	ids, err := s.teamPathIDs(teamId, agentWorkspaceId, agentId)
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/agent-teams" + "/" + ids[1] + "/members" + "/" + ids[2] + "/" + ids[3]

	out := &AgentTeamRecord{}
	if err := s.do(ctx, http.MethodPut, withQuery(path, []QueryOption{WithQueryParam("expected_membership_version", strconv.FormatInt(expectedMembershipVersion, 10))}), nil, &teamResponse{message: out, array: false}); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveTeamMember calls the corresponding Catalog-owned Team product operation.
func (s *AgentService) RemoveTeamMember(ctx context.Context, teamId string, agentWorkspaceId string, agentId string, expectedMembershipVersion int64) (*AgentTeamRecord, error) {
	ids, err := s.teamPathIDs(teamId, agentWorkspaceId, agentId)
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/agent-teams" + "/" + ids[1] + "/members" + "/" + ids[2] + "/" + ids[3]

	out := &AgentTeamRecord{}
	if err := s.do(ctx, http.MethodDelete, withQuery(path, []QueryOption{WithQueryParam("expected_membership_version", strconv.FormatInt(expectedMembershipVersion, 10))}), nil, &teamResponse{message: out, array: false}); err != nil {
		return nil, err
	}
	return out, nil
}

// ListIngressConnections lists inbound connections associated with one typed target.
func (s *AgentService) ListIngressConnections(ctx context.Context, target *AgentTargetRef, opts ...QueryOption) (*IngressConnectionList, error) {
	ids, err := s.teamPathIDs()
	if err != nil {
		return nil, err
	}
	if target == nil || target.GetKind() == "" || target.GetWorkspaceId() == "" || target.GetId() == "" {
		return nil, ErrIDRequired
	}
	query := append([]QueryOption{}, opts...)
	query = append(query,
		WithQueryParam("target_kind", target.GetKind()),
		WithQueryParam("target_workspace_id", target.GetWorkspaceId()),
		WithQueryParam("target_id", target.GetId()),
	)
	out := &IngressConnectionList{}
	if err := s.do(ctx, http.MethodGet, withQuery("/workspaces/"+ids[0]+"/ingress-connections", query), nil, &teamResponse{message: out}); err != nil {
		return nil, err
	}
	return out, nil
}

// ListIngressCapabilities returns the provider receive modes implemented by
// the connected MOI deployment.
func (s *AgentService) ListIngressCapabilities(ctx context.Context) (*IngressCapabilityList, error) {
	ids, err := s.teamPathIDs()
	if err != nil {
		return nil, err
	}
	out := &IngressCapabilityList{}
	if err := s.do(ctx, http.MethodGet, "/workspaces/"+ids[0]+"/ingress-capabilities", nil, &teamResponse{message: out}); err != nil {
		return nil, err
	}
	return out, nil
}

// ResolveWeComQRBinding starts a QR session when sessionToken is empty, or polls
// the existing session. Authorized credentials are only for creating a connection.
func (s *AgentService) ResolveWeComQRBinding(ctx context.Context, target *AgentTargetRef, sessionToken string) (*WeComQRBindingResult, error) {
	ids, err := s.teamPathIDs()
	if err != nil {
		return nil, err
	}
	request := struct {
		Target       *AgentTargetRef `json:"target"`
		SessionToken string          `json:"session_token"`
	}{target, sessionToken}
	out := &WeComQRBindingResult{}
	if err := s.do(ctx, http.MethodPost, "/workspaces/"+ids[0]+"/ingress-connections/wecom-qr", request, &teamResponse{message: out}); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateIngressConnection creates either a callback/mail connection backed by
// a ChannelInstance or a managed provider connection with its own credential.
func (s *AgentService) CreateIngressConnection(ctx context.Context, request IngressConnectionCreateRequest) (*IngressConnection, error) {
	ids, err := s.teamPathIDs()
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/ingress-connections"
	out := &IngressConnection{}
	if err := s.do(ctx, http.MethodPost, path, request, &teamResponse{message: out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AgentService) DeleteIngressConnection(ctx context.Context, connectionID string, expectedAssociationVersion, expectedConfigVersion int64) (*IngressConnection, error) {
	ids, err := s.teamPathIDs(connectionID)
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/ingress-connections/" + ids[1]
	out := &IngressConnection{}
	query := []QueryOption{WithQueryParam("expected_association_version", strconv.FormatInt(expectedAssociationVersion, 10)), WithQueryParam("expected_config_version", strconv.FormatInt(expectedConfigVersion, 10))}
	if err := s.do(ctx, http.MethodDelete, withQuery(path, query), nil, &teamResponse{message: out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AgentService) GetIngressConnection(ctx context.Context, connectionID string, opts ...QueryOption) (*IngressConnection, error) {
	ids, err := s.teamPathIDs(connectionID)
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/ingress-connections/" + ids[1]
	out := &IngressConnection{}
	if err := s.do(ctx, http.MethodGet, withQuery(path, opts), nil, &teamResponse{message: out, array: false}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AgentService) UpdateIngressConnection(ctx context.Context, connectionID string, request IngressConnectionUpdateRequest) (*IngressConnection, error) {
	ids, err := s.teamPathIDs(connectionID)
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/ingress-connections/" + ids[1]
	out := &IngressConnection{}
	if err := s.do(ctx, http.MethodPatch, path, request, &teamResponse{message: out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AgentService) VerifyIngressConnection(ctx context.Context, connectionID string, request IngressConnectionVerifyRequest) (*IngressVerificationResult, error) {
	ids, err := s.teamPathIDs(connectionID)
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/ingress-connections/" + ids[1] + "/test"
	out := &IngressVerificationResult{}
	if err := s.do(ctx, http.MethodPost, path, request, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListTeamSessions calls the corresponding Catalog-owned Team product operation.
func (s *AgentService) ListTeamSessions(ctx context.Context, teamId string, opts ...QueryOption) (*AgentTeamSessionListResult, error) {
	ids, err := s.teamPathIDs(teamId)
	if err != nil {
		return nil, err
	}
	path := "/workspaces/" + ids[0] + "/agent-teams" + "/" + ids[1] + "/sessions"

	out := &AgentTeamSessionListResult{}
	if err := s.do(ctx, http.MethodGet, withQuery(path, opts), nil, &teamResponse{message: out, array: false}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AgentService) CreateTeamSession(ctx context.Context, teamID string, request AgentTeamSessionCreateRequest) (*AgentTeamSession, error) {
	ids, err := s.teamPathIDs(teamID)
	if err != nil {
		return nil, err
	}
	out := &AgentTeamSession{}
	if err := s.do(ctx, http.MethodPost, "/workspaces/"+ids[0]+"/agent-teams/"+ids[1]+"/sessions", request, &teamResponse{message: out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AgentService) RenameTeamSession(ctx context.Context, teamID, contextID string, request AgentTeamSessionRenameRequest) (*AgentTeamSession, error) {
	ids, err := s.teamPathIDs(teamID, contextID)
	if err != nil {
		return nil, err
	}
	out := &AgentTeamSession{}
	if err := s.do(ctx, http.MethodPatch, "/workspaces/"+ids[0]+"/agent-teams/"+ids[1]+"/sessions/"+ids[2], request, &teamResponse{message: out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AgentService) SubmitTeamMessage(ctx context.Context, teamID, contextID string, request AgentTeamMessageRequest) (*AgentTeamSubmitResult, error) {
	ids, err := s.teamPathIDs(teamID, contextID)
	if err != nil {
		return nil, err
	}
	out := &AgentTeamSubmitResult{}
	if err := s.do(ctx, http.MethodPost, "/workspaces/"+ids[0]+"/agent-teams/"+ids[1]+"/sessions/"+ids[2]+"/messages", request, &teamResponse{message: out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AgentService) ListTeamMessages(ctx context.Context, teamID, contextID string, opts ...QueryOption) (*AgentTeamMessageListResult, error) {
	ids, err := s.teamPathIDs(teamID, contextID)
	if err != nil {
		return nil, err
	}
	out := &AgentTeamMessageListResult{}
	path := "/workspaces/" + ids[0] + "/agent-teams/" + ids[1] + "/sessions/" + ids[2] + "/messages"
	if err := s.do(ctx, http.MethodGet, withQuery(path, opts), nil, &teamResponse{message: out}); err != nil {
		return nil, err
	}
	return out, nil
}
