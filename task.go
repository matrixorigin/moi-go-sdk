package sdk

import (
	"context"
	"fmt"
)

// GetTask retrieves detailed information about a task by its ID.
//
// This method queries the task information endpoint to get task details
// including status, configuration, and results.
//
// Example:
//
//	resp, err := client.GetTask(ctx, &sdk.TaskInfoRequest{
//		TaskID: 123,
//	})
//	if err != nil {
//		return err
//	}
//	fmt.Printf("Task: %s, Status: %s\n", resp.Name, resp.Status)
func (c *RawClient) GetTask(ctx context.Context, req *TaskInfoRequest, opts ...CallOption) (*TaskInfoResponse, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	if req.TaskID == 0 {
		return nil, fmt.Errorf("task_id is required")
	}

	// Add task_id as query parameter
	opts = append(opts, WithQueryParam("task_id", fmt.Sprintf("%d", req.TaskID)))

	var resp TaskInfoResponse
	if err := c.getJSON(ctx, "/task/get", &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}
