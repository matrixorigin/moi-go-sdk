package sdk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

const (
	callbackProviderWeCom = "wecom"
)

// ChannelInstanceRecord is one workspace channel provider configuration instance.
type ChannelInstanceRecord struct {
	ID                string            `json:"id"`
	WorkspaceID       string            `json:"workspace_id"`
	Provider          string            `json:"provider"`
	ChannelType       string            `json:"channel_type"`
	Name              string            `json:"name"`
	Description       string            `json:"description,omitempty"`
	Status            string            `json:"status"`
	CredentialRef     string            `json:"credential_ref"`
	CallbackPath      string            `json:"callback_path,omitempty"`
	Capabilities      []string          `json:"capabilities,omitempty"`
	ToolIDs           []string          `json:"tool_ids,omitempty"`
	Config            map[string]any    `json:"config,omitempty"`
	LastTestStatus    string            `json:"last_test_status,omitempty"`
	LastTestedAt      string            `json:"last_tested_at,omitempty"`
	LastTestError     string            `json:"last_test_error,omitempty"`
	LastTestErrorCode string            `json:"last_test_error_code,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	CreatedAt         string            `json:"created_at"`
	UpdatedAt         string            `json:"updated_at"`
	Version           int               `json:"version"`
}

// ChannelInstanceListResult is the channel instance list response.
type ChannelInstanceListResult struct {
	Items  []ChannelInstanceRecord `json:"items"`
	Total  int                     `json:"total"`
	Limit  int                     `json:"limit"`
	Offset int                     `json:"offset"`
}

// ChannelInstanceCreateRequest is the single channel instance creation payload.
// channel_type selects the provider-specific config and secret keys.
type ChannelInstanceCreateRequest struct {
	ChannelType string            `json:"channel_type"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
	Secrets     map[string]string `json:"secrets,omitempty"`
	Visibility  string            `json:"visibility,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// ChannelInstanceUpdateRequest is the single partial channel instance update payload.
type ChannelInstanceUpdateRequest struct {
	ChannelType     *string            `json:"channel_type,omitempty"`
	Name            *string            `json:"name,omitempty"`
	Description     *string            `json:"description,omitempty"`
	Config          map[string]string  `json:"config,omitempty"`
	Secrets         map[string]string  `json:"secrets,omitempty"`
	Visibility      *string            `json:"visibility,omitempty"`
	Labels          *map[string]string `json:"labels,omitempty"`
	ExpectedVersion int                `json:"expected_version,omitempty"`
}

// ChannelInstanceTestResult is returned after testing one channel instance.
type ChannelInstanceTestResult struct {
	Instance        *ChannelInstanceRecord `json:"instance,omitempty"`
	OK              bool                   `json:"ok"`
	Status          string                 `json:"status"`
	WorkerID        string                 `json:"worker_id,omitempty"`
	RequestID       string                 `json:"request_id,omitempty"`
	LastTestStatus  string                 `json:"last_test_status"`
	LastTestError   string                 `json:"last_test_error,omitempty"`
	ErrorCode       string                 `json:"error_code,omitempty"`
	ErrorStage      string                 `json:"error_stage,omitempty"`
	ProviderCode    string                 `json:"provider_error_code,omitempty"`
	ProviderMessage string                 `json:"provider_error_message,omitempty"`
	ErrorCategory   string                 `json:"error_category,omitempty"`
	Suggestion      string                 `json:"suggestion,omitempty"`
	LastTestedAt    string                 `json:"last_tested_at"`
}

// CallbackRuleMatch describes callback event match predicates.
type CallbackRuleMatch struct {
	ResourceKinds []string          `json:"resource_kinds,omitempty"`
	EventTypes    []string          `json:"event_types,omitempty"`
	EventActions  []string          `json:"event_actions,omitempty"`
	FieldEquals   map[string]string `json:"field_equals,omitempty"`
}

// CallbackRuleRecord is a workspace/provider/credential scoped callback routing rule.
type CallbackRuleRecord struct {
	ID             string            `json:"id"`
	WorkspaceID    string            `json:"workspace_id"`
	Provider       string            `json:"provider"`
	CredentialRef  string            `json:"credential_ref"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	Enabled        bool              `json:"enabled"`
	Priority       int               `json:"priority"`
	Match          CallbackRuleMatch `json:"match"`
	TargetType     string            `json:"target_type"`
	TargetConfig   map[string]any    `json:"target_config,omitempty"`
	StopAfterMatch bool              `json:"stop_after_match"`
	Version        int               `json:"version"`
	CreatedBy      string            `json:"created_by,omitempty"`
	UpdatedBy      string            `json:"updated_by,omitempty"`
	CreatedAt      string            `json:"created_at"`
	UpdatedAt      string            `json:"updated_at"`
}

