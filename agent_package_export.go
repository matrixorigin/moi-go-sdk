package sdk

import (
	"context"
	"net/http"
)

// ExportAgentPackage streams one published agent version as a .moiagent archive.
func (s *AgentService) ExportAgentPackage(ctx context.Context, agentID, version string) (*StreamResponse, error) {
	workspace, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	agent, err := pathID(agentID)
	if err != nil {
		return nil, err
	}
	revision, err := pathID(version)
	if err != nil {
		return nil, err
	}
	return s.doDownload(ctx, http.MethodGet, "/workspaces/"+workspace+"/agent-packages/"+agent+"/versions/"+revision+"/export", nil)
}
