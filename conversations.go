package sdk

import (
	"context"
	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"net/http"
	"net/url"
	"strconv"
)

// ConversationService owns the published persisted session/message APIs.
// Their numeric IDs and tag lifecycle differ from Agent A2A task contexts.
type ConversationService struct{ workspaceService }

func (c *Client) Conversations(workspaceID string) *ConversationService {
	return &ConversationService{workspaceService{client: c, workspaceID: workspaceID}}
}
func (s *ConversationService) workspacePath() (string, error) {
	id, err := pathID(s.workspaceID)
	return "/workspaces/" + id + "/llm", err
}

type ConversationTag = product.ConversationTag
type ConversationSession = product.ConversationSession
type ConversationSessionList = product.ConversationSessionList
type ConversationUISessionRequest = product.ConversationUISessionRequest
type ConversationWorkspaceSessionRequest = product.ConversationWorkspaceSessionRequest
type ConversationUISessionListRequest = product.ConversationUISessionListRequest
type ConversationMutation = product.ConversationMutation
type ConversationUIMessageListRequest = product.ConversationUIMessageListRequest
type ConversationUIMessage = product.ConversationUIMessage
type ConversationWorkspaceMessage = product.ConversationWorkspaceMessage
type ConversationWorkspaceMessageRequest = product.ConversationWorkspaceMessageRequest
type ConversationWorkspaceMessageList = product.ConversationWorkspaceMessageList
type ConversationLatestMessageID = product.ConversationLatestMessageID
type ConversationTagRequest = product.ConversationTagRequest
type ConversationTagList = product.ConversationTagList
type conversationUIMessageList struct{ items []*ConversationUIMessage }

func (result *conversationUIMessageList) UnmarshalJSON(data []byte) error {
	items, err := decodeProductMessageList(data, func() *ConversationUIMessage { return &ConversationUIMessage{} })
	result.items = items
	return err
}

func (s *ConversationService) CreateUISession(ctx context.Context, request *ConversationUISessionRequest) (*ConversationSession, error) {
	result := &ConversationSession{}
	err := s.do(ctx, http.MethodPost, "/sessions", request, result)
	return result, err
}

func (s *ConversationService) UpdateUISession(ctx context.Context, sessionID int64, request *ConversationUISessionRequest) (*ConversationSession, error) {
	result := &ConversationSession{}
	err := s.do(ctx, http.MethodPut, "/sessions"+"/"+strconv.FormatInt(sessionID, 10), request, result)
	return result, err
}

func (s *ConversationService) GetUISession(ctx context.Context, sessionID int64) (*ConversationSession, error) {
	result := &ConversationSession{}
	err := s.do(ctx, http.MethodGet, "/sessions/"+strconv.FormatInt(sessionID, 10), nil, result)
	return result, err
}

func (s *ConversationService) DeleteUISession(ctx context.Context, sessionID int64) (*ConversationMutation, error) {
	result := &ConversationMutation{}
	err := s.do(ctx, http.MethodDelete, "/sessions/"+strconv.FormatInt(sessionID, 10)+"", nil, result)
	return result, err
}

func (s *ConversationService) PinUISession(ctx context.Context, sessionID int64) (*ConversationMutation, error) {
	result := &ConversationMutation{}
	err := s.do(ctx, http.MethodPut, "/sessions/"+strconv.FormatInt(sessionID, 10)+"/pin", nil, result)
	return result, err
}

func (s *ConversationService) UnpinUISession(ctx context.Context, sessionID int64) (*ConversationMutation, error) {
	result := &ConversationMutation{}
	err := s.do(ctx, http.MethodDelete, "/sessions/"+strconv.FormatInt(sessionID, 10)+"/pin", nil, result)
	return result, err
}

func (s *ConversationService) ListUISessions(ctx context.Context, request *ConversationUISessionListRequest) (*ConversationSessionList, error) {
	result := &ConversationSessionList{}
	err := s.do(ctx, http.MethodPost, "/sessions/list", request, result)
	return result, err
}

func (s *ConversationService) ListPinnedUISessions(ctx context.Context, request *ConversationUISessionListRequest) (*ConversationSessionList, error) {
	result := &ConversationSessionList{}
	err := s.do(ctx, http.MethodPost, "/sessions/pinned/list", request, result)
	return result, err
}

func (s *ConversationService) ListUnpinnedUISessions(ctx context.Context, request *ConversationUISessionListRequest) (*ConversationSessionList, error) {
	result := &ConversationSessionList{}
	err := s.do(ctx, http.MethodPost, "/sessions/unpinned/list", request, result)
	return result, err
}