// CallbackRuleCreateRequest creates a callback routing rule.
type CallbackRuleCreateRequest struct {
	Provider       string            `json:"provider"`
	CredentialRef  string            `json:"credential_ref"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	Enabled        *bool             `json:"enabled,omitempty"`
	Priority       int               `json:"priority,omitempty"`
	Match          CallbackRuleMatch `json:"match,omitempty"`
	TargetType     string            `json:"target_type"`
	TargetConfig   map[string]any    `json:"target_config,omitempty"`
	StopAfterMatch bool              `json:"stop_after_match,omitempty"`
}

// CallbackRuleUpdateRequest updates a callback routing rule with optimistic locking.
type CallbackRuleUpdateRequest struct {
	Name           *string            `json:"name,omitempty"`
	Description    *string            `json:"description,omitempty"`
	Enabled        *bool              `json:"enabled,omitempty"`
	Priority       *int               `json:"priority,omitempty"`
	Match          *CallbackRuleMatch `json:"match,omitempty"`
	TargetType     *string            `json:"target_type,omitempty"`
	TargetConfig   map[string]any     `json:"target_config,omitempty"`
	StopAfterMatch *bool              `json:"stop_after_match,omitempty"`
	Version        int                `json:"version"`
}

// CallbackRuleListResult is the callback routing rule list response.
type CallbackRuleListResult struct {
	Items  []CallbackRuleRecord `json:"items"`
	Total  int                  `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}

// CallbackRuleDeleteResult is returned after deleting a callback routing rule.
type CallbackRuleDeleteResult struct {
	Deleted bool `json:"deleted"`
}

