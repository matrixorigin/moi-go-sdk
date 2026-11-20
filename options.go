package sdk

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"
)

// Option configures a Product SDK client.
type Option func(*config)

type config struct {
	allowUnauthenticated bool
	apiKey               string
	token                string
	sessionCookie        string
	timeout              time.Duration
	a2aWaitTimeout       time.Duration
	httpClient           *http.Client
	requestModifiers     []RequestModifier
	customHeaders        map[string]string
	workspaceCurrentRole []workspaceCurrentRole
}

// RequestModifier modifies each outgoing product request before it is sent.
type RequestModifier func(*http.Request)

// WithUnauthenticated permits calls to public bootstrap and browser-login endpoints.
// Protected operations still rely on the server's normal authentication checks.
func WithUnauthenticated() Option {
	return func(c *config) { c.allowUnauthenticated = true }
}

// WithAPIKey configures user-scoped API key authentication against moi-backend.
func WithAPIKey(apiKey string) Option {
	return func(c *config) {
		c.apiKey = apiKey
	}
}

// WithToken configures bearer token authentication against moi-backend.
func WithToken(token string) Option {
	return func(c *config) {
		c.token = token
	}
}

// WithSessionCookie configures session-cookie authentication against moi-backend.
func WithSessionCookie(cookie string) Option {
	return func(c *config) {
		c.sessionCookie = cookie
	}
}

// WithTimeout sets the timeout for each product request.
func WithTimeout(timeout time.Duration) Option {
	return func(c *config) {
		c.timeout = timeout
	}
}

// WithA2AWaitTimeout sets how long a message/send call waits for the admitted
// task to reach a terminal state before returning ErrTimeout with the task id.
// It defaults to 10 minutes and never changes the per-request timeout.
func WithA2AWaitTimeout(timeout time.Duration) Option {
	return func(c *config) {
		c.a2aWaitTimeout = timeout
	}
}

// WithHTTPClient sets the HTTP client used by the SDK.
func WithHTTPClient(client *http.Client) Option {
	return func(c *config) {
		c.httpClient = client
	}
}

// WithRequestModifier adds a request modifier called before each request.
// X-Workspace-ID and X-MOI-Role-ID are SDK-reserved and rebound after the
// modifier returns.
func WithRequestModifier(modifier RequestModifier) Option {
	return func(c *config) {
		c.requestModifiers = append(c.requestModifiers, modifier)
	}
}

// WithCustomHeader adds a custom header to each product request.
// X-Workspace-ID and X-MOI-Role-ID are SDK-reserved and cannot be overridden.
func WithCustomHeader(key, value string) Option {
	return func(c *config) {
		if c.customHeaders == nil {
			c.customHeaders = map[string]string{}
		}
		c.customHeaders[key] = value
	}
}

// WithRuntimeAuthorization adds the short-lived sandbox child proof required
// by native task-scoped runtime operations. It supplements the technical API
// key and is not a user Role or permission input.
func WithRuntimeAuthorization(value string) Option {
	return WithCustomHeader(headerRuntimeAuthorization, value)
}

// WithWorkspaceCurrentRoleID selects an untrusted Role Candidate for one
// workspace. Backend and Core still validate binding and lifecycle.
func WithWorkspaceCurrentRoleID(workspaceID, roleID string) Option {
	return func(c *config) {
		c.workspaceCurrentRole = append(c.workspaceCurrentRole, workspaceCurrentRole{
			workspaceID: workspaceID,
			roleID:      roleID,
		})
	}
}

type connectorDefinitionOptions struct {
	config map[string]any
}

// ConnectorDefinitionOption configures connector create/validate requests.
type ConnectorDefinitionOption func(*connectorDefinitionOptions)

func WithConnectorConfig(config map[string]any) ConnectorDefinitionOption {
	return func(o *connectorDefinitionOptions) {
		o.config = config
	}
}

type connectorUpdateOptions struct {
	name       *string
	sourceType *int
	usageType  *int
	config     map[string]any
}

// ConnectorUpdateOption configures connector updates.
type ConnectorUpdateOption func(*connectorUpdateOptions)

func WithConnectorName(name string) ConnectorUpdateOption {
	return func(o *connectorUpdateOptions) {
		o.name = &name
	}
}

func WithConnectorSourceType(sourceType int) ConnectorUpdateOption {
	return func(o *connectorUpdateOptions) {
		o.sourceType = &sourceType
	}
}

func WithConnectorUsageType(usageType int) ConnectorUpdateOption {
	return func(o *connectorUpdateOptions) {
		o.usageType = &usageType
	}
}

func WithConnectorUpdateConfig(config map[string]any) ConnectorUpdateOption {
	return func(o *connectorUpdateOptions) {
		o.config = config
	}
}

type connectorPreviewOptions struct {
	connFileID    *string
	sheetName     *string
	isColumnName  *bool
	rowStart      *int
	columnNameRow *int
	csv           map[string]any
	fileType      *int
}

// ConnectorPreviewOption configures connector file preview.
type ConnectorPreviewOption func(*connectorPreviewOptions)

func WithConnectorPreviewConnFileID(connFileID string) ConnectorPreviewOption {
	return func(o *connectorPreviewOptions) {
		o.connFileID = &connFileID
	}
}

func WithConnectorPreviewSheetName(sheetName string) ConnectorPreviewOption {
	return func(o *connectorPreviewOptions) {
		o.sheetName = &sheetName
	}
}

func WithConnectorPreviewColumnName(isColumnName bool) ConnectorPreviewOption {
	return func(o *connectorPreviewOptions) {
		o.isColumnName = &isColumnName
	}
}

func WithConnectorPreviewRowStart(rowStart int) ConnectorPreviewOption {
	return func(o *connectorPreviewOptions) {
		o.rowStart = &rowStart
	}
}

func WithConnectorPreviewColumnNameRow(columnNameRow int) ConnectorPreviewOption {
	return func(o *connectorPreviewOptions) {
		o.columnNameRow = &columnNameRow
	}
}

func WithConnectorPreviewCSV(separator, delimiter string, isEscape bool) ConnectorPreviewOption {
	return func(o *connectorPreviewOptions) {
		o.csv = map[string]any{"separator": separator, "delimiter": delimiter, "isEscape": isEscape}
	}
}

func WithConnectorPreviewFileType(fileType int) ConnectorPreviewOption {
	return func(o *connectorPreviewOptions) {
		o.fileType = &fileType
	}
}

type connectorDBOptions struct {
	schema *string
}

// ConnectorDBOption configures connector DB browsing requests.
type ConnectorDBOption func(*connectorDBOptions)

func WithConnectorSchema(schema string) ConnectorDBOption {
	return func(o *connectorDBOptions) {
		o.schema = &schema
	}
}

type createImportTaskOptions struct {
	name     *string
	volumeID *string
}

// CreateImportTaskOption configures import task creation.
type CreateImportTaskOption func(*createImportTaskOptions)

func WithImportTaskName(name string) CreateImportTaskOption {
	return func(o *createImportTaskOptions) {
		o.name = &name
	}
}

func WithImportTaskVolumeID(volumeID string) CreateImportTaskOption {
	return func(o *createImportTaskOptions) {
		o.volumeID = &volumeID
	}
}

type exportTaskListOptions struct {
	keyword        *string
	targetCategory *string
	limit          *int
	offset         *int
	connectorName  *string
	taskID         *string
	orderBy        *string
	orderDirection *string
	statuses       []int
	creator        *string
}

// ExportTaskListOption configures export task list requests.
type ExportTaskListOption func(*exportTaskListOptions)

// WithExportTaskKeyword searches task names, connector names and source paths.
func WithExportTaskKeyword(keyword string) ExportTaskListOption {
	return func(o *exportTaskListOptions) { o.keyword = &keyword }
}

// WithExportTaskTargetCategory accepts database, storage, model or other.
func WithExportTaskTargetCategory(category string) ExportTaskListOption {
	return func(o *exportTaskListOptions) { o.targetCategory = &category }
}

func WithExportTaskLimit(limit int) ExportTaskListOption {
	return func(o *exportTaskListOptions) {
		o.limit = &limit
	}
}

func WithExportTaskOffset(offset int) ExportTaskListOption {
	return func(o *exportTaskListOptions) {
		o.offset = &offset
	}
}

func WithExportTaskConnectorName(connectorName string) ExportTaskListOption {
	return func(o *exportTaskListOptions) {
		o.connectorName = &connectorName
	}
}

func WithExportTaskID(taskID string) ExportTaskListOption {
	return func(o *exportTaskListOptions) {
		o.taskID = &taskID
	}
}

func WithExportTaskOrderBy(orderBy string) ExportTaskListOption {
	return func(o *exportTaskListOptions) {
		o.orderBy = &orderBy
	}
}

func WithExportTaskOrderDirection(orderDirection string) ExportTaskListOption {
	return func(o *exportTaskListOptions) {
		o.orderDirection = &orderDirection
	}
}

func WithExportTaskStatuses(statuses ...int) ExportTaskListOption {
	return func(o *exportTaskListOptions) {
		o.statuses = append(o.statuses, statuses...)
	}
}

func WithExportTaskCreator(creator string) ExportTaskListOption {
	return func(o *exportTaskListOptions) {
		o.creator = &creator
	}
}

type exportTaskFilesOptions struct {
	limit          *int
	offset         *int
	statuses       []int
	orderBy        *string
	orderDirection *string
}

// ExportTaskFilesOption configures export task file-list requests.
type ExportTaskFilesOption func(*exportTaskFilesOptions)

func WithExportTaskFilesLimit(limit int) ExportTaskFilesOption {
	return func(o *exportTaskFilesOptions) {
		o.limit = &limit
	}
}

func WithExportTaskFilesOffset(offset int) ExportTaskFilesOption {
	return func(o *exportTaskFilesOptions) {
		o.offset = &offset
	}
}

func WithExportTaskFileStatuses(statuses ...int) ExportTaskFilesOption {
	return func(o *exportTaskFilesOptions) {
		o.statuses = append(o.statuses, statuses...)
	}
}

func WithExportTaskFilesOrderBy(orderBy string) ExportTaskFilesOption {
	return func(o *exportTaskFilesOptions) {
		o.orderBy = &orderBy
	}
}