func (s *ConversationService) ListUIMessages(ctx context.Context, request *ConversationUIMessageListRequest) ([]*ConversationUIMessage, error) {
	result := &conversationUIMessageList{}
	err := s.do(ctx, http.MethodPost, "/messages/session/list", request, result)
	return result.items, err
}

func (s *ConversationService) GetUIMessage(ctx context.Context, messageID int64) (*ConversationUIMessage, error) {
	result := &ConversationUIMessage{}
	err := s.do(ctx, http.MethodGet, "/messages/"+strconv.FormatInt(messageID, 10)+"", nil, result)
	return result, err
}

func (s *ConversationService) LikeUIMessage(ctx context.Context, messageID int64) (*ConversationUIMessage, error) {
	result := &ConversationUIMessage{}
	err := s.do(ctx, http.MethodPut, "/messages/"+strconv.FormatInt(messageID, 10)+"/like", nil, result)
	return result, err
}

func (s *ConversationService) DislikeUIMessage(ctx context.Context, messageID int64) (*ConversationUIMessage, error) {
	result := &ConversationUIMessage{}
	err := s.do(ctx, http.MethodPut, "/messages/"+strconv.FormatInt(messageID, 10)+"/dislike", nil, result)
	return result, err
}

func (s *ConversationService) LatestUIMessageID(ctx context.Context, sessionID int64) (*ConversationLatestMessageID, error) {
	result := &ConversationLatestMessageID{}
	err := s.do(ctx, http.MethodGet, "/messages/sessions/"+strconv.FormatInt(sessionID, 10)+"/latest", nil, result)
	return result, err
}

func (s *ConversationService) CreateSession(ctx context.Context, request *ConversationWorkspaceSessionRequest) (*ConversationSession, error) {
	prefix, err := s.workspacePath()
	if err != nil {
		return nil, err
	}
	result := &ConversationSession{}
	err = s.do(ctx, http.MethodPost, prefix+"/sessions", request, result)
	return result, err
}

func (s *ConversationService) ListSessions(ctx context.Context, options ...QueryOption) (*ConversationSessionList, error) {
	prefix, err := s.workspacePath()
	if err != nil {
		return nil, err
	}
	result := &ConversationSessionList{}
	err = s.do(ctx, http.MethodGet, withQuery(prefix+"/sessions", options), nil, result)
	return result, err
}

func (s *ConversationService) GetSession(ctx context.Context, sessionID int64) (*ConversationSession, error) {
	prefix, err := s.workspacePath()
	if err != nil {
		return nil, err
	}
	result := &ConversationSession{}
	err = s.do(ctx, http.MethodGet, prefix+"/sessions/"+strconv.FormatInt(sessionID, 10)+"", nil, result)
	return result, err
}

func (s *ConversationService) UpdateSession(ctx context.Context, sessionID int64, request *ConversationWorkspaceSessionRequest) (*ConversationSession, error) {
	prefix, err := s.workspacePath()
	if err != nil {
		return nil, err
	}
	result := &ConversationSession{}
	err = s.do(ctx, http.MethodPut, prefix+"/sessions/"+strconv.FormatInt(sessionID, 10)+"", request, result)
	return result, err
}

func (s *ConversationService) DeleteSession(ctx context.Context, sessionID int64) error {
	prefix, err := s.workspacePath()
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodDelete, prefix+"/sessions/"+strconv.FormatInt(sessionID, 10)+"", nil, nil)
}

func (s *ConversationService) CreateMessage(ctx context.Context, sessionID int64, request *ConversationWorkspaceMessageRequest) (*ConversationWorkspaceMessage, error) {
	prefix, err := s.workspacePath()
	if err != nil {
		return nil, err
	}
	result := &ConversationWorkspaceMessage{}
	err = s.do(ctx, http.MethodPost, prefix+"/sessions/"+strconv.FormatInt(sessionID, 10)+"/messages", request, result)
	return result, err
}

func (s *ConversationService) ListMessages(ctx context.Context, sessionID int64, options ...QueryOption) (*ConversationWorkspaceMessageList, error) {
	prefix, err := s.workspacePath()
	if err != nil {
		return nil, err
	}
	result := &ConversationWorkspaceMessageList{}
	err = s.do(ctx, http.MethodGet, withQuery(prefix+"/sessions/"+strconv.FormatInt(sessionID, 10)+"/messages", options), nil, result)
	return result, err
}

