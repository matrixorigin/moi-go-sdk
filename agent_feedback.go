package sdk

import (
	"context"
	"encoding/json"
	"net/http"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/encoding/protojson"
)

type FeedbackTicketCreateRequest = product.FeedbackTicketCreateRequest
type FeedbackTicketMessageInput = product.FeedbackTicketMessageInput
type FeedbackTicketAdminUpdateRequest = product.FeedbackTicketAdminUpdateRequest

func (s *AgentService) CreateFeedbackTicket(ctx context.Context, request *FeedbackTicketCreateRequest) (*ProductData, error) {
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "feedback ticket request is required"}
	}
	body, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(request)
	if err != nil {
		return nil, err
	}
	result := &ProductData{}
	err = s.do(ctx, http.MethodPost, "/feedback-tickets", json.RawMessage(body), result)
	return result, err
}
func (s *AgentService) MyFeedbackTickets(ctx context.Context, options ...QueryOption) (*ProductData, error) {
	result := &ProductData{}
	err := s.do(ctx, http.MethodGet, withQuery("/feedback-tickets/mine", options), nil, result)
	return result, err
}

// AdminFeedbackTicket reads the platform administrator ticket projection.
func (s *AgentService) AdminFeedbackTicket(ctx context.Context, ticketID string) (*ProductData, error) {
	ticketPath, err := pathID(ticketID)
	if err != nil {
		return nil, err
	}
	result := &ProductData{}
	err = s.do(ctx, http.MethodGet, "/admin/feedback-tickets/"+ticketPath, nil, result)
	return result, err
}

// BuildFeedbackGithubIssueDraft previews an administrator GitHub issue draft.
func (s *AgentService) BuildFeedbackGithubIssueDraft(ctx context.Context, ticketID, kind string) (*ProductData, error) {
	ticketPath, err := pathID(ticketID)
	if err != nil {
		return nil, err
	}
	result := &ProductData{}
	err = s.do(ctx, http.MethodPost, "/admin/feedback-tickets/"+ticketPath+"/github-issue-draft", struct {
		Kind string `json:"kind"`
	}{Kind: kind}, result)
	return result, err
}

// CreateFeedbackGithubIssue requests issue creation through the product API.
func (s *AgentService) CreateFeedbackGithubIssue(ctx context.Context, ticketID, kind, title, body string, retry bool) (*ProductData, error) {
	ticketPath, err := pathID(ticketID)
	if err != nil {
		return nil, err
	}
	result := &ProductData{}
	err = s.do(ctx, http.MethodPost, "/admin/feedback-tickets/"+ticketPath+"/github-issue", struct {
		Kind  string `json:"kind"`
		Title string `json:"title"`
		Body  string `json:"body"`
		Retry bool   `json:"retry"`
	}{Kind: kind, Title: title, Body: body, Retry: retry}, result)
	return result, err
}

// ApproveFeedbackReward records the amount-free approval fact. The product API
// owns authentication and platform administrator admission.
func (s *AgentService) ApproveFeedbackReward(ctx context.Context, ticketID string, request *FeedbackTicketAdminUpdateRequest) (*ProductData, error) {
	ticketPath, err := pathID(ticketID)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "feedback ticket request is required"}
	}
	result := &ProductData{}
	err = s.do(ctx, http.MethodPost, "/admin/feedback-tickets/"+ticketPath+"/reward-approval", request, result)
	return result, err
}

func (s *AgentService) AdminFeedbackTickets(ctx context.Context, opts ...QueryOption) (*ProductData, error) {
	out := &ProductData{}
	err := s.do(ctx, http.MethodGet, withQuery("/admin/feedback-tickets", opts), nil, out)
	return out, err
}
func (s *AgentService) UpdateAdminFeedbackTicket(ctx context.Context, id string, request *FeedbackTicketAdminUpdateRequest) (*ProductData, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	out := &ProductData{}
	err = s.do(ctx, http.MethodPatch, "/admin/feedback-tickets/"+path, request, out)
	return out, err
}