func WithExportTaskFilesOrderDirection(orderDirection string) ExportTaskFilesOption {
	return func(o *exportTaskFilesOptions) {
		o.orderDirection = &orderDirection
	}
}

// ExportFileParam describes one file selected for export task creation.
type ExportFileParam struct {
	FileID       string   `json:"file_id"`
	RefFileID    string   `json:"ref_file_id,omitempty"`
	ParsedFileID string   `json:"parsed_file_id,omitempty"`
	FullPath     []string `json:"full_path"`
	IsRaw        bool     `json:"is_raw"`
}

type sqlDatabaseListOptions struct {
	pageSize  *int
	pageToken *string
	search    *string
	name      *string
}

// SQLDatabaseListOption configures database-name listing, without enumerating tables.
type SQLDatabaseListOption func(*sqlDatabaseListOptions)

func WithSQLDatabasePageSize(size int) SQLDatabaseListOption {
	return func(o *sqlDatabaseListOptions) { o.pageSize = &size }
}

func WithSQLDatabasePageToken(token string) SQLDatabaseListOption {
	return func(o *sqlDatabaseListOptions) { o.pageToken = &token }
}

func WithSQLDatabaseSearch(search string) SQLDatabaseListOption {
	return func(o *sqlDatabaseListOptions) { o.search = &search }
}

func WithSQLDatabaseName(name string) SQLDatabaseListOption {
	return func(o *sqlDatabaseListOptions) { o.name = &name }
}

type workbookListOptions struct {
	search    *string
	pageSize  *int
	pageToken *string
}

// WorkbookListOption configures SQL workbook list requests.
type WorkbookListOption func(*workbookListOptions)

func WithWorkbookSearch(search string) WorkbookListOption {
	return func(o *workbookListOptions) {
		o.search = &search
	}
}

func WithWorkbookPageSize(pageSize int) WorkbookListOption {
	return func(o *workbookListOptions) {
		o.pageSize = &pageSize
	}
}

func WithWorkbookPageToken(pageToken string) WorkbookListOption {
	return func(o *workbookListOptions) {
		o.pageToken = &pageToken
	}
}

type workbookVersionListOptions struct {
	pageSize  *int
	pageToken *string
}

// WorkbookVersionListOption configures SQL workbook version list requests.
type WorkbookVersionListOption func(*workbookVersionListOptions)

func WithWorkbookVersionPageSize(pageSize int) WorkbookVersionListOption {
	return func(o *workbookVersionListOptions) {
		o.pageSize = &pageSize
	}
}

func WithWorkbookVersionPageToken(pageToken string) WorkbookVersionListOption {
	return func(o *workbookVersionListOptions) {
		o.pageToken = &pageToken
	}
}

type sqlQueryOptions struct {
	dbName  *string
	sqlType *string
	offset  *int
	limit   *int
}

// SQLQueryOption configures SQL execution requests.
type SQLQueryOption func(*sqlQueryOptions)

func WithSQLQueryDBName(dbName string) SQLQueryOption {
	return func(o *sqlQueryOptions) {
		o.dbName = &dbName
	}
}

func WithSQLQueryType(sqlType string) SQLQueryOption {
	return func(o *sqlQueryOptions) {
		o.sqlType = &sqlType
	}
}

func WithSQLQueryOffset(offset int) SQLQueryOption {
	return func(o *sqlQueryOptions) {
		o.offset = &offset
	}
}

func WithSQLQueryLimit(limit int) SQLQueryOption {
	return func(o *sqlQueryOptions) {
		o.limit = &limit
	}
}

type sqlResultOptions struct {
	offset *int
	limit  *int
}

// SQLResultOption configures SQL result pagination requests.
type SQLResultOption func(*sqlResultOptions)

func WithSQLResultOffset(offset int) SQLResultOption {
	return func(o *sqlResultOptions) {
		o.offset = &offset
	}
}

func WithSQLResultLimit(limit int) SQLResultOption {
	return func(o *sqlResultOptions) {
		o.limit = &limit
	}
}

// QueryHistoryScope selects a history category, not a separate permission.
type QueryHistoryScope string

const (
	QueryHistoryScopeSelf      QueryHistoryScope = "self"
	QueryHistoryScopeWorkspace QueryHistoryScope = "workspace"
)

type queryHistoryOverviewOptions struct {
	scope          QueryHistoryScope
	nonUser        *bool
	start          *time.Time
	end            *time.Time
	sqlSourceTypes []string
}

// QueryHistoryOverviewOption configures SQL query-history overview requests.
type QueryHistoryOverviewOption func(*queryHistoryOverviewOptions)

func WithQueryHistoryOverviewScope(scope QueryHistoryScope) QueryHistoryOverviewOption {
	return func(o *queryHistoryOverviewOptions) { o.scope = scope }
}

func WithQueryHistoryOverviewNonUser(nonUser bool) QueryHistoryOverviewOption {
	return func(o *queryHistoryOverviewOptions) {
		o.nonUser = &nonUser
	}
}

func WithQueryHistoryOverviewStart(start time.Time) QueryHistoryOverviewOption {
	return func(o *queryHistoryOverviewOptions) {
		o.start = &start
	}
}

func WithQueryHistoryOverviewEnd(end time.Time) QueryHistoryOverviewOption {
	return func(o *queryHistoryOverviewOptions) {
		o.end = &end
	}
}

func WithQueryHistoryOverviewSQLSourceTypes(types ...string) QueryHistoryOverviewOption {
	return func(o *queryHistoryOverviewOptions) {
		o.sqlSourceTypes = append(o.sqlSourceTypes, types...)
	}
}

type queryHistoryOptions struct {
	scope          QueryHistoryScope
	nonUser        *bool
	status         *string
	users          []string
	databases      []string
	statement      *string
	queryTypes     []string
	minDuration    *uint64
	start          *time.Time
	end            *time.Time
	statementID    *string
	transactionID  *string
	sessionID      *string
	selectedFields []string
	sqlSourceTypes []string
	offset         *uint
	limit          *uint
	orderBy        []string
	order          *string
	cu             *uint
}

// QueryHistoryOption configures SQL query-history list requests.
type QueryHistoryOption func(*queryHistoryOptions)

func WithQueryHistoryScope(scope QueryHistoryScope) QueryHistoryOption {
	return func(o *queryHistoryOptions) { o.scope = scope }
}

func WithQueryHistoryNonUser(nonUser bool) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.nonUser = &nonUser
	}
}

func WithQueryHistoryStatus(status string) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.status = &status
	}
}

func WithQueryHistoryUsers(users ...string) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.users = append(o.users, users...)
	}
}

func WithQueryHistoryDatabases(databases ...string) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.databases = append(o.databases, databases...)
	}
}

func WithQueryHistoryStatement(statement string) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.statement = &statement
	}
}

func WithQueryHistoryTypes(types ...string) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.queryTypes = append(o.queryTypes, types...)
	}
}

func WithQueryHistoryMinDuration(duration uint64) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.minDuration = &duration
	}
}

func WithQueryHistoryStart(start time.Time) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.start = &start
	}
}

func WithQueryHistoryEnd(end time.Time) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.end = &end
	}
}

func WithQueryHistoryStatementID(statementID string) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.statementID = &statementID
	}
}

func WithQueryHistoryTransactionID(transactionID string) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.transactionID = &transactionID
	}
}

func WithQueryHistorySessionID(sessionID string) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.sessionID = &sessionID
	}
}

func WithQueryHistorySelectedFields(fields ...string) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.selectedFields = append(o.selectedFields, fields...)
	}
}

func WithQueryHistorySQLSourceTypes(types ...string) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.sqlSourceTypes = append(o.sqlSourceTypes, types...)
	}
}

func WithQueryHistoryOffset(offset uint) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.offset = &offset
	}
}

func WithQueryHistoryLimit(limit uint) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.limit = &limit
	}
}

func WithQueryHistoryOrderBy(fields ...string) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.orderBy = append(o.orderBy, fields...)
	}
}

func WithQueryHistoryOrder(order string) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.order = &order
	}
}

func WithQueryHistoryCU(cu uint) QueryHistoryOption {
	return func(o *queryHistoryOptions) {
		o.cu = &cu
	}
}

type queryRecordOptions struct {
	scope QueryHistoryScope
	start *string
	end   *string
}

// QueryRecordOption configures SQL query detail/profile requests.
type QueryRecordOption func(*queryRecordOptions)

func WithQueryRecordScope(scope QueryHistoryScope) QueryRecordOption {
	return func(o *queryRecordOptions) { o.scope = scope }
}

func WithQueryRecordStart(start string) QueryRecordOption {
	return func(o *queryRecordOptions) {
		o.start = &start
	}
}

func WithQueryRecordEnd(end string) QueryRecordOption {
	return func(o *queryRecordOptions) {
		o.end = &end
	}
}

