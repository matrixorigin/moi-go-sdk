package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// WebappService is the route-aligned escape hatch for the anonymous public
// Agent Webapp protocol. Prefer typed AgentHandle helpers for publisher
// user-facing workflows. Link and session credentials are explicit request
// inputs; product credentials and workspace/runtime grants are never added.
type WebappService struct{ client *Client }

func (s *WebappService) do(ctx context.Context, method, path string, body any, opts []QueryOption) (*ProductData, error) {
	var requestBody io.Reader
	contentType := ""
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		requestBody = bytes.NewReader(raw)
		contentType = contentTypeJSON
	}
	response, err := s.client.doPublicRawResponse(ctx, method, withQuery(path, opts), requestBody, contentType, contentTypeJSON)
	if err != nil {
		return nil, err
	}
	return parseWebappProductData(response)
}

func parseWebappProductData(response *publicRawResponse) (*ProductData, error) {
	out := &ProductData{}
	if err := parseProductHTTPResponse(response.status, response.body, requiredProductResult{out}, response.requestID); err != nil {
		return nil, err
	}
	return out, nil
}

// Resolve resolves a long or short public Webapp link.
func (s *WebappService) Resolve(ctx context.Context, body map[string]any) (*ProductData, error) {
	return s.do(ctx, http.MethodPost, "/public/webapp/resolve", body, nil)
}

// CreateSession creates an anonymous end-user session for a public link.
func (s *WebappService) CreateSession(ctx context.Context, body map[string]any) (*ProductData, error) {
	return s.do(ctx, http.MethodPost, "/public/webapp/session", body, nil)
}

// RefreshSession slides a valid anonymous session credential.
func (s *WebappService) RefreshSession(ctx context.Context, body map[string]any) (*ProductData, error) {
	return s.do(ctx, http.MethodPost, "/public/webapp/session/refresh", body, nil)
}

// CreateConversation creates a conversation owned by the admitted end user.
func (s *WebappService) CreateConversation(ctx context.Context, body map[string]any, opts ...QueryOption) (*ProductData, error) {
	return s.do(ctx, http.MethodPost, "/public/webapp/conversations", body, opts)
}

// ListConversations lists the admitted end user's conversations.
func (s *WebappService) ListConversations(ctx context.Context, opts ...QueryOption) (*ProductData, error) {
	return s.do(ctx, http.MethodGet, "/public/webapp/conversations", nil, opts)
}

// RenameConversation renames one admitted conversation.
func (s *WebappService) RenameConversation(ctx context.Context, conversationID string, body map[string]any, opts ...QueryOption) (*ProductData, error) {
	id, err := pathID(conversationID)
	if err != nil {
		return nil, err
	}
	return s.do(ctx, http.MethodPatch, "/public/webapp/conversations/"+id, body, opts)
}

// DeleteConversation archives one admitted conversation.
func (s *WebappService) DeleteConversation(ctx context.Context, conversationID string, opts ...QueryOption) (*ProductData, error) {
	id, err := pathID(conversationID)
	if err != nil {
		return nil, err
	}
	return s.do(ctx, http.MethodDelete, "/public/webapp/conversations/"+id, nil, opts)
}

// ListMessages reads one admitted conversation's durable history.
func (s *WebappService) ListMessages(ctx context.Context, conversationID string, opts ...QueryOption) (*ProductData, error) {
	id, err := pathID(conversationID)
	if err != nil {
		return nil, err
	}
	return s.do(ctx, http.MethodGet, "/public/webapp/conversations/"+id+"/messages", nil, opts)
}

// ListTaskEvents reads a task's durable event journal after a sequence cursor.
func (s *WebappService) ListTaskEvents(ctx context.Context, conversationID, taskID string, opts ...QueryOption) (*ProductData, error) {
	conversation, err := pathID(conversationID)
	if err != nil {
		return nil, err
	}
	task, err := pathID(taskID)
	if err != nil {
		return nil, err
	}
	return s.do(ctx, http.MethodGet, "/public/webapp/conversations/"+conversation+"/tasks/"+task+"/events", nil, opts)
}

// SubmitFeedback submits feedback for one admitted conversation turn.
func (s *WebappService) SubmitFeedback(ctx context.Context, body map[string]any, opts ...QueryOption) (*ProductData, error) {
	return s.do(ctx, http.MethodPost, "/public/webapp/feedback", body, opts)
}

// CallMCPApp calls an MCP App tool in an admitted conversation turn.
func (s *WebappService) CallMCPApp(ctx context.Context, body map[string]any, opts ...QueryOption) (*ProductData, error) {
	return s.do(ctx, http.MethodPost, "/public/webapp/mcp-apps/call", body, opts)
}

// ReadMCPAppResource reads an MCP App resource in an admitted conversation turn.
func (s *WebappService) ReadMCPAppResource(ctx context.Context, body map[string]any, opts ...QueryOption) (*ProductData, error) {
	return s.do(ctx, http.MethodPost, "/public/webapp/mcp-apps/resource", body, opts)
}

// A2AStream starts the public message/stream JSON-RPC exchange. Callers own
// the response body and must close it after consuming or handing off the SSE.
func (s *WebappService) A2AStream(ctx context.Context, body map[string]any, opts ...QueryOption) (*StreamResponse, error) {
	return s.client.doStreamURLWithAuthentication(ctx, http.MethodPost, s.client.productURL(withQuery("/public/webapp/a2a", opts)), "", body, acceptEventStream, false, false)
}

func webappTaskFilePath(conversationID, taskID, fileID, action string) (string, error) {
	conversation, err := pathID(conversationID)
	if err != nil {
		return "", err
	}
	task, err := pathID(taskID)
	if err != nil {
		return "", err
	}
	file, err := pathID(fileID)
	if err != nil {
		return "", err
	}
	return "/public/webapp/conversations/" + conversation + "/tasks/" + task + "/files/" + file + "/" + action, nil
}

// DownloadTaskFile downloads one admitted runtime artifact without product authentication.
func (s *WebappService) DownloadTaskFile(ctx context.Context, conversationID, taskID, fileID string, opts ...QueryOption) (*StreamResponse, error) {
	path, err := webappTaskFilePath(conversationID, taskID, fileID, "download")
	if err != nil {
		return nil, err
	}
	return s.client.doStreamURLWithAuthentication(ctx, http.MethodGet, s.client.productURL(withQuery(path, opts)), "", nil, acceptAny, false, false)
}

// PreviewTaskFile streams one admitted runtime artifact's browser preview.
func (s *WebappService) PreviewTaskFile(ctx context.Context, conversationID, taskID, fileID string, opts ...QueryOption) (*StreamResponse, error) {
	path, err := webappTaskFilePath(conversationID, taskID, fileID, "preview")
	if err != nil {
		return nil, err
	}
	return s.client.doStreamURLWithAuthentication(ctx, http.MethodGet, s.client.productURL(withQuery(path, opts)), "", nil, acceptAny, false, false)
}

// UploadFile uploads one multipart file into an admitted public conversation.
func (s *WebappService) UploadFile(ctx context.Context, filename, contentType string, reader io.Reader, opts ...QueryOption) (*ProductData, error) {
	body, multipartContentType, err := multipartBody(nil, []MultipartFile{{FieldName: "file", Filename: filename, ContentType: contentType, Reader: reader}})
	if err != nil {
		return nil, err
	}
	response, err := s.client.doPublicRawResponse(ctx, http.MethodPost, withQuery("/public/webapp/files", opts), body, multipartContentType, contentTypeJSON)
	if err != nil {
		return nil, err
	}
	return parseWebappProductData(response)
}