// ConnectorCatalogItem describes one backend-owned connector capability and its
// workspace configuration state.
type ConnectorCatalogItem struct {
	Provider    string `json:"provider"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Configured  bool   `json:"configured"`
}

// ConnectorCatalogResult is the workspace connector catalog.
type ConnectorCatalogResult struct {
	Items []ConnectorCatalogItem `json:"items"`
}

// ListConnectorCatalog returns the backend-owned connector catalog for this workspace.
func (s *AgentService) ListConnectorCatalog(ctx context.Context) (*ConnectorCatalogResult, error) {
	var out ConnectorCatalogResult
	err := s.doAgentResource(ctx, http.MethodGet, "connector-catalog", "", nil, nil, &out)
	return &out, err
}

// GenerateWeComCallbackSecrets returns fresh callback credentials without
// persisting or rotating a channel connection.
func (s *AgentService) GenerateWeComCallbackSecrets(ctx context.Context) (*ProductData, error) {
	var out ProductData
	err := s.doAgentResource(ctx, http.MethodPost, "tools", "/wecom/callback-secrets/generate", nil, nil, &out)
	return &out, err
}

// CreateChannelInstance creates one channel instance through the provider route.
func (s *AgentService) CreateChannelInstance(ctx context.Context, provider string, req ChannelInstanceCreateRequest) (*ChannelInstanceRecord, error) {
	providerPath, err := pathID(provider)
	if err != nil {
		return nil, err
	}
	var out ChannelInstanceRecord
	err = s.doAgentResource(ctx, http.MethodPost, "channels", "/"+providerPath+"/instances", req, nil, &out)
	return &out, err
}

// ListChannelInstances lists channel provider instances in this workspace.
func (s *AgentService) ListChannelInstances(ctx context.Context, provider string, opts ...QueryOption) (*ChannelInstanceListResult, error) {
	providerPath, err := pathID(provider)
	if err != nil {
		return nil, err
	}
	var out ChannelInstanceListResult
	err = s.doAgentResource(ctx, http.MethodGet, "channels", "/"+providerPath+"/instances", nil, opts, &out)
	return &out, err
}

// GetChannelInstance returns one channel instance by provider and instance id.
func (s *AgentService) GetChannelInstance(ctx context.Context, provider, instanceID string) (*ChannelInstanceRecord, error) {
	providerPath, err := pathID(provider)
	if err != nil {
		return nil, err
	}
	instancePath, err := pathID(instanceID)
	if err != nil {
		return nil, err
	}
	var out ChannelInstanceRecord
	err = s.doAgentResource(ctx, http.MethodGet, "channels", "/"+providerPath+"/instances/"+instancePath, nil, nil, &out)
	return &out, err
}

// UpdateChannelInstance updates one channel instance through the provider route.
func (s *AgentService) UpdateChannelInstance(ctx context.Context, provider, instanceID string, req ChannelInstanceUpdateRequest) (*ChannelInstanceRecord, error) {
	providerPath, err := pathID(provider)
	if err != nil {
		return nil, err
	}
	instancePath, err := pathID(instanceID)
	if err != nil {
		return nil, err
	}
	var out ChannelInstanceRecord
	err = s.doAgentResource(ctx, http.MethodPatch, "channels", "/"+providerPath+"/instances/"+instancePath, req, nil, &out)
	return &out, err
}

// TestChannelConfiguration validates an unsaved channel configuration.
func (s *AgentService) TestChannelConfiguration(ctx context.Context, provider string, req ChannelInstanceCreateRequest) (*ChannelInstanceTestResult, error) {
	providerPath, err := pathID(provider)
	if err != nil {
		return nil, err
	}
	var out ChannelInstanceTestResult
	err = s.doAgentResource(ctx, http.MethodPost, "channels", "/"+providerPath+"/instances/test", req, nil, &out)
	return &out, err
}

// DisableChannelInstance disables one channel instance and its credential.
func (s *AgentService) DisableChannelInstance(ctx context.Context, provider, instanceID string) (*ChannelInstanceRecord, error) {
	providerPath, err := pathID(provider)
	if err != nil {
		return nil, err
	}
	instancePath, err := pathID(instanceID)
	if err != nil {
		return nil, err
	}
	var out ChannelInstanceRecord
	err = s.doAgentResource(ctx, http.MethodDelete, "channels", "/"+providerPath+"/instances/"+instancePath, nil, nil, &out)
	return &out, err
}

// TestChannelInstance tests one channel instance credential.
func (s *AgentService) TestChannelInstance(ctx context.Context, provider, instanceID string) (*ChannelInstanceTestResult, error) {
	providerPath, err := pathID(provider)
	if err != nil {
		return nil, err
	}
	instancePath, err := pathID(instanceID)
	if err != nil {
		return nil, err
	}
	var out ChannelInstanceTestResult
	err = s.doAgentResource(ctx, http.MethodPost, "channels", "/"+providerPath+"/instances/"+instancePath+"/test", nil, nil, &out)
	return &out, err
}

// CreateCallbackRule creates a callback routing rule scoped to this workspace.
func (s *AgentService) CreateCallbackRule(ctx context.Context, req CallbackRuleCreateRequest) (*CallbackRuleRecord, error) {
	var out CallbackRuleRecord
	err := s.doAgentResource(ctx, http.MethodPost, "callback-rules", "", req, nil, &out)
	return &out, err
}

// ListCallbackRules lists callback routing rules scoped to this workspace.
func (s *AgentService) ListCallbackRules(ctx context.Context, opts ...QueryOption) (*CallbackRuleListResult, error) {
	var out CallbackRuleListResult
	err := s.doAgentResource(ctx, http.MethodGet, "callback-rules", "", nil, opts, &out)
	return &out, err
}

// GetCallbackRule returns one callback routing rule by id.
func (s *AgentService) GetCallbackRule(ctx context.Context, ruleID string) (*CallbackRuleRecord, error) {
	rulePath, err := pathID(ruleID)
	if err != nil {
		return nil, err
	}
	var out CallbackRuleRecord
	err = s.doAgentResource(ctx, http.MethodGet, "callback-rules", "/"+rulePath, nil, nil, &out)
	return &out, err
}

// UpdateCallbackRule updates one callback routing rule by id.
func (s *AgentService) UpdateCallbackRule(ctx context.Context, ruleID string, req CallbackRuleUpdateRequest) (*CallbackRuleRecord, error) {
	rulePath, err := pathID(ruleID)
	if err != nil {
		return nil, err
	}
	var out CallbackRuleRecord
	err = s.doAgentResource(ctx, http.MethodPatch, "callback-rules", "/"+rulePath, req, nil, &out)
	return &out, err
}

// DeleteCallbackRule deletes one callback routing rule by id.
func (s *AgentService) DeleteCallbackRule(ctx context.Context, ruleID string) (*CallbackRuleDeleteResult, error) {
	rulePath, err := pathID(ruleID)
	if err != nil {
		return nil, err
	}
	var out CallbackRuleDeleteResult
	err = s.doAgentResource(ctx, http.MethodDelete, "callback-rules", "/"+rulePath, nil, nil, &out)
	return &out, err
}

func (s *AgentService) doAgentResource(ctx context.Context, method, resource, tailPath string, body any, opts []QueryOption, result any) error {
	resourcePath, err := pathID(resource)
	if err != nil {
		return err
	}
	path, err := agentResourcePath(s.workspaceID, resourcePath, tailPath)
	if err != nil {
		return err
	}
	return s.do(ctx, method, withQuery(path, opts), body, result)
}

// VerifyWeComCallback sends the public WeCom URL verification request without product authentication.
func (c *Client) VerifyWeComCallback(ctx context.Context, workspaceID, credentialRef string, opts ...QueryOption) ([]byte, error) {
	return c.VerifyChannelCallback(ctx, callbackProviderWeCom, workspaceID, credentialRef, opts...)
}

// VerifyChannelCallback sends a provider challenge without product authentication.
func (c *Client) VerifyChannelCallback(ctx context.Context, provider, workspaceID, credentialRef string, opts ...QueryOption) ([]byte, error) {
	path, err := providerCallbackPath("callbacks", provider, workspaceID, credentialRef)
	if err != nil {
		return nil, err
	}
	return c.doPublicRaw(ctx, http.MethodGet, withQuery(path, opts), nil, "", "text/plain")
}

// PostWeComCallback sends a public WeCom event callback without product authentication.
func (c *Client) PostWeComCallback(ctx context.Context, workspaceID, credentialRef string, body []byte, opts ...QueryOption) ([]byte, error) {
	path, err := weComCallbackPath(workspaceID, credentialRef)
	if err != nil {
		return nil, err
	}
	return c.doPublicRaw(ctx, http.MethodPost, withQuery(path, opts), bytes.NewReader(body), "application/xml; charset=utf-8", "text/plain")
}

// PostChannelCallback sends a provider webhook to the public callback endpoint
// without product authentication. Provider-specific signature headers can be
// supplied with WithRequestModifier.
func (c *Client) PostChannelCallback(
	ctx context.Context,
	provider string,
	workspaceID string,
	credentialRef string,
	body []byte,
	opts ...QueryOption,
) ([]byte, error) {
	path, err := providerCallbackPath("callbacks", provider, workspaceID, credentialRef)
	if err != nil {
		return nil, err
	}
	return c.doPublicRaw(ctx, http.MethodPost, withQuery(path, opts), bytes.NewReader(body), "application/json; charset=utf-8", "text/plain")
}

// VerifyIngressCallback sends a provider challenge to a connection's public
// ingress endpoint without product authentication.
func (c *Client) VerifyIngressCallback(ctx context.Context, provider, workspaceID, connectionID string, opts ...QueryOption) ([]byte, error) {
	path, err := providerCallbackPath("ingress-callbacks", provider, workspaceID, connectionID)
	if err != nil {
		return nil, err
	}
	return c.doPublicRaw(ctx, http.MethodGet, withQuery(path, opts), nil, "", "text/plain")
}

// PostIngressCallback sends the provider's raw payload to a connection's public
// ingress endpoint. Signature headers use WithRequestModifier.
func (c *Client) PostIngressCallback(ctx context.Context, provider, workspaceID, connectionID string, body []byte, opts ...QueryOption) ([]byte, error) {
	path, err := providerCallbackPath("ingress-callbacks", provider, workspaceID, connectionID)
	if err != nil {
		return nil, err
	}
	return c.doPublicRaw(ctx, http.MethodPost, withQuery(path, opts), bytes.NewReader(body), "application/octet-stream", "text/plain")
}

func weComCallbackPath(workspaceID, credentialRef string) (string, error) {
	workspacePath, err := pathID(workspaceID)
	if err != nil {
		return "", err
	}
	credentialPath, err := pathID(credentialRef)
	if err != nil {
		return "", err
	}
	return "/callbacks/wecom/" + workspacePath + "/" + credentialPath, nil
}

func providerCallbackPath(route, provider, workspaceID, referenceID string) (string, error) {
	providerPath, err := pathID(provider)
	if err != nil {
		return "", err
	}
	workspacePath, err := pathID(workspaceID)
	if err != nil {
		return "", err
	}
	referencePath, err := pathID(referenceID)
	if err != nil {
		return "", err
	}
	return "/" + route + "/" + providerPath + "/" + workspacePath + "/" + referencePath, nil
}

func (c *Client) doPublicRaw(ctx context.Context, method, path string, body io.Reader, contentType, accept string) ([]byte, error) {
	response, err := c.doPublicRawResponse(ctx, method, path, body, contentType, accept)
	if err != nil {
		return nil, err
	}
	return response.body, nil
}

type publicRawResponse struct {
	status    int
	requestID string
	body      []byte
}

func (c *Client) doPublicRawResponse(ctx context.Context, method, path string, body io.Reader, contentType, accept string) (*publicRawResponse, error) {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, method, c.productURL(path), body)
	if err != nil {
		return nil, fmt.Errorf("create product public request: %w", err)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if contentType != "" {
		req.Header.Set(headerContentType, contentType)
	}
	for k, v := range c.customHeaders {
		req.Header.Set(k, v)
	}
	for _, modifier := range c.requestModifier {
		modifier(req)
	}
	clearPublicProductHeaders(req.Header)
	httpClient := c.httpClient
	if httpClient.Jar != nil {
		clone := *httpClient
		clone.Jar = nil
		httpClient = &clone
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, productTransportError(ctx, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, productResponseReadError(ctx, resp.StatusCode, resp.Header.Get(headerRequestID), err)
	}
	result := &publicRawResponse{status: resp.StatusCode, requestID: resp.Header.Get(headerRequestID), body: respBody}
	if resp.StatusCode >= 400 {
		return nil, parseHTTPError(resp.StatusCode, respBody, resp.Header.Get(headerRequestID))
	}
	return result, nil
}