// WorkflowVolumeTriggerParam configures a workflow volume trigger.
type WorkflowVolumeTriggerParam struct {
	VolumeID int64  `json:"volume_id,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
	VarsJSON string `json:"vars_json,omitempty"`
}

// WorkflowUIResourceParam binds one workspace HTML file as the published
// service UI. The server derives the public resource URI.
type WorkflowUIResourceParam struct {
	FileID   string `json:"file_id"`
	MimeType string `json:"mime_type,omitempty"`
}

// WorkflowDynamicServiceParam configures a workflow dynamic-service trigger.
// A nil UIResource on an explicit dynamic-service block clears a binding.
type WorkflowDynamicServiceParam struct {
	ServiceName     string                   `json:"service_name,omitempty"`
	InputSchema     string                   `json:"input_schema,omitempty"`
	OutputSchema    string                   `json:"output_schema,omitempty"`
	ResultMode      string                   `json:"result_mode,omitempty"`
	RuntimeSpecJSON string                   `json:"runtime_spec_json,omitempty"`
	UIResource      *WorkflowUIResourceParam `json:"ui_resource,omitempty"`
}

// WorkflowCandidateRef identifies a server-owned A2A workflow candidate artifact.
type WorkflowCandidateRef struct {
	TaskID     string `json:"task_id"`
	ArtifactID string `json:"artifact_id"`
}

type workflowDeployOptions struct {
	workflowID        *string
	description       *string
	executionMode     *string
	cronExpression    *string
	sourceType        *string
	workflow          *json.RawMessage
	inputForm         any
	formLayout        any
	volumeTrigger     *WorkflowVolumeTriggerParam
	dynamicService    *WorkflowDynamicServiceParam
	defaultValues     map[string]any
	computeResourceID *string
	sessionID         *string
	notification      *workflowHTTPNotification
}

type workflowHTTPNotification struct {
	Type                 string   `json:"type"`
	URL                  string   `json:"url"`
	NotifyWorkflowStates []string `json:"notify_workflow_states"`
}

// WithWorkflowHTTPNotification configures the existing terminal HTTP callback.
// States use the server contract: COMPLETED, FAILED, CANCELLED.
func WithWorkflowHTTPNotification(url string, workflowStates ...string) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.notification = &workflowHTTPNotification{Type: "http", URL: url, NotifyWorkflowStates: workflowStates}
	}
}

// WorkflowDeployOption configures workflow deployment requests.
type WorkflowDeployOption func(*workflowDeployOptions)

func WithWorkflowID(workflowID string) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.workflowID = &workflowID
	}
}

func WithWorkflowDescription(description string) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.description = &description
	}
}

func WithWorkflowExecutionMode(mode string) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.executionMode = &mode
	}
}

func WithWorkflowCronExpression(expr string) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.cronExpression = &expr
	}
}

func WithWorkflowSourceType(sourceType string) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.sourceType = &sourceType
	}
}

// WithWorkflowMOWL deploys an authored native workflow through the existing
// manual_mowl publication contract. Pass an empty dslYAML to Deploy.
// Accepted server candidates must continue to use DeployCandidate.
func WithWorkflowMOWL(workflow json.RawMessage) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.workflow = &workflow
	}
}

func WithWorkflowInputForm(inputForm any) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.inputForm = inputForm
	}
}

func WithWorkflowFormLayout(formLayout any) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.formLayout = formLayout
	}
}

func WithWorkflowVolumeTrigger(trigger WorkflowVolumeTriggerParam) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.volumeTrigger = &trigger
	}
}

func WithWorkflowDynamicService(service WorkflowDynamicServiceParam) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.dynamicService = &service
	}
}

func WithWorkflowDefaultValues(values map[string]any) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.defaultValues = values
	}
}

func WithWorkflowComputeResourceID(computeResourceID string) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.computeResourceID = &computeResourceID
	}
}

// WithWorkflowSessionID binds an existing Copilot A2A conversation while deploying.
func WithWorkflowSessionID(sessionID string) WorkflowDeployOption {
	return func(o *workflowDeployOptions) {
		o.sessionID = &sessionID
	}
}

type workflowUpdateOptions struct {
	name           *string
	description    *string
	executionMode  *string
	cronExpression *string
	dslYAML        *string
	inputForm      any
	formLayout     any
	defaultValues  map[string]any
	volumeTrigger  *WorkflowVolumeTriggerParam
	dynamicService *WorkflowDynamicServiceParam
}

// WorkflowUpdateOption configures workflow update requests.
type WorkflowUpdateOption func(*workflowUpdateOptions)

func WithWorkflowUpdateName(name string) WorkflowUpdateOption {
	return func(o *workflowUpdateOptions) {
		o.name = &name
	}
}

func WithWorkflowUpdateDescription(description string) WorkflowUpdateOption {
	return func(o *workflowUpdateOptions) {
		o.description = &description
	}
}

func WithWorkflowUpdateExecutionMode(mode string) WorkflowUpdateOption {
	return func(o *workflowUpdateOptions) {
		o.executionMode = &mode
	}
}

func WithWorkflowUpdateCronExpression(expr string) WorkflowUpdateOption {
	return func(o *workflowUpdateOptions) {
		o.cronExpression = &expr
	}
}

func WithWorkflowUpdateDSLYAML(dslYAML string) WorkflowUpdateOption {
	return func(o *workflowUpdateOptions) {
		o.dslYAML = &dslYAML
	}
}

// WithWorkflowUpdateInputForm sets runtime form fields (JSON key: runtime_fields).
func WithWorkflowUpdateInputForm(inputForm any) WorkflowUpdateOption {
	return func(o *workflowUpdateOptions) {
		o.inputForm = inputForm
	}
}

// WithWorkflowUpdateFormLayout sets runtime form layout (JSON key: runtime_layout).
func WithWorkflowUpdateFormLayout(formLayout any) WorkflowUpdateOption {
	return func(o *workflowUpdateOptions) {
		o.formLayout = formLayout
	}
}

func WithWorkflowUpdateDefaultValues(values map[string]any) WorkflowUpdateOption {
	return func(o *workflowUpdateOptions) {
		o.defaultValues = values
	}
}

func WithWorkflowUpdateVolumeTrigger(trigger WorkflowVolumeTriggerParam) WorkflowUpdateOption {
	return func(o *workflowUpdateOptions) {
		o.volumeTrigger = &trigger
	}
}

func WithWorkflowUpdateDynamicService(service WorkflowDynamicServiceParam) WorkflowUpdateOption {
	return func(o *workflowUpdateOptions) {
		o.dynamicService = &service
	}
}

type workflowRerunOptions struct {
	values                    map[string]any
	replaceSavedDefaults      bool
	triggerNow                *bool
	configOverride            *json.RawMessage
	expectedRerunContractHash *string
	reason                    *string
}

// WorkflowRerunOption configures workflow retry and node rerun requests.
type WorkflowRerunOption func(*workflowRerunOptions)

func WithWorkflowRerunValues(values map[string]any) WorkflowRerunOption {
	return func(o *workflowRerunOptions) {
		o.values = values
	}
}

// WithWorkflowRetryReplaceSavedDefaults replaces saved defaults with the retry
// values. False preserves the default merge behavior. This applies to RetryRun.
func WithWorkflowRetryReplaceSavedDefaults(replace bool) WorkflowRerunOption {
	return func(o *workflowRerunOptions) {
		o.replaceSavedDefaults = replace
	}
}

func WithWorkflowRerunTriggerNow(triggerNow bool) WorkflowRerunOption {
	return func(o *workflowRerunOptions) {
		o.triggerNow = &triggerNow
	}
}

func WithWorkflowRerunConfigOverrideJSON(configOverride json.RawMessage) WorkflowRerunOption {
	return func(o *workflowRerunOptions) {
		o.configOverride = &configOverride
	}
}

// WithExpectedWorkflowRerunContractHash supplies the required immutable
// contract returned by the rerun plan for node rerun creation.
func WithExpectedWorkflowRerunContractHash(contractHash string) WorkflowRerunOption {
	return func(o *workflowRerunOptions) {
		o.expectedRerunContractHash = &contractHash
	}
}

func WithWorkflowRerunReason(reason string) WorkflowRerunOption {
	return func(o *workflowRerunOptions) {
		o.reason = &reason
	}
}

type workflowCancelOptions struct {
	reason *string
}

// WorkflowCancelOption configures workflow cancel requests.
type WorkflowCancelOption func(*workflowCancelOptions)

func WithWorkflowCancelReason(reason string) WorkflowCancelOption {
	return func(o *workflowCancelOptions) {
		o.reason = &reason
	}
}

type customOperatorOptions struct {
	nodeID          *string
	kind            *string
	language        *string
	handler         *string
	version         *string
	isolationLevel  *string
	enabled         *bool
	code            *string
	sourceFileID    *string
	baseNodeID      *string
	baseNodeVersion *string
	bindingConfig   any
	catalogID       *int64
	databaseID      *int64
}

// CustomOperatorOption configures custom operator creation.
type CustomOperatorOption func(*customOperatorOptions)

func WithCustomOperatorNodeID(nodeID string) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.nodeID = &nodeID
	}
}

func WithCustomOperatorKind(kind string) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.kind = &kind
	}
}

func WithCustomOperatorLanguage(language string) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.language = &language
	}
}

func WithCustomOperatorHandler(handler string) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.handler = &handler
	}
}

func WithCustomOperatorVersion(version string) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.version = &version
	}
}

func WithCustomOperatorIsolationLevel(isolationLevel string) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.isolationLevel = &isolationLevel
	}
}

func WithCustomOperatorEnabled(enabled bool) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.enabled = &enabled
	}
}

func WithCustomOperatorCode(code string) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.code = &code
	}
}

func WithCustomOperatorSourceFileID(sourceFileID string) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.sourceFileID = &sourceFileID
	}
}

func WithCustomOperatorBaseNodeID(baseNodeID string) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.baseNodeID = &baseNodeID
	}
}

func WithCustomOperatorBaseNodeVersion(baseNodeVersion string) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.baseNodeVersion = &baseNodeVersion
	}
}

func WithCustomOperatorBindingConfig(bindingConfig any) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.bindingConfig = bindingConfig
	}
}

func WithCustomOperatorCatalogID(catalogID int64) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.catalogID = &catalogID
	}
}

func WithCustomOperatorDatabaseID(databaseID int64) CustomOperatorOption {
	return func(o *customOperatorOptions) {
		o.databaseID = &databaseID
	}
}

type customOperatorUpdateOptions struct {
	name            *string
	description     *string
	kind            *string
	language        *string
	handler         *string
	version         *string
	isolationLevel  *string
	inputSchema     any
	outputSchema    any
	enabled         *bool
	code            *string
	sourceFileID    *string
	baseNodeID      *string
	baseNodeVersion *string
	bindingConfig   any
	catalogID       *int64
	databaseID      *int64
}

// CustomOperatorUpdateOption configures custom operator updates.
type CustomOperatorUpdateOption func(*customOperatorUpdateOptions)

func WithCustomOperatorUpdateName(name string) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.name = &name
	}
}

func WithCustomOperatorUpdateDescription(description string) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.description = &description
	}
}

func WithCustomOperatorUpdateKind(kind string) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.kind = &kind
	}
}

func WithCustomOperatorUpdateLanguage(language string) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.language = &language
	}
}

func WithCustomOperatorUpdateHandler(handler string) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.handler = &handler
	}
}

func WithCustomOperatorUpdateVersion(version string) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.version = &version
	}
}

func WithCustomOperatorUpdateIsolationLevel(isolationLevel string) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.isolationLevel = &isolationLevel
	}
}

func WithCustomOperatorUpdateEnabled(enabled bool) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.enabled = &enabled
	}
}

func WithCustomOperatorUpdateInputSchema(inputSchema any) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.inputSchema = inputSchema
	}
}

func WithCustomOperatorUpdateOutputSchema(outputSchema any) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.outputSchema = outputSchema
	}
}

func WithCustomOperatorUpdateCode(code string) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.code = &code
	}
}

func WithCustomOperatorUpdateSourceFileID(sourceFileID string) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.sourceFileID = &sourceFileID
	}
}

func WithCustomOperatorUpdateBaseNodeID(baseNodeID string) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.baseNodeID = &baseNodeID
	}
}

func WithCustomOperatorUpdateBaseNodeVersion(baseNodeVersion string) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.baseNodeVersion = &baseNodeVersion
	}
}

func WithCustomOperatorUpdateBindingConfig(bindingConfig any) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.bindingConfig = bindingConfig
	}
}

func WithCustomOperatorUpdateCatalogID(catalogID int64) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.catalogID = &catalogID
	}
}

func WithCustomOperatorUpdateDatabaseID(databaseID int64) CustomOperatorUpdateOption {
	return func(o *customOperatorUpdateOptions) {
		o.databaseID = &databaseID
	}
}

type workflowTemplateOptions struct {
	templateKey *string
	description *string
	inputForm   *string
	isBuiltin   *bool
}

// WorkflowTemplateOption configures workflow template creation/update.
type WorkflowTemplateOption func(*workflowTemplateOptions)

func WithWorkflowTemplateKey(templateKey string) WorkflowTemplateOption {
	return func(o *workflowTemplateOptions) {
		o.templateKey = &templateKey
	}
}

func WithWorkflowTemplateDescription(description string) WorkflowTemplateOption {
	return func(o *workflowTemplateOptions) {
		o.description = &description
	}
}

func WithWorkflowTemplateInputForm(inputForm string) WorkflowTemplateOption {
	return func(o *workflowTemplateOptions) {
		o.inputForm = &inputForm
	}
}

func WithWorkflowTemplateBuiltin(isBuiltin bool) WorkflowTemplateOption {
	return func(o *workflowTemplateOptions) {
		o.isBuiltin = &isBuiltin
	}
}

type computeCreateOptions struct {
	kind                *string
	pyRequirementsTxt   *string
	description         *string
	gpu                 *int
	minReplicas         *int
	maxReplicas         *int
	autoSuspendMinutes  *int
	workerImages        *[]*WorkerImageSelection
	goWorkerImageID     *string
	pythonWorkerImageID *string
	platform            *string
}

// ComputeCreateOption configures compute resource creation.
type ComputeCreateOption func(*computeCreateOptions)

func WithComputeKind(kind string) ComputeCreateOption {
	return func(o *computeCreateOptions) { o.kind = &kind }
}

func WithComputePyRequirementsTxt(requirements string) ComputeCreateOption {
	return func(o *computeCreateOptions) { o.pyRequirementsTxt = &requirements }
}

func WithComputeDescription(description string) ComputeCreateOption {
	return func(o *computeCreateOptions) {
		o.description = &description
	}
}

func WithComputeGPU(gpu int) ComputeCreateOption {
	return func(o *computeCreateOptions) {
		o.gpu = &gpu
	}
}

func WithComputeMinReplicas(minReplicas int) ComputeCreateOption {
	return func(o *computeCreateOptions) {
		o.minReplicas = &minReplicas
	}
}

func WithComputeMaxReplicas(maxReplicas int) ComputeCreateOption {
	return func(o *computeCreateOptions) {
		o.maxReplicas = &maxReplicas
	}
}

// WithComputeAutoSuspendMinutes sets the idle timeout in minutes. The
// Catalog validates task resources; an explicit zero is sent unchanged.
// Values above 60 are capped at 60, the longest idle policy the cloud runtime
// can express.
func WithComputeAutoSuspendMinutes(minutes int) ComputeCreateOption {
	return func(o *computeCreateOptions) {
		o.autoSuspendMinutes = &minutes
	}
}

// WithComputeWorkerImages supplies the current Catalog default selection for
// each worker type. Non-default image IDs are rejected by Catalog; omit this
// option to let the configured defaults select the deployment images.
func WithComputeWorkerImages(workerImages ...*WorkerImageSelection) ComputeCreateOption {
	return func(o *computeCreateOptions) {
		images := append([]*WorkerImageSelection(nil), workerImages...)
		o.workerImages = &images
	}
}

func WithComputeGoWorkerImageID(imageID string) ComputeCreateOption {
	return func(o *computeCreateOptions) {
		o.goWorkerImageID = &imageID
	}
}

func WithComputePythonWorkerImageID(imageID string) ComputeCreateOption {
	return func(o *computeCreateOptions) {
		o.pythonWorkerImageID = &imageID
	}
}

func WithComputePlatform(platform string) ComputeCreateOption {
	return func(o *computeCreateOptions) {
		o.platform = &platform
	}
}

type computeUpdateOptions struct {
	kind                *string
	pyRequirementsTxt   *string
	name                *string
	description         *string
	specID              *string
	cpu                 *int
	memoryGiB           *int
	gpu                 *int
	minReplicas         *int
	maxReplicas         *int
	autoSuspendMinutes  *int
	workerImages        *[]*WorkerImageSelection
	goWorkerImageID     *string
	pythonWorkerImageID *string
	platform            *string
}

// ComputeUpdateOption configures compute resource updates.
type ComputeUpdateOption func(*computeUpdateOptions)

func WithComputeUpdateKind(kind string) ComputeUpdateOption {
	return func(o *computeUpdateOptions) { o.kind = &kind }
}

// WithComputeUpdatePyRequirementsTxt replaces the Python worker dependencies.
// An explicit empty string clears them; omission leaves them unchanged.
func WithComputeUpdatePyRequirementsTxt(requirements string) ComputeUpdateOption {
	return func(o *computeUpdateOptions) { o.pyRequirementsTxt = &requirements }
}

func WithComputeUpdateName(name string) ComputeUpdateOption {
	return func(o *computeUpdateOptions) {
		o.name = &name
	}
}

func WithComputeUpdateDescription(description string) ComputeUpdateOption {
	return func(o *computeUpdateOptions) {
		o.description = &description
	}
}

func WithComputeUpdateSpecID(specID string) ComputeUpdateOption {
	return func(o *computeUpdateOptions) {
		o.specID = &specID
	}
}

func WithComputeUpdateCPU(cpu int) ComputeUpdateOption {
	return func(o *computeUpdateOptions) {
		o.cpu = &cpu
	}
}

func WithComputeUpdateMemoryGiB(memoryGiB int) ComputeUpdateOption {
	return func(o *computeUpdateOptions) {
		o.memoryGiB = &memoryGiB
	}
}

func WithComputeUpdateGPU(gpu int) ComputeUpdateOption {
	return func(o *computeUpdateOptions) {
		o.gpu = &gpu
	}
}

func WithComputeUpdateMinReplicas(minReplicas int) ComputeUpdateOption {
	return func(o *computeUpdateOptions) {
		o.minReplicas = &minReplicas
	}
}

func WithComputeUpdateMaxReplicas(maxReplicas int) ComputeUpdateOption {
	return func(o *computeUpdateOptions) {
		o.maxReplicas = &maxReplicas
	}
}

// WithComputeUpdateAutoSuspendMinutes sets the idle timeout in minutes. The
// Catalog validates task resources; an explicit zero is sent unchanged.
// Values above 60 are capped at 60, the longest idle policy the cloud runtime
// can express.
func WithComputeUpdateAutoSuspendMinutes(minutes int) ComputeUpdateOption {
	return func(o *computeUpdateOptions) {
		o.autoSuspendMinutes = &minutes
	}
}

// WithComputeUpdateWorkerImages replaces the complete worker image metadata
// with current Catalog default selections. Non-default image IDs are rejected;
// omit this option when the update should keep Catalog-driven deployment.
func WithComputeUpdateWorkerImages(workerImages ...*WorkerImageSelection) ComputeUpdateOption {
	return func(o *computeUpdateOptions) {
		images := append([]*WorkerImageSelection(nil), workerImages...)
		o.workerImages = &images
	}
}

func WithComputeUpdateGoWorkerImageID(imageID string) ComputeUpdateOption {
	return func(o *computeUpdateOptions) {
		o.goWorkerImageID = &imageID
	}
}

func WithComputeUpdatePythonWorkerImageID(imageID string) ComputeUpdateOption {
	return func(o *computeUpdateOptions) {
		o.pythonWorkerImageID = &imageID
	}
}

func WithComputeUpdatePlatform(platform string) ComputeUpdateOption {
	return func(o *computeUpdateOptions) {
		o.platform = &platform
	}
}

type computeSpecOptions struct {
	nodePlacement *map[string]any
	gpuCount      *int
	gpuMemoryMiB  *int
	gpuCores      *int
	creditPerHour *float64
	description   *string
	enabled       *bool
}

// ComputeSpecOption configures compute resource spec creation.
type ComputeSpecOption func(*computeSpecOptions)

// WithComputeSpecNodePlacement sends the owner's placement object unchanged.
// A nil map sends null; omit the option to omit the field.
func WithComputeSpecNodePlacement(placement map[string]any) ComputeSpecOption {
	return func(o *computeSpecOptions) { o.nodePlacement = &placement }
}

func WithComputeSpecGPUCount(gpuCount int) ComputeSpecOption {
	return func(o *computeSpecOptions) {
		o.gpuCount = &gpuCount
	}
}

func WithComputeSpecGPUMemoryMiB(gpuMemoryMiB int) ComputeSpecOption {
	return func(o *computeSpecOptions) {
		o.gpuMemoryMiB = &gpuMemoryMiB
	}
}

func WithComputeSpecGPUCores(gpuCores int) ComputeSpecOption {
	return func(o *computeSpecOptions) {
		o.gpuCores = &gpuCores
	}
}

func WithComputeSpecCreditPerHour(creditPerHour float64) ComputeSpecOption {
	return func(o *computeSpecOptions) {
		o.creditPerHour = &creditPerHour
	}
}

func WithComputeSpecDescription(description string) ComputeSpecOption {
	return func(o *computeSpecOptions) {
		o.description = &description
	}
}

func WithComputeSpecEnabled(enabled bool) ComputeSpecOption {
	return func(o *computeSpecOptions) {
		o.enabled = &enabled
	}
}

type computeSpecUpdateOptions struct {
	computeSpecOptions
	kind         *string
	family       *string
	familyName   *string
	familyNameEN *string
	cpuMilli     *int
	memoryMiB    *int
}

// ComputeSpecUpdateOption configures compute resource spec updates.
type ComputeSpecUpdateOption func(*computeSpecUpdateOptions)

func WithComputeSpecUpdateNodePlacement(placement map[string]any) ComputeSpecUpdateOption {
	return func(o *computeSpecUpdateOptions) { o.nodePlacement = &placement }
}

func WithComputeSpecUpdateKind(kind string) ComputeSpecUpdateOption {
	return func(o *computeSpecUpdateOptions) {
		o.kind = &kind
	}
}

func WithComputeSpecUpdateFamily(family string) ComputeSpecUpdateOption {
	return func(o *computeSpecUpdateOptions) {
		o.family = &family
	}
}

func WithComputeSpecUpdateFamilyName(familyName string) ComputeSpecUpdateOption {
	return func(o *computeSpecUpdateOptions) {
		o.familyName = &familyName
	}
}

func WithComputeSpecUpdateFamilyNameEN(familyNameEN string) ComputeSpecUpdateOption {
	return func(o *computeSpecUpdateOptions) {
		o.familyNameEN = &familyNameEN
	}
}

func WithComputeSpecUpdateCPUMilli(cpuMilli int) ComputeSpecUpdateOption {
	return func(o *computeSpecUpdateOptions) {
		o.cpuMilli = &cpuMilli
	}
}

func WithComputeSpecUpdateMemoryMiB(memoryMiB int) ComputeSpecUpdateOption {
	return func(o *computeSpecUpdateOptions) {
		o.memoryMiB = &memoryMiB
	}
}

func WithComputeSpecUpdateGPUCount(gpuCount int) ComputeSpecUpdateOption {
	return func(o *computeSpecUpdateOptions) {
		o.gpuCount = &gpuCount
	}
}

func WithComputeSpecUpdateGPUMemoryMiB(gpuMemoryMiB int) ComputeSpecUpdateOption {
	return func(o *computeSpecUpdateOptions) {
		o.gpuMemoryMiB = &gpuMemoryMiB
	}
}

func WithComputeSpecUpdateGPUCores(gpuCores int) ComputeSpecUpdateOption {
	return func(o *computeSpecUpdateOptions) {
		o.gpuCores = &gpuCores
	}
}

func WithComputeSpecUpdateCreditPerHour(creditPerHour float64) ComputeSpecUpdateOption {
	return func(o *computeSpecUpdateOptions) {
		o.creditPerHour = &creditPerHour
	}
}

func WithComputeSpecUpdateDescription(description string) ComputeSpecUpdateOption {
	return func(o *computeSpecUpdateOptions) {
		o.description = &description
	}
}

func WithComputeSpecUpdateEnabled(enabled bool) ComputeSpecUpdateOption {
	return func(o *computeSpecUpdateOptions) {
		o.enabled = &enabled
	}
}

type providerBackendOptions struct {
	providerType             *string
	reasoningControlProtocol *string
	apiKey                   *string
	address                  *string
	timeoutSeconds           *int32
	models                   []string
	supportedMimeTypes       []string
}

// ProviderBackendOption configures LLM, embedding, and parser backend creation.
type ProviderBackendOption func(*providerBackendOptions)

func WithProviderBackendType(providerType string) ProviderBackendOption {
	return func(o *providerBackendOptions) {
		o.providerType = &providerType
	}
}

func WithProviderBackendReasoningControlProtocol(protocol string) ProviderBackendOption {
	return func(o *providerBackendOptions) {
		o.reasoningControlProtocol = &protocol
	}
}

func WithProviderBackendAPIKey(apiKey string) ProviderBackendOption {
	return func(o *providerBackendOptions) {
		o.apiKey = &apiKey
	}
}

func WithProviderBackendAddress(address string) ProviderBackendOption {
	return func(o *providerBackendOptions) {
		o.address = &address
	}
}

func WithProviderBackendTimeoutSeconds(timeoutSeconds int32) ProviderBackendOption {
	return func(o *providerBackendOptions) {
		o.timeoutSeconds = &timeoutSeconds
	}
}

func WithProviderBackendModels(models ...string) ProviderBackendOption {
	return func(o *providerBackendOptions) {
		o.models = append(o.models, models...)
	}
}

func WithProviderBackendSupportedMimeTypes(mimeTypes ...string) ProviderBackendOption {
	return func(o *providerBackendOptions) {
		o.supportedMimeTypes = append(o.supportedMimeTypes, mimeTypes...)
	}
}

type providerBackendUpdateOptions struct {
	name                     *string
	apiKey                   *string
	reasoningControlProtocol *string
	timeoutSeconds           *int32
	models                   []string
	supportedMimeTypes       []string
}

// ProviderBackendUpdateOption configures LLM, embedding, and parser backend updates.
type ProviderBackendUpdateOption func(*providerBackendUpdateOptions)

func WithProviderBackendUpdateName(name string) ProviderBackendUpdateOption {
	return func(o *providerBackendUpdateOptions) {
		o.name = &name
	}
}

func WithProviderBackendUpdateAPIKey(apiKey string) ProviderBackendUpdateOption {
	return func(o *providerBackendUpdateOptions) {
		o.apiKey = &apiKey
	}
}

func WithProviderBackendUpdateReasoningControlProtocol(protocol string) ProviderBackendUpdateOption {
	return func(o *providerBackendUpdateOptions) {
		o.reasoningControlProtocol = &protocol
	}
}

func WithProviderBackendUpdateTimeoutSeconds(timeoutSeconds int32) ProviderBackendUpdateOption {
	return func(o *providerBackendUpdateOptions) {
		o.timeoutSeconds = &timeoutSeconds
	}
}

func WithProviderBackendUpdateModels(models ...string) ProviderBackendUpdateOption {
	return func(o *providerBackendUpdateOptions) {
		o.models = append(o.models, models...)
	}
}

func WithProviderBackendUpdateSupportedMimeTypes(mimeTypes ...string) ProviderBackendUpdateOption {
	return func(o *providerBackendUpdateOptions) {
		o.supportedMimeTypes = append(o.supportedMimeTypes, mimeTypes...)
	}
}

type routerOptions struct {
	strategy            *string
	healthCheckInterval *int32
	maxRetries          *int32
	sessionAffinity     *bool
}

// RouterOption configures provider router updates.
type RouterOption func(*routerOptions)

func WithRouterStrategy(strategy string) RouterOption {
	return func(o *routerOptions) {
		o.strategy = &strategy
	}
}

func WithRouterHealthCheckInterval(interval int32) RouterOption {
	return func(o *routerOptions) {
		o.healthCheckInterval = &interval
	}
}

func WithRouterMaxRetries(maxRetries int32) RouterOption {
	return func(o *routerOptions) {
		o.maxRetries = &maxRetries
	}
}

func WithRouterSessionAffinity(enabled bool) RouterOption {
	return func(o *routerOptions) {
		o.sessionAffinity = &enabled
	}
}

type dataSharePublishOptions struct {
	remark *string
}

// DataSharePublishOption configures data-share publication creation.
type DataSharePublishOption func(*dataSharePublishOptions)

func WithDataShareRemark(remark string) DataSharePublishOption {
	return func(o *dataSharePublishOptions) {
		o.remark = &remark
	}
}

type dataSharePublishUpdateOptions struct {
	tableScope         *DataShareScope
	targetWorkspaceIDs []string
	remark             *string
}

// DataSharePublishUpdateOption configures data-share publication updates.
type DataSharePublishUpdateOption func(*dataSharePublishUpdateOptions)

func WithDataShareUpdateTableScope(scope *DataShareScope) DataSharePublishUpdateOption {
	return func(o *dataSharePublishUpdateOptions) {
		o.tableScope = scope
	}
}

func WithDataShareUpdateTargetWorkspaceIDs(workspaceIDs ...string) DataSharePublishUpdateOption {
	return func(o *dataSharePublishUpdateOptions) {
		if o.targetWorkspaceIDs == nil {
			o.targetWorkspaceIDs = []string{}
		}
		o.targetWorkspaceIDs = append(o.targetWorkspaceIDs, workspaceIDs...)
	}
}

func WithDataShareUpdateRemark(remark string) DataSharePublishUpdateOption {
	return func(o *dataSharePublishUpdateOptions) {
		o.remark = &remark
	}
}

type semanticModelOptions struct {
	description *string
	tables      any
	files       any
}

// SemanticModelOption configures semantic model creation and update.
type SemanticModelOption func(*semanticModelOptions)

// SemanticModelCreateSpec configures creation-only embedding and drawing
// behavior. Description, tables and files use the shared SemanticModelOption
// values accepted by CreateWithSpec; update does not accept this spec.
type SemanticModelCreateSpec struct {
	Name                     string  `json:"name"`
	PreserveEmbedding        *bool   `json:"preserve_embedding,omitempty"`
	TextSendDimensions       *bool   `json:"text_send_dimensions,omitempty"`
	ImageSendDimensions      *bool   `json:"image_send_dimensions,omitempty"`
	DrawingStructuredEnabled *bool   `json:"drawing_structured_enabled,omitempty"`
	DrawingNumberPolicy      *string `json:"drawing_number_policy,omitempty"`
}

func WithSemanticModelDescription(description string) SemanticModelOption {
	return func(o *semanticModelOptions) {
		o.description = &description
	}
}

func WithSemanticModelTables(tables any) SemanticModelOption {
	return func(o *semanticModelOptions) {
		o.tables = tables
	}
}

func WithSemanticModelFiles(files any) SemanticModelOption {
	return func(o *semanticModelOptions) {
		o.files = files
	}
}

type semanticModelWithSourcesOptions struct {
	description       *string
	targetCatalogID   *int64
	files             any
	imageIndexEnabled *bool
	sourceSelections  []SemanticModelSourceSelectionInput
}

// SemanticModelWithSourcesOption configures semantic model creation with initial sources.
type SemanticModelWithSourcesOption func(*semanticModelWithSourcesOptions)

func WithSemanticModelWithSourcesTargetCatalogID(catalogID int64) SemanticModelWithSourcesOption {
	return func(o *semanticModelWithSourcesOptions) {
		o.targetCatalogID = &catalogID
	}
}

func WithSemanticModelWithSourcesDescription(description string) SemanticModelWithSourcesOption {
	return func(o *semanticModelWithSourcesOptions) {
		o.description = &description
	}
}

func WithSemanticModelWithSourcesFiles(files any) SemanticModelWithSourcesOption {
	return func(o *semanticModelWithSourcesOptions) {
		o.files = files
	}
}

func WithSemanticModelWithSourcesImageIndexEnabled(enabled bool) SemanticModelWithSourcesOption {
	return func(o *semanticModelWithSourcesOptions) {
		o.imageIndexEnabled = &enabled
	}
}

// WithSemanticModelWithSourcesSourceSelections attaches Catalog source
// selections to creation, matching the create-with-sources request body.
func WithSemanticModelWithSourcesSourceSelections(selections ...SemanticModelSourceSelectionInput) SemanticModelWithSourcesOption {
	return func(o *semanticModelWithSourcesOptions) {
		o.sourceSelections = append([]SemanticModelSourceSelectionInput{}, selections...)
	}
}

type semanticModelEmptyOptions struct {
	description       *string
	imageIndexEnabled *bool
}

// SemanticModelEmptyOption configures data-side knowledge base creation without sources.
type SemanticModelEmptyOption func(*semanticModelEmptyOptions)

func WithSemanticModelEmptyDescription(description string) SemanticModelEmptyOption {
	return func(o *semanticModelEmptyOptions) {
		o.description = &description
	}
}

func WithSemanticModelEmptyImageIndexEnabled(enabled bool) SemanticModelEmptyOption {
	return func(o *semanticModelEmptyOptions) {
		o.imageIndexEnabled = &enabled
	}
}

// SemanticModelSourceInput describes one semantic model source request.
// VolumeID is required for catalog_file sources (authoritative volume location).
type SemanticModelSourceInput struct {
	SourceType  string
	FileName    string
	UploadKind  string
	ParseMode   string
	TableConfig string
	FileID      string
	VolumeID    int64
	TableID     int64
}

// SemanticModelSourceSelectionFilters describes optional catalog leaf filters.
type SemanticModelSourceSelectionFilters struct {
	TableName string
	FileName  string
	FileExt   []string
}

// SemanticModelSourceSelectionInput describes one database-table or volume-file selection.
type SemanticModelSourceSelectionInput struct {
	Kind             string
	DatabaseID       int64
	VolumeID         int64
	AllSelected      bool
	SelectedTableIDs []int64
	ExcludedTableIDs []int64
	SelectedFileIDs  []string
	ExcludedFileIDs  []string
	Filters          SemanticModelSourceSelectionFilters
}

type semanticModelAppendSourcesOptions struct {
	selections []SemanticModelSourceSelectionInput
}

// SemanticModelAppendSourcesOption configures catalog-backed source selection.
type SemanticModelAppendSourcesOption func(*semanticModelAppendSourcesOptions)

func WithSemanticModelAppendSourceSelections(selections ...SemanticModelSourceSelectionInput) SemanticModelAppendSourcesOption {
	return func(o *semanticModelAppendSourcesOptions) { o.selections = slices.Clone(selections) }
}

type sourceJobReconcileOptions struct {
	retrySourceRowID string
}

// SourceJobReconcileOption configures an explicit retry during source job reconciliation.
type SourceJobReconcileOption func(*sourceJobReconcileOptions)

// WithRetrySourceRowID retries the selected failed knowledge source.
func WithRetrySourceRowID(sourceRowID string) SourceJobReconcileOption {
	return func(o *sourceJobReconcileOptions) { o.retrySourceRowID = sourceRowID }
}

type semanticModelSourceGovernanceOptions struct {
	tags                    *[]string
	expiresAt               *int64
	clearExpiresAt          bool
	enabled                 *bool
	forceEnabledAfterExpiry *bool
}

// SemanticModelSourceGovernanceOption configures source governance updates.
type SemanticModelSourceGovernanceOption func(*semanticModelSourceGovernanceOptions)

func WithSemanticModelSourceTags(tags ...string) SemanticModelSourceGovernanceOption {
	return func(o *semanticModelSourceGovernanceOptions) {
		values := append([]string{}, tags...)
		o.tags = &values
	}
}

func WithSemanticModelSourceExpiresAt(expiresAt int64) SemanticModelSourceGovernanceOption {
	return func(o *semanticModelSourceGovernanceOptions) {
		o.expiresAt = &expiresAt
		o.clearExpiresAt = false
	}
}

func WithSemanticModelSourceExpiresAtCleared() SemanticModelSourceGovernanceOption {
	return func(o *semanticModelSourceGovernanceOptions) {
		o.expiresAt = nil
		o.clearExpiresAt = true
	}
}

func WithSemanticModelSourceEnabled(enabled bool) SemanticModelSourceGovernanceOption {
	return func(o *semanticModelSourceGovernanceOptions) {
		o.enabled = &enabled
	}
}

func WithSemanticModelSourceForceEnabledAfterExpiry(force bool) SemanticModelSourceGovernanceOption {
	return func(o *semanticModelSourceGovernanceOptions) {
		o.forceEnabledAfterExpiry = &force
	}
}

// SemanticModelSegmentBase identifies the current document snapshot being edited.
type SemanticModelSegmentBase struct {
	IfMatch string `json:"if_match"`
}

// SemanticModelSegmentUpdateInput describes one segment edit.
type SemanticModelSegmentUpdateInput struct {
	SemanticModelSegmentBase
	Content          *string
	OCRText          *string
	ImageDescription *string
}

// SemanticModelSegmentCreateInput describes one segment creation request.
type SemanticModelSegmentCreateInput struct {
	SemanticModelSegmentBase
	Level            string
	Content          *string
	OCRText          *string
	ImageDescription *string
	ImageFileID      string
	PageImageFileID  string
	BBox             any
	Metadata         any
}

// SemanticEntryInput describes one semantic model entry request.
type SemanticEntryInput struct {
	Kind   string
	Key    string
	Tables []string
	Spec   any
	Domain string
}

// Validate checks whether this semantic entry has the required product fields.
func (e SemanticEntryInput) Validate() error {
	if e.Kind == "" {
		return ErrKindRequired
	}
	if e.Key == "" {
		return ErrKeyRequired
	}
	if e.Spec == nil {
		return ErrSpecRequired
	}
	return nil
}

type lineageRevisionOptions struct {
	baseRevisionID              *string
	useOriginal                 *bool
	useCurrentEffective         *bool
	expectedEffectiveSetVersion *int64
	expectedBaseContentHash     *string
	revisionContentRef          *string
	revisionContentHash         *string
	revisionContentPayload      *string
	patchJSON                   *string
}

// LineageRevisionOption configures lineage output-block revision creation.
type LineageRevisionOption func(*lineageRevisionOptions)

func WithLineageBaseRevisionID(baseRevisionID string) LineageRevisionOption {
	return func(o *lineageRevisionOptions) {
		o.baseRevisionID = &baseRevisionID
	}
}

func WithLineageUseOriginal(useOriginal bool) LineageRevisionOption {
	return func(o *lineageRevisionOptions) {
		o.useOriginal = &useOriginal
	}
}

func WithLineageUseCurrentEffective(useCurrentEffective bool) LineageRevisionOption {
	return func(o *lineageRevisionOptions) {
		o.useCurrentEffective = &useCurrentEffective
	}
}

func WithLineageExpectedEffectiveSetVersion(version int64) LineageRevisionOption {
	return func(o *lineageRevisionOptions) {
		o.expectedEffectiveSetVersion = &version
	}
}

func WithLineageExpectedBaseContentHash(contentHash string) LineageRevisionOption {
	return func(o *lineageRevisionOptions) {
		o.expectedBaseContentHash = &contentHash
	}
}

func WithLineageRevisionContentRef(contentRef string) LineageRevisionOption {
	return func(o *lineageRevisionOptions) {
		o.revisionContentRef = &contentRef
	}
}

func WithLineageRevisionContentHash(contentHash string) LineageRevisionOption {
	return func(o *lineageRevisionOptions) {
		o.revisionContentHash = &contentHash
	}
}

func WithLineageRevisionContentPayload(payload string) LineageRevisionOption {
	return func(o *lineageRevisionOptions) {
		o.revisionContentPayload = &payload
	}
}

func WithLineagePatchJSON(patchJSON string) LineageRevisionOption {
	return func(o *lineageRevisionOptions) {
		o.patchJSON = &patchJSON
	}
}

type LineageRevisionSwitch struct {
	BlockID             string `json:"block_id"`
	EffectiveRevisionID string `json:"effective_revision_id,omitempty"`
	UseOriginal         bool   `json:"use_original,omitempty"`
}

type lineageSwitchOptions struct {
	expectedEffectiveSetVersion *int64
}

// LineageSwitchOption configures lineage effective-revision switches.
type LineageSwitchOption func(*lineageSwitchOptions)

func WithLineageSwitchExpectedEffectiveSetVersion(version int64) LineageSwitchOption {
	return func(o *lineageSwitchOptions) {
		o.expectedEffectiveSetVersion = &version
	}
}

type lineageRerunOptions struct {
	mode                     *string
	configOverride           json.RawMessage
	inputPayloadOverrideJSON json.RawMessage
	varsPayloadOverrideJSON  json.RawMessage
}

// LineageRerunOption configures lineage rerun requests.
type LineageRerunOption func(*lineageRerunOptions)

func WithLineageRerunMode(mode string) LineageRerunOption {
	return func(o *lineageRerunOptions) {
		o.mode = &mode
	}
}

func WithLineageRerunConfigOverride(configOverride json.RawMessage) LineageRerunOption {
	return func(o *lineageRerunOptions) {
		o.configOverride = configOverride
	}
}

func WithLineageRerunInputPayloadOverride(inputPayloadOverride json.RawMessage) LineageRerunOption {
	return func(o *lineageRerunOptions) {
		o.inputPayloadOverrideJSON = inputPayloadOverride
	}
}

func WithLineageRerunVarsPayloadOverride(varsPayloadOverride json.RawMessage) LineageRerunOption {
	return func(o *lineageRerunOptions) {
		o.varsPayloadOverrideJSON = varsPayloadOverride
	}
}

type lineageRevisionRerunOptions struct {
	mode                      *string
	sourceEffectiveSetVersion *int64
	selectedRevisionsJSON     json.RawMessage
}

// LineageRevisionRerunOption configures revision-based lineage reruns.
type LineageRevisionRerunOption func(*lineageRevisionRerunOptions)

func WithLineageRevisionRerunMode(mode string) LineageRevisionRerunOption {
	return func(o *lineageRevisionRerunOptions) {
		o.mode = &mode
	}
}

func WithLineageRevisionRerunSourceEffectiveSetVersion(version int64) LineageRevisionRerunOption {
	return func(o *lineageRevisionRerunOptions) {
		o.sourceEffectiveSetVersion = &version
	}
}

func WithLineageSelectedRevisionsJSON(selectedRevisions json.RawMessage) LineageRevisionRerunOption {
	return func(o *lineageRevisionRerunOptions) {
		o.selectedRevisionsJSON = selectedRevisions
	}
}

type queryOptions struct {
	values url.Values
}

// QueryOption configures product endpoint query parameters.
type QueryOption func(*queryOptions)

// ListOption configures product list methods.
type ListOption = QueryOption

// WithLimit sets list limit.
func WithLimit(limit int) ListOption {
	return WithQueryParam("limit", strconv.Itoa(limit))
}

// WithOffset sets list offset.
func WithOffset(offset int) ListOption {
	return WithQueryParam("offset", strconv.Itoa(offset))
}

// WithQueryParam sets one query parameter value exactly as provided.
func WithQueryParam(key, value string) QueryOption {
	return func(o *queryOptions) {
		if o.values == nil {
			o.values = url.Values{}
		}
		o.values.Set(key, value)
	}
}

// WithQueryValues appends query parameter values exactly as provided.
func WithQueryValues(key string, values ...string) QueryOption {
	return func(o *queryOptions) {
		if o.values == nil {
			o.values = url.Values{}
		}
		for _, value := range values {
			o.values.Add(key, value)
		}
	}
}

// WithDeleteData sets the delete_data query parameter exactly as a boolean flag.
func WithDeleteData(deleteData bool) QueryOption {
	return WithQueryParam("delete_data", strconv.FormatBool(deleteData))
}

func WithPage(page int) QueryOption {
	return WithQueryParam("page", strconv.Itoa(page))
}

func WithPageSize(pageSize int) QueryOption {
	return WithQueryParam("page_size", strconv.Itoa(pageSize))
}

func WithPageToken(pageToken string) QueryOption {
	return WithQueryParam("page_token", pageToken)
}

func WithSearch(search string) QueryOption {
	return WithQueryParam("search", search)
}

// WithWorkspaceNameFilter filters workspace lists by name exactly as provided.
func WithWorkspaceNameFilter(name string) ListOption {
	return WithQueryParam("name", name)
}

func WithStatus(status string) QueryOption {
	return WithQueryParam("status", status)
}

func WithKind(kind string) QueryOption {
	return WithQueryParam("kind", kind)
}

func WithSemanticDomain(domain string) QueryOption {
	return WithQueryParam("domain", domain)
}

func WithType(typeValue string) QueryOption {
	return WithQueryParam("type", typeValue)
}

func WithFileID(fileID string) QueryOption {
	return WithQueryParam("file_id", fileID)
}

func WithForce(force bool) QueryOption {
	return WithQueryParam("force", strconv.FormatBool(force))
}

func WithSourceNodeExecutionID(sourceNodeExecutionID string) QueryOption {
	return WithQueryParam("source_node_execution_id", sourceNodeExecutionID)
}

type createWorkspaceOptions struct{}

// CreateWorkspaceOption configures product workspace creation.
type CreateWorkspaceOption func(*createWorkspaceOptions)

type catalogInfoOptions struct {
	description *string
}

// CatalogListOption configures catalog/database listing statistics.
type CatalogListOption func(*catalogListOptions)

type catalogListOptions struct {
	includeStats *bool
	pageSize     *int
	pageToken    *string
}

// WithCatalogIncludeStats controls child statistics (default true). When false,
// count fields are uncomputed; use the next level's list length after expansion.
func WithCatalogIncludeStats(include bool) CatalogListOption {
	return func(options *catalogListOptions) { options.includeStats = &include }
}

// WithCatalogPageSize enables pagination (1–100). Omission preserves the full-list contract.
func WithCatalogPageSize(size int) CatalogListOption {
	return func(options *catalogListOptions) { options.pageSize = &size }
}

func WithCatalogPageToken(token string) CatalogListOption {
	return func(options *catalogListOptions) { options.pageToken = &token }
}

// CatalogInfoOption configures catalog create/update requests.
type CatalogInfoOption func(*catalogInfoOptions)

type databaseTableLeavesOptions struct {
	pageSize  *int
	pageToken *string
	search    *string
}

// DatabaseTableLeavesOption configures database table-leaf pagination and search.
type DatabaseTableLeavesOption func(*databaseTableLeavesOptions)

func WithDatabaseTableLeavesPageSize(pageSize int) DatabaseTableLeavesOption {
	return func(o *databaseTableLeavesOptions) {
		o.pageSize = &pageSize
	}
}

func WithDatabaseTableLeavesPageToken(pageToken string) DatabaseTableLeavesOption {
	return func(o *databaseTableLeavesOptions) {
		o.pageToken = &pageToken
	}
}

func WithDatabaseTableLeavesSearch(search string) DatabaseTableLeavesOption {
	return func(o *databaseTableLeavesOptions) {
		o.search = &search
	}
}

type dataDashboardTableSearchOptions struct {
	pageSize  *int
	pageToken *string
	search    *string
}

// DataDashboardTableSearchOption configures cross-database dashboard table search.
type DataDashboardTableSearchOption func(*dataDashboardTableSearchOptions)

func WithDataDashboardTableSearchPageSize(pageSize int) DataDashboardTableSearchOption {
	return func(o *dataDashboardTableSearchOptions) { o.pageSize = &pageSize }
}

func WithDataDashboardTableSearchPageToken(pageToken string) DataDashboardTableSearchOption {
	return func(o *dataDashboardTableSearchOptions) { o.pageToken = &pageToken }
}

func WithDataDashboardTableSearchQuery(search string) DataDashboardTableSearchOption {
	return func(o *dataDashboardTableSearchOptions) { o.search = &search }
}

func WithCatalogDescription(description string) CatalogInfoOption {
	return func(o *catalogInfoOptions) {
		o.description = &description
	}
}

// TableColumn describes one table column in product catalog table creation.
type TableColumn struct {
	Name string `json:"name"`
	// Type is a MatrixOne SQL type, such as VARCHAR(255), TEXT or INT.
	// TEXT cannot be a primary key. Default is a literal, not a SQL expression.
	Type      string  `json:"type"`
	IsPK      *bool   `json:"is_pk,omitempty"`
	Comment   string  `json:"comment,omitempty"`
	Precision []int   `json:"precision,omitempty"`
	Default   *string `json:"default,omitempty"`
}

// ColumnLoadOption describes how one table column should load imported data.
type ColumnLoadOption struct {
	Column   string `json:"column"`
	DataFrom string `json:"data_from"`
}

type createTableOptions struct {
	comment           *string
	dataFileURL       *string
	columnLoadOptions []ColumnLoadOption
}

// CreateTableOption configures catalog table creation.
type CreateTableOption func(*createTableOptions)

func WithTableComment(comment string) CreateTableOption {
	return func(o *createTableOptions) {
		o.comment = &comment
	}
}

func WithTableDataFileURL(dataFileURL string) CreateTableOption {
	return func(o *createTableOptions) {
		o.dataFileURL = &dataFileURL
	}
}

func WithColumnLoadOption(column, dataFrom string) CreateTableOption {
	return func(o *createTableOptions) {
		o.columnLoadOptions = append(o.columnLoadOptions, ColumnLoadOption{Column: column, DataFrom: dataFrom})
	}
}

type tableLookupOptions struct {
	name         *string
	databaseID   *int
	databaseName *string
	includeStats *bool
}

// TableLookupOption configures table lookup by name and database.
type TableLookupOption func(*tableLookupOptions)

func WithTableLookupName(name string) TableLookupOption {
	return func(o *tableLookupOptions) {
		o.name = &name
	}
}

func WithTableLookupDatabaseID(databaseID int) TableLookupOption {
	return func(o *tableLookupOptions) {
		o.databaseID = &databaseID
	}
}

// WithTableLookupDatabaseName looks up a MatrixOne system table by system database name.
// Mutually exclusive with WithTableLookupDatabaseID.
func WithTableLookupDatabaseName(databaseName string) TableLookupOption {
	return func(o *tableLookupOptions) {
		o.databaseName = &databaseName
	}
}

// WithTableLookupIncludeStats controls whether table metadata includes statistics.
func WithTableLookupIncludeStats(include bool) TableLookupOption {
	return func(o *tableLookupOptions) {
		o.includeStats = &include
	}
}

type tableDataOptions struct {
	page         *int
	pageSize     *int
	name         *string
	databaseID   *int
	databaseName *string
	previewLines *int
	includeTotal *bool
}

// TableDataOption configures table data, preview, and download-data requests.
type TableDataOption func(*tableDataOptions)

func WithTableDataPage(page int) TableDataOption {
	return func(o *tableDataOptions) {
		o.page = &page
	}
}

func WithTableDataPageSize(pageSize int) TableDataOption {
	return func(o *tableDataOptions) {
		o.pageSize = &pageSize
	}
}

func WithTableDataName(name string) TableDataOption {
	return func(o *tableDataOptions) {
		o.name = &name
	}
}

func WithTableDataDatabaseID(databaseID int) TableDataOption {
	return func(o *tableDataOptions) {
		o.databaseID = &databaseID
	}
}

// WithTableDataDatabaseName reads sample/preview data for a MatrixOne system table by database name.
// Mutually exclusive with WithTableDataDatabaseID.
func WithTableDataDatabaseName(databaseName string) TableDataOption {
	return func(o *tableDataOptions) {
		o.databaseName = &databaseName
	}
}

func WithTablePreviewLines(previewLines int) TableDataOption {
	return func(o *tableDataOptions) {
		o.previewLines = &previewLines
	}
}

// WithTableDataIncludeTotal controls whether table data includes the total row count.
func WithTableDataIncludeTotal(include bool) TableDataOption {
	return func(o *tableDataOptions) {
		o.includeTotal = &include
	}
}

// FileFilter describes one catalog file-list filter.
type FileFilter struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
	Fuzzy  *bool    `json:"fuzzy,omitempty"`
}

type fileListOptions struct {
	page     *int
	pageSize *int
	order    *string
	orderBy  *string
	filters  []FileFilter
}

// FileListOption configures catalog file-list requests.
type FileListOption func(*fileListOptions)

func WithFileListPage(page int) FileListOption {
	return func(o *fileListOptions) {
		o.page = &page
	}
}

func WithFileListPageSize(pageSize int) FileListOption {
	return func(o *fileListOptions) {
		o.pageSize = &pageSize
	}
}

func WithFileListOrder(order string) FileListOption {
	return func(o *fileListOptions) {
		o.order = &order
	}
}

func WithFileListOrderBy(orderBy string) FileListOption {
	return func(o *fileListOptions) {
		o.orderBy = &orderBy
	}
}

func WithFileListFilter(name string, values ...string) FileListOption {
	return func(o *fileListOptions) {
		o.filters = append(o.filters, FileFilter{Name: name, Values: values})
	}
}

func WithFuzzyFileListFilter(name string, fuzzy bool, values ...string) FileListOption {
	return func(o *fileListOptions) {
		o.filters = append(o.filters, FileFilter{Name: name, Values: values, Fuzzy: &fuzzy})
	}
}

type downloadFileOptions struct {
	asZip *bool
}

// DownloadFileOption configures catalog file downloads.
type DownloadFileOption func(*downloadFileOptions)

func WithFileAsZip(asZip bool) DownloadFileOption {
	return func(o *downloadFileOptions) {
		o.asZip = &asZip
	}
}

type createFolderOptions struct {
	description *string
}

// CreateFolderOption configures catalog folder creation.
type CreateFolderOption func(*createFolderOptions)

func WithFolderDescription(description string) CreateFolderOption {
	return func(o *createFolderOptions) {
		o.description = &description
	}
}

type updateFolderOptions struct {
	name        *string
	description *string
}

// UpdateFolderOption configures catalog folder updates.
type UpdateFolderOption func(*updateFolderOptions)

func WithFolderName(name string) UpdateFolderOption {
	return func(o *updateFolderOptions) {
		o.name = &name
	}
}

func WithUpdatedFolderDescription(description string) UpdateFolderOption {
	return func(o *updateFolderOptions) {
		o.description = &description
	}
}

type workspaceMemberListOptions struct {
	page     *int
	pageSize *int
	filters  []map[string]any
}

// WorkspaceMemberListOption configures workspace member list requests.
type WorkspaceMemberListOption func(*workspaceMemberListOptions)

func WithMemberPage(page int) WorkspaceMemberListOption {
	return func(o *workspaceMemberListOptions) {
		o.page = &page
	}
}

func WithMemberPageSize(pageSize int) WorkspaceMemberListOption {
	return func(o *workspaceMemberListOptions) {
		o.pageSize = &pageSize
	}
}

func WithMemberFilter(name string, values ...string) WorkspaceMemberListOption {
	return func(o *workspaceMemberListOptions) {
		o.filters = append(o.filters, map[string]any{"name": name, "values": values})
	}
}

type createUserOptions struct {
	email       *string
	mobile      *string
	githubLogin *string
	description *string
	roleIDs     []int
	tags        []userTag
	autoAccept  bool
}

// CreateUserOption configures workspace member invitations.
type CreateUserOption func(*createUserOptions)

func WithUserEmail(email string) CreateUserOption {
	return func(o *createUserOptions) {
		o.email = &email
	}
}

// WithUserMobileNumber sets a raw 11-digit mainland China mobile number for a workspace invitation.
func WithUserMobileNumber(mobileNumber string) CreateUserOption {
	return func(o *createUserOptions) {
		o.mobile = &mobileNumber
	}
}

// WithUserGithubLogin sets a GitHub username for a workspace invitation.
// An existing UC account is seated immediately; an unknown account returns USER_NOT_FOUND.
func WithUserGithubLogin(login string) CreateUserOption {
	return func(o *createUserOptions) {
		o.githubLogin = &login
	}
}

func WithUserDescription(description string) CreateUserOption {
	return func(o *createUserOptions) {
		o.description = &description
	}
}

func WithUserRoleIDs(roleIDs ...int) CreateUserOption {
	return func(o *createUserOptions) {
		o.roleIDs = append(o.roleIDs, roleIDs...)
	}
}

// WithUserAutoAccept asks MOI to complete the invitation through its durable
// acceptance saga without waiting for invitee confirmation.
func WithUserAutoAccept(autoAccept bool) CreateUserOption {
	return func(o *createUserOptions) {
		o.autoAccept = autoAccept
	}
}

type userTag struct {
	ID    int    `json:"id"`
	Value string `json:"value"`
}

func WithCreateUserTag(id int, value string) CreateUserOption {
	return func(o *createUserOptions) {
		o.tags = append(o.tags, userTag{ID: id, Value: value})
	}
}

type updateUserOptions struct {
	tags []userTag
}

// UpdateUserOption configures permission user updates.
type UpdateUserOption func(*updateUserOptions)

func WithUserTag(id int, value string) UpdateUserOption {
	return func(o *updateUserOptions) {
		o.tags = append(o.tags, userTag{ID: id, Value: value})
	}
}

// AuditLogOption configures audit log list query parameters.
type AuditLogOption = QueryOption

func WithAuditMethod(method string) AuditLogOption {
	return WithQueryParam("method", method)
}

func WithAuditResult(result string) AuditLogOption {
	return WithQueryParam("result", result)
}

func WithAuditStatusCode(statusCode int) AuditLogOption {
	return WithQueryParam("status_code", strconv.Itoa(statusCode))
}

func WithAuditStartTime(start time.Time) AuditLogOption {
	return WithQueryParam("start_time", start.Format(time.RFC3339))
}

func WithAuditEndTime(end time.Time) AuditLogOption {
	return WithQueryParam("end_time", end.Format(time.RFC3339))
}

// SystemCatalogTreeOption selects a level of the system catalog navigation tree.
type SystemCatalogTreeOption func(*systemCatalogTreeOptions)

type systemCatalogTreeOptions struct {
	pageSize           *int
	pageToken          *string
	search             *string
	includeTableLeaves *bool
	databaseName       *string
	tableName          *string
}

// WithSystemCatalogTableLeaves controls table enumeration; the default is true.
func WithSystemCatalogTableLeaves(include bool) SystemCatalogTreeOption {
	return func(o *systemCatalogTreeOptions) { o.includeTableLeaves = &include }
}

// WithSystemCatalogDatabase limits the system tree to one database.
func WithSystemCatalogDatabase(name string) SystemCatalogTreeOption {
	return func(o *systemCatalogTreeOptions) { o.databaseName = &name }
}

// WithSystemCatalogTableName resolves an exact system table in the navigation tree.
func WithSystemCatalogTableName(name string) SystemCatalogTreeOption {
	return func(o *systemCatalogTreeOptions) { o.tableName = &name }
}

// CatalogUploadOption configures one uploaded Catalog file.
type CatalogUploadOption func(*catalogUploadOptions)
type catalogUploadOptions struct {
	contentType       string
	externalSourceURI string
}

// WithCatalogUploadContentType supplies the file's multipart MIME type.
func WithCatalogUploadContentType(contentType string) CatalogUploadOption {
	return func(options *catalogUploadOptions) { options.contentType = contentType }
}

// WithCatalogUploadExternalSourceURI sets the optional trusted http(s) origin.
// An empty value omits the field and does not clear an origin already stored.
func WithCatalogUploadExternalSourceURI(uri string) CatalogUploadOption {
	return func(options *catalogUploadOptions) { options.externalSourceURI = uri }
}

// CatalogTreeOption configures bounded navigation search or exact path lookup.
type CatalogTreeOption func(*catalogTreeOptions)
type catalogTreeOptions struct {
	PageSize           *int     `json:"page_size,omitempty"`
	PageToken          *string  `json:"page_token,omitempty"`
	Search             *string  `json:"search,omitempty"`
	ResourceTypes      []string `json:"resource_types,omitempty"`
	ResourceID         *int64   `json:"resource_id,omitempty"`
	IncludeTableLeaves *bool    `json:"include_table_leaves,omitempty"`
}

func WithCatalogTreePageSize(size int) CatalogTreeOption {
	return func(o *catalogTreeOptions) { o.PageSize = &size }
}
func WithCatalogTreePageToken(token string) CatalogTreeOption {
	return func(o *catalogTreeOptions) { o.PageToken = &token }
}
func WithCatalogTreeSearch(search string) CatalogTreeOption {
	return func(o *catalogTreeOptions) { o.Search = &search }
}
func WithCatalogTreeResourceTypes(kinds ...string) CatalogTreeOption {
	return func(o *catalogTreeOptions) { o.ResourceTypes = slices.Clone(kinds) }
}
func WithCatalogTreeResourceID(id int64) CatalogTreeOption {
	return func(o *catalogTreeOptions) { o.ResourceID = &id }
}

// WithCatalogTreeTableLeaves controls table enumeration in navigation and search.
// Omission uses the server default; false excludes table leaves.
func WithCatalogTreeTableLeaves(include bool) CatalogTreeOption {
	return func(o *catalogTreeOptions) { o.IncludeTableLeaves = &include }
}

func WithSystemCatalogPageSize(size int) SystemCatalogTreeOption {
	return func(o *systemCatalogTreeOptions) { o.pageSize = &size }
}
func WithSystemCatalogPageToken(token string) SystemCatalogTreeOption {
	return func(o *systemCatalogTreeOptions) { o.pageToken = &token }
}
func WithSystemCatalogSearch(search string) SystemCatalogTreeOption {
	return func(o *systemCatalogTreeOptions) { o.search = &search }
}
