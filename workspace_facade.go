package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// WorkspaceHandle is a user-story handle for operations scoped to one UI-visible workspace.
type WorkspaceHandle struct {
	client *Client
	id     string
	name   string
}

// CatalogHandle is a user-story handle for operations scoped to one catalog.
type CatalogHandle struct {
	workspace *WorkspaceHandle
	id        int
	name      string
}

// DatabaseHandle is a user-story handle for operations scoped to one database.
type DatabaseHandle struct {
	catalog *CatalogHandle
	id      int
	name    string
}

// VolumeHandle is a user-story handle for operations scoped to one catalog volume.
type VolumeHandle struct {
	database *DatabaseHandle
	id       int
	name     string
}

// TableHandle is a user-story handle for one catalog table.
type TableHandle struct {
	database *DatabaseHandle
	id       int
	name     string
}

// FileHandle is a user-story handle for one file attached to a catalog volume.
type FileHandle struct {
	volume *VolumeHandle
	id     string
}

// FolderHandle is a user-story handle for one folder under a catalog volume.
type FolderHandle struct {
	volume *VolumeHandle
	id     int
	name   string
}

// ArtifactHandle is a user-story handle for one workflow artifact.
type ArtifactHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// ArtifactBlockHandle is a user-story handle for one artifact output block.
type ArtifactBlockHandle struct {
	artifact *ArtifactHandle
	id       string
}

// ArtifactNodeHandle is a user-story handle for one artifact lineage node.
type ArtifactNodeHandle struct {
	artifact *ArtifactHandle
	id       string
}

// ArtifactNodeBlockHandle is a user-story handle for one artifact node output block.
type ArtifactNodeBlockHandle struct {
	node *ArtifactNodeHandle
	id   string
}

// WorkflowHandle is a user-story handle for one UI-visible workflow app.
type WorkflowHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// WorkflowRunHandle is a user-story handle for one workflow execution.
type WorkflowRunHandle struct {
	workflow *WorkflowHandle
	id       string
}

// WorkflowRunNodeHandle is a user-story handle for one node inside a workflow execution.
type WorkflowRunNodeHandle struct {
	run *WorkflowRunHandle
	key string
}

// WorkflowRunRerunHandle is a user-story handle for one rerun branch of a workflow execution.
type WorkflowRunRerunHandle struct {
	run *WorkflowRunHandle
	id  string
}

// CustomOperatorHandle is a user-story handle for one workflow custom operator.
type CustomOperatorHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// WorkitemAPIServiceHandle is a user-story handle for publishing and invoking one catalog WorkItem API service.
type WorkitemAPIServiceHandle struct {
	workspace *WorkspaceHandle
	nodeID    string
	version   string
}

// CustomOperatorSpec describes the complete product request for creating a custom operator.
// Optional pointer fields distinguish an omitted value from an explicitly supplied value.
type CustomOperatorSpec struct {
	Name            string         `json:"name"`
	Identifier      string         `json:"identifier"`
	Description     string         `json:"description"`
	InputSchema     map[string]any `json:"input_schema"`
	OutputSchema    map[string]any `json:"output_schema"`
	NodeID          *string        `json:"node_id,omitempty"`
	Kind            *string        `json:"kind,omitempty"`
	Language        *string        `json:"language,omitempty"`
	Handler         *string        `json:"handler,omitempty"`
	Version         *string        `json:"version,omitempty"`
	IsolationLevel  *string        `json:"isolation_level,omitempty"`
	Enabled         *bool          `json:"enabled,omitempty"`
	Code            *string        `json:"code,omitempty"`
	SourceFileID    *string        `json:"source_file_id,omitempty"`
	BaseNodeID      *string        `json:"base_node_id,omitempty"`
	BaseNodeVersion *string        `json:"base_node_version,omitempty"`
	BindingConfig   any            `json:"binding_config,omitempty"`
	CatalogID       *int64         `json:"catalog_id,omitempty"`
	DatabaseID      *int64         `json:"database_id,omitempty"`
}

// CustomOperatorTestInput describes one custom operator test-run request.
type CustomOperatorTestInput struct {
	Input              map[string]any `json:"input"`
	WaitTimeoutSeconds *int           `json:"wait_timeout_seconds,omitempty"`
}

// WorkitemAPIServicePublishSpec describes one WorkItem API service publish request.
type WorkitemAPIServicePublishSpec struct {
	Version           string                   `json:"version"`
	ServiceName       string                   `json:"service_name,omitempty"`
	ComputeResourceID string                   `json:"compute_resource_id,omitempty"`
	TimeoutSeconds    int                      `json:"timeout_seconds,omitempty"`
	MaxConcurrency    int                      `json:"max_concurrency,omitempty"`
	RateLimitPerMin   int                      `json:"rate_limit_per_min,omitempty"`
	UIResource        *WorkflowUIResourceParam `json:"ui_resource,omitempty"`
}

// Validate checks whether this publish request has the required product fields.
func (s WorkitemAPIServicePublishSpec) Validate() error {
	if s.Version == "" {
		return ErrVersionRequired
	}
	return nil
}

// WorkitemAPIServiceInvokeSpec describes one WorkItem API service invocation.
type WorkitemAPIServiceInvokeSpec struct {
	ServiceName string          `json:"service_name"`
	Type        string          `json:"type"`
	Version     string          `json:"version"`
	Payload     json.RawMessage `json:"payload"`
}

// DynamicServiceInvokeSpec describes one workspace dynamic service invocation.
type DynamicServiceInvokeSpec struct {
	Files         []MultipartFile `json:"-"`
	ServiceName   string          `json:"service_name"`
	Version       string          `json:"version,omitempty"`
	Payload       json.RawMessage `json:"payload"`
	VersionNumber int32           `json:"version_number,omitempty"`
	Trace         json.RawMessage `json:"trace,omitempty"`
}

