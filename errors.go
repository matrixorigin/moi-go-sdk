// Package sdk provides the MOI product-level SDK.
package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
)

// ErrorDetails preserves the owner's JSON diagnostic object. Numbers use
// json.Number so nested resource IDs retain their full precision.
type ErrorDetails map[string]any

func (details *ErrorDetails) UnmarshalJSON(data []byte) error {
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	// These existing machine-readable metadata fields have string contracts;
	// other diagnostic values may be nested JSON objects, arrays or scalars.
	for _, field := range []string{"reason", "domain", "request_id"} {
		if item, exists := value[field]; exists {
			if _, ok := item.(string); !ok {
				return fmt.Errorf("product error detail %q must be a string", field)
			}
		}
	}
	*details = value
	return nil
}

// Error is a product SDK error returned by the product endpoint or client.
// It is a diagnostic and explicit error-branch value, not a user-story SDK entry point, AI task-selection source, recovery-strategy source, or input-construction source.
type Error struct {
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	Status    int             `json:"status,omitzero"`
	RequestID string          `json:"request_id,omitempty"`
	Details   ErrorDetails    `json:"details,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	cause     error
}

func (e *Error) Unwrap() error { return e.cause }

func productTransportError(ctx context.Context, err error) error {
	if failure := productDeadlineError(ctx, err); failure != nil {
		return failure
	}
	return &Error{Code: "ErrTransportUnavailable", Message: "no response was received from the service", cause: err}
}

func productDeadlineError(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	var networkError net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.As(err, &networkError) && networkError.Timeout() {
		return &Error{Code: "ErrTimeout", Message: "request timeout", cause: err}
	}
	return nil
}

func productResponseReadError(ctx context.Context, status int, requestID string, err error) error {
	if failure := productDeadlineError(ctx, err); failure != nil {
		if diagnostic, ok := failure.(*Error); ok {
			diagnostic.Status, diagnostic.RequestID = status, requestID
		}
		return failure
	}
	return &Error{Code: "ErrResponseInvalid", Message: "service response could not be read", Status: status, RequestID: requestID, cause: err}
}

func (e *Error) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("[%s] %s (request_id: %s)", e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// IsCode reports whether err is an SDK Error with the specified product code.
// Use it only after the selected task's User Story Error Recovery Coverage documents that caller-owned branch.
func IsCode(err error, code string) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Code == code
	}
	return false
}

var (
	// Err... sentinel values identify validation or product failures for diagnostics and documented branches only.
	// They are not user-story SDK entry points, AI task-selection sources, recovery-strategy sources, or input-construction sources.
	ErrEndpointRequired                = &Error{Code: "ErrEndpointRequired", Message: "product endpoint is required"}
	ErrAuthRequired                    = &Error{Code: "ErrAuthRequired", Message: "api key, token, or session cookie is required"}
	ErrAPIKeyRequired                  = &Error{Code: "ErrAPIKeyRequired", Message: "api key is required"}
	ErrMalformedPersonalAccessToken    = &Error{Code: "ErrMalformedPersonalAccessToken", Message: "personal access token must be a single raw header value"}
	ErrPersonalAccessTokenAuthConflict = &Error{Code: "ErrPersonalAccessTokenAuthConflict", Message: "personal access token clients cannot use bearer tokens or session cookies"}
	ErrWorkspaceRequired               = &Error{Code: "ErrWorkspaceRequired", Message: "workspace id is required"}
	ErrCurrentRoleInvalid              = &Error{Code: "ErrCurrentRoleInvalid", Message: "current role id must be a canonical positive int64"}
	ErrWorkspaceCurrentRoleConflict    = &Error{Code: "ErrWorkspaceCurrentRoleConflict", Message: "workspace has conflicting current role ids"}
	ErrAgentRequired                   = &Error{Code: "ErrAgentRequired", Message: "agent_code or agent_id is required"}
	ErrJSONRPCInvalid                  = &Error{Code: "ErrJSONRPCInvalid", Message: "jsonrpc must be 2.0"}
	ErrMethodRequired                  = &Error{Code: "ErrMethodRequired", Message: "method is required"}
	ErrMethodInvalid                   = &Error{Code: "ErrMethodInvalid", Message: "method is invalid"}
	ErrIDRequired                      = &Error{Code: "ErrIDRequired", Message: "id is required"}
	ErrIAMBindingVersionInvalid        = &Error{Code: "ErrIAMBindingVersionInvalid", Message: "IAM binding version is invalid"}
	ErrNameRequired                    = &Error{Code: "ErrNameRequired", Message: "name is required"}
	ErrDatabaseRenameUnsupported       = &Error{Code: "ErrDatabaseRenameUnsupported", Message: "database rename is not supported by the product API"}
	ErrCodeRequired                    = &Error{Code: "ErrCodeRequired", Message: "code is required"}
	ErrEmailRequired                   = &Error{Code: "ErrEmailRequired", Message: "email is required"}
	ErrMobileRequired                  = &Error{Code: "ErrMobileRequired", Message: "mobile number is required"}
	ErrInvitationContactRequired       = &Error{Code: "ErrInvitationContactRequired", Message: "exactly one invitation contact is required"}
	ErrInvitationContactConflict       = &Error{Code: "ErrInvitationContactConflict", Message: "email, mobile number, and github login cannot be combined"}
	ErrInvitationGitHubLoginInvalid    = &Error{Code: "ErrInvitationGitHubLoginInvalid", Message: "github login must be 1-39 letters, digits, or hyphens and cannot start or end with a hyphen"}
	ErrInvitationMobileNumberInvalid   = &Error{Code: "ErrInvitationMobileNumberInvalid", Message: "invitation mobile number must be an 11-digit mainland China number"}
	ErrDefaultRoleRequired             = &Error{Code: "ErrDefaultRoleRequired", Message: "default role id is required"}
	ErrDefaultRoleNotAssigned          = &Error{Code: "ErrDefaultRoleNotAssigned", Message: "default role must be included in assigned roles"}
	ErrFileIDRequired                  = &Error{Code: "ErrFileIDRequired", Message: "file id is required"}
	ErrFileIDsRequired                 = &Error{Code: "ErrFileIDsRequired", Message: "file ids are required"}
	ErrVolumeIDRequired                = &Error{Code: "ErrVolumeIDRequired", Message: "volume id is required"}
	ErrConnectorIDRequired             = &Error{Code: "ErrConnectorIDRequired", Message: "connector id is required"}
	ErrConnectorFileIDRequired         = &Error{Code: "ErrConnectorFileIDRequired", Message: "conn_file_id is required"}
	ErrDatabaseRequired                = &Error{Code: "ErrDatabaseRequired", Message: "database is required"}
	ErrTableRequired                   = &Error{Code: "ErrTableRequired", Message: "table is required"}
	ErrTablesRequired                  = &Error{Code: "ErrTablesRequired", Message: "tables are required"}
	ErrTaskIDRequired                  = &Error{Code: "ErrTaskIDRequired", Message: "task_id is required"}
	ErrTokenRequired                   = &Error{Code: "ErrTokenRequired", Message: "token is required"}
	ErrOriginRequired                  = &Error{Code: "ErrOriginRequired", Message: "origin is required"}
	ErrTurnIDRequired                  = &Error{Code: "ErrTurnIDRequired", Message: "turn_id is required"}
	ErrAnswersRequired                 = &Error{Code: "ErrAnswersRequired", Message: "answers are required"}
	ErrVersionRequired                 = &Error{Code: "ErrVersionRequired", Message: "version is required"}
	ErrServiceNameRequired             = &Error{Code: "ErrServiceNameRequired", Message: "service_name is required"}
	ErrPayloadRequired                 = &Error{Code: "ErrPayloadRequired", Message: "payload is required"}
	ErrWorkflowIDRequired              = &Error{Code: "ErrWorkflowIDRequired", Message: "workflow_id is required"}
	ErrExecutionIDRequired             = &Error{Code: "ErrExecutionIDRequired", Message: "execution_id is required"}
	ErrTaskNameRequired                = &Error{Code: "ErrTaskNameRequired", Message: "task name is required"}
	ErrSourceConfigRequired            = &Error{Code: "ErrSourceConfigRequired", Message: "source_config is required"}
	ErrWorkbookIDRequired              = &Error{Code: "ErrWorkbookIDRequired", Message: "workbook_id is required"}
	ErrVersionIDRequired               = &Error{Code: "ErrVersionIDRequired", Message: "version_id is required"}
	ErrQueryRequired                   = &Error{Code: "ErrQueryRequired", Message: "query is required"}
	ErrSubjectRequired                 = &Error{Code: "ErrSubjectRequired", Message: "subject_id is required"}
	ErrQueryIDRequired                 = &Error{Code: "ErrQueryIDRequired", Message: "query_id is required"}
	ErrDataSourcesRequired             = &Error{Code: "ErrDataSourcesRequired", Message: "data_sources is required"}
	ErrStatementIDRequired             = &Error{Code: "ErrStatementIDRequired", Message: "statement_id is required"}
	ErrDSLYAMLRequired                 = &Error{Code: "ErrDSLYAMLRequired", Message: "dsl_yaml is required"}
	ErrDescriptionRequired             = &Error{Code: "ErrDescriptionRequired", Message: "description is required"}
	ErrIdentifierRequired              = &Error{Code: "ErrIdentifierRequired", Message: "identifier is required"}
	ErrInputSchemaRequired             = &Error{Code: "ErrInputSchemaRequired", Message: "input_schema is required"}
	ErrOutputSchemaRequired            = &Error{Code: "ErrOutputSchemaRequired", Message: "output_schema is required"}
	ErrModeRequired                    = &Error{Code: "ErrModeRequired", Message: "mode is required"}
	ErrSourceNodeRequired              = &Error{Code: "ErrSourceNodeRequired", Message: "source_node_execution_id is required"}
	ErrRerunContractHashRequired       = &Error{Code: "ErrRerunContractHashRequired", Message: "expected_rerun_contract_hash is required"}
	ErrSpecIDRequired                  = &Error{Code: "ErrSpecIDRequired", Message: "spec_id is required"}
	ErrCPURequired                     = &Error{Code: "ErrCPURequired", Message: "cpu is required"}
	ErrMemoryRequired                  = &Error{Code: "ErrMemoryRequired", Message: "memory_gib is required"}
	ErrAddressRequired                 = &Error{Code: "ErrAddressRequired", Message: "address is required"}
	ErrModelsRequired                  = &Error{Code: "ErrModelsRequired", Message: "models are required"}
	ErrStatusRequired                  = &Error{Code: "ErrStatusRequired", Message: "status is required"}
	ErrTableScopeRequired              = &Error{Code: "ErrTableScopeRequired", Message: "table_scope is required"}
	ErrTargetWorkspaceRequired         = &Error{Code: "ErrTargetWorkspaceRequired", Message: "target_workspace_ids are required"}
	ErrSourceDatabaseIDRequired        = &Error{Code: "ErrSourceDatabaseIDRequired", Message: "source_database_id is required"}
	ErrGroupIDRequired                 = &Error{Code: "ErrGroupIDRequired", Message: "group_id is required"}
	ErrExpressionIDRequired            = &Error{Code: "ErrExpressionIDRequired", Message: "expression_id is required"}
	ErrLevelRequired                   = &Error{Code: "ErrLevelRequired", Message: "level is required"}
	ErrKindRequired                    = &Error{Code: "ErrKindRequired", Message: "kind is required"}
	ErrWorkerTypeConflict              = &Error{Code: "ErrWorkerTypeConflict", Message: "worker_type and kind must match when both are provided"}
	ErrKeyRequired                     = &Error{Code: "ErrKeyRequired", Message: "key is required"}
	ErrSpecRequired                    = &Error{Code: "ErrSpecRequired", Message: "spec is required"}
	ErrEntriesRequired                 = &Error{Code: "ErrEntriesRequired", Message: "entries are required"}
	ErrKnowledgeBaseIDRequired         = &Error{Code: "ErrKnowledgeBaseIDRequired", Message: "knowledge_base_id is required"}
	ErrConfigRequired                  = &Error{Code: "ErrConfigRequired", Message: "config is required"}
	ErrTypeRequired                    = &Error{Code: "ErrTypeRequired", Message: "type is required"}
	ErrSessionIDRequired               = &Error{Code: "ErrSessionIDRequired", Message: "session_id is required"}
	ErrMessageIDRequired               = &Error{Code: "ErrMessageIDRequired", Message: "message_id is required"}
	ErrProviderRequired                = &Error{Code: "ErrProviderRequired", Message: "provider is required"}
	ErrActionRequired                  = &Error{Code: "ErrActionRequired", Message: "action is required"}
	ErrCategoryRequired                = &Error{Code: "ErrCategoryRequired", Message: "category is required"}
	ErrSQLRequired                     = &Error{Code: "ErrSQLRequired", Message: "sql is required"}
	ErrURLRequired                     = &Error{Code: "ErrURLRequired", Message: "url is required"}
	ErrColumnsRequired                 = &Error{Code: "ErrColumnsRequired", Message: "columns are required"}
	ErrPathRequired                    = &Error{Code: "ErrPathRequired", Message: "path is required"}
	ErrPathInvalid                     = &Error{Code: "ErrPathInvalid", Message: "path is invalid"}
)
