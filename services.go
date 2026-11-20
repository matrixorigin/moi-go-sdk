package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/encoding/protojson"
)

// DefaultQueryHistoryLookback is the range sent when neither history boundary is configured.
const DefaultQueryHistoryLookback = 5 * time.Minute

// ProductData is decoded product response data for product methods whose
// domain-specific result contract has not been promoted to protobuf yet.
// JSON numbers are decoded as json.Number so int64 identifiers and counts
// preserve their exact product values.
type ProductData struct {
	Value any
}

// MarshalJSON preserves the product data shape. Value is a Go storage detail,
// not a field in the API response (or in the CLI's JSON output).
func (d ProductData) MarshalJSON() ([]byte, error) { return json.Marshal(d.Value) }

func decodeExactJSONWithNumbers(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("product JSON must contain exactly one value")
	}
	return nil
}

func (d *ProductData) UnmarshalJSON(data []byte) error {
	var value any
	if err := decodeExactJSONWithNumbers(data, &value); err != nil {
		return err
	}
	d.Value = value
	return nil
}

func (d ProductData) AsMap() (map[string]any, bool) {
	value, ok := d.Value.(map[string]any)
	return value, ok
}

// WorkspaceService is a route-aligned escape hatch. Prefer typed Client workspace helpers and WorkspaceHandle for user-facing workflows.
type WorkspaceService struct {
	client *Client
}

// WorkspaceMemberService is a route-aligned escape hatch. Prefer typed WorkspaceHandle member methods and WorkspaceMemberHandle for user-facing workflows.
type WorkspaceMemberService struct{ workspaceService }

// IAMService is a route-aligned escape hatch. Prefer typed WorkspaceHandle IAM methods for user-facing workflows.
type IAMService struct{ workspaceService }

// CatalogService is a route-aligned escape hatch. Prefer typed catalog, database, table, volume, folder, and file handles for user-facing workflows.
type CatalogService struct{ workspaceService }

// MemoryService is the route-aligned escape hatch for legacy consumer-memory
// and consumer-profile routes. Prefer typed ConsumerMemoryHandle methods when
// using a legacy server.
//
// Deprecated: These routes are not part of the current MOI product API.
type MemoryService struct{ workspaceService }

// KnowledgeService is a route-aligned escape hatch. Prefer typed semantic model and entry handles for user-facing workflows.
type KnowledgeService struct{ workspaceService }

// WorkflowService is a route-aligned escape hatch. Prefer typed workflow, run, operator, template, and WorkItem API handles for user-facing workflows.
type WorkflowService struct{ workspaceService }

// AcceptedWorkflowCandidate is the typed handoff produced only from a validated
// server acceptance response. Deployment consumes this handle instead of a bare
// pending candidate reference.
type AcceptedWorkflowCandidate struct {
	ref        WorkflowCandidateRef
	acceptance *WorkflowCandidateAcceptance
}

// Ref returns the exact task/artifact identity accepted by the server.
func (c *AcceptedWorkflowCandidate) Ref() WorkflowCandidateRef {
	if c == nil {
		return WorkflowCandidateRef{}
	}
	return c.ref
}

// Acceptance returns the server acceptance bundle for review and provenance inspection.
func (c *AcceptedWorkflowCandidate) Acceptance() *WorkflowCandidateAcceptance {
	if c == nil {
		return nil
	}
	return c.acceptance
}

// AgentService is a route-aligned escape hatch. Prefer typed AgentHandle A2A helpers for user-facing workflows.
type AgentService struct{ workspaceService }

// ModelService is a route-aligned escape hatch. Prefer typed provider backend and router handles for user-facing workflows.
type ModelService struct{ workspaceService }

// ConnectorService is a route-aligned escape hatch. Prefer typed connector, import task, export task, and connector file handles for user-facing workflows.
type ConnectorService struct{ workspaceService }

// SQLService is a route-aligned escape hatch. Prefer typed SQL metadata, workbook, query, and statement handles for user-facing workflows.
type SQLService struct{ workspaceService }

// ComputeService is a route-aligned escape hatch. Prefer typed compute resource handles for user-facing workflows.
type ComputeService struct{ workspaceService }

// DashboardService is a route-aligned escape hatch. Prefer typed workflow handles for user-facing workflows.
type DashboardService struct{ workspaceService }

// DataShareService is a route-aligned escape hatch. Prefer typed publish and subscription handles for user-facing workflows.
type DataShareService struct{ workspaceService }

type RESTParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type RESTRequest struct {
	Path    string          `json:"path"`
	Method  string          `json:"method"`
	Params  []RESTParameter `json:"params,omitempty"`
	Headers []RESTParameter `json:"headers,omitempty"`
	JSON    string          `json:"json,omitempty"`
	Data    []RESTParameter `json:"data,omitempty"`
}

type RESTPagination struct {
	Type      string `json:"type"`
	Param     string `json:"param,omitempty"`
	Start     *int   `json:"start,omitempty"`
	Step      int    `json:"step,omitempty"`
	NextPath  string `json:"next_path,omitempty"`
	TotalPath string `json:"total_path,omitempty"`
}

type RESTSchemaFormat string

type RESTPreview struct {
	RESTRequest
	ReadOnlyPOST  bool             `json:"read_only_post,omitempty"`
	RecordsPath   string           `json:"records_path,omitempty"`
	SchemaPath    string           `json:"schema_path,omitempty"`
	SchemaFormat  RESTSchemaFormat `json:"schema_format,omitempty"`
	Pagination    RESTPagination   `json:"pagination"`
	CursorParam   string           `json:"cursor_param,omitempty"`
	InitialCursor string           `json:"initial_cursor,omitempty"`
}

type StructuredSourceRef struct {
	ConnectorID string `json:"connector_id"`
	SourceType  string `json:"source_type"`
	Database    string `json:"database,omitempty"`
	Schema      string `json:"schema,omitempty"`
	Table       string `json:"table,omitempty"`
	Collection  string `json:"collection,omitempty"`
}

type StructuredSourceCapabilities struct {
	ListDatabases        bool `json:"list_databases"`
	ListSchemas          bool `json:"list_schemas"`
	ListTables           bool `json:"list_tables"`
	DescribeSchema       bool `json:"describe_schema"`
	Sample               bool `json:"sample"`
	BackfillStartResolve bool `json:"backfill_start_resolve"`
	HivePartitions       bool `json:"hive_partitions"`
	MongodbFlatten       bool `json:"mongodb_flatten"`
}

type StructuredSourceDescriptor struct {
	ConnectorID           string                       `json:"connector_id"`
	ConnectorName         string                       `json:"connector_name"`
	SourceType            string                       `json:"source_type"`
	Enabled               bool                         `json:"enabled"`
	DisabledReason        string                       `json:"disabled_reason"`
	CapabilityProfileHash string                       `json:"capability_profile_hash,omitempty"`
	Capabilities          StructuredSourceCapabilities `json:"capabilities"`
}

type StructuredSourceListResult struct {
	Sources []StructuredSourceDescriptor `json:"sources"`
}

type StructuredSourceNamedRef struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

type StructuredSourceDatabasesResult struct {
	Databases       []StructuredSourceNamedRef `json:"databases"`
	EndpointResults []json.RawMessage          `json:"endpoint_results"`
}

type StructuredSourceSchemasResult struct {
	Schemas         []StructuredSourceNamedRef `json:"schemas"`
	EndpointResults []json.RawMessage          `json:"endpoint_results"`
}

type StructuredSourceObjectsRequest struct {
	ConnectorID string `json:"connector_id"`
	SourceType  string `json:"source_type"`
	Database    string `json:"database"`
	Schema      string `json:"schema,omitempty"`
}

