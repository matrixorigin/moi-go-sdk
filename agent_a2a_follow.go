package sdk

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"strconv"
	"strings"
	"time"
)

const (
	// defaultA2AWaitTimeout bounds how long a folded message/send call waits
	// for the task to reach a terminal state before returning ErrTimeout.
	defaultA2AWaitTimeout = 10 * time.Minute
	// a2aFollowPollInterval is the tasks/get polling interval used when the
	// resubscribe stream is unavailable or keeps disconnecting.
	a2aFollowPollInterval = 2 * time.Second
	// a2aFollowMaxReconnects bounds SSE reconnect attempts before the follow
	// degrades to tasks/get polling.
	a2aFollowMaxReconnects = 5
	// a2aFollowInitialBackoff is the first reconnect delay; it doubles per attempt.
	a2aFollowInitialBackoff = 200 * time.Millisecond
)

// A2A task states. The terminal set matches the product wire contract:
// completed/failed/canceled/rejected plus the interrupted input-required and
// auth-required states, which are handed back to the caller for handling.
const (
	a2aTaskStateSubmitted     = "submitted"
	a2aTaskStateWorking       = "working"
	a2aTaskStateCompleted     = "completed"
	a2aTaskStateFailed        = "failed"
	a2aTaskStateCanceled      = "canceled"
	a2aTaskStateRejected      = "rejected"
	a2aTaskStateInputRequired = "input-required"
	a2aTaskStateAuthRequired  = "auth-required"
)

// errA2AFollowDegraded marks follow attempts that exhausted the SSE path and
// must continue through tasks/get polling.
var errA2AFollowDegraded = errors.New("a2a task follow degraded to polling")

// foldA2ASendResult folds a non-terminal message/send task result into its
// terminal state: the SDK follows the task through tasks/resubscribe (with
// tasks/get polling as the degraded path) and returns the persisted terminal
// task projection. Message-kind results (feedback ack, run-control ack,
// provider interaction) and terminal tasks pass through unchanged.
func (s *AgentService) foldA2ASendResult(ctx context.Context, req AgentA2ARequest, result *AgentA2AResult) (*AgentA2AResult, error) {
	if req.Method != A2AMethodSend || req.NoWait {
		return result, nil
	}
	payload, err := AgentA2APayload(result)
	if err != nil {
		return result, nil
	}
	kind, _ := payload["kind"].(string)
	if kind != "task" {
		return result, nil
	}
	status, _ := payload["status"].(map[string]any)
	state, _ := status["state"].(string)
	if state != a2aTaskStateSubmitted && state != a2aTaskStateWorking {
		return result, nil
	}
	taskID, _ := payload["id"].(string)
	if taskID == "" {
		return result, nil
	}
	return s.followA2ATask(ctx, req.AgentSelector, taskID, a2aMetadataCursor(payload))
}

// followA2ATask waits for one admitted task to reach a terminal state and
// returns its persisted terminal projection read through tasks/get.
func (s *AgentService) followA2ATask(ctx context.Context, selector AgentSelector, taskID string, cursor int64) (*AgentA2AResult, error) {
	waitTimeout := s.client.a2aWaitTimeout
	if waitTimeout <= 0 {
		waitTimeout = defaultA2AWaitTimeout
	}
	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	if err := s.watchA2ATaskTerminal(waitCtx, selector, taskID, &cursor); err != nil {
		if !errors.Is(err, errA2AFollowDegraded) {
			return nil, a2aFollowError(ctx, taskID, err)
		}
		if err := s.pollA2ATaskTerminal(waitCtx, selector, taskID); err != nil {
			return nil, a2aFollowError(ctx, taskID, err)
		}
	}
	return s.readA2ATerminalTask(waitCtx, ctx, selector, taskID)
}

