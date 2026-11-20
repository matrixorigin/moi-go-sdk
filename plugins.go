package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type PluginPublication = product.PluginPublication
type PluginPublicationList = product.PluginPublicationList
type PluginRelease = product.PluginRelease
type PluginDiscoveryPage = product.PluginDiscoveryPage
type PluginCapabilityDescriptor = product.PluginCapabilityDescriptor
type PluginInstallationBinding = product.PluginInstallationBinding
type PluginInstallation = product.PluginInstallation
type PluginInstallationList = product.PluginInstallationList
type PluginConnection = product.PluginConnection
type PluginConnectionList = product.PluginConnectionList
type PluginAuthorizationStatus = product.PluginAuthorizationStatus
type PluginOAuthAction = product.PluginOAuthAction
type PluginOAuthProgress = product.PluginOAuthProgress
type PluginMutationResult = product.PluginMutationResult
type PluginInstallRequest = product.PluginInstallRequest
type PluginUpdateInstallationRequest = product.PluginUpdateInstallationRequest
type PluginCreateConnectionRequest = product.PluginCreateConnectionRequest
type PluginUpdateConnectionRequest = product.PluginUpdateConnectionRequest
type PluginStoreSecretRequest = product.PluginStoreSecretRequest

// pluginRequestBody keeps the shared protobuf request model while preserving
// scalar zero values and empty collections required by the HTTP contract.
type pluginRequestBody struct {
	proto.Message
	omitEmpty protoreflect.Name
}

func (body pluginRequestBody) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(body.Message)
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	fields := body.Message.ProtoReflect().Descriptor().Fields()
	if body.omitEmpty != "" && fields.ByName(body.omitEmpty) == nil {
		return nil, fmt.Errorf("plugin request field %q is not defined", body.omitEmpty)
	}
	for index := 0; index < fields.Len(); index++ {
		field := fields.Get(index)
		name := string(field.Name())
		if _, exists := object[name]; exists {
			continue
		}
		if field.Name() == body.omitEmpty {
			continue
		}
		var zero json.RawMessage
		switch {
		case field.IsMap():
			zero = json.RawMessage(`{}`)
		case field.Kind() == protoreflect.StringKind:
			zero = json.RawMessage(`""`)
		case field.Kind() == protoreflect.Int32Kind || field.Kind() == protoreflect.Sint32Kind || field.Kind() == protoreflect.Sfixed32Kind ||
			field.Kind() == protoreflect.Int64Kind || field.Kind() == protoreflect.Sint64Kind || field.Kind() == protoreflect.Sfixed64Kind:
			zero = json.RawMessage(`0`)
		default:
			return nil, fmt.Errorf("plugin request field %q kind %s has no HTTP zero encoding", name, field.Kind())
		}
		object[name] = zero
	}
	return json.Marshal(object)
}

// CompletePluginOAuthCallback completes a browser code or denial callback on
// the configured MOI origin. The HTTP client's cookie jar must be shared with
// the authenticated browser begin request. SDK credentials, headers and request
// modifiers are omitted; redirects are returned without following them.
// Configured endpoints containing userinfo or fragments are rejected.
func (c *Client) CompletePluginOAuthCallback(ctx context.Context, registrationID string, request *BrowserLoginCallbackRequest, issuer string) (*StreamResponse, error) {
	return c.completePluginOAuthCallback(ctx, registrationID, request, issuer, false)
}

// CompletePluginOAuthProductCallback completes the same browser protocol at the
// configured product prefix (normally /newmoi). It shares the begin request's
// cookie jar and omits SDK credentials, headers and modifiers, just like the
// legacy-root CompletePluginOAuthCallback. Redirects are never followed.
func (c *Client) CompletePluginOAuthProductCallback(ctx context.Context, registrationID string, request *BrowserLoginCallbackRequest, issuer string) (*StreamResponse, error) {
	return c.completePluginOAuthCallback(ctx, registrationID, request, issuer, true)
}