type StructuredSourceObject struct {
	Database    string `json:"database,omitempty"`
	Schema      string `json:"schema,omitempty"`
	Table       string `json:"table,omitempty"`
	Collection  string `json:"collection,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	ObjectKind  string `json:"object_kind"`
}

type StructuredSourceObjectsResult struct {
	Objects         []StructuredSourceObject `json:"objects"`
	EndpointResults []json.RawMessage        `json:"endpoint_results"`
}

type StructuredSourceSchemaRequest struct {
	ConnectorID      string                     `json:"connector_id"`
	Source           StructuredSourceRef        `json:"source"`
	MongoSampleLimit *int32                     `json:"mongo_sample_limit,omitempty"`
	KafkaSchema      *StructuredKafkaSchemaSpec `json:"kafka_schema,omitempty"`
}

type StructuredKafkaSampleSpec struct {
	TopicMode    string   `json:"topic_mode"`
	Topics       []string `json:"topics,omitempty"`
	TopicPattern string   `json:"topic_pattern,omitempty"`
}

type StructuredKafkaSchemaSpec struct {
	KeyFormat    string                     `json:"key_format"`
	ValueFormat  string                     `json:"value_format"`
	SchemaSource string                     `json:"schema_source"`
	JSONSchema   string                     `json:"json_schema,omitempty"`
	Sample       *StructuredKafkaSampleSpec `json:"sample,omitempty"`
	SampleLimit  int32                      `json:"sample_limit,omitempty"`
}

type StructuredSourceSchemaResult struct {
	SchemaSnapshot       json.RawMessage   `json:"schema_snapshot"`
	MappingDraft         json.RawMessage   `json:"mapping_draft,omitempty"`
	TargetTypeCatalog    json.RawMessage   `json:"target_type_catalog,omitempty"`
	FieldCompatibilities json.RawMessage   `json:"field_compatibilities,omitempty"`
	SourceSchemaHash     string            `json:"source_schema_hash,omitempty"`
	SourceObjectHash     string            `json:"source_object_hash,omitempty"`
	ConnectorConfigHash  string            `json:"connector_config_hash,omitempty"`
	MappingHash          string            `json:"mapping_hash,omitempty"`
	FlattenHash          string            `json:"flatten_hash,omitempty"`
	EndpointResults      []json.RawMessage `json:"endpoint_results"`
}

type StructuredSourceCapabilityResult struct {
	CapabilityProfileHash       string   `json:"capability_profile_hash"`
	MysqlCDCAvailable           bool     `json:"mysql_cdc_available"`
	DorisCDCAvailable           bool     `json:"doris_cdc_available"`
	DisabledReasons             []string `json:"disabled_reasons"`
	DorisFlightSQLReadAvailable bool     `json:"doris_flight_sql_read_available"`
	DorisThriftReadAvailable    bool     `json:"doris_thrift_read_available"`
	DorisJDBCReadAvailable      bool     `json:"doris_jdbc_read_available"`
	DorisReadMode               string   `json:"doris_read_mode"`
	DorisReadDisabledReasons    []string `json:"doris_read_disabled_reasons"`
}

type StructuredSourceSampleResult struct {
	Rows         []json.RawMessage `json:"rows"`
	NextPosition json.RawMessage   `json:"next_position,omitempty"`
	HasMore      bool              `json:"has_more"`
}

type StructuredBackfillStartResolveRequest struct {
	ConnectorID      string              `json:"connector_id"`
	Source           StructuredSourceRef `json:"source"`
	Watermark        json.RawMessage     `json:"watermark,omitempty"`
	StartMode        string              `json:"start_mode"`
	CustomStartValue json.RawMessage     `json:"custom_start_value,omitempty"`
}

type StructuredBackfillStartResolveResult struct {
	StartPosition    json.RawMessage `json:"start_position"`
	DisplayValue     string          `json:"display_value,omitempty"`
	SourceObjectHash string          `json:"source_object_hash,omitempty"`
	SourceSchemaHash string          `json:"source_schema_hash,omitempty"`
}

type Workspace = product.Workspace
type WorkspaceListResult = product.WorkspaceListResult
type WorkspaceCreateResult = product.WorkspaceCreateResult
type WorkspaceClaimResult = product.WorkspaceClaimResult
type AgentCardResult = product.AgentCardResult
type AgentA2AResult = product.AgentA2AResult
type TriggerFileWorkflowResult = product.TriggerFileWorkflowResult
type CatalogFileUploadLocation = product.CatalogFileUploadLocation
type UploadFileResult = product.UploadFileResult
type RuntimeArtifactUploadResult = product.RuntimeArtifactUploadResult
type RuntimeImageAnalysisResult = product.RuntimeImageAnalysisResult
type FileInfoResult = product.FileInfoResult
type CreateFolderResult = product.CreateFolderResult
type DataTag = product.DataTag
type DataTagListResult = product.DataTagListResult
type PathResult = product.PathResult
type URLResult = product.URLResult
type BoolDataResult = product.BoolDataResult
type CatalogNL2SQLRunResult = product.CatalogNL2SQLRunResult
type HealthResult = product.HealthResult
type Memory = product.Memory
type MemoryListResult = product.MemoryListResult
type ConsumerProfileRenderResult = product.ConsumerProfileRenderResult

type CatalogTreeResult = product.CatalogTreeResult
type CatalogListResult = product.CatalogListResult
type CatalogAllowedActionsResult = product.CatalogAllowedActionsResult
type IDResult = product.IDResult
type CatalogInfoResult = product.CatalogInfoResult
type DatabaseListResult = product.DatabaseListResult
type DatabaseInfoResult = product.DatabaseInfoResult
type DatabaseChildrenResult = product.DatabaseChildrenResult
type DatabaseTableLeavesResult = product.DatabaseTableLeavesResult
type DataDashboardTableSearchResult = product.DataDashboardTableSearchResult
type VolumeInfoResult = product.VolumeInfoResult
type CatalogCreateTableResult = product.CatalogCreateTableResult
type CatalogTableColumn = product.CatalogTableColumn
type CatalogTableStatistic = product.CatalogTableStatistic
type CatalogTableInfoResult = product.CatalogTableInfoResult
type CatalogMultiTableInfoResult = product.CatalogMultiTableInfoResult
type CatalogTableDataColumn = product.CatalogTableDataColumn
type CatalogTableDataResult = product.CatalogTableDataResult
type CatalogTableOverviewItem = product.CatalogTableOverviewItem
type CatalogTableOverviewResult = product.CatalogTableOverviewResult
type CatalogFileFolder = product.CatalogFileFolder
type CatalogFileListResult = product.CatalogFileListResult
type WorkspaceFileListResult = product.WorkspaceFileListResult
type WorkspaceFilePlacement = product.WorkspaceFilePlacement
type CatalogDataAsset = product.CatalogDataAsset
type CatalogDataDerivation = product.CatalogDataDerivation
type CatalogParsedManifest = product.CatalogParsedManifest
type CatalogDataAssetResolveResult = product.CatalogDataAssetResolveResult
type CatalogBatchGetDataAssetsResult = product.CatalogBatchGetDataAssetsResult
type CatalogFileArtifactResult = product.CatalogFileArtifactResult
type CatalogFileArtifactResolveItem = product.CatalogFileArtifactResolveItem
type CatalogBatchResolveFileArtifactsResult = product.CatalogBatchResolveFileArtifactsResult
type ConnectorValidateResult = product.ConnectorValidateResult
type RESTReadResult = product.RESTReadResult
type RESTSourceField = product.RESTSourceField
type Connector = product.Connector
type ConnectorListResult = product.ConnectorListResult
type ConnectorFile = product.ConnectorFile
type ConnectorFileListResult = product.ConnectorFileListResult
type UploadResult = product.UploadResult
type UploadResponse = product.UploadResponse
type LocalUploadResult = product.LocalUploadResult
type PreviewRow = product.PreviewRow
type PreviewSheet = product.PreviewSheet
type PreviewFileResult = product.PreviewFileResult
type DBListDatabasesResult = product.DBListDatabasesResult
type DBListSchemasResult = product.DBListSchemasResult
type DBListTablesResult = product.DBListTablesResult
type ColumnSchemaResult = product.ColumnSchema
type DBSourceSchemaResult = product.DBSourceSchemaResult
type MODatabaseListResult = product.MODatabaseListResult
type MODatabaseTableListResult = product.MODatabaseTableListResult
type MOCheckTableNameResult = product.MOCheckTableNameResult
type MOColumnMappingResult = product.MOColumnMappingResult
type FileDownloadResult = product.FileDownloadResult
type ImportTask = product.ImportTask
type ImportTaskStats = product.ImportTaskStats

type ImportTaskListResult = product.ImportTaskListResult
type ImportTaskCreateResult = product.ImportTaskCreateResult
type ImportTaskInfoResult = product.ImportTaskInfoResult
type ImportTaskFile = product.ImportTaskFile
type ImportTaskFileListResult = product.ImportTaskFileListResult
type ImportTaskRun = product.ImportTaskRun
type ImportTaskRunListResult = product.ImportTaskRunListResult
type ExportTask = product.ExportTask
type ExportTaskInfo = product.ExportTaskInfo
type ExportTaskListResult = product.ExportTaskListResult
type ExportTaskStats = product.ExportTaskStats
type ExportTaskFileStats = product.ExportTaskFileStats
type ExportTaskFileResult = product.ExportTaskFile
type ExportTaskFilesResult = product.ExportTaskFilesResult
type ExportTaskStateResult = product.ExportTaskStateResult
type PublishSummaryResult = product.PublishSummaryResult
type CheckNameResult = product.CheckNameResult
type DataShareWorkspaceName = product.DataShareWorkspaceName
type DataShareWorkspaceNameListResult = product.DataShareWorkspaceNameListResult
type DataSharePublishTarget = product.DataSharePublishTarget
type DataShareScope = product.DataShareScope
type DataShareSourceTable = product.DataShareSourceTable
type DataShareSourceTableListResult = product.DataShareSourceTableListResult
type DataSharePublish = product.DataSharePublish
type DataSharePublishListResult = product.DataSharePublishListResult
type DataShareSubscription = product.DataShareSubscription
type DataShareSubscriptionListResult = product.DataShareSubscriptionListResult
type ComputeResource = product.ComputeResource
type ComputeResourcePyDeps = product.ComputeResourcePyDeps
type ComputeResourceOwnership = product.ComputeResourceOwnership
type AdminComputeResource = product.AdminComputeResource
type AdminComputeResourceListResult = product.AdminComputeResourceListResult
type ComputeResourceWorkspaceInfo = product.ComputeResourceWorkspaceInfo
type ComputeResourceSpec = product.ComputeResourceSpec
type ComputeResourceSpecManagementPermission = product.ComputeResourceSpecManagementPermission
type WorkerImageSelection = product.WorkerImageSelection
type WorkerImage = product.WorkerImage
type ComputeWorkerRuntime = product.ComputeWorkerRuntime
type ComputeWorkerRuntimeListResult = product.ComputeWorkerRuntimeListResult
type BoundWorkflowInfo = product.BoundWorkflowInfo
type ComputeDeletePreflightResult = product.ComputeDeletePreflightResult
type WorkitemAPIServiceEnvelope = product.WorkitemAPIServiceEnvelope
type WorkitemAPIService = product.WorkitemAPIService
type WorkitemAPIServiceInvokeInfo = product.WorkitemAPIServiceInvokeInfo
type WorkitemAPIServiceInvokeEnvelope = product.WorkitemAPIServiceInvokeEnvelope
type WorkitemAPIServiceInvokeResult = product.WorkitemAPIServiceInvokeResult
type CustomOperator = product.CustomOperator
type CustomOperatorListEnvelope = product.CustomOperatorListEnvelope
type CustomOperatorCodeEnvelope = product.CustomOperatorCodeEnvelope
type CustomOperatorSourceEnvelope = product.CustomOperatorSourceEnvelope
type CustomOperatorTestEnvelope = product.CustomOperatorTestEnvelope
type CustomOperatorTestResult = product.CustomOperatorTestResult

type WorkflowTemplateList = product.WorkflowTemplateList
type WorkflowTemplate = product.WorkflowTemplate
type WorkflowTemplateDeleteEnvelope = product.WorkflowTemplateDeleteEnvelope
type LineageOverview = product.LineageOverview
type LineageArtifactNode = product.LineageArtifactNode
type LineageOutputBlocks = product.LineageOutputBlocks
type LineageBlockRevisions = product.LineageBlockRevisions
type LineageBlockRevision = product.LineageBlockRevision
type LineageEffectiveRevisions = product.LineageEffectiveRevisions
type LineageRerun = product.LineageRerun
type WorkitemCatalogList = product.WorkitemCatalogList
type WorkitemCatalogItem = product.WorkitemCatalogItem
type WorkitemCatalogDetail = product.WorkitemCatalogDetail
type WorkflowSummary = product.WorkflowSummary
type WorkflowDeploymentEnvelope = product.WorkflowDeploymentEnvelope
type WorkflowCandidateAcceptance = product.WorkflowCandidateAcceptance
type WorkflowCandidateBundle = product.WorkflowCandidateBundle
type WorkflowHistoryPage = product.WorkflowHistoryPage
type WorkflowHistoryEntry = product.WorkflowHistoryEntry
type WorkflowHistoryRestoreRequest struct {
	ContextID    string
	RequestID    string
	CandidateRef WorkflowCandidateRef
	ParentRef    *WorkflowCandidateRef
}

func (r WorkflowHistoryRestoreRequest) body() map[string]any {
	body := map[string]any{
		"context_id":    r.ContextID,
		"request_id":    r.RequestID,
		"candidate_ref": map[string]any{"task_id": r.CandidateRef.TaskID, "artifact_id": r.CandidateRef.ArtifactID},
	}
	if r.ParentRef != nil {
		body["parent_ref"] = map[string]any{"task_id": r.ParentRef.TaskID, "artifact_id": r.ParentRef.ArtifactID}
	}
	return body
}

type WorkflowCandidateConfirmation = product.WorkflowCandidateConfirmation
type WorkflowCandidateProposalRejection = product.WorkflowCandidateProposalRejection
type WorkflowCandidateDiagnostic = product.WorkflowCandidateDiagnostic
type WorkflowListEnvelope = product.WorkflowListEnvelope
type WorkflowListItem = product.WorkflowListItem
type WorkflowDetail = product.WorkflowDetail
type WorkflowEnvelope = product.WorkflowEnvelope
type WorkflowDeleteEnvelope = product.WorkflowDeleteEnvelope
type WorkflowRunEnvelope = product.WorkflowRunEnvelope
type WorkflowRunResult = product.WorkflowRunResult
type WorkflowBatchRetryEnvelope = product.WorkflowBatchRetryEnvelope
type WorkflowExecutionListEnvelope = product.WorkflowExecutionListEnvelope
type WorkflowExecutionListItem = product.WorkflowExecutionListItem
type WorkflowDispatchJob = product.WorkflowDispatchJob

type WorkflowDispatchJobEnvelope = product.WorkflowDispatchJobEnvelope

type WorkflowExecutionEnvelope = product.WorkflowExecutionEnvelope
type WorkflowExecutionDetail = product.WorkflowExecutionDetail
type WorkflowExecutionResultEnvelope = product.WorkflowExecutionResultEnvelope
type WorkflowExecutionResultDetail = product.WorkflowExecutionResultDetail
type WorkflowExecutionNodeState = product.WorkflowExecutionNodeState
type WorkflowFileExecutionList = product.WorkflowFileExecutionList
type WorkflowFileExecution = product.WorkflowFileExecution
type WorkflowImportContextEnvelope = product.WorkflowImportContextEnvelope
type WorkflowImportContext = product.WorkflowImportContext
type WorkflowImportFileListEnvelope = product.WorkflowImportFileListEnvelope
type WorkflowImportFile = product.WorkflowImportFile
type WorkflowExecutionNodeDetailEnvelope = product.WorkflowExecutionNodeDetailEnvelope
type WorkflowExecutionNodeDetail = product.WorkflowExecutionNodeDetail
type WorkflowExecutionNodeRun = product.WorkflowExecutionNodeRun
type WorkflowDeveloperLogs = product.DeveloperLogs
type WorkflowRerunPlanEnvelope = product.WorkflowRerunPlanEnvelope
type WorkflowRerunPlan = product.WorkflowRerunPlan
type WorkflowRerunAffectedNode = product.WorkflowRerunAffectedNode
type WorkflowRerunPlanValidation = product.WorkflowRerunPlanValidation
type WorkflowRerunNodeExecutionCandidate = product.WorkflowRerunNodeExecutionCandidate
type WorkflowRerunEnvelope = product.WorkflowRerunEnvelope
type WorkflowRerunBranchListEnvelope = product.WorkflowRerunBranchListEnvelope
type WorkflowRerunBranch = product.WorkflowRerunBranch
type SemanticModelInfo = product.SemanticModelInfo
type SemanticModelSourceCounts = product.SemanticModelSourceCounts
type SemanticModelListResult = product.SemanticModelListResult
type KnowledgeReferencingAgentsPage = product.KnowledgeReferencingAgentsPage
type KnowledgeReferencingAgent = product.KnowledgeReferencingAgent
type SemanticModelTagStat = product.SemanticModelTagStat
type SemanticModelTagListResult = product.SemanticModelTagListResult
type SemanticEntry = product.SemanticEntry
type SemanticEntryListResult = product.SemanticEntryListResult
type SemanticMutationResult = product.SemanticMutationResult
type SemanticImportResult = product.SemanticImportResult
type SemanticExportResult = product.SemanticExportResult
type SemanticValidateResult = product.SemanticValidateResult
type KnowledgeBaseDataDomain = product.KnowledgeBaseDataDomain
type SemanticModelSource = product.SemanticModelSource
type KnowledgeBaseSourceJobRun = product.KnowledgeBaseSourceJobRun
type KnowledgeBaseSourceJobView = product.KnowledgeBaseSourceJobView
type SemanticModelWithSourcesResult = product.SemanticModelWithSourcesResult
type SemanticModelEmptyResult = product.SemanticModelEmptyResult
type SemanticModelAppendSourcesResult = product.SemanticModelAppendSourcesResult
type SemanticModelSourceListResult = product.SemanticModelSourceListResult
type SemanticModelSourceSelectionPreviewResult = product.SemanticModelSourceSelectionPreviewResult
type SemanticModelSourceJobListResult = product.SemanticModelSourceJobListResult
type SemanticModelSourceExistenceResult = product.SemanticModelSourceExistenceResult
type SemanticModelSourcePreview = product.SemanticModelSourcePreview
type SemanticModelSourceFileInfo = product.SemanticModelSourceFileInfo
type SemanticModelSegmentStatus = product.SemanticModelSegmentStatus
type SemanticModelSegment = product.SemanticModelSegment
type SemanticModelDocumentSegment = product.SemanticModelDocumentSegment
type SemanticModelSourceDocument = product.SemanticModelSourceDocument
type SemanticModelSegmentMutationResult = product.SemanticModelSegmentMutationResult
type SemanticModelSourceGovernanceResult = product.SemanticModelSourceGovernanceResult
type SQLWorkbook = product.SQLWorkbook
type SQLWorkbookListResult = product.SQLWorkbookListResult
type SQLWorkbookDetailResult = product.SQLWorkbookDetailResult
type SQLWorkbookVersion = product.SQLWorkbookVersion
type SQLWorkbookVersionListResult = product.SQLWorkbookVersionListResult
type SQLWorkbookVersionCreateResult = product.SQLWorkbookVersionCreateResult
type SQLDatabaseMetadata = product.SQLDatabaseMetadata
type SQLDatabaseListResult = product.SQLDatabaseListResult
type SQLDatabaseTreeResult = product.SQLDatabaseTreeResult
type SQLTableMetadata = product.SQLTableMetadata
type SQLTableGroup = product.SQLTableGroup
type SQLDatabaseTablesResult = product.SQLDatabaseTablesResult
type SQLColumnMetadata = product.SQLColumnMetadata
type SQLTableInfoResult = product.SQLTableInfoResult
type SQLTableColumnsResult = product.SQLTableColumnsResult
type SQLTableDDLResult = product.SQLTableDDLResult
type SQLQueryStartResult = product.SQLQueryStartResult
type SQLQueryOperation = product.SQLQueryOperation
type SQLQueryResultColumn = product.SQLQueryResultColumn
type SQLQueryResultItem = product.SQLQueryResultItem
type SQLEmptySet = product.SQLEmptySet
type SQLQueryResult = product.SQLQueryResult
type SQLQueryHistoryOverviewResult = product.SQLQueryHistoryOverviewResult
type SQLStatementRecord = product.SQLStatementRecord
type SQLQueryHistoryResult = product.SQLQueryHistoryResult
type SQLQueryProfileResult = product.SQLQueryProfileResult
type SQLExplainData = product.SQLExplainData
type SQLExplainStep = product.SQLExplainStep
type SQLGraphData = product.SQLGraphData
type SQLGraphNode = product.SQLGraphNode
type SQLGraphEdge = product.SQLGraphEdge
type SQLGraphLabel = product.SQLGraphLabel
type SQLGraphGlobal = product.SQLGraphGlobal
type SQLStatistics = product.SQLStatistics
type SQLCost = product.SQLCost
type SQLTotalStats = product.SQLTotalStats
type SQLStatisticValue = product.SQLStatisticValue
type ProviderBackend = product.ProviderBackend
type ProviderBackendListResult = product.ProviderBackendListResult
type ProviderEndpoint = product.ProviderEndpoint
type ProviderEndpointListResult = product.ProviderEndpointListResult
type ProviderRouterConfig = product.ProviderRouterConfig
type ProviderModel = product.ProviderModel
type ProviderModelListResult = product.ProviderModelListResult
type ProviderProbeModelsResult = product.ProviderProbeModelsResult
type ProviderSetupBackendResult = product.ProviderSetupBackendResult
type ProviderMutationResult = product.ProviderMutationResult

// ProviderBackendService is a route-aligned escape hatch. Prefer typed provider backend and router handles for user-facing workflows.
type ProviderBackendService struct {
	workspaceService
	basePath string
}

// parserBackendList retains the parser endpoint's raw-array contract while
// distinguishing its successful empty response from a missing typed envelope.
type parserBackendList struct {
	backends []*ProviderBackend
}

func (*parserBackendList) acceptsProductNull() {}

// parserEndpointList retains the parser endpoint's raw-array contract while
// distinguishing its successful empty response from a missing typed envelope.
type parserEndpointList struct {
	endpoints []*ProviderEndpoint
}

func (*parserEndpointList) acceptsProductNull() {}

// MultipartFile describes one multipart file part.
type MultipartFile struct {
	ContentType string
	FieldName   string
	Filename    string
	Reader      io.Reader
}

type connectorUploadOptions struct {
	fields map[string]string
	files  []MultipartFile
}

const (
	// A2A method constants are explicit protocol-control values for low-level A2A calls.
	// They are not user-story SDK entry points or AI task-selection sources; prefer AgentHandle typed A2A helpers for promoted workflows.
	A2AMethodSend             = "message/send"
	A2AMethodStream           = "message/stream"
	A2AMethodTasksGet         = "tasks/get"
	A2AMethodTasksCancel      = "tasks/cancel"
	A2AMethodTasksResubscribe = "tasks/resubscribe"
	A2AMethodInputSubmit      = "matrixflow/input.submit"
)

// AgentSelector identifies a registered product agent by code or ID and its owning workspace.
type AgentSelector struct {
	AgentCode        string `json:"agent_code,omitempty"`
	AgentID          string `json:"agent_id,omitempty"`
	AgentWorkspaceID string `json:"agent_workspace_id,omitempty"`
}

// A2ARequest is the JSON-RPC envelope for an explicit low-level A2A method call.
// It is a protocol-control value, not a user-story SDK entry point.
type A2ARequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Model   string          `json:"model,omitempty"`
	// NoWait returns a non-terminal message/send task result as submitted
	// instead of following it to a terminal state. It is a client-side
	// protocol-control switch and is never sent on the wire.
	NoWait bool `json:"-"`
}

// AgentA2ARequest combines an agent selector with an explicit low-level A2A JSON-RPC request.
// It is a protocol-control value for route-aligned calls, not an AI task-selection source.
type AgentA2ARequest struct {
	AgentSelector
	A2ARequest
}

// ConnectorUploadOption configures connector multipart upload requests.
type ConnectorUploadOption func(*connectorUploadOptions)

// WithConnectorUploadField adds one form field exactly as provided.
func WithConnectorUploadField(name, value string) ConnectorUploadOption {
	return func(o *connectorUploadOptions) {
		if o.fields == nil {
			o.fields = map[string]string{}
		}
		o.fields[name] = value
	}
}

// WithConnectorUploadFile adds one file part to a connector upload.
func WithConnectorUploadFile(fieldName, filename string, reader io.Reader) ConnectorUploadOption {
	return func(o *connectorUploadOptions) {
		o.files = append(o.files, MultipartFile{FieldName: fieldName, Filename: filename, Reader: reader})
	}
}

// AgentCard returns the generic A2A agent card through the product endpoint.
func (s *AgentService) AgentCard(ctx context.Context, opts ...QueryOption) (*AgentCardResult, error) {
	out := &product.AgentCardResult{}
	err := s.do(ctx, http.MethodGet, withQuery("/agents/card", opts), nil, out)
	return out, err
}

// A2A sends a generic A2A JSON-RPC request through the product endpoint.
// It is an explicit protocol-control escape hatch; prefer AgentHandle typed A2A helpers for promoted workflows.
// A message/send result carrying a non-terminal task (submitted/working) is
// followed to its persisted terminal state before returning, unless the
// request sets NoWait.
func (s *AgentService) A2A(ctx context.Context, req AgentA2ARequest) (*AgentA2AResult, error) {
	if err := validateAgentA2ARequest(req, false); err != nil {
		return nil, err
	}
	out := &product.AgentA2AResult{}
	if err := s.do(ctx, http.MethodPost, "/agents/a2a", req, out); err != nil {
		return out, err
	}
	return s.foldA2ASendResult(ctx, req, out)
}

// A2AStream sends a generic A2A streaming request through the product endpoint.
// It is an explicit protocol-control escape hatch; prefer AgentHandle typed A2A helpers for promoted workflows.
func (s *AgentService) A2AStream(ctx context.Context, req AgentA2ARequest) (*StreamResponse, error) {
	if err := validateAgentA2ARequest(req, true); err != nil {
		return nil, err
	}
	return s.doStream(ctx, http.MethodPost, "/agents/a2a", req)
}

const agentPackageMediaType = "application/vnd.moi.agent+zip"

// LoadAgentPackage loads a raw .moiagent archive into the workspace.
func (s *AgentService) LoadAgentPackage(ctx context.Context, archive io.Reader) (*ProductData, error) {
	path, err := agentResourcePath(s.workspaceID, "agent-packages", "/load")
	if err != nil {
		return nil, err
	}
	var out ProductData
	err = s.doRaw(ctx, http.MethodPost, path, archive, agentPackageMediaType, contentTypeJSON, &out)
	return &out, err
}

// UploadWorkspaceFile uploads a file to the workspace file resource.
func (s *AgentService) UploadWorkspaceFile(ctx context.Context, filename string, reader io.Reader) (*ProductData, error) {
	return s.uploadAgentResource(ctx, "files", "", filename, reader)
}

// InspectSkillPackage validates a skill package before importing it.
func (s *AgentService) InspectSkillPackage(ctx context.Context, filename string, archive io.Reader) (*ProductData, error) {
	return s.uploadAgentResource(ctx, "skills", "/import/inspect", filename, archive)
}

type SkillPackageImportOption func(*skillPackageImportOptions)

type skillPackageImportOptions struct {
	name, description, category *string
	tags                        *[]string
}

func WithSkillPackageImportName(name string) SkillPackageImportOption {
	return func(options *skillPackageImportOptions) { options.name = &name }
}

func WithSkillPackageImportDescription(description string) SkillPackageImportOption {
	return func(options *skillPackageImportOptions) { options.description = &description }
}

func WithSkillPackageImportCategory(category string) SkillPackageImportOption {
	return func(options *skillPackageImportOptions) { options.category = &category }
}

func WithSkillPackageImportTags(tags []string) SkillPackageImportOption {
	return func(options *skillPackageImportOptions) { options.tags = &tags }
}

// ImportSkillPackage imports a skill package into the workspace.
func (s *AgentService) ImportSkillPackage(ctx context.Context, filename string, archive io.Reader, opts ...SkillPackageImportOption) (*ProductData, error) {
	settings := &skillPackageImportOptions{}
	for _, opt := range opts {
		opt(settings)
	}
	fields := map[string]string{}
	for name, value := range map[string]*string{"name": settings.name, "description": settings.description, "category": settings.category} {
		if value != nil {
			fields[name] = *value
		}
	}
	if settings.tags != nil {
		encoded, err := json.Marshal(*settings.tags)
		if err != nil {
			return nil, err
		}
		fields["tags"] = string(encoded)
	}
	return s.uploadAgentResourceWithFields(ctx, "skills", "/import", filename, archive, fields)
}

// GetSkillFile streams one imported static skill file. Callers own the response body.
func (s *AgentService) GetSkillFile(ctx context.Context, skillID, filePath string) (*StreamResponse, error) {
	if skillID == "" {
		return nil, ErrIDRequired
	}
	if filePath == "" {
		return nil, ErrPathRequired
	}
	skillPath, err := pathID(skillID)
	if err != nil {
		return nil, err
	}
	requestPath, err := agentResourcePath(s.workspaceID, "skills", "/"+skillPath+"/files/content")
	if err != nil {
		return nil, err
	}
	return s.doDownload(ctx, http.MethodGet, withQuery(requestPath, []QueryOption{WithQueryParam("path", filePath)}), nil)
}

// PolishSkillStream opens the skill polishing SSE stream for the supplied draft.
func (s *AgentService) PolishSkillStream(ctx context.Context, draft map[string]any) (*StreamResponse, error) {
	path, err := agentResourcePath(s.workspaceID, "skills", "/polish/stream")
	if err != nil {
		return nil, err
	}
	return s.doStream(ctx, http.MethodPost, path, draft)
}

// ResourceGet is a route-aligned Agent resource proxy escape hatch. Prefer AgentHandle A2A helpers for user-facing workflows.
func (s *AgentService) ResourceGet(ctx context.Context, resource string, tailPath string, opts ...QueryOption) (*ProductData, error) {
	return s.resource(ctx, http.MethodGet, resource, tailPath, nil, opts...)
}

// ResourcePost is a route-aligned Agent resource proxy escape hatch. Prefer AgentHandle A2A helpers for user-facing workflows.
func (s *AgentService) ResourcePost(ctx context.Context, resource string, tailPath string, body any, opts ...QueryOption) (*ProductData, error) {
	return s.resource(ctx, http.MethodPost, resource, tailPath, body, opts...)
}

// ResourcePut is a route-aligned Agent resource proxy escape hatch. Prefer AgentHandle A2A helpers for user-facing workflows.
func (s *AgentService) ResourcePut(ctx context.Context, resource string, tailPath string, body any, opts ...QueryOption) (*ProductData, error) {
	return s.resource(ctx, http.MethodPut, resource, tailPath, body, opts...)
}

// ResourcePatch is a route-aligned Agent resource proxy escape hatch. Prefer AgentHandle A2A helpers for user-facing workflows.
func (s *AgentService) ResourcePatch(ctx context.Context, resource string, tailPath string, body any, opts ...QueryOption) (*ProductData, error) {
	return s.resource(ctx, http.MethodPatch, resource, tailPath, body, opts...)
}

// ResourceDelete is a route-aligned Agent resource proxy escape hatch. Prefer AgentHandle A2A helpers for user-facing workflows.
func (s *AgentService) ResourceDelete(ctx context.Context, resource string, tailPath string, opts ...QueryOption) (*ProductData, error) {
	return s.resource(ctx, http.MethodDelete, resource, tailPath, nil, opts...)
}

// ResourceAny is a route-aligned Agent resource proxy escape hatch with an explicit HTTP method. Prefer AgentHandle A2A helpers for user-facing workflows.
func (s *AgentService) ResourceAny(ctx context.Context, method, resource string, tailPath string, body any, opts ...QueryOption) (*ProductData, error) {
	return s.resource(ctx, method, resource, tailPath, body, opts...)
}

func (s *AgentService) resource(ctx context.Context, method, resource, tailPath string, body any, opts ...QueryOption) (*ProductData, error) {
	resourcePath, err := pathID(resource)
	if err != nil {
		return nil, err
	}
	path, err := agentResourcePath(s.workspaceID, resourcePath, tailPath)
	if err != nil {
		return nil, err
	}
	var out ProductData
	err = s.do(ctx, method, withQuery(path, opts), body, &out)
	return &out, err
}

func (s *AgentService) uploadAgentResource(ctx context.Context, resource, tailPath, filename string, reader io.Reader) (*ProductData, error) {
	return s.uploadAgentResourceWithFields(ctx, resource, tailPath, filename, reader, nil)
}

func (s *AgentService) uploadAgentResourceWithFields(ctx context.Context, resource, tailPath, filename string, reader io.Reader, fields map[string]string) (*ProductData, error) {
	buf, contentType, err := multipartBody(fields, []MultipartFile{{FieldName: "file", Filename: filename, Reader: reader}})
	if err != nil {
		return nil, err
	}
	path, err := agentResourcePath(s.workspaceID, resource, tailPath)
	if err != nil {
		return nil, err
	}
	var out ProductData
	err = s.doRaw(ctx, http.MethodPost, path, buf, contentType, contentTypeJSON, &out)
	return &out, err
}

func agentResourcePath(workspaceID, resourcePath, tailPath string) (string, error) {
	if workspaceID == "" {
		return "", ErrWorkspaceRequired
	}
	workspacePath, err := pathID(workspaceID)
	if err != nil {
		return "", err
	}
	if tailPath != "" && !strings.HasPrefix(tailPath, "/") {
		return "", ErrPathInvalid
	}
	return "/workspaces/" + workspacePath + "/" + resourcePath + tailPath, nil
}

func validateAgentA2ARequest(req AgentA2ARequest, stream bool) error {
	if req.AgentCode == "" && req.AgentID == "" {
		return ErrAgentRequired
	}
	if req.Method == "" {
		return ErrMethodRequired
	}
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		return ErrJSONRPCInvalid
	}
	switch req.Method {
	case A2AMethodStream, A2AMethodTasksResubscribe:
		if !stream {
			return ErrMethodInvalid
		}
	case A2AMethodSend, A2AMethodTasksGet, A2AMethodTasksCancel, A2AMethodInputSubmit:
		if stream {
			return ErrMethodInvalid
		}
	default:
		return ErrMethodInvalid
	}
	return nil
}

// List returns the current user's product workspaces.
func (s *WorkspaceService) List(ctx context.Context, opts ...ListOption) (*WorkspaceListResult, error) {
	out := &product.WorkspaceListResult{}
	err := s.client.do(ctx, http.MethodGet, withQuery("/workspaces", opts), "", nil, out)
	return out, err
}

// Create creates a product workspace visible in MOI UI.
func (s *WorkspaceService) Create(ctx context.Context, name string, opts ...CreateWorkspaceOption) (*WorkspaceCreateResult, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	options := &createWorkspaceOptions{}
	for _, opt := range opts {
		opt(options)
	}
	out := &product.WorkspaceCreateResult{}
	err := s.client.do(ctx, http.MethodPost, "/workspaces", "", map[string]any{"name": name}, out)
	return out, err
}

// Claim claims a workspace for the current product user.
func (s *WorkspaceService) Claim(ctx context.Context, workspaceID string) (*WorkspaceClaimResult, error) {
	if workspaceID == "" {
		return nil, ErrWorkspaceRequired
	}
	out := &product.WorkspaceClaimResult{}
	err := s.client.do(ctx, http.MethodPost, "/workspaces/claim", "", map[string]any{"workspace_id": workspaceID}, out)
	return out, err
}

// Delete deletes a product workspace. WithQueryParam("wait", "true") waits
// for physical account deletion; canceling the wait does not undo deletion.
func (s *WorkspaceService) Delete(ctx context.Context, workspaceID string, opts ...QueryOption) error {
	id, err := pathID(workspaceID)
	if err != nil {
		return err
	}
	return s.client.do(ctx, http.MethodDelete, withQuery("/workspaces/"+id, opts), "", nil, nil)
}

// Health checks product catalog routing through moi-backend.
func (s *CatalogService) Health(ctx context.Context) (*HealthResult, error) {
	var out HealthResult
	err := s.do(ctx, http.MethodGet, "/catalog/healthz", nil, &out)
	return &out, err
}

// Tree returns the UI catalog tree.
func (s *CatalogService) Tree(ctx context.Context, opts ...CatalogTreeOption) (*CatalogTreeResult, error) {
	options := catalogTreeOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	var out CatalogTreeResult
	err := s.do(ctx, http.MethodPost, "/catalog/tree", options, &out)
	return &out, err
}

// MemoryQuery is the request for MemoryService.Retrieve / Search, mirroring
// moi-backend's pkg/memoria/retrieve.Request.
type MemoryQuery struct {
	Query        string   `json:"query"`
	SubjectID    string   `json:"subject_id"`
	TopK         int64    `json:"top_k,omitempty"`
	SessionID    string   `json:"session_id,omitempty"`
	SessionScope string   `json:"session_scope,omitempty"`
	MemoryTypes  []string `json:"memory_types,omitempty"`
	Branch       string   `json:"branch,omitempty"`
}

// MemoryFulltextQuery performs pure keyword search with optional exact
// top-level scalar metadata filters.
type MemoryFulltextQuery struct {
	Query               string         `json:"query"`
	SubjectID           string         `json:"subject_id"`
	TopK                int64          `json:"top_k,omitempty"`
	SessionID           string         `json:"session_id,omitempty"`
	MemoryTypes         []string       `json:"memory_types,omitempty"`
	TrustTier           string         `json:"trust_tier,omitempty"`
	Branch              string         `json:"branch,omitempty"`
	ExtraMetadataFilter map[string]any `json:"extra_metadata_filter,omitempty"`
}

// MemoryStructuredQuery performs exact structured filtering without
// relevance recall. All metadata filters are combined with AND.
type MemoryStructuredQuery struct {
	SubjectID           string         `json:"subject_id"`
	Limit               int64          `json:"limit,omitempty"`
	Cursor              string         `json:"cursor,omitempty"`
	SessionID           string         `json:"session_id,omitempty"`
	MemoryTypes         []string       `json:"memory_types,omitempty"`
	TrustTier           string         `json:"trust_tier,omitempty"`
	Branch              string         `json:"branch,omitempty"`
	ExtraMetadataFilter map[string]any `json:"extra_metadata_filter,omitempty"`
}

var memoryMetadataKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

func validateMemoryMetadataFilter(filter map[string]any) error {
	if len(filter) > 16 {
		return &Error{Code: "ErrInvalidArgument", Message: "extra_metadata_filter supports at most 16 fields"}
	}
	for key, value := range filter {
		if !memoryMetadataKeyPattern.MatchString(key) {
			return &Error{Code: "ErrInvalidArgument", Message: "extra_metadata_filter contains an invalid field name"}
		}
		switch number := value.(type) {
		case string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		case float32:
			if math.IsNaN(float64(number)) || math.IsInf(float64(number), 0) {
				return &Error{Code: "ErrInvalidArgument", Message: "extra_metadata_filter values must be finite scalars"}
			}
		case float64:
			if math.IsNaN(number) || math.IsInf(number, 0) {
				return &Error{Code: "ErrInvalidArgument", Message: "extra_metadata_filter values must be finite scalars"}
			}
		default:
			return &Error{Code: "ErrInvalidArgument", Message: "extra_metadata_filter values must be string, number, or boolean"}
		}
		encoded, err := json.Marshal(value)
		if err != nil || len(encoded) > 1024 {
			return &Error{Code: "ErrInvalidArgument", Message: "extra_metadata_filter value exceeds 1024 encoded bytes"}
		}
	}
	return nil
}

func (q MemoryFulltextQuery) validate() error {
	if strings.TrimSpace(q.Query) == "" {
		return ErrQueryRequired
	}
	if q.TopK < 0 || q.TopK > 100 {
		return &Error{Code: "ErrInvalidArgument", Message: "top_k must be between 0 and 100"}
	}
	if err := validateMemoryMetadataFilter(q.ExtraMetadataFilter); err != nil {
		return err
	}
	if strings.TrimSpace(q.SubjectID) == "" {
		return ErrSubjectRequired
	}
	return nil
}

func (q MemoryStructuredQuery) validate() error {
	if q.Limit < 0 || q.Limit > 500 {
		return &Error{Code: "ErrInvalidArgument", Message: "limit must be between 0 and 500"}
	}
	if err := validateMemoryMetadataFilter(q.ExtraMetadataFilter); err != nil {
		return err
	}
	if strings.TrimSpace(q.SubjectID) == "" {
		return ErrSubjectRequired
	}
	return nil
}

func (q MemoryQuery) validate() error {
	if strings.TrimSpace(q.Query) == "" {
		return ErrQueryRequired
	}
	if strings.TrimSpace(q.SubjectID) == "" {
		return ErrSubjectRequired
	}
	if q.TopK < 0 || q.TopK > 100 {
		return &Error{Code: "ErrInvalidArgument", Message: "top_k must be between 0 and 100"}
	}
	return nil
}

// Retrieve retrieves consumer memories for one subject via POST /newmoi/memories/retrieve.
func (s *MemoryService) Retrieve(ctx context.Context, q MemoryQuery) (*MemoryListResult, error) {
	if err := q.validate(); err != nil {
		return nil, err
	}
	var out MemoryListResult
	err := s.do(ctx, http.MethodPost, "/memories/retrieve", q, &out)
	return &out, err
}

// Search searches consumer memories for one subject via POST /newmoi/memories/search.
func (s *MemoryService) Search(ctx context.Context, q MemoryQuery) (*MemoryListResult, error) {
	if err := q.validate(); err != nil {
		return nil, err
	}
	var out MemoryListResult
	err := s.do(ctx, http.MethodPost, "/memories/search", q, &out)
	return &out, err
}

// FulltextSearch searches exact keywords and optionally applies structured
// metadata pre-filters via POST /newmoi/memories/fulltext-search.
func (s *MemoryService) FulltextSearch(ctx context.Context, q MemoryFulltextQuery) (*MemoryListResult, error) {
	if err := q.validate(); err != nil {
		return nil, err
	}
	var out MemoryListResult
	err := s.do(ctx, http.MethodPost, "/memories/fulltext-search", q, &out)
	return &out, err
}

// Query runs an exact structured query via POST /newmoi/memories/query.
func (s *MemoryService) Query(ctx context.Context, q MemoryStructuredQuery) (*MemoryListResult, error) {
	if err := q.validate(); err != nil {
		return nil, err
	}
	var out MemoryListResult
	err := s.do(ctx, http.MethodPost, "/memories/query", q, &out)
	return &out, err
}

// RenderProfile renders one subject's consumer profile via
// GET /newmoi/consumer-profiles/{user_id}/render. maxTokens == 0 uses the
// server-side default render budget; negative values are invalid.
func (s *MemoryService) RenderProfile(ctx context.Context, subjectID string, maxTokens int) (*ConsumerProfileRenderResult, error) {
	if strings.TrimSpace(subjectID) == "" {
		return nil, ErrSubjectRequired
	}
	if maxTokens < 0 {
		return nil, &Error{Code: "ErrInvalidArgument", Message: "max_tokens must not be negative"}
	}
	userPath := url.PathEscape(subjectID)
	var opts []QueryOption
	if maxTokens > 0 {
		opts = append(opts, WithQueryParam("max_tokens", strconv.Itoa(maxTokens)))
	}
	var out ConsumerProfileRenderResult
	err := s.do(ctx, http.MethodGet, withQuery("/consumer-profiles/"+userPath+"/render", opts), nil, &out)
	return &out, err
}

// SystemTree returns the MatrixOne system-database navigation tree (name-key nodes).
func (s *CatalogService) SystemTree(ctx context.Context, opts ...SystemCatalogTreeOption) (*CatalogTreeResult, error) {
	options := systemCatalogTreeOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if options.includeTableLeaves != nil {
		body["include_table_leaves"] = *options.includeTableLeaves
	}
	if options.databaseName != nil {
		body["database_name"] = *options.databaseName
	}
	if options.tableName != nil {
		body["table_name"] = *options.tableName
	}
	if options.pageSize != nil {
		body["page_size"] = *options.pageSize
	}
	if options.pageToken != nil {
		body["page_token"] = *options.pageToken
	}
	if options.search != nil {
		body["search"] = *options.search
	}
	var out CatalogTreeResult
	err := s.do(ctx, http.MethodPost, "/catalog/system/tree", body, &out)
	return &out, err
}

// List returns UI-visible catalogs.
func (s *CatalogService) List(ctx context.Context, opts ...CatalogListOption) (*CatalogListResult, error) {
	body := map[string]any{}
	addCatalogListOptions(body, opts)
	var out CatalogListResult
	err := s.do(ctx, http.MethodPost, "/catalog/list", body, &out)
	return &out, err
}

// AllowedActions returns the effective actions for the requested Catalog resources.
func (s *CatalogService) AllowedActions(ctx context.Context, resourceType string, resourceIDs []string) (*CatalogAllowedActionsResult, error) {
	body := map[string]any{"resource_type": resourceType, "resource_ids": resourceIDs}
	var out CatalogAllowedActionsResult
	err := s.do(ctx, http.MethodPost, "/catalog/allowed-actions", body, &out)
	return &out, err
}

// Create creates a UI-visible catalog.
func (s *CatalogService) Create(ctx context.Context, name string, opts ...CatalogInfoOption) (*IDResult, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	body := map[string]any{"name": name}
	addCatalogInfoOptions(body, opts)
	var out IDResult
	err := s.do(ctx, http.MethodPost, "/catalog/create", body, &out)
	return &out, err
}

// Info returns one catalog.
func (s *CatalogService) Info(ctx context.Context, catalogID int) (*CatalogInfoResult, error) {
	if catalogID == 0 {
		return nil, ErrIDRequired
	}
	var out CatalogInfoResult
	err := s.do(ctx, http.MethodPost, "/catalog/info", map[string]any{"id": catalogID}, &out)
	return &out, err
}

// Update updates one catalog.
func (s *CatalogService) Update(ctx context.Context, catalogID int, name string, opts ...CatalogInfoOption) error {
	if catalogID == 0 {
		return ErrIDRequired
	}
	if name == "" {
		return ErrNameRequired
	}
	body := map[string]any{"id": catalogID, "name": name}
	addCatalogInfoOptions(body, opts)
	return s.do(ctx, http.MethodPost, "/catalog/update", body, nil)
}

// Delete deletes one catalog.
func (s *CatalogService) Delete(ctx context.Context, catalogID int) error {
	if catalogID == 0 {
		return ErrIDRequired
	}
	return s.do(ctx, http.MethodPost, "/catalog/delete", map[string]any{"id": catalogID}, nil)
}

// ListDatabases returns databases under one catalog.
func (s *CatalogService) ListDatabases(ctx context.Context, catalogID int, opts ...CatalogListOption) (*DatabaseListResult, error) {
	if catalogID == 0 {
		return nil, ErrIDRequired
	}
	body := map[string]any{"id": catalogID}
	addCatalogListOptions(body, opts)
	var out DatabaseListResult
	err := s.do(ctx, http.MethodPost, "/catalog/database/list", body, &out)
	return &out, err
}

// CreateDatabase creates a UI-visible database.
func (s *CatalogService) CreateDatabase(ctx context.Context, catalogID int, name string, opts ...CatalogInfoOption) (*IDResult, error) {
	if catalogID == 0 {
		return nil, ErrIDRequired
	}
	if name == "" {
		return nil, ErrNameRequired
	}
	body := map[string]any{"catalog_id": catalogID, "name": name}
	addCatalogInfoOptions(body, opts)
	var out IDResult
	err := s.do(ctx, http.MethodPost, "/catalog/database/create", body, &out)
	return &out, err
}

// DatabaseInfo returns one database.
func (s *CatalogService) DatabaseInfo(ctx context.Context, databaseID int) (*DatabaseInfoResult, error) {
	if databaseID == 0 {
		return nil, ErrIDRequired
	}
	var out DatabaseInfoResult
	err := s.do(ctx, http.MethodPost, "/catalog/database/info", map[string]any{"id": databaseID}, &out)
	return &out, err
}

// UpdateDatabase is retained for source compatibility. Database rename is not
// supported by the product API; use UpdateDatabaseDescription instead.
// Deprecated: database rename is not supported.
func (s *CatalogService) UpdateDatabase(ctx context.Context, databaseID int, name string, opts ...CatalogInfoOption) (*IDResult, error) {
	if databaseID == 0 {
		return nil, ErrIDRequired
	}
	return nil, ErrDatabaseRenameUnsupported
}

// UpdateDatabaseDescription updates one database description.
func (s *CatalogService) UpdateDatabaseDescription(ctx context.Context, databaseID int, opts ...CatalogInfoOption) (*IDResult, error) {
	if databaseID == 0 {
		return nil, ErrIDRequired
	}
	body := map[string]any{"id": databaseID}
	addCatalogInfoOptions(body, opts)
	var out IDResult
	err := s.do(ctx, http.MethodPost, "/catalog/database/update", body, &out)
	return &out, err
}

// DeleteDatabase deletes one database.
func (s *CatalogService) DeleteDatabase(ctx context.Context, databaseID int) error {
	if databaseID == 0 {
		return ErrIDRequired
	}
	return s.do(ctx, http.MethodPost, "/catalog/database/delete", map[string]any{"id": databaseID}, nil)
}

// DatabaseChildren returns the UI children list for one database.
func (s *CatalogService) DatabaseChildren(ctx context.Context, databaseID int, opts ...CatalogListOption) (*DatabaseChildrenResult, error) {
	if databaseID == 0 {
		return nil, ErrIDRequired
	}
	body := map[string]any{"id": databaseID}
	addCatalogListOptions(body, opts)
	var out DatabaseChildrenResult
	err := s.do(ctx, http.MethodPost, "/catalog/database/children", body, &out)
	return &out, err
}

// DatabaseTableLeaves returns one page of tables directly owned by a database.
func (s *CatalogService) DatabaseTableLeaves(ctx context.Context, databaseID int, opts ...DatabaseTableLeavesOption) (*DatabaseTableLeavesResult, error) {
	if databaseID == 0 {
		return nil, ErrIDRequired
	}
	options := databaseTableLeavesOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"database_id": databaseID}
	if options.pageSize != nil {
		body["page_size"] = *options.pageSize
	}
	if options.pageToken != nil {
		body["page_token"] = *options.pageToken
	}
	if options.search != nil {
		body["search"] = *options.search
	}
	var out DatabaseTableLeavesResult
	err := s.do(ctx, http.MethodPost, "/catalog/database/table-leaves", body, &out)
	return &out, err
}

// SearchDataDashboardTables searches one permission-scoped page of internal tables across databases.
func (s *CatalogService) SearchDataDashboardTables(ctx context.Context, opts ...DataDashboardTableSearchOption) (*DataDashboardTableSearchResult, error) {
	options := dataDashboardTableSearchOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if options.search != nil {
		body["search"] = *options.search
	}
	if options.pageSize != nil {
		body["page_size"] = *options.pageSize
	}
	if options.pageToken != nil {
		body["page_token"] = *options.pageToken
	}
	var out DataDashboardTableSearchResult
	err := s.do(ctx, http.MethodPost, "/catalog/data-dashboard/table-search", body, &out)
	return &out, err
}

// CreateVolume creates a UI-visible volume.
func (s *CatalogService) CreateVolume(ctx context.Context, databaseID int, name string, opts ...CatalogInfoOption) (*IDResult, error) {
	if databaseID == 0 {
		return nil, ErrIDRequired
	}
	if name == "" {
		return nil, ErrNameRequired
	}
	body := map[string]any{"database_id": databaseID, "name": name}
	addCatalogInfoOptions(body, opts)
	var out IDResult
	err := s.do(ctx, http.MethodPost, "/catalog/volume/create", body, &out)
	return &out, err
}

// VolumeInfo returns one volume.
func (s *CatalogService) VolumeInfo(ctx context.Context, volumeID string) (*VolumeInfoResult, error) {
	if volumeID == "" {
		return nil, ErrIDRequired
	}
	var out VolumeInfoResult
	err := s.do(ctx, http.MethodPost, "/catalog/volume/info", map[string]any{"id": volumeID}, &out)
	return &out, err
}

// UpdateVolume updates one volume.
func (s *CatalogService) UpdateVolume(ctx context.Context, volumeID int, name string, opts ...CatalogInfoOption) (*IDResult, error) {
	if volumeID == 0 {
		return nil, ErrIDRequired
	}
	if name == "" {
		return nil, ErrNameRequired
	}
	body := map[string]any{"id": volumeID, "name": name}
	addCatalogInfoOptions(body, opts)
	var out IDResult
	err := s.do(ctx, http.MethodPost, "/catalog/volume/update", body, &out)
	return &out, err
}

// DeleteVolume deletes one volume.
func (s *CatalogService) DeleteVolume(ctx context.Context, volumeID int) error {
	if volumeID == 0 {
		return ErrIDRequired
	}
	return s.do(ctx, http.MethodPost, "/catalog/volume/delete", map[string]any{"id": volumeID}, nil)
}

// VolumeFullPath returns the UI full path for one volume.
func (s *CatalogService) VolumeFullPath(ctx context.Context, volumeID int) (*PathResult, error) {
	if volumeID == 0 {
		return nil, ErrIDRequired
	}
	var out PathResult
	err := s.do(ctx, http.MethodPost, "/catalog/volume/full_path", map[string]any{"id": volumeID}, &out)
	return &out, err
}

// CreateTable creates a UI-visible table.
func (s *CatalogService) CreateTable(ctx context.Context, databaseID int, name string, columns []TableColumn, opts ...CreateTableOption) (*CatalogCreateTableResult, error) {
	if databaseID == 0 {
		return nil, ErrIDRequired
	}
	if name == "" {
		return nil, ErrNameRequired
	}
	if len(columns) == 0 {
		return nil, ErrColumnsRequired
	}
	options := createTableOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{
		"database_id": databaseID,
		"name":        name,
		"columns":     columns,
	}
	if options.comment != nil {
		body["comment"] = *options.comment
	}
	if options.dataFileURL != nil {
		body["data_file_url"] = *options.dataFileURL
	}
	if len(options.columnLoadOptions) > 0 {
		body["column_load_options"] = options.columnLoadOptions
	}
	var out CatalogCreateTableResult
	err := s.do(ctx, http.MethodPost, "/catalog/table/create", body, &out)
	return &out, err
}

// TableInfo returns table metadata.
func (s *CatalogService) TableInfo(ctx context.Context, tableID int, opts ...TableLookupOption) (*CatalogTableInfoResult, error) {
	body := tableLookupBody(tableID, opts...)
	var out CatalogTableInfoResult
	err := s.do(ctx, http.MethodPost, "/catalog/table/info", body, &out)
	return &out, err
}

// MultiTableInfo returns metadata for multiple tables.
func (s *CatalogService) MultiTableInfo(ctx context.Context, tableIDs ...int) (*CatalogMultiTableInfoResult, error) {
	var out CatalogMultiTableInfoResult
	err := s.do(ctx, http.MethodPost, "/catalog/table/multi_info", map[string]any{"ids": tableIDs}, &out)
	return &out, err
}

// DeleteTable deletes one table.
func (s *CatalogService) DeleteTable(ctx context.Context, tableID int) error {
	if tableID == 0 {
		return ErrIDRequired
	}
	return s.do(ctx, http.MethodPost, "/catalog/table/delete", map[string]any{"id": tableID}, nil)
}

// TableData returns table data for UI preview grids.
func (s *CatalogService) TableData(ctx context.Context, tableID int, opts ...TableDataOption) (*CatalogTableDataResult, error) {
	body := tableDataBody(tableID, opts...)
	var out CatalogTableDataResult
	err := s.do(ctx, http.MethodPost, "/catalog/table/data", body, &out)
	return &out, err
}

// PreviewTable returns table preview data.
func (s *CatalogService) PreviewTable(ctx context.Context, tableID int, opts ...TableDataOption) (*CatalogTableDataResult, error) {
	body := tableDataBody(tableID, opts...)
	var out CatalogTableDataResult
	err := s.do(ctx, http.MethodPost, "/catalog/table/preview", body, &out)
	return &out, err
}

// DownloadTable streams a CSV table download. Callers own the response body.
func (s *CatalogService) DownloadTable(ctx context.Context, tableID int) (*StreamResponse, error) {
	if tableID == 0 {
		return nil, ErrIDRequired
	}
	return s.doDownload(ctx, http.MethodPost, "/catalog/table/download", map[string]any{"id": tableID})
}

// TableExists checks whether a table exists under a database.
func (s *CatalogService) TableExists(ctx context.Context, databaseID int, name string) (*BoolDataResult, error) {
	if databaseID == 0 {
		return nil, ErrIDRequired
	}
	if name == "" {
		return nil, ErrNameRequired
	}
	var out BoolDataResult
	err := s.do(ctx, http.MethodPost, "/catalog/table/exist", map[string]any{"database_id": databaseID, "name": name}, &out)
	return &out, err
}

// TableFullPath returns the UI full path for one table.
func (s *CatalogService) TableFullPath(ctx context.Context, tableID int) (*PathResult, error) {
	if tableID == 0 {
		return nil, ErrIDRequired
	}
	var out PathResult
	err := s.do(ctx, http.MethodPost, "/catalog/table/full_path", map[string]any{"id": tableID}, &out)
	return &out, err
}

// TruncateTable clears table data.
func (s *CatalogService) TruncateTable(ctx context.Context, tableID int) error {
	if tableID == 0 {
		return ErrIDRequired
	}
	return s.do(ctx, http.MethodPost, "/catalog/table/truncate", map[string]any{"id": tableID}, nil)
}

// TableOverview returns the catalog table overview.
func (s *CatalogService) TableOverview(ctx context.Context, databaseID int) (*CatalogTableOverviewResult, error) {
	if databaseID == 0 {
		return nil, ErrIDRequired
	}
	var out CatalogTableOverviewResult
	err := s.do(ctx, http.MethodPost, "/catalog/table/overview", map[string]any{"id": databaseID}, &out)
	return &out, err
}

func tableLookupBody(tableID int, opts ...TableLookupOption) map[string]any {
	options := tableLookupOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if tableID != 0 {
		body["id"] = tableID
	}
	if options.name != nil {
		body["name"] = *options.name
	}
	if options.databaseID != nil {
		body["database_id"] = *options.databaseID
	}
	if options.databaseName != nil {
		body["database_name"] = *options.databaseName
	}
	if options.includeStats != nil {
		body["include_stats"] = *options.includeStats
	}
	return body
}

func tableDataBody(tableID int, opts ...TableDataOption) map[string]any {
	options := tableDataOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if tableID != 0 {
		body["id"] = tableID
	}
	if options.page != nil {
		body["page"] = *options.page
	}
	if options.pageSize != nil {
		body["page_size"] = *options.pageSize
	}
	if options.name != nil {
		body["name"] = *options.name
	}
	if options.databaseID != nil {
		body["database_id"] = *options.databaseID
	}
	if options.databaseName != nil {
		body["database_name"] = *options.databaseName
	}
	if options.previewLines != nil {
		body["preview_lines"] = *options.previewLines
	}
	if options.includeTotal != nil {
		body["include_total"] = *options.includeTotal
	}
	return body
}

func fileListBody(opts ...FileListOption) map[string]any {
	options := fileListOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if options.page != nil {
		body["page"] = *options.page
	}
	if options.pageSize != nil {
		body["page_size"] = *options.pageSize
	}
	if options.order != nil {
		body["order"] = *options.order
	}
	if options.orderBy != nil {
		body["order_by"] = *options.orderBy
	}
	if len(options.filters) > 0 {
		body["filters"] = options.filters
	}
	return body
}

// Files lists files and folders in the catalog.
func (s *CatalogService) Files(ctx context.Context, opts ...FileListOption) (*CatalogFileListResult, error) {
	var out CatalogFileListResult
	err := s.do(ctx, http.MethodPost, "/catalog/file/list", fileListBody(opts...), &out)
	return &out, err
}

// ListWorkspaceFiles discovers IAM-visible file placements across all Volumes.
// Runtime callers retain their task proof and frozen Role through this route.
func (s *CatalogService) ListWorkspaceFiles(ctx context.Context, opts ...QueryOption) (*WorkspaceFileListResult, error) {
	workspaceID, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	var out WorkspaceFileListResult
	err = s.do(ctx, http.MethodGet, withQuery("/workspaces/"+workspaceID+"/files", opts), nil, &out)
	return &out, err
}

// CreateFile creates file references visible in the catalog.
func (s *CatalogService) CreateFile(ctx context.Context, volumeID int, fileIDs ...string) error {
	if volumeID == 0 {
		return ErrVolumeIDRequired
	}
	if len(fileIDs) == 0 {
		return ErrFileIDsRequired
	}
	return s.do(ctx, http.MethodPost, "/catalog/file/create", map[string]any{"volume_id": volumeID, "file_ids": fileIDs}, nil)
}

// FileInfo returns one file.
func (s *CatalogService) FileInfo(ctx context.Context, fileID string) (*FileInfoResult, error) {
	if fileID == "" {
		return nil, ErrFileIDRequired
	}
	var out FileInfoResult
	err := s.do(ctx, http.MethodPost, "/catalog/file/info", map[string]any{"file_id": fileID}, &out)
	return &out, err
}

// DeleteFile deletes one file.
func (s *CatalogService) DeleteFile(ctx context.Context, fileID string, volumeID string) error {
	if fileID == "" {
		return ErrFileIDRequired
	}
	return s.do(ctx, http.MethodPost, "/catalog/file/delete", map[string]any{"id": fileID, "volume_id": volumeID}, nil)
}

// DeleteFileRef deletes file references from a volume.
func (s *CatalogService) DeleteFileRef(ctx context.Context, volumeID int, fileIDs ...string) error {
	if volumeID == 0 {
		return ErrVolumeIDRequired
	}
	if len(fileIDs) == 0 {
		return ErrFileIDsRequired
	}
	return s.do(ctx, http.MethodPost, "/catalog/file/delete_ref", map[string]any{"volume_id": volumeID, "file_ids": fileIDs}, nil)
}

// TriggerFileWorkflow triggers workflows bound to catalog files.
func (s *CatalogService) TriggerFileWorkflow(ctx context.Context, volumeID int, fileIDs ...string) (*TriggerFileWorkflowResult, error) {
	if volumeID == 0 {
		return nil, ErrVolumeIDRequired
	}
	if len(fileIDs) == 0 {
		return nil, ErrFileIDsRequired
	}
	var out TriggerFileWorkflowResult
	err := s.do(ctx, http.MethodPost, "/catalog/file/trigger_workflow", map[string]any{"volume_id": volumeID, "file_ids": fileIDs}, &out)
	return &out, err
}

// DownloadFile streams a catalog file. Callers own the response body.
func (s *CatalogService) DownloadFile(ctx context.Context, volumeID, fileID string, opts ...DownloadFileOption) (*StreamResponse, error) {
	if volumeID == "" {
		return nil, ErrVolumeIDRequired
	}
	if fileID == "" {
		return nil, ErrFileIDRequired
	}
	options := downloadFileOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"volume_id": volumeID, "file_id": fileID}
	if options.asZip != nil {
		body["as_zip"] = *options.asZip
	}
	return s.doDownload(ctx, http.MethodPost, "/catalog/file/download", body)
}

// PreviewFile streams file preview content. Callers own the response body.
func (s *CatalogService) PreviewFile(ctx context.Context, volumeID, fileID string) (*StreamResponse, error) {
	if volumeID == "" {
		return nil, ErrVolumeIDRequired
	}
	if fileID == "" {
		return nil, ErrFileIDRequired
	}
	return s.doDownload(ctx, http.MethodPost, "/catalog/file/preview_stream", map[string]any{"volume_id": volumeID, "file_id": fileID})
}

// UploadBytes uploads caller-provided bytes as a catalog file.
func (s *CatalogService) UploadBytes(ctx context.Context, filename string, data []byte) (*UploadFileResult, error) {
	return s.Upload(ctx, filename, bytes.NewReader(data))
}

// UploadFile uploads a caller-supplied local file path to the product catalog.
// It is a local file input convenience method, not AI file-discovery, local-filesystem probing, filename inference, or default upload entry point.
func (s *CatalogService) UploadFile(ctx context.Context, filePath string) (*UploadFileResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open upload file: %w", err)
	}
	defer file.Close()
	return s.Upload(ctx, filepath.Base(filePath), file)
}

// Upload uploads a catalog file using the backend multipart field named "file".
func (s *CatalogService) Upload(ctx context.Context, filename string, reader io.Reader, options ...CatalogUploadOption) (*UploadFileResult, error) {
	settings := catalogUploadOptions{}
	for _, option := range options {
		option(&settings)
	}
	buf, contentType, err := catalogUploadMultipart(filename, reader, settings)
	if err != nil {
		return nil, err
	}
	var out UploadFileResult
	err = s.doRaw(ctx, http.MethodPost, "/catalog/file/upload", buf, contentType, contentTypeJSON, &out)
	return &out, err
}

// UploadPrivateBytes uploads caller-provided bytes to the current user's private Catalog volume.
func (s *CatalogService) UploadPrivateBytes(ctx context.Context, filename string, data []byte) (*UploadFileResult, error) {
	return s.UploadPrivate(ctx, filename, bytes.NewReader(data))
}

// UploadPrivateFile uploads a caller-supplied local file path to the current user's private Catalog volume.
// It is a local file input convenience method, not AI file-discovery, local-filesystem probing, filename inference, or default upload entry point.
func (s *CatalogService) UploadPrivateFile(ctx context.Context, filePath string) (*UploadFileResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open private Catalog upload file: %w", err)
	}
	defer file.Close()
	return s.UploadPrivate(ctx, filepath.Base(filePath), file)
}

// UploadPrivate uploads a catalog file to the current user's private Catalog volume.
func (s *CatalogService) UploadPrivate(ctx context.Context, filename string, reader io.Reader, options ...CatalogUploadOption) (*UploadFileResult, error) {
	if s.workspaceID == "" {
		return nil, ErrWorkspaceRequired
	}
	workspaceID, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	settings := catalogUploadOptions{}
	for _, option := range options {
		option(&settings)
	}
	buf, contentType, err := catalogUploadMultipart(filename, reader, settings)
	if err != nil {
		return nil, err
	}
	var out UploadFileResult
	err = s.doRaw(ctx, http.MethodPost, "/workspaces/"+workspaceID+"/catalog-files/uploads", buf, contentType, contentTypeJSON, &out)
	return &out, err
}

// UploadRuntimeArtifact publishes one sandbox-child file through MOI's
// task-scoped runtime endpoint. The client must also have been configured with
// WithRuntimeAuthorization from the child environment.
//
// This endpoint is native-only child runtime transport, so it deliberately is
// not added to the moi-backend route manifest.
func (s *CatalogService) UploadRuntimeArtifact(ctx context.Context, filename string, reader io.Reader) (*RuntimeArtifactUploadResult, error) {
	workspaceID, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	buf, contentType, err := catalogUploadMultipart(filename, reader)
	if err != nil {
		return nil, err
	}
	var out RuntimeArtifactUploadResult
	err = s.doRaw(ctx, http.MethodPost, "/workspaces/"+workspaceID+"/runtime-artifacts", buf, contentType, contentTypeJSON, &out)
	return &out, err
}

// AnalyzeRuntimeImage sends one prepared image and question through MOI's
// task-scoped native image-analysis endpoint. The client must include the
// child runtime proof with WithRuntimeAuthorization; this native-only transport
// is intentionally absent from the moi-backend route manifest.
func (s *CatalogService) AnalyzeRuntimeImage(
	ctx context.Context,
	filename string,
	reader io.Reader,
	question string,
) (*RuntimeImageAnalysisResult, error) {
	workspaceID, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	body, contentType, err := runtimeImageAnalysisMultipart(filename, reader, question)
	if err != nil {
		return nil, err
	}
	var out RuntimeImageAnalysisResult
	err = s.doRaw(ctx, http.MethodPost, "/workspaces/"+workspaceID+"/runtime-image-analysis", body, contentType, contentTypeJSON, &out)
	return &out, err
}

// DownloadWorkspaceFile streams a task-visible workspace file. Callers own
// the response body. A sandbox child includes WithRuntimeAuthorization; an
// ordinary product caller follows the route's normal Product identity path.
func (s *CatalogService) DownloadWorkspaceFile(ctx context.Context, fileID string) (*StreamResponse, error) {
	workspaceID, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	filePathID, err := pathID(fileID)
	if err != nil {
		return nil, err
	}
	return s.doDownload(ctx, http.MethodGet, "/workspaces/"+workspaceID+"/files/"+filePathID+"/download", nil)
}

func catalogUploadMultipart(filename string, reader io.Reader, options ...catalogUploadOptions) (*bytes.Buffer, string, error) {
	var settings catalogUploadOptions
	if len(options) > 0 {
		settings = options[0]
	}
	fileContentType := settings.contentType
	if filename == "" {
		return nil, "", &Error{Code: "ErrFilenameRequired", Message: "filename is required"}
	}
	if reader == nil {
		return nil, "", &Error{Code: "ErrFileReaderRequired", Message: "file reader is required"}
	}
	fields := map[string]string{}
	if uri := strings.TrimSpace(settings.externalSourceURI); uri != "" {
		fields["external_source_uri"] = uri
	}
	buf, contentType, err := multipartBody(fields, []MultipartFile{{FieldName: "file", Filename: filename, Reader: reader, ContentType: fileContentType}})
	if err != nil {
		return nil, "", err
	}
	return buf, contentType, nil
}

func runtimeImageAnalysisMultipart(filename string, reader io.Reader, question string) (*bytes.Buffer, string, error) {
	return multipartBody(map[string]string{"question": question}, []MultipartFile{{
		FieldName: "file", Filename: filename, Reader: reader,
	}})
}

// DataAsset returns one workflow data asset.
func (s *CatalogService) DataAsset(ctx context.Context, assetID string) (*CatalogDataAssetResolveResult, error) {
	id, err := pathID(assetID)
	if err != nil {
		return nil, err
	}
	var out CatalogDataAssetResolveResult
	err = s.do(ctx, http.MethodGet, "/catalog/data-assets/"+id, nil, &out)
	return &out, err
}

// BatchGetDataAssets returns data assets for raw file ids.
func (s *CatalogService) BatchGetDataAssets(ctx context.Context, rawFileIDs ...string) (*CatalogBatchGetDataAssetsResult, error) {
	if len(rawFileIDs) == 0 {
		return nil, ErrFileIDsRequired
	}
	var out CatalogBatchGetDataAssetsResult
	err := s.do(ctx, http.MethodPost, "/catalog/data-assets/batch-get", map[string]any{"raw_file_ids": rawFileIDs}, &out)
	return &out, err
}

// FileDataAsset returns the data asset derived from a raw file.
func (s *CatalogService) FileDataAsset(ctx context.Context, fileID string) (*CatalogDataAssetResolveResult, error) {
	id, err := pathID(fileID)
	if err != nil {
		return nil, err
	}
	var out CatalogDataAssetResolveResult
	err = s.do(ctx, http.MethodGet, "/catalog/files/"+id+"/data-asset", nil, &out)
	return &out, err
}

// FileArtifact returns parsed artifact metadata for a file.
func (s *CatalogService) FileArtifact(ctx context.Context, fileID string) (*CatalogFileArtifactResult, error) {
	id, err := pathID(fileID)
	if err != nil {
		return nil, err
	}
	var out CatalogFileArtifactResult
	err = s.do(ctx, http.MethodGet, "/catalog/file-artifacts/"+id, nil, &out)
	return &out, err
}

// BatchResolveFileArtifacts resolves parsed artifact metadata for files.
func (s *CatalogService) BatchResolveFileArtifacts(ctx context.Context, fileIDs ...string) (*CatalogBatchResolveFileArtifactsResult, error) {
	if len(fileIDs) == 0 {
		return nil, ErrFileIDsRequired
	}
	var out CatalogBatchResolveFileArtifactsResult
	err := s.do(ctx, http.MethodPost, "/catalog/file-artifacts:batchResolve", map[string]any{"file_ids": fileIDs}, &out)
	return &out, err
}

// CreateFolder creates a catalog folder.
func (s *CatalogService) CreateFolder(ctx context.Context, parentVolumeID, databaseID int, name string, opts ...CreateFolderOption) (*CreateFolderResult, error) {
	if parentVolumeID == 0 {
		return nil, ErrVolumeIDRequired
	}
	if name == "" {
		return nil, ErrNameRequired
	}
	options := createFolderOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"parent_volume_id": parentVolumeID, "database_id": databaseID, "name": name}
	if options.description != nil {
		body["description"] = *options.description
	}
	var out CreateFolderResult
	err := s.do(ctx, http.MethodPost, "/catalog/folder/create", body, &out)
	return &out, err
}

// UpdateFolder updates a catalog folder.
func (s *CatalogService) UpdateFolder(ctx context.Context, folderID int, opts ...UpdateFolderOption) error {
	if folderID == 0 {
		return ErrIDRequired
	}
	options := updateFolderOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"id": folderID}
	if options.name != nil {
		body["name"] = *options.name
	}
	if options.description != nil {
		body["description"] = *options.description
	}
	return s.do(ctx, http.MethodPost, "/catalog/folder/update", body, nil)
}

// DeleteFolder deletes a catalog folder.
func (s *CatalogService) DeleteFolder(ctx context.Context, folderID int) error {
	if folderID == 0 {
		return ErrIDRequired
	}
	return s.do(ctx, http.MethodPost, "/catalog/folder/delete", map[string]any{"id": folderID}, nil)
}

// CleanFolder cleans a catalog folder.
func (s *CatalogService) CleanFolder(ctx context.Context, folderID int) error {
	if folderID == 0 {
		return ErrIDRequired
	}
	return s.do(ctx, http.MethodPost, "/catalog/folder/clean", map[string]any{"id": folderID}, nil)
}

// RunNL2SQL executes SQL generated by the NL2SQL UI path.
func (s *CatalogService) RunNL2SQL(ctx context.Context, databaseID int, sql string) (*CatalogNL2SQLRunResult, error) {
	if databaseID == 0 {
		return nil, ErrIDRequired
	}
	if sql == "" {
		return nil, ErrSQLRequired
	}
	var out CatalogNL2SQLRunResult
	err := s.do(ctx, http.MethodPost, "/catalog/nl2sql/run_sql", map[string]any{"database_id": databaseID, "sql": sql}, &out)
	return &out, err
}

func connectorDefinitionBody(name string, sourceType int, usageType int, opts ...ConnectorDefinitionOption) map[string]any {
	options := connectorDefinitionOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"name": name, "source_type": sourceType, "usage_type": usageType}
	if options.config != nil {
		body["config"] = options.config
	}
	return body
}

func connectorUpdateBody(opts ...ConnectorUpdateOption) map[string]any {
	options := connectorUpdateOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if options.name != nil {
		body["name"] = *options.name
	}
	if options.sourceType != nil {
		body["source_type"] = *options.sourceType
	}
	if options.usageType != nil {
		body["usage_type"] = *options.usageType
	}
	if options.config != nil {
		body["config"] = options.config
	}
	return body
}

func connectorPreviewBody(connectorID, uri string, opts ...ConnectorPreviewOption) map[string]any {
	options := connectorPreviewOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"connector_id": connectorID, "uri": uri}
	if options.connFileID != nil {
		body["conn_file_id"] = *options.connFileID
	}
	if options.sheetName != nil {
		body["sheet_name"] = *options.sheetName
	}
	if options.isColumnName != nil {
		body["isColumnName"] = *options.isColumnName
	}
	if options.rowStart != nil {
		body["rowStart"] = *options.rowStart
	}
	if options.columnNameRow != nil {
		body["columnNameRow"] = *options.columnNameRow
	}
	if options.csv != nil {
		body["csv"] = options.csv
	}
	if options.fileType != nil {
		body["file_type"] = *options.fileType
	}
	return body
}

func connectorDBBody(connectorID, database string, opts ...ConnectorDBOption) map[string]any {
	options := connectorDBOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"connector_id": connectorID, "database": database}
	if options.schema != nil {
		body["schema"] = *options.schema
	}
	return body
}

func exportTaskListBody(opts ...ExportTaskListOption) map[string]any {
	options := exportTaskListOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if options.keyword != nil {
		body["keyword"] = *options.keyword
	}
	if options.targetCategory != nil {
		body["target_category"] = *options.targetCategory
	}
	if options.limit != nil {
		body["limit"] = *options.limit
	}
	if options.offset != nil {
		body["offset"] = *options.offset
	}
	if options.connectorName != nil {
		body["connector_name"] = *options.connectorName
	}
	if options.taskID != nil {
		body["task_id"] = *options.taskID
	}
	if options.orderBy != nil {
		body["order_by"] = *options.orderBy
	}
	if options.orderDirection != nil {
		body["order_direction"] = *options.orderDirection
	}
	if len(options.statuses) > 0 {
		body["statuses"] = options.statuses
	}
	if options.creator != nil {
		body["creator"] = *options.creator
	}
	return body
}

func exportTaskFilesBody(opts ...ExportTaskFilesOption) map[string]any {
	options := exportTaskFilesOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if options.limit != nil {
		body["limit"] = *options.limit
	}
	if options.offset != nil {
		body["offset"] = *options.offset
	}
	if len(options.statuses) > 0 {
		body["statuses"] = options.statuses
	}
	if options.orderBy != nil {
		body["order_by"] = *options.orderBy
	}
	if options.orderDirection != nil {
		body["order_direction"] = *options.orderDirection
	}
	return body
}

func workbookListBody(opts ...WorkbookListOption) map[string]any {
	options := workbookListOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if options.search != nil {
		body["search"] = *options.search
	}
	if options.pageSize != nil {
		body["page_size"] = *options.pageSize
	}
	if options.pageToken != nil {
		body["page_token"] = *options.pageToken
	}
	return body
}

func workbookVersionListBody(workbookID string, opts ...WorkbookVersionListOption) map[string]any {
	options := workbookVersionListOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"workbook_id": workbookID}
	if options.pageSize != nil {
		body["page_size"] = *options.pageSize
	}
	if options.pageToken != nil {
		body["page_token"] = *options.pageToken
	}
	return body
}

func sqlQueryBody(query string, opts ...SQLQueryOption) map[string]any {
	options := sqlQueryOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"query": query}
	if options.dbName != nil {
		body["db_name"] = *options.dbName
	}
	if options.sqlType != nil {
		body["sql_type"] = *options.sqlType
	}
	if options.offset != nil {
		body["offset"] = *options.offset
	}
	if options.limit != nil {
		body["limit"] = *options.limit
	}
	return body
}

func sqlResultBody(statementID string, opts ...SQLResultOption) map[string]any {
	options := sqlResultOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"statement_id": statementID}
	if options.offset != nil {
		body["offset"] = *options.offset
	}
	if options.limit != nil {
		body["limit"] = *options.limit
	}
	return body
}

func queryHistoryOverviewBody(opts ...QueryHistoryOverviewOption) map[string]any {
	options := queryHistoryOverviewOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	applyDefaultQueryHistoryTimeRange(&options.start, &options.end, time.Now())
	body := map[string]any{}
	if options.scope != "" {
		body["scope"] = options.scope
	}
	if options.nonUser != nil {
		body["non_user"] = *options.nonUser
	}
	if options.start != nil {
		body["start"] = *options.start
	}
	if options.end != nil {
		body["end"] = *options.end
	}
	if len(options.sqlSourceTypes) > 0 {
		body["sql_source_type"] = options.sqlSourceTypes
	}
	return body
}

func queryHistoryBody(opts ...QueryHistoryOption) map[string]any {
	options := queryHistoryOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	applyDefaultQueryHistoryTimeRange(&options.start, &options.end, time.Now())
	body := map[string]any{}
	if options.scope != "" {
		body["scope"] = options.scope
	}
	if options.nonUser != nil {
		body["non_user"] = *options.nonUser
	}
	if options.status != nil {
		body["status"] = *options.status
	}
	if len(options.users) > 0 {
		body["user"] = options.users
	}
	if len(options.databases) > 0 {
		body["databases"] = options.databases
	}
	if options.statement != nil {
		body["statement"] = *options.statement
	}
	if len(options.queryTypes) > 0 {
		body["query_type"] = options.queryTypes
	}
	if options.minDuration != nil {
		body["duration"] = *options.minDuration
	}
	if options.start != nil {
		body["start"] = *options.start
	}
	if options.end != nil {
		body["end"] = *options.end
	}
	if options.statementID != nil {
		body["statement_id"] = *options.statementID
	}
	if options.transactionID != nil {
		body["transaction_id"] = *options.transactionID
	}
	if options.sessionID != nil {
		body["session_id"] = *options.sessionID
	}
	if len(options.selectedFields) > 0 {
		body["selected_field"] = options.selectedFields
	}
	if len(options.sqlSourceTypes) > 0 {
		body["sql_source_type"] = options.sqlSourceTypes
	}
	if options.offset != nil {
		body["offset"] = *options.offset
	}
	if options.limit != nil {
		body["limit"] = *options.limit
	}
	if len(options.orderBy) > 0 {
		body["order_by"] = options.orderBy
	}
	if options.order != nil {
		body["order"] = *options.order
	}
	if options.cu != nil {
		body["cu"] = *options.cu
	}
	return body
}

func applyDefaultQueryHistoryTimeRange(start, end **time.Time, now time.Time) {
	if *start != nil || *end != nil {
		return
	}
	endValue := now.UTC()
	startValue := endValue.Add(-DefaultQueryHistoryLookback)
	*start = &startValue
	*end = &endValue
}

func queryRecordBody(statementID string, opts ...QueryRecordOption) map[string]any {
	options := queryRecordOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"statement_id": statementID}
	if options.scope != "" {
		body["scope"] = options.scope
	}
	if options.start != nil {
		body["start"] = *options.start
	}
	if options.end != nil {
		body["end"] = *options.end
	}
	return body
}

func workflowDeployBody(name, dslYAML string, opts ...WorkflowDeployOption) (map[string]any, error) {
	options := workflowDeployOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"name": name, "dsl_yaml": dslYAML}
	if options.workflow != nil {
		if dslYAML != "" {
			return nil, fmt.Errorf("manual_mowl deployment does not accept dsl_yaml")
		}
		if options.sourceType != nil && *options.sourceType != "manual_mowl" {
			return nil, fmt.Errorf("authored workflow source_type must be manual_mowl")
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(*options.workflow, &object); err != nil || object == nil {
			return nil, fmt.Errorf("authored workflow must be a JSON object")
		}
		delete(body, "dsl_yaml")
		body["source_type"] = "manual_mowl"
		body["workflow"] = *options.workflow
	} else {
		if options.sourceType != nil && *options.sourceType != "manual_dsl" {
			return nil, fmt.Errorf("DSL workflow source_type must be manual_dsl")
		}
		if dslYAML == "" {
			return nil, ErrDSLYAMLRequired
		}
	}
	if options.workflowID != nil {
		body["workflow_id"] = *options.workflowID
	}
	if options.description != nil {
		body["description"] = *options.description
	}
	if options.executionMode != nil {
		body["execution_mode"] = *options.executionMode
	}
	if options.cronExpression != nil {
		body["cron_expression"] = *options.cronExpression
	}
	if options.sourceType != nil {
		body["source_type"] = *options.sourceType
	}
	if options.inputForm != nil {
		// Product workflow v2 deploy binds runtime form as runtime_fields/runtime_layout.
		body["runtime_fields"] = options.inputForm
	}
	if options.formLayout != nil {
		body["runtime_layout"] = options.formLayout
	}
	if options.volumeTrigger != nil {
		body["volume_trigger"] = *options.volumeTrigger
	}
	if options.dynamicService != nil {
		body["dynamic_service"] = *options.dynamicService
	}
	if options.defaultValues != nil {
		body["default_values"] = options.defaultValues
	}
	if options.computeResourceID != nil {
		body["compute_resource_id"] = *options.computeResourceID
	}
	if options.sessionID != nil {
		body["session_id"] = *options.sessionID
	}
	if options.notification != nil {
		body["notification_config"] = *options.notification
	}
	return body, nil
}

// workflowMOWLDeployBody builds a manual_mowl deployment request: the caller
// owns a compiled MOWL publication graph instead of DSL YAML. The graph must
// reference published workspace artifacts; the backend validates custom
// operator references before persisting the deployment.
func workflowMOWLDeployBody(name string, workflow map[string]any, opts ...WorkflowDeployOption) (map[string]any, error) {
	options := workflowDeployOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	if options.sourceType != nil && *options.sourceType != "manual_mowl" {
		return nil, fmt.Errorf("MOWL workflow source_type must be manual_mowl")
	}
	body := map[string]any{"name": name, "source_type": "manual_mowl", "workflow": workflow}
	if options.workflowID != nil {
		body["workflow_id"] = *options.workflowID
	}
	if options.description != nil {
		body["description"] = *options.description
	}
	if options.executionMode != nil {
		body["execution_mode"] = *options.executionMode
	}
	if options.cronExpression != nil {
		body["cron_expression"] = *options.cronExpression
	}
	if options.inputForm != nil {
		body["runtime_fields"] = options.inputForm
	}
	if options.formLayout != nil {
		body["runtime_layout"] = options.formLayout
	}
	if options.volumeTrigger != nil {
		body["volume_trigger"] = *options.volumeTrigger
	}
	if options.dynamicService != nil {
		body["dynamic_service"] = *options.dynamicService
	}
	if options.defaultValues != nil {
		body["default_values"] = options.defaultValues
	}
	if options.computeResourceID != nil {
		body["compute_resource_id"] = *options.computeResourceID
	}
	if options.sessionID != nil {
		body["session_id"] = *options.sessionID
	}
	if options.notification != nil {
		body["notification_config"] = *options.notification
	}
	return body, nil
}

// AcceptedWorkflowCandidateFromAcceptance validates a server acceptance result
// and creates the only candidate handle accepted by deployment.
func AcceptedWorkflowCandidateFromAcceptance(acceptance *WorkflowCandidateAcceptance) (*AcceptedWorkflowCandidate, error) {
	if acceptance == nil {
		return nil, fmt.Errorf("workflow candidate acceptance is required")
	}
	ref := acceptance.GetCandidateRef()
	candidate := acceptance.GetCandidate()
	if ref == nil || ref.GetTaskId() == "" || ref.GetArtifactId() == "" {
		return nil, fmt.Errorf("workflow candidate acceptance identity is required")
	}
	if acceptance.GetAcceptedAt() == "" {
		return nil, fmt.Errorf("workflow candidate accepted_at is required")
	}
	if candidate == nil || !candidate.GetOk() {
		return nil, fmt.Errorf("workflow candidate acceptance bundle is not successful")
	}
	if candidate.GetCandidateId() != ref.GetArtifactId() {
		return nil, fmt.Errorf("workflow candidate acceptance candidate_id does not match artifact_id")
	}
	if candidate.GetWorkflow() == nil {
		return nil, fmt.Errorf("workflow candidate acceptance workflow is required")
	}
	return &AcceptedWorkflowCandidate{
		ref: WorkflowCandidateRef{
			TaskID:     ref.GetTaskId(),
			ArtifactID: ref.GetArtifactId(),
		},
		acceptance: acceptance,
	}, nil
}

func workflowCandidateDeployBody(name string, accepted *AcceptedWorkflowCandidate, opts ...WorkflowDeployOption) (map[string]any, error) {
	if accepted == nil {
		return nil, fmt.Errorf("accepted workflow candidate is required")
	}
	ref := accepted.Ref()
	if ref.TaskID == "" || ref.ArtifactID == "" {
		return nil, fmt.Errorf("accepted workflow candidate identity is required")
	}
	options := workflowDeployOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	if options.sourceType != nil && *options.sourceType != "nl2dsl" {
		return nil, fmt.Errorf("candidate workflow source_type must be nl2dsl")
	}
	if options.inputForm != nil || options.formLayout != nil {
		return nil, fmt.Errorf("candidate workflow deploy does not accept runtime form or layout")
	}
	if options.workflow != nil {
		return nil, fmt.Errorf("candidate workflow deploy does not accept client-authored workflow")
	}
	body := map[string]any{
		"name":          name,
		"source_type":   "nl2dsl",
		"candidate_ref": ref,
	}
	if options.workflowID != nil {
		body["workflow_id"] = *options.workflowID
	}
	if options.description != nil {
		body["description"] = *options.description
	}
	if options.executionMode != nil {
		body["execution_mode"] = *options.executionMode
	}
	if options.cronExpression != nil {
		body["cron_expression"] = *options.cronExpression
	}
	if options.volumeTrigger != nil {
		body["volume_trigger"] = *options.volumeTrigger
	}
	if options.dynamicService != nil {
		body["dynamic_service"] = *options.dynamicService
	}
	if options.defaultValues != nil {
		body["default_values"] = options.defaultValues
	}
	if options.computeResourceID != nil {
		body["compute_resource_id"] = *options.computeResourceID
	}
	if options.notification != nil {
		body["notification_config"] = *options.notification
	}
	return body, nil
}

func workflowUpdateBody(opts ...WorkflowUpdateOption) map[string]any {
	options := workflowUpdateOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if options.name != nil {
		body["name"] = *options.name
	}
	if options.description != nil {
		body["description"] = *options.description
	}
	if options.executionMode != nil {
		body["execution_mode"] = *options.executionMode
	}
	if options.cronExpression != nil {
		body["cron_expression"] = *options.cronExpression
	}
	if options.dslYAML != nil {
		body["dsl_yaml"] = *options.dslYAML
	}
	if options.inputForm != nil {
		body["runtime_fields"] = options.inputForm
	}
	if options.formLayout != nil {
		body["runtime_layout"] = options.formLayout
	}
	if options.defaultValues != nil {
		body["default_values"] = options.defaultValues
	}
	if options.volumeTrigger != nil {
		body["volume_trigger"] = *options.volumeTrigger
	}
	if options.dynamicService != nil {
		body["dynamic_service"] = *options.dynamicService
	}
	return body
}

func workflowRetryBody(opts ...WorkflowRerunOption) map[string]any {
	options := workflowRerunOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if options.values != nil {
		body["values"] = options.values
	}
	if options.replaceSavedDefaults {
		body["replace_saved_defaults"] = true
	}
	if options.triggerNow != nil {
		body["trigger_now"] = *options.triggerNow
	}
	return body
}

func workflowNodeRerunBody(mode, sourceNodeExecutionID string, opts ...WorkflowRerunOption) map[string]any {
	options := workflowRerunOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"mode": mode, "source_node_execution_id": sourceNodeExecutionID}
	if options.values != nil {
		body["values"] = options.values
	}
	if options.configOverride != nil {
		body["config_override"] = *options.configOverride
	}
	if options.expectedRerunContractHash != nil {
		body["expected_rerun_contract_hash"] = *options.expectedRerunContractHash
	}
	if options.reason != nil {
		body["reason"] = *options.reason
	}
	return body
}

func workflowCancelBody(opts ...WorkflowCancelOption) map[string]any {
	options := workflowCancelOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if options.reason != nil {
		body["reason"] = *options.reason
	}
	return body
}

func customOperatorBody(name, identifier, description string, inputSchema, outputSchema any, opts ...CustomOperatorOption) map[string]any {
	options := customOperatorOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{
		"name":          name,
		"identifier":    identifier,
		"description":   description,
		"input_schema":  inputSchema,
		"output_schema": outputSchema,
	}
	if options.nodeID != nil {
		body["node_id"] = *options.nodeID
	}
	if options.kind != nil {
		body["kind"] = *options.kind
	}
	if options.language != nil {
		body["language"] = *options.language
	}
	if options.handler != nil {
		body["handler"] = *options.handler
	}
	if options.version != nil {
		body["version"] = *options.version
	}
	if options.isolationLevel != nil {
		body["isolation_level"] = *options.isolationLevel
	}
	if options.enabled != nil {
		body["enabled"] = *options.enabled
	}
	if options.code != nil {
		body["code"] = *options.code
	}
	if options.sourceFileID != nil {
		body["source_file_id"] = *options.sourceFileID
	}
	if options.baseNodeID != nil {
		body["base_node_id"] = *options.baseNodeID
	}
	if options.baseNodeVersion != nil {
		body["base_node_version"] = *options.baseNodeVersion
	}
	if options.bindingConfig != nil {
		body["binding_config"] = options.bindingConfig
	}
	if options.catalogID != nil {
		body["catalog_id"] = *options.catalogID
	}
	if options.databaseID != nil {
		body["database_id"] = *options.databaseID
	}
	return body
}

func customOperatorUpdateBody(opts ...CustomOperatorUpdateOption) map[string]any {
	options := customOperatorUpdateOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if options.name != nil {
		body["name"] = *options.name
	}
	if options.description != nil {
		body["description"] = *options.description
	}
	if options.kind != nil {
		body["kind"] = *options.kind
	}
	if options.language != nil {
		body["language"] = *options.language
	}
	if options.handler != nil {
		body["handler"] = *options.handler
	}
	if options.version != nil {
		body["version"] = *options.version
	}
	if options.isolationLevel != nil {
		body["isolation_level"] = *options.isolationLevel
	}
	if options.inputSchema != nil {
		body["input_schema"] = options.inputSchema
	}
	if options.outputSchema != nil {
		body["output_schema"] = options.outputSchema
	}
	if options.enabled != nil {
		body["enabled"] = *options.enabled
	}
	if options.code != nil {
		body["code"] = *options.code
	}
	if options.sourceFileID != nil {
		body["source_file_id"] = *options.sourceFileID
	}
	if options.baseNodeID != nil {
		body["base_node_id"] = *options.baseNodeID
	}
	if options.baseNodeVersion != nil {
		body["base_node_version"] = *options.baseNodeVersion
	}
	if options.bindingConfig != nil {
		body["binding_config"] = options.bindingConfig
	}
	if options.catalogID != nil {
		body["catalog_id"] = *options.catalogID
	}
	if options.databaseID != nil {
		body["database_id"] = *options.databaseID
	}
	return body
}

func workitemAPIServicePublishBody(spec WorkitemAPIServicePublishSpec) map[string]any {
	body := map[string]any{"version": spec.Version}
	if spec.ServiceName != "" {
		body["service_name"] = spec.ServiceName
	}
	if spec.ComputeResourceID != "" {
		body["compute_resource_id"] = spec.ComputeResourceID
	}
	if spec.TimeoutSeconds != 0 {
		body["timeout_seconds"] = spec.TimeoutSeconds
	}
	if spec.MaxConcurrency != 0 {
		body["max_concurrency"] = spec.MaxConcurrency
	}
	if spec.RateLimitPerMin != 0 {
		body["rate_limit_per_min"] = spec.RateLimitPerMin
	}
	if spec.UIResource != nil {
		body["ui_resource"] = spec.UIResource
	}
	return body
}

func workitemAPIServiceInvokeBody(spec WorkitemAPIServiceInvokeSpec) map[string]any {
	return map[string]any{
		"service_name": spec.ServiceName,
		"type":         spec.Type,
		"version":      spec.Version,
		"payload":      json.RawMessage(spec.Payload),
	}
}

func dynamicServiceInvokeBody(spec DynamicServiceInvokeSpec) map[string]any {
	body := map[string]any{
		"service_name": spec.ServiceName,
		"payload":      json.RawMessage(spec.Payload),
	}
	if spec.Version != "" {
		body["version"] = spec.Version
	}
	if spec.VersionNumber != 0 {
		body["version_number"] = spec.VersionNumber
	}
	if len(spec.Trace) > 0 {
		body["trace"] = json.RawMessage(spec.Trace)
	}
	return body
}

func workflowTemplateBody(name, dslYAML string, opts ...WorkflowTemplateOption) map[string]any {
	options := workflowTemplateOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{"name": name, "dsl_yaml": dslYAML}
	if options.templateKey != nil {
		body["template_key"] = *options.templateKey
	}
	if options.description != nil {
		body["description"] = *options.description
	}
	if options.inputForm != nil {
		body["runtime_fields"] = *options.inputForm
	}
	if options.isBuiltin != nil {
		body["is_builtin"] = *options.isBuiltin
	}
	return body
}

func workflowMOWLTemplateBody(name string, workflow map[string]any, opts ...WorkflowTemplateOption) (map[string]any, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	if len(workflow) == 0 {
		return nil, fmt.Errorf("MOWL workflow graph is required")
	}
	body := workflowTemplateBody(name, "", opts...)
	delete(body, "dsl_yaml")
	body["definition_format"] = "mowl"
	body["workflow"] = workflow
	return body, nil
}

func computeCreateBody(name, specID string, cpu, memoryGiB int, opts ...ComputeCreateOption) (map[string]any, error) {
	options := &computeCreateOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{
		"name":                 name,
		"spec_id":              specID,
		"cpu":                  cpu,
		"memory_gib":           memoryGiB,
		"auto_suspend_minutes": 10,
	}
	if options.kind != nil {
		body["kind"] = *options.kind
	}
	if options.pyRequirementsTxt != nil {
		body["py_requirements_txt"] = *options.pyRequirementsTxt
	}
	if options.description != nil {
		body["description"] = *options.description
	}
	if options.gpu != nil {
		body["gpu"] = *options.gpu
	}
	if options.minReplicas != nil {
		body["min_replicas"] = *options.minReplicas
	}
	if options.maxReplicas != nil {
		body["max_replicas"] = *options.maxReplicas
	}
	if options.autoSuspendMinutes != nil {
		body["auto_suspend_minutes"] = *options.autoSuspendMinutes
	}
	if options.workerImages != nil {
		workerImages, err := computeWorkerImageSelectionsBody(*options.workerImages)
		if err != nil {
			return nil, err
		}
		body["worker_images"] = workerImages
	}
	if options.goWorkerImageID != nil {
		body["go_worker_image_id"] = *options.goWorkerImageID
	}
	if options.pythonWorkerImageID != nil {
		body["python_worker_image_id"] = *options.pythonWorkerImageID
	}
	if options.platform != nil {
		body["platform"] = *options.platform
	}
	return body, nil
}

func computeUpdateBody(opts ...ComputeUpdateOption) (map[string]any, error) {
	options := &computeUpdateOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{}
	if options.kind != nil {
		body["kind"] = *options.kind
	}
	if options.pyRequirementsTxt != nil {
		body["py_requirements_txt"] = *options.pyRequirementsTxt
	}
	if options.name != nil {
		body["name"] = *options.name
	}
	if options.description != nil {
		body["description"] = *options.description
	}
	if options.specID != nil {
		body["spec_id"] = *options.specID
	}
	if options.cpu != nil {
		body["cpu"] = *options.cpu
	}
	if options.memoryGiB != nil {
		body["memory_gib"] = *options.memoryGiB
	}
	if options.gpu != nil {
		body["gpu"] = *options.gpu
	}
	if options.minReplicas != nil {
		body["min_replicas"] = *options.minReplicas
	}
	if options.maxReplicas != nil {
		body["max_replicas"] = *options.maxReplicas
	}
	if options.autoSuspendMinutes != nil {
		body["auto_suspend_minutes"] = *options.autoSuspendMinutes
	}
	if options.workerImages != nil {
		workerImages, err := computeWorkerImageSelectionsBody(*options.workerImages)
		if err != nil {
			return nil, err
		}
		body["worker_images"] = workerImages
	}
	if options.goWorkerImageID != nil {
		body["go_worker_image_id"] = *options.goWorkerImageID
	}
	if options.pythonWorkerImageID != nil {
		body["python_worker_image_id"] = *options.pythonWorkerImageID
	}
	if options.platform != nil {
		body["platform"] = *options.platform
	}
	return body, nil
}

// computeWorkerImageSelectionsBody keeps kind as an SDK input compatibility
// alias while sending only the canonical scheduling identity to Product APIs.
// Conflicting aliases are rejected before the request is sent.
func computeWorkerImageSelectionsBody(selections []*WorkerImageSelection) ([]map[string]string, error) {
	body := make([]map[string]string, 0, len(selections))
	for _, selection := range selections {
		workerType, kind, imageID, platform := "", "", "", ""
		if selection != nil {
			workerType = strings.TrimSpace(selection.GetWorkerType())
			kind = strings.TrimSpace(selection.GetKind())
			if workerType != "" && kind != "" && workerType != kind {
				return nil, ErrWorkerTypeConflict
			}
			if workerType == "" {
				workerType = kind
			}
			imageID = selection.GetImageId()
			platform = selection.GetPlatform()
		}
		body = append(body, map[string]string{
			"worker_type": workerType,
			"image_id":    imageID,
			"platform":    platform,
		})
	}
	return body, nil
}

func computeSpecBody(specID, kind, family, familyName, familyNameEN string, cpuMilli, memoryMiB int, opts ...ComputeSpecOption) (map[string]any, error) {
	if specID == "" {
		return nil, ErrSpecIDRequired
	}
	if kind == "" {
		return nil, ErrKindRequired
	}
	if family == "" {
		return nil, ErrTypeRequired
	}
	if familyName == "" {
		return nil, ErrNameRequired
	}
	if familyNameEN == "" {
		return nil, ErrNameRequired
	}
	if cpuMilli == 0 {
		return nil, ErrCPURequired
	}
	if memoryMiB == 0 {
		return nil, ErrMemoryRequired
	}
	body := map[string]any{
		"id":             specID,
		"kind":           kind,
		"family":         family,
		"family_name":    familyName,
		"family_name_en": familyNameEN,
		"cpu_milli":      cpuMilli,
		"memory_mib":     memoryMiB,
	}
	applyComputeSpecOptions(body, opts...)
	return body, nil
}

func computeSpecUpdateBody(opts ...ComputeSpecUpdateOption) map[string]any {
	options := &computeSpecUpdateOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{}
	if options.kind != nil {
		body["kind"] = *options.kind
	}
	if options.family != nil {
		body["family"] = *options.family
	}
	if options.familyName != nil {
		body["family_name"] = *options.familyName
	}
	if options.familyNameEN != nil {
		body["family_name_en"] = *options.familyNameEN
	}
	if options.cpuMilli != nil {
		body["cpu_milli"] = *options.cpuMilli
	}
	if options.memoryMiB != nil {
		body["memory_mib"] = *options.memoryMiB
	}
	applyComputeSpecOptionsFromValues(body, &options.computeSpecOptions)
	return body
}

func applyComputeSpecOptions(body map[string]any, opts ...ComputeSpecOption) {
	options := &computeSpecOptions{}
	for _, opt := range opts {
		opt(options)
	}
	applyComputeSpecOptionsFromValues(body, options)
}

func applyComputeSpecOptionsFromValues(body map[string]any, options *computeSpecOptions) {
	if options.nodePlacement != nil {
		body["node_placement"] = *options.nodePlacement
	}
	if options.gpuCount != nil {
		body["gpu_count"] = *options.gpuCount
	}
	if options.gpuMemoryMiB != nil {
		body["gpu_memory_mib"] = *options.gpuMemoryMiB
	}
	if options.gpuCores != nil {
		body["gpu_cores"] = *options.gpuCores
	}
	if options.creditPerHour != nil {
		body["credit_per_hour"] = *options.creditPerHour
	}
	if options.description != nil {
		body["description"] = *options.description
	}
	if options.enabled != nil {
		body["enabled"] = *options.enabled
	}
}

func providerBackendBody(name string, opts ...ProviderBackendOption) map[string]any {
	options := &providerBackendOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{"name": name}
	if options.providerType != nil {
		body["type"] = *options.providerType
	}
	if options.reasoningControlProtocol != nil {
		body["reasoning_control_protocol"] = *options.reasoningControlProtocol
	}
	if options.apiKey != nil {
		body["api_key"] = *options.apiKey
	}
	if options.address != nil {
		body["address"] = *options.address
	}
	if options.timeoutSeconds != nil {
		body["timeout_seconds"] = *options.timeoutSeconds
	}
	if len(options.models) > 0 {
		body["models"] = options.models
	}
	// supported_mime_types is parser-backend only (CreateParserBackendRequest).
	// LLM/embedding SetupBackendRequest does not accept it — apply only when
	// the caller is building a parser body (see applySupportedMimeTypes).
	return body
}

// applySupportedMimeTypes adds parser-only mime types onto an existing body.
// Used by ProviderBackendService when basePath is /parser.
func applySupportedMimeTypes(body map[string]any, opts ...ProviderBackendOption) {
	options := &providerBackendOptions{}
	for _, opt := range opts {
		opt(options)
	}
	if len(options.supportedMimeTypes) > 0 {
		body["supported_mime_types"] = options.supportedMimeTypes
	}
}

func providerBackendUpdateBody(opts ...ProviderBackendUpdateOption) map[string]any {
	options := &providerBackendUpdateOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{}
	if options.name != nil {
		body["name"] = *options.name
	}
	if options.apiKey != nil {
		body["api_key"] = *options.apiKey
	}
	if options.reasoningControlProtocol != nil {
		body["reasoning_control_protocol"] = *options.reasoningControlProtocol
	}
	if options.timeoutSeconds != nil {
		body["timeout_seconds"] = *options.timeoutSeconds
	}
	if len(options.models) > 0 {
		body["models"] = options.models
	}
	if len(options.supportedMimeTypes) > 0 {
		body["supported_mime_types"] = options.supportedMimeTypes
	}
	return body
}

func providerProbeBody(address string, opts ...ProviderBackendOption) map[string]any {
	options := &providerBackendOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{"address": address}
	if options.apiKey != nil {
		body["api_key"] = *options.apiKey
	}
	return body
}

func providerBackendProbeBody(opts ...ProviderBackendOption) map[string]any {
	options := &providerBackendOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{}
	if options.apiKey != nil {
		body["api_key"] = *options.apiKey
	}
	return body
}

func providerEndpointBody(address string) map[string]any {
	return map[string]any{"address": address}
}

func endpointStatusBody(status string) map[string]any {
	return map[string]any{"status": status}
}

func routerBody(opts ...RouterOption) map[string]any {
	options := &routerOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{}
	if options.strategy != nil {
		body["strategy"] = *options.strategy
	}
	if options.healthCheckInterval != nil {
		body["health_check_interval"] = *options.healthCheckInterval
	}
	if options.maxRetries != nil {
		body["max_retries"] = *options.maxRetries
	}
	if options.sessionAffinity != nil {
		body["session_affinity"] = *options.sessionAffinity
	}
	return body
}

func dataSharePublishBody(name, sourceDatabaseID string, tableScope *DataShareScope, targetWorkspaceIDs []string, opts ...DataSharePublishOption) map[string]any {
	options := &dataSharePublishOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{
		"name":                 name,
		"source_database_id":   sourceDatabaseID,
		"table_scope":          tableScope,
		"target_workspace_ids": targetWorkspaceIDs,
	}
	if options.remark != nil {
		body["remark"] = *options.remark
	}
	return body
}

func dataSharePublishUpdateBody(opts ...DataSharePublishUpdateOption) map[string]any {
	options := &dataSharePublishUpdateOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{}
	if options.tableScope != nil {
		body["table_scope"] = options.tableScope
	}
	if options.targetWorkspaceIDs != nil {
		body["target_workspace_ids"] = options.targetWorkspaceIDs
	}
	if options.remark != nil {
		body["remark"] = *options.remark
	}
	return body
}

func dataShareSubscribeBody(subName string) map[string]any {
	return map[string]any{"sub_name": subName}
}

func semanticModelBody(name string, opts ...SemanticModelOption) map[string]any {
	options := &semanticModelOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{"name": name}
	if options.description != nil {
		body["description"] = *options.description
	}
	if options.tables != nil {
		body["tables"] = options.tables
	}
	if options.files != nil {
		body["files"] = options.files
	}
	return body
}

func semanticModelCreateBody(spec SemanticModelCreateSpec, opts ...SemanticModelOption) map[string]any {
	body := semanticModelBody(spec.Name, opts...)
	if spec.PreserveEmbedding != nil {
		body["preserve_embedding"] = *spec.PreserveEmbedding
	}
	if spec.TextSendDimensions != nil {
		body["text_send_dimensions"] = *spec.TextSendDimensions
	}
	if spec.ImageSendDimensions != nil {
		body["image_send_dimensions"] = *spec.ImageSendDimensions
	}
	if spec.DrawingStructuredEnabled != nil {
		body["drawing_structured_enabled"] = *spec.DrawingStructuredEnabled
	}
	if spec.DrawingNumberPolicy != nil {
		body["drawing_number_policy"] = *spec.DrawingNumberPolicy
	}
	return body
}

func semanticModelWithSourcesBody(name string, sources []SemanticModelSourceInput, opts ...SemanticModelWithSourcesOption) map[string]any {
	options := &semanticModelWithSourcesOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{"name": name, "sources": semanticModelSourcesBody(sources)}
	if options.targetCatalogID != nil {
		body["target_catalog_id"] = *options.targetCatalogID
	}
	if options.description != nil {
		body["description"] = *options.description
	}
	if options.files != nil {
		body["files"] = options.files
	}
	if options.imageIndexEnabled != nil {
		body["image_index_enabled"] = *options.imageIndexEnabled
	}
	if options.sourceSelections != nil {
		body["source_selections"] = semanticModelSourceSelectionsBody(options.sourceSelections)
	}
	return body
}

func semanticModelEmptyBody(name string, opts ...SemanticModelEmptyOption) map[string]any {
	options := &semanticModelEmptyOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{"name": name}
	if options.description != nil {
		body["description"] = *options.description
	}
	if options.imageIndexEnabled != nil {
		body["image_index_enabled"] = *options.imageIndexEnabled
	}
	return body
}

func semanticModelSourcesBody(sources []SemanticModelSourceInput) []map[string]any {
	items := make([]map[string]any, 0, len(sources))
	for _, source := range sources {
		body := map[string]any{"source_type": source.SourceType}
		if source.FileName != "" {
			body["file_name"] = source.FileName
		}
		if source.UploadKind != "" {
			body["upload_kind"] = source.UploadKind
		}
		if source.ParseMode != "" {
			body["parse_mode"] = source.ParseMode
		}
		if source.TableConfig != "" {
			body["table_config"] = source.TableConfig
		}
		if source.FileID != "" {
			body["file_id"] = source.FileID
		}
		if source.VolumeID != 0 {
			body["volume_id"] = source.VolumeID
		}
		if source.TableID != 0 {
			body["table_id"] = source.TableID
		}
		items = append(items, body)
	}
	return items
}

func semanticModelSourceSelectionsBody(selections []SemanticModelSourceSelectionInput) []map[string]any {
	items := make([]map[string]any, 0, len(selections))
	for _, selection := range selections {
		item := map[string]any{
			"kind":         selection.Kind,
			"all_selected": selection.AllSelected,
		}
		if selection.DatabaseID != 0 {
			item["database_id"] = selection.DatabaseID
		}
		if selection.VolumeID != 0 {
			item["volume_id"] = selection.VolumeID
		}
		if len(selection.SelectedTableIDs) > 0 {
			item["selected_table_ids"] = selection.SelectedTableIDs
		}
		if len(selection.ExcludedTableIDs) > 0 {
			item["excluded_table_ids"] = selection.ExcludedTableIDs
		}
		if len(selection.SelectedFileIDs) > 0 {
			item["selected_file_ids"] = selection.SelectedFileIDs
		}
		if len(selection.ExcludedFileIDs) > 0 {
			item["excluded_file_ids"] = selection.ExcludedFileIDs
		}
		filters := map[string]any{}
		if selection.Filters.TableName != "" {
			filters["table_name"] = selection.Filters.TableName
		}
		if selection.Filters.FileName != "" {
			filters["file_name"] = selection.Filters.FileName
		}
		if len(selection.Filters.FileExt) > 0 {
			filters["file_ext"] = selection.Filters.FileExt
		}
		if len(filters) > 0 {
			item["filters"] = filters
		}
		items = append(items, item)
	}
	return items
}

func semanticModelSourceGovernanceBody(opts ...SemanticModelSourceGovernanceOption) map[string]any {
	options := &semanticModelSourceGovernanceOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{}
	if options.tags != nil {
		body["tags"] = *options.tags
	}
	if options.clearExpiresAt {
		body["expires_at"] = nil
	} else if options.expiresAt != nil {
		body["expires_at"] = *options.expiresAt
	}
	if options.enabled != nil {
		body["enabled"] = *options.enabled
	}
	if options.forceEnabledAfterExpiry != nil {
		body["force_enabled_after_expiry"] = *options.forceEnabledAfterExpiry
	}
	return body
}

func semanticModelSegmentBaseBody(base SemanticModelSegmentBase) map[string]any {
	return map[string]any{
		"if_match": base.IfMatch,
	}
}

func semanticModelSegmentUpdateBody(input SemanticModelSegmentUpdateInput) map[string]any {
	body := semanticModelSegmentBaseBody(input.SemanticModelSegmentBase)
	if input.Content != nil {
		body["content"] = *input.Content
	}
	if input.OCRText != nil {
		body["ocr_text"] = *input.OCRText
	}
	if input.ImageDescription != nil {
		body["image_description"] = *input.ImageDescription
	}
	return body
}

func semanticModelSegmentCreateBody(input SemanticModelSegmentCreateInput) map[string]any {
	body := semanticModelSegmentBaseBody(input.SemanticModelSegmentBase)
	if input.Level != "" {
		body["level"] = input.Level
	}
	if input.Content != nil {
		body["content"] = *input.Content
	}
	if input.OCRText != nil {
		body["ocr_text"] = *input.OCRText
	}
	if input.ImageDescription != nil {
		body["image_description"] = *input.ImageDescription
	}
	if input.ImageFileID != "" {
		body["image_file_id"] = input.ImageFileID
	}
	if input.PageImageFileID != "" {
		body["page_image_file_id"] = input.PageImageFileID
	}
	if input.BBox != nil {
		body["bbox"] = input.BBox
	}
	if input.Metadata != nil {
		body["metadata"] = input.Metadata
	}
	return body
}

func semanticEntryBody(entry SemanticEntryInput) map[string]any {
	body := map[string]any{"kind": entry.Kind, "key": entry.Key, "spec": entry.Spec}
	if len(entry.Tables) > 0 {
		body["tables"] = entry.Tables
	}
	if domain := strings.TrimSpace(entry.Domain); domain != "" {
		body["domain"] = domain
	}
	return body
}

func semanticImportBody(entries []SemanticEntryInput) map[string]any {
	items := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		items = append(items, semanticEntryBody(entry))
	}
	return map[string]any{"entries": items}
}

func validateSemanticEntry(entry SemanticEntryInput) error {
	if entry.Kind == "" {
		return ErrKindRequired
	}
	if entry.Key == "" {
		return ErrKeyRequired
	}
	if entry.Spec == nil {
		return ErrSpecRequired
	}
	return nil
}

func lineageRevisionBody(opts ...LineageRevisionOption) map[string]any {
	options := &lineageRevisionOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{}
	if options.baseRevisionID != nil {
		body["base_revision_id"] = *options.baseRevisionID
	}
	if options.useOriginal != nil {
		body["use_original"] = *options.useOriginal
	}
	if options.useCurrentEffective != nil {
		body["use_current_effective"] = *options.useCurrentEffective
	}
	if options.expectedEffectiveSetVersion != nil {
		body["expected_effective_set_version"] = *options.expectedEffectiveSetVersion
	}
	if options.expectedBaseContentHash != nil {
		body["expected_base_content_hash"] = *options.expectedBaseContentHash
	}
	if options.revisionContentRef != nil {
		body["revision_content_ref"] = *options.revisionContentRef
	}
	if options.revisionContentHash != nil {
		body["revision_content_hash"] = *options.revisionContentHash
	}
	if options.revisionContentPayload != nil {
		body["revision_content_payload"] = *options.revisionContentPayload
	}
	if options.patchJSON != nil {
		body["patch_json"] = *options.patchJSON
	}
	return body
}

func lineageSwitchBody(switches []LineageRevisionSwitch, opts ...LineageSwitchOption) map[string]any {
	options := &lineageSwitchOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{"switches": switches}
	if options.expectedEffectiveSetVersion != nil {
		body["expected_effective_set_version"] = *options.expectedEffectiveSetVersion
	}
	return body
}

func lineageRerunBody(opts ...LineageRerunOption) map[string]any {
	options := &lineageRerunOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{}
	if options.mode != nil {
		body["mode"] = *options.mode
	}
	if len(options.configOverride) > 0 {
		body["config_override"] = options.configOverride
	}
	if len(options.inputPayloadOverrideJSON) > 0 {
		body["input_payload_override"] = options.inputPayloadOverrideJSON
	}
	if len(options.varsPayloadOverrideJSON) > 0 {
		body["vars_payload_override"] = options.varsPayloadOverrideJSON
	}
	return body
}

func lineageRevisionRerunBody(opts ...LineageRevisionRerunOption) map[string]any {
	options := &lineageRevisionRerunOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{}
	if options.mode != nil {
		body["mode"] = *options.mode
	}
	if options.sourceEffectiveSetVersion != nil {
		body["source_effective_set_version"] = *options.sourceEffectiveSetVersion
	}
	if len(options.selectedRevisionsJSON) > 0 {
		body["selected_revisions_json"] = options.selectedRevisionsJSON
	}
	return body
}

// List returns product connector definitions visible in UI.
func (s *ConnectorService) List(ctx context.Context, opts ...QueryOption) (*ConnectorListResult, error) {
	var out ConnectorListResult
	err := s.do(ctx, http.MethodGet, withQuery("/connectors/list", opts), nil, &out)
	return &out, err
}

// Get returns one connector by ID.
func (s *ConnectorService) Get(ctx context.Context, connectorID string) (*Connector, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	var out Connector
	err := s.do(ctx, http.MethodPost, "/connectors/get", map[string]any{"id": connectorID}, &out)
	return &out, err
}

// Create creates a product connector.
func (s *ConnectorService) Create(ctx context.Context, name string, sourceType int, usageType int, opts ...ConnectorDefinitionOption) (*Connector, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	var out Connector
	err := s.do(ctx, http.MethodPost, "/connectors", connectorDefinitionBody(name, sourceType, usageType, opts...), &out)
	return &out, err
}

// Update updates one connector.
func (s *ConnectorService) Update(ctx context.Context, connectorID string, opts ...ConnectorUpdateOption) error {
	if connectorID == "" {
		return ErrConnectorIDRequired
	}
	id, err := pathID(connectorID)
	if err != nil {
		return err
	}
	body := connectorUpdateBody(opts...)
	return s.do(ctx, http.MethodPut, "/connectors/"+id, body, nil)
}

// Delete deletes one connector.
func (s *ConnectorService) Delete(ctx context.Context, connectorID string) error {
	if connectorID == "" {
		return ErrConnectorIDRequired
	}
	id, err := pathID(connectorID)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodDelete, "/connectors/"+id, nil, nil)
}

// Validate validates a connector definition.
func (s *ConnectorService) Validate(ctx context.Context, name string, sourceType int, usageType int, opts ...ConnectorDefinitionOption) (*ConnectorValidateResult, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	var out ConnectorValidateResult
	err := s.do(ctx, http.MethodPost, "/connectors/validate", connectorDefinitionBody(name, sourceType, usageType, opts...), &out)
	return &out, err
}

// ProbeRESTDraft validates an unsaved REST API connector configuration.
func (s *ConnectorService) ProbeRESTDraft(ctx context.Context, config map[string]any) (*ConnectorValidateResult, error) {
	var out ConnectorValidateResult
	err := s.do(ctx, http.MethodPost, "/connectors/rest-api/probe", map[string]any{"config": config}, &out)
	return &out, err
}

// ProbeREST validates a saved REST API connector.
func (s *ConnectorService) ProbeREST(ctx context.Context, connectorID string) (*ConnectorValidateResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	id, err := pathID(connectorID)
	if err != nil {
		return nil, err
	}
	var out ConnectorValidateResult
	err = s.do(ctx, http.MethodPost, "/connectors/"+id+"/rest-api/probe", nil, &out)
	return &out, err
}

// PreviewREST reads a saved REST API connector with an explicit preview request.
func (s *ConnectorService) PreviewREST(ctx context.Context, connectorID string, preview RESTPreview) (*RESTReadResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	id, err := pathID(connectorID)
	if err != nil {
		return nil, err
	}
	var out RESTReadResult
	err = s.do(ctx, http.MethodPost, "/connectors/"+id+"/rest-api/preview", preview, &out)
	return &out, err
}

// Files lists temporary connector files.
func (s *ConnectorService) Files(ctx context.Context, opts ...QueryOption) (*ConnectorFileListResult, error) {
	var out ConnectorFileListResult
	err := s.do(ctx, http.MethodGet, withQuery("/connectors/files/list", opts), nil, &out)
	return &out, err
}

// Upload uploads connector input files and associated form fields.
func (s *ConnectorService) Upload(ctx context.Context, opts ...ConnectorUploadOption) (*UploadResponse, error) {
	options := &connectorUploadOptions{fields: map[string]string{}}
	for _, opt := range opts {
		opt(options)
	}
	buf, contentType, err := multipartBody(options.fields, options.files)
	if err != nil {
		return nil, err
	}
	var out UploadResponse
	err = s.doRaw(ctx, http.MethodPost, "/connectors/upload", buf, contentType, contentTypeJSON, &out)
	return &out, err
}

// LocalUpload uploads local connector files into backend temporary storage.
func (s *ConnectorService) LocalUpload(ctx context.Context, files ...MultipartFile) (*LocalUploadResult, error) {
	buf, contentType, err := multipartBody(nil, files)
	if err != nil {
		return nil, err
	}
	var out LocalUploadResult
	err = s.doRaw(ctx, http.MethodPost, "/connectors/file/upload", buf, contentType, contentTypeJSON, &out)
	return &out, err
}

// PreviewFile previews a connector file.
func (s *ConnectorService) PreviewFile(ctx context.Context, connectorID, uri string, opts ...ConnectorPreviewOption) (*PreviewFileResult, error) {
	body := connectorPreviewBody(connectorID, uri, opts...)
	if body["connector_id"] == "" && body["conn_file_id"] == nil {
		return nil, ErrConnectorIDRequired
	}
	var out PreviewFileResult
	err := s.do(ctx, http.MethodPost, "/connectors/file/preview", body, &out)
	return &out, err
}

// DBListDatabases lists source databases.
func (s *ConnectorService) DBListDatabases(ctx context.Context, connectorID string) (*DBListDatabasesResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	var out DBListDatabasesResult
	err := s.do(ctx, http.MethodPost, "/connectors/db/database/list", map[string]any{"connector_id": connectorID}, &out)
	return &out, err
}

// DBListSchemas lists source database schemas.
func (s *ConnectorService) DBListSchemas(ctx context.Context, connectorID, database string) (*DBListSchemasResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if database == "" {
		return nil, ErrDatabaseRequired
	}
	var out DBListSchemasResult
	err := s.do(ctx, http.MethodPost, "/connectors/db/database/schema/list", map[string]any{"connector_id": connectorID, "database": database}, &out)
	return &out, err
}

// DBListTables lists source database tables.
func (s *ConnectorService) DBListTables(ctx context.Context, connectorID, database string, opts ...ConnectorDBOption) (*DBListTablesResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if database == "" {
		return nil, ErrDatabaseRequired
	}
	body := connectorDBBody(connectorID, database, opts...)
	var out DBListTablesResult
	err := s.do(ctx, http.MethodPost, "/connectors/db/database/table/list", body, &out)
	return &out, err
}

// DBSourceSchema returns source table schema.
func (s *ConnectorService) DBSourceSchema(ctx context.Context, connectorID, database, table string, opts ...ConnectorDBOption) (*DBSourceSchemaResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if database == "" {
		return nil, ErrDatabaseRequired
	}
	if table == "" {
		return nil, ErrTableRequired
	}
	body := connectorDBBody(connectorID, database, opts...)
	body["table"] = table
	var out DBSourceSchemaResult
	err := s.do(ctx, http.MethodPost, "/connectors/db/schema", body, &out)
	return &out, err
}

// StructuredSources lists connector sources supported by structured loading.
func (s *ConnectorService) StructuredSources(ctx context.Context) (*StructuredSourceListResult, error) {
	var out StructuredSourceListResult
	err := s.do(ctx, http.MethodGet, "/connectors/structured/v1/sources", nil, &out)
	return &out, err
}

// StructuredSourceDatabases lists databases for a structured load source.
func (s *ConnectorService) StructuredSourceDatabases(ctx context.Context, connectorID, sourceType string) (*StructuredSourceDatabasesResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if sourceType == "" {
		return nil, ErrTypeRequired
	}
	var out StructuredSourceDatabasesResult
	err := s.do(ctx, http.MethodPost, "/connectors/structured/v1/source/databases", map[string]any{"connector_id": connectorID, "source_type": sourceType}, &out)
	return &out, err
}

// StructuredSourceSchemas lists schemas for a structured load source database.
func (s *ConnectorService) StructuredSourceSchemas(ctx context.Context, connectorID, sourceType, database string) (*StructuredSourceSchemasResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if sourceType == "" {
		return nil, ErrTypeRequired
	}
	if database == "" {
		return nil, ErrDatabaseRequired
	}
	var out StructuredSourceSchemasResult
	err := s.do(ctx, http.MethodPost, "/connectors/structured/v1/source/schemas", map[string]any{"connector_id": connectorID, "source_type": sourceType, "database": database}, &out)
	return &out, err
}

// StructuredSourceObjects lists tables or collections for a structured load source.
func (s *ConnectorService) StructuredSourceObjects(ctx context.Context, req StructuredSourceObjectsRequest) (*StructuredSourceObjectsResult, error) {
	if req.ConnectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if req.SourceType == "" {
		return nil, ErrTypeRequired
	}
	if req.Database == "" {
		return nil, ErrDatabaseRequired
	}
	var out StructuredSourceObjectsResult
	err := s.do(ctx, http.MethodPost, "/connectors/structured/v1/source/objects", req, &out)
	return &out, err
}

// StructuredSourceSchema returns the schema snapshot for one structured source object.
func (s *ConnectorService) StructuredSourceSchema(ctx context.Context, req StructuredSourceSchemaRequest) (*StructuredSourceSchemaResult, error) {
	if req.ConnectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if req.Source.SourceType == "" {
		return nil, ErrTypeRequired
	}
	if req.Source.SourceType != "SOURCE_TYPE_KAFKA" && req.Source.Database == "" {
		return nil, ErrDatabaseRequired
	}
	if req.Source.SourceType != "SOURCE_TYPE_KAFKA" && req.Source.Table == "" && req.Source.Collection == "" {
		return nil, ErrTableRequired
	}
	var out StructuredSourceSchemaResult
	err := s.do(ctx, http.MethodPost, "/connectors/structured/v1/source/schema", req, &out)
	return &out, err
}

// StructuredSourceCapability probes the runtime capabilities of one structured source object.
func (s *ConnectorService) StructuredSourceCapability(ctx context.Context, connectorID, sourceType, database, table, schema string) (*StructuredSourceCapabilityResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if sourceType == "" {
		return nil, ErrTypeRequired
	}
	if database == "" {
		return nil, ErrDatabaseRequired
	}
	if table == "" {
		return nil, ErrTableRequired
	}
	source := StructuredSourceRef{ConnectorID: connectorID, SourceType: sourceType, Database: database, Table: table, Schema: schema}
	var out StructuredSourceCapabilityResult
	err := s.do(ctx, http.MethodPost, "/connectors/structured/v1/source/capabilities", map[string]any{"connector_id": connectorID, "source": source}, &out)
	return &out, err
}

// StructuredSourceSample reads a bounded sample from a Kafka structured source.
func (s *ConnectorService) StructuredSourceSample(ctx context.Context, connectorID, sourceType string, limit int32, topicMode string, topics []string, topicPattern string) (*StructuredSourceSampleResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if sourceType == "" {
		return nil, ErrTypeRequired
	}
	if limit <= 0 {
		return nil, &Error{Code: "ErrLimitRequired", Message: "limit is required"}
	}
	sample := map[string]any{"topic_mode": topicMode}
	if len(topics) > 0 {
		sample["topics"] = topics
	}
	if topicPattern != "" {
		sample["topic_pattern"] = topicPattern
	}
	source := StructuredSourceRef{ConnectorID: connectorID, SourceType: sourceType}
	var out StructuredSourceSampleResult
	err := s.do(ctx, http.MethodPost, "/connectors/structured/v1/source/sample", map[string]any{
		"connector_id": connectorID,
		"source":       source,
		"limit":        limit,
		"kafka_sample": sample,
	}, &out)
	return &out, err
}

// StructuredBackfillStartResolve resolves a structured load backfill start position.
func (s *ConnectorService) StructuredBackfillStartResolve(ctx context.Context, req StructuredBackfillStartResolveRequest) (*StructuredBackfillStartResolveResult, error) {
	if req.ConnectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if req.Source.SourceType == "" {
		return nil, ErrTypeRequired
	}
	if req.Source.Database == "" {
		return nil, ErrDatabaseRequired
	}
	if req.StartMode == "" {
		return nil, ErrModeRequired
	}
	var out StructuredBackfillStartResolveResult
	err := s.do(ctx, http.MethodPost, "/connectors/structured/v1/source/backfill-start/resolve", req, &out)
	return &out, err
}

// FileDownload returns a temporary connector file download URL.
func (s *ConnectorService) FileDownload(ctx context.Context, connFileID string) (*FileDownloadResult, error) {
	if connFileID == "" {
		return nil, ErrConnectorFileIDRequired
	}
	var out FileDownloadResult
	err := s.do(ctx, http.MethodPost, "/connectors/file/download", map[string]any{"conn_file_id": connFileID}, &out)
	return &out, err
}

// FileStream streams a temporary connector file by conn_file_id.
func (s *ConnectorService) FileStream(ctx context.Context, connFileID string) (*StreamResponse, error) {
	if connFileID == "" {
		return nil, ErrConnectorFileIDRequired
	}
	return s.doDownload(ctx, http.MethodGet, withQuery("/connectors/file/stream", []QueryOption{WithQueryParam("conn_file_id", connFileID)}), nil)
}

// FileDelete deletes a temporary connector file.
func (s *ConnectorService) FileDelete(ctx context.Context, connFileID string) error {
	if connFileID == "" {
		return ErrConnectorFileIDRequired
	}
	return s.do(ctx, http.MethodPost, "/connectors/file/delete", map[string]any{"conn_file_id": connFileID}, nil)
}

// MODatabaseList lists MatrixOne databases for export/import screens.
func (s *ConnectorService) MODatabaseList(ctx context.Context, connectorID string) (*MODatabaseListResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	var out MODatabaseListResult
	err := s.do(ctx, http.MethodPost, "/connectors/mo/database/list", map[string]any{"connector_id": connectorID}, &out)
	return &out, err
}

// MODatabaseTableList lists MatrixOne tables for export/import screens.
func (s *ConnectorService) MODatabaseTableList(ctx context.Context, connectorID, database string) (*MODatabaseTableListResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if database == "" {
		return nil, ErrDatabaseRequired
	}
	var out MODatabaseTableListResult
	err := s.do(ctx, http.MethodPost, "/connectors/mo/database/table/list", map[string]any{"connector_id": connectorID, "database": database}, &out)
	return &out, err
}

// MOCheckTableName checks MatrixOne target table name availability.
func (s *ConnectorService) MOCheckTableName(ctx context.Context, connectorID, database, table string) (*MOCheckTableNameResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if database == "" {
		return nil, ErrDatabaseRequired
	}
	if table == "" {
		return nil, ErrTableRequired
	}
	var out MOCheckTableNameResult
	err := s.do(ctx, http.MethodPost, "/connectors/mo/check_table_name", map[string]any{"connector_id": connectorID, "database": database, "table": table}, &out)
	return &out, err
}

// MOColumnMapping returns MatrixOne column mapping suggestions.
func (s *ConnectorService) MOColumnMapping(ctx context.Context, connectorID, database, table string, exportColumns map[string]string) (*MOColumnMappingResult, error) {
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	if database == "" {
		return nil, ErrDatabaseRequired
	}
	if table == "" {
		return nil, ErrTableRequired
	}
	var out MOColumnMappingResult
	err := s.do(ctx, http.MethodPost, "/connectors/mo/column/mapping", map[string]any{
		"connector_id":   connectorID,
		"database":       database,
		"table":          table,
		"export_columns": exportColumns,
	}, &out)
	return &out, err
}

// TaskStats returns visible workspace task counts without task details.
func (s *ConnectorService) TaskStats(ctx context.Context) (*ImportTaskStats, error) {
	var out ImportTaskStats
	err := s.do(ctx, http.MethodGet, "/task/stats", nil, &out)
	return &out, err
}

// Tasks lists import tasks.
func (s *ConnectorService) Tasks(ctx context.Context, opts ...QueryOption) (*ImportTaskListResult, error) {
	var out ImportTaskListResult
	err := s.do(ctx, http.MethodGet, withQuery("/task/list", opts), nil, &out)
	return &out, err
}

// CreateTask creates an import task.
func (s *ConnectorService) CreateTask(ctx context.Context, spec ImportTaskCreateSpec) (*ImportTaskCreateResult, error) {
	body, err := spec.Body()
	if err != nil {
		return nil, err
	}
	var out ImportTaskCreateResult
	err = s.do(ctx, http.MethodPost, "/task", body, &out)
	return &out, err
}

// UpdateTask updates an import task.
func (s *ConnectorService) UpdateTask(ctx context.Context, taskID string, spec ImportTaskUpdateSpec) error {
	if taskID == "" {
		return ErrTaskIDRequired
	}
	updates, err := spec.Updates()
	if err != nil {
		return err
	}
	body := map[string]any{"task_id": taskID}
	for key, value := range updates {
		body[key] = value
	}
	return s.do(ctx, http.MethodPost, "/task/update", body, nil)
}

// UpdateTaskFromSpec updates an import task with an explicit product spec.
func (s *ConnectorService) UpdateTaskFromSpec(ctx context.Context, taskID string, spec ImportTaskUpdateSpec) error {
	return s.UpdateTask(ctx, taskID, spec)
}

// DeleteTask deletes an import task.
func (s *ConnectorService) DeleteTask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return ErrTaskIDRequired
	}
	return s.do(ctx, http.MethodPost, "/task/delete", map[string]any{"task_id": taskID}, nil)
}

// PauseTask pauses an import task.
func (s *ConnectorService) PauseTask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return ErrTaskIDRequired
	}
	return s.do(ctx, http.MethodPost, "/task/pause", map[string]any{"task_id": taskID}, nil)
}

// ResumeTask resumes an import task.
func (s *ConnectorService) ResumeTask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return ErrTaskIDRequired
	}
	return s.do(ctx, http.MethodPost, "/task/resume", map[string]any{"task_id": taskID}, nil)
}

// RunTaskNow starts one scheduled connector import task immediately.
func (s *ConnectorService) RunTaskNow(ctx context.Context, taskID string) error {
	if taskID == "" {
		return ErrTaskIDRequired
	}
	return s.do(ctx, http.MethodPost, "/task/run-now", map[string]any{"task_id": taskID}, nil)
}

// RunTaskFullRefresh starts one full-refresh execution without changing the task schedule.
func (s *ConnectorService) RunTaskFullRefresh(ctx context.Context, taskID string) error {
	if taskID == "" {
		return ErrTaskIDRequired
	}
	return s.do(ctx, http.MethodPost, "/task/run-full-refresh", map[string]any{"task_id": taskID}, nil)
}

// GetTask returns one import task.
func (s *ConnectorService) GetTask(ctx context.Context, taskID string) (*ImportTaskInfoResult, error) {
	if taskID == "" {
		return nil, ErrTaskIDRequired
	}
	var out ImportTaskInfoResult
	err := s.do(ctx, http.MethodGet, withQuery("/task/get", []QueryOption{WithQueryParam("task_id", taskID)}), nil, &out)
	return &out, err
}

// TaskFiles lists files for import tasks.
func (s *ConnectorService) TaskFiles(ctx context.Context, opts ...QueryOption) (*ImportTaskFileListResult, error) {
	var out ImportTaskFileListResult
	err := s.do(ctx, http.MethodGet, withQuery("/task/files", opts), nil, &out)
	return &out, err
}

// TaskRuns lists task runs by task id.
func (s *ConnectorService) TaskRuns(ctx context.Context, taskID string) (*ImportTaskRunListResult, error) {
	if taskID == "" {
		return nil, ErrTaskIDRequired
	}
	var out ImportTaskRunListResult
	err := s.do(ctx, http.MethodGet, withQuery("/task/runs", []QueryOption{WithQueryParam("task_id", taskID)}), nil, &out)
	return &out, err
}

// RetryTaskFiles retries failed import task files.
func (s *ConnectorService) RetryTaskFiles(ctx context.Context, taskID string, ids ...string) error {
	if taskID == "" {
		return ErrTaskIDRequired
	}
	return s.do(ctx, http.MethodPost, "/task/retry", map[string]any{"task_id": taskID, "ids": ids}, nil)
}

// ExportTaskStats counts visible export tasks independently of list filters.
func (s *ConnectorService) ExportTaskStats(ctx context.Context) (*ExportTaskStats, error) {
	var out ExportTaskStats
	err := s.do(ctx, http.MethodGet, "/export/task/stats", nil, &out)
	return &out, err
}

// ExportTasks lists export tasks.
func (s *ConnectorService) ExportTasks(ctx context.Context, opts ...ExportTaskListOption) (*ExportTaskListResult, error) {
	var out ExportTaskListResult
	err := s.do(ctx, http.MethodPost, "/export/task/list", exportTaskListBody(opts...), &out)
	return &out, err
}

// ExportTaskInfo returns one export task detail (backend ExportTaskInfo wire).
func (s *ConnectorService) ExportTaskInfo(ctx context.Context, taskID string) (*ExportTaskInfo, error) {
	if taskID == "" {
		return nil, ErrTaskIDRequired
	}
	var out ExportTaskInfo
	err := s.do(ctx, http.MethodPost, "/export/task/info", map[string]any{"id": taskID}, &out)
	return &out, err
}

// ExportTaskFiles lists files for one export task.
func (s *ConnectorService) ExportTaskFiles(ctx context.Context, taskID string, opts ...ExportTaskFilesOption) (*ExportTaskFilesResult, error) {
	if taskID == "" {
		return nil, ErrTaskIDRequired
	}
	body := exportTaskFilesBody(opts...)
	body["task_id"] = taskID
	var out ExportTaskFilesResult
	err := s.do(ctx, http.MethodPost, "/export/task/files", body, &out)
	return &out, err
}

// DeleteExportTask deletes one export task.
func (s *ConnectorService) DeleteExportTask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return ErrTaskIDRequired
	}
	return s.do(ctx, http.MethodPost, "/export/task/delete", map[string]any{"id": taskID}, nil)
}

// CreateExportTask creates an export task.
func (s *ConnectorService) CreateExportTask(ctx context.Context, spec ExportTaskSpec) (*ExportTask, error) {
	body, err := spec.Body()
	if err != nil {
		return nil, err
	}
	var out ExportTask
	err = s.do(ctx, http.MethodPost, "/export/task/create", body, &out)
	return &out, err
}

// RetryExportTask retries one export task.
func (s *ConnectorService) RetryExportTask(ctx context.Context, taskID string, fileIDs ...string) error {
	if taskID == "" {
		return ErrTaskIDRequired
	}
	id, err := pathID(taskID)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodPost, "/export/task/"+id+"/rerun", map[string]any{"files": fileIDs}, nil)
}

// ExportTaskState returns one export task state.
func (s *ConnectorService) ExportTaskState(ctx context.Context, taskID string) (*ExportTaskStateResult, error) {
	if taskID == "" {
		return nil, ErrTaskIDRequired
	}
	id, err := pathID(taskID)
	if err != nil {
		return nil, err
	}
	var out ExportTaskStateResult
	err = s.do(ctx, http.MethodGet, "/export/task/"+id+"/state", nil, &out)
	return &out, err
}

// CreateWorkbook creates a SQL workbook.
func (s *SQLService) CreateWorkbook(ctx context.Context, name string) (*SQLWorkbook, error) {
	var out SQLWorkbook
	err := s.do(ctx, http.MethodPost, "/workbook/create", map[string]any{"name": name}, &out)
	return &out, err
}

// Workbooks lists SQL workbooks.
func (s *SQLService) Workbooks(ctx context.Context, opts ...WorkbookListOption) (*SQLWorkbookListResult, error) {
	var out SQLWorkbookListResult
	err := s.do(ctx, http.MethodPost, "/workbook/list", workbookListBody(opts...), &out)
	return &out, err
}

// UpdateWorkbook updates a SQL workbook.
func (s *SQLService) UpdateWorkbook(ctx context.Context, workbookID, name string) error {
	if workbookID == "" {
		return ErrWorkbookIDRequired
	}
	if name == "" {
		return ErrNameRequired
	}
	return s.do(ctx, http.MethodPost, "/workbook/update", map[string]any{"workbook_id": workbookID, "name": name}, nil)
}

// DeleteWorkbook deletes a SQL workbook.
func (s *SQLService) DeleteWorkbook(ctx context.Context, workbookID string) error {
	if workbookID == "" {
		return ErrWorkbookIDRequired
	}
	return s.do(ctx, http.MethodPost, "/workbook/delete", map[string]any{"workbook_id": workbookID}, nil)
}

// WorkbookDetail returns one workbook version content.
func (s *SQLService) WorkbookDetail(ctx context.Context, workbookID, versionID string) (*SQLWorkbookDetailResult, error) {
	if workbookID == "" {
		return nil, ErrWorkbookIDRequired
	}
	if versionID == "" {
		return nil, ErrVersionIDRequired
	}
	var out SQLWorkbookDetailResult
	err := s.do(ctx, http.MethodPost, "/workbook/detail", map[string]any{"workbook_id": workbookID, "version_id": versionID}, &out)
	return &out, err
}

// WorkbookVersions lists versions for one workbook.
func (s *SQLService) WorkbookVersions(ctx context.Context, workbookID string, opts ...WorkbookVersionListOption) (*SQLWorkbookVersionListResult, error) {
	if workbookID == "" {
		return nil, ErrWorkbookIDRequired
	}
	var out SQLWorkbookVersionListResult
	err := s.do(ctx, http.MethodPost, "/workbook/version/list", workbookVersionListBody(workbookID, opts...), &out)
	return &out, err
}

// CreateWorkbookVersion creates a workbook version.
func (s *SQLService) CreateWorkbookVersion(ctx context.Context, workbookID, sqlContent string) (*SQLWorkbookVersionCreateResult, error) {
	if workbookID == "" {
		return nil, ErrWorkbookIDRequired
	}
	var out SQLWorkbookVersionCreateResult
	err := s.do(ctx, http.MethodPost, "/workbook/version/create", map[string]any{"workbook_id": workbookID, "sql_content": sqlContent}, &out)
	return &out, err
}

// UpdateWorkbookVersion updates a workbook version draft.
func (s *SQLService) UpdateWorkbookVersion(ctx context.Context, workbookID, versionID, sqlContent string) error {
	if workbookID == "" {
		return ErrWorkbookIDRequired
	}
	if versionID == "" {
		return ErrVersionIDRequired
	}
	return s.do(ctx, http.MethodPost, "/workbook/version/update", map[string]any{"workbook_id": workbookID, "version_id": versionID, "sql_content": sqlContent}, nil)
}

// SaveWorkbookVersion finalizes a workbook version.
func (s *SQLService) SaveWorkbookVersion(ctx context.Context, workbookID, versionID string, sqlContent *string) error {
	if workbookID == "" {
		return ErrWorkbookIDRequired
	}
	if versionID == "" {
		return ErrVersionIDRequired
	}
	body := map[string]any{"workbook_id": workbookID, "version_id": versionID}
	if sqlContent != nil {
		body["sql_content"] = *sqlContent
	}
	return s.do(ctx, http.MethodPost, "/workbook/version/save", body, nil)
}

// MetaDatabases lists SQL editor database names. Without options it returns all names.
func (s *SQLService) MetaDatabases(ctx context.Context, opts ...SQLDatabaseListOption) (*SQLDatabaseListResult, error) {
	options := sqlDatabaseListOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	body := map[string]any{}
	if options.pageSize != nil {
		body["page_size"] = *options.pageSize
	}
	if options.pageToken != nil {
		body["page_token"] = *options.pageToken
	}
	if options.search != nil {
		body["search"] = *options.search
	}
	if options.name != nil {
		body["name"] = *options.name
	}
	var out SQLDatabaseListResult
	var requestBody any
	if len(body) > 0 {
		requestBody = body
	}
	err := s.do(ctx, http.MethodPost, "/meta/db/info", requestBody, &out)
	return &out, err
}

// MetaDBTree returns SQL editor database tree metadata.
func (s *SQLService) MetaDBTree(ctx context.Context) (*SQLDatabaseTreeResult, error) {
	var out SQLDatabaseTreeResult
	err := s.do(ctx, http.MethodPost, "/meta/db/tree", nil, &out)
	return &out, err
}

// MetaDBTables lists tables under one database.
func (s *SQLService) MetaDBTables(ctx context.Context, dbName string) (*SQLDatabaseTablesResult, error) {
	if dbName == "" {
		return nil, ErrDatabaseRequired
	}
	var out SQLDatabaseTablesResult
	err := s.do(ctx, http.MethodPost, "/meta/db/table", map[string]any{"name": dbName}, &out)
	return &out, err
}

// MetaTableInfo returns table data and columns.
func (s *SQLService) MetaTableInfo(ctx context.Context, dbName, tableName string) (*SQLTableInfoResult, error) {
	if dbName == "" {
		return nil, ErrDatabaseRequired
	}
	if tableName == "" {
		return nil, ErrTableRequired
	}
	var out SQLTableInfoResult
	err := s.do(ctx, http.MethodPost, "/meta/table/info", map[string]any{"db_name": dbName, "table_name": tableName}, &out)
	return &out, err
}

// MetaTableColumns returns table columns.
func (s *SQLService) MetaTableColumns(ctx context.Context, dbName, tableName string) (*SQLTableColumnsResult, error) {
	if dbName == "" {
		return nil, ErrDatabaseRequired
	}
	if tableName == "" {
		return nil, ErrTableRequired
	}
	var out SQLTableColumnsResult
	err := s.do(ctx, http.MethodPost, "/meta/table/column", map[string]any{"db_name": dbName, "table_name": tableName}, &out)
	return &out, err
}

// MetaTableColumnStats returns table column statistics.
func (s *SQLService) MetaTableColumnStats(ctx context.Context, dbName, tableName string) (*SQLTableColumnsResult, error) {
	if dbName == "" {
		return nil, ErrDatabaseRequired
	}
	if tableName == "" {
		return nil, ErrTableRequired
	}
	var out SQLTableColumnsResult
	err := s.do(ctx, http.MethodPost, "/meta/table/column_stat", map[string]any{"db_name": dbName, "table_name": tableName}, &out)
	return &out, err
}

// MetaTableDDL returns table DDL.
func (s *SQLService) MetaTableDDL(ctx context.Context, dbName, tableName string) (*SQLTableDDLResult, error) {
	if dbName == "" {
		return nil, ErrDatabaseRequired
	}
	if tableName == "" {
		return nil, ErrTableRequired
	}
	var out SQLTableDDLResult
	err := s.do(ctx, http.MethodPost, "/meta/table/ddl", map[string]any{"db_name": dbName, "table_name": tableName}, &out)
	return &out, err
}

// DropMetaDatabase drops a database from SQL editor.
func (s *SQLService) DropMetaDatabase(ctx context.Context, dbName string) error {
	if dbName == "" {
		return ErrDatabaseRequired
	}
	return s.do(ctx, http.MethodPost, "/meta/db/drop", map[string]any{"db_name": dbName}, nil)
}

// DropMetaTable drops a table from SQL editor.
func (s *SQLService) DropMetaTable(ctx context.Context, dbName, tableName string) error {
	if dbName == "" {
		return ErrDatabaseRequired
	}
	if tableName == "" {
		return ErrTableRequired
	}
	return s.do(ctx, http.MethodPost, "/meta/table/drop", map[string]any{"db_name": dbName, "table_name": tableName}, nil)
}

// DropMetaView drops a view from SQL editor.
func (s *SQLService) DropMetaView(ctx context.Context, dbName, tableName string) error {
	if dbName == "" {
		return ErrDatabaseRequired
	}
	if tableName == "" {
		return ErrTableRequired
	}
	return s.do(ctx, http.MethodPost, "/meta/view/drop", map[string]any{"db_name": dbName, "table_name": tableName}, nil)
}

// ExecuteQuery starts SQL execution.
func (s *SQLService) ExecuteQuery(ctx context.Context, query string, opts ...SQLQueryOption) (*SQLQueryStartResult, error) {
	if query == "" {
		return nil, ErrQueryRequired
	}
	var out SQLQueryStartResult
	err := s.do(ctx, http.MethodPost, "/query/execute", sqlQueryBody(query, opts...), &out)
	return &out, err
}

// RunQuery executes once and returns the bounded result in the same response.
// Unlike ExecuteQuery, it does not require a subsequent describe/result read.
func (s *SQLService) RunQuery(ctx context.Context, query string, opts ...SQLQueryOption) (*SQLQueryResult, error) {
	if query == "" {
		return nil, ErrQueryRequired
	}
	var out SQLQueryResult
	err := s.do(ctx, http.MethodPost, "/query/run", sqlQueryBody(query, opts...), &out)
	return &out, err
}

// DescribeQuery returns query operation state.
func (s *SQLService) DescribeQuery(ctx context.Context, queryID string) (*SQLQueryOperation, error) {
	if queryID == "" {
		return nil, ErrQueryIDRequired
	}
	var out SQLQueryOperation
	err := s.do(ctx, http.MethodPost, "/query/describe", map[string]any{"query_id": queryID}, &out)
	return &out, err
}

// KillQuery cancels a running query.
func (s *SQLService) KillQuery(ctx context.Context, queryID string) error {
	if queryID == "" {
		return ErrQueryIDRequired
	}
	return s.do(ctx, http.MethodPost, "/query/kill", map[string]any{"query_id": queryID}, nil)
}

// QueryResult returns query result rows.
func (s *SQLService) QueryResult(ctx context.Context, statementID string, opts ...SQLResultOption) (*SQLQueryResult, error) {
	if statementID == "" {
		return nil, ErrStatementIDRequired
	}
	var out SQLQueryResult
	err := s.do(ctx, http.MethodPost, "/query/result", sqlResultBody(statementID, opts...), &out)
	return &out, err
}

// QueryHistoryOverview returns SQL query history overview.
func (s *SQLService) QueryHistoryOverview(ctx context.Context, opts ...QueryHistoryOverviewOption) (*SQLQueryHistoryOverviewResult, error) {
	var out SQLQueryHistoryOverviewResult
	err := s.do(ctx, http.MethodPost, "/query/history/overview", queryHistoryOverviewBody(opts...), &out)
	return &out, err
}

// QueryHistory returns SQL query history list.
func (s *SQLService) QueryHistory(ctx context.Context, opts ...QueryHistoryOption) (*SQLQueryHistoryResult, error) {
	var out SQLQueryHistoryResult
	err := s.do(ctx, http.MethodPost, "/query/history", queryHistoryBody(opts...), &out)
	return &out, err
}

// QueryDetail returns SQL query detail.
func (s *SQLService) QueryDetail(ctx context.Context, statementID string, opts ...QueryRecordOption) (*SQLStatementRecord, error) {
	if statementID == "" {
		return nil, ErrStatementIDRequired
	}
	var out sqlStatementDetail
	err := s.do(ctx, http.MethodPost, "/query/detail", queryRecordBody(statementID, opts...), &out)
	return out.statement, err
}

// QueryProfile returns SQL query profile.
func (s *SQLService) QueryProfile(ctx context.Context, statementID string, opts ...QueryRecordOption) (*SQLQueryProfileResult, error) {
	if statementID == "" {
		return nil, ErrStatementIDRequired
	}
	var out SQLQueryProfileResult
	err := s.do(ctx, http.MethodPost, "/query/profile", queryRecordBody(statementID, opts...), &out)
	return &out, err
}

// QueryWebSocketURL returns the product WebSocket URL for SQL editor sessions.
func (s *SQLService) QueryWebSocketURL() (string, error) {
	base, err := s.client.webSocketBase(s.workspaceID)
	if err != nil {
		return "", err
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/query/ws"
	return base.String(), nil
}

// QueryWebSocketHeaders 返回 SQL 编辑器握手所需的产品身份和工作区。
func (s *SQLService) QueryWebSocketHeaders() http.Header {
	return s.client.webSocketHeaders(s.workspaceID)
}

// QueryWebSocketSubprotocols 返回产品认证子协议。
func (s *SQLService) QueryWebSocketSubprotocols() []string {
	return s.client.webSocketSubprotocols()
}

// List returns UI-visible workflow apps.
func (s *WorkflowService) List(ctx context.Context, opts ...ListOption) (*WorkflowListEnvelope, error) {
	var out WorkflowListEnvelope
	err := s.do(ctx, http.MethodGet, withListQuery("/workflow/v2/workflow-apps", opts), nil, &out)
	return &out, err
}

// Deploy deploys a manual workflow definition into a UI-visible workflow app.
func (s *WorkflowService) Deploy(ctx context.Context, name, dslYAML string, opts ...WorkflowDeployOption) (*WorkflowDeploymentEnvelope, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	body, err := workflowDeployBody(name, dslYAML, opts...)
	if err != nil {
		return nil, err
	}
	var out WorkflowDeploymentEnvelope
	err = s.do(ctx, http.MethodPost, "/workflow/v2/workflow-deployments", body, &out)
	return &out, err
}

// DeployMOWL deploys a caller-owned MOWL publication graph (source_type
// manual_mowl), the same path the editor uses for template-instantiated and
// imported workflows. Every custom operator reference in the graph must
// already exist in the workspace operator library; the backend rejects
// dangling references with ErrWorkflowCustomOperatorNotFound.
func (s *WorkflowService) DeployMOWL(ctx context.Context, name string, workflow map[string]any, opts ...WorkflowDeployOption) (*WorkflowDeploymentEnvelope, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	if len(workflow) == 0 {
		return nil, fmt.Errorf("MOWL workflow graph is required")
	}
	body, err := workflowMOWLDeployBody(name, workflow, opts...)
	if err != nil {
		return nil, err
	}
	var out WorkflowDeploymentEnvelope
	err = s.do(ctx, http.MethodPost, "/workflow/v2/workflow-deployments", body, &out)
	return &out, err
}

// AcceptCandidate explicitly accepts one server-owned A2A workflow candidate
// and returns the typed handle required for deployment.
func (s *WorkflowService) AcceptCandidate(ctx context.Context, ref WorkflowCandidateRef, confirmation *WorkflowCandidateConfirmation) (*AcceptedWorkflowCandidate, error) {
	if ref.TaskID == "" || ref.ArtifactID == "" {
		return nil, fmt.Errorf("candidate_ref task_id and artifact_id are required")
	}
	if confirmation == nil || confirmation.ProposalId == "" || confirmation.IdempotencyKey == "" {
		return nil, fmt.Errorf("proposal_id and idempotency_key are required")
	}
	var out WorkflowCandidateAcceptance
	if err := s.do(ctx, http.MethodPost, "/workflow/v2/workflow-candidates/accept", map[string]any{
		"candidate_ref": ref, "proposal_id": confirmation.ProposalId, "idempotency_key": confirmation.IdempotencyKey,
	}, &out); err != nil {
		return nil, err
	}
	return AcceptedWorkflowCandidateFromAcceptance(&out)
}

// RejectCandidateProposal discards an unaccepted workflow proposal without changing its base.
func (s *WorkflowService) RejectCandidateProposal(ctx context.Context, proposalID string) (*WorkflowCandidateProposalRejection, error) {
	if proposalID == "" {
		return nil, fmt.Errorf("proposal_id is required")
	}
	var out WorkflowCandidateProposalRejection
	if err := s.do(ctx, http.MethodPost, "/workflow/v2/workflow-candidates/reject-proposal", map[string]any{"proposal_id": proposalID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// History lists durable authoring changes without changing the active workflow.
func (s *WorkflowService) History(ctx context.Context, contextID string, limit, offset int) (*WorkflowHistoryPage, error) {
	var out WorkflowHistoryPage
	if err := s.do(ctx, http.MethodGet, withQuery("/workflow/v2/workflow-candidates/history", []QueryOption{
		WithQueryParam("context_id", contextID), WithQueryParam("limit", strconv.Itoa(limit)), WithQueryParam("offset", strconv.Itoa(offset)),
	}), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RestoreHistory creates a reviewable proposal, never an implicit acceptance.
func (s *WorkflowService) RestoreHistory(ctx context.Context, request WorkflowHistoryRestoreRequest) (*WorkflowHistoryEntry, error) {
	var out WorkflowHistoryEntry
	if err := s.do(ctx, http.MethodPost, "/workflow/v2/workflow-candidates/restore", request.body(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeployCandidate deploys a previously accepted server-owned A2A workflow candidate.
func (s *WorkflowService) DeployCandidate(ctx context.Context, name string, accepted *AcceptedWorkflowCandidate, opts ...WorkflowDeployOption) (*WorkflowDeploymentEnvelope, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	body, err := workflowCandidateDeployBody(name, accepted, opts...)
	if err != nil {
		return nil, err
	}
	var out WorkflowDeploymentEnvelope
	err = s.do(ctx, http.MethodPost, "/workflow/v2/workflow-deployments", body, &out)
	return &out, err
}

// Get returns one UI-visible workflow app.
func (s *WorkflowService) Get(ctx context.Context, workflowID string) (*WorkflowEnvelope, error) {
	id, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	var out WorkflowEnvelope
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/"+id, nil, &out)
	return &out, err
}

// Update updates one UI-visible workflow app.
func (s *WorkflowService) Update(ctx context.Context, workflowID string, opts ...WorkflowUpdateOption) (*WorkflowEnvelope, error) {
	id, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	var out WorkflowEnvelope
	err = s.do(ctx, http.MethodPatch, "/workflow/v2/workflow-apps/"+id, workflowUpdateBody(opts...), &out)
	return &out, err
}

// Delete deletes one UI-visible workflow app.
func (s *WorkflowService) Delete(ctx context.Context, workflowID string) (*WorkflowDeleteEnvelope, error) {
	id, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	var out WorkflowDeleteEnvelope
	err = s.do(ctx, http.MethodDelete, "/workflow/v2/workflow-apps/"+id, nil, &out)
	return &out, err
}

// Pause pauses one UI-visible workflow app.
func (s *WorkflowService) Pause(ctx context.Context, workflowID string) (*WorkflowEnvelope, error) {
	return s.workflowAction(ctx, workflowID, "pause")
}

// Enable transitions one disabled UI-visible workflow app to ready.
func (s *WorkflowService) Enable(ctx context.Context, workflowID string) (*WorkflowEnvelope, error) {
	return s.workflowAction(ctx, workflowID, "enable")
}

// Resume resumes one UI-visible workflow app.
func (s *WorkflowService) Resume(ctx context.Context, workflowID string) (*WorkflowEnvelope, error) {
	return s.workflowAction(ctx, workflowID, "resume")
}

type runWorkflowOptions struct {
	input                map[string]any
	replaceSavedDefaults bool
	triggerNow           *bool
}

// WorkflowRunOption configures workflow run creation.
type WorkflowRunOption func(*runWorkflowOptions)

// WithWorkflowRunInput sets runtime values for a workflow execution.
// Product workflow v2 binds these under JSON field "values" (field_id keyed).
func WithWorkflowRunInput(input map[string]any) WorkflowRunOption {
	return func(o *runWorkflowOptions) {
		o.input = input
	}
}

// WithWorkflowRunValues is an alias for WithWorkflowRunInput that matches the
// backend RunWorkflowRequest.values contract name.
func WithWorkflowRunValues(values map[string]any) WorkflowRunOption {
	return WithWorkflowRunInput(values)
}

// WithWorkflowRunReplaceSavedDefaults requests replacing the workflow's saved
// defaults with the supplied values. The backend contract only serializes the
// true case; false keeps the default merge behavior.
func WithWorkflowRunReplaceSavedDefaults(replace bool) WorkflowRunOption {
	return func(o *runWorkflowOptions) {
		o.replaceSavedDefaults = replace
	}
}

// WithWorkflowRunTriggerNow controls immediate dispatch for the execution.
// A pointer preserves omission versus an explicit false value.
func WithWorkflowRunTriggerNow(triggerNow bool) WorkflowRunOption {
	return func(o *runWorkflowOptions) {
		o.triggerNow = &triggerNow
	}
}

// Run starts a UI-visible workflow execution.
func (s *WorkflowService) Run(ctx context.Context, workflowID string, opts ...WorkflowRunOption) (*WorkflowRunEnvelope, error) {
	id, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	options := &runWorkflowOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{}
	if options.input != nil {
		// Backend RunWorkflowRequest uses "values", not "input".
		body["values"] = options.input
	}
	if options.replaceSavedDefaults {
		body["replace_saved_defaults"] = true
	}
	if options.triggerNow != nil {
		body["trigger_now"] = *options.triggerNow
	}
	var out WorkflowRunEnvelope
	err = s.do(ctx, http.MethodPost, "/workflow/v2/workflow-apps/"+id+"/executions", body, &out)
	return &out, err
}

// ListRuns lists executions for one workflow app.
func (s *WorkflowService) ListRuns(ctx context.Context, workflowID string, opts ...ListOption) (*WorkflowExecutionListEnvelope, error) {
	id, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	var out WorkflowExecutionListEnvelope
	err = s.do(ctx, http.MethodGet, withListQuery("/workflow/v2/workflow-apps/"+id+"/executions", opts), nil, &out)
	return &out, err
}

// ListAllRuns lists workflow executions without requiring a workflow id.
func (s *WorkflowService) ListAllRuns(ctx context.Context, opts ...ListOption) (*WorkflowExecutionListEnvelope, error) {
	var out WorkflowExecutionListEnvelope
	err := s.do(ctx, http.MethodGet, withListQuery("/workflow/v2/workflow-apps/executions", opts), nil, &out)
	return &out, err
}

// GetDispatchJob observes a submitted batch independently of its per-file executions.
func (s *WorkflowService) GetDispatchJob(ctx context.Context, workflowID, dispatchJobID string) (*WorkflowDispatchJobEnvelope, error) {
	workflowPath, dispatchPath, err := workflowRunPath(workflowID, dispatchJobID)
	if err != nil {
		return nil, err
	}
	var out WorkflowDispatchJobEnvelope
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/"+workflowPath+"/dispatch-jobs/"+dispatchPath, nil, &out)
	return &out, err
}

// GetRun returns one workflow execution.
func (s *WorkflowService) GetRun(ctx context.Context, workflowID, executionID string) (*WorkflowExecutionEnvelope, error) {
	workflowPath, executionPath, err := workflowRunPath(workflowID, executionID)
	if err != nil {
		return nil, err
	}
	var out WorkflowExecutionEnvelope
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/"+workflowPath+"/executions/"+executionPath, nil, &out)
	return &out, err
}

// GetRunByID returns one workflow execution without requiring a workflow id.
func (s *WorkflowService) GetRunByID(ctx context.Context, executionID string) (*WorkflowExecutionEnvelope, error) {
	id, err := pathID(executionID)
	if err != nil {
		return nil, err
	}
	var out WorkflowExecutionEnvelope
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/executions/"+id, nil, &out)
	return &out, err
}

// RefreshRun refreshes one workflow execution envelope.
func (s *WorkflowService) RefreshRun(ctx context.Context, workflowID, executionID string) (*WorkflowExecutionEnvelope, error) {
	var out WorkflowExecutionEnvelope
	err := s.workflowRunAction(ctx, workflowID, executionID, "refresh", nil, &out)
	return &out, err
}

// PauseRun pauses one workflow execution.
func (s *WorkflowService) PauseRun(ctx context.Context, workflowID, executionID string) (*WorkflowExecutionEnvelope, error) {
	var out WorkflowExecutionEnvelope
	err := s.workflowRunAction(ctx, workflowID, executionID, "pause", nil, &out)
	return &out, err
}

// ResumeRun resumes one workflow execution.
func (s *WorkflowService) ResumeRun(ctx context.Context, workflowID, executionID string) (*WorkflowExecutionEnvelope, error) {
	var out WorkflowExecutionEnvelope
	err := s.workflowRunAction(ctx, workflowID, executionID, "resume", nil, &out)
	return &out, err
}

// GetRunResult returns one workflow execution result.
func (s *WorkflowService) GetRunResult(ctx context.Context, workflowID, executionID string) (*WorkflowExecutionResultEnvelope, error) {
	if workflowID == "" {
		return nil, ErrWorkflowIDRequired
	}
	if executionID == "" {
		return nil, ErrExecutionIDRequired
	}
	workflowPath, executionPath, err := workflowRunPath(workflowID, executionID)
	if err != nil {
		return nil, err
	}
	var out WorkflowExecutionResultEnvelope
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/"+workflowPath+"/executions/"+executionPath+"/result", nil, &out)
	return &out, err
}

// GetRunResultByID returns one workflow execution result without requiring a workflow id.
func (s *WorkflowService) GetRunResultByID(ctx context.Context, executionID string) (*WorkflowExecutionResultEnvelope, error) {
	if executionID == "" {
		return nil, ErrExecutionIDRequired
	}
	id, err := pathID(executionID)
	if err != nil {
		return nil, err
	}
	var out WorkflowExecutionResultEnvelope
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/executions/"+id+"/result", nil, &out)
	return &out, err
}

// GetRunImportContextByID returns import context without requiring a workflow id.
func (s *WorkflowService) GetRunImportContextByID(ctx context.Context, executionID string) (*WorkflowImportContextEnvelope, error) {
	id, err := pathID(executionID)
	if err != nil {
		return nil, err
	}
	var out WorkflowImportContextEnvelope
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/executions/"+id+"/import-context", nil, &out)
	return &out, err
}

// ListRunImportFilesByID lists import files without requiring a workflow id.
func (s *WorkflowService) ListRunImportFilesByID(ctx context.Context, executionID string, opts ...QueryOption) (*WorkflowImportFileListEnvelope, error) {
	id, err := pathID(executionID)
	if err != nil {
		return nil, err
	}
	var out WorkflowImportFileListEnvelope
	err = s.do(ctx, http.MethodGet, withQuery("/workflow/v2/workflow-apps/executions/"+id+"/import-files", opts), nil, &out)
	return &out, err
}

// GetRunNodeByID returns one node detail without requiring a workflow id.
func (s *WorkflowService) GetRunNodeByID(ctx context.Context, executionID, nodeKey string) (*WorkflowExecutionNodeDetailEnvelope, error) {
	executionPath, err := pathID(executionID)
	if err != nil {
		return nil, err
	}
	nodePath, err := pathID(nodeKey)
	if err != nil {
		return nil, err
	}
	var out WorkflowExecutionNodeDetailEnvelope
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/executions/"+executionPath+"/nodes/"+nodePath, nil, &out)
	return &out, err
}

// GetRunImportContext returns import context for one workflow execution.
func (s *WorkflowService) GetRunImportContext(ctx context.Context, workflowID, executionID string) (*WorkflowImportContextEnvelope, error) {
	workflowPath, executionPath, err := workflowRunPath(workflowID, executionID)
	if err != nil {
		return nil, err
	}
	var out WorkflowImportContextEnvelope
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/"+workflowPath+"/executions/"+executionPath+"/import-context", nil, &out)
	return &out, err
}

// ListRunImportFiles lists import files for one workflow execution.
func (s *WorkflowService) ListRunImportFiles(ctx context.Context, workflowID, executionID string, opts ...QueryOption) (*WorkflowImportFileListEnvelope, error) {
	workflowPath, executionPath, err := workflowRunPath(workflowID, executionID)
	if err != nil {
		return nil, err
	}
	var out WorkflowImportFileListEnvelope
	err = s.do(ctx, http.MethodGet, withQuery("/workflow/v2/workflow-apps/"+workflowPath+"/executions/"+executionPath+"/import-files", opts), nil, &out)
	return &out, err
}

// GetRunNode returns one node detail from a workflow execution.
func (s *WorkflowService) GetRunNode(ctx context.Context, workflowID, executionID, nodeKey string) (*WorkflowExecutionNodeDetailEnvelope, error) {
	workflowPath, executionPath, nodePath, err := workflowRunNodePath(workflowID, executionID, nodeKey)
	if err != nil {
		return nil, err
	}
	var out WorkflowExecutionNodeDetailEnvelope
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/"+workflowPath+"/executions/"+executionPath+"/nodes/"+nodePath, nil, &out)
	return &out, err
}

// GetRunNodeRerunPlan returns rerun plan for one execution node.
func (s *WorkflowService) GetRunNodeRerunPlan(ctx context.Context, workflowID, executionID, nodeKey string, opts ...QueryOption) (*WorkflowRerunPlanEnvelope, error) {
	workflowPath, executionPath, nodePath, err := workflowRunNodePath(workflowID, executionID, nodeKey)
	if err != nil {
		return nil, err
	}
	var out WorkflowRerunPlanEnvelope
	err = s.do(ctx, http.MethodGet, withQuery("/workflow/v2/workflow-apps/"+workflowPath+"/executions/"+executionPath+"/nodes/"+nodePath+"/rerun-plan", opts), nil, &out)
	return &out, err
}

// CreateRunNodeRerun creates a rerun for one execution node.
func (s *WorkflowService) CreateRunNodeRerun(ctx context.Context, workflowID, executionID, nodeKey, mode, sourceNodeExecutionID string, opts ...WorkflowRerunOption) (*WorkflowRerunEnvelope, error) {
	if mode == "" {
		return nil, ErrModeRequired
	}
	if sourceNodeExecutionID == "" {
		return nil, ErrSourceNodeRequired
	}
	options := workflowRerunOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	if options.expectedRerunContractHash == nil || strings.TrimSpace(*options.expectedRerunContractHash) == "" {
		return nil, ErrRerunContractHashRequired
	}
	workflowPath, executionPath, nodePath, err := workflowRunNodePath(workflowID, executionID, nodeKey)
	if err != nil {
		return nil, err
	}
	var out WorkflowRerunEnvelope
	err = s.do(ctx, http.MethodPost, "/workflow/v2/workflow-apps/"+workflowPath+"/executions/"+executionPath+"/nodes/"+nodePath+"/reruns", workflowNodeRerunBody(mode, sourceNodeExecutionID, opts...), &out)
	return &out, err
}

// ListRunRerunBranches lists rerun branches for one workflow execution.
func (s *WorkflowService) ListRunRerunBranches(ctx context.Context, workflowID, executionID string) (*WorkflowRerunBranchListEnvelope, error) {
	workflowPath, executionPath, err := workflowRunPath(workflowID, executionID)
	if err != nil {
		return nil, err
	}
	var out WorkflowRerunBranchListEnvelope
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workflow-apps/"+workflowPath+"/executions/"+executionPath+"/rerun-branches", nil, &out)
	return &out, err
}

// GetRunRerun returns one rerun execution.
func (s *WorkflowService) GetRunRerun(ctx context.Context, workflowID, executionID, rerunID string) (*WorkflowRerunEnvelope, error) {
	path, err := workflowRerunPath(workflowID, executionID, rerunID)
	if err != nil {
		return nil, err
	}
	var out WorkflowRerunEnvelope
	err = s.do(ctx, http.MethodGet, path, nil, &out)
	return &out, err
}

// CancelRunRerun cancels one rerun execution.
func (s *WorkflowService) CancelRunRerun(ctx context.Context, workflowID, executionID, rerunID string, opts ...WorkflowCancelOption) (*WorkflowRerunEnvelope, error) {
	path, err := workflowRerunPath(workflowID, executionID, rerunID)
	if err != nil {
		return nil, err
	}
	var out WorkflowRerunEnvelope
	err = s.do(ctx, http.MethodPost, path+"/cancel", workflowCancelBody(opts...), &out)
	return &out, err
}

// RetryRun retries one workflow execution.
func (s *WorkflowService) RetryRun(ctx context.Context, workflowID, executionID string, opts ...WorkflowRerunOption) (*WorkflowRunEnvelope, error) {
	options := workflowRerunOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	if options.reason != nil {
		return nil, fmt.Errorf("workflow retry does not support reason; use reason with node rerun instead")
	}
	var out WorkflowRunEnvelope
	err := s.workflowRunAction(ctx, workflowID, executionID, "retry", workflowRetryBody(opts...), &out)
	return &out, err
}

// BatchRetryFailedFiles creates one retry dispatch job for failed file
// executions from a single source dispatch job.
func (s *WorkflowService) BatchRetryFailedFiles(ctx context.Context, workflowID, sourceDispatchJobID string, executionIDs []string, allFailed bool, requestID string) (*WorkflowBatchRetryEnvelope, error) {
	workflowPath, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(sourceDispatchJobID) == "" {
		return nil, fmt.Errorf("source dispatch job id is required")
	}
	if strings.TrimSpace(requestID) == "" {
		return nil, fmt.Errorf("request id is required")
	}
	if allFailed == (len(executionIDs) > 0) {
		return nil, fmt.Errorf("exactly one of execution ids or all failed must be specified")
	}
	body := map[string]any{
		"source_dispatch_job_id": strings.TrimSpace(sourceDispatchJobID),
		"request_id":             strings.TrimSpace(requestID),
	}
	if allFailed {
		body["all_failed"] = true
	} else {
		body["execution_ids"] = append([]string(nil), executionIDs...)
	}
	var out WorkflowBatchRetryEnvelope
	err = s.do(ctx, http.MethodPost, "/workflow/v2/workflow-apps/"+workflowPath+"/executions/batch-retry", body, &out)
	return &out, err
}

// CancelRun cancels one workflow execution.
func (s *WorkflowService) CancelRun(ctx context.Context, workflowID, executionID string, opts ...WorkflowCancelOption) (*WorkflowExecutionEnvelope, error) {
	var out WorkflowExecutionEnvelope
	err := s.workflowRunAction(ctx, workflowID, executionID, "cancel", workflowCancelBody(opts...), &out)
	return &out, err
}

// DeleteRun deletes one workflow execution.
func (s *WorkflowService) DeleteRun(ctx context.Context, workflowID, executionID string) (*WorkflowDeleteEnvelope, error) {
	workflowPath, executionPath, err := workflowRunPath(workflowID, executionID)
	if err != nil {
		return nil, err
	}
	var out WorkflowDeleteEnvelope
	err = s.do(ctx, http.MethodDelete, "/workflow/v2/workflow-apps/"+workflowPath+"/executions/"+executionPath, nil, &out)
	return &out, err
}

// ListFileExecutions lists file-triggered workflow executions.
func (s *WorkflowService) ListFileExecutions(ctx context.Context, opts ...QueryOption) (*WorkflowFileExecutionList, error) {
	var out WorkflowFileExecutionList
	err := s.do(ctx, http.MethodGet, withQuery("/workflow/v2/workflow-apps/file-executions", opts), nil, &out)
	return &out, err
}

// WorkitemCatalog returns the workflow designer workitem catalog.
func (s *WorkflowService) WorkitemCatalog(ctx context.Context) (*WorkitemCatalogList, error) {
	var out WorkitemCatalogList
	err := s.do(ctx, http.MethodGet, "/workflow/v2/workitems/catalog", nil, &out)
	return &out, err
}

// WorkitemCatalogItem returns one workflow designer workitem catalog item.
func (s *WorkflowService) WorkitemCatalogItem(ctx context.Context, nodeID string) (*WorkitemCatalogDetail, error) {
	id, err := pathID(nodeID)
	if err != nil {
		return nil, err
	}
	var out WorkitemCatalogDetail
	err = s.do(ctx, http.MethodGet, "/workflow/v2/workitems/catalog/"+id, nil, &out)
	return &out, err
}

// GetWorkitemAPIService returns the published API service info for one catalog WorkItem version.
func (s *WorkflowService) GetWorkitemAPIService(ctx context.Context, nodeID, version string) (*WorkitemAPIServiceEnvelope, error) {
	id, err := pathID(nodeID)
	if err != nil {
		return nil, err
	}
	if version == "" {
		return nil, ErrVersionRequired
	}
	var out WorkitemAPIServiceEnvelope
	err = s.do(ctx, http.MethodGet, withQuery("/workflow/v2/workitems/catalog/"+id+"/api-service", []QueryOption{WithQueryParam("version", version)}), nil, &out)
	return &out, err
}

// PublishWorkitemAPIService publishes or updates the API service for one catalog WorkItem version.
func (s *WorkflowService) PublishWorkitemAPIService(ctx context.Context, nodeID string, spec WorkitemAPIServicePublishSpec) (*WorkitemAPIServiceEnvelope, error) {
	id, err := pathID(nodeID)
	if err != nil {
		return nil, err
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	var out WorkitemAPIServiceEnvelope
	err = s.do(ctx, http.MethodPost, "/workflow/v2/workitems/catalog/"+id+"/api-service", workitemAPIServicePublishBody(spec), &out)
	return &out, err
}

// InvokeWorkitemAPIService invokes one published catalog WorkItem API service.
func (s *WorkflowService) InvokeWorkitemAPIService(ctx context.Context, nodeID string, spec WorkitemAPIServiceInvokeSpec) (*WorkitemAPIServiceInvokeEnvelope, error) {
	id, err := pathID(nodeID)
	if err != nil {
		return nil, err
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	var out WorkitemAPIServiceInvokeEnvelope
	err = s.do(ctx, http.MethodPost, "/workflow/v2/workitems/catalog/"+id+"/api-service/invoke", workitemAPIServiceInvokeBody(spec), &out)
	return &out, err
}

// InvokeDynamicService invokes one workspace dynamic service.
func (s *WorkflowService) InvokeDynamicService(ctx context.Context, spec DynamicServiceInvokeSpec) (*DynamicServiceInvokeResult, error) {
	workspacePath, err := pathID(s.workspaceID)
	if err != nil {
		return nil, err
	}
	return s.invokeDynamicService(ctx, "/workspaces/"+workspacePath+"/dynamic-services/invoke", spec)
}

// InvokeWorkflowDynamicService invokes one workflow dynamic service through the workflow v2 API.
func (s *WorkflowService) InvokeWorkflowDynamicService(ctx context.Context, spec DynamicServiceInvokeSpec) (*DynamicServiceInvokeResult, error) {
	return s.invokeDynamicService(ctx, "/workflow/v2/dynamic-services/invoke", spec)
}

func (s *WorkflowService) invokeDynamicService(ctx context.Context, path string, spec DynamicServiceInvokeSpec) (*DynamicServiceInvokeResult, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	var out DynamicServiceInvokeResult
	if len(spec.Files) > 0 {
		fields := map[string]string{"service_name": spec.ServiceName, "payload": string(spec.Payload)}
		if spec.Version != "" {
			fields["version"] = spec.Version
		}
		if spec.VersionNumber != 0 {
			fields["version_number"] = strconv.FormatInt(int64(spec.VersionNumber), 10)
		}
		if len(spec.Trace) > 0 {
			fields["trace"] = string(spec.Trace)
		}
		body, contentType, err := multipartBody(fields, spec.Files)
		if err != nil {
			return nil, err
		}
		err = s.doRaw(ctx, http.MethodPost, path, body, contentType, contentTypeJSON, &out)
		return &out, err
	}
	err := s.do(ctx, http.MethodPost, path, dynamicServiceInvokeBody(spec), &out)
	return &out, err
}

// EnableWorkitemAPIService enables the API service for one catalog WorkItem version.
func (s *WorkflowService) EnableWorkitemAPIService(ctx context.Context, nodeID, version string) (*WorkitemAPIServiceEnvelope, error) {
	return s.setWorkitemAPIServiceEnabled(ctx, nodeID, version, "enable")
}

// DisableWorkitemAPIService disables the API service for one catalog WorkItem version.
func (s *WorkflowService) DisableWorkitemAPIService(ctx context.Context, nodeID, version string) (*WorkitemAPIServiceEnvelope, error) {
	return s.setWorkitemAPIServiceEnabled(ctx, nodeID, version, "disable")
}

func (s *WorkflowService) setWorkitemAPIServiceEnabled(ctx context.Context, nodeID, version, action string) (*WorkitemAPIServiceEnvelope, error) {
	id, err := pathID(nodeID)
	if err != nil {
		return nil, err
	}
	if version == "" {
		return nil, ErrVersionRequired
	}
	var out WorkitemAPIServiceEnvelope
	err = s.do(ctx, http.MethodPost, withQuery("/workflow/v2/workitems/catalog/"+id+"/api-service/"+action, []QueryOption{WithQueryParam("version", version)}), map[string]any{}, &out)
	return &out, err
}

// CustomOperators lists custom operators visible in the workflow designer.
func (s *WorkflowService) CustomOperators(ctx context.Context, opts ...QueryOption) (*CustomOperatorListEnvelope, error) {
	var out CustomOperatorListEnvelope
	err := s.do(ctx, http.MethodGet, withQuery("/workflow/v2/custom-operators", opts), nil, &out)
	return &out, err
}

// CreateCustomOperator creates a custom operator.
func (s *WorkflowService) CreateCustomOperator(ctx context.Context, spec CustomOperatorSpec, opts ...CustomOperatorOption) (*CustomOperator, error) {
	body, err := spec.Body(opts...)
	if err != nil {
		return nil, err
	}
	var out CustomOperator
	err = s.do(ctx, http.MethodPost, "/workflow/v2/custom-operators", body, &out)
	return &out, err
}

// GetCustomOperator returns one custom operator.
func (s *WorkflowService) GetCustomOperator(ctx context.Context, operatorID string) (*CustomOperator, error) {
	id, err := pathID(operatorID)
	if err != nil {
		return nil, err
	}
	var out CustomOperator
	err = s.do(ctx, http.MethodGet, "/workflow/v2/custom-operators/"+id, nil, &out)
	return &out, err
}

// GetCustomOperatorCode returns one custom operator source payload.
func (s *WorkflowService) GetCustomOperatorCode(ctx context.Context, operatorID string) (*CustomOperatorCodeEnvelope, error) {
	id, err := pathID(operatorID)
	if err != nil {
		return nil, err
	}
	var out CustomOperatorCodeEnvelope
	err = s.do(ctx, http.MethodGet, "/workflow/v2/custom-operators/"+id+"/code", nil, &out)
	return &out, err
}

// GetCustomOperatorSource returns an authoring draft or published source by
// canonical WorkItem identity.
func (s *WorkflowService) GetCustomOperatorSource(ctx context.Context, nodeID, version, taskID string) (*CustomOperatorSourceEnvelope, error) {
	nodeID = strings.TrimSpace(nodeID)
	version = strings.TrimSpace(version)
	if nodeID == "" {
		return nil, ErrIDRequired
	}
	if version == "" {
		return nil, ErrVersionRequired
	}
	opts := []QueryOption{
		WithQueryParam("node_id", nodeID),
		WithQueryParam("version", version),
	}
	if taskID = strings.TrimSpace(taskID); taskID != "" {
		opts = append(opts, WithQueryParam("task_id", taskID))
	}
	var out CustomOperatorSourceEnvelope
	err := s.do(ctx, http.MethodGet, withQuery("/workflow/v2/custom-operators/source", opts), nil, &out)
	return &out, err
}

// TestCustomOperator starts a custom operator test run.
func (s *WorkflowService) TestCustomOperator(ctx context.Context, operatorID string, input CustomOperatorTestInput) (*CustomOperatorTestEnvelope, error) {
	id, err := pathID(operatorID)
	if err != nil {
		return nil, err
	}
	body, err := input.Body()
	if err != nil {
		return nil, err
	}
	var out CustomOperatorTestEnvelope
	err = s.do(ctx, http.MethodPost, "/workflow/v2/custom-operators/"+id+"/test-run", body, &out)
	return &out, err
}

// UpdateCustomOperator updates a custom operator.
func (s *WorkflowService) UpdateCustomOperator(ctx context.Context, operatorID string, opts ...CustomOperatorUpdateOption) (*CustomOperator, error) {
	id, err := pathID(operatorID)
	if err != nil {
		return nil, err
	}
	var out CustomOperator
	err = s.do(ctx, http.MethodPut, "/workflow/v2/custom-operators/"+id, customOperatorUpdateBody(opts...), &out)
	return &out, err
}

// EnableCustomOperator enables a custom operator.
func (s *WorkflowService) EnableCustomOperator(ctx context.Context, operatorID string) (*CustomOperator, error) {
	return s.customOperatorAction(ctx, operatorID, "enable")
}

// DisableCustomOperator disables a custom operator.
func (s *WorkflowService) DisableCustomOperator(ctx context.Context, operatorID string) (*CustomOperator, error) {
	return s.customOperatorAction(ctx, operatorID, "disable")
}

// DeleteCustomOperator deletes a custom operator.
func (s *WorkflowService) DeleteCustomOperator(ctx context.Context, operatorID string) (*WorkflowDeleteEnvelope, error) {
	id, err := pathID(operatorID)
	if err != nil {
		return nil, err
	}
	var out WorkflowDeleteEnvelope
	err = s.do(ctx, http.MethodDelete, "/workflow/v2/custom-operators/"+id, nil, &out)
	return &out, err
}

// CatalogFileLineageOverview returns lineage overview for a catalog file artifact.
func (s *WorkflowService) CatalogFileLineageOverview(ctx context.Context, fileID string, opts ...QueryOption) (*LineageOverview, error) {
	id, err := pathID(fileID)
	if err != nil {
		return nil, err
	}
	var out LineageOverview
	err = s.do(ctx, http.MethodGet, withQuery("/lineage/catalog-files/"+id+"/overview", opts), nil, &out)
	return &out, err
}

// ArtifactLineageOverview returns lineage overview for an artifact.
func (s *WorkflowService) ArtifactLineageOverview(ctx context.Context, artifactID string) (*LineageOverview, error) {
	id, err := pathID(artifactID)
	if err != nil {
		return nil, err
	}
	var out LineageOverview
	err = s.do(ctx, http.MethodGet, "/lineage/artifacts/"+id+"/overview", nil, &out)
	return &out, err
}

// ArtifactOutputBlocks lists artifact output blocks.
func (s *WorkflowService) ArtifactOutputBlocks(ctx context.Context, artifactID string) (*LineageOutputBlocks, error) {
	id, err := pathID(artifactID)
	if err != nil {
		return nil, err
	}
	var out LineageOutputBlocks
	err = s.do(ctx, http.MethodGet, "/lineage/artifacts/"+id+"/output-blocks", nil, &out)
	return &out, err
}

// ArtifactBlockRevisions lists revisions for an artifact output block.
func (s *WorkflowService) ArtifactBlockRevisions(ctx context.Context, artifactID, blockID string) (*LineageBlockRevisions, error) {
	artifactPath, blockPath, err := artifactBlockPath(artifactID, blockID)
	if err != nil {
		return nil, err
	}
	var out LineageBlockRevisions
	err = s.do(ctx, http.MethodGet, "/lineage/artifacts/"+artifactPath+"/blocks/"+blockPath+"/revisions", nil, &out)
	return &out, err
}

// CreateArtifactBlockRevision creates a revision for an artifact output block.
func (s *WorkflowService) CreateArtifactBlockRevision(ctx context.Context, artifactID, blockID string, opts ...LineageRevisionOption) (*LineageBlockRevision, error) {
	artifactPath, blockPath, err := artifactBlockPath(artifactID, blockID)
	if err != nil {
		return nil, err
	}
	var out LineageBlockRevision
	err = s.do(ctx, http.MethodPost, "/lineage/artifacts/"+artifactPath+"/blocks/"+blockPath+"/revisions", lineageRevisionBody(opts...), &out)
	return &out, err
}

// SwitchArtifactEffectiveRevisions switches effective revisions for an artifact.
func (s *WorkflowService) SwitchArtifactEffectiveRevisions(ctx context.Context, artifactID string, switches []LineageRevisionSwitch, opts ...LineageSwitchOption) (*LineageEffectiveRevisions, error) {
	id, err := pathID(artifactID)
	if err != nil {
		return nil, err
	}
	var out LineageEffectiveRevisions
	err = s.do(ctx, http.MethodPost, "/lineage/artifacts/"+id+"/effective-revisions", lineageSwitchBody(switches, opts...), &out)
	return &out, err
}

// ArtifactNode returns one artifact lineage node.
func (s *WorkflowService) ArtifactNode(ctx context.Context, artifactID, nodeID string) (*LineageArtifactNode, error) {
	artifactPath, nodePath, err := artifactNodePath(artifactID, nodeID)
	if err != nil {
		return nil, err
	}
	var out LineageArtifactNode
	err = s.do(ctx, http.MethodGet, "/lineage/artifacts/"+artifactPath+"/nodes/"+nodePath, nil, &out)
	return &out, err
}

// ArtifactNodeOutputBlocks lists output blocks for an artifact node.
func (s *WorkflowService) ArtifactNodeOutputBlocks(ctx context.Context, artifactID, nodeID string) (*LineageOutputBlocks, error) {
	artifactPath, nodePath, err := artifactNodePath(artifactID, nodeID)
	if err != nil {
		return nil, err
	}
	var out LineageOutputBlocks
	err = s.do(ctx, http.MethodGet, "/lineage/artifacts/"+artifactPath+"/nodes/"+nodePath+"/output-blocks", nil, &out)
	return &out, err
}

// ArtifactNodeBlockRevisions lists revisions for a node output block.
func (s *WorkflowService) ArtifactNodeBlockRevisions(ctx context.Context, artifactID, nodeID, blockID string) (*LineageBlockRevisions, error) {
	artifactPath, nodePath, blockPath, err := artifactNodeBlockPath(artifactID, nodeID, blockID)
	if err != nil {
		return nil, err
	}
	var out LineageBlockRevisions
	err = s.do(ctx, http.MethodGet, "/lineage/artifacts/"+artifactPath+"/nodes/"+nodePath+"/blocks/"+blockPath+"/revisions", nil, &out)
	return &out, err
}

// CreateArtifactNodeBlockRevision creates a revision for a node output block.
func (s *WorkflowService) CreateArtifactNodeBlockRevision(ctx context.Context, artifactID, nodeID, blockID string, opts ...LineageRevisionOption) (*LineageBlockRevision, error) {
	artifactPath, nodePath, blockPath, err := artifactNodeBlockPath(artifactID, nodeID, blockID)
	if err != nil {
		return nil, err
	}
	var out LineageBlockRevision
	err = s.do(ctx, http.MethodPost, "/lineage/artifacts/"+artifactPath+"/nodes/"+nodePath+"/blocks/"+blockPath+"/revisions", lineageRevisionBody(opts...), &out)
	return &out, err
}

// SwitchArtifactNodeEffectiveRevisions switches effective revisions for a node.
func (s *WorkflowService) SwitchArtifactNodeEffectiveRevisions(ctx context.Context, artifactID, nodeID string, switches []LineageRevisionSwitch, opts ...LineageSwitchOption) (*LineageEffectiveRevisions, error) {
	artifactPath, nodePath, err := artifactNodePath(artifactID, nodeID)
	if err != nil {
		return nil, err
	}
	var out LineageEffectiveRevisions
	err = s.do(ctx, http.MethodPost, "/lineage/artifacts/"+artifactPath+"/nodes/"+nodePath+"/effective-revisions", lineageSwitchBody(switches, opts...), &out)
	return &out, err
}

// CreateArtifactNodeRerun reruns one artifact lineage node.
func (s *WorkflowService) CreateArtifactNodeRerun(ctx context.Context, artifactID, nodeID string, opts ...LineageRerunOption) (*LineageRerun, error) {
	artifactPath, nodePath, err := artifactNodePath(artifactID, nodeID)
	if err != nil {
		return nil, err
	}
	var out LineageRerun
	err = s.do(ctx, http.MethodPost, "/lineage/artifacts/"+artifactPath+"/nodes/"+nodePath+"/rerun", lineageRerunBody(opts...), &out)
	return &out, err
}

// CreateTargetArtifactNodeRevisionRerun reruns a node for one target output emission.
func (s *WorkflowService) CreateTargetArtifactNodeRevisionRerun(ctx context.Context, artifactID, nodeID, targetOutputEmissionID string, opts ...LineageRevisionRerunOption) (*LineageRerun, error) {
	artifactPath, nodePath, err := artifactNodePath(artifactID, nodeID)
	if err != nil {
		return nil, err
	}
	targetPath, err := pathID(targetOutputEmissionID)
	if err != nil {
		return nil, err
	}
	var out LineageRerun
	err = s.do(ctx, http.MethodPost, "/lineage/artifacts/"+artifactPath+"/nodes/"+nodePath+"/output-emissions/"+targetPath+"/rerun-with-revisions", lineageRevisionRerunBody(opts...), &out)
	return &out, err
}

// WorkflowTemplates lists workflow templates.
func (s *WorkflowService) WorkflowTemplates(ctx context.Context) (*WorkflowTemplateList, error) {
	var out WorkflowTemplateList
	err := s.do(ctx, http.MethodGet, "/workflow-templates", nil, &out)
	return &out, err
}

// GetWorkflowTemplate returns one workflow template.
func (s *WorkflowService) GetWorkflowTemplate(ctx context.Context, templateID string) (*WorkflowTemplate, error) {
	id, err := pathID(templateID)
	if err != nil {
		return nil, err
	}
	var out WorkflowTemplate
	err = s.do(ctx, http.MethodGet, "/workflow-templates/"+id, nil, &out)
	return &out, err
}

// CreateWorkflowTemplate creates a workflow template.
func (s *WorkflowService) CreateWorkflowTemplate(ctx context.Context, name, dslYAML string, opts ...WorkflowTemplateOption) (*WorkflowTemplate, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	if dslYAML == "" {
		return nil, ErrDSLYAMLRequired
	}
	var out WorkflowTemplate
	err := s.do(ctx, http.MethodPost, "/workflow-templates", workflowTemplateBody(name, dslYAML, opts...), &out)
	return &out, err
}

// CreateMOWLWorkflowTemplate saves a caller-owned MOWL publication graph as a
// workflow template. The graph must reference published workspace artifacts;
// the backend validates custom operator references before persisting the
// template so instantiation cannot produce a dangling workflow.
func (s *WorkflowService) CreateMOWLWorkflowTemplate(ctx context.Context, name string, workflow map[string]any, opts ...WorkflowTemplateOption) (*WorkflowTemplate, error) {
	body, err := workflowMOWLTemplateBody(name, workflow, opts...)
	if err != nil {
		return nil, err
	}
	var out WorkflowTemplate
	err = s.do(ctx, http.MethodPost, "/workflow-templates", body, &out)
	return &out, err
}

// UpdateMOWLWorkflowTemplate replaces a template's MOWL publication graph. The
// server validates referenced operators and preserves template type semantics.
func (s *WorkflowService) UpdateMOWLWorkflowTemplate(ctx context.Context, templateID, name string, workflow map[string]any, opts ...WorkflowTemplateOption) (*WorkflowTemplate, error) {
	body, err := workflowMOWLTemplateBody(name, workflow, opts...)
	if err != nil {
		return nil, err
	}
	id, err := pathID(templateID)
	if err != nil {
		return nil, err
	}
	var out WorkflowTemplate
	err = s.do(ctx, http.MethodPut, "/workflow-templates/"+id, body, &out)
	return &out, err
}

// UpdateWorkflowTemplate updates a workflow template.
func (s *WorkflowService) UpdateWorkflowTemplate(ctx context.Context, templateID, name, dslYAML string, opts ...WorkflowTemplateOption) (*WorkflowTemplate, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	if dslYAML == "" {
		return nil, ErrDSLYAMLRequired
	}
	id, err := pathID(templateID)
	if err != nil {
		return nil, err
	}
	var out WorkflowTemplate
	err = s.do(ctx, http.MethodPut, "/workflow-templates/"+id, workflowTemplateBody(name, dslYAML, opts...), &out)
	return &out, err
}

// DeleteWorkflowTemplate deletes a workflow template.
func (s *WorkflowService) DeleteWorkflowTemplate(ctx context.Context, templateID string) (*WorkflowTemplateDeleteEnvelope, error) {
	id, err := pathID(templateID)
	if err != nil {
		return nil, err
	}
	var out WorkflowTemplateDeleteEnvelope
	err = s.do(ctx, http.MethodDelete, "/workflow-templates/"+id, nil, &out)
	return &out, err
}

// List returns UI-visible knowledge bases.
func (s *KnowledgeService) List(ctx context.Context, opts ...ListOption) (*SemanticModelListResult, error) {
	var out SemanticModelListResult
	err := s.do(ctx, http.MethodGet, withListQuery("/semantic-models", opts), nil, &out)
	return &out, err
}

// ListTags lists aggregated semantic model tags.
func (s *KnowledgeService) ListTags(ctx context.Context, opts ...QueryOption) (*SemanticModelTagListResult, error) {
	var out SemanticModelTagListResult
	err := s.do(ctx, http.MethodGet, withQuery("/semantic-models/tags", opts), nil, &out)
	return &out, err
}

// Create creates a knowledge base. The name is also its immutable Catalog database name.
func (s *KnowledgeService) Create(ctx context.Context, name string, opts ...SemanticModelOption) (*SemanticModelInfo, error) {
	return s.CreateWithSpec(ctx, SemanticModelCreateSpec{Name: name}, opts...)
}

// CreateWithSpec creates a knowledge base with embedding and drawing controls.
// Omitted controls use server behavior; an explicit false drawing flag still
// expresses drawing intent. The server validates configurations and policies.
func (s *KnowledgeService) CreateWithSpec(ctx context.Context, spec SemanticModelCreateSpec, opts ...SemanticModelOption) (*SemanticModelInfo, error) {
	if spec.Name == "" {
		return nil, ErrNameRequired
	}
	var out SemanticModelInfo
	err := s.do(ctx, http.MethodPost, "/semantic-models", semanticModelCreateBody(spec, opts...), &out)
	return &out, err
}

// Get returns one UI-visible semantic model.
func (s *KnowledgeService) Get(ctx context.Context, modelID string) (*SemanticModelInfo, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticModelInfo
	err = s.do(ctx, http.MethodGet, "/semantic-models/"+id, nil, &out)
	return &out, err
}

// ListReferencingAgents returns visible Agents bound to a knowledge base.
func (s *KnowledgeService) ListReferencingAgents(ctx context.Context, modelID string, limit int, cursor string) (*KnowledgeReferencingAgentsPage, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	options := make([]QueryOption, 0, 2)
	if limit > 0 {
		options = append(options, WithLimit(limit))
	}
	if cursor != "" {
		options = append(options, WithQueryParam("cursor", cursor))
	}
	var out KnowledgeReferencingAgentsPage
	err = s.do(ctx, http.MethodGet, withQuery("/semantic-models/"+id+"/referencing-agents", options), nil, &out)
	return &out, err
}

// Update updates one knowledge base. name must equal its existing immutable name.
func (s *KnowledgeService) Update(ctx context.Context, modelID, name string, opts ...SemanticModelOption) (*SemanticMutationResult, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticMutationResult
	err = s.do(ctx, http.MethodPut, "/semantic-models/"+id, semanticModelBody(name, opts...), &out)
	return &out, err
}

// Delete deletes one UI-visible semantic model.
func (s *KnowledgeService) Delete(ctx context.Context, modelID string) (*SemanticMutationResult, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticMutationResult
	err = s.do(ctx, http.MethodDelete, "/semantic-models/"+id, nil, &out)
	return &out, err
}

// CreateWithSources creates a knowledge base with initial knowledge sources.
// The name is also its immutable Catalog database name.
func (s *KnowledgeService) CreateWithSources(ctx context.Context, name string, sources []SemanticModelSourceInput, opts ...SemanticModelWithSourcesOption) (*SemanticModelWithSourcesResult, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	var out SemanticModelWithSourcesResult
	err := s.do(ctx, http.MethodPost, "/semantic-models/create-with-sources", semanticModelWithSourcesBody(name, sources, opts...), &out)
	return &out, err
}

// CreateEmpty creates a data-side knowledge base without initial sources or workflows.
func (s *KnowledgeService) CreateEmpty(ctx context.Context, name string, opts ...SemanticModelEmptyOption) (*SemanticModelEmptyResult, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	var out SemanticModelEmptyResult
	err := s.do(ctx, http.MethodPost, "/semantic-models/create-empty", semanticModelEmptyBody(name, opts...), &out)
	return &out, err
}

// UploadLocalFile uploads a local knowledge file before creating a semantic model.
func (s *KnowledgeService) UploadLocalFile(ctx context.Context, filename string, reader io.Reader) (*UploadFileResult, error) {
	buf, contentType, err := catalogUploadMultipart(filename, reader)
	if err != nil {
		return nil, err
	}
	var out UploadFileResult
	err = s.doRaw(ctx, http.MethodPost, "/semantic-models/local-files/upload", buf, contentType, contentTypeJSON, &out)
	return &out, err
}

// UploadModelLocalFile uploads a local knowledge file for an existing semantic model.
func (s *KnowledgeService) UploadModelLocalFile(ctx context.Context, modelID, filename string, reader io.Reader) (*UploadFileResult, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	buf, contentType, err := catalogUploadMultipart(filename, reader)
	if err != nil {
		return nil, err
	}
	var out UploadFileResult
	err = s.doRaw(ctx, http.MethodPost, "/semantic-models/"+id+"/local-files/upload", buf, contentType, contentTypeJSON, &out)
	return &out, err
}

// PreviewSourceSelectionCounts returns authoritative deduplicated counts before creating a semantic model.
func (s *KnowledgeService) PreviewSourceSelectionCounts(ctx context.Context, selections []SemanticModelSourceSelectionInput) (*SemanticModelSourceSelectionPreviewResult, error) {
	var out SemanticModelSourceSelectionPreviewResult
	err := s.do(ctx, http.MethodPost, "/semantic-models/source-selections/preview", map[string]any{
		"source_selections": semanticModelSourceSelectionsBody(selections),
	}, &out)
	return &out, err
}

// PreviewModelSourceSelectionCounts returns authoritative deduplicated counts before appending to a semantic model.
func (s *KnowledgeService) PreviewModelSourceSelectionCounts(ctx context.Context, modelID string, selections []SemanticModelSourceSelectionInput) (*SemanticModelSourceSelectionPreviewResult, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticModelSourceSelectionPreviewResult
	err = s.do(ctx, http.MethodPost, "/semantic-models/"+id+"/source-selections/preview", map[string]any{
		"source_selections": semanticModelSourceSelectionsBody(selections),
	}, &out)
	return &out, err
}

// ListSources lists source rows for one semantic model.
func (s *KnowledgeService) ListSources(ctx context.Context, modelID string, opts ...QueryOption) (*SemanticModelSourceListResult, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticModelSourceListResult
	err = s.do(ctx, http.MethodGet, withQuery("/semantic-models/"+id+"/sources", opts), nil, &out)
	return &out, err
}

// ListSourcesPage lists one page of source rows for one semantic model.
func (s *KnowledgeService) ListSourcesPage(ctx context.Context, modelID string, page, pageSize int, opts ...QueryOption) (*SemanticModelSourceListResult, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticModelSourceListResult
	options := []QueryOption{WithPage(page), WithPageSize(pageSize)}
	options = append(options, opts...)
	path := withQuery("/semantic-models/"+id+"/sources", options)
	err = s.do(ctx, http.MethodGet, path, nil, &out)
	return &out, err
}

// CheckSourceExistence returns requested source IDs already present in one semantic model.
func (s *KnowledgeService) CheckSourceExistence(ctx context.Context, modelID string, fileIDs []string, tableIDs []int64) (*SemanticModelSourceExistenceResult, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticModelSourceExistenceResult
	err = s.do(ctx, http.MethodPost, "/semantic-models/"+id+"/sources/existence", map[string]any{
		"file_ids":  fileIDs,
		"table_ids": tableIDs,
	}, &out)
	return &out, err
}

// AppendSources appends knowledge sources to one semantic model.
func (s *KnowledgeService) AppendSources(ctx context.Context, modelID string, sources []SemanticModelSourceInput, opts ...SemanticModelAppendSourcesOption) (*SemanticModelAppendSourcesResult, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticModelAppendSourcesResult
	var options semanticModelAppendSourcesOptions
	for _, option := range opts {
		option(&options)
	}
	body := map[string]any{"sources": semanticModelSourcesBody(sources)}
	if options.selections != nil {
		body["source_selections"] = semanticModelSourceSelectionsBody(options.selections)
	}
	err = s.do(ctx, http.MethodPost, "/semantic-models/"+id+"/sources", body, &out)
	return &out, err
}

// BackfillLegacySources creates source rows for legacy semantic model lineage.
func (s *KnowledgeService) BackfillLegacySources(ctx context.Context, modelID string) (*SemanticMutationResult, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticMutationResult
	err = s.do(ctx, http.MethodPost, "/semantic-models/"+id+"/sources/backfill-legacy", nil, &out)
	return &out, err
}

// DeleteSource removes one knowledge source from a semantic model.
func (s *KnowledgeService) DeleteSource(ctx context.Context, modelID, sourceID string) (*SemanticMutationResult, error) {
	modelPath, sourcePath, err := semanticModelSourcePath(modelID, sourceID)
	if err != nil {
		return nil, err
	}
	var out SemanticMutationResult
	err = s.do(ctx, http.MethodDelete, "/semantic-models/"+modelPath+"/sources/"+sourcePath, nil, &out)
	return &out, err
}

// GetSourceDocument returns document and segment detail for one knowledge source.
func (s *KnowledgeService) GetSourceDocument(ctx context.Context, modelID, sourceID string, opts ...QueryOption) (*SemanticModelSourceDocument, error) {
	modelPath, sourcePath, err := semanticModelSourcePath(modelID, sourceID)
	if err != nil {
		return nil, err
	}
	var out SemanticModelSourceDocument
	err = s.do(ctx, http.MethodGet, withQuery("/semantic-models/"+modelPath+"/sources/"+sourcePath+"/document", opts), nil, &out)
	return &out, err
}

// UpdateSourceGovernance updates governance fields for one knowledge source.
func (s *KnowledgeService) UpdateSourceGovernance(ctx context.Context, modelID, sourceID string, opts ...SemanticModelSourceGovernanceOption) (*SemanticModelSourceGovernanceResult, error) {
	modelPath, sourcePath, err := semanticModelSourcePath(modelID, sourceID)
	if err != nil {
		return nil, err
	}
	var out SemanticModelSourceGovernanceResult
	err = s.do(ctx, http.MethodPatch, "/semantic-models/"+modelPath+"/sources/"+sourcePath+"/governance", semanticModelSourceGovernanceBody(opts...), &out)
	return &out, err
}

// UpdateSegment edits one segment and commits a new segment version.
func (s *KnowledgeService) UpdateSegment(ctx context.Context, modelID, sourceID, segmentID string, input SemanticModelSegmentUpdateInput) (*SemanticModelSegmentMutationResult, error) {
	modelPath, sourcePath, segmentPath, err := semanticModelSegmentPath(modelID, sourceID, segmentID)
	if err != nil {
		return nil, err
	}
	var out SemanticModelSegmentMutationResult
	err = s.do(ctx, http.MethodPatch, "/semantic-models/"+modelPath+"/sources/"+sourcePath+"/segments/"+segmentPath, semanticModelSegmentUpdateBody(input), &out)
	return &out, err
}

// DeleteSegment removes one segment and commits a new segment version.
func (s *KnowledgeService) DeleteSegment(ctx context.Context, modelID, sourceID, segmentID string, base SemanticModelSegmentBase) (*SemanticModelSegmentMutationResult, error) {
	modelPath, sourcePath, segmentPath, err := semanticModelSegmentPath(modelID, sourceID, segmentID)
	if err != nil {
		return nil, err
	}
	var out SemanticModelSegmentMutationResult
	err = s.do(ctx, http.MethodDelete, "/semantic-models/"+modelPath+"/sources/"+sourcePath+"/segments/"+segmentPath, semanticModelSegmentBaseBody(base), &out)
	return &out, err
}

// CreateSegment appends one segment and commits a new segment version.
func (s *KnowledgeService) CreateSegment(ctx context.Context, modelID, sourceID string, input SemanticModelSegmentCreateInput) (*SemanticModelSegmentMutationResult, error) {
	modelPath, sourcePath, err := semanticModelSourcePath(modelID, sourceID)
	if err != nil {
		return nil, err
	}
	var out SemanticModelSegmentMutationResult
	err = s.do(ctx, http.MethodPost, "/semantic-models/"+modelPath+"/sources/"+sourcePath+"/segments", semanticModelSegmentCreateBody(input), &out)
	return &out, err
}

// UpdateSegmentEnabled toggles one segment and commits a new segment version.
func (s *KnowledgeService) UpdateSegmentEnabled(ctx context.Context, modelID, sourceID, segmentID string, base SemanticModelSegmentBase, enabled bool) (*SemanticModelSegmentMutationResult, error) {
	modelPath, sourcePath, segmentPath, err := semanticModelSegmentPath(modelID, sourceID, segmentID)
	if err != nil {
		return nil, err
	}
	body := semanticModelSegmentBaseBody(base)
	body["enabled"] = enabled
	var out SemanticModelSegmentMutationResult
	err = s.do(ctx, http.MethodPatch, "/semantic-models/"+modelPath+"/sources/"+sourcePath+"/segments/"+segmentPath+"/enabled", body, &out)
	return &out, err
}

// ReembedSegments rematerializes the current segment version.
func (s *KnowledgeService) ReembedSegments(ctx context.Context, modelID, sourceID string, base SemanticModelSegmentBase) (*SemanticModelSegmentMutationResult, error) {
	modelPath, sourcePath, err := semanticModelSourcePath(modelID, sourceID)
	if err != nil {
		return nil, err
	}
	var out SemanticModelSegmentMutationResult
	err = s.do(ctx, http.MethodPost, "/semantic-models/"+modelPath+"/sources/"+sourcePath+"/segments/re-embedding", semanticModelSegmentBaseBody(base), &out)
	return &out, err
}

// ListSourceJobs lists source jobs for one semantic model.
func (s *KnowledgeService) ListSourceJobs(ctx context.Context, modelID string) (*SemanticModelSourceJobListResult, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticModelSourceJobListResult
	err = s.do(ctx, http.MethodGet, "/semantic-models/"+id+"/source-jobs", nil, &out)
	return &out, err
}

// ReconcileSourceJobs requests a retry for the source selected by WithRetrySourceRowID.
// With no source selected, it does not advance or retry source jobs.
func (s *KnowledgeService) ReconcileSourceJobs(ctx context.Context, modelID string, opts ...SourceJobReconcileOption) (*SemanticMutationResult, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticMutationResult
	options := sourceJobReconcileOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	var body any
	if options.retrySourceRowID != "" {
		body = map[string]any{"retry_source_row_id": options.retrySourceRowID}
	}
	err = s.do(ctx, http.MethodPost, "/semantic-models/"+id+"/source-jobs/reconcile", body, &out)
	return &out, err
}

// ListEntries lists entries for one UI-visible semantic model.
func (s *KnowledgeService) ListEntries(ctx context.Context, modelID string, opts ...QueryOption) (*SemanticEntryListResult, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticEntryListResult
	err = s.do(ctx, http.MethodGet, withQuery("/semantic-models/"+id+"/entries", opts), nil, &out)
	return &out, err
}

// CreateEntry creates one semantic model entry.
func (s *KnowledgeService) CreateEntry(ctx context.Context, modelID string, entry SemanticEntryInput) (*SemanticEntry, error) {
	if err := validateSemanticEntry(entry); err != nil {
		return nil, err
	}
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticEntry
	err = s.do(ctx, http.MethodPost, "/semantic-models/"+id+"/entries", semanticEntryBody(entry), &out)
	return &out, err
}

// UpdateEntry updates one semantic model entry.
func (s *KnowledgeService) UpdateEntry(ctx context.Context, modelID, entryID string, entry SemanticEntryInput) (*SemanticMutationResult, error) {
	if err := validateSemanticEntry(entry); err != nil {
		return nil, err
	}
	modelPath, entryPath, err := semanticEntryPath(modelID, entryID)
	if err != nil {
		return nil, err
	}
	var out SemanticMutationResult
	err = s.do(ctx, http.MethodPut, "/semantic-models/"+modelPath+"/entries/"+entryPath, semanticEntryBody(entry), &out)
	return &out, err
}

// DeleteEntry deletes one semantic model entry.
func (s *KnowledgeService) DeleteEntry(ctx context.Context, modelID, entryID string) (*SemanticMutationResult, error) {
	modelPath, entryPath, err := semanticEntryPath(modelID, entryID)
	if err != nil {
		return nil, err
	}
	var out SemanticMutationResult
	err = s.do(ctx, http.MethodDelete, "/semantic-models/"+modelPath+"/entries/"+entryPath, nil, &out)
	return &out, err
}

// Import imports semantic model entries.
func (s *KnowledgeService) Import(ctx context.Context, modelID string, entries []SemanticEntryInput) (*SemanticImportResult, error) {
	if len(entries) == 0 {
		return nil, ErrEntriesRequired
	}
	for _, entry := range entries {
		if err := validateSemanticEntry(entry); err != nil {
			return nil, err
		}
	}
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticImportResult
	err = s.do(ctx, http.MethodPost, "/semantic-models/"+id+"/import", semanticImportBody(entries), &out)
	return &out, err
}

// Export exports semantic model entries.
func (s *KnowledgeService) Export(ctx context.Context, modelID string) (*SemanticExportResult, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticExportResult
	err = s.do(ctx, http.MethodGet, "/semantic-models/"+id+"/export", nil, &out)
	return &out, err
}

// Validate validates one semantic model.
func (s *KnowledgeService) Validate(ctx context.Context, modelID string) (*SemanticValidateResult, error) {
	id, err := pathID(modelID)
	if err != nil {
		return nil, err
	}
	var out SemanticValidateResult
	err = s.do(ctx, http.MethodPost, "/semantic-models/"+id+"/validate", nil, &out)
	return &out, err
}

// DrawingSearchRequest is the product drawing search request.
// DrawingNumber is an exact current-drawing lookup and does not require Query.
// Query image is a local path, stream, or bytes; query_image_file_id is not a public field.
type DrawingSearchRequest struct {
	KnowledgeBaseID int64  `json:"knowledge_base_id"`
	DrawingNumber   string `json:"drawing_number,omitempty"`
	Query           string `json:"query,omitempty"`
	TopK            int    `json:"top_k,omitempty"`
	// TopKPresent preserves an explicitly supplied zero TopK in JSON and multipart requests.
	TopKPresent        bool      `json:"-"`
	QueryImagePath     string    `json:"-"`
	QueryImageFilename string    `json:"-"`
	QueryImage         io.Reader `json:"-"`
	QueryImageBytes    []byte    `json:"-"`
}

// DrawingSearchResponse is the product drawing search response payload.
type DrawingSearchResponse struct {
	Drawings    []DrawingSearchResult    `json:"drawings"`
	Count       int                      `json:"count"`
	MatchMode   string                   `json:"match_mode,omitempty"`
	Diagnostics DrawingSearchDiagnostics `json:"diagnostics"`
}

func (r *DrawingSearchResponse) UnmarshalJSON(data []byte) error {
	if err := validateDrawingSearchResponseJSON(data); err != nil {
		return err
	}
	type alias DrawingSearchResponse
	var out alias
	if err := json.Unmarshal(data, &out); err != nil {
		return &Error{Code: "ErrResponseInvalid", Message: "drawing search response is invalid"}
	}
	*r = DrawingSearchResponse(out)
	return nil
}

func validateDrawingSearchResponseJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return &Error{Code: "ErrResponseInvalid", Message: "drawing search response must be an object"}
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return &Error{Code: "ErrResponseInvalid", Message: "drawing search response must be an object"}
	}
	drawings, hasDrawings := fields["drawings"]
	count, hasCount := fields["count"]
	diagnostics, hasDiagnostics := fields["diagnostics"]
	if !hasDrawings || !hasCount || !hasDiagnostics {
		return &Error{Code: "ErrResponseInvalid", Message: "drawing search response missing required fields"}
	}
	drawings = bytes.TrimSpace(drawings)
	if len(drawings) == 0 || drawings[0] != '[' {
		return &Error{Code: "ErrResponseInvalid", Message: "drawing search drawings must be a list"}
	}
	if err := requireNonNegativeJSONInt(count); err != nil {
		return &Error{Code: "ErrResponseInvalid", Message: "drawing search count must be a non-negative integer"}
	}
	diagnostics = bytes.TrimSpace(diagnostics)
	if len(diagnostics) == 0 || diagnostics[0] != '{' {
		return &Error{Code: "ErrResponseInvalid", Message: "drawing search diagnostics must be an object"}
	}
	return nil
}

func requireNonNegativeJSONInt(raw json.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return fmt.Errorf("missing")
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	var n json.Number
	if err := dec.Decode(&n); err != nil {
		return err
	}
	i, err := strconv.ParseInt(string(n), 10, 64)
	if err != nil {
		return err
	}
	if i < 0 {
		return fmt.Errorf("negative")
	}
	return nil
}

// DrawingSearchResult is one aggregated drawing candidate.
type DrawingSearchResult struct {
	SourceFileID         string              `json:"source_file_id"`
	DrawingNumber        string              `json:"drawing_number,omitempty"`
	DrawingID            string              `json:"drawing_id"`
	Score                float64             `json:"score"`
	BestHit              DrawingSearchHit    `json:"best_hit"`
	SupportingHits       []DrawingSearchHit  `json:"supporting_hits"`
	ExternalSourceURI    *string             `json:"external_source_uri,omitempty"`
	VisualManifestFileID *string             `json:"visual_manifest_file_id,omitempty"`
	ProvenanceStatus     string              `json:"provenance_status,omitempty"`
	Gaps                 []string            `json:"gaps,omitempty"`
	Pages                []DrawingSearchPage `json:"pages,omitempty"`
	DefaultVersion       bool                `json:"default_version,omitzero"`
}

// DrawingSearchPage is one full-page preview for a drawing source.
type DrawingSearchPage struct {
	PageNumber      int    `json:"page_number"`
	PageImageFileID string `json:"page_image_file_id"`
	PreviewURL      string `json:"preview_url"`
}

// DrawingProvenance is the page and origin snapshot for one source file.
type DrawingProvenance struct {
	KnowledgeBaseID      int64               `json:"knowledge_base_id"`
	SourceFileID         string              `json:"source_file_id"`
	ExternalSourceURI    *string             `json:"external_source_uri"`
	VisualManifestFileID *string             `json:"visual_manifest_file_id"`
	ProvenanceStatus     string              `json:"provenance_status"`
	Gaps                 []string            `json:"gaps"`
	Pages                []DrawingSearchPage `json:"pages"`
}

// DrawingSearchHit is one region-level retrieval hit.
type DrawingSearchHit struct {
	ObjectID         string    `json:"object_id,omitempty"`
	ObjectKind       string    `json:"object_kind,omitempty"`
	RetrievalLevel   string    `json:"retrieval_level,omitempty"`
	RetrievalChannel string    `json:"retrieval_channel,omitempty"`
	RetrievalScope   string    `json:"retrieval_scope,omitempty"`
	SourceFileID     string    `json:"source_file_id"`
	DrawingID        string    `json:"drawing_id,omitempty"`
	ImageFileID      string    `json:"image_file_id,omitempty"`
	PageImageFileID  string    `json:"page_image_file_id,omitempty"`
	PageNumber       int       `json:"page_number,omitempty"`
	BBox             []float64 `json:"bbox,omitempty"`
	Score            float64   `json:"score"`
	Content          string    `json:"content,omitempty"`
	Modality         string    `json:"modality,omitempty"`
	Reason           string    `json:"reason,omitempty"`
	Channel          string    `json:"channel,omitempty"`
}

// DrawingSearchDiagnostics reports latency and retrieval counts.
type DrawingSearchDiagnostics struct {
	TotalMS                    int64    `json:"total_ms"`
	PlanMS                     int64    `json:"plan_ms"`
	EmbedMS                    int64    `json:"embed_ms,omitempty"`
	TextRetrieveMS             int64    `json:"text_retrieve_ms,omitempty"`
	ImageRetrieveMS            int64    `json:"image_retrieve_ms,omitempty"`
	MergeMS                    int64    `json:"merge_ms"`
	AggregateMS                int64    `json:"aggregate_ms"`
	Channels                   []string `json:"channels"`
	CandidateHitCount          int      `json:"candidate_hit_count"`
	UniqueDrawingCount         int      `json:"unique_drawing_count"`
	ResultCount                int      `json:"result_count"`
	IdentifierRouteUsed        bool     `json:"identifier_route_used"`
	ExtractedIdentifiers       []string `json:"extracted_identifiers,omitempty"`
	IdentifierHitCount         int      `json:"identifier_hit_count"`
	IdentifierFilenameHitCount int      `json:"identifier_filename_hit_count"`
	IdentifierL1HitCount       int      `json:"identifier_l1_hit_count"`
	VectorHitCount             int      `json:"vector_hit_count"`
}

// SearchDrawings runs POST /newmoi/drawings/search.
// Exact number lookup is JSON. A query image uses multipart and may also carry drawing_number.
func (s *KnowledgeService) SearchDrawings(ctx context.Context, req DrawingSearchRequest) (*DrawingSearchResponse, error) {
	if req.KnowledgeBaseID <= 0 {
		return nil, ErrKnowledgeBaseIDRequired
	}
	number := strings.TrimSpace(req.DrawingNumber)
	query := strings.TrimSpace(req.Query)
	filename, reader, closer, err := drawingSearchImage(req)
	if err != nil {
		return nil, err
	}
	if closer != nil {
		defer closer()
	}
	if number == "" && query == "" && reader == nil {
		return nil, ErrQueryRequired
	}
	if reader == nil {
		body := map[string]any{"knowledge_base_id": req.KnowledgeBaseID}
		if number != "" {
			body["drawing_number"] = number
		}
		if query != "" {
			body["query"] = query
		}
		if req.TopK != 0 || req.TopKPresent {
			body["top_k"] = req.TopK
		}
		var out DrawingSearchResponse
		if err := s.do(ctx, http.MethodPost, "/drawings/search", body, &out); err != nil {
			return nil, err
		}
		return &out, nil
	}
	fields := map[string]string{"knowledge_base_id": strconv.FormatInt(req.KnowledgeBaseID, 10)}
	if number != "" {
		fields["drawing_number"] = number
	}
	if query != "" {
		fields["query"] = query
	}
	if req.TopK != 0 || req.TopKPresent {
		fields["top_k"] = strconv.Itoa(req.TopK)
	}
	buf, contentType, err := multipartBody(fields, []MultipartFile{{FieldName: "file", Filename: filename, Reader: reader}})
	if err != nil {
		return nil, err
	}
	var out DrawingSearchResponse
	if err := s.doRaw(ctx, http.MethodPost, "/drawings/search", buf, contentType, contentTypeJSON, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDrawingProvenance reads GET /newmoi/drawings/{source_file_id}/provenance.
func (s *KnowledgeService) GetDrawingProvenance(ctx context.Context, knowledgeBaseID int64, sourceFileID string, pageNumber int) (*DrawingProvenance, error) {
	if knowledgeBaseID <= 0 {
		return nil, ErrKnowledgeBaseIDRequired
	}
	id, err := pathID(sourceFileID)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("knowledge_base_id", strconv.FormatInt(knowledgeBaseID, 10))
	if pageNumber > 0 {
		query.Set("page_number", strconv.Itoa(pageNumber))
	}
	var out DrawingProvenance
	if err := s.do(ctx, http.MethodGet, "/drawings/"+id+"/provenance?"+query.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func drawingSearchImage(req DrawingSearchRequest) (string, io.Reader, func(), error) {
	if path := strings.TrimSpace(req.QueryImagePath); path != "" {
		file, err := os.Open(path)
		if err != nil {
			return "", nil, nil, err
		}
		name := strings.TrimSpace(req.QueryImageFilename)
		if name == "" {
			name = filepath.Base(path)
		}
		return name, file, func() { _ = file.Close() }, nil
	}
	if len(req.QueryImageBytes) > 0 {
		name := strings.TrimSpace(req.QueryImageFilename)
		if name == "" {
			return "", nil, nil, &Error{Code: "ErrFilenameRequired", Message: "filename is required"}
		}
		return name, bytes.NewReader(req.QueryImageBytes), nil, nil
	}
	if req.QueryImage != nil {
		name := strings.TrimSpace(req.QueryImageFilename)
		if name == "" {
			return "", nil, nil, &Error{Code: "ErrFilenameRequired", Message: "filename is required"}
		}
		return name, req.QueryImage, nil, nil
	}
	return "", nil, nil, nil
}

// List returns UI-visible compute resources.
func (s *ComputeService) List(ctx context.Context, opts ...ListOption) ([]*ComputeResource, error) {
	var out []*ComputeResource
	err := s.do(ctx, http.MethodGet, withListQuery("/compute-resources", opts), nil, &out)
	return out, err
}

// Specs lists compute resource specs visible for compute resource creation.
func (s *ComputeService) Specs(ctx context.Context) ([]*ComputeResourceSpec, error) {
	var out []*ComputeResourceSpec
	err := s.do(ctx, http.MethodGet, "/compute-resource-specs", nil, &out)
	return out, err
}

// SpecManagementPermission returns whether the caller can manage compute resource specs.
func (s *ComputeService) SpecManagementPermission(ctx context.Context) (*ComputeResourceSpecManagementPermission, error) {
	var out ComputeResourceSpecManagementPermission
	err := s.do(ctx, http.MethodGet, "/compute-resource-specs/management-permission", nil, &out)
	return &out, err
}

// CreateSpec creates one compute resource spec.
func (s *ComputeService) CreateSpec(ctx context.Context, specID, kind, family, familyName, familyNameEN string, cpuMilli, memoryMiB int, opts ...ComputeSpecOption) (*ComputeResourceSpec, error) {
	body, err := computeSpecBody(specID, kind, family, familyName, familyNameEN, cpuMilli, memoryMiB, opts...)
	if err != nil {
		return nil, err
	}
	var out ComputeResourceSpec
	err = s.do(ctx, http.MethodPost, "/compute-resource-specs", body, &out)
	return &out, err
}

// UpdateSpec updates one compute resource spec.
func (s *ComputeService) UpdateSpec(ctx context.Context, specID string, opts ...ComputeSpecUpdateOption) (*ComputeResourceSpec, error) {
	id, err := pathID(specID)
	if err != nil {
		return nil, err
	}
	var out ComputeResourceSpec
	err = s.do(ctx, http.MethodPut, "/compute-resource-specs/"+id, computeSpecUpdateBody(opts...), &out)
	return &out, err
}

// SetSpecEnabled enables or disables one compute resource spec.
func (s *ComputeService) SetSpecEnabled(ctx context.Context, specID string, enabled bool) (*ComputeResourceSpec, error) {
	id, err := pathID(specID)
	if err != nil {
		return nil, err
	}
	var out ComputeResourceSpec
	err = s.do(ctx, http.MethodPatch, "/compute-resource-specs/"+id+"/enabled", map[string]any{"enabled": enabled}, &out)
	return &out, err
}

// Create creates a UI-visible compute resource.
func (s *ComputeService) Create(ctx context.Context, name, specID string, cpu, memoryGiB int, opts ...ComputeCreateOption) (*ComputeResource, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	if specID == "" {
		return nil, ErrSpecIDRequired
	}
	if cpu == 0 {
		return nil, ErrCPURequired
	}
	if memoryGiB == 0 {
		return nil, ErrMemoryRequired
	}
	body, err := computeCreateBody(name, specID, cpu, memoryGiB, opts...)
	if err != nil {
		return nil, err
	}
	var out ComputeResource
	err = s.do(ctx, http.MethodPost, "/compute-resources", body, &out)
	return &out, err
}

// ComputeResourceWorker is a per-instance 观测态 row for a compute resource:
// the design-#19948 workers listing. It is served as raw DTO JSON (no proto
// message) by GET /compute-resources/{id}/workers.
type ComputeResourceWorker struct {
	InstanceID           string `json:"instance_id"`
	WorkerType           string `json:"worker_type"`
	Status               string `json:"status"`
	PyRequirementsSHA    string `json:"py_requirements_sha,omitempty"`
	PyRequirementsStatus string `json:"py_requirements_status,omitempty"`
	PyRequirementsError  string `json:"py_requirements_error,omitempty"`
}

// Workers returns the per-instance 观测态 (design-#19948) for one readable
// compute resource. The handler preflights compute_resource.read:resourceID;
// an empty list means no worker row has been observed yet.
func (s *ComputeService) Workers(ctx context.Context, computeResourceID string) ([]*ComputeResourceWorker, error) {
	id, err := pathID(computeResourceID)
	if err != nil {
		return nil, err
	}
	var out []*ComputeResourceWorker
	if err := s.do(ctx, http.MethodGet, "/compute-resources/"+id+"/workers", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ComputePyDepsLimits is the read-only projection of the py_deps.* runtime
// dynamic config consumed by the py_deps editor and detail block. It is served
// as raw DTO JSON by GET /compute-resources/py-deps-limits.
type ComputePyDepsLimits struct {
	Enabled               bool     `json:"enabled"`
	MaxBytes              int64    `json:"max_bytes"`
	MaxLines              int64    `json:"max_lines"`
	AllowedIndexHosts     []string `json:"allowed_index_hosts"`
	PipIndexURL           string   `json:"pip_index_url,omitempty"`
	InstallTimeoutSeconds int64    `json:"install_timeout_seconds"`
}

// PyDepsLimits reads the current py_deps.* limits. When the deployment has not
// surfaced the py_deps configuration view the backend returns 503
// ErrLimitsUnavailable, surfaced as an *Error; callers treat that as "feature
// off" rather than inventing a default.
func (s *ComputeService) PyDepsLimits(ctx context.Context) (*ComputePyDepsLimits, error) {
	var out ComputePyDepsLimits
	if err := s.do(ctx, http.MethodGet, "/compute-resources/py-deps-limits", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WorkerImages lists Catalog worker images. Callers may use only entries with
// IsDefault=true in WithComputeWorkerImages; omitting that option lets Catalog
// select the current configured default during deployment.
func (s *ComputeService) WorkerImages(ctx context.Context, opts ...QueryOption) ([]*WorkerImage, error) {
	var out []*WorkerImage
	if err := s.do(ctx, http.MethodGet, withQuery("/compute-resources/worker-images", opts), nil, &out); err != nil {
		return nil, err
	}
	if err := validateComputeWorkerImages(out); err != nil {
		return nil, err
	}
	return out, nil
}

func validateComputeWorkerImages(images []*WorkerImage) error {
	for index, image := range images {
		if image == nil {
			return &Error{Code: "ErrResponseInvalid", Message: fmt.Sprintf("worker images[%d] is null", index)}
		}
		workerType := image.GetWorkerType()
		imageKey := image.GetImageKey()
		legacyType := image.GetType()
		if workerType == "" || workerType != strings.TrimSpace(workerType) {
			return &Error{Code: "ErrResponseInvalid", Message: fmt.Sprintf("worker images[%d] has invalid worker_type", index)}
		}
		if imageKey == "" || imageKey != strings.TrimSpace(imageKey) {
			return &Error{Code: "ErrResponseInvalid", Message: fmt.Sprintf("worker images[%d] has invalid image_key", index)}
		}
		if legacyType == "" || legacyType != strings.TrimSpace(legacyType) || legacyType != imageKey {
			return &Error{Code: "ErrResponseInvalid", Message: fmt.Sprintf("worker images[%d] type must equal image_key", index)}
		}
	}
	return nil
}

// Runtime returns persisted Worker-type runtime rows for one readable compute resource.
func (s *ComputeService) Runtime(ctx context.Context, computeResourceID string) ([]*ComputeWorkerRuntime, error) {
	id, err := pathID(computeResourceID)
	if err != nil {
		return nil, err
	}
	var out strictComputeWorkerRuntimeList
	if err := s.do(ctx, http.MethodGet, "/compute-resources/"+id+"/runtime", nil, &out); err != nil {
		return nil, err
	}
	return out.runtimes, nil
}

// AdminRuntime returns persisted Worker-type runtime rows for one resource
// through the spec-administrator route.
func (s *ComputeService) AdminRuntime(ctx context.Context, computeResourceID string) ([]*ComputeWorkerRuntime, error) {
	id, err := pathID(computeResourceID)
	if err != nil {
		return nil, err
	}
	var out strictComputeWorkerRuntimeList
	if err := s.do(ctx, http.MethodGet, "/admin/compute-resources/"+id+"/runtime", nil, &out); err != nil {
		return nil, err
	}
	return out.runtimes, nil
}

type strictComputeWorkerRuntimeList struct {
	runtimes []*ComputeWorkerRuntime
}

func (*strictComputeWorkerRuntimeList) requireProductEnvelope() {}

func (out *strictComputeWorkerRuntimeList) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("runtime data must be an object: %w", err)
	}
	rawRuntimes, ok := fields["runtimes"]
	if !ok || string(rawRuntimes) == "null" {
		return fmt.Errorf("runtime data is missing runtimes")
	}
	if len(fields) != 1 {
		return fmt.Errorf("runtime data contains an unknown field")
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(rawRuntimes, &entries); err != nil {
		return fmt.Errorf("runtime data runtimes must be an array: %w", err)
	}
	decoded := make([]*ComputeWorkerRuntime, 0, len(entries))
	decoder := protojson.UnmarshalOptions{DiscardUnknown: false}
	for index, entry := range entries {
		runtime := &ComputeWorkerRuntime{}
		if err := decoder.Unmarshal(entry, runtime); err != nil {
			return fmt.Errorf("runtime data runtimes[%d]: %w", index, err)
		}
		if err := validateComputeWorkerRuntime(runtime, index); err != nil {
			return err
		}
		decoded = append(decoded, runtime)
	}
	out.runtimes = decoded
	return nil
}

func validateComputeWorkerRuntime(runtime *ComputeWorkerRuntime, index int) error {
	if runtime == nil {
		return &Error{Code: "ErrResponseInvalid", Message: fmt.Sprintf("runtime data runtimes[%d] is null", index)}
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "compute_resource_id", value: runtime.GetComputeResourceId()},
		{name: "worker_type", value: runtime.GetWorkerType()},
		{name: "status", value: runtime.GetStatus()},
		{name: "created_at", value: runtime.GetCreatedAt()},
		{name: "updated_at", value: runtime.GetUpdatedAt()},
	} {
		if field.value == "" || field.value != strings.TrimSpace(field.value) {
			return &Error{Code: "ErrResponseInvalid", Message: fmt.Sprintf("runtime data runtimes[%d] has invalid %s", index, field.name)}
		}
	}
	if runtime.GetDesiredReplicas() < 0 || runtime.GetCurrentReplicas() < 0 {
		return &Error{Code: "ErrResponseInvalid", Message: fmt.Sprintf("runtime data runtimes[%d] has negative replicas", index)}
	}
	return nil
}

// Get returns one UI-visible compute resource.
func (s *ComputeService) Get(ctx context.Context, id string) (*ComputeResource, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out ComputeResource
	err = s.do(ctx, http.MethodGet, "/compute-resources/"+path, nil, &out)
	return &out, err
}

// Update updates one UI-visible compute resource.
func (s *ComputeService) Update(ctx context.Context, id string, opts ...ComputeUpdateOption) (*ComputeResource, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	body, err := computeUpdateBody(opts...)
	if err != nil {
		return nil, err
	}
	var out ComputeResource
	err = s.do(ctx, http.MethodPut, "/compute-resources/"+path, body, &out)
	return &out, err
}

// AdoptLegacyOwnership explicitly records canonical Ownership for a historical
// ComputeResource. The product API admits only the audited repair permission
// and a Core-verified SuperAdmin; activation paths never call this method.
func (s *ComputeService) AdoptLegacyOwnership(ctx context.Context, id string) (*ComputeResourceOwnership, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out ComputeResourceOwnership
	err = s.do(ctx, http.MethodPost, "/compute-resources/"+path+"/adopt-legacy-ownership", map[string]any{}, &out)
	return &out, err
}

// PreflightDelete checks whether one compute resource can be deleted.
func (s *ComputeService) PreflightDelete(ctx context.Context, id string) (*ComputeDeletePreflightResult, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out ComputeDeletePreflightResult
	err = s.do(ctx, http.MethodGet, "/compute-resources/"+path+"/preflight-delete", nil, &out)
	return &out, err
}

// ComputeDeleteAcknowledgement is the typed data product DELETE returns once
// the durable delete intent has been persisted. Deletion completes
// asynchronously; poll Get to observe the TERMINATING → DELETED transition.
type ComputeDeleteAcknowledgement struct {
	ResourceID string `json:"resource_id"`
	Status     string `json:"status"`
}

// Delete records a durable delete intent for one UI-visible compute resource
// and returns once the resource Status has been flipped to TERMINATING. The
// remaining ownership cleanup and runtime teardown run asynchronously on the
// server; callers should poll Get to observe the terminal DELETED status or
// inspect intermediate TERMINATING state instead of relying on a long-lived
// synchronous request.
func (s *ComputeService) Delete(ctx context.Context, id string, opts ...QueryOption) error {
	path, err := pathID(id)
	if err != nil {
		return err
	}
	var ack ComputeDeleteAcknowledgement
	return s.do(ctx, http.MethodDelete, withQuery("/compute-resources/"+path, opts), nil, &ack)
}

// Suspend suspends one compute resource.
func (s *ComputeService) Suspend(ctx context.Context, id string) (*ComputeResource, error) {
	return s.computeAction(ctx, id, "suspend")
}

// Resume resumes one compute resource.
func (s *ComputeService) Resume(ctx context.Context, id string) (*ComputeResource, error) {
	return s.computeAction(ctx, id, "resume")
}

// Retry retries one compute resource.
func (s *ComputeService) Retry(ctx context.Context, id string) (*ComputeResource, error) {
	return s.computeAction(ctx, id, "retry")
}

// AdminList returns compute resources across workspaces for spec administrators.
func (s *ComputeService) AdminList(ctx context.Context, opts ...ListOption) (*AdminComputeResourceListResult, error) {
	var out AdminComputeResourceListResult
	err := s.do(ctx, http.MethodGet, withListQuery("/admin/compute-resources", opts), nil, &out)
	return &out, err
}

// AdminPreflightDelete checks whether a spec administrator can delete one compute resource.
func (s *ComputeService) AdminPreflightDelete(ctx context.Context, id string) (*ComputeDeletePreflightResult, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out ComputeDeletePreflightResult
	err = s.do(ctx, http.MethodGet, "/admin/compute-resources/"+path+"/preflight-delete", nil, &out)
	return &out, err
}

// AdminUpdate updates one compute resource as a spec administrator.
func (s *ComputeService) AdminUpdate(ctx context.Context, id string, opts ...ComputeUpdateOption) (*ComputeResource, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	body, err := computeUpdateBody(opts...)
	if err != nil {
		return nil, err
	}
	var out ComputeResource
	err = s.do(ctx, http.MethodPut, "/admin/compute-resources/"+path, body, &out)
	return &out, err
}

// AdminDelete deletes one compute resource as a spec administrator. The call
// returns once the delete intent is durably persisted (HTTP 202 with status
// TERMINATING); teardown then continues asynchronously. Poll the admin
// Get/List for Status DELETED to observe the terminal state.
func (s *ComputeService) AdminDelete(ctx context.Context, id string, opts ...QueryOption) error {
	path, err := pathID(id)
	if err != nil {
		return err
	}
	var ack ComputeDeleteAcknowledgement
	return s.do(ctx, http.MethodDelete, withQuery("/admin/compute-resources/"+path, opts), nil, &ack)
}

// AdminSuspend suspends one compute resource as a spec administrator.
func (s *ComputeService) AdminSuspend(ctx context.Context, id string) (*ComputeResource, error) {
	return s.adminComputeAction(ctx, id, "suspend")
}

// AdminResume resumes one compute resource as a spec administrator.
func (s *ComputeService) AdminResume(ctx context.Context, id string) (*ComputeResource, error) {
	return s.adminComputeAction(ctx, id, "resume")
}

// AdminRetry retries one compute resource as a spec administrator.
func (s *ComputeService) AdminRetry(ctx context.Context, id string) (*ComputeResource, error) {
	return s.adminComputeAction(ctx, id, "retry")
}

// Publishes lists data-share publish resources.
func (s *DataShareService) Publishes(ctx context.Context, opts ...QueryOption) (*DataSharePublishListResult, error) {
	var out DataSharePublishListResult
	err := s.do(ctx, http.MethodGet, withQuery("/data-share/publishes", opts), nil, &out)
	return &out, err
}

// WorkspaceNamesResult resolves data-share workspace names by id.
func (s *DataShareService) WorkspaceNamesResult(ctx context.Context, ids []string) (*DataShareWorkspaceNameListResult, error) {
	var out DataShareWorkspaceNameListResult
	err := s.do(ctx, http.MethodPost, "/data-share/workspace-names", map[string]any{"ids": ids}, &out)
	return &out, err
}

// ResolveWorkspaceNames resolves data-share workspace names by id without requiring an existing share relation.
func (s *DataShareService) ResolveWorkspaceNames(ctx context.Context, ids []string) (*DataShareWorkspaceNameListResult, error) {
	var out DataShareWorkspaceNameListResult
	err := s.do(ctx, http.MethodPost, "/data-share/workspaces/resolve-names", map[string]any{"ids": ids}, &out)
	return &out, err
}

// PublishSummary returns data-share publication summary counts.
func (s *DataShareService) PublishSummary(ctx context.Context) (*PublishSummaryResult, error) {
	var out PublishSummaryResult
	err := s.do(ctx, http.MethodGet, "/data-share/publishes/summary", nil, &out)
	return &out, err
}

// SourceTables lists publishable Tables using stable MatrixOne rel_id values.
func (s *DataShareService) SourceTables(ctx context.Context, sourceDatabaseID string, opts ...QueryOption) (*DataShareSourceTableListResult, error) {
	id, err := pathID(sourceDatabaseID)
	if err != nil {
		return nil, ErrSourceDatabaseIDRequired
	}
	var out DataShareSourceTableListResult
	err = s.do(ctx, http.MethodGet, withQuery("/data-share/source-databases/"+id+"/tables", opts), nil, &out)
	return &out, err
}

// CheckPublishName checks data-share publish name availability.
func (s *DataShareService) CheckPublishName(ctx context.Context, opts ...QueryOption) (*CheckNameResult, error) {
	var out CheckNameResult
	err := s.do(ctx, http.MethodGet, withQuery("/data-share/publishes/check-name", opts), nil, &out)
	return &out, err
}

// CreatePublish creates a data-share publish resource.
func (s *DataShareService) CreatePublish(ctx context.Context, name, sourceDatabaseID string, tableScope *DataShareScope, targetWorkspaceIDs []string, opts ...DataSharePublishOption) (*DataSharePublish, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	if sourceDatabaseID == "" {
		return nil, ErrSourceDatabaseIDRequired
	}
	if !validDataShareScope(tableScope) {
		return nil, ErrTableScopeRequired
	}
	if len(targetWorkspaceIDs) == 0 {
		return nil, ErrTargetWorkspaceRequired
	}
	var out DataSharePublish
	err := s.do(ctx, http.MethodPost, "/data-share/publishes", dataSharePublishBody(name, sourceDatabaseID, tableScope, targetWorkspaceIDs, opts...), &out)
	return &out, err
}

// UpdatePublish updates a data-share publish resource.
func (s *DataShareService) UpdatePublish(ctx context.Context, publishID string, opts ...DataSharePublishUpdateOption) (*DataSharePublish, error) {
	id, err := pathID(publishID)
	if err != nil {
		return nil, err
	}
	var out DataSharePublish
	err = s.do(ctx, http.MethodPut, "/data-share/publishes/"+id, dataSharePublishUpdateBody(opts...), &out)
	return &out, err
}

// DeletePublish deletes a data-share publish resource.
func (s *DataShareService) DeletePublish(ctx context.Context, publishID string) error {
	id, err := pathID(publishID)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodDelete, "/data-share/publishes/"+id, nil, nil)
}

// Subscriptions lists data-share subscriptions.
func (s *DataShareService) Subscriptions(ctx context.Context, opts ...QueryOption) (*DataShareSubscriptionListResult, error) {
	var out DataShareSubscriptionListResult
	err := s.do(ctx, http.MethodGet, withQuery("/data-share/subscriptions", opts), nil, &out)
	return &out, err
}

// Subscribe subscribes to a data-share subscription.
func (s *DataShareService) Subscribe(ctx context.Context, subscriptionID, subName string) (*DataShareSubscription, error) {
	if subName == "" {
		return nil, ErrNameRequired
	}
	return s.subscriptionAction(ctx, subscriptionID, "subscribe", dataShareSubscribeBody(subName))
}

func validDataShareScope(scope *DataShareScope) bool {
	if scope == nil {
		return false
	}
	switch scope.GetMode() {
	case "all":
		return len(scope.GetObjectIds()) == 0
	case "selected":
		return len(scope.GetObjectIds()) > 0
	default:
		return false
	}
}

// Unsubscribe unsubscribes from a data-share subscription.
func (s *DataShareService) Unsubscribe(ctx context.Context, subscriptionID string) error {
	id, err := pathID(subscriptionID)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodPost, "/data-share/subscriptions/"+id+"/unsubscribe", nil, nil)
}

func (s *DataShareService) subscriptionAction(ctx context.Context, subscriptionID, action string, body any) (*DataShareSubscription, error) {
	id, err := pathID(subscriptionID)
	if err != nil {
		return nil, err
	}
	var out DataShareSubscription
	err = s.do(ctx, http.MethodPost, "/data-share/subscriptions/"+id+"/"+action, body, &out)
	return &out, err
}

// LLMBackends returns UI-visible LLM backend management APIs.
func (s *ModelService) LLMBackends() *ProviderBackendService {
	return &ProviderBackendService{workspaceService: s.workspaceService, basePath: "/llm"}
}

// EmbeddingBackends returns UI-visible embedding backend management APIs.
func (s *ModelService) EmbeddingBackends() *ProviderBackendService {
	return &ProviderBackendService{workspaceService: s.workspaceService, basePath: "/embedding"}
}

// ParserBackends returns UI-visible parser backend management APIs.
func (s *ModelService) ParserBackends() *ProviderBackendService {
	return &ProviderBackendService{workspaceService: s.workspaceService, basePath: "/parser"}
}

// List returns the flat model list used by product selectors.
func (s *ModelService) List(ctx context.Context, opts ...QueryOption) (*ProviderModelListResult, error) {
	var out ProviderModelListResult
	err := s.do(ctx, http.MethodGet, withQuery("/models", opts), nil, &out)
	return &out, err
}

// ListEmbeddingModels returns the flat embedding model list used by product selectors.
func (s *ModelService) ListEmbeddingModels(ctx context.Context) (*ProviderModelListResult, error) {
	var out ProviderModelListResult
	err := s.do(ctx, http.MethodGet, "/embedding/models", nil, &out)
	return &out, err
}

// SetupLLMBackend creates an LLM backend and endpoint in one product operation.
func (s *ModelService) SetupLLMBackend(ctx context.Context, name, address string, models []string, opts ...ProviderBackendOption) (*ProviderSetupBackendResult, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	if address == "" {
		return nil, ErrAddressRequired
	}
	if len(models) == 0 {
		return nil, ErrModelsRequired
	}
	body := providerBackendBody(name, opts...)
	body["address"] = address
	body["models"] = models
	var out ProviderSetupBackendResult
	err := s.do(ctx, http.MethodPost, "/llm/setup", body, &out)
	return &out, err
}

// ProbeLLMModels probes a remote OpenAI-compatible endpoint.
func (s *ModelService) ProbeLLMModels(ctx context.Context, address string, opts ...ProviderBackendOption) (*ProviderProbeModelsResult, error) {
	if address == "" {
		return nil, ErrAddressRequired
	}
	var out ProviderProbeModelsResult
	err := s.do(ctx, http.MethodPost, "/llm/probe-models", providerProbeBody(address, opts...), &out)
	return &out, err
}

// ProbeEmbeddingModels probes embedding models from a remote endpoint.
func (s *ModelService) ProbeEmbeddingModels(ctx context.Context, address string, opts ...ProviderBackendOption) (*ProviderProbeModelsResult, error) {
	if address == "" {
		return nil, ErrAddressRequired
	}
	var out ProviderProbeModelsResult
	err := s.do(ctx, http.MethodPost, "/embedding/probe-models", providerProbeBody(address, opts...), &out)
	return &out, err
}

// List lists provider backends.
func (s *ProviderBackendService) List(ctx context.Context) (*ProviderBackendListResult, error) {
	if s.basePath == "/parser" {
		var out parserBackendList
		err := s.do(ctx, http.MethodGet, s.basePath+"/backends", nil, &out)
		return &ProviderBackendListResult{Backends: out.backends}, err
	}
	var out ProviderBackendListResult
	err := s.do(ctx, http.MethodGet, s.basePath+"/backends", nil, &out)
	return &out, err
}

// Create creates one provider backend.
func (s *ProviderBackendService) Create(ctx context.Context, name string, opts ...ProviderBackendOption) (*ProviderBackend, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	body := providerBackendBody(name, opts...)
	if s.basePath == "/parser" {
		// Parser CreateParserBackendRequest accepts supported_mime_types; LLM/embedding do not.
		applySupportedMimeTypes(body, opts...)
	} else {
		if _, ok := body["models"]; !ok {
			return nil, ErrModelsRequired
		}
	}
	var out ProviderBackend
	err := s.do(ctx, http.MethodPost, s.basePath+"/backends", body, &out)
	return &out, err
}

// Get returns one provider backend.
func (s *ProviderBackendService) Get(ctx context.Context, id string) (*ProviderBackend, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out ProviderBackend
	err = s.do(ctx, http.MethodGet, s.basePath+"/backends/"+path, nil, &out)
	return &out, err
}

// Update updates one provider backend.
func (s *ProviderBackendService) Update(ctx context.Context, id string, opts ...ProviderBackendUpdateOption) (*ProviderBackend, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out ProviderBackend
	err = s.do(ctx, http.MethodPut, s.basePath+"/backends/"+path, providerBackendUpdateBody(opts...), &out)
	return &out, err
}

// Delete deletes one provider backend.
func (s *ProviderBackendService) Delete(ctx context.Context, id string) (*ProviderMutationResult, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	if s.basePath == "/parser" {
		err = s.do(ctx, http.MethodDelete, s.basePath+"/backends/"+path, nil, nil)
		return &ProviderMutationResult{Success: err == nil}, err
	}
	var out ProviderMutationResult
	err = s.do(ctx, http.MethodDelete, s.basePath+"/backends/"+path, nil, &out)
	return &out, err
}

// CreateEndpoint creates an endpoint under one provider backend.
func (s *ProviderBackendService) CreateEndpoint(ctx context.Context, backendID, address string) (*ProviderEndpoint, error) {
	path, err := pathID(backendID)
	if err != nil {
		return nil, err
	}
	if address == "" {
		return nil, ErrAddressRequired
	}
	var out ProviderEndpoint
	err = s.do(ctx, http.MethodPost, s.basePath+"/backends/"+path+"/endpoints", providerEndpointBody(address), &out)
	return &out, err
}

// ListEndpoints lists endpoints under one provider backend.
func (s *ProviderBackendService) ListEndpoints(ctx context.Context, backendID string) (*ProviderEndpointListResult, error) {
	path, err := pathID(backendID)
	if err != nil {
		return nil, err
	}
	if s.basePath == "/parser" {
		var out parserEndpointList
		err = s.do(ctx, http.MethodGet, s.basePath+"/backends/"+path+"/endpoints", nil, &out)
		return &ProviderEndpointListResult{Endpoints: out.endpoints}, err
	}
	var out ProviderEndpointListResult
	err = s.do(ctx, http.MethodGet, s.basePath+"/backends/"+path+"/endpoints", nil, &out)
	return &out, err
}

// SetEndpointStatus updates one provider endpoint status.
func (s *ProviderBackendService) SetEndpointStatus(ctx context.Context, backendID, endpointID, status string) (*ProviderMutationResult, error) {
	backendPath, endpointPath, err := providerEndpointPath(backendID, endpointID)
	if err != nil {
		return nil, err
	}
	if status == "" {
		return nil, ErrStatusRequired
	}
	if s.basePath == "/parser" {
		err = s.do(ctx, http.MethodPut, s.basePath+"/backends/"+backendPath+"/endpoints/"+endpointPath+"/status", endpointStatusBody(status), nil)
		return &ProviderMutationResult{Success: err == nil}, err
	}
	var out ProviderMutationResult
	err = s.do(ctx, http.MethodPut, s.basePath+"/backends/"+backendPath+"/endpoints/"+endpointPath+"/status", endpointStatusBody(status), &out)
	return &out, err
}

// Router returns the provider router config.
func (s *ProviderBackendService) Router(ctx context.Context) (*ProviderRouterConfig, error) {
	var out ProviderRouterConfig
	err := s.do(ctx, http.MethodGet, s.basePath+"/router", nil, &out)
	return &out, err
}

// UpdateRouter updates the provider router config.
func (s *ProviderBackendService) UpdateRouter(ctx context.Context, opts ...RouterOption) (*ProviderRouterConfig, error) {
	var out ProviderRouterConfig
	err := s.do(ctx, http.MethodPut, s.basePath+"/router", routerBody(opts...), &out)
	return &out, err
}

// ProbeBackendModels probes models using one configured provider backend.
func (s *ProviderBackendService) ProbeBackendModels(ctx context.Context, backendID string, opts ...ProviderBackendOption) (*ProviderProbeModelsResult, error) {
	path, err := pathID(backendID)
	if err != nil {
		return nil, err
	}
	if s.basePath == "/parser" {
		return nil, &Error{Code: "ErrUnsupportedOperation", Message: "provider kind does not support backend model probe: parser"}
	}
	probeBody := providerBackendProbeBody(opts...)
	if _, ok := probeBody["api_key"]; !ok {
		return nil, ErrAPIKeyRequired
	}
	var out ProviderProbeModelsResult
	err = s.do(ctx, http.MethodPost, s.basePath+"/backends/"+path+"/probe-models", probeBody, &out)
	return &out, err
}

func (s *CatalogService) catalogPost(ctx context.Context, path string, body any) (*ProductData, error) {
	var out ProductData
	err := s.do(ctx, http.MethodPost, path, body, &out)
	return &out, err
}

func addCatalogListOptions(body map[string]any, opts []CatalogListOption) {
	options := &catalogListOptions{}
	for _, opt := range opts {
		opt(options)
	}
	if options.includeStats != nil {
		body["include_stats"] = *options.includeStats
	}
	if options.pageSize != nil {
		body["page_size"] = *options.pageSize
	}
	if options.pageToken != nil {
		body["page_token"] = *options.pageToken
	}
}

func addCatalogInfoOptions(body map[string]any, opts []CatalogInfoOption) {
	options := &catalogInfoOptions{}
	for _, opt := range opts {
		opt(options)
	}
	if options.description != nil {
		body["description"] = *options.description
	}
}

func (s *ConnectorService) connectorPost(ctx context.Context, path string, body any) (*ProductData, error) {
	var out ProductData
	err := s.do(ctx, http.MethodPost, path, body, &out)
	return &out, err
}

func (s *SQLService) sqlPost(ctx context.Context, path string, body any) (*ProductData, error) {
	var out ProductData
	err := s.do(ctx, http.MethodPost, path, body, &out)
	return &out, err
}

func (s *WorkflowService) workflowRunAction(ctx context.Context, workflowID, executionID, action string, body any, out any) error {
	workflowPath, executionPath, err := workflowRunPath(workflowID, executionID)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodPost, "/workflow/v2/workflow-apps/"+workflowPath+"/executions/"+executionPath+"/"+action, body, out)
}

func (s *WorkflowService) workflowAction(ctx context.Context, workflowID, action string) (*WorkflowEnvelope, error) {
	id, err := pathID(workflowID)
	if err != nil {
		return nil, err
	}
	var out WorkflowEnvelope
	err = s.do(ctx, http.MethodPost, "/workflow/v2/workflow-apps/"+id+"/"+action, nil, &out)
	return &out, err
}

func (s *WorkflowService) customOperatorAction(ctx context.Context, operatorID, action string) (*CustomOperator, error) {
	id, err := pathID(operatorID)
	if err != nil {
		return nil, err
	}
	var out CustomOperator
	err = s.do(ctx, http.MethodPost, "/workflow/v2/custom-operators/"+id+"/"+action, nil, &out)
	return &out, err
}

func (s *ComputeService) computeAction(ctx context.Context, id, action string) (*ComputeResource, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out ComputeResource
	err = s.do(ctx, http.MethodPost, "/compute-resources/"+path+"/"+action, nil, &out)
	return &out, err
}

func (s *ComputeService) adminComputeAction(ctx context.Context, id, action string) (*ComputeResource, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	var out ComputeResource
	err = s.do(ctx, http.MethodPost, "/admin/compute-resources/"+path+"/"+action, nil, &out)
	return &out, err
}

func workflowRunPath(workflowID, executionID string) (string, string, error) {
	workflowPath, err := pathID(workflowID)
	if err != nil {
		return "", "", err
	}
	executionPath, err := pathID(executionID)
	if err != nil {
		return "", "", err
	}
	return workflowPath, executionPath, nil
}

func workflowRunNodePath(workflowID, executionID, nodeKey string) (string, string, string, error) {
	workflowPath, executionPath, err := workflowRunPath(workflowID, executionID)
	if err != nil {
		return "", "", "", err
	}
	nodePath, err := pathID(nodeKey)
	if err != nil {
		return "", "", "", err
	}
	return workflowPath, executionPath, nodePath, nil
}

func workflowRerunPath(workflowID, executionID, rerunID string) (string, error) {
	workflowPath, executionPath, err := workflowRunPath(workflowID, executionID)
	if err != nil {
		return "", err
	}
	rerunPath, err := pathID(rerunID)
	if err != nil {
		return "", err
	}
	return "/workflow/v2/workflow-apps/" + workflowPath + "/executions/" + executionPath + "/reruns/" + rerunPath, nil
}

func artifactBlockPath(artifactID, blockID string) (string, string, error) {
	artifactPath, err := pathID(artifactID)
	if err != nil {
		return "", "", err
	}
	blockPath, err := pathID(blockID)
	if err != nil {
		return "", "", err
	}
	return artifactPath, blockPath, nil
}

func artifactNodePath(artifactID, nodeID string) (string, string, error) {
	artifactPath, err := pathID(artifactID)
	if err != nil {
		return "", "", err
	}
	nodePath, err := pathID(nodeID)
	if err != nil {
		return "", "", err
	}
	return artifactPath, nodePath, nil
}

func artifactNodeBlockPath(artifactID, nodeID, blockID string) (string, string, string, error) {
	artifactPath, nodePath, err := artifactNodePath(artifactID, nodeID)
	if err != nil {
		return "", "", "", err
	}
	blockPath, err := pathID(blockID)
	if err != nil {
		return "", "", "", err
	}
	return artifactPath, nodePath, blockPath, nil
}

func semanticEntryPath(modelID, entryID string) (string, string, error) {
	modelPath, err := pathID(modelID)
	if err != nil {
		return "", "", err
	}
	entryPath, err := pathID(entryID)
	if err != nil {
		return "", "", err
	}
	return modelPath, entryPath, nil
}

func semanticModelSourcePath(modelID, sourceID string) (string, string, error) {
	modelPath, err := pathID(modelID)
	if err != nil {
		return "", "", err
	}
	sourcePath, err := pathID(sourceID)
	if err != nil {
		return "", "", err
	}
	return modelPath, sourcePath, nil
}

func semanticModelSegmentPath(modelID, sourceID, segmentID string) (string, string, string, error) {
	modelPath, sourcePath, err := semanticModelSourcePath(modelID, sourceID)
	if err != nil {
		return "", "", "", err
	}
	segmentPath, err := pathID(segmentID)
	if err != nil {
		return "", "", "", err
	}
	return modelPath, sourcePath, segmentPath, nil
}

func providerEndpointPath(backendID, endpointID string) (string, string, error) {
	backendPath, err := pathID(backendID)
	if err != nil {
		return "", "", err
	}
	endpointPath, err := pathID(endpointID)
	if err != nil {
		return "", "", err
	}
	return backendPath, endpointPath, nil
}

func multipartBody(fields map[string]string, files []MultipartFile) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return nil, "", fmt.Errorf("write multipart field %s: %w", name, err)
		}
	}
	for _, file := range files {
		fieldName := file.FieldName
		if fieldName == "" {
			fieldName = "file"
		}
		if file.Filename == "" {
			return nil, "", &Error{Code: "ErrFilenameRequired", Message: "filename is required"}
		}
		if file.Reader == nil {
			return nil, "", &Error{Code: "ErrFileReaderRequired", Message: "file reader is required"}
		}
		var part io.Writer
		var err error
		if file.ContentType == "" {
			part, err = writer.CreateFormFile(fieldName, file.Filename)
		} else {
			if _, _, err := mime.ParseMediaType(file.ContentType); err != nil {
				return nil, "", fmt.Errorf("parse multipart content type: %w", err)
			}
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": fieldName, "filename": file.Filename}))
			header.Set("Content-Type", file.ContentType)
			part, err = writer.CreatePart(header)
		}
		if err != nil {
			return nil, "", fmt.Errorf("create multipart file part: %w", err)
		}
		if _, err := io.Copy(part, file.Reader); err != nil {
			return nil, "", fmt.Errorf("copy multipart file content: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("close multipart writer: %w", err)
	}
	return &buf, writer.FormDataContentType(), nil
}
