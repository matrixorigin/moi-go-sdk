package sdk

import (
	"context"
	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"net/http"
)

type WorkflowAuthoringScope = product.WorkflowAuthoringScope
type WorkflowCandidateSubmitRequest = product.WorkflowCandidateSubmitRequest
type WorkflowScratchScope = product.WorkflowScratchScope
type WorkflowScratchLease = product.WorkflowScratchLease
type WorkflowScratchObservation = product.WorkflowScratchObservation
type WorkflowScratchResetResult = product.WorkflowScratchResetResult
type WorkflowCandidateTestRequest = product.WorkflowCandidateTestRequest
type WorkflowCandidateTestWorkItem = product.WorkflowCandidateTestWorkItem
type WorkflowCandidateTestReport = product.WorkflowCandidateTestReport

func (s *WorkflowService) SubmitCandidate(ctx context.Context, request *WorkflowCandidateSubmitRequest) (*WorkflowCandidateBundle, error) {
	result := &WorkflowCandidateBundle{}
	err := s.do(ctx, http.MethodPost, "/workflow/v2/workflow-candidates/submit", request, result)
	return result, err
}

func (s *WorkflowService) TestCandidate(ctx context.Context, request *WorkflowCandidateTestRequest) (*WorkflowCandidateTestReport, error) {
	result := &WorkflowCandidateTestReport{}
	err := s.do(ctx, http.MethodPost, "/workflow/v2/workflow-candidates/test", request, result)
	return result, err
}

func (s *WorkflowService) EnsureTestScratch(ctx context.Context, request *WorkflowScratchScope) (*WorkflowScratchLease, error) {
	result := &WorkflowScratchLease{}
	err := s.do(ctx, http.MethodPost, "/workflow/v2/workflow-test-scratch/ensure", request, result)
	return result, err
}

func (s *WorkflowService) ObserveTestScratch(ctx context.Context, request *WorkflowScratchScope) (*WorkflowScratchObservation, error) {
	result := &WorkflowScratchObservation{}
	err := s.do(ctx, http.MethodPost, "/workflow/v2/workflow-test-scratch/observe", request, result)
	return result, err
}
func (s *WorkflowService) ResetTestScratch(ctx context.Context, taskID string) (*WorkflowScratchResetResult, error) {
	result := &WorkflowScratchResetResult{}
	err := s.do(ctx, http.MethodPost, "/workflow/v2/workflow-test-scratch/reset", map[string]string{"task_id": taskID}, result)
	return result, err
}