func (c *Client) completePluginOAuthCallback(ctx context.Context, registrationID string, request *BrowserLoginCallbackRequest, issuer string, productRoute bool) (*StreamResponse, error) {
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "browser callback request is required"}
	}
	registration, err := pathID(registrationID)
	if err != nil {
		return nil, err
	}
	if registrationID == "." || registrationID == ".." {
		return nil, ErrPathInvalid
	}
	configuredEndpoint, err := url.Parse(c.endpoint)
	if err != nil || configuredEndpoint.User != nil || configuredEndpoint.Fragment != "" {
		return nil, ErrPathInvalid
	}
	endpoint, err := c.nativeURL("/api/v1/plugins/oauth/callback/")
	if err != nil {
		return nil, err
	}
	if productRoute {
		configuredEndpoint.Path += "/api/v1/plugins/oauth/callback/"
		configuredEndpoint.RawPath, configuredEndpoint.RawQuery = "", ""
		endpoint = configuredEndpoint.String()
	}
	query := url.Values{"state": {request.GetState()}, "code": {request.GetCode()}, "error": {request.GetError()}}
	if issuer != "" {
		query.Set("iss", issuer)
	}
	callback, err := New(c.endpoint, WithUnauthenticated(), WithHTTPClient(c.httpClient), WithTimeout(c.timeout))
	if err != nil {
		return nil, err
	}
	response, err := callback.doStreamURL(ctx, http.MethodGet, endpoint+registration+"?"+query.Encode(), "", nil, acceptAny, true)
	var failure *Error
	if err != nil && !errors.As(err, &failure) {
		return nil, productTransportError(ctx, err)
	}
	return response, err
}

