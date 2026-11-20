package sdk

import (
	"context"
	"net/http"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

// AgentRunner is the public Runner contract generated from Product proto.
type AgentRunner = product.AgentRunner

// RunnerService manages workspace runners through the product API.
type RunnerService struct{ workspaceService }

// Create registers a custom runner and returns its one-time connect command.
func (s *RunnerService) Create(ctx context.Context, name string) (*AgentRunner, error) {
	var out AgentRunner
	err := s.do(ctx, http.MethodPost, "/agent-runners/custom", &product.RunnerCreateRequest{Name: name}, &out)
	return &out, err
}

// List returns visible runners without issuing new connect commands.
func (s *RunnerService) List(ctx context.Context) ([]*AgentRunner, error) {
	var out runnerList
	err := s.do(ctx, http.MethodGet, "/agent-runners", nil, &out)
	return out, err
}

// Get returns one runner without issuing a new connect command.
func (s *RunnerService) Get(ctx context.Context, id string) (*AgentRunner, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out AgentRunner
	err = s.do(ctx, http.MethodGet, "/agent-runners/"+path, nil, &out)
	return &out, err
}

// Refresh rotates the edge token and returns the new connect command.
func (s *RunnerService) Refresh(ctx context.Context, id string) (*AgentRunner, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out AgentRunner
	err = s.do(ctx, http.MethodPost, "/agent-runners/"+path+"/refresh-token", nil, &out)
	return &out, err
}

// Delete revokes the runner token and removes the runner.
func (s *RunnerService) Delete(ctx context.Context, id string) error {
	path, err := pathID(id)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodDelete, "/agent-runners/"+path, nil, nil)
}

type runnerList []*AgentRunner

func (items *runnerList) UnmarshalJSON(data []byte) error {
	decoded, err := decodeProductMessageList(data, func() *AgentRunner { return &AgentRunner{} })
	if err != nil {
		return err
	}
	*items = decoded
	return nil
}
