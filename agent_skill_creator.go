package sdk

import (
	"context"
	"net/http"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

type ConfirmSkillCandidateRequest = product.ConfirmSkillCandidateRequest
type ConfirmSkillCandidateResult = product.ConfirmSkillCandidateResult
type SkillCandidateConfirmationResult = product.SkillCandidateConfirmationResult

func (s *AgentService) GetSkillCandidateConfirmation(ctx context.Context, taskID, callID, sourceDigest string) (*SkillCandidateConfirmationResult, error) {
	taskPath, err := pathID(taskID)
	if err != nil {
		return nil, err
	}
	path, err := agentResourcePath(s.workspaceID, "agent-runtime-tasks", "/"+taskPath+"/skills/confirmation")
	if err != nil {
		return nil, err
	}
	result := &SkillCandidateConfirmationResult{}
	err = s.do(ctx, http.MethodGet, withQuery(path, []QueryOption{WithQueryParam("call_id", callID), WithQueryParam("source_digest", sourceDigest)}), nil, result)
	return result, err
}

// ConfirmSkillCandidate saves a task's reviewed skill and optionally binds it to the current agent.
func (s *AgentService) ConfirmSkillCandidate(ctx context.Context, taskID string, request *ConfirmSkillCandidateRequest) (*ConfirmSkillCandidateResult, error) {
	taskPath, err := pathID(taskID)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "skill candidate request is required"}
	}
	path, err := agentResourcePath(s.workspaceID, "agent-runtime-tasks", "/"+taskPath+"/skills/confirm")
	if err != nil {
		return nil, err
	}
	result := &ConfirmSkillCandidateResult{}
	err = s.do(ctx, http.MethodPost, path, request, result)
	return result, err
}