// SearchPlugins searches only IAM-visible installations and marketplace releases.
// availability is all, installed or market; cursor and limit use QueryOption.
func (s *AgentService) SearchPlugins(ctx context.Context, opts ...QueryOption) (*PluginDiscoveryPage, error) {
	out := &product.PluginDiscoveryPage{}
	err := s.doAgentResource(ctx, http.MethodGet, "plugin-discovery", "/search", nil, opts, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) DescribePluginCapability(ctx context.Context, ref string) (*PluginCapabilityDescriptor, error) {
	out := &product.PluginCapabilityDescriptor{}
	err := s.doAgentResource(ctx, http.MethodGet, "plugin-discovery", "/describe", nil, []QueryOption{WithQueryParam("ref", ref)}, requiredProductResult{value: out})
	return out, err
}

// ListPluginReleases requires namespace=system or workspace in its query options.
func (s *AgentService) ListPluginReleases(ctx context.Context, opts ...QueryOption) (*PluginPublicationList, error) {
	out := &product.PluginPublicationList{}
	err := s.doAgentResource(ctx, http.MethodGet, "plugin-releases", "", nil, opts, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) GetPluginRelease(ctx context.Context, id string, opts ...QueryOption) (*PluginRelease, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	out := &product.PluginRelease{}
	err = s.doAgentResource(ctx, http.MethodGet, "plugin-releases", "/"+path, nil, opts, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) InstallPlugin(ctx context.Context, req *PluginInstallRequest) (*PluginInstallation, error) {
	if req == nil {
		return nil, ErrSpecRequired
	}
	out := &product.PluginInstallation{}
	body := pluginRequestBody{Message: req, omitEmpty: "setup_operation_id"}
	err := s.doAgentResource(ctx, http.MethodPost, "plugin-installations", "", body, nil, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) ListPluginInstallations(ctx context.Context, opts ...QueryOption) (*PluginInstallationList, error) {
	out := &product.PluginInstallationList{}
	err := s.doAgentResource(ctx, http.MethodGet, "plugin-installations", "", nil, opts, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) GetPluginInstallation(ctx context.Context, id string) (*PluginInstallation, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	out := &product.PluginInstallation{}
	err = s.doAgentResource(ctx, http.MethodGet, "plugin-installations", "/"+path, nil, nil, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) UpdatePluginInstallation(ctx context.Context, id string, req *PluginUpdateInstallationRequest) (*PluginInstallation, error) {
	if req == nil {
		return nil, ErrSpecRequired
	}
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	out := &product.PluginInstallation{}
	body := pluginRequestBody{Message: req, omitEmpty: "setup_operation_id"}
	err = s.doAgentResource(ctx, http.MethodPatch, "plugin-installations", "/"+path, body, nil, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) DeletePluginInstallation(ctx context.Context, id string, expectedVersion int64) (*PluginMutationResult, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	out := &product.PluginMutationResult{}
	err = s.doAgentResource(ctx, http.MethodDelete, "plugin-installations", "/"+path, map[string]any{"expected_version": expectedVersion}, nil, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) CreatePluginConnection(ctx context.Context, req *PluginCreateConnectionRequest) (*PluginConnection, error) {
	if req == nil {
		return nil, ErrSpecRequired
	}
	out := &product.PluginConnection{}
	err := s.doAgentResource(ctx, http.MethodPost, "plugin-connections", "", pluginRequestBody{Message: req}, nil, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) ListPluginConnections(ctx context.Context, opts ...QueryOption) (*PluginConnectionList, error) {
	out := &product.PluginConnectionList{}
	err := s.doAgentResource(ctx, http.MethodGet, "plugin-connections", "", nil, opts, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) GetPluginConnection(ctx context.Context, id string) (*PluginConnection, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	out := &product.PluginConnection{}
	err = s.doAgentResource(ctx, http.MethodGet, "plugin-connections", "/"+path, nil, nil, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) UpdatePluginConnection(ctx context.Context, id string, req *PluginUpdateConnectionRequest) (*PluginConnection, error) {
	if req == nil {
		return nil, ErrSpecRequired
	}
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	out := &product.PluginConnection{}
	err = s.doAgentResource(ctx, http.MethodPatch, "plugin-connections", "/"+path, pluginRequestBody{Message: req}, nil, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) DeletePluginConnection(ctx context.Context, id string, expectedVersion int) (*PluginMutationResult, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	out := &product.PluginMutationResult{}
	err = s.doAgentResource(ctx, http.MethodDelete, "plugin-connections", "/"+path, map[string]any{"expected_version": expectedVersion}, nil, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) GetPluginAuthorization(ctx context.Context, id string) (*PluginAuthorizationStatus, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	out := &product.PluginAuthorizationStatus{}
	err = s.doAgentResource(ctx, http.MethodGet, "plugin-connections", "/"+path+"/authorization", nil, nil, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) StorePluginSecret(ctx context.Context, id string, req *PluginStoreSecretRequest) (*PluginAuthorizationStatus, error) {
	if req == nil {
		return nil, ErrSpecRequired
	}
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	out := &product.PluginAuthorizationStatus{}
	err = s.doAgentResource(ctx, http.MethodPost, "plugin-connections", "/"+path+"/secret", pluginRequestBody{Message: req}, nil, requiredProductResult{value: out})
	return out, err
}

// BeginPluginOAuth returns a browser handoff for code grants, or the direct
// user action for device grants. Code authorization starts in the signed-in
// browser so its proof cookie never has to leave that browser.
func (s *AgentService) BeginPluginOAuth(ctx context.Context, id string, expectedVersion int) (*PluginOAuthAction, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	out := &product.PluginOAuthAction{}
	err = s.doAgentResource(ctx, http.MethodPost, "plugin-connections", "/"+path+"/oauth", map[string]any{"expected_version": expectedVersion, "initiator": "external"}, nil, requiredProductResult{value: out})
	return out, err
}

// PollPluginOAuth performs exactly one device poll. Callers respect poll_after
// and never automatically replay an exchange after an ambiguous failure.
func (s *AgentService) PollPluginOAuth(ctx context.Context, id, attemptID string, expectedVersion int) (*PluginOAuthProgress, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	attempt, err := pathID(attemptID)
	if err != nil {
		return nil, err
	}
	out := &product.PluginOAuthProgress{}
	err = s.doAgentResource(ctx, http.MethodPost, "plugin-connections", "/"+path+"/oauth/"+attempt+"/poll", map[string]any{"expected_version": expectedVersion}, nil, requiredProductResult{value: out})
	return out, err
}

func (s *AgentService) DisconnectPluginConnection(ctx context.Context, id string, expectedGeneration int64) (*PluginMutationResult, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	out := &product.PluginMutationResult{}
	err = s.doAgentResource(ctx, http.MethodPost, "plugin-connections", "/"+path+"/disconnect", map[string]any{"expected_generation": expectedGeneration}, nil, requiredProductResult{value: out})
	return out, err
}
