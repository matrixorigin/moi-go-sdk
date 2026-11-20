package sdk

import (
	"context"
	"net/http"
	"strconv"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

type AgentArtifact = product.AgentArtifact
type AgentArtifactAgentSource = product.AgentArtifactAgentSource
type AgentArtifactCreatorSource = product.AgentArtifactCreatorSource
type AgentArtifactListResult = product.AgentArtifactListResult
type AgentArtifactRef = product.AgentArtifactRef
type AgentArtifactFailure = product.AgentArtifactFailure
type AgentArtifactBatchRequest = product.AgentArtifactBatchRequest
type AgentArtifactDeleteResult = product.AgentArtifactDeleteResult
type AgentArtifactResolveItem = product.AgentArtifactResolveItem
type AgentArtifactResolveResult = product.AgentArtifactResolveResult
type AgentArtifactBackfillRequest = product.AgentArtifactBackfillRequest
type AgentArtifactBackfillResult = product.AgentArtifactBackfillResult

// Artifacts 查询工作区可见成果及其来源筛选信息。
func (s *AgentService) Artifacts(ctx context.Context, opts ...QueryOption) (*AgentArtifactListResult, error) {
	path, err := agentResourcePath(s.workspaceID, "agent-artifacts", "")
	if err != nil {
		return nil, err
	}
	out := &AgentArtifactListResult{}
	err = s.do(ctx, http.MethodGet, withQuery(path, opts), nil, out)
	return out, err
}

// Artifact 按成果 ID 与明确版本读取记录。
func (s *AgentService) Artifact(ctx context.Context, artifactID string, version int64) (*AgentArtifact, error) {
	id, err := pathID(artifactID)
	if err != nil {
		return nil, err
	}
	if version <= 0 {
		return nil, ErrVersionRequired
	}
	path, err := agentResourcePath(s.workspaceID, "agent-artifacts", "/"+id+"/versions/"+strconv.FormatInt(version, 10))
	if err != nil {
		return nil, err
	}
	out := &AgentArtifact{}
	err = s.do(ctx, http.MethodGet, path, nil, out)
	return out, err
}

// DeleteArtifacts 保留批量删除的成功项及逐项失败原因。
func (s *AgentService) DeleteArtifacts(ctx context.Context, request *AgentArtifactBatchRequest) (*AgentArtifactDeleteResult, error) {
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "artifact request is required"}
	}
	path, err := agentResourcePath(s.workspaceID, "agent-artifacts", "/batch-delete")
	if err != nil {
		return nil, err
	}
	out := &AgentArtifactDeleteResult{}
	err = s.do(ctx, http.MethodPost, path, request, out)
	return out, err
}

// ResolveArtifacts 将明确的成果版本解析为可访问的文件引用。
func (s *AgentService) ResolveArtifacts(ctx context.Context, request *AgentArtifactBatchRequest) (*AgentArtifactResolveResult, error) {
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "artifact request is required"}
	}
	path, err := agentResourcePath(s.workspaceID, "agent-artifacts", "/resolve")
	if err != nil {
		return nil, err
	}
	out := &AgentArtifactResolveResult{}
	err = s.do(ctx, http.MethodPost, path, request, out)
	return out, err
}

// BackfillArtifacts 显式提交维护操作，不自动扫描或重试。
func (s *AgentService) BackfillArtifacts(ctx context.Context, request *AgentArtifactBackfillRequest) (*AgentArtifactBackfillResult, error) {
	if request == nil {
		return nil, &Error{Code: "ErrRequestRequired", Message: "artifact request is required"}
	}
	path, err := agentResourcePath(s.workspaceID, "agent-artifacts", "/backfill")
	if err != nil {
		return nil, err
	}
	out := &AgentArtifactBackfillResult{}
	err = s.do(ctx, http.MethodPost, path, request, out)
	return out, err
}
