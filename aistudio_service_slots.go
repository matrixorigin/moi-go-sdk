package sdk

import (
	"context"
	"net/http"
	"strconv"
)

// AIStudioService reads account-level model policy and manages Genesis AI
// Studio service slots. Only administration requires taas:admin scope.
type AIStudioService struct{ client *Client }

// PutAIStudioServiceSlotRequest is the service slot write body.
type PutAIStudioServiceSlotRequest struct {
	SlotID         string   `json:"slot_id,omitempty"`
	DefaultModelID string   `json:"default_model_id"`
	ModelIDs       []string `json:"model_ids"`
	ExtraJSON      string   `json:"extra_json"`
}

// AIStudio returns the account-level AI Studio administration facade.
func (c *Client) AIStudio() *AIStudioService { return &AIStudioService{client: c} }

// ChatModelPolicy reads the product's agent_chat allowlist and default model.
// It requires a product identity, without a workspace or administrator scope.
func (s *AIStudioService) ChatModelPolicy(ctx context.Context) (*ProductData, error) {
	return s.get(ctx, "/aistudio/model-policy")
}

// ServiceSlots lists the configured service slots.
func (s *AIStudioService) ServiceSlots(ctx context.Context) (*ProductData, error) {
	return s.get(ctx, "/admin/aistudio/service-slots")
}

// ServiceSlotCandidates lists the models that can be assigned to service slots.
func (s *AIStudioService) ServiceSlotCandidates(ctx context.Context) (*ProductData, error) {
	return s.get(ctx, "/admin/aistudio/service-slots/candidates")
}

// ServiceSlotLogs lists service slot operation logs.
func (s *AIStudioService) ServiceSlotLogs(ctx context.Context, opts ...QueryOption) (*ProductData, error) {
	return s.get(ctx, withQuery("/admin/aistudio/service-slots/operation-logs", opts))
}

// PutServiceSlot replaces the model assignment of one service slot.
func (s *AIStudioService) PutServiceSlot(ctx context.Context, slotID string, request *PutAIStudioServiceSlotRequest) (*ProductData, error) {
	slotPath, err := pathID(slotID)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "service slot request is required"}
	}
	body := *request
	if body.ModelIDs == nil {
		body.ModelIDs = []string{}
	}
	result := &ProductData{}
	err = s.client.do(ctx, http.MethodPut, "/admin/aistudio/service-slots/"+slotPath, "", &body, result)
	return result, err
}

func (s *AIStudioService) get(ctx context.Context, path string) (*ProductData, error) {
	result := &ProductData{}
	err := s.client.do(ctx, http.MethodGet, path, "", nil, result)
	return result, err
}

// WithServiceSlotLogSlotID filters service slot operation logs by slot.
func WithServiceSlotLogSlotID(slotID string) QueryOption {
	return WithQueryParam("slot_id", slotID)
}

// WithServiceSlotLogPageSize limits the service slot operation log page size.
func WithServiceSlotLogPageSize(pageSize int) QueryOption {
	return WithQueryParam("page_size", strconv.Itoa(pageSize))
}
