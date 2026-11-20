package sdk

import (
	"context"
	"fmt"
	"io"
	"net/http"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/types/known/structpb"
)

type RuntimeGatewayService struct{ client *Client }

// RuntimeGateway exposes source callbacks. Supply the task grant as the bearer
// credential; a product API key is not a runtime grant.
func (c *Client) RuntimeGateway() *RuntimeGatewayService { return &RuntimeGatewayService{client: c} }

// RuntimeModelChatRequest carries the extensible OpenAI chat-completions wire object.
type RuntimeModelChatRequest = structpb.Struct

type RuntimeEdgeTokenCheck = product.RuntimeEdgeTokenCheck
type RuntimeEdgeTokenRenewal = product.RuntimeEdgeTokenRenewal
type RuntimeModelResolveRequest = product.RuntimeModelResolveRequest
type RuntimeModelResolution = product.RuntimeModelResolution
type RuntimeRPCRequest = product.RuntimeRPCRequest
type RuntimeRPCResponse = product.RuntimeRPCResponse
type RuntimeExecutorAuthorizationRequest = product.RuntimeExecutorAuthorizationRequest
type ComputeBindingEvent = product.ComputeBindingEvent

func (s *RuntimeGatewayService) call(ctx context.Context, path string, body, result any) error {
	endpoint, err := s.client.nativeURL(path)
	if err != nil {
		return err
	}
	return s.client.doURL(ctx, http.MethodPost, endpoint, "", body, result)
}
func (s *RuntimeGatewayService) CheckEdgeToken(ctx context.Context, token string) (*RuntimeEdgeTokenCheck, error) {
	out := &RuntimeEdgeTokenCheck{}
	err := s.call(ctx, "/api/v1/astra/edge-tokens/check", &product.RuntimeEdgeTokenRequest{Token: token}, out)
	return out, err
}
func (s *RuntimeGatewayService) RenewEdgeToken(ctx context.Context, token string) (*RuntimeEdgeTokenRenewal, error) {
	out := &RuntimeEdgeTokenRenewal{}
	err := s.client.do(ctx, http.MethodPost, "/edge-tokens/renew", "", &product.RuntimeEdgeTokenRequest{Token: token}, out)
	return out, err
}
func (s *RuntimeGatewayService) ResolveModel(ctx context.Context, request *RuntimeModelResolveRequest) (*RuntimeModelResolution, error) {
	out := &RuntimeModelResolution{}
	err := s.call(ctx, "/api/v1/models/resolve", request, out)
	return out, err
}
func (s *RuntimeGatewayService) InvokeModel(ctx context.Context, request *RuntimeModelChatRequest) (*StreamResponse, error) {
	endpoint, err := s.client.nativeURL("/api/v1/models/openai/chat/completions")
	if err != nil {
		return nil, err
	}
	return s.client.doStreamURL(ctx, http.MethodPost, endpoint, "", request, acceptAny, false)
}
func (s *RuntimeGatewayService) MCP(ctx context.Context, request *RuntimeRPCRequest) (*RuntimeRPCResponse, error) {
	if request.GetMethod() == "notifications/cancelled" && request.GetId() == nil {
		endpoint, err := s.client.nativeURL("/api/v1/mcp/http")
		if err != nil {
			return nil, err
		}
		return nil, s.client.notifyRuntimeMCP(ctx, endpoint, "", request)
	}
	out := &RuntimeRPCResponse{}
	err := s.call(ctx, "/api/v1/mcp/http", request, out)
	return out, err
}
func (s *RuntimeGatewayService) AuthorizeExecutor(ctx context.Context, request *RuntimeExecutorAuthorizationRequest) error {
	return s.call(ctx, "/api/v1/runtime-executors/authorize", request, nil)
}
func (s *RuntimeGatewayService) ApplyComputeBindingEvent(ctx context.Context, request *ComputeBindingEvent) (*ProductData, error) {
	out := &ProductData{}
	err := s.call(ctx, "/api/v1/internal/compute-bindings/events", request, out)
	return out, err
}

// WorkspaceMCP requires product authentication and WithRuntimeAuthorization
// containing the live task grant for the authenticated actor and agent.
func (s *AgentService) WorkspaceMCP(ctx context.Context, agentID string, request *RuntimeRPCRequest) (*RuntimeRPCResponse, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	agent, err := pathID(agentID)
	if err != nil {
		return nil, err
	}
	if request.GetMethod() == "notifications/cancelled" && request.GetId() == nil {
		endpoint := s.client.productURL("/workspaces/" + workspace + "/agents/" + agent + "/mcp/http")
		return nil, s.client.notifyRuntimeMCP(ctx, endpoint, s.workspaceID, request)
	}
	out := &RuntimeRPCResponse{}
	err = s.do(ctx, http.MethodPost, "/workspaces/"+workspace+"/agents/"+agent+"/mcp/http", request, out)
	return out, err
}

// Notification acceptance is not a tool response or execution-settlement proof.
func (c *Client) notifyRuntimeMCP(ctx context.Context, endpoint, workspaceID string, request *RuntimeRPCRequest) error {
	response, err := c.doStreamURL(ctx, http.MethodPost, endpoint, workspaceID, request, contentTypeJSON, false)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1))
	if err != nil {
		return fmt.Errorf("read MCP notification acknowledgement: %w", err)
	}
	if response.StatusCode != http.StatusAccepted || len(body) != 0 {
		return &Error{Code: "ErrResponseInvalid", Message: "MCP notification requires an empty HTTP 202 acknowledgement", RequestID: response.Header.Get(headerRequestID)}
	}
	return nil
}
