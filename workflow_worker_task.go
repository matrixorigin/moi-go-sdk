package sdk

import (
	"context"
	"net/http"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/encoding/protojson"
)

type WorkflowWorkerTaskCreateRequest = product.WorkflowWorkerTaskCreateRequest
type WorkflowWorkerTask = product.WorkflowWorkerTask

type workflowWorkerTaskResult struct{ task *WorkflowWorkerTask }

func (result *workflowWorkerTaskResult) UnmarshalJSON(data []byte) error {
	envelope := &product.WorkflowWorkerTaskEnvelope{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(data, envelope); err != nil {
		return &Error{Code: "ErrResponseInvalid", Message: "worker task response is invalid", cause: err}
	}
	if envelope.Task == nil {
		return &Error{Code: "ErrResponseInvalid", Message: "worker task response is missing task"}
	}
	result.task = envelope.Task
	return nil
}

// CreateWorkerTask starts the selected published workflow version through the
// workspace worker-task trigger. data and vars remain JSON-encoded strings.
func (s *WorkflowService) CreateWorkerTask(ctx context.Context, request *WorkflowWorkerTaskCreateRequest) (*WorkflowWorkerTask, error) {
	if request == nil {
		return nil, ErrSpecRequired
	}
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	body := &product.WorkflowWorkerTaskCreateEnvelope{Task: request}
	result := &workflowWorkerTaskResult{}
	if err := s.do(ctx, http.MethodPost, "/workspaces/"+workspace+"/tasks", body, result); err != nil {
		return nil, err
	}
	return result.task, nil
}