// DynamicServiceInvokeResult is the HTTP dynamic service invoke response.
// Agent automation invocations return run_id/status/result_url; plain dynamic
// services return case_id/status/result/error from Mowl.
type DynamicServiceInvokeResult struct {
	CaseID    string          `json:"case_id,omitempty"`
	RunID     string          `json:"run_id,omitempty"`
	Status    string          `json:"status,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	ResultURL string          `json:"result_url,omitempty"`
}

// Validate checks whether this invoke request has the required product fields.
func workitemAPIServiceInvokeValidationError(sentinel *Error, message, field, reason, expected string) *Error {
	details := ErrorDetails{"field": field, "reason": reason}
	if expected != "" {
		details["expected"] = expected
	}
	return &Error{Code: sentinel.Code, Message: message, Details: details}
}

func (s WorkitemAPIServiceInvokeSpec) Validate() error {
	if s.ServiceName == "" {
		return workitemAPIServiceInvokeValidationError(ErrServiceNameRequired, "service_name is required for WorkItem API service invocation; use the published service.service_name", "service_name", "required", "")
	}
	if s.Type == "" {
		return workitemAPIServiceInvokeValidationError(ErrTypeRequired, "type is required for WorkItem API service invocation; use \"operator\"", "type", "required", "operator")
	}
	if s.Version == "" {
		return workitemAPIServiceInvokeValidationError(ErrVersionRequired, "version is required for WorkItem API service invocation; use the published service.version", "version", "required", "")
	}
	if len(s.Payload) == 0 {
		return workitemAPIServiceInvokeValidationError(ErrPayloadRequired, "payload is required for WorkItem API service invocation; provide the business input object, not an input wrapper", "payload", "required", "")
	}
	return nil
}

// Validate checks whether this dynamic service invoke request has the required product fields.
func (s DynamicServiceInvokeSpec) Validate() error {
	if s.ServiceName == "" {
		return ErrServiceNameRequired
	}
	if len(s.Payload) == 0 {
		return ErrPayloadRequired
	}
	return nil
}

// Validate checks whether this custom operator create spec has the required product fields.
func (s CustomOperatorSpec) Validate() error {
	if s.Name == "" {
		return ErrNameRequired
	}
	if s.Identifier == "" {
		return ErrIdentifierRequired
	}
	if s.Description == "" {
		return ErrDescriptionRequired
	}
	if s.InputSchema == nil {
		return ErrInputSchemaRequired
	}
	if s.OutputSchema == nil {
		return ErrOutputSchemaRequired
	}
	return nil
}

// Body returns the product custom-operator create payload.
func (s CustomOperatorSpec) Body(opts ...CustomOperatorOption) (map[string]any, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	body := map[string]any{
		"name": s.Name, "identifier": s.Identifier, "description": s.Description,
		"input_schema": s.InputSchema, "output_schema": s.OutputSchema,
	}
	for _, field := range []struct {
		name  string
		value *string
	}{
		{"node_id", s.NodeID}, {"kind", s.Kind}, {"language", s.Language}, {"handler", s.Handler},
		{"version", s.Version}, {"isolation_level", s.IsolationLevel}, {"code", s.Code},
		{"source_file_id", s.SourceFileID}, {"base_node_id", s.BaseNodeID}, {"base_node_version", s.BaseNodeVersion},
	} {
		if field.value != nil {
			body[field.name] = *field.value
		}
	}
	if s.Enabled != nil {
		body["enabled"] = *s.Enabled
	}
	if s.BindingConfig != nil {
		body["binding_config"] = s.BindingConfig
	}
	for _, field := range []struct {
		name  string
		value *int64
	}{{"catalog_id", s.CatalogID}, {"database_id", s.DatabaseID}} {
		if field.value != nil {
			body[field.name] = *field.value
		}
	}
	// Explicit function options override JSON/spec fields, matching the CLI's
	// long-standing flag-over-JSON behavior.
	for key, value := range customOperatorBody(s.Name, s.Identifier, s.Description, s.InputSchema, s.OutputSchema, opts...) {
		body[key] = value
	}
	return body, nil
}

// Validate checks whether this custom operator test input has a request payload.
func (i CustomOperatorTestInput) Validate() error {
	if i.Input == nil {
		return ErrPayloadRequired
	}
	return nil
}

// Body returns the product custom-operator test-run payload.
func (i CustomOperatorTestInput) Body() (map[string]any, error) {
	if err := i.Validate(); err != nil {
		return nil, err
	}
	body := map[string]any{"input": i.Input}
	if i.WaitTimeoutSeconds != nil {
		body["wait_timeout_seconds"] = *i.WaitTimeoutSeconds
	}
	return body, nil
}

// WorkflowTemplateHandle is a user-story handle for one workflow template.
type WorkflowTemplateHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// ConnectorHandle is a user-story handle for one product connector.
type ConnectorHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// ConnectorFileHandle is a user-story handle for one temporary connector file.
type ConnectorFileHandle struct {
	workspace *WorkspaceHandle
	connector *ConnectorHandle
	id        string
}

// ImportTaskCreateSpec describes an import task create request.
type ImportTaskCreateSpec struct {
	TaskID               string
	SourceConnectorID    string
	ConfigType           int
	SourceConfig         map[string]any
	StructuredLoadConfig map[string]any
	Name                 string
	VolumeID             string
}

// FileImportTaskSpec describes a file import task from the user's perspective.
type FileImportTaskSpec struct {
	TaskID             string
	ConfigType         int
	Name               string
	VolumeID           string
	URIs               []string
	LoadModeConfig     map[string]any
	FileFilterConfig   map[string]any
	UnzipKeepStructure *bool
	Dedup              map[string]any
	TableConfig        any
	// FileStorageMode selects copy (default: source content is copied into MOI
	// storage) or reference (issue #19365: the Catalog file points at the
	// source object without copying).
	FileStorageMode string
}

// Body returns the product create-task payload.
func (s ImportTaskCreateSpec) Body() (map[string]any, error) {
	if s.SourceConnectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if s.SourceConfig == nil && s.StructuredLoadConfig == nil {
		return nil, ErrSourceConfigRequired
	}
	body := map[string]any{
		"source_connector_id": s.SourceConnectorID,
		"config_type":         s.ConfigType,
	}
	if s.TaskID != "" {
		body["task_id"] = s.TaskID
	}
	if s.SourceConfig != nil {
		body["source_config"] = s.SourceConfig
	}
	if s.StructuredLoadConfig != nil {
		body["structured_load_config"] = s.StructuredLoadConfig
	}
	if s.Name != "" {
		body["name"] = s.Name
	}
	if s.VolumeID != "" {
		body["volume_id"] = s.VolumeID
	}
	return body, nil
}

// Validate checks whether this file import task spec has the required create-task fields.
func (s FileImportTaskSpec) Validate() error {
	if s.ConfigType == 0 {
		return ErrTypeRequired
	}
	if _, err := s.SourceConfig(); err != nil {
		return err
	}
	return nil
}

// ImportTaskCreateSpec returns the service-level import task create spec.
func (s FileImportTaskSpec) ImportTaskCreateSpec(sourceConnectorID string) (ImportTaskCreateSpec, error) {
	sourceConfig, err := s.SourceConfig()
	if err != nil {
		return ImportTaskCreateSpec{}, err
	}
	return ImportTaskCreateSpec{
		TaskID:            s.TaskID,
		SourceConnectorID: sourceConnectorID,
		ConfigType:        s.ConfigType,
		SourceConfig:      sourceConfig,
		Name:              s.Name,
		VolumeID:          s.VolumeID,
	}, nil
}

// ImportTaskUpdateSpec describes editable import task fields without exposing the raw route body.
type ImportTaskUpdateSpec struct {
	Name             string
	LoadModeConfig   map[string]any
	FileFilterConfig map[string]any
	URIs             []string
}

// SourceConfig returns the product source_config for a file import task.
func (s FileImportTaskSpec) SourceConfig() (map[string]any, error) {
	if len(s.URIs) == 0 {
		return nil, ErrURLRequired
	}
	if s.LoadModeConfig == nil {
		return nil, ErrModeRequired
	}
	if s.FileFilterConfig == nil {
		return nil, ErrSourceConfigRequired
	}
	common := map[string]any{
		"uris":               s.URIs,
		"load_mode_config":   s.LoadModeConfig,
		"file_filter_config": s.FileFilterConfig,
	}
	if s.UnzipKeepStructure != nil {
		common["unzip_keep_structure"] = *s.UnzipKeepStructure
	}
	if s.Dedup != nil {
		common["dedup"] = s.Dedup
	}
	if s.TableConfig != nil {
		common["table_config"] = s.TableConfig
	}
	if s.FileStorageMode != "" {
		common["file_storage_mode"] = s.FileStorageMode
	}
	return map[string]any{"common_file_task_config": common}, nil
}

// Updates returns the product update payload for an import task.
func (s ImportTaskUpdateSpec) Updates() (map[string]any, error) {
	updates := map[string]any{}
	if s.Name != "" {
		updates["name"] = s.Name
	}
	if s.LoadModeConfig != nil {
		updates["load_mode_config"] = s.LoadModeConfig
	}
	if s.FileFilterConfig != nil {
		updates["file_filter_config"] = s.FileFilterConfig
	}
	if s.URIs != nil {
		if len(s.URIs) == 0 {
			return nil, ErrURLRequired
		}
		updates["uris"] = s.URIs
	}
	if len(updates) == 0 {
		return nil, ErrSourceConfigRequired
	}
	return updates, nil
}

// ExportTaskSpec describes an export task without exposing the raw route body.
type ExportTaskSpec struct {
	ConnectorID   string
	Name          string
	Creator       string
	ConnectorName string
	Type          int
	Config        ExportTaskConfig
	Files         []ExportTaskFile
}

// ExportTaskConfig describes export target configuration.
type ExportTaskConfig struct {
	MergeTitleToText *bool
	DifyConfig       map[string]any
	MOConfig         map[string]any
	S3Config         map[string]any
}

// Validate checks whether this export task config contains at least one target config field.
func (c ExportTaskConfig) Validate() error {
	if c.MergeTitleToText == nil && c.DifyConfig == nil && c.MOConfig == nil && c.S3Config == nil {
		return ErrConfigRequired
	}
	return nil
}

// Map returns the product config object for an export task.
func (c ExportTaskConfig) Map() map[string]any {
	body := map[string]any{}
	if c.MergeTitleToText != nil {
		body["merge_title_to_text"] = *c.MergeTitleToText
	}
	if c.DifyConfig != nil {
		body["dify_config"] = c.DifyConfig
	}
	if c.MOConfig != nil {
		body["mo_config"] = c.MOConfig
	}
	if c.S3Config != nil {
		body["s3_config"] = c.S3Config
	}
	return body
}

// Validate checks whether this export task spec has the required product fields.
func (s ExportTaskSpec) Validate() error {
	if s.Name == "" {
		return ErrTaskNameRequired
	}
	if s.ConnectorID == "" {
		return ErrConnectorIDRequired
	}
	if s.ConnectorName == "" {
		return ErrConnectorIDRequired
	}
	if s.Type == 0 {
		return ErrTypeRequired
	}
	if err := s.Config.Validate(); err != nil {
		return err
	}
	if len(s.Files) == 0 {
		return ErrFileIDsRequired
	}
	for _, file := range s.Files {
		if _, err := file.Param(); err != nil {
			return err
		}
	}
	return nil
}

// Body returns the product export-task create payload.
func (s ExportTaskSpec) Body() (map[string]any, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	files := make([]ExportFileParam, 0, len(s.Files))
	for _, file := range s.Files {
		param, err := file.Param()
		if err != nil {
			return nil, err
		}
		files = append(files, param)
	}
	body := map[string]any{
		"task_name":      s.Name,
		"connector_id":   s.ConnectorID,
		"connector_name": s.ConnectorName,
		"type":           s.Type,
		"config":         s.Config.Map(),
		"files":          files,
	}
	if s.Creator != "" {
		body["creator"] = s.Creator
	}
	return body, nil
}

// ExportTaskFile describes one selected file in an export task.
type ExportTaskFile struct {
	FileID       string
	FullPath     []string
	IsRaw        *bool
	RefFileID    string
	ParsedFileID string
}

// Param returns the route-level export file parameter.
func (f ExportTaskFile) Param() (ExportFileParam, error) {
	if f.FileID == "" {
		return ExportFileParam{}, ErrFileIDRequired
	}
	if len(f.FullPath) == 0 {
		return ExportFileParam{}, ErrPathInvalid
	}
	param := ExportFileParam{
		FileID:       f.FileID,
		RefFileID:    f.RefFileID,
		ParsedFileID: f.ParsedFileID,
		FullPath:     f.FullPath,
	}
	if f.IsRaw != nil {
		param.IsRaw = *f.IsRaw
	}
	return param, nil
}

// SemanticModelHandle is a user-story handle for one semantic model.
type SemanticModelHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// SemanticEntryHandle is a user-story handle for one semantic model entry.
type SemanticEntryHandle struct {
	model *SemanticModelHandle
	id    string
}

// ImportTaskHandle is a user-story handle for one connector import task.
type ImportTaskHandle struct {
	connector *ConnectorHandle
	id        string
}

// ExportTaskHandle is a user-story handle for one connector export task.
type ExportTaskHandle struct {
	connector *ConnectorHandle
	id        string
}

// ModelProviderHandle is a user-story handle for one model provider family.
type ModelProviderHandle struct {
	workspace *WorkspaceHandle
	service   *ProviderBackendService
	kind      string
}

// ProviderBackendHandle is a user-story handle for one model provider backend.
type ProviderBackendHandle struct {
	provider *ModelProviderHandle
	id       string
}

// ProviderRouterHandle is a user-story handle for one model provider router.
type ProviderRouterHandle struct {
	provider *ModelProviderHandle
}

// ComputeResourceHandle is a user-story handle for one compute resource.
type ComputeResourceHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// DataSharePublishHandle is a user-story handle for one data-share publication.
type DataSharePublishHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// DataShareSubscriptionHandle is a user-story handle for one data-share subscription.
type DataShareSubscriptionHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// AgentHandle is a user-story handle for one selected product agent.
type AgentHandle struct {
	workspace *WorkspaceHandle
	selector  AgentSelector
}

// AgentTextMessage describes one user text message sent through A2A.
type AgentTextMessage struct {
	RequestID       any
	MessageID       string
	TaskID          string
	ContextID       string
	SessionID       string
	Text            string
	Model           string
	IdempotencyKey  string
	MessageMetadata map[string]any
	Metadata        map[string]any
	Configuration   map[string]any
}

// AgentTaskRequest describes one A2A task control request.
type AgentTaskRequest struct {
	RequestID any
	TaskID    string
	AfterSeq  *int
}

// AgentInputSubmission describes one human input answer set submitted to an A2A task.
type AgentInputSubmission struct {
	RequestID any
	TaskID    string
	TurnID    string
	CallID    string
	Answers   map[string][]string
}

// Params returns A2A task control params.
func (r AgentTaskRequest) Params() (map[string]any, error) {
	if r.TaskID == "" {
		return nil, ErrTaskIDRequired
	}
	params := map[string]any{"id": r.TaskID}
	if r.AfterSeq != nil {
		params["afterSeq"] = *r.AfterSeq
	}
	return params, nil
}

// Params returns matrixflow/input.submit params for this answer set.
func (s AgentInputSubmission) Params() (map[string]any, error) {
	if s.TaskID == "" {
		return nil, ErrTaskIDRequired
	}
	if s.TurnID == "" {
		return nil, ErrTurnIDRequired
	}
	if len(s.Answers) == 0 {
		return nil, ErrAnswersRequired
	}
	answers := make(map[string]any, len(s.Answers))
	for key, values := range s.Answers {
		if len(values) == 0 {
			return nil, ErrAnswersRequired
		}
		answers[key] = map[string]any{"answers": values}
	}
	params := map[string]any{
		"taskId":  s.TaskID,
		"turnId":  s.TurnID,
		"answers": answers,
	}
	if s.CallID != "" {
		params["callId"] = s.CallID
	}
	return params, nil
}

// Params returns A2A message/send params for this text message.
func (m AgentTextMessage) Params() (map[string]any, error) {
	if m.Text == "" {
		return nil, ErrQueryRequired
	}
	if m.MessageID == "" {
		return nil, ErrMessageIDRequired
	}
	message := map[string]any{
		"kind":      "message",
		"role":      "user",
		"messageId": m.MessageID,
		"parts":     []map[string]any{{"kind": "text", "text": m.Text}},
	}
	if m.TaskID != "" {
		message["taskId"] = m.TaskID
	}
	if m.ContextID != "" {
		message["contextId"] = m.ContextID
	}
	if m.MessageMetadata != nil {
		message["metadata"] = m.MessageMetadata
	}
	params := map[string]any{"message": message}
	if m.IdempotencyKey != "" {
		params["idempotencyKey"] = m.IdempotencyKey
	}
	if m.Model != "" {
		params["model"] = m.Model
	}
	metadata := m.Metadata
	if m.SessionID != "" {
		metadata = make(map[string]any, len(m.Metadata)+1)
		for key, value := range m.Metadata {
			metadata[key] = value
		}
		metadata["session_id"] = m.SessionID
	}
	if metadata != nil {
		params["metadata"] = metadata
	}
	if m.Configuration != nil {
		params["configuration"] = m.Configuration
	}
	return params, nil
}

// SQLMetadataHandle is a user-story handle for SQL editor metadata browsing.
type SQLMetadataHandle struct {
	workspace *WorkspaceHandle
}

// SQLDatabaseMetadataHandle is a user-story handle for one SQL editor database.
type SQLDatabaseMetadataHandle struct {
	metadata *SQLMetadataHandle
	name     string
}

// SQLTableMetadataHandle is a user-story handle for one SQL editor table or view.
type SQLTableMetadataHandle struct {
	database *SQLDatabaseMetadataHandle
	name     string
}

// WorkspaceMemberHandle is a handle for one canonical workspace member.
type WorkspaceMemberHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// SQLWorkbookHandle is a user-story handle for one SQL workbook.
type SQLWorkbookHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// SQLWorkbookVersionHandle is a user-story handle for one SQL workbook version.
type SQLWorkbookVersionHandle struct {
	workbook *SQLWorkbookHandle
	id       string
}

// SQLQueryHandle is a user-story handle for one SQL query operation.
type SQLQueryHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// SQLStatementHandle is a user-story handle for one SQL statement result/history record.
type SQLStatementHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// UploadedFilePreview is the result of uploading a file and opening its preview stream.
type UploadedFilePreview struct {
	File    *FileHandle
	Preview *StreamResponse
}

// Workspace returns a scoped handle for a known workspace id.
func (c *Client) Workspace(workspaceID string) (*WorkspaceHandle, error) {
	if workspaceID == "" {
		return nil, ErrWorkspaceRequired
	}
	return &WorkspaceHandle{client: c, id: workspaceID}, nil
}

// EnsureWorkspace finds a visible workspace by exact name, or creates it when absent.
func (c *Client) EnsureWorkspace(ctx context.Context, name string) (*WorkspaceHandle, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	workspaces, err := c.Workspaces().List(ctx, WithWorkspaceNameFilter(name))
	if err != nil {
		return nil, err
	}
	var matched *WorkspaceHandle
	for _, workspace := range workspaces.GetWorkspaces() {
		if workspace.GetName() == name {
			if matched != nil {
				return nil, duplicateNameError("workspace", name)
			}
			matched = &WorkspaceHandle{client: c, id: workspace.GetId(), name: workspace.GetName()}
		}
	}
	if matched != nil {
		return matched, nil
	}
	created, err := c.Workspaces().Create(ctx, name)
	if err != nil {
		return nil, err
	}
	return &WorkspaceHandle{client: c, id: created.GetId(), name: name}, nil
}

// ID returns the workspace id used for product API calls.
func (w *WorkspaceHandle) ID() string {
	return w.id
}

// Name returns the workspace name when it was resolved by EnsureWorkspace.
func (w *WorkspaceHandle) Name() string {
	return w.name
}

// Delete deletes this workspace.
func (w *WorkspaceHandle) Delete(ctx context.Context, opts ...QueryOption) error {
	return w.client.Workspaces().Delete(ctx, w.id, opts...)
}

// DBConnection returns the database connection information for this workspace.
func (w *WorkspaceHandle) DBConnection(ctx context.Context) (*DBConnectionResult, error) {
	return w.client.Account().DBConnection(ctx, w.id)
}

// RecentFailedAuditLogs returns recent failed audit logs in this workspace.
func (w *WorkspaceHandle) RecentFailedAuditLogs(ctx context.Context, limit int) (*AuditLogListResult, error) {
	if limit <= 0 {
		return nil, &Error{Code: "ErrLimitRequired", Message: "limit is required"}
	}
	return w.Members().AuditLogs(ctx, WithAuditResult("failed"), WithPage(1), WithPageSize(limit))
}

// PrepareCatalog finds a catalog by exact name, or creates it when absent.
func (w *WorkspaceHandle) PrepareCatalog(ctx context.Context, name string, opts ...CatalogInfoOption) (*CatalogHandle, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	catalogs, err := w.Catalog().List(ctx)
	if err != nil {
		return nil, err
	}
	var matched *CatalogHandle
	for _, catalog := range catalogs.GetList() {
		if catalog.GetName() != name {
			continue
		}
		if matched != nil {
			return nil, duplicateNameError("catalog", name)
		}
		matched = &CatalogHandle{workspace: w, id: int(catalog.GetId()), name: catalog.GetName()}
	}
	if matched != nil {
		return matched, nil
	}
	created, err := w.Catalog().Create(ctx, name, opts...)
	if err != nil {
		return nil, err
	}
	return &CatalogHandle{workspace: w, id: int(created.GetId()), name: name}, nil
}

// PrepareDataVolume finds or creates the catalog, database, and volume needed to upload data.
func (w *WorkspaceHandle) PrepareDataVolume(ctx context.Context, catalogName, databaseName, volumeName string) (*VolumeHandle, error) {
	catalog, err := w.PrepareCatalog(ctx, catalogName)
	if err != nil {
		return nil, err
	}
	database, err := catalog.PrepareDatabase(ctx, databaseName)
	if err != nil {
		return nil, err
	}
	return database.PrepareVolume(ctx, volumeName)
}

// UploadBytesToDataVolume prepares a named data volume, uploads bytes, attaches the file, and returns a file handle.
func (w *WorkspaceHandle) UploadBytesToDataVolume(ctx context.Context, catalogName, databaseName, volumeName, filename string, data []byte) (*FileHandle, error) {
	volume, err := w.PrepareDataVolume(ctx, catalogName, databaseName, volumeName)
	if err != nil {
		return nil, err
	}
	return volume.UploadBytesHandle(ctx, filename, data)
}

// UploadFileToDataVolume prepares a named data volume, uploads a caller-supplied local file path, attaches it, and returns a file handle.
// It is a local file input convenience method, not AI file-discovery, local-filesystem probing, filename inference, or default upload entry point; prefer UploadBytesToDataVolume when bytes are already caller-owned.
func (w *WorkspaceHandle) UploadFileToDataVolume(ctx context.Context, catalogName, databaseName, volumeName, filePath string) (*FileHandle, error) {
	volume, err := w.PrepareDataVolume(ctx, catalogName, databaseName, volumeName)
	if err != nil {
		return nil, err
	}
	return volume.UploadFileHandle(ctx, filePath)
}

// UploadBytesPreviewToDataVolume uploads bytes to a named data volume and opens its preview stream.
func (w *WorkspaceHandle) UploadBytesPreviewToDataVolume(ctx context.Context, catalogName, databaseName, volumeName, filename string, data []byte) (*UploadedFilePreview, error) {
	file, err := w.UploadBytesToDataVolume(ctx, catalogName, databaseName, volumeName, filename, data)
	if err != nil {
		return nil, err
	}
	preview, err := file.Preview(ctx)
	if err != nil {
		return nil, err
	}
	return &UploadedFilePreview{File: file, Preview: preview}, nil
}

// UploadFilePreviewToDataVolume uploads a caller-supplied local file path to a named data volume and opens its preview stream.
// It is a local file input convenience method, not AI file-discovery, local-filesystem probing, filename inference, or default upload entry point; prefer UploadBytesPreviewToDataVolume when bytes are already caller-owned.
func (w *WorkspaceHandle) UploadFilePreviewToDataVolume(ctx context.Context, catalogName, databaseName, volumeName, filePath string) (*UploadedFilePreview, error) {
	file, err := w.UploadFileToDataVolume(ctx, catalogName, databaseName, volumeName, filePath)
	if err != nil {
		return nil, err
	}
	preview, err := file.Preview(ctx)
	if err != nil {
		return nil, err
	}
	return &UploadedFilePreview{File: file, Preview: preview}, nil
}

// DataVolumeFullPath prepares a named data volume and returns its UI full path.
func (w *WorkspaceHandle) DataVolumeFullPath(ctx context.Context, catalogName, databaseName, volumeName string) (*PathResult, error) {
	volume, err := w.PrepareDataVolume(ctx, catalogName, databaseName, volumeName)
	if err != nil {
		return nil, err
	}
	return volume.FullPath(ctx)
}

// Catalogs lists UI-visible catalogs as scoped handles.
func (w *WorkspaceHandle) Catalogs(ctx context.Context, opts ...CatalogListOption) ([]*CatalogHandle, *CatalogListResult, error) {
	result, err := w.Catalog().List(ctx, opts...)
	if err != nil {
		return nil, nil, err
	}
	handles := make([]*CatalogHandle, 0, len(result.GetList()))
	for _, catalog := range result.GetList() {
		handles = append(handles, &CatalogHandle{workspace: w, id: int(catalog.GetId()), name: catalog.GetName()})
	}
	return handles, result, nil
}

// Workflow returns a handle for a known workflow app id.
func (w *WorkspaceHandle) Workflow(workflowID string) (*WorkflowHandle, error) {
	if workflowID == "" {
		return nil, ErrIDRequired
	}
	return &WorkflowHandle{workspace: w, id: workflowID}, nil
}

// DeployWorkflow creates a workflow when workflowID is empty, or deploys a new
// version of an existing workflow. The returned handle uses the response ID.
func (w *WorkspaceHandle) DeployWorkflow(ctx context.Context, name, dslYAML, workflowID string, opts ...WorkflowDeployOption) (*WorkflowHandle, *WorkflowDeploymentEnvelope, error) {
	if workflowID != "" {
		opts = append([]WorkflowDeployOption{WithWorkflowID(workflowID)}, opts...)
	}
	result, err := w.Workflows().Deploy(ctx, name, dslYAML, opts...)
	return w.workflowDeploymentHandle(result, err)
}

// AcceptWorkflowCandidate explicitly accepts a server-owned A2A candidate and
// returns the typed handoff required by DeployWorkflowCandidate.
func (w *WorkspaceHandle) AcceptWorkflowCandidate(ctx context.Context, candidateRef WorkflowCandidateRef, confirmation *WorkflowCandidateConfirmation) (*AcceptedWorkflowCandidate, error) {
	return w.Workflows().AcceptCandidate(ctx, candidateRef, confirmation)
}

// DeployWorkflowCandidate deploys an explicitly accepted candidate. An empty
// workflowID performs its first deployment (or replays that deployment).
func (w *WorkspaceHandle) DeployWorkflowCandidate(ctx context.Context, name string, acceptedCandidate *AcceptedWorkflowCandidate, workflowID string, opts ...WorkflowDeployOption) (*WorkflowHandle, *WorkflowDeploymentEnvelope, error) {
	if workflowID != "" {
		opts = append([]WorkflowDeployOption{WithWorkflowID(workflowID)}, opts...)
	}
	result, err := w.Workflows().DeployCandidate(ctx, name, acceptedCandidate, opts...)
	return w.workflowDeploymentHandle(result, err)
}

func (w *WorkspaceHandle) workflowDeploymentHandle(result *WorkflowDeploymentEnvelope, err error) (*WorkflowHandle, *WorkflowDeploymentEnvelope, error) {
	if err != nil {
		return nil, nil, err
	}
	id, err := createdStringPathID(result.GetWorkflow().GetId(), "workflow")
	if err != nil {
		return nil, result, err
	}
	return &WorkflowHandle{workspace: w, id: id}, result, nil
}

// CustomOperator returns a handle for a known workflow custom operator id.
func (w *WorkspaceHandle) CustomOperator(operatorID string) (*CustomOperatorHandle, error) {
	if operatorID == "" {
		return nil, ErrIDRequired
	}
	return &CustomOperatorHandle{workspace: w, id: operatorID}, nil
}

// WorkitemAPIService returns a handle for one catalog WorkItem API service version.
func (w *WorkspaceHandle) WorkitemAPIService(nodeID, version string) (*WorkitemAPIServiceHandle, error) {
	if nodeID == "" {
		return nil, ErrIDRequired
	}
	if version == "" {
		return nil, ErrVersionRequired
	}
	return &WorkitemAPIServiceHandle{workspace: w, nodeID: nodeID, version: version}, nil
}

// CreateCustomOperator creates a workflow custom operator and returns a handle for the backend-created operator id.
func (w *WorkspaceHandle) CreateCustomOperator(ctx context.Context, name, identifier, description string, inputSchema, outputSchema map[string]any, opts ...CustomOperatorOption) (*CustomOperatorHandle, *CustomOperator, error) {
	spec := CustomOperatorSpec{
		Name:         name,
		Identifier:   identifier,
		Description:  description,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	}
	result, err := w.Workflows().CreateCustomOperator(ctx, spec, opts...)
	if err != nil {
		return nil, nil, err
	}
	operatorID, err := createdIntPathID(result.GetId(), "custom operator")
	if err != nil {
		return nil, nil, err
	}
	return &CustomOperatorHandle{workspace: w, id: operatorID}, result, nil
}

// CreateCustomOperatorFromSpec creates a workflow custom operator from an explicit product spec.
func (w *WorkspaceHandle) CreateCustomOperatorFromSpec(ctx context.Context, spec CustomOperatorSpec, opts ...CustomOperatorOption) (*CustomOperatorHandle, *CustomOperator, error) {
	if err := spec.Validate(); err != nil {
		return nil, nil, err
	}
	result, err := w.Workflows().CreateCustomOperator(ctx, spec, opts...)
	if err != nil {
		return nil, nil, err
	}
	operatorID, err := createdIntPathID(result.GetId(), "custom operator")
	if err != nil {
		return nil, nil, err
	}
	return &CustomOperatorHandle{workspace: w, id: operatorID}, result, nil
}

// WorkflowTemplate returns a handle for a known workflow template id.
func (w *WorkspaceHandle) WorkflowTemplate(templateID string) (*WorkflowTemplateHandle, error) {
	if templateID == "" {
		return nil, ErrIDRequired
	}
	return &WorkflowTemplateHandle{workspace: w, id: templateID}, nil
}

// CreateWorkflowTemplate creates a workflow template and returns a handle for the backend-created template id.
func (w *WorkspaceHandle) CreateWorkflowTemplate(ctx context.Context, name, dslYAML string, opts ...WorkflowTemplateOption) (*WorkflowTemplateHandle, *WorkflowTemplate, error) {
	result, err := w.Workflows().CreateWorkflowTemplate(ctx, name, dslYAML, opts...)
	if err != nil {
		return nil, nil, err
	}
	templateID, err := createdUintPathID(result.GetId(), "workflow template")
	if err != nil {
		return nil, nil, err
	}
	return &WorkflowTemplateHandle{workspace: w, id: templateID}, result, nil
}

// Connector returns a handle for a known connector id.
func (w *WorkspaceHandle) Connector(connectorID string) (*ConnectorHandle, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	return &ConnectorHandle{workspace: w, id: connectorID}, nil
}

// ConnectorFile returns a handle for a known temporary connector file id.
func (w *WorkspaceHandle) ConnectorFile(connFileID string) (*ConnectorFileHandle, error) {
	if connFileID == "" {
		return nil, ErrConnectorFileIDRequired
	}
	return &ConnectorFileHandle{workspace: w, id: connFileID}, nil
}

// CreateConnector creates a connector and returns a handle for the backend connector id.
func (w *WorkspaceHandle) CreateConnector(ctx context.Context, name string, sourceType, usageType int, opts ...ConnectorDefinitionOption) (*ConnectorHandle, *Connector, error) {
	result, err := w.Connectors().Create(ctx, name, sourceType, usageType, opts...)
	if err != nil {
		return nil, nil, err
	}
	return &ConnectorHandle{workspace: w, id: result.GetId()}, result, nil
}

// SemanticModel returns a handle for a known semantic model id.
func (w *WorkspaceHandle) SemanticModel(modelID string) (*SemanticModelHandle, error) {
	if modelID == "" {
		return nil, ErrIDRequired
	}
	return &SemanticModelHandle{workspace: w, id: modelID}, nil
}

// CreateSemanticModel creates a knowledge base and returns a handle for the backend-created model id.
// name is also the immutable Catalog database name.
func (w *WorkspaceHandle) CreateSemanticModel(ctx context.Context, name string, opts ...SemanticModelOption) (*SemanticModelHandle, *SemanticModelInfo, error) {
	result, err := w.Knowledge().Create(ctx, name, opts...)
	if err != nil {
		return nil, nil, err
	}
	modelID, err := createdIntPathID(result.GetId(), "semantic model")
	if err != nil {
		return nil, nil, err
	}
	return &SemanticModelHandle{workspace: w, id: modelID}, result, nil
}

// Artifact returns a handle for a known workflow artifact id.
func (w *WorkspaceHandle) Artifact(artifactID string) (*ArtifactHandle, error) {
	if artifactID == "" {
		return nil, ErrIDRequired
	}
	return &ArtifactHandle{workspace: w, id: artifactID}, nil
}

// ID returns the catalog id used for catalog API calls.
func (c *CatalogHandle) ID() int {
	return c.id
}

// Name returns the catalog name.
func (c *CatalogHandle) Name() string {
	return c.name
}

// Service returns the route-aligned escape hatch catalog service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (c *CatalogHandle) Service() *CatalogService {
	return c.workspace.Catalog()
}

// Info returns product catalog metadata for this catalog.
func (c *CatalogHandle) Info(ctx context.Context) (*CatalogInfoResult, error) {
	return c.Service().Info(ctx, c.id)
}

// Delete deletes this catalog.
func (c *CatalogHandle) Delete(ctx context.Context) error {
	return c.Service().Delete(ctx, c.id)
}

// Rename updates this catalog name and returns the same handle for chaining.
func (c *CatalogHandle) Rename(ctx context.Context, name string, opts ...CatalogInfoOption) (*CatalogHandle, error) {
	if err := c.Service().Update(ctx, c.id, name, opts...); err != nil {
		return nil, err
	}
	c.name = name
	return c, nil
}

// Databases lists databases under this catalog as scoped handles.
func (c *CatalogHandle) Databases(ctx context.Context, opts ...CatalogListOption) ([]*DatabaseHandle, *DatabaseListResult, error) {
	result, err := c.Service().ListDatabases(ctx, c.id, opts...)
	if err != nil {
		return nil, nil, err
	}
	handles := make([]*DatabaseHandle, 0, len(result.GetList()))
	for _, database := range result.GetList() {
		handles = append(handles, &DatabaseHandle{catalog: c, id: int(database.GetId()), name: database.GetName()})
	}
	return handles, result, nil
}

// PrepareDatabase finds a database by exact name under this catalog, or creates it when absent.
func (c *CatalogHandle) PrepareDatabase(ctx context.Context, name string, opts ...CatalogInfoOption) (*DatabaseHandle, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	databases, err := c.Service().ListDatabases(ctx, c.id)
	if err != nil {
		return nil, err
	}
	var matched *DatabaseHandle
	for _, database := range databases.GetList() {
		if database.GetName() != name {
			continue
		}
		if matched != nil {
			return nil, duplicateNameError("database", name)
		}
		matched = &DatabaseHandle{catalog: c, id: int(database.GetId()), name: database.GetName()}
	}
	if matched != nil {
		return matched, nil
	}
	created, err := c.Service().CreateDatabase(ctx, c.id, name, opts...)
	if err != nil {
		return nil, err
	}
	return &DatabaseHandle{catalog: c, id: int(created.GetId()), name: name}, nil
}

// ID returns the database id used for catalog API calls.
func (d *DatabaseHandle) ID() int {
	return d.id
}

// Name returns the database name.
func (d *DatabaseHandle) Name() string {
	return d.name
}

// Service returns the route-aligned escape hatch catalog service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (d *DatabaseHandle) Service() *CatalogService {
	return d.catalog.Service()
}

// Info returns product catalog metadata for this database.
func (d *DatabaseHandle) Info(ctx context.Context) (*DatabaseInfoResult, error) {
	return d.Service().DatabaseInfo(ctx, d.id)
}

// Delete deletes this database.
func (d *DatabaseHandle) Delete(ctx context.Context) error {
	return d.Service().DeleteDatabase(ctx, d.id)
}

// Rename returns an explicit error because the product API cannot rename databases.
func (d *DatabaseHandle) Rename(ctx context.Context, name string, opts ...CatalogInfoOption) (*DatabaseHandle, error) {
	return nil, ErrDatabaseRenameUnsupported
}

// UpdateDescription updates this database description and returns the same handle for chaining.
func (d *DatabaseHandle) UpdateDescription(ctx context.Context, description string) (*DatabaseHandle, error) {
	if _, err := d.Service().UpdateDatabaseDescription(ctx, d.id, WithCatalogDescription(description)); err != nil {
		return nil, err
	}
	return d, nil
}

// Table returns a handle for a known table under this database.
func (d *DatabaseHandle) Table(tableID int) (*TableHandle, error) {
	if tableID == 0 {
		return nil, ErrIDRequired
	}
	return &TableHandle{database: d, id: tableID}, nil
}

// Children returns UI children under this database.
func (d *DatabaseHandle) Children(ctx context.Context, opts ...CatalogListOption) (*DatabaseChildrenResult, error) {
	return d.Service().DatabaseChildren(ctx, d.id, opts...)
}

// Volumes lists volume children under this database as scoped handles.
func (d *DatabaseHandle) Volumes(ctx context.Context) ([]*VolumeHandle, *DatabaseChildrenResult, error) {
	children, err := d.Service().DatabaseChildren(ctx, d.id)
	if err != nil {
		return nil, nil, err
	}
	handles := make([]*VolumeHandle, 0)
	for _, child := range children.GetList() {
		if child.GetType() != "volume" {
			continue
		}
		volumeID, err := parsePositiveIntID(child.GetId(), "volume")
		if err != nil {
			return nil, nil, err
		}
		handles = append(handles, &VolumeHandle{database: d, id: volumeID, name: child.GetName()})
	}
	return handles, children, nil
}

// Tables lists table children under this database as scoped handles.
func (d *DatabaseHandle) Tables(ctx context.Context) ([]*TableHandle, *DatabaseChildrenResult, error) {
	children, err := d.Service().DatabaseChildren(ctx, d.id)
	if err != nil {
		return nil, nil, err
	}
	handles := make([]*TableHandle, 0)
	for _, child := range children.GetList() {
		if child.GetType() != "table" {
			continue
		}
		tableID, err := parsePositiveIntID(child.GetId(), "table")
		if err != nil {
			return nil, nil, err
		}
		handles = append(handles, &TableHandle{database: d, id: tableID, name: child.GetName()})
	}
	return handles, children, nil
}

// CreateTable creates a table and returns a handle for the backend-created table id.
func (d *DatabaseHandle) CreateTable(ctx context.Context, name string, columns []TableColumn, opts ...CreateTableOption) (*TableHandle, *CatalogCreateTableResult, error) {
	result, err := d.Service().CreateTable(ctx, d.id, name, columns, opts...)
	if err != nil {
		return nil, nil, err
	}
	return &TableHandle{database: d, id: int(result.GetId()), name: name}, result, nil
}

// TableExists checks whether a table name exists under this database.
func (d *DatabaseHandle) TableExists(ctx context.Context, name string) (*BoolDataResult, error) {
	return d.Service().TableExists(ctx, d.id, name)
}

// TableOverview returns table overview data scoped to this database.
func (d *DatabaseHandle) TableOverview(ctx context.Context) (*CatalogTableOverviewResult, error) {
	return d.Service().TableOverview(ctx, d.id)
}

// PrepareVolume finds a volume by exact name under this database, or creates it when absent.
func (d *DatabaseHandle) PrepareVolume(ctx context.Context, name string, opts ...CatalogInfoOption) (*VolumeHandle, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	children, err := d.Service().DatabaseChildren(ctx, d.id)
	if err != nil {
		return nil, err
	}
	var matched *VolumeHandle
	for _, child := range children.GetList() {
		if child.GetType() != "volume" || child.GetName() != name {
			continue
		}
		if matched != nil {
			return nil, duplicateNameError("volume", name)
		}
		volumeID, err := parsePositiveIntID(child.GetId(), "volume")
		if err != nil {
			return nil, err
		}
		matched = &VolumeHandle{database: d, id: volumeID, name: child.GetName()}
	}
	if matched != nil {
		return matched, nil
	}
	created, err := d.Service().CreateVolume(ctx, d.id, name, opts...)
	if err != nil {
		return nil, err
	}
	return &VolumeHandle{database: d, id: int(created.GetId()), name: name}, nil
}

// ID returns the catalog table id.
func (t *TableHandle) ID() int {
	return t.id
}

// Name returns the table name when this handle was created by name.
func (t *TableHandle) Name() string {
	return t.name
}

// Service returns the route-aligned escape hatch catalog service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (t *TableHandle) Service() *CatalogService {
	return t.database.Service()
}

// Info returns table metadata.
func (t *TableHandle) Info(ctx context.Context, opts ...TableLookupOption) (*CatalogTableInfoResult, error) {
	return t.Service().TableInfo(ctx, t.id, opts...)
}

// Data returns table data for UI preview grids.
func (t *TableHandle) Data(ctx context.Context, opts ...TableDataOption) (*CatalogTableDataResult, error) {
	return t.Service().TableData(ctx, t.id, opts...)
}

// Preview returns table preview data.
func (t *TableHandle) Preview(ctx context.Context, opts ...TableDataOption) (*CatalogTableDataResult, error) {
	return t.Service().PreviewTable(ctx, t.id, opts...)
}

// Download streams this table download.
func (t *TableHandle) Download(ctx context.Context) (*StreamResponse, error) {
	return t.Service().DownloadTable(ctx, t.id)
}

// FullPath returns the UI full path for this table.
func (t *TableHandle) FullPath(ctx context.Context) (*PathResult, error) {
	return t.Service().TableFullPath(ctx, t.id)
}

// Truncate clears this table's data.
func (t *TableHandle) Truncate(ctx context.Context) error {
	return t.Service().TruncateTable(ctx, t.id)
}

// Delete deletes this table.
func (t *TableHandle) Delete(ctx context.Context) error {
	return t.Service().DeleteTable(ctx, t.id)
}

// ID returns the volume id used for catalog API calls.
func (v *VolumeHandle) ID() int {
	return v.id
}

// Name returns the volume name.
func (v *VolumeHandle) Name() string {
	return v.name
}

// Service returns the route-aligned escape hatch catalog service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (v *VolumeHandle) Service() *CatalogService {
	return v.database.Service()
}

// Info returns product catalog metadata for this volume.
func (v *VolumeHandle) Info(ctx context.Context) (*VolumeInfoResult, error) {
	return v.Service().VolumeInfo(ctx, strconv.Itoa(v.id))
}

// Delete deletes this volume.
func (v *VolumeHandle) Delete(ctx context.Context) error {
	return v.Service().DeleteVolume(ctx, v.id)
}

// Rename updates this volume name and returns the same handle for chaining.
func (v *VolumeHandle) Rename(ctx context.Context, name string, opts ...CatalogInfoOption) (*VolumeHandle, error) {
	if _, err := v.Service().UpdateVolume(ctx, v.id, name, opts...); err != nil {
		return nil, err
	}
	v.name = name
	return v, nil
}

// FullPath returns the UI full path for this volume.
func (v *VolumeHandle) FullPath(ctx context.Context) (*PathResult, error) {
	return v.Service().VolumeFullPath(ctx, v.id)
}

// Folder returns a handle for a known folder id under this volume.
func (v *VolumeHandle) Folder(folderID int) (*FolderHandle, error) {
	if folderID == 0 {
		return nil, ErrIDRequired
	}
	return &FolderHandle{volume: v, id: folderID}, nil
}

// CreateFolder creates a folder under this volume and returns its handle.
func (v *VolumeHandle) CreateFolder(ctx context.Context, name string, opts ...CreateFolderOption) (*FolderHandle, *CreateFolderResult, error) {
	created, err := v.Service().CreateFolder(ctx, v.id, v.database.id, name, opts...)
	if err != nil {
		return nil, nil, err
	}
	return &FolderHandle{volume: v, id: int(created.GetId()), name: name}, created, nil
}

// UploadBytes uploads bytes and attaches the uploaded file to this volume.
func (v *VolumeHandle) UploadBytes(ctx context.Context, filename string, data []byte) (*UploadFileResult, error) {
	uploaded, err := v.Service().UploadBytes(ctx, filename, data)
	if err != nil {
		return nil, err
	}
	if err := v.Service().CreateFile(ctx, v.id, uploaded.GetFileId()); err != nil {
		return nil, err
	}
	return uploaded, nil
}

// UploadBytesHandle uploads bytes, attaches the file, and returns a file handle.
func (v *VolumeHandle) UploadBytesHandle(ctx context.Context, filename string, data []byte) (*FileHandle, error) {
	uploaded, err := v.UploadBytes(ctx, filename, data)
	if err != nil {
		return nil, err
	}
	return v.File(uploaded.GetFileId())
}

// UploadFile uploads a caller-supplied local file path and attaches it to this volume.
// It is a local file input convenience method, not AI file-discovery, local-filesystem probing, filename inference, or default upload entry point.
func (v *VolumeHandle) UploadFile(ctx context.Context, filePath string) (*UploadFileResult, error) {
	uploaded, err := v.Service().UploadFile(ctx, filePath)
	if err != nil {
		return nil, err
	}
	if err := v.Service().CreateFile(ctx, v.id, uploaded.GetFileId()); err != nil {
		return nil, err
	}
	return uploaded, nil
}

// UploadFileHandle uploads a caller-supplied local file path, attaches it, and returns a file handle.
// It is a local file input convenience method, not AI file-discovery, local-filesystem probing, filename inference, or default upload entry point; prefer UploadBytesHandle when bytes are already caller-owned.
func (v *VolumeHandle) UploadFileHandle(ctx context.Context, filePath string) (*FileHandle, error) {
	uploaded, err := v.UploadFile(ctx, filePath)
	if err != nil {
		return nil, err
	}
	return v.File(uploaded.GetFileId())
}

// UploadBytesPreview uploads bytes, attaches the file, and opens its preview stream.
func (v *VolumeHandle) UploadBytesPreview(ctx context.Context, filename string, data []byte) (*UploadedFilePreview, error) {
	file, err := v.UploadBytesHandle(ctx, filename, data)
	if err != nil {
		return nil, err
	}
	preview, err := file.Preview(ctx)
	if err != nil {
		return nil, err
	}
	return &UploadedFilePreview{File: file, Preview: preview}, nil
}

// UploadFilePreview uploads a caller-supplied local file path, attaches it, and opens its preview stream.
// It is a local file input convenience method, not AI file-discovery, local-filesystem probing, filename inference, or default upload entry point; prefer UploadBytesPreview when bytes are already caller-owned.
func (v *VolumeHandle) UploadFilePreview(ctx context.Context, filePath string) (*UploadedFilePreview, error) {
	file, err := v.UploadFileHandle(ctx, filePath)
	if err != nil {
		return nil, err
	}
	preview, err := file.Preview(ctx)
	if err != nil {
		return nil, err
	}
	return &UploadedFilePreview{File: file, Preview: preview}, nil
}

// File returns a handle for a known file id attached to this volume.
func (v *VolumeHandle) File(fileID string) (*FileHandle, error) {
	if fileID == "" {
		return nil, ErrFileIDRequired
	}
	return &FileHandle{volume: v, id: fileID}, nil
}

// Files lists files and folders scoped to this volume.
func (v *VolumeHandle) Files(ctx context.Context, opts ...FileListOption) (*CatalogFileListResult, error) {
	scopedOpts := append([]FileListOption{WithFileListFilter("volume_id", strconv.Itoa(v.id))}, opts...)
	return v.Service().Files(ctx, scopedOpts...)
}

// ID returns the folder id.
func (f *FolderHandle) ID() int {
	return f.id
}

// Name returns the folder name when this handle was created by name.
func (f *FolderHandle) Name() string {
	return f.name
}

// Rename updates this folder name and returns the same handle for chaining.
func (f *FolderHandle) Rename(ctx context.Context, name string, opts ...UpdateFolderOption) (*FolderHandle, error) {
	updateOpts := append([]UpdateFolderOption{WithFolderName(name)}, opts...)
	if err := f.volume.Service().UpdateFolder(ctx, f.id, updateOpts...); err != nil {
		return nil, err
	}
	f.name = name
	return f, nil
}

// Clean removes folder contents using backend catalog semantics.
func (f *FolderHandle) Clean(ctx context.Context) error {
	return f.volume.Service().CleanFolder(ctx, f.id)
}

// Delete deletes this folder.
func (f *FolderHandle) Delete(ctx context.Context) error {
	return f.volume.Service().DeleteFolder(ctx, f.id)
}

// ID returns the file id used for catalog file API calls.
func (f *FileHandle) ID() string {
	return f.id
}

// Info returns catalog file metadata for this file.
func (f *FileHandle) Info(ctx context.Context) (*FileInfoResult, error) {
	return f.volume.Service().FileInfo(ctx, f.id)
}

// Rename keeps the handle for chaining. Product catalog file update does not
// persist renames; this matches the Python facade and other catalog resources.
func (f *FileHandle) Rename(ctx context.Context, name string) (*FileHandle, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	_ = ctx
	return f, nil
}

// Download streams this file. Callers own the response body.
func (f *FileHandle) Download(ctx context.Context, opts ...DownloadFileOption) (*StreamResponse, error) {
	return f.volume.Service().DownloadFile(ctx, strconv.Itoa(f.volume.id), f.id, opts...)
}

// Preview streams this file preview. Callers own the response body.
func (f *FileHandle) Preview(ctx context.Context) (*StreamResponse, error) {
	return f.volume.Service().PreviewFile(ctx, strconv.Itoa(f.volume.id), f.id)
}

// TriggerWorkflows triggers workflows bound to this file's volume.
func (f *FileHandle) TriggerWorkflows(ctx context.Context) (*TriggerFileWorkflowResult, error) {
	return f.volume.Service().TriggerFileWorkflow(ctx, f.volume.id, f.id)
}

// DataAsset returns the workflow data asset derived from this file.
func (f *FileHandle) DataAsset(ctx context.Context) (*CatalogDataAssetResolveResult, error) {
	return f.volume.Service().FileDataAsset(ctx, f.id)
}

// Artifact returns parsed artifact metadata for this file.
func (f *FileHandle) Artifact(ctx context.Context) (*CatalogFileArtifactResult, error) {
	return f.volume.Service().FileArtifact(ctx, f.id)
}

// LineageOverview returns workflow artifact lineage for this catalog file.
func (f *FileHandle) LineageOverview(ctx context.Context, opts ...QueryOption) (*LineageOverview, error) {
	return f.volume.database.catalog.workspace.Workflows().CatalogFileLineageOverview(ctx, f.id, opts...)
}

// DeleteRef removes this file reference from the volume.
func (f *FileHandle) DeleteRef(ctx context.Context) error {
	return f.volume.Service().DeleteFileRef(ctx, f.volume.id, f.id)
}

// Delete deletes this file from the catalog for this volume.
func (f *FileHandle) Delete(ctx context.Context) error {
	return f.volume.Service().DeleteFile(ctx, f.id, strconv.Itoa(f.volume.id))
}

// ID returns the artifact id.
func (a *ArtifactHandle) ID() string {
	return a.id
}

// Service returns the route-aligned escape hatch workflow service scoped to this artifact's workspace. Prefer typed handle methods for user-facing workflows.
func (a *ArtifactHandle) Service() *WorkflowService {
	return a.workspace.Workflows()
}

// Overview returns lineage overview for this artifact.
func (a *ArtifactHandle) Overview(ctx context.Context) (*LineageOverview, error) {
	return a.Service().ArtifactLineageOverview(ctx, a.id)
}

// OutputBlocks lists output blocks for this artifact.
func (a *ArtifactHandle) OutputBlocks(ctx context.Context) (*LineageOutputBlocks, error) {
	return a.Service().ArtifactOutputBlocks(ctx, a.id)
}

// Block returns a handle for a known artifact output block.
func (a *ArtifactHandle) Block(blockID string) (*ArtifactBlockHandle, error) {
	if blockID == "" {
		return nil, ErrIDRequired
	}
	return &ArtifactBlockHandle{artifact: a, id: blockID}, nil
}

// SwitchEffectiveRevisions switches effective revisions for this artifact.
func (a *ArtifactHandle) SwitchEffectiveRevisions(ctx context.Context, switches []LineageRevisionSwitch, opts ...LineageSwitchOption) (*LineageEffectiveRevisions, error) {
	return a.Service().SwitchArtifactEffectiveRevisions(ctx, a.id, switches, opts...)
}

// Node returns a handle for a known artifact lineage node.
func (a *ArtifactHandle) Node(nodeID string) (*ArtifactNodeHandle, error) {
	if nodeID == "" {
		return nil, ErrIDRequired
	}
	return &ArtifactNodeHandle{artifact: a, id: nodeID}, nil
}

// ID returns the artifact output block id.
func (b *ArtifactBlockHandle) ID() string {
	return b.id
}

// Revisions lists revisions for this artifact output block.
func (b *ArtifactBlockHandle) Revisions(ctx context.Context) (*LineageBlockRevisions, error) {
	return b.artifact.Service().ArtifactBlockRevisions(ctx, b.artifact.id, b.id)
}

// CreateRevision creates a revision for this artifact output block.
func (b *ArtifactBlockHandle) CreateRevision(ctx context.Context, opts ...LineageRevisionOption) (*LineageBlockRevision, error) {
	return b.artifact.Service().CreateArtifactBlockRevision(ctx, b.artifact.id, b.id, opts...)
}

// ID returns the artifact lineage node id.
func (n *ArtifactNodeHandle) ID() string {
	return n.id
}

// Info returns this artifact lineage node.
func (n *ArtifactNodeHandle) Info(ctx context.Context) (*LineageArtifactNode, error) {
	return n.artifact.Service().ArtifactNode(ctx, n.artifact.id, n.id)
}

// OutputBlocks lists output blocks for this artifact lineage node.
func (n *ArtifactNodeHandle) OutputBlocks(ctx context.Context) (*LineageOutputBlocks, error) {
	return n.artifact.Service().ArtifactNodeOutputBlocks(ctx, n.artifact.id, n.id)
}

// Block returns a handle for a known output block on this artifact lineage node.
func (n *ArtifactNodeHandle) Block(blockID string) (*ArtifactNodeBlockHandle, error) {
	if blockID == "" {
		return nil, ErrIDRequired
	}
	return &ArtifactNodeBlockHandle{node: n, id: blockID}, nil
}

// SwitchEffectiveRevisions switches effective revisions for this artifact lineage node.
func (n *ArtifactNodeHandle) SwitchEffectiveRevisions(ctx context.Context, switches []LineageRevisionSwitch, opts ...LineageSwitchOption) (*LineageEffectiveRevisions, error) {
	return n.artifact.Service().SwitchArtifactNodeEffectiveRevisions(ctx, n.artifact.id, n.id, switches, opts...)
}

// Rerun reruns this artifact lineage node.
func (n *ArtifactNodeHandle) Rerun(ctx context.Context, opts ...LineageRerunOption) (*LineageRerun, error) {
	return n.artifact.Service().CreateArtifactNodeRerun(ctx, n.artifact.id, n.id, opts...)
}

// RerunTargetWithRevisions reruns this artifact lineage node for one target output emission.
func (n *ArtifactNodeHandle) RerunTargetWithRevisions(ctx context.Context, targetOutputEmissionID string, opts ...LineageRevisionRerunOption) (*LineageRerun, error) {
	return n.artifact.Service().CreateTargetArtifactNodeRevisionRerun(ctx, n.artifact.id, n.id, targetOutputEmissionID, opts...)
}

// ID returns the artifact node output block id.
func (b *ArtifactNodeBlockHandle) ID() string {
	return b.id
}

// Revisions lists revisions for this artifact node output block.
func (b *ArtifactNodeBlockHandle) Revisions(ctx context.Context) (*LineageBlockRevisions, error) {
	return b.node.artifact.Service().ArtifactNodeBlockRevisions(ctx, b.node.artifact.id, b.node.id, b.id)
}

// CreateRevision creates a revision for this artifact node output block.
func (b *ArtifactNodeBlockHandle) CreateRevision(ctx context.Context, opts ...LineageRevisionOption) (*LineageBlockRevision, error) {
	return b.node.artifact.Service().CreateArtifactNodeBlockRevision(ctx, b.node.artifact.id, b.node.id, b.id, opts...)
}

// ID returns the workflow app id.
func (wf *WorkflowHandle) ID() string {
	return wf.id
}

// Service returns the route-aligned escape hatch workflow service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (wf *WorkflowHandle) Service() *WorkflowService {
	return wf.workspace.Workflows()
}

// Info returns one UI-visible workflow app.
func (wf *WorkflowHandle) Info(ctx context.Context) (*WorkflowEnvelope, error) {
	return wf.Service().Get(ctx, wf.id)
}

// Update updates this workflow app.
func (wf *WorkflowHandle) Update(ctx context.Context, opts ...WorkflowUpdateOption) (*WorkflowEnvelope, error) {
	return wf.Service().Update(ctx, wf.id, opts...)
}

// Delete deletes this workflow app.
func (wf *WorkflowHandle) Delete(ctx context.Context) (*WorkflowDeleteEnvelope, error) {
	return wf.Service().Delete(ctx, wf.id)
}

// Run starts a workflow execution.
func (wf *WorkflowHandle) Run(ctx context.Context, opts ...WorkflowRunOption) (*WorkflowRunEnvelope, error) {
	return wf.Service().Run(ctx, wf.id, opts...)
}

// Pause pauses this workflow app.
func (wf *WorkflowHandle) Pause(ctx context.Context) (*WorkflowEnvelope, error) {
	return wf.Service().Pause(ctx, wf.id)
}

// Enable transitions this disabled workflow app to ready.
func (wf *WorkflowHandle) Enable(ctx context.Context) (*WorkflowEnvelope, error) {
	return wf.Service().Enable(ctx, wf.id)
}

// Resume resumes this workflow app.
func (wf *WorkflowHandle) Resume(ctx context.Context) (*WorkflowEnvelope, error) {
	return wf.Service().Resume(ctx, wf.id)
}

// Runs lists executions for this workflow app.
func (wf *WorkflowHandle) Runs(ctx context.Context, opts ...ListOption) (*WorkflowExecutionListEnvelope, error) {
	return wf.Service().ListRuns(ctx, wf.id, opts...)
}

// RunHandle returns a handle for a known execution id under this workflow.
func (wf *WorkflowHandle) RunHandle(executionID string) (*WorkflowRunHandle, error) {
	if executionID == "" {
		return nil, ErrIDRequired
	}
	return &WorkflowRunHandle{workflow: wf, id: executionID}, nil
}

// ID returns the workflow execution id.
func (run *WorkflowRunHandle) ID() string {
	return run.id
}

// Info returns this workflow execution.
func (run *WorkflowRunHandle) Info(ctx context.Context) (*WorkflowExecutionEnvelope, error) {
	if run.id == "" {
		return nil, ErrIDRequired
	}
	return run.workflow.Service().GetRun(ctx, run.workflow.id, run.id)
}

// Refresh refreshes this workflow execution envelope.
func (run *WorkflowRunHandle) Refresh(ctx context.Context) (*WorkflowExecutionEnvelope, error) {
	if run.id == "" {
		return nil, ErrIDRequired
	}
	return run.workflow.Service().RefreshRun(ctx, run.workflow.id, run.id)
}

// Pause pauses this workflow execution.
func (run *WorkflowRunHandle) Pause(ctx context.Context) (*WorkflowExecutionEnvelope, error) {
	if run.id == "" {
		return nil, ErrIDRequired
	}
	return run.workflow.Service().PauseRun(ctx, run.workflow.id, run.id)
}

// Resume resumes this workflow execution.
func (run *WorkflowRunHandle) Resume(ctx context.Context) (*WorkflowExecutionEnvelope, error) {
	if run.id == "" {
		return nil, ErrIDRequired
	}
	return run.workflow.Service().ResumeRun(ctx, run.workflow.id, run.id)
}

// Result returns this workflow execution result.
func (run *WorkflowRunHandle) Result(ctx context.Context) (*WorkflowExecutionResultEnvelope, error) {
	if run.id == "" {
		return nil, ErrIDRequired
	}
	return run.workflow.Service().GetRunResult(ctx, run.workflow.id, run.id)
}

// ImportContext returns this workflow execution import context.
func (run *WorkflowRunHandle) ImportContext(ctx context.Context) (*WorkflowImportContextEnvelope, error) {
	if run.id == "" {
		return nil, ErrIDRequired
	}
	return run.workflow.Service().GetRunImportContext(ctx, run.workflow.id, run.id)
}

// ImportFiles lists files imported by this workflow execution.
func (run *WorkflowRunHandle) ImportFiles(ctx context.Context, opts ...QueryOption) (*WorkflowImportFileListEnvelope, error) {
	if run.id == "" {
		return nil, ErrIDRequired
	}
	return run.workflow.Service().ListRunImportFiles(ctx, run.workflow.id, run.id, opts...)
}

// Node returns one node detail from this workflow execution.
func (run *WorkflowRunHandle) Node(ctx context.Context, nodeKey string) (*WorkflowExecutionNodeDetailEnvelope, error) {
	if run.id == "" {
		return nil, ErrIDRequired
	}
	return run.workflow.Service().GetRunNode(ctx, run.workflow.id, run.id, nodeKey)
}

// NodeHandle returns a handle for a known node key in this workflow execution.
func (run *WorkflowRunHandle) NodeHandle(nodeKey string) (*WorkflowRunNodeHandle, error) {
	if nodeKey == "" {
		return nil, ErrIDRequired
	}
	return &WorkflowRunNodeHandle{run: run, key: nodeKey}, nil
}

// RerunBranches lists rerun branches for this workflow execution.
func (run *WorkflowRunHandle) RerunBranches(ctx context.Context) (*WorkflowRerunBranchListEnvelope, error) {
	if run.id == "" {
		return nil, ErrIDRequired
	}
	return run.workflow.Service().ListRunRerunBranches(ctx, run.workflow.id, run.id)
}

// Rerun returns a handle for a known rerun id under this workflow execution.
func (run *WorkflowRunHandle) Rerun(rerunID string) (*WorkflowRunRerunHandle, error) {
	if rerunID == "" {
		return nil, ErrIDRequired
	}
	return &WorkflowRunRerunHandle{run: run, id: rerunID}, nil
}

// Retry retries this workflow execution.
func (run *WorkflowRunHandle) Retry(ctx context.Context, opts ...WorkflowRerunOption) (*WorkflowRunEnvelope, error) {
	if run.id == "" {
		return nil, ErrIDRequired
	}
	return run.workflow.Service().RetryRun(ctx, run.workflow.id, run.id, opts...)
}

// Cancel cancels this workflow execution.
func (run *WorkflowRunHandle) Cancel(ctx context.Context, opts ...WorkflowCancelOption) (*WorkflowExecutionEnvelope, error) {
	if run.id == "" {
		return nil, ErrIDRequired
	}
	return run.workflow.Service().CancelRun(ctx, run.workflow.id, run.id, opts...)
}

// Delete deletes this workflow execution.
func (run *WorkflowRunHandle) Delete(ctx context.Context) (*WorkflowDeleteEnvelope, error) {
	if run.id == "" {
		return nil, ErrIDRequired
	}
	return run.workflow.Service().DeleteRun(ctx, run.workflow.id, run.id)
}

// Key returns the workflow execution node key.
func (node *WorkflowRunNodeHandle) Key() string {
	return node.key
}

// Info returns this workflow execution node.
func (node *WorkflowRunNodeHandle) Info(ctx context.Context) (*WorkflowExecutionNodeDetailEnvelope, error) {
	return node.run.workflow.Service().GetRunNode(ctx, node.run.workflow.id, node.run.id, node.key)
}

// RerunPlan returns the rerun plan for this workflow execution node.
func (node *WorkflowRunNodeHandle) RerunPlan(ctx context.Context, opts ...QueryOption) (*WorkflowRerunPlanEnvelope, error) {
	return node.run.workflow.Service().GetRunNodeRerunPlan(ctx, node.run.workflow.id, node.run.id, node.key, opts...)
}

// CreateRerun creates a rerun for this workflow execution node.
func (node *WorkflowRunNodeHandle) CreateRerun(ctx context.Context, mode, sourceNodeExecutionID string, opts ...WorkflowRerunOption) (*WorkflowRerunEnvelope, error) {
	return node.run.workflow.Service().CreateRunNodeRerun(ctx, node.run.workflow.id, node.run.id, node.key, mode, sourceNodeExecutionID, opts...)
}

// ID returns the workflow rerun id.
func (rerun *WorkflowRunRerunHandle) ID() string {
	return rerun.id
}

// Info returns this workflow rerun branch.
func (rerun *WorkflowRunRerunHandle) Info(ctx context.Context) (*WorkflowRerunEnvelope, error) {
	return rerun.run.workflow.Service().GetRunRerun(ctx, rerun.run.workflow.id, rerun.run.id, rerun.id)
}

// Cancel cancels this workflow rerun branch.
func (rerun *WorkflowRunRerunHandle) Cancel(ctx context.Context, opts ...WorkflowCancelOption) (*WorkflowRerunEnvelope, error) {
	return rerun.run.workflow.Service().CancelRunRerun(ctx, rerun.run.workflow.id, rerun.run.id, rerun.id, opts...)
}

// ID returns the custom operator id.
func (o *CustomOperatorHandle) ID() string {
	return o.id
}

// Service returns the route-aligned escape hatch workflow service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (o *CustomOperatorHandle) Service() *WorkflowService {
	return o.workspace.Workflows()
}

// Info returns this workflow custom operator.
func (o *CustomOperatorHandle) Info(ctx context.Context) (*CustomOperator, error) {
	return o.Service().GetCustomOperator(ctx, o.id)
}

// Code returns this workflow custom operator source payload.
func (o *CustomOperatorHandle) Code(ctx context.Context) (*CustomOperatorCodeEnvelope, error) {
	return o.Service().GetCustomOperatorCode(ctx, o.id)
}

// Test starts a custom operator test run.
func (o *CustomOperatorHandle) Test(ctx context.Context, input map[string]any, waitTimeoutSeconds ...int) (*CustomOperatorTestEnvelope, error) {
	var timeout *int
	if len(waitTimeoutSeconds) > 0 {
		timeout = &waitTimeoutSeconds[0]
	}
	return o.Service().TestCustomOperator(ctx, o.id, CustomOperatorTestInput{Input: input, WaitTimeoutSeconds: timeout})
}

// TestWithInput starts a custom operator test run from an explicit product test input.
func (o *CustomOperatorHandle) TestWithInput(ctx context.Context, input CustomOperatorTestInput) (*CustomOperatorTestEnvelope, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	return o.Service().TestCustomOperator(ctx, o.id, input)
}

// Update updates this workflow custom operator.
func (o *CustomOperatorHandle) Update(ctx context.Context, opts ...CustomOperatorUpdateOption) (*CustomOperator, error) {
	return o.Service().UpdateCustomOperator(ctx, o.id, opts...)
}

// Enable enables this workflow custom operator.
func (o *CustomOperatorHandle) Enable(ctx context.Context) (*CustomOperator, error) {
	return o.Service().EnableCustomOperator(ctx, o.id)
}

// Disable disables this workflow custom operator.
func (o *CustomOperatorHandle) Disable(ctx context.Context) (*CustomOperator, error) {
	return o.Service().DisableCustomOperator(ctx, o.id)
}

// Delete deletes this workflow custom operator.
func (o *CustomOperatorHandle) Delete(ctx context.Context) (*WorkflowDeleteEnvelope, error) {
	return o.Service().DeleteCustomOperator(ctx, o.id)
}

// NodeID returns the catalog WorkItem node id.
func (s *WorkitemAPIServiceHandle) NodeID() string {
	return s.nodeID
}

// Version returns the catalog WorkItem version.
func (s *WorkitemAPIServiceHandle) Version() string {
	return s.version
}

// Service returns the route-aligned escape hatch workflow service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (s *WorkitemAPIServiceHandle) Service() *WorkflowService {
	return s.workspace.Workflows()
}

// Info returns the API service info for this catalog WorkItem version.
func (s *WorkitemAPIServiceHandle) Info(ctx context.Context) (*WorkitemAPIServiceEnvelope, error) {
	return s.Service().GetWorkitemAPIService(ctx, s.nodeID, s.version)
}

// Publish publishes or updates this catalog WorkItem API service.
func (s *WorkitemAPIServiceHandle) Publish(ctx context.Context, spec WorkitemAPIServicePublishSpec) (*WorkitemAPIServiceEnvelope, error) {
	return s.Service().PublishWorkitemAPIService(ctx, s.nodeID, spec)
}

// Enable enables this catalog WorkItem API service.
func (s *WorkitemAPIServiceHandle) Enable(ctx context.Context) (*WorkitemAPIServiceEnvelope, error) {
	return s.Service().EnableWorkitemAPIService(ctx, s.nodeID, s.version)
}

// Disable disables this catalog WorkItem API service.
func (s *WorkitemAPIServiceHandle) Disable(ctx context.Context) (*WorkitemAPIServiceEnvelope, error) {
	return s.Service().DisableWorkitemAPIService(ctx, s.nodeID, s.version)
}

// Invoke invokes this catalog WorkItem API service.
func (s *WorkitemAPIServiceHandle) Invoke(ctx context.Context, spec WorkitemAPIServiceInvokeSpec) (*WorkitemAPIServiceInvokeEnvelope, error) {
	return s.Service().InvokeWorkitemAPIService(ctx, s.nodeID, spec)
}

// ID returns the workflow template id.
func (t *WorkflowTemplateHandle) ID() string {
	return t.id
}

// Service returns the route-aligned escape hatch workflow service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (t *WorkflowTemplateHandle) Service() *WorkflowService {
	return t.workspace.Workflows()
}

// Info returns this workflow template.
func (t *WorkflowTemplateHandle) Info(ctx context.Context) (*WorkflowTemplate, error) {
	return t.Service().GetWorkflowTemplate(ctx, t.id)
}

// Update updates this workflow template.
func (t *WorkflowTemplateHandle) Update(ctx context.Context, name, dslYAML string, opts ...WorkflowTemplateOption) (*WorkflowTemplate, error) {
	return t.Service().UpdateWorkflowTemplate(ctx, t.id, name, dslYAML, opts...)
}

// UpdateMOWL replaces this template's MOWL publication graph.
func (t *WorkflowTemplateHandle) UpdateMOWL(ctx context.Context, name string, workflow map[string]any, opts ...WorkflowTemplateOption) (*WorkflowTemplate, error) {
	return t.Service().UpdateMOWLWorkflowTemplate(ctx, t.id, name, workflow, opts...)
}

// Delete deletes this workflow template.
func (t *WorkflowTemplateHandle) Delete(ctx context.Context) (*WorkflowTemplateDeleteEnvelope, error) {
	return t.Service().DeleteWorkflowTemplate(ctx, t.id)
}

// ID returns the semantic model id.
func (m *SemanticModelHandle) ID() string {
	return m.id
}

// Service returns the route-aligned escape hatch knowledge service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (m *SemanticModelHandle) Service() *KnowledgeService {
	return m.workspace.Knowledge()
}

// Info returns this semantic model.
func (m *SemanticModelHandle) Info(ctx context.Context) (*SemanticModelInfo, error) {
	return m.Service().Get(ctx, m.id)
}

// Update updates this knowledge base. name must equal its existing immutable name.
func (m *SemanticModelHandle) Update(ctx context.Context, name string, opts ...SemanticModelOption) (*SemanticMutationResult, error) {
	return m.Service().Update(ctx, m.id, name, opts...)
}

// Delete deletes this semantic model.
func (m *SemanticModelHandle) Delete(ctx context.Context) (*SemanticMutationResult, error) {
	return m.Service().Delete(ctx, m.id)
}

// Entries lists entries in this semantic model.
func (m *SemanticModelHandle) Entries(ctx context.Context, opts ...QueryOption) (*SemanticEntryListResult, error) {
	return m.Service().ListEntries(ctx, m.id, opts...)
}

// CreateEntry creates one semantic model entry and returns a handle for the backend-created entry id.
func (m *SemanticModelHandle) CreateEntry(ctx context.Context, entry SemanticEntryInput) (*SemanticEntryHandle, *SemanticEntry, error) {
	if err := entry.Validate(); err != nil {
		return nil, nil, err
	}
	result, err := m.Service().CreateEntry(ctx, m.id, entry)
	if err != nil {
		return nil, nil, err
	}
	entryID, err := createdIntPathID(result.GetId(), "semantic entry")
	if err != nil {
		return nil, nil, err
	}
	return &SemanticEntryHandle{model: m, id: entryID}, result, nil
}

// Entry returns a handle for a known semantic model entry id.
func (m *SemanticModelHandle) Entry(entryID string) (*SemanticEntryHandle, error) {
	if entryID == "" {
		return nil, ErrIDRequired
	}
	return &SemanticEntryHandle{model: m, id: entryID}, nil
}

// Import imports semantic model entries.
func (m *SemanticModelHandle) Import(ctx context.Context, entries []SemanticEntryInput) (*SemanticImportResult, error) {
	if len(entries) == 0 {
		return nil, ErrEntriesRequired
	}
	for _, entry := range entries {
		if err := entry.Validate(); err != nil {
			return nil, err
		}
	}
	return m.Service().Import(ctx, m.id, entries)
}

// Export exports semantic model entries.
func (m *SemanticModelHandle) Export(ctx context.Context) (*SemanticExportResult, error) {
	return m.Service().Export(ctx, m.id)
}

// Validate validates this semantic model.
func (m *SemanticModelHandle) Validate(ctx context.Context) (*SemanticValidateResult, error) {
	return m.Service().Validate(ctx, m.id)
}

// ID returns the semantic entry id.
func (e *SemanticEntryHandle) ID() string {
	return e.id
}

// Model returns this entry's semantic model handle.
func (e *SemanticEntryHandle) Model() *SemanticModelHandle {
	return e.model
}

// Update updates this semantic model entry.
func (e *SemanticEntryHandle) Update(ctx context.Context, entry SemanticEntryInput) (*SemanticMutationResult, error) {
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	return e.model.Service().UpdateEntry(ctx, e.model.id, e.id, entry)
}

// Delete deletes this semantic model entry.
func (e *SemanticEntryHandle) Delete(ctx context.Context) (*SemanticMutationResult, error) {
	return e.model.Service().DeleteEntry(ctx, e.model.id, e.id)
}

// ID returns the connector id.
func (c *ConnectorHandle) ID() string {
	return c.id
}

// Service returns the route-aligned escape hatch connector service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (c *ConnectorHandle) Service() *ConnectorService {
	return c.workspace.Connectors()
}

// Info returns this connector definition.
func (c *ConnectorHandle) Info(ctx context.Context) (*Connector, error) {
	return c.Service().Get(ctx, c.id)
}

// Update updates this connector definition.
func (c *ConnectorHandle) Update(ctx context.Context, opts ...ConnectorUpdateOption) error {
	return c.Service().Update(ctx, c.id, opts...)
}

// Delete deletes this connector definition.
func (c *ConnectorHandle) Delete(ctx context.Context) error {
	return c.Service().Delete(ctx, c.id)
}

// PreviewFile previews a source file through this connector.
func (c *ConnectorHandle) PreviewFile(ctx context.Context, uri string, opts ...ConnectorPreviewOption) (*PreviewFileResult, error) {
	return c.Service().PreviewFile(ctx, c.id, uri, opts...)
}

// File returns a handle for a temporary file associated with this connector.
func (c *ConnectorHandle) File(connFileID string) (*ConnectorFileHandle, error) {
	if connFileID == "" {
		return nil, ErrConnectorFileIDRequired
	}
	return &ConnectorFileHandle{workspace: c.workspace, connector: c, id: connFileID}, nil
}

// SourceDatabases lists source databases for this connector.
func (c *ConnectorHandle) SourceDatabases(ctx context.Context) (*DBListDatabasesResult, error) {
	return c.Service().DBListDatabases(ctx, c.id)
}

// SourceSchemas lists source schemas for this connector.
func (c *ConnectorHandle) SourceSchemas(ctx context.Context, database string) (*DBListSchemasResult, error) {
	return c.Service().DBListSchemas(ctx, c.id, database)
}

// SourceTables lists source tables for this connector.
func (c *ConnectorHandle) SourceTables(ctx context.Context, database string, opts ...ConnectorDBOption) (*DBListTablesResult, error) {
	return c.Service().DBListTables(ctx, c.id, database, opts...)
}

// SourceSchema returns source table schema through this connector.
func (c *ConnectorHandle) SourceSchema(ctx context.Context, database, table string, opts ...ConnectorDBOption) (*DBSourceSchemaResult, error) {
	return c.Service().DBSourceSchema(ctx, c.id, database, table, opts...)
}

// MODatabases lists MatrixOne target databases for this connector.
func (c *ConnectorHandle) MODatabases(ctx context.Context) (*MODatabaseListResult, error) {
	return c.Service().MODatabaseList(ctx, c.id)
}

// MODatabaseTables lists MatrixOne target tables for this connector.
func (c *ConnectorHandle) MODatabaseTables(ctx context.Context, database string) (*MODatabaseTableListResult, error) {
	return c.Service().MODatabaseTableList(ctx, c.id, database)
}

// CheckMOTableName checks whether a MatrixOne target table name is available through this connector.
func (c *ConnectorHandle) CheckMOTableName(ctx context.Context, database, table string) (*MOCheckTableNameResult, error) {
	return c.Service().MOCheckTableName(ctx, c.id, database, table)
}

// MOColumnMapping returns MatrixOne column mapping suggestions through this connector.
func (c *ConnectorHandle) MOColumnMapping(ctx context.Context, database, table string, exportColumns map[string]string) (*MOColumnMappingResult, error) {
	return c.Service().MOColumnMapping(ctx, c.id, database, table, exportColumns)
}

// CreateImportTask creates an import task and returns a handle for the backend task id.
func (c *ConnectorHandle) CreateImportTask(ctx context.Context, configType int, sourceConfig map[string]any, opts ...CreateImportTaskOption) (*ImportTaskHandle, *ImportTaskCreateResult, error) {
	options := createImportTaskOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	spec := ImportTaskCreateSpec{
		SourceConnectorID: c.id,
		ConfigType:        configType,
		SourceConfig:      sourceConfig,
	}
	if options.name != nil {
		spec.Name = *options.name
	}
	if options.volumeID != nil {
		spec.VolumeID = *options.volumeID
	}
	result, err := c.Service().CreateTask(ctx, spec)
	if err != nil {
		return nil, nil, err
	}
	return &ImportTaskHandle{connector: c, id: result.GetTaskId()}, result, nil
}

// CreateFileImportTask creates a file import task from an explicit file import spec.
func (c *ConnectorHandle) CreateFileImportTask(ctx context.Context, spec FileImportTaskSpec) (*ImportTaskHandle, *ImportTaskCreateResult, error) {
	if err := spec.Validate(); err != nil {
		return nil, nil, err
	}
	createSpec, err := spec.ImportTaskCreateSpec(c.id)
	if err != nil {
		return nil, nil, err
	}
	result, err := c.Service().CreateTask(ctx, createSpec)
	if err != nil {
		return nil, nil, err
	}
	return &ImportTaskHandle{connector: c, id: result.GetTaskId()}, result, nil
}

// ImportTask returns a handle for a known import task id.
func (c *ConnectorHandle) ImportTask(taskID string) (*ImportTaskHandle, error) {
	if taskID == "" {
		return nil, ErrTaskIDRequired
	}
	return &ImportTaskHandle{connector: c, id: taskID}, nil
}

// CreateExportTask creates an export task and returns a handle for the backend task id.
func (c *ConnectorHandle) CreateExportTask(ctx context.Context, taskName, connectorName string, exportType int, config ExportTaskConfig, files []ExportTaskFile) (*ExportTaskHandle, *ExportTask, error) {
	taskFiles := make([]ExportTaskFile, 0, len(files))
	for _, file := range files {
		taskFiles = append(taskFiles, file)
	}
	result, err := c.Service().CreateExportTask(ctx, ExportTaskSpec{
		ConnectorID:   c.id,
		Name:          taskName,
		ConnectorName: connectorName,
		Type:          exportType,
		Config:        config,
		Files:         taskFiles,
	})
	if err != nil {
		return nil, nil, err
	}
	return &ExportTaskHandle{connector: c, id: result.GetId()}, result, nil
}

// CreateExportTaskFromSpec creates an export task from an explicit export spec.
func (c *ConnectorHandle) CreateExportTaskFromSpec(ctx context.Context, spec ExportTaskSpec) (*ExportTaskHandle, *ExportTask, error) {
	spec.ConnectorID = c.id
	if err := spec.Validate(); err != nil {
		return nil, nil, err
	}
	result, err := c.Service().CreateExportTask(ctx, spec)
	if err != nil {
		return nil, nil, err
	}
	return &ExportTaskHandle{connector: c, id: result.GetId()}, result, nil
}

// ExportTask returns a handle for a known export task id.
func (c *ConnectorHandle) ExportTask(taskID string) (*ExportTaskHandle, error) {
	if taskID == "" {
		return nil, ErrTaskIDRequired
	}
	return &ExportTaskHandle{connector: c, id: taskID}, nil
}

// ID returns the temporary connector file id.
func (f *ConnectorFileHandle) ID() string {
	return f.id
}

// Service returns the route-aligned escape hatch connector service scoped to this file's workspace. Prefer typed handle methods for user-facing workflows.
func (f *ConnectorFileHandle) Service() *ConnectorService {
	return f.workspace.Connectors()
}

// Preview previews this temporary connector file by conn_file_id.
func (f *ConnectorFileHandle) Preview(ctx context.Context, opts ...ConnectorPreviewOption) (*PreviewFileResult, error) {
	scoped := append([]ConnectorPreviewOption{WithConnectorPreviewConnFileID(f.id)}, opts...)
	if f.connector != nil {
		return f.connector.PreviewFile(ctx, "", scoped...)
	}
	return f.Service().PreviewFile(ctx, "", "", scoped...)
}

// Download returns a temporary connector file download URL.
func (f *ConnectorFileHandle) Download(ctx context.Context) (*FileDownloadResult, error) {
	return f.Service().FileDownload(ctx, f.id)
}

// Stream streams this temporary connector file. Callers own the response body.
func (f *ConnectorFileHandle) Stream(ctx context.Context) (*StreamResponse, error) {
	return f.Service().FileStream(ctx, f.id)
}

// Delete deletes this temporary connector file.
func (f *ConnectorFileHandle) Delete(ctx context.Context) error {
	return f.Service().FileDelete(ctx, f.id)
}

// ID returns the import task id.
func (t *ImportTaskHandle) ID() string {
	return t.id
}

// Info returns this import task.
func (t *ImportTaskHandle) Info(ctx context.Context) (*ImportTaskInfoResult, error) {
	return t.connector.Service().GetTask(ctx, t.id)
}

// Update updates this import task.
func (t *ImportTaskHandle) Update(ctx context.Context, spec ImportTaskUpdateSpec) error {
	return t.connector.Service().UpdateTask(ctx, t.id, spec)
}

// UpdateConfig updates this import task from an explicit typed spec.
func (t *ImportTaskHandle) UpdateConfig(ctx context.Context, spec ImportTaskUpdateSpec) error {
	return t.connector.Service().UpdateTaskFromSpec(ctx, t.id, spec)
}

// Pause pauses this import task.
func (t *ImportTaskHandle) Pause(ctx context.Context) error {
	return t.connector.Service().PauseTask(ctx, t.id)
}

// Resume resumes this import task.
func (t *ImportTaskHandle) Resume(ctx context.Context) error {
	return t.connector.Service().ResumeTask(ctx, t.id)
}

// Runs lists this import task's runs.
func (t *ImportTaskHandle) Runs(ctx context.Context) (*ImportTaskRunListResult, error) {
	return t.connector.Service().TaskRuns(ctx, t.id)
}

// Files lists import task files filtered to this task id.
func (t *ImportTaskHandle) Files(ctx context.Context, opts ...QueryOption) (*ImportTaskFileListResult, error) {
	taskOpts := append([]QueryOption{WithQueryParam("task_id", t.id)}, opts...)
	return t.connector.Service().TaskFiles(ctx, taskOpts...)
}

// RetryFiles retries failed files in this import task.
func (t *ImportTaskHandle) RetryFiles(ctx context.Context, ids ...string) error {
	return t.connector.Service().RetryTaskFiles(ctx, t.id, ids...)
}

// Delete deletes this import task.
func (t *ImportTaskHandle) Delete(ctx context.Context) error {
	return t.connector.Service().DeleteTask(ctx, t.id)
}

// ID returns the export task id.
func (t *ExportTaskHandle) ID() string {
	return t.id
}

// Info returns this export task detail (backend ExportTaskInfo wire).
func (t *ExportTaskHandle) Info(ctx context.Context) (*ExportTaskInfo, error) {
	return t.connector.Service().ExportTaskInfo(ctx, t.id)
}

// Files lists this export task's files.
func (t *ExportTaskHandle) Files(ctx context.Context, opts ...ExportTaskFilesOption) (*ExportTaskFilesResult, error) {
	return t.connector.Service().ExportTaskFiles(ctx, t.id, opts...)
}

// Retry retries files for this export task.
func (t *ExportTaskHandle) Retry(ctx context.Context, fileIDs ...string) error {
	return t.connector.Service().RetryExportTask(ctx, t.id, fileIDs...)
}

// State returns this export task's runtime state.
func (t *ExportTaskHandle) State(ctx context.Context) (*ExportTaskStateResult, error) {
	return t.connector.Service().ExportTaskState(ctx, t.id)
}

// Delete deletes this export task.
func (t *ExportTaskHandle) Delete(ctx context.Context) error {
	return t.connector.Service().DeleteExportTask(ctx, t.id)
}

// LLMProvider returns a user-story handle for LLM backend and router operations.
func (w *WorkspaceHandle) LLMProvider() *ModelProviderHandle {
	return &ModelProviderHandle{workspace: w, service: w.Models().LLMBackends(), kind: "llm"}
}

// EmbeddingProvider returns a user-story handle for embedding backend and router operations.
func (w *WorkspaceHandle) EmbeddingProvider() *ModelProviderHandle {
	return &ModelProviderHandle{workspace: w, service: w.Models().EmbeddingBackends(), kind: "embedding"}
}

// ParserProvider returns a user-story handle for parser backend and router operations.
func (w *WorkspaceHandle) ParserProvider() *ModelProviderHandle {
	return &ModelProviderHandle{workspace: w, service: w.Models().ParserBackends(), kind: "parser"}
}

// LLMBackend returns a handle for a known LLM backend id.
func (w *WorkspaceHandle) LLMBackend(backendID string) (*ProviderBackendHandle, error) {
	return w.LLMProvider().Backend(backendID)
}

// EmbeddingBackend returns a handle for a known embedding backend id.
func (w *WorkspaceHandle) EmbeddingBackend(backendID string) (*ProviderBackendHandle, error) {
	return w.EmbeddingProvider().Backend(backendID)
}

// ParserBackend returns a handle for a known parser backend id.
func (w *WorkspaceHandle) ParserBackend(backendID string) (*ProviderBackendHandle, error) {
	return w.ParserProvider().Backend(backendID)
}

// SetupLLMBackend creates an LLM backend with endpoint and returns a handle for the created backend.
func (w *WorkspaceHandle) SetupLLMBackend(ctx context.Context, name, address string, models []string, opts ...ProviderBackendOption) (*ProviderBackendHandle, *ProviderSetupBackendResult, error) {
	if name == "" {
		return nil, nil, ErrNameRequired
	}
	if address == "" {
		return nil, nil, ErrAddressRequired
	}
	if len(models) == 0 {
		return nil, nil, ErrModelsRequired
	}
	result, err := w.Models().SetupLLMBackend(ctx, name, address, models, opts...)
	if err != nil {
		return nil, nil, err
	}
	if result.GetBackend().GetId() == 0 {
		return nil, nil, &Error{Code: "ErrResponseInvalid", Message: "provider setup backend data missing backend id"}
	}
	return &ProviderBackendHandle{provider: w.LLMProvider(), id: strconv.FormatInt(result.GetBackend().GetId(), 10)}, result, nil
}

// Kind returns the provider family name: llm, embedding, or parser.
func (p *ModelProviderHandle) Kind() string {
	return p.kind
}

// Service returns the route-aligned escape hatch provider backend service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (p *ModelProviderHandle) Service() *ProviderBackendService {
	return p.service
}

// Backends lists configured provider backends in this family.
func (p *ModelProviderHandle) Backends(ctx context.Context) (*ProviderBackendListResult, error) {
	return p.service.List(ctx)
}

// CreateBackend creates a provider backend and returns a handle for the created backend.
func (p *ModelProviderHandle) CreateBackend(ctx context.Context, name string, opts ...ProviderBackendOption) (*ProviderBackendHandle, *ProviderBackend, error) {
	if name == "" {
		return nil, nil, ErrNameRequired
	}
	result, err := p.service.Create(ctx, name, opts...)
	if err != nil {
		return nil, nil, err
	}
	if result.GetId() == 0 {
		return nil, nil, &Error{Code: "ErrResponseInvalid", Message: "provider backend data missing id"}
	}
	return &ProviderBackendHandle{provider: p, id: strconv.FormatInt(result.GetId(), 10)}, result, nil
}

// Backend returns a handle for a known provider backend id.
func (p *ModelProviderHandle) Backend(backendID string) (*ProviderBackendHandle, error) {
	if backendID == "" {
		return nil, ErrIDRequired
	}
	return &ProviderBackendHandle{provider: p, id: backendID}, nil
}

// Router returns a handle for this provider family's router config.
func (p *ModelProviderHandle) Router() *ProviderRouterHandle {
	return &ProviderRouterHandle{provider: p}
}

// ProbeAddress probes a remote endpoint before creating a configured backend.
func (p *ModelProviderHandle) ProbeAddress(ctx context.Context, address string, opts ...ProviderBackendOption) (*ProviderProbeModelsResult, error) {
	switch p.kind {
	case "llm":
		return p.workspace.Models().ProbeLLMModels(ctx, address, opts...)
	case "embedding":
		return p.workspace.Models().ProbeEmbeddingModels(ctx, address, opts...)
	default:
		return nil, &Error{
			Code:    "ErrUnsupportedOperation",
			Message: "provider kind does not support address probe: " + p.kind,
		}
	}
}

// ID returns the provider backend id.
func (b *ProviderBackendHandle) ID() string {
	return b.id
}

// Provider returns the provider family handle for this backend.
func (b *ProviderBackendHandle) Provider() *ModelProviderHandle {
	return b.provider
}

// Service returns the route-aligned escape hatch provider backend service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (b *ProviderBackendHandle) Service() *ProviderBackendService {
	return b.provider.Service()
}

// Info returns this provider backend.
func (b *ProviderBackendHandle) Info(ctx context.Context) (*ProviderBackend, error) {
	return b.Service().Get(ctx, b.id)
}

// Update updates this provider backend.
func (b *ProviderBackendHandle) Update(ctx context.Context, opts ...ProviderBackendUpdateOption) (*ProviderBackend, error) {
	return b.Service().Update(ctx, b.id, opts...)
}

// Delete deletes this provider backend.
func (b *ProviderBackendHandle) Delete(ctx context.Context) (*ProviderMutationResult, error) {
	return b.Service().Delete(ctx, b.id)
}

// CreateEndpoint creates an endpoint under this provider backend.
func (b *ProviderBackendHandle) CreateEndpoint(ctx context.Context, address string) (*ProviderEndpoint, error) {
	if address == "" {
		return nil, ErrAddressRequired
	}
	return b.Service().CreateEndpoint(ctx, b.id, address)
}

// Endpoints lists endpoints under this provider backend.
func (b *ProviderBackendHandle) Endpoints(ctx context.Context) (*ProviderEndpointListResult, error) {
	return b.Service().ListEndpoints(ctx, b.id)
}

// SetEndpointStatus updates one endpoint status under this provider backend.
func (b *ProviderBackendHandle) SetEndpointStatus(ctx context.Context, endpointID, status string) (*ProviderMutationResult, error) {
	if endpointID == "" {
		return nil, ErrIDRequired
	}
	if status == "" {
		return nil, ErrStatusRequired
	}
	return b.Service().SetEndpointStatus(ctx, b.id, endpointID, status)
}

// ProbeModels probes models through this configured provider backend.
func (b *ProviderBackendHandle) ProbeModels(ctx context.Context, opts ...ProviderBackendOption) (*ProviderProbeModelsResult, error) {
	return b.Service().ProbeBackendModels(ctx, b.id, opts...)
}

// Provider returns the provider family handle for this router.
func (r *ProviderRouterHandle) Provider() *ModelProviderHandle {
	return r.provider
}

// Info returns this provider router config.
func (r *ProviderRouterHandle) Info(ctx context.Context) (*ProviderRouterConfig, error) {
	return r.provider.Service().Router(ctx)
}

// Update updates this provider router config.
func (r *ProviderRouterHandle) Update(ctx context.Context, opts ...RouterOption) (*ProviderRouterConfig, error) {
	return r.provider.Service().UpdateRouter(ctx, opts...)
}

// ComputeResource returns a handle for a known compute resource id.
func (w *WorkspaceHandle) ComputeResource(resourceID string) (*ComputeResourceHandle, error) {
	if resourceID == "" {
		return nil, ErrIDRequired
	}
	return &ComputeResourceHandle{workspace: w, id: resourceID}, nil
}

// CreateComputeResource creates a compute resource and returns a handle for the backend-created resource id.
func (w *WorkspaceHandle) CreateComputeResource(ctx context.Context, name, specID string, cpu, memoryGiB int, opts ...ComputeCreateOption) (*ComputeResourceHandle, *ComputeResource, error) {
	result, err := w.Compute().Create(ctx, name, specID, cpu, memoryGiB, opts...)
	if err != nil {
		return nil, nil, err
	}
	resourceID, err := createdStringPathID(result.GetId(), "compute resource")
	if err != nil {
		return nil, nil, err
	}
	return &ComputeResourceHandle{workspace: w, id: resourceID}, result, nil
}

// Service returns the route-aligned escape hatch compute service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (r *ComputeResourceHandle) Service() *ComputeService {
	return r.workspace.Compute()
}

// ID returns the compute resource id.
func (r *ComputeResourceHandle) ID() string {
	return r.id
}

// Info returns this compute resource.
func (r *ComputeResourceHandle) Info(ctx context.Context) (*ComputeResource, error) {
	return r.Service().Get(ctx, r.id)
}

// Runtime returns persisted Worker-type runtime rows for this compute resource.
func (r *ComputeResourceHandle) Runtime(ctx context.Context) ([]*ComputeWorkerRuntime, error) {
	return r.Service().Runtime(ctx, r.id)
}

// Update updates this compute resource.
func (r *ComputeResourceHandle) Update(ctx context.Context, opts ...ComputeUpdateOption) (*ComputeResource, error) {
	return r.Service().Update(ctx, r.id, opts...)
}

// AdoptLegacyOwnership explicitly records canonical Ownership for this
// historical compute resource through the audited repair API.
func (r *ComputeResourceHandle) AdoptLegacyOwnership(ctx context.Context) (*ComputeResourceOwnership, error) {
	return r.Service().AdoptLegacyOwnership(ctx, r.id)
}

// PreflightDelete checks whether this compute resource can be deleted.
func (r *ComputeResourceHandle) PreflightDelete(ctx context.Context) (*ComputeDeletePreflightResult, error) {
	return r.Service().PreflightDelete(ctx, r.id)
}

// Delete deletes this compute resource.
func (r *ComputeResourceHandle) Delete(ctx context.Context, opts ...QueryOption) error {
	return r.Service().Delete(ctx, r.id, opts...)
}

// Suspend suspends this compute resource.
func (r *ComputeResourceHandle) Suspend(ctx context.Context) (*ComputeResource, error) {
	return r.Service().Suspend(ctx, r.id)
}

// Resume resumes this compute resource.
func (r *ComputeResourceHandle) Resume(ctx context.Context) (*ComputeResource, error) {
	return r.Service().Resume(ctx, r.id)
}

// Retry retries this compute resource.
func (r *ComputeResourceHandle) Retry(ctx context.Context) (*ComputeResource, error) {
	return r.Service().Retry(ctx, r.id)
}

// ConsumerMemory returns a handle for one consumer subject's memories and profile.
// The subject id is the handle's stable identity.
//
// Deprecated: Current MOI does not serve the legacy consumer-memory/profile API.
func (w *WorkspaceHandle) ConsumerMemory(subjectID string) (*ConsumerMemoryHandle, error) {
	if subjectID == "" {
		return nil, ErrSubjectRequired
	}
	return &ConsumerMemoryHandle{workspace: w, id: subjectID}, nil
}

// ConsumerMemoryHandle is a typed handle scoped to one consumer subject's
// memories and rendered profile. Its id is the consumer subject id.
//
// Deprecated: This handle requires a server implementing the legacy consumer API.
type ConsumerMemoryHandle struct {
	workspace *WorkspaceHandle
	id        string
}

// ConsumerMemoryFulltextSearchInput configures keyword retrieval for one
// ConsumerMemoryHandle. Subject scope is supplied by the handle.
type ConsumerMemoryFulltextSearchInput struct {
	Query               string
	TopK                int64
	SessionID           string
	MemoryTypes         []string
	TrustTier           string
	Branch              string
	ExtraMetadataFilter map[string]any
}

// Validate checks the Product API keyword and metadata-filter contract.
func (i ConsumerMemoryFulltextSearchInput) Validate() error {
	if strings.TrimSpace(i.Query) == "" {
		return ErrQueryRequired
	}
	if i.TopK < 0 || i.TopK > 100 {
		return &Error{Code: "ErrInvalidArgument", Message: "top_k must be between 0 and 100"}
	}
	return validateMemoryMetadataFilter(i.ExtraMetadataFilter)
}

// ConsumerMemoryStructuredQueryInput configures exact structured retrieval for
// one ConsumerMemoryHandle. Subject scope is supplied by the handle.
type ConsumerMemoryStructuredQueryInput struct {
	Limit               int64
	Cursor              string
	SessionID           string
	MemoryTypes         []string
	TrustTier           string
	Branch              string
	ExtraMetadataFilter map[string]any
}

// Validate checks the Product API structured-query and metadata contract.
func (i ConsumerMemoryStructuredQueryInput) Validate() error {
	if i.Limit < 0 || i.Limit > 500 {
		return &Error{Code: "ErrInvalidArgument", Message: "limit must be between 0 and 500"}
	}
	return validateMemoryMetadataFilter(i.ExtraMetadataFilter)
}

// Service returns the route-aligned escape hatch consumer memory service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (r *ConsumerMemoryHandle) Service() *MemoryService {
	return r.workspace.Memory()
}

// ID returns the consumer subject id this handle is scoped to.
func (r *ConsumerMemoryHandle) ID() string {
	return r.id
}

// Retrieve retrieves this subject's memories for the given query.
func (r *ConsumerMemoryHandle) Retrieve(ctx context.Context, query string) (*MemoryListResult, error) {
	return r.Service().Retrieve(ctx, MemoryQuery{Query: query, SubjectID: r.id})
}

// FulltextSearch performs pure keyword search for this consumer.
func (r *ConsumerMemoryHandle) FulltextSearch(ctx context.Context, input ConsumerMemoryFulltextSearchInput) (*MemoryListResult, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	return r.Service().FulltextSearch(ctx, MemoryFulltextQuery{
		Query: input.Query, SubjectID: r.id, TopK: input.TopK, SessionID: input.SessionID,
		MemoryTypes: input.MemoryTypes, TrustTier: input.TrustTier, Branch: input.Branch,
		ExtraMetadataFilter: input.ExtraMetadataFilter,
	})
}

// Query performs exact structured filtering for this consumer.
func (r *ConsumerMemoryHandle) Query(ctx context.Context, input ConsumerMemoryStructuredQueryInput) (*MemoryListResult, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	return r.Service().Query(ctx, MemoryStructuredQuery{
		SubjectID: r.id, Limit: input.Limit, Cursor: input.Cursor, SessionID: input.SessionID,
		MemoryTypes: input.MemoryTypes, TrustTier: input.TrustTier, Branch: input.Branch,
		ExtraMetadataFilter: input.ExtraMetadataFilter,
	})
}

// RenderProfile renders this subject's consumer profile.
func (r *ConsumerMemoryHandle) RenderProfile(ctx context.Context) (*ConsumerProfileRenderResult, error) {
	return r.Service().RenderProfile(ctx, r.id, 0)
}

// DataShareSummary returns data-share publication summary counts for this workspace.
func (w *WorkspaceHandle) DataShareSummary(ctx context.Context) (*PublishSummaryResult, error) {
	return w.DataShare().PublishSummary(ctx)
}

// DataShareSourceTables lists Tables that can be selected for publication.
func (w *WorkspaceHandle) DataShareSourceTables(ctx context.Context, sourceDatabaseID string, opts ...QueryOption) (*DataShareSourceTableListResult, error) {
	return w.DataShare().SourceTables(ctx, sourceDatabaseID, opts...)
}

// DataSharePublish returns a handle for a known data-share publication id.
func (w *WorkspaceHandle) DataSharePublish(publishID string) (*DataSharePublishHandle, error) {
	if publishID == "" {
		return nil, ErrIDRequired
	}
	return &DataSharePublishHandle{workspace: w, id: publishID}, nil
}

// CreateDataSharePublish creates a data-share publication and returns a handle for the backend-created publish id.
func (w *WorkspaceHandle) CreateDataSharePublish(ctx context.Context, name, sourceDatabaseID string, tableScope *DataShareScope, targetWorkspaceIDs []string, opts ...DataSharePublishOption) (*DataSharePublishHandle, *DataSharePublish, error) {
	result, err := w.DataShare().CreatePublish(ctx, name, sourceDatabaseID, tableScope, targetWorkspaceIDs, opts...)
	if err != nil {
		return nil, nil, err
	}
	publishID, err := createdStringPathID(result.GetId(), "data-share publish")
	if err != nil {
		return nil, nil, err
	}
	return &DataSharePublishHandle{workspace: w, id: publishID}, result, nil
}

// DataShareSubscription returns a handle for a known data-share subscription id.
func (w *WorkspaceHandle) DataShareSubscription(subscriptionID string) (*DataShareSubscriptionHandle, error) {
	if subscriptionID == "" {
		return nil, ErrIDRequired
	}
	return &DataShareSubscriptionHandle{workspace: w, id: subscriptionID}, nil
}

// ID returns the data-share publication id.
func (p *DataSharePublishHandle) ID() string {
	return p.id
}

// Service returns the route-aligned escape hatch data-share service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (p *DataSharePublishHandle) Service() *DataShareService {
	return p.workspace.DataShare()
}

// Update updates this data-share publication.
func (p *DataSharePublishHandle) Update(ctx context.Context, opts ...DataSharePublishUpdateOption) (*DataSharePublish, error) {
	return p.Service().UpdatePublish(ctx, p.id, opts...)
}

// Delete deletes this data-share publication.
func (p *DataSharePublishHandle) Delete(ctx context.Context) error {
	return p.Service().DeletePublish(ctx, p.id)
}

// ID returns the data-share subscription id.
func (s *DataShareSubscriptionHandle) ID() string {
	return s.id
}

// Service returns the route-aligned escape hatch data-share service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (s *DataShareSubscriptionHandle) Service() *DataShareService {
	return s.workspace.DataShare()
}

// Subscribe subscribes this subscription into the workspace shared catalog.
func (s *DataShareSubscriptionHandle) Subscribe(ctx context.Context, subName string) (*DataShareSubscription, error) {
	return s.Service().Subscribe(ctx, s.id, subName)
}

// Unsubscribe unsubscribes this subscription.
func (s *DataShareSubscriptionHandle) Unsubscribe(ctx context.Context) error {
	return s.Service().Unsubscribe(ctx, s.id)
}

// AgentByCode returns a handle for a product agent selected by code.
func (w *WorkspaceHandle) AgentByCode(agentCode string) (*AgentHandle, error) {
	if agentCode == "" {
		return nil, ErrAgentRequired
	}
	return &AgentHandle{workspace: w, selector: AgentSelector{AgentCode: agentCode}}, nil
}

// AgentByCodeInWorkspace returns a handle for an agent selected by code in a specific agent workspace.
// Use "system" for a system-provided agent.
func (w *WorkspaceHandle) AgentByCodeInWorkspace(agentCode, agentWorkspaceID string) (*AgentHandle, error) {
	if agentCode == "" {
		return nil, ErrAgentRequired
	}
	if agentWorkspaceID == "" {
		return nil, ErrWorkspaceRequired
	}
	return &AgentHandle{workspace: w, selector: AgentSelector{
		AgentCode:        agentCode,
		AgentWorkspaceID: agentWorkspaceID,
	}}, nil
}

// AgentByID returns a handle for a product agent selected by id.
func (w *WorkspaceHandle) AgentByID(agentID string) (*AgentHandle, error) {
	if agentID == "" {
		return nil, ErrAgentRequired
	}
	return &AgentHandle{workspace: w, selector: AgentSelector{AgentID: agentID}}, nil
}

// Selector returns the selector carried by this agent handle.
func (a *AgentHandle) Selector() AgentSelector {
	return a.selector
}

// Service returns the route-aligned escape hatch agent service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (a *AgentHandle) Service() *AgentService {
	return a.workspace.Agents()
}

// A2A sends an A2A JSON-RPC request to this selected agent.
// It is an explicit protocol-control escape hatch, not a user-story SDK entry point or AI task-selection source.
// Prefer SendText, StreamText, GetTask, CancelTask, ResubscribeTask, and SubmitInput for promoted workflows.
func (a *AgentHandle) A2A(ctx context.Context, req A2ARequest) (*AgentA2AResult, error) {
	return a.Service().A2A(ctx, AgentA2ARequest{
		AgentSelector: a.selector,
		A2ARequest:    req,
	})
}

// SendText sends one user text message through the agent A2A message/send method.
// A non-terminal task admission (submitted/working) is followed to its
// persisted terminal state before returning, bounded by WithA2AWaitTimeout.
func (a *AgentHandle) SendText(ctx context.Context, message AgentTextMessage) (*AgentA2AResult, error) {
	params, err := message.Params()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return a.A2A(ctx, A2ARequest{
		JSONRPC: "2.0",
		ID:      message.RequestID,
		Method:  A2AMethodSend,
		Params:  data,
		Model:   message.Model,
	})
}

// StreamText streams one user text message through the agent A2A message/stream method.
func (a *AgentHandle) StreamText(ctx context.Context, message AgentTextMessage) (*StreamResponse, error) {
	params, err := message.Params()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return a.Service().A2AStream(ctx, AgentA2ARequest{
		AgentSelector: a.selector,
		A2ARequest: A2ARequest{
			JSONRPC: "2.0",
			ID:      message.RequestID,
			Method:  A2AMethodStream,
			Params:  data,
			Model:   message.Model,
		},
	})
}

// GetTask returns one A2A task from this selected agent.
func (a *AgentHandle) GetTask(ctx context.Context, request AgentTaskRequest) (*AgentA2AResult, error) {
	return a.taskControl(ctx, A2AMethodTasksGet, request)
}

// CancelTask cancels one A2A task on this selected agent.
func (a *AgentHandle) CancelTask(ctx context.Context, request AgentTaskRequest) (*AgentA2AResult, error) {
	return a.taskControl(ctx, A2AMethodTasksCancel, request)
}

// SubmitInput submits a user's answers for an A2A input request.
func (a *AgentHandle) SubmitInput(ctx context.Context, submission AgentInputSubmission) (*AgentA2AResult, error) {
	params, err := submission.Params()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return a.A2A(ctx, A2ARequest{
		JSONRPC: "2.0",
		ID:      submission.RequestID,
		Method:  A2AMethodInputSubmit,
		Params:  data,
	})
}

// ResubscribeTask streams updates for one A2A task on this selected agent.
func (a *AgentHandle) ResubscribeTask(ctx context.Context, request AgentTaskRequest) (*StreamResponse, error) {
	params, err := request.Params()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return a.Service().A2AStream(ctx, AgentA2ARequest{
		AgentSelector: a.selector,
		A2ARequest: A2ARequest{
			JSONRPC: "2.0",
			ID:      request.RequestID,
			Method:  A2AMethodTasksResubscribe,
			Params:  data,
		},
	})
}

func (a *AgentHandle) taskControl(ctx context.Context, method string, request AgentTaskRequest) (*AgentA2AResult, error) {
	params, err := request.Params()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return a.A2A(ctx, A2ARequest{
		JSONRPC: "2.0",
		ID:      request.RequestID,
		Method:  method,
		Params:  data,
	})
}

// AgentA2ATaskID returns the task id carried by an A2A result.
func AgentA2ATaskID(result *AgentA2AResult) (string, error) {
	return agentA2AStringField(result, "id", "task id")
}

// AgentA2ATurnID returns the input turn id carried by an A2A result.
func AgentA2ATurnID(result *AgentA2AResult) (string, error) {
	return agentA2AStringField(result, "turnId", "turn id")
}

// AgentA2APayload returns the A2A task/message object carried by a product A2A
// result. Product backends may return either a bare A2A object or a JSON-RPC
// envelope with that object under "result"; both shapes are accepted. JSON
// numbers are decoded as json.Number to preserve int64 task fields.
func AgentA2APayload(result *AgentA2AResult) (map[string]any, error) {
	if result == nil {
		return nil, agentA2AResultFieldError("payload", "result is nil")
	}
	var payload map[string]any
	if err := decodeExactJSONWithNumbers([]byte(result.GetJson()), &payload); err != nil {
		return nil, agentA2AResultFieldError("payload", err.Error())
	}
	return unwrapAgentA2APayload(payload), nil
}

func unwrapAgentA2APayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	// JSON-RPC 2.0 success responses nest the A2A object under "result". Prefer
	// that object when present so helpers do not read the RPC request id as the
	// task id.
	if result, ok := payload["result"].(map[string]any); ok {
		if _, hasJSONRPC := payload["jsonrpc"]; hasJSONRPC {
			return result
		}
		// Some product paths return {"result": <task>} without repeating jsonrpc.
		if _, hasKind := result["kind"]; hasKind {
			return result
		}
		if _, hasID := result["id"]; hasID {
			return result
		}
	}
	return payload
}

func agentA2AStringField(result *AgentA2AResult, field, label string) (string, error) {
	payload, err := AgentA2APayload(result)
	if err != nil {
		return "", agentA2AResultFieldError(label, err.Error())
	}
	value, ok := payload[field].(string)
	if !ok || value == "" {
		return "", agentA2AResultFieldError(label, "field is missing")
	}
	return value, nil
}

func agentA2AResultFieldError(label, reason string) error {
	return &Error{
		Code:    "ErrResponseInvalid",
		Message: fmt.Sprintf("A2A result missing %s", label),
		Details: map[string]any{
			"field":  label,
			"reason": reason,
		},
	}
}

// SQLMetadata returns a handle for SQL editor metadata browsing.
func (w *WorkspaceHandle) SQLMetadata() *SQLMetadataHandle {
	return &SQLMetadataHandle{workspace: w}
}

// SQLWorkbook returns a handle for a known SQL workbook id.
func (w *WorkspaceHandle) SQLWorkbook(workbookID string) (*SQLWorkbookHandle, error) {
	if workbookID == "" {
		return nil, ErrWorkbookIDRequired
	}
	return &SQLWorkbookHandle{workspace: w, id: workbookID}, nil
}

// CreateSQLWorkbook creates a SQL workbook and returns a handle for the backend-generated workbook id.
func (w *WorkspaceHandle) CreateSQLWorkbook(ctx context.Context, name string) (*SQLWorkbookHandle, *SQLWorkbook, error) {
	result, err := w.SQL().CreateWorkbook(ctx, name)
	if err != nil {
		return nil, nil, err
	}
	return &SQLWorkbookHandle{workspace: w, id: result.GetWorkbookId()}, result, nil
}

// SQLQuery returns a handle for a known SQL query id.
func (w *WorkspaceHandle) SQLQuery(queryID string) (*SQLQueryHandle, error) {
	if queryID == "" {
		return nil, ErrQueryIDRequired
	}
	return &SQLQueryHandle{workspace: w, id: queryID}, nil
}

// ExecuteSQL starts SQL execution and returns a handle for the backend-generated query id.
func (w *WorkspaceHandle) ExecuteSQL(ctx context.Context, query string, opts ...SQLQueryOption) (*SQLQueryHandle, *SQLQueryStartResult, error) {
	result, err := w.SQL().ExecuteQuery(ctx, query, opts...)
	if err != nil {
		return nil, nil, err
	}
	return &SQLQueryHandle{workspace: w, id: result.GetQueryId()}, result, nil
}

// SQLStatement returns a handle for a known SQL statement id.
func (w *WorkspaceHandle) SQLStatement(statementID string) (*SQLStatementHandle, error) {
	if statementID == "" {
		return nil, ErrStatementIDRequired
	}
	return &SQLStatementHandle{workspace: w, id: statementID}, nil
}

// Service returns the route-aligned escape hatch SQL service scoped to this metadata handle's workspace. Prefer typed handle methods for user-facing workflows.
func (m *SQLMetadataHandle) Service() *SQLService {
	return m.workspace.SQL()
}

// Databases lists SQL editor databases.
func (m *SQLMetadataHandle) Databases(ctx context.Context, opts ...SQLDatabaseListOption) (*SQLDatabaseListResult, error) {
	return m.Service().MetaDatabases(ctx, opts...)
}

// Tree returns SQL editor database tree metadata.
func (m *SQLMetadataHandle) Tree(ctx context.Context) (*SQLDatabaseTreeResult, error) {
	return m.Service().MetaDBTree(ctx)
}

// Database returns a handle for a known SQL editor database.
func (m *SQLMetadataHandle) Database(name string) (*SQLDatabaseMetadataHandle, error) {
	if name == "" {
		return nil, ErrDatabaseRequired
	}
	return &SQLDatabaseMetadataHandle{metadata: m, name: name}, nil
}

// Name returns the SQL editor database name.
func (d *SQLDatabaseMetadataHandle) Name() string {
	return d.name
}

// Tables lists tables under this SQL editor database.
func (d *SQLDatabaseMetadataHandle) Tables(ctx context.Context) (*SQLDatabaseTablesResult, error) {
	return d.metadata.Service().MetaDBTables(ctx, d.name)
}

// Table returns a handle for a known SQL editor table or view.
func (d *SQLDatabaseMetadataHandle) Table(name string) (*SQLTableMetadataHandle, error) {
	if name == "" {
		return nil, ErrTableRequired
	}
	return &SQLTableMetadataHandle{database: d, name: name}, nil
}

// Drop drops this SQL editor database.
func (d *SQLDatabaseMetadataHandle) Drop(ctx context.Context) error {
	return d.metadata.Service().DropMetaDatabase(ctx, d.name)
}

// Name returns the SQL editor table or view name.
func (t *SQLTableMetadataHandle) Name() string {
	return t.name
}

// Info returns SQL editor table data and columns.
func (t *SQLTableMetadataHandle) Info(ctx context.Context) (*SQLTableInfoResult, error) {
	return t.database.metadata.Service().MetaTableInfo(ctx, t.database.name, t.name)
}

// Columns returns SQL editor table columns.
func (t *SQLTableMetadataHandle) Columns(ctx context.Context) (*SQLTableColumnsResult, error) {
	return t.database.metadata.Service().MetaTableColumns(ctx, t.database.name, t.name)
}

// ColumnStats returns SQL editor table column statistics.
func (t *SQLTableMetadataHandle) ColumnStats(ctx context.Context) (*SQLTableColumnsResult, error) {
	return t.database.metadata.Service().MetaTableColumnStats(ctx, t.database.name, t.name)
}

// DDL returns SQL editor table DDL.
func (t *SQLTableMetadataHandle) DDL(ctx context.Context) (*SQLTableDDLResult, error) {
	return t.database.metadata.Service().MetaTableDDL(ctx, t.database.name, t.name)
}

// DropTable drops this SQL editor table.
func (t *SQLTableMetadataHandle) DropTable(ctx context.Context) error {
	return t.database.metadata.Service().DropMetaTable(ctx, t.database.name, t.name)
}

// DropView drops this SQL editor view.
func (t *SQLTableMetadataHandle) DropView(ctx context.Context) error {
	return t.database.metadata.Service().DropMetaView(ctx, t.database.name, t.name)
}

// ID returns the SQL workbook id.
func (b *SQLWorkbookHandle) ID() string {
	return b.id
}

// Service returns the route-aligned escape hatch SQL service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (b *SQLWorkbookHandle) Service() *SQLService {
	return b.workspace.SQL()
}

// Rename updates this SQL workbook name.
func (b *SQLWorkbookHandle) Rename(ctx context.Context, name string) error {
	return b.Service().UpdateWorkbook(ctx, b.id, name)
}

// Delete deletes this SQL workbook.
func (b *SQLWorkbookHandle) Delete(ctx context.Context) error {
	return b.Service().DeleteWorkbook(ctx, b.id)
}

// Versions lists versions for this SQL workbook.
func (b *SQLWorkbookHandle) Versions(ctx context.Context, opts ...WorkbookVersionListOption) (*SQLWorkbookVersionListResult, error) {
	return b.Service().WorkbookVersions(ctx, b.id, opts...)
}

// CreateVersion creates a SQL workbook version and returns a handle for the backend-generated version id.
func (b *SQLWorkbookHandle) CreateVersion(ctx context.Context, sqlContent string) (*SQLWorkbookVersionHandle, *SQLWorkbookVersionCreateResult, error) {
	result, err := b.Service().CreateWorkbookVersion(ctx, b.id, sqlContent)
	if err != nil {
		return nil, nil, err
	}
	return &SQLWorkbookVersionHandle{workbook: b, id: result.GetId()}, result, nil
}

// Version returns a handle for a known SQL workbook version id.
func (b *SQLWorkbookHandle) Version(versionID string) (*SQLWorkbookVersionHandle, error) {
	if versionID == "" {
		return nil, ErrVersionIDRequired
	}
	return &SQLWorkbookVersionHandle{workbook: b, id: versionID}, nil
}

// ID returns the SQL workbook version id.
func (v *SQLWorkbookVersionHandle) ID() string {
	return v.id
}

// Workbook returns this version's workbook handle.
func (v *SQLWorkbookVersionHandle) Workbook() *SQLWorkbookHandle {
	return v.workbook
}

// Info returns this SQL workbook version content.
func (v *SQLWorkbookVersionHandle) Info(ctx context.Context) (*SQLWorkbookDetailResult, error) {
	return v.workbook.Service().WorkbookDetail(ctx, v.workbook.id, v.id)
}

// Update updates this SQL workbook version draft.
func (v *SQLWorkbookVersionHandle) Update(ctx context.Context, sqlContent string) error {
	return v.workbook.Service().UpdateWorkbookVersion(ctx, v.workbook.id, v.id, sqlContent)
}

// Save finalizes this SQL workbook version.
func (v *SQLWorkbookVersionHandle) Save(ctx context.Context, sqlContent ...string) error {
	var content *string
	if len(sqlContent) > 0 {
		content = &sqlContent[0]
	}
	return v.workbook.Service().SaveWorkbookVersion(ctx, v.workbook.id, v.id, content)
}

// ID returns the SQL query id.
func (q *SQLQueryHandle) ID() string {
	return q.id
}

// Service returns the route-aligned escape hatch SQL service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (q *SQLQueryHandle) Service() *SQLService {
	return q.workspace.SQL()
}

// Describe returns this SQL query operation state.
func (q *SQLQueryHandle) Describe(ctx context.Context) (*SQLQueryOperation, error) {
	return q.Service().DescribeQuery(ctx, q.id)
}

// Kill cancels this SQL query.
func (q *SQLQueryHandle) Kill(ctx context.Context) error {
	return q.Service().KillQuery(ctx, q.id)
}

// ID returns the SQL statement id.
func (s *SQLStatementHandle) ID() string {
	return s.id
}

// Service returns the route-aligned escape hatch SQL service scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (s *SQLStatementHandle) Service() *SQLService {
	return s.workspace.SQL()
}

// Result returns result rows for this SQL statement.
func (s *SQLStatementHandle) Result(ctx context.Context, opts ...SQLResultOption) (*SQLQueryResult, error) {
	return s.Service().QueryResult(ctx, s.id, opts...)
}

// Detail returns history detail for this SQL statement.
func (s *SQLStatementHandle) Detail(ctx context.Context, opts ...QueryRecordOption) (*SQLStatementRecord, error) {
	return s.Service().QueryDetail(ctx, s.id, opts...)
}

// Profile returns execution profile for this SQL statement.
func (s *SQLStatementHandle) Profile(ctx context.Context, opts ...QueryRecordOption) (*SQLQueryProfileResult, error) {
	return s.Service().QueryProfile(ctx, s.id, opts...)
}

// Members returns canonical member APIs scoped to this workspace.
func (w *WorkspaceHandle) Members() *WorkspaceMemberService {
	return w.client.WorkspaceMembers(w.id)
}

// IAM 返回当前 workspace 的 IAM 强类型服务。
func (w *WorkspaceHandle) IAM() *IAMService {
	return w.client.IAM(w.id)
}

// Member returns a handle for a known canonical member user id.
func (w *WorkspaceHandle) Member(userID string) (*WorkspaceMemberHandle, error) {
	if userID == "" {
		return nil, ErrIDRequired
	}
	return &WorkspaceMemberHandle{workspace: w, id: userID}, nil
}

// InviteMember invites by email, mobile, or GitHub login. Email/mobile remain
// pending until the target accepts. A GitHub directory hit seats immediately
// and the result ID is the canonical user id (treat as already a member); a
// miss returns USER_NOT_FOUND without creating an invitation.
func (w *WorkspaceHandle) InviteMember(ctx context.Context, name string, defaultRoleID int, opts ...CreateUserOption) (*WorkspaceMemberInviteResult, error) {
	return w.Members().Invite(ctx, name, defaultRoleID, opts...)
}

// ID returns the canonical member user id.
func (u *WorkspaceMemberHandle) ID() string {
	return u.id
}

// Service returns the route-aligned escape hatch for canonical member APIs scoped to this handle's workspace. Prefer typed handle methods for user-facing workflows.
func (u *WorkspaceMemberHandle) Service() *WorkspaceMemberService {
	return u.workspace.Members()
}

// Info returns this workspace member's detail.
func (u *WorkspaceMemberHandle) Info(ctx context.Context) (*WorkspaceMember, error) {
	return u.Service().Member(ctx, u.id)
}

// Update updates this workspace member's editable metadata.
func (u *WorkspaceMemberHandle) Update(ctx context.Context, name, description string, opts ...UpdateUserOption) error {
	return u.Service().Update(ctx, u.id, name, description, opts...)
}

// UpdateStatus updates this workspace member's lifecycle action.
func (u *WorkspaceMemberHandle) UpdateStatus(ctx context.Context, action string) error {
	return u.Service().UpdateStatus(ctx, u.id, action)
}

// Delete removes this workspace member.
func (u *WorkspaceMemberHandle) Delete(ctx context.Context) error {
	return u.Service().Delete(ctx, u.id)
}

// Catalog returns catalog APIs scoped to this workspace.
func (w *WorkspaceHandle) Catalog() *CatalogService {
	return w.client.Catalog(w.id)
}

// Knowledge returns knowledge APIs scoped to this workspace.
func (w *WorkspaceHandle) Knowledge() *KnowledgeService {
	return w.client.Knowledge(w.id)
}

// Workflows returns workflow APIs scoped to this workspace.
func (w *WorkspaceHandle) Workflows() *WorkflowService {
	return w.client.Workflows(w.id)
}

// Agents returns agent APIs scoped to this workspace.
func (w *WorkspaceHandle) Agents() *AgentService {
	return w.client.Agents(w.id)
}

// Models returns model/provider APIs scoped to this workspace.
func (w *WorkspaceHandle) Models() *ModelService {
	return w.client.Models(w.id)
}

// Connectors returns connector APIs scoped to this workspace.
func (w *WorkspaceHandle) Connectors() *ConnectorService {
	return w.client.Connectors(w.id)
}

// SQL returns SQL editor APIs scoped to this workspace.
func (w *WorkspaceHandle) SQL() *SQLService {
	return w.client.SQL(w.id)
}

// Compute returns compute resource APIs scoped to this workspace.
func (w *WorkspaceHandle) Compute() *ComputeService {
	return w.client.Compute(w.id)
}

// Memory returns the route-aligned escape hatch consumer memory service scoped to this workspace. Prefer typed handle methods for user-facing workflows.
//
// Deprecated: Current MOI does not serve the legacy consumer-memory/profile API.
func (w *WorkspaceHandle) Memory() *MemoryService {
	return w.client.Memory(w.id)
}

// DataShare returns data-share APIs scoped to this workspace.
func (w *WorkspaceHandle) DataShare() *DataShareService {
	return w.client.DataShare(w.id)
}

func duplicateNameError(kind, name string) error {
	return &Error{
		Code:    "ErrDuplicateName",
		Message: kind + " name is not unique: " + name,
		Details: map[string]any{
			"kind": kind,
			"name": name,
		},
	}
}

func parsePositiveIntID(value int64, kind string) (int, error) {
	if value <= 0 {
		return 0, &Error{
			Code:    "ErrResponseInvalid",
			Message: fmt.Sprintf("%s id is invalid: %d", kind, value),
			Details: map[string]any{
				"kind": kind,
				"id":   fmt.Sprintf("%d", value),
			},
		}
	}
	return int(value), nil
}

func createdStringPathID(value, kind string) (string, error) {
	if value == "" {
		return "", createdIDError(kind)
	}
	return value, nil
}

func createdIntPathID(value int64, kind string) (string, error) {
	if value <= 0 {
		return "", createdIDError(kind)
	}
	return strconv.FormatInt(value, 10), nil
}

func createdUintPathID(value uint64, kind string) (string, error) {
	if value == 0 {
		return "", createdIDError(kind)
	}
	return strconv.FormatUint(value, 10), nil
}

func createdIDError(kind string) error {
	return &Error{
		Code:    "ErrResponseInvalid",
		Message: kind + " create response missing id",
		Details: map[string]any{
			"kind": kind,
		},
	}
}