func (s *ConversationService) GetMessage(ctx context.Context, messageID int64) (*ConversationWorkspaceMessage, error) {
	prefix, err := s.workspacePath()
	if err != nil {
		return nil, err
	}
	result := &ConversationWorkspaceMessage{}
	err = s.do(ctx, http.MethodGet, prefix+"/messages/"+strconv.FormatInt(messageID, 10)+"", nil, result)
	return result, err
}

func (s *ConversationService) LatestMessageID(ctx context.Context, sessionID int64) (*ConversationLatestMessageID, error) {
	prefix, err := s.workspacePath()
	if err != nil {
		return nil, err
	}
	result := &ConversationLatestMessageID{}
	err = s.do(ctx, http.MethodGet, prefix+"/sessions/"+strconv.FormatInt(sessionID, 10)+"/messages/latest", nil, result)
	return result, err
}

func (s *ConversationService) CreateTag(ctx context.Context, request *ConversationTagRequest) (*ConversationTag, error) {
	prefix, err := s.workspacePath()
	if err != nil {
		return nil, err
	}
	result := &ConversationTag{}
	err = s.do(ctx, http.MethodPost, prefix+"/tags", request, result)
	return result, err
}

func (s *ConversationService) ListTags(ctx context.Context, options ...QueryOption) (*ConversationTagList, error) {
	prefix, err := s.workspacePath()
	if err != nil {
		return nil, err
	}
	result := &ConversationTagList{}
	err = s.do(ctx, http.MethodGet, withQuery(prefix+"/tags", options), nil, result)
	return result, err
}

func (s *ConversationService) DeleteTag(ctx context.Context, source string, name string) error {
	prefix, err := s.workspacePath()
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodDelete, prefix+"/tags/"+url.PathEscape(source)+"/"+url.PathEscape(name)+"", nil, nil)
}

func (s *ConversationService) ListSessionTags(ctx context.Context, sessionID int64) (*ConversationTagList, error) {
	prefix, err := s.workspacePath()
	if err != nil {
		return nil, err
	}
	result := &ConversationTagList{}
	err = s.do(ctx, http.MethodGet, prefix+"/sessions/"+strconv.FormatInt(sessionID, 10)+"/tags", nil, result)
	return result, err
}

func (s *ConversationService) AddSessionTag(ctx context.Context, sessionID int64, request *ConversationTagRequest) error {
	prefix, err := s.workspacePath()
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodPost, prefix+"/sessions/"+strconv.FormatInt(sessionID, 10)+"/tags", request, nil)
}

func (s *ConversationService) RemoveSessionTag(ctx context.Context, sessionID int64, source string, name string) error {
	prefix, err := s.workspacePath()
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodDelete, prefix+"/sessions/"+strconv.FormatInt(sessionID, 10)+"/tags"+"/"+url.PathEscape(source)+"/"+url.PathEscape(name), nil, nil)
}

func (s *ConversationService) ListMessageTags(ctx context.Context, messageID int64) (*ConversationTagList, error) {
	prefix, err := s.workspacePath()
	if err != nil {
		return nil, err
	}
	result := &ConversationTagList{}
	err = s.do(ctx, http.MethodGet, prefix+"/messages/"+strconv.FormatInt(messageID, 10)+"/tags", nil, result)
	return result, err
}

func (s *ConversationService) AddMessageTag(ctx context.Context, messageID int64, request *ConversationTagRequest) error {
	prefix, err := s.workspacePath()
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodPost, prefix+"/messages/"+strconv.FormatInt(messageID, 10)+"/tags", request, nil)
}

func (s *ConversationService) RemoveMessageTag(ctx context.Context, messageID int64, source string, name string) error {
	prefix, err := s.workspacePath()
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodDelete, prefix+"/messages/"+strconv.FormatInt(messageID, 10)+"/tags"+"/"+url.PathEscape(source)+"/"+url.PathEscape(name), nil, nil)
}

func (s *ConversationService) ModifyResponse(ctx context.Context, sessionID, messageID int64, content string) error {
	prefix, err := s.workspacePath()
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodPut, prefix+"/sessions/"+strconv.FormatInt(sessionID, 10)+"/messages/"+strconv.FormatInt(messageID, 10)+"/modify-response", map[string]string{"modified_response": content}, nil)
}

func (s *ConversationService) AppendModifiedResponse(ctx context.Context, sessionID, messageID int64, content string) error {
	prefix, err := s.workspacePath()
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodPost, prefix+"/sessions/"+strconv.FormatInt(sessionID, 10)+"/messages/"+strconv.FormatInt(messageID, 10)+"/append-modified-response", map[string]string{"append_content": content}, nil)
}