// watchA2ATaskTerminal streams task events through tasks/resubscribe until a
// terminal event arrives. It reconnects with the persisted event cursor after
// mid-stream drops and degrades to polling once the bounded reconnect budget
// is exhausted or the stream cannot be opened.
func (s *AgentService) watchA2ATaskTerminal(ctx context.Context, selector AgentSelector, taskID string, cursor *int64) error {
	backoff := a2aFollowInitialBackoff
	for attempt := 0; ; attempt++ {
		terminal, rpcErr, err := s.streamA2ATaskTerminal(ctx, selector, taskID, cursor)
		if rpcErr != nil {
			return rpcErr
		}
		if err == nil || terminal {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt == 0 && errors.Is(err, errA2AStreamOpen) {
			return errA2AFollowDegraded
		}
		if attempt >= a2aFollowMaxReconnects {
			return errA2AFollowDegraded
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		backoff *= 2
	}
}

// errA2AStreamOpen marks a resubscribe open failure, which degrades to
// polling without spending reconnect attempts.
var errA2AStreamOpen = errors.New("a2a task resubscribe open failed")

// streamA2ATaskTerminal consumes one resubscribe stream until a terminal
// event, returning terminal=true once one was observed. A JSON-RPC error
// frame is an explicit server failure and returns as rpcErr without spending
// reconnect attempts.
func (s *AgentService) streamA2ATaskTerminal(ctx context.Context, selector AgentSelector, taskID string, cursor *int64) (bool, *Error, error) {
	params, err := json.Marshal(map[string]any{"id": taskID, "afterSeq": *cursor})
	if err != nil {
		return false, nil, err
	}
	stream, err := s.A2AStream(ctx, AgentA2ARequest{
		AgentSelector: selector,
		A2ARequest: A2ARequest{
			JSONRPC: "2.0",
			Method:  A2AMethodTasksResubscribe,
			Params:  params,
		},
	})
	if err != nil {
		return false, nil, fmt.Errorf("%w: %w", errA2AStreamOpen, err)
	}
	defer stream.Body.Close()
	frames := &a2aSSEFrameReader{reader: bufio.NewReader(stream.Body)}
	for {
		frame, err := frames.next()
		if err != nil {
			if ctx.Err() != nil {
				return false, nil, ctx.Err()
			}
			return false, nil, err
		}
		if frame.seq > *cursor {
			*cursor = frame.seq
		}
		if frame.rpcError != nil {
			return false, frame.rpcError, nil
		}
		if frame.terminal {
			return true, nil, nil
		}
	}
}

// pollA2ATaskTerminal is the degraded follow path: it reads the persisted
// task projection on a fixed interval until the state turns terminal.
func (s *AgentService) pollA2ATaskTerminal(ctx context.Context, selector AgentSelector, taskID string) error {
	ticker := time.NewTicker(a2aFollowPollInterval)
	defer ticker.Stop()
	for {
		terminal, err := s.a2aTaskIsTerminal(ctx, selector, taskID)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if terminal {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// readA2ATerminalTask returns the persisted terminal task projection after
// the follow observed a terminal event.
func (s *AgentService) readA2ATerminalTask(waitCtx, parent context.Context, selector AgentSelector, taskID string) (*AgentA2AResult, error) {
	ticker := time.NewTicker(a2aFollowPollInterval)
	defer ticker.Stop()
	for {
		params, err := json.Marshal(map[string]any{"id": taskID})
		if err != nil {
			return nil, err
		}
		result, err := s.A2A(waitCtx, AgentA2ARequest{
			AgentSelector: selector,
			A2ARequest: A2ARequest{
				JSONRPC: "2.0",
				Method:  A2AMethodTasksGet,
				Params:  params,
			},
		})
		if err == nil {
			if payload, payloadErr := AgentA2APayload(result); payloadErr == nil && a2aPayloadTerminal(payload) {
				return result, nil
			}
		} else if waitCtx.Err() != nil {
			return nil, a2aFollowError(parent, taskID, waitCtx.Err())
		}
		select {
		case <-waitCtx.Done():
			return nil, a2aFollowError(parent, taskID, waitCtx.Err())
		case <-ticker.C:
		}
	}
}

// a2aTaskIsTerminal reads the persisted projection once and reports whether
// the task state belongs to the terminal set.
func (s *AgentService) a2aTaskIsTerminal(ctx context.Context, selector AgentSelector, taskID string) (bool, error) {
	params, err := json.Marshal(map[string]any{"id": taskID})
	if err != nil {
		return false, err
	}
	result, err := s.A2A(ctx, AgentA2ARequest{
		AgentSelector: selector,
		A2ARequest: A2ARequest{
			JSONRPC: "2.0",
			Method:  A2AMethodTasksGet,
			Params:  params,
		},
	})
	if err != nil {
		return false, err
	}
	payload, err := AgentA2APayload(result)
	if err != nil {
		return false, err
	}
	return a2aPayloadTerminal(payload), nil
}

// a2aFollowError normalizes follow failures: the caller's own context wins,
// wait-budget exhaustion becomes ErrTimeout, and every error carries the task
// id so the caller can resume the task instead of treating it as lost.
func a2aFollowError(parent context.Context, taskID string, err error) error {
	if parent.Err() != nil {
		return parent.Err()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{
			Code:    "ErrTimeout",
			Message: fmt.Sprintf("A2A task %s did not reach a terminal state before the wait timeout", taskID),
			Details: map[string]any{"task_id": taskID},
		}
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	var sdkErr *Error
	if errors.As(err, &sdkErr) {
		clone := *sdkErr
		details := make(ErrorDetails, len(sdkErr.Details)+1)
		maps.Copy(details, sdkErr.Details)
		details["task_id"] = taskID
		clone.Details = details
		return &clone
	}
	return &Error{
		Code:    "ErrTransportUnavailable",
		Message: "A2A task follow failed",
		Details: map[string]any{"task_id": taskID},
		cause:   err,
	}
}

// a2aPayloadTerminal reports whether one A2A task payload carries a terminal
// state: completed/failed/canceled/rejected plus the interrupted
// input-required/auth-required states.
func a2aPayloadTerminal(payload map[string]any) bool {
	kind, _ := payload["kind"].(string)
	if kind != "task" {
		return false
	}
	status, _ := payload["status"].(map[string]any)
	state, _ := status["state"].(string)
	switch state {
	case a2aTaskStateCompleted, a2aTaskStateFailed, a2aTaskStateCanceled,
		a2aTaskStateRejected, a2aTaskStateInputRequired, a2aTaskStateAuthRequired:
		return true
	default:
		return false
	}
}

// a2aMetadataCursor extracts the persisted event cursor from task metadata.
func a2aMetadataCursor(payload map[string]any) int64 {
	metadata, _ := payload["metadata"].(map[string]any)
	switch value := metadata["eventCursor"].(type) {
	case float64:
		return int64(value)
	case json.Number:
		cursor, _ := value.Int64()
		return cursor
	default:
		return 0
	}
}

// a2aSSEFrame is one parsed Server-Sent-Events frame from an A2A stream.
type a2aSSEFrame struct {
	seq      int64
	terminal bool
	rpcError *Error
}

// a2aSSEFrameReader parses the product A2A SSE wire format: heartbeat comment
// lines, an optional decimal id line, an event line, and one or more data
// lines carrying a JSON-RPC response terminated by a blank line.
type a2aSSEFrameReader struct {
	reader *bufio.Reader
}

func (r *a2aSSEFrameReader) next() (*a2aSSEFrame, error) {
	var (
		seq     int64
		data    strings.Builder
		hasData bool
	)
	for {
		line, err := r.reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		switch {
		case trimmed == "":
			if !hasData {
				if err != nil {
					return nil, io.EOF
				}
				continue
			}
			return decodeA2ASSEFrame(seq, data.String())
		case strings.HasPrefix(trimmed, ":"):
			// Heartbeat comment; carries no task state.
		default:
			field, value, found := strings.Cut(trimmed, ":")
			if !found {
				return nil, fmt.Errorf("malformed SSE line %q", trimmed)
			}
			value = strings.TrimPrefix(value, " ")
			switch field {
			case "id":
				parsed, parseErr := strconv.ParseInt(value, 10, 64)
				if parseErr != nil {
					return nil, fmt.Errorf("malformed SSE id %q", value)
				}
				seq = parsed
			case "event":
				// Event names carry no task state beyond the data payload.
			case "data":
				if hasData {
					data.WriteByte('\n')
				}
				data.WriteString(value)
				hasData = true
			}
		}
		if err != nil {
			if hasData {
				return decodeA2ASSEFrame(seq, data.String())
			}
			return nil, io.EOF
		}
	}
}

// decodeA2ASSEFrame decodes one JSON-RPC SSE payload. Error frames map to the
// same numeric-code Error contract as non-stream JSON-RPC error responses.
func decodeA2ASSEFrame(seq int64, data string) (*a2aSSEFrame, error) {
	frame := &a2aSSEFrame{seq: seq}
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int             `json:"code"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return nil, fmt.Errorf("malformed SSE data frame: %w", err)
	}
	if envelope.Error != nil {
		frame.rpcError = &Error{
			Code:    strconv.Itoa(envelope.Error.Code),
			Message: envelope.Error.Message,
			Data:    envelope.Error.Data,
		}
		return frame, nil
	}
	var result map[string]any
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		return nil, fmt.Errorf("malformed SSE result frame: %w", err)
	}
	kind, _ := result["kind"].(string)
	switch kind {
	case "status-update":
		if cursor := a2aMetadataCursor(result); cursor > frame.seq {
			frame.seq = cursor
		}
		final, _ := result["final"].(bool)
		frame.terminal = final
	case "task":
		if cursor := a2aMetadataCursor(result); cursor > frame.seq {
			frame.seq = cursor
		}
		frame.terminal = a2aPayloadTerminal(result)
	}
	return frame, nil
}
