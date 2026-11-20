package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func TestWorkflowRetryPreservesExistingRuntimeInputs(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []WorkflowRerunOption
		body map[string]any
	}{
		{"values and defaults", []WorkflowRerunOption{WithWorkflowRerunValues(map[string]any{"customer": "acme"}), WithWorkflowRetryReplaceSavedDefaults(true), WithWorkflowRerunTriggerNow(false)}, map[string]any{"values": map[string]any{"customer": "acme"}, "replace_saved_defaults": true, "trigger_now": false}},
		{"immediate dispatch", []WorkflowRerunOption{WithWorkflowRerunTriggerNow(true)}, map[string]any{"trigger_now": true}},
		{"merge defaults", []WorkflowRerunOption{WithWorkflowRetryReplaceSavedDefaults(false)}, map[string]any{}},
		{"omitted inputs", nil, map[string]any{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/retry" {
					t.Errorf("retry request = %s %s", r.Method, r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(body, tc.body) {
					t.Errorf("retry body = %#v, want %#v", body, tc.body)
				}
				_, _ = w.Write([]byte(`{"code":"OK","data":{"workflow_run":{"execution_id":"exec-2"}}}`))
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.Workflows("ws-1").RetryRun(t.Context(), "wf-1", "exec-1", tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if result.GetWorkflowRun().GetExecutionId() != "exec-2" {
				t.Fatalf("retry result = %v", result)
			}
		})
	}
}

func TestStructuredKafkaProvidedSchemaPreservesPublicRequest(t *testing.T) {
	want := `{"connector_id":"kafka-connector","source":{"connector_id":"kafka-connector","source_type":"SOURCE_TYPE_KAFKA"},"kafka_schema":{"key_format":"binary","value_format":"json","schema_source":"provided","json_schema":"{\"type\":\"object\"}"}}`
	var request StructuredSourceSchemaRequest
	if err := json.Unmarshal([]byte(want), &request); err != nil {
		t.Fatal(err)
	}
	var received map[string]any
	client, err := New("http://sdk.example/newmoi", WithAPIKey("test-key"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/newmoi/connectors/structured/v1/source/schema" {
			t.Errorf("schema request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"code":"OK","msg":"OK","data":{"schema_snapshot":{"columns":[]}}}`))}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := client.Connectors("workspace").StructuredSourceSchema(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	var expected map[string]any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected["kafka_schema"], received["kafka_schema"]) {
		t.Fatalf("Kafka schema options = %#v, want %#v", received["kafka_schema"], expected["kafka_schema"])
	}
	if !reflect.DeepEqual(expected["source"], received["source"]) {
		t.Fatalf("Kafka source = %#v, want %#v", received["source"], expected["source"])
	}
	if _, err := client.Connectors("workspace").StructuredSourceSchema(t.Context(), StructuredSourceSchemaRequest{ConnectorID: "database-connector", Source: StructuredSourceRef{SourceType: "SOURCE_TYPE_MYSQL"}}); !errors.Is(err, ErrDatabaseRequired) {
		t.Fatalf("database source missing database = %v", err)
	}
}

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestClientAPIKeyWorkspaceAndProductResponse(t *testing.T) {
	var gotAPIKey, gotWorkspace string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get(headerAPIKey)
		gotWorkspace = r.Header.Get(headerWorkspaceID)
		if r.URL.Path != "/newmoi/workflow/v2/workflow-apps" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"total": 0, "workflows": []any{}},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Workflows("ws-1").List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotAPIKey != "user-key" {
		t.Fatalf("X-API-Key = %q", gotAPIKey)
	}
	if gotWorkspace != "ws-1" {
		t.Fatalf("X-Workspace-ID = %q", gotWorkspace)
	}
	if resp.GetTotal() != 0 || len(resp.GetWorkflows()) != 0 {
		t.Fatalf("missing response data: %+v", resp)
	}
}

func TestWorkflowBatchRetryFailedFilesContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/newmoi/workflow/v2/workflow-apps/wf-1/executions/batch-retry" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["source_dispatch_job_id"] != "dispatch-source" || body["request_id"] != "request-1" {
			t.Fatalf("body = %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK", "msg": "OK",
			"data": map[string]any{
				"source_dispatch_job_id": "dispatch-source", "dispatch_job_id": "dispatch-retry",
				"status": "waiting", "accepted_count": 2, "replayed": false,
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Workflows("ws-1").BatchRetryFailedFiles(context.Background(), "wf-1", "dispatch-source", []string{"exec-1", "exec-2"}, false, "request-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetDispatchJobId() != "dispatch-retry" || resp.GetAcceptedCount() != 2 {
		t.Fatalf("response = %+v", resp)
	}
}

func TestWorkflowGetCustomOperatorSourceContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/newmoi/workflow/v2/custom-operators/source" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("node_id") != "node-1" || r.URL.Query().Get("version") != "v1" || r.URL.Query().Get("task_id") != "task-1" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK", "msg": "OK",
			"data": map[string]any{
				"source": "package workflow", "origin": "draft", "node_id": "node-1", "version": "v1",
				"language": "go", "handler": "Run", "name": "Counter", "identifier": "counter",
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Workflows("ws-1").GetCustomOperatorSource(context.Background(), " node-1 ", " v1 ", " task-1 ")
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetOrigin() != "draft" || resp.GetNodeId() != "node-1" || resp.GetLanguage() != "go" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestSemanticModelWithSourcesBodyIncludesImageIndexEnabled(t *testing.T) {
	body := semanticModelWithSourcesBody("kb", nil, WithSemanticModelWithSourcesImageIndexEnabled(true))
	if got := body["image_index_enabled"]; got != true {
		t.Fatalf("image_index_enabled = %#v, want true", got)
	}
}

func TestSemanticModelEmptyBodyIncludesImageIndexEnabled(t *testing.T) {
	body := semanticModelEmptyBody("kb", WithSemanticModelEmptyDescription("data-side"), WithSemanticModelEmptyImageIndexEnabled(true))
	if body["description"] != "data-side" || body["image_index_enabled"] != true {
		t.Fatalf("body = %#v", body)
	}
}

func TestSemanticModelSourcesBodyUsesUploadedFileID(t *testing.T) {
	items := semanticModelSourcesBody([]SemanticModelSourceInput{{
		SourceType: "local_file",
		FileName:   "manual.pdf",
		FileID:     "uploaded-file-1",
	}})
	if len(items) != 1 || items[0]["file_id"] != "uploaded-file-1" {
		t.Fatalf("sources = %#v", items)
	}
	if _, ok := items[0]["content_base64"]; ok {
		t.Fatalf("sources must not contain content_base64: %#v", items)
	}
}

func TestSemanticModelSourcesBodyIncludesCatalogFileVolumeID(t *testing.T) {
	items := semanticModelSourcesBody([]SemanticModelSourceInput{{
		SourceType: "catalog_file",
		FileID:     "catalog-file-1",
		VolumeID:   41,
	}})
	if len(items) != 1 || items[0]["file_id"] != "catalog-file-1" || items[0]["volume_id"] != int64(41) {
		t.Fatalf("sources = %#v", items)
	}
}

func TestCatalogTreePreservesNumericAndStringNodeIDs(t *testing.T) {
	var gotWorkspace string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotWorkspace = r.Header.Get(headerWorkspaceID)
		if r.URL.Path != "/newmoi/catalog/tree" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"tree": []any{map[string]any{
					"id":           10,
					"type":         "catalog",
					"name":         "orders",
					"display_name": "Orders",
					"node_list": []any{map[string]any{
						"id":                  "20",
						"type":                "database",
						"name":                "raw",
						"display_description": "User database leaf",
					}},
				}},
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := client.Catalog("ws-1").Tree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotWorkspace != "ws-1" {
		t.Fatalf("X-Workspace-ID = %q", gotWorkspace)
	}
	if tree.GetTree()[0].GetId().GetNumberValue() != 10 {
		t.Fatalf("root id = %#v", tree.GetTree()[0].GetId())
	}
	if tree.GetTree()[0].GetDisplayName() != "Orders" {
		t.Fatalf("display_name = %q", tree.GetTree()[0].GetDisplayName())
	}
	leaf := tree.GetTree()[0].GetNodeList()[0]
	if leaf.GetId().GetStringValue() != "20" {
		t.Fatalf("leaf id = %#v", leaf.GetId())
	}
	if leaf.GetDisplayDescription() != "User database leaf" {
		t.Fatalf("display_description = %q", leaf.GetDisplayDescription())
	}
}

func TestCatalogSystemTreeUsesNameKeyNodes(t *testing.T) {
	var gotWorkspace string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotWorkspace = r.Header.Get(headerWorkspaceID)
		if r.URL.Path != "/newmoi/catalog/system/tree" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"tree": []any{map[string]any{
					"id":           "system",
					"type":         "catalog",
					"name":         "system",
					"display_name": "System",
					"node_list": []any{map[string]any{
						"id":           "system:database:system",
						"type":         "database",
						"name":         "system",
						"display_name": "system",
						"node_list": []any{map[string]any{
							"id":                  "system:table:system:tables",
							"type":                "table",
							"name":                "tables",
							"display_description": "System table leaf",
						}},
					}},
				}},
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := client.Catalog("ws-1").SystemTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotWorkspace != "ws-1" {
		t.Fatalf("X-Workspace-ID = %q", gotWorkspace)
	}
	if tree.GetTree()[0].GetId().GetStringValue() != "system" {
		t.Fatalf("root id = %#v", tree.GetTree()[0].GetId())
	}
	if tree.GetTree()[0].GetDisplayName() != "System" {
		t.Fatalf("display_name = %q", tree.GetTree()[0].GetDisplayName())
	}
	db := tree.GetTree()[0].GetNodeList()[0]
	if db.GetId().GetStringValue() != "system:database:system" {
		t.Fatalf("database id = %#v", db.GetId())
	}
	leaf := db.GetNodeList()[0]
	if leaf.GetId().GetStringValue() != "system:table:system:tables" {
		t.Fatalf("leaf id = %#v", leaf.GetId())
	}
	if leaf.GetDisplayDescription() != "System table leaf" {
		t.Fatalf("display_description = %q", leaf.GetDisplayDescription())
	}
}

func TestCatalogSystemTableLookupAndDataUseDatabaseName(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		responseData := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/table/info":
			responseData = map[string]any{"name": "tables", "columns": []any{map[string]any{"name": "id", "type": "INT"}}}
		case "/newmoi/catalog/table/data", "/newmoi/catalog/table/preview":
			responseData = map[string]any{"data": []any{[]any{"1"}}, "columns": []any{map[string]any{"name": "id", "type": "INT"}}, "total_rows": 1, "page": 1, "page_size": 20}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	ctx := context.Background()
	if _, err := catalog.TableInfo(ctx, 0, WithTableLookupName("tables"), WithTableLookupDatabaseName("system")); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.TableData(ctx, 0, WithTableDataName("tables"), WithTableDataDatabaseName("system"), WithTableDataPage(1), WithTableDataPageSize(20)); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.PreviewTable(ctx, 0, WithTableDataName("tables"), WithTableDataDatabaseName("system"), WithTablePreviewLines(10)); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 3 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/catalog/table/info" ||
		requests[0].body["id"] != nil ||
		requests[0].body["name"] != "tables" ||
		requests[0].body["database_name"] != "system" {
		t.Fatalf("table info = %#v", requests[0])
	}
	if requests[1].path != "/newmoi/catalog/table/data" ||
		requests[1].body["id"] != nil ||
		requests[1].body["name"] != "tables" ||
		requests[1].body["database_name"] != "system" ||
		requests[1].body["page"] != float64(1) ||
		requests[1].body["page_size"] != float64(20) {
		t.Fatalf("table data = %#v", requests[1])
	}
	if requests[2].path != "/newmoi/catalog/table/preview" ||
		requests[2].body["id"] != nil ||
		requests[2].body["name"] != "tables" ||
		requests[2].body["database_name"] != "system" ||
		requests[2].body["preview_lines"] != float64(10) {
		t.Fatalf("preview table = %#v", requests[2])
	}
}

func TestClientUsesEndpointPathAsProductBasePath(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if gotPath != "/tenant/moi/workflow/v2/workflow-apps" {
			t.Fatalf("path = %s", gotPath)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"total": 0, "workflows": []any{}},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/tenant/moi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Workflows("ws-1").List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(gotPath, "/newmoi") {
		t.Fatalf("path still contains legacy product prefix: %s", gotPath)
	}
}

func TestSQLQueryWebSocketURLUsesEndpointPathAsProductBasePath(t *testing.T) {
	client, err := New("https://product.example.test/tenant/moi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.SQL("ws-1").QueryWebSocketURL()
	if err != nil {
		t.Fatal(err)
	}
	want := "wss://product.example.test/tenant/moi/query/ws"
	if got != want {
		t.Fatalf("websocket url = %q, want %q", got, want)
	}
	if strings.Contains(got, "/newmoi") {
		t.Fatalf("websocket url still contains legacy product prefix: %s", got)
	}
}

func TestClientReturnsRawJSONWhenResponseIsNotProductEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":  "business-code",
			"name":  "raw response",
			"items": []any{"one"},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Agents("ws-1").ResourceGet(context.Background(), "agents", "/raw")
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.AsMap()
	if !ok {
		t.Fatalf("response type = %T", resp)
	}
	if data["code"] != "business-code" || data["name"] != "raw response" {
		t.Fatalf("raw response was not preserved: %#v", data)
	}
	items, ok := data["items"].([]any)
	if !ok || len(items) != 1 || items[0] != "one" {
		t.Fatalf("items = %#v", data["items"])
	}
}

// TestClientNeverRetriesHTTP locks the contract that the product SDK issues
// exactly one HTTP attempt for both reads and writes.
func TestClientNeverRetriesHTTP(t *testing.T) {
	var getAttempts, postAttempts int
	client, err := New("https://product.example.test",
		WithAPIKey("user-key"),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.Method {
			case http.MethodGet:
				getAttempts++
			case http.MethodPost:
				postAttempts++
			}
			return nil, errors.New("transport unavailable")
		})}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Workspaces().List(context.Background()); err == nil {
		t.Fatal("GET err is nil")
	}
	if getAttempts != 1 {
		t.Fatalf("GET attempts = %d, want 1", getAttempts)
	}
	if _, err := client.Workspaces().Create(context.Background(), "workspace"); err == nil {
		t.Fatal("POST err is nil")
	}
	if postAttempts != 1 {
		t.Fatalf("POST attempts = %d, want 1", postAttempts)
	}
}

func TestClientStreamHonorsConfiguredTimeoutBeforeHeaders(t *testing.T) {
	client, err := New("https://product.example.test",
		WithAPIKey("user-key"),
		WithTimeout(20*time.Millisecond),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()
			return nil, req.Context().Err()
		})}),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Agents("ws-1").A2AStream(context.Background(), AgentA2ARequest{
		AgentSelector: AgentSelector{AgentCode: "explore"},
		A2ARequest:    A2ARequest{JSONRPC: "2.0", Method: A2AMethodStream},
	})
	if !IsCode(err, "ErrTimeout") {
		t.Fatalf("stream timeout err = %v", err)
	}
}

func TestClientTypedProductResponseRequiresData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Workflows("ws-1").List(context.Background()); !IsCode(err, "ErrResponseInvalid") {
		t.Fatalf("typed list missing data err = %v", err)
	}
}

func TestKnowledgeListSourceJobsParsesViewFields(t *testing.T) {
	var gotPath, gotWorkspace string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotWorkspace = r.Header.Get(headerWorkspaceID)
		if gotPath != "/newmoi/semantic-models/101/source-jobs" {
			t.Fatalf("path = %s", gotPath)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"items": []map[string]any{{
					"job_id":             "job-1",
					"source_id":          "source-1",
					"job_status":         "succeeded",
					"kb_file_id":         "kb-file-1",
					"reconcile_required": true,
				}},
				"total":              1,
				"reconcile_required": true,
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Knowledge("ws-1").ListSourceJobs(context.Background(), "101")
	if err != nil {
		t.Fatal(err)
	}
	if gotWorkspace != "ws-1" {
		t.Fatalf("X-Workspace-ID = %q", gotWorkspace)
	}
	if resp.GetTotal() != 1 || len(resp.GetItems()) != 1 || !resp.GetReconcileRequired() {
		t.Fatalf("response = %+v", resp)
	}
	item := resp.GetItems()[0]
	if item.GetJobId() != "job-1" || item.GetJobStatus() != "succeeded" || item.GetKbFileId() != "kb-file-1" || !item.GetReconcileRequired() {
		t.Fatalf("item = %+v", item)
	}
}

func TestKnowledgeCheckSourceExistenceUsesTypedContract(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/newmoi/semantic-models/101/sources/existence" {
			t.Fatalf("request = %s %s", r.Method, r.URL.EscapedPath())
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"file_ids": []string{"file-1"}, "table_ids": []int64{10}},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Knowledge("ws-1").CheckSourceExistence(context.Background(), "101", []string{"file-1", "file-2"}, []int64{10, 20})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.GetFileIds(), []string{"file-1"}) || !reflect.DeepEqual(result.GetTableIds(), []int64{10}) {
		t.Fatalf("result = %+v", result)
	}
	if !reflect.DeepEqual(gotBody["file_ids"], []any{"file-1", "file-2"}) || !reflect.DeepEqual(gotBody["table_ids"], []any{float64(10), float64(20)}) {
		t.Fatalf("body = %#v", gotBody)
	}
}

func TestKnowledgeLocalFileUploadsUseMultipartRoutes(t *testing.T) {
	type upload struct {
		path      string
		workspace string
		filename  string
		content   string
	}
	var uploads []upload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		content, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		uploads = append(uploads, upload{
			path:      r.URL.EscapedPath(),
			workspace: r.Header.Get(headerWorkspaceID),
			filename:  header.Filename,
			content:   string(content),
		})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"file_id": "file-1"},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	created, err := client.Knowledge("ws-1").UploadLocalFile(context.Background(), " report.pdf ", strings.NewReader("create"))
	if err != nil {
		t.Fatal(err)
	}
	appended, err := client.Knowledge("ws-1").UploadModelLocalFile(context.Background(), "101", "notes.txt", strings.NewReader("append"))
	if err != nil {
		t.Fatal(err)
	}
	if created.GetFileId() != "file-1" || appended.GetFileId() != "file-1" {
		t.Fatalf("results = create:%+v append:%+v", created, appended)
	}
	want := []upload{
		{path: "/newmoi/semantic-models/local-files/upload", workspace: "ws-1", filename: " report.pdf ", content: "create"},
		{path: "/newmoi/semantic-models/101/local-files/upload", workspace: "ws-1", filename: "notes.txt", content: "append"},
	}
	if !reflect.DeepEqual(uploads, want) {
		t.Fatalf("uploads = %#v, want %#v", uploads, want)
	}
}

func TestKnowledgeSearchDrawingsUsesMultipartWithoutFileID(t *testing.T) {
	var gotPath, gotCT, gotKB, gotQuery, gotTopK, gotFilename, gotFile string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotCT = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		gotKB = r.FormValue("knowledge_base_id")
		gotQuery = r.FormValue("query")
		gotTopK = r.FormValue("top_k")
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		content, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		gotFilename = header.Filename
		gotFile = string(content)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"drawings": []any{}, "count": 0, "diagnostics": map[string]any{}},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Knowledge("ws-1").SearchDrawings(context.Background(), DrawingSearchRequest{
		KnowledgeBaseID:    42,
		Query:              "bolt",
		TopK:               3,
		QueryImageFilename: "q.png",
		QueryImageBytes:    []byte("png-bytes"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/newmoi/drawings/search" {
		t.Fatalf("path = %s", gotPath)
	}
	if !strings.HasPrefix(gotCT, "multipart/form-data;") {
		t.Fatalf("content-type = %s", gotCT)
	}
	if gotKB != "42" || gotQuery != "bolt" || gotTopK != "3" || gotFilename != "q.png" || gotFile != "png-bytes" {
		t.Fatalf("form = kb:%s query:%s top_k:%s file:%s %q", gotKB, gotQuery, gotTopK, gotFilename, gotFile)
	}
}

func TestKnowledgeSearchDrawingsSendsDrawingNumberAsJSON(t *testing.T) {
	var gotCT string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"drawings":    []any{},
				"count":       0,
				"match_mode":  "exact",
				"diagnostics": map[string]any{"channels": []any{}},
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Knowledge("ws-1").SearchDrawings(context.Background(), DrawingSearchRequest{
		KnowledgeBaseID: 42,
		DrawingNumber:   "20C710331",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gotCT, "application/json") {
		t.Fatalf("content-type = %s", gotCT)
	}
	if body["drawing_number"] != "20C710331" || body["knowledge_base_id"] != float64(42) {
		t.Fatalf("body = %#v", body)
	}
	if _, ok := body["query"]; ok {
		t.Fatalf("exact search sent query: %#v", body)
	}
	if result.MatchMode != "exact" || result.Count != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestKnowledgeSearchDrawingsKeepsVersionIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":"OK","msg":"OK","data":{
			"count":2,
			"diagnostics":{"channels":["identifier"]},
			"drawings":[
				{"source_file_id":"older","drawing_id":"older","drawing_number":"20C710331","score":1,"best_hit":{"source_file_id":"older","score":1},"supporting_hits":[]},
				{"source_file_id":"current","drawing_id":"current","drawing_number":"20C710331","score":1,"default_version":true,"best_hit":{"source_file_id":"current","score":1},"supporting_hits":[]}
			]
		}}`))
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Knowledge("ws-1").SearchDrawings(t.Context(), DrawingSearchRequest{KnowledgeBaseID: 42, Query: "20C710331"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Drawings) != 2 || result.Drawings[0].DrawingNumber != "20C710331" || result.Drawings[0].DefaultVersion ||
		result.Drawings[1].SourceFileID != "current" || !result.Drawings[1].DefaultVersion || result.Drawings[1].DrawingNumber != "20C710331" {
		t.Fatalf("drawings = %#v", result.Drawings)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var printed map[string]any
	if err := json.Unmarshal(encoded, &printed); err != nil {
		t.Fatal(err)
	}
	drawings, _ := printed["drawings"].([]any)
	if len(drawings) != 2 {
		t.Fatalf("printed = %s", encoded)
	}
	first, _ := drawings[0].(map[string]any)
	second, _ := drawings[1].(map[string]any)
	if first["drawing_number"] != "20C710331" || first["default_version"] != nil || second["drawing_number"] != "20C710331" || second["default_version"] != true {
		t.Fatalf("printed drawings = %#v", drawings)
	}
}

func TestKnowledgeSearchDrawingsSendsNegativeTopK(t *testing.T) {
	var gotTopK float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		gotTopK, _ = body["top_k"].(float64)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "ErrParamInvalid",
			"msg":  "top_k must be a positive integer",
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Knowledge("ws-1").SearchDrawings(context.Background(), DrawingSearchRequest{
		KnowledgeBaseID: 42,
		Query:           "bolt",
		TopK:            -1,
	})
	if gotTopK != -1 {
		t.Fatalf("top_k = %v, want -1", gotTopK)
	}
	if !IsCode(err, "ErrParamInvalid") {
		t.Fatalf("err = %v, want ErrParamInvalid", err)
	}
}

func TestKnowledgeSearchDrawingsRejectsInvalidSuccessPayload(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{name: "empty object", data: `{}`},
		{name: "missing diagnostics", data: `{"drawings":[],"count":0}`},
		{name: "drawings object", data: `{"drawings":{},"count":0,"diagnostics":{}}`},
		{name: "negative count", data: `{"drawings":[],"count":-1,"diagnostics":{}}`},
		{name: "diagnostics array", data: `{"drawings":[],"count":0,"diagnostics":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"code":"OK","msg":"OK","data":` + tc.data + `}`))
			}))
			t.Cleanup(server.Close)

			client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.Knowledge("ws-1").SearchDrawings(context.Background(), DrawingSearchRequest{
				KnowledgeBaseID: 42,
				Query:           "bolt",
			})
			if result != nil {
				t.Fatalf("result = %#v, want nil", result)
			}
			if !IsCode(err, "ErrResponseInvalid") {
				t.Fatalf("err = %v, want ErrResponseInvalid", err)
			}
		})
	}
}

func TestKnowledgeListSourcesPaginationCompatibility(t *testing.T) {
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.EscapedPath() != "/newmoi/semantic-models/101/sources" {
			t.Fatalf("request = %s %s", r.Method, r.URL.EscapedPath())
		}
		queries = append(queries, r.URL.RawQuery)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"items": []any{}, "total": 21, "page": 2, "page_size": 20},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	all, err := client.Knowledge("ws-1").ListSources(context.Background(), "101")
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.Knowledge("ws-1").ListSourcesPage(context.Background(), "101", 2, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(queries, []string{"", "page=2&page_size=20"}) {
		t.Fatalf("queries = %#v", queries)
	}
	if all.GetTotal() != 21 || page.GetPage() != 2 || page.GetPageSize() != 20 {
		t.Fatalf("results = all:%+v page:%+v", all, page)
	}
}

func TestKnowledgeListReferencingAgents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/newmoi/semantic-models/101/referencing-agents" || r.URL.Query().Get("limit") != "10" || r.URL.Query().Get("cursor") != "next" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK", "msg": "OK", "data": map[string]any{
				"items": []any{map[string]any{"id": "ag_1", "workspace_id": "ws-1", "name": "Agent", "status": "active", "knowledge_base_count": 2}},
				"limit": 10,
			},
		})
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.Knowledge("ws-1").ListReferencingAgents(t.Context(), "101", 10, "next")
	if err != nil || len(page.GetItems()) != 1 || page.GetItems()[0].GetKnowledgeBaseCount() != 2 {
		t.Fatalf("referencing agents = %+v, err = %v", page, err)
	}
}

func TestKnowledgePreviewSourceSelectionCountsUsesTypedContract(t *testing.T) {
	type request struct {
		path string
		body map[string]any
	}
	var requests []request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, request{path: r.URL.EscapedPath(), body: body})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"file_count": 140, "table_count": 0, "total_count": 140},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	selections := []SemanticModelSourceSelectionInput{{
		Kind:            "volume_files",
		VolumeID:        42,
		AllSelected:     true,
		ExcludedFileIDs: []string{"file-95"},
		Filters: SemanticModelSourceSelectionFilters{
			FileName: "report",
			FileExt:  []string{"pdf"},
		},
	}}
	createResult, err := client.Knowledge("ws-1").PreviewSourceSelectionCounts(context.Background(), selections)
	if err != nil {
		t.Fatal(err)
	}
	modelResult, err := client.Knowledge("ws-1").PreviewModelSourceSelectionCounts(context.Background(), "101", selections)
	if err != nil {
		t.Fatal(err)
	}
	if createResult.GetTotalCount() != 140 || modelResult.GetFileCount() != 140 {
		t.Fatalf("results = create:%+v model:%+v", createResult, modelResult)
	}
	if len(requests) != 2 || requests[0].path != "/newmoi/semantic-models/source-selections/preview" || requests[1].path != "/newmoi/semantic-models/101/source-selections/preview" {
		t.Fatalf("requests = %#v", requests)
	}
	want := map[string]any{"source_selections": []any{map[string]any{
		"kind": "volume_files", "volume_id": float64(42), "all_selected": true,
		"excluded_file_ids": []any{"file-95"},
		"filters":           map[string]any{"file_name": "report", "file_ext": []any{"pdf"}},
	}}}
	if !reflect.DeepEqual(requests[0].body, want) || !reflect.DeepEqual(requests[1].body, want) {
		t.Fatalf("bodies = %#v", requests)
	}
}

func TestKnowledgeAppendSelectionsPreservesDuplicateFeedback(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/newmoi/semantic-models/101/sources" || r.Header.Get("X-Workspace-ID") != "ws-1" {
			t.Errorf("unexpected selection request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"OK","msg":"OK","data":{"sources":[],"jobs":[],"skipped_existing_file_ids":["file-existing"],"skipped_existing_table_ids":[42]}}`))
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Knowledge("ws-1").AppendSources(t.Context(), "101", nil,
		WithSemanticModelAppendSourceSelections(SemanticModelSourceSelectionInput{Kind: "volume_files", VolumeID: 7, SelectedFileIDs: []string{"file-existing", "file-new"}}))
	if err != nil {
		t.Fatal(err)
	}
	selection := map[string]any{"kind": "volume_files", "volume_id": float64(7), "all_selected": false, "selected_file_ids": []any{"file-existing", "file-new"}}
	if !reflect.DeepEqual(received["source_selections"], []any{selection}) {
		t.Fatalf("source selections=%#v", received)
	}
	if !reflect.DeepEqual(result.GetSkippedExistingFileIds(), []string{"file-existing"}) || !reflect.DeepEqual(result.GetSkippedExistingTableIds(), []int64{42}) {
		t.Fatalf("duplicate feedback was lost: %v", result)
	}
}

func TestCatalogDatabaseTableLeavesUsesPaginationOptions(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/newmoi/catalog/database/table-leaves" {
			t.Fatalf("request = %s %s", r.Method, r.URL.EscapedPath())
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"items":           []map[string]any{{"table_id": 10, "table_name": "orders", "database_id": 20}},
				"total":           2,
				"next_page_token": "10",
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Catalog("ws-1").DatabaseTableLeaves(
		context.Background(),
		20,
		WithDatabaseTableLeavesPageSize(25),
		WithDatabaseTableLeavesPageToken("5"),
		WithDatabaseTableLeavesSearch("ord"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.GetTotal() != 2 || result.GetNextPageToken() != "10" || len(result.GetItems()) != 1 || result.GetItems()[0].GetTableName() != "orders" {
		t.Fatalf("result = %+v", result)
	}
	if gotBody["database_id"] != float64(20) || gotBody["page_size"] != float64(25) || gotBody["page_token"] != "5" || gotBody["search"] != "ord" {
		t.Fatalf("body = %#v", gotBody)
	}
}

func TestCatalogSearchDataDashboardTablesUsesPaginationOptions(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/newmoi/catalog/data-dashboard/table-search" {
			t.Fatalf("request = %s %s", r.Method, r.URL.EscapedPath())
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"items": []map[string]any{{
					"table_id": 10, "table_name": "orders", "database_id": 20,
					"database_name": "sales", "catalog_id": 30, "catalog_name": "internal",
					"description": "orders table", "matched_fields": []string{"table_name"},
					"matched_columns": []string{"order_id"}, "metadata_match": true,
				}},
				"total": 2, "next_page_token": "10",
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Catalog("ws-1").SearchDataDashboardTables(
		context.Background(),
		WithDataDashboardTableSearchQuery("ord"),
		WithDataDashboardTableSearchPageSize(25),
		WithDataDashboardTableSearchPageToken("5"),
	)
	if err != nil {
		t.Fatal(err)
	}
	item := result.GetItems()[0]
	if result.GetTotal() != 2 || result.GetNextPageToken() != "10" || item.GetDatabaseName() != "sales" || item.GetMatchedColumns()[0] != "order_id" || !item.GetMetadataMatch() {
		t.Fatalf("result = %+v", result)
	}
	if gotBody["page_size"] != float64(25) || gotBody["page_token"] != "5" || gotBody["search"] != "ord" {
		t.Fatalf("body = %#v", gotBody)
	}
}

func TestKnowledgeSemanticModelSourceCountsAndMissingFieldCompatibility(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data map[string]any
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/semantic-models":
			data = map[string]any{
				"items": []map[string]any{{
					"id":   101,
					"name": "orders-model",
					"source_counts": map[string]any{
						"files":  2,
						"tables": 1,
						"total":  3,
					},
				}},
				"total": 1,
			}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/semantic-models":
			data = map[string]any{
				"id":   101,
				"name": "orders-model",
				"source_counts": map[string]any{
					"files":  2,
					"tables": 1,
					"total":  3,
				},
			}
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/semantic-models/101":
			data = map[string]any{"id": 101, "name": "orders-model"}
		default:
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}

	list, err := client.Knowledge("ws-1").List(context.Background())
	if err != nil {
		t.Fatalf("list semantic models err = %v", err)
	}
	if len(list.GetItems()) != 1 {
		t.Fatalf("list items = %d", len(list.GetItems()))
	}
	listCounts := list.GetItems()[0].GetSourceCounts()
	if listCounts == nil || listCounts.GetFiles() != 2 || listCounts.GetTables() != 1 || listCounts.GetTotal() != 3 {
		t.Fatalf("list source counts = %#v", listCounts)
	}

	created, err := client.Knowledge("ws-1").Create(context.Background(), "orders-model")
	if err != nil {
		t.Fatalf("create semantic model err = %v", err)
	}
	createdCounts := created.GetSourceCounts()
	if createdCounts == nil || createdCounts.GetFiles() != 2 || createdCounts.GetTables() != 1 || createdCounts.GetTotal() != 3 {
		t.Fatalf("create source counts = %#v", createdCounts)
	}

	model, err := client.Knowledge("ws-1").Get(context.Background(), "101")
	if err != nil {
		t.Fatalf("get semantic model without source counts err = %v", err)
	}
	if model.GetSourceCounts() != nil {
		t.Fatalf("missing source counts = %#v", model.GetSourceCounts())
	}
}

func TestWorkspaceRequired(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Workflows("").List(context.Background())
	if !IsCode(err, ErrWorkspaceRequired.Code) {
		t.Fatalf("err = %v", err)
	}
}

func TestWorkspaceCreateUsesOptionsStyleBody(t *testing.T) {
	var requests []struct {
		method string
		path   string
		body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method string
			path   string
			body   map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), body: body})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"id": "workspace 1"},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	created, err := client.Workspaces().Create(context.Background(), "workspace 1")
	if err != nil {
		t.Fatal(err)
	}
	if created.GetId() != "workspace 1" {
		t.Fatalf("created workspace id = %q", created.GetId())
	}

	if len(requests) != 1 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].method != http.MethodPost || requests[0].path != "/newmoi/workspaces" || requests[0].body["name"] != "workspace 1" {
		t.Fatalf("create request = %#v", requests[0])
	}
}

func TestWorkspaceCreateRequiresName(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Workspaces().Create(context.Background(), ""); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("create err = %v", err)
	}
}

func TestWorkspaceClaimUsesExplicitArguments(t *testing.T) {
	var requests []struct {
		method string
		path   string
		body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method string
			path   string
			body   map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), body: body})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"workspace_id":    "workspace 1",
				"claimed":         true,
				"already_claimed": false,
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := client.Workspaces().Claim(context.Background(), "workspace 1")
	if err != nil {
		t.Fatal(err)
	}
	if claimed.GetWorkspaceId() != "workspace 1" || !claimed.GetClaimed() || claimed.GetAlreadyClaimed() {
		t.Fatalf("claim result = %#v", claimed)
	}
	if len(requests) != 1 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].method != http.MethodPost || requests[0].path != "/newmoi/workspaces/claim" ||
		requests[0].body["workspace_id"] != "workspace 1" {
		t.Fatalf("claim request = %#v", requests[0])
	}
}

func TestEnsureWorkspaceFindsExistingWorkspace(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		if r.URL.Path != "/newmoi/workspaces" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"total": 1,
				"workspaces": []map[string]any{{
					"id":         "ws-existing",
					"name":       "analytics",
					"status":     "ACTIVE",
					"owner_id":   "user-1",
					"created_at": "2026-06-26T00:00:00Z",
				}},
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.EnsureWorkspace(context.Background(), "analytics")
	if err != nil {
		t.Fatal(err)
	}
	if workspace.ID() != "ws-existing" || workspace.Name() != "analytics" {
		t.Fatalf("workspace = %#v", workspace)
	}
	if len(requests) != 1 || !strings.HasPrefix(requests[0], "GET /newmoi/workspaces") {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestEnsureWorkspaceRejectsDuplicateWorkspaceName(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		if r.URL.Path != "/newmoi/workspaces" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"total": 2,
				"workspaces": []map[string]any{
					{
						"id":         "ws-1",
						"name":       "analytics",
						"status":     "ACTIVE",
						"owner_id":   "user-1",
						"created_at": "2026-06-26T00:00:00Z",
					},
					{
						"id":         "ws-2",
						"name":       "analytics",
						"status":     "ACTIVE",
						"owner_id":   "user-2",
						"created_at": "2026-06-26T00:00:00Z",
					},
				},
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.EnsureWorkspace(context.Background(), "analytics")
	if !IsCode(err, "ErrDuplicateName") {
		t.Fatalf("error = %v", err)
	}
	if len(requests) != 1 || !strings.HasPrefix(requests[0], "GET /newmoi/workspaces") {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestEnsureWorkspaceCreatesMissingWorkspace(t *testing.T) {
	var requests []struct {
		method string
		path   string
		body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method string
			path   string
			body   map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), body: body})
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": "OK",
				"msg":  "OK",
				"data": map[string]any{"total": 0, "workspaces": []any{}},
			})
		case http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": "OK",
				"msg":  "OK",
				"data": map[string]any{"id": "ws-created"},
			})
		default:
			t.Fatalf("method = %s", r.Method)
		}
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.EnsureWorkspace(context.Background(), "analytics")
	if err != nil {
		t.Fatal(err)
	}
	if workspace.ID() != "ws-created" || workspace.Name() != "analytics" {
		t.Fatalf("workspace = %#v", workspace)
	}
	if len(requests) != 2 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].method != http.MethodGet || requests[0].path != "/newmoi/workspaces" {
		t.Fatalf("list request = %#v", requests[0])
	}
	if requests[1].method != http.MethodPost || requests[1].path != "/newmoi/workspaces" || requests[1].body["name"] != "analytics" {
		t.Fatalf("create request = %#v", requests[1])
	}
}

func TestWorkspaceHandleScopesServices(t *testing.T) {
	var requests []struct {
		path      string
		workspace string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, struct {
			path      string
			workspace string
		}{path: r.URL.EscapedPath(), workspace: r.Header.Get(headerWorkspaceID)})
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": "OK",
				"msg":  "OK",
				"data": map[string]any{"status": "ok"},
			})
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.Catalog().Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 {
		t.Fatalf("request count = %d", len(requests))
	}
	for _, request := range requests {
		if request.workspace != "ws-1" {
			t.Fatalf("X-Workspace-ID = %q for %#v", request.workspace, request)
		}
	}
	if requests[0].path != "/newmoi/catalog/healthz" {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestWorkspaceHandleManagesWorkspaceLifecycle(t *testing.T) {
	var requests []struct {
		method string
		path   string
		body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method string
			path   string
			body   map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), body: body})
		switch {
		case r.Method == http.MethodPatch:
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK"})
		case r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": "OK",
				"msg":  "OK",
				"data": map[string]any{"total": 0, "user_list": []any{}},
			})
		case r.Method == http.MethodDelete:
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK"})
		default:
			t.Fatalf("method = %s", r.Method)
		}
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("workspace 1")
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 1 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].method != http.MethodDelete || requests[0].path != "/newmoi/workspaces/workspace%201" {
		t.Fatalf("delete request = %#v", requests[0])
	}
}

func TestWorkspaceHandleAuditConvenience(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		rawQuery  string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method    string
			path      string
			rawQuery  string
			workspace string
			body      map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), rawQuery: r.URL.RawQuery, workspace: r.Header.Get(headerWorkspaceID), body: body})
		var responseData any
		switch r.URL.EscapedPath() {
		case "/newmoi/audit/logs":
			responseData = map[string]any{
				"items": []map[string]any{{
					"id":           "log-1",
					"workspace_id": "ws-1",
					"method":       "POST",
					"path":         "/newmoi/catalog/file/create",
					"status_code":  500,
					"result":       "failed",
					"created_at":   "2026-06-26T00:00:00Z",
				}},
				"total":     1,
				"page":      1,
				"page_size": 5,
			}
		default:
			t.Fatalf("path = %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": responseData})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	logs, err := workspace.RecentFailedAuditLogs(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if logs.GetTotal() != 1 || len(logs.GetItems()) != 1 || logs.GetItems()[0].GetResult() != "failed" {
		t.Fatalf("logs = %#v", logs)
	}

	if len(requests) != 1 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].method != http.MethodGet || requests[0].path != "/newmoi/audit/logs" || requests[0].workspace != "ws-1" {
		t.Fatalf("audit request = %#v", requests[0])
	}
	if requests[0].rawQuery != "page=1&page_size=5&result=failed" {
		t.Fatalf("audit query = %s", requests[0].rawQuery)
	}
}

func TestWorkspaceHandlePreparesCatalogAndDatabase(t *testing.T) {
	var requests []struct {
		path      string
		workspace string
		body      map[string]any
		filename  string
		fileData  string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		var filename, fileData string
		if strings.HasPrefix(r.Header.Get(headerContentType), "multipart/form-data; boundary=") {
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			filename = header.Filename
			data, err := io.ReadAll(file)
			if err != nil {
				t.Fatal(err)
			}
			fileData = string(data)
		} else if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path      string
			workspace string
			body      map[string]any
			filename  string
			fileData  string
		}{path: r.URL.EscapedPath(), workspace: r.Header.Get(headerWorkspaceID), body: body, filename: filename, fileData: fileData})
		var responseData any
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/list":
			responseData = map[string]any{"list": []map[string]any{{"id": 10, "name": "orders", "display_name": "Orders", "description": "existing", "display_description": "Existing catalog", "allowed_actions": []string{"catalog.update_catalog"}}}}
		case "/newmoi/catalog/database/list":
			responseData = map[string]any{"list": []any{}}
		case "/newmoi/catalog/database/create":
			responseData = map[string]any{"id": 20}
		case "/newmoi/catalog/database/children":
			responseData = map[string]any{"list": []map[string]any{{"id": "30", "name": "landing", "type": "volume"}}}
		case "/newmoi/catalog/file/upload":
			responseData = map[string]any{"file_id": "file-1"}
		case "/newmoi/catalog/file/create":
			responseData = nil
		case "/newmoi/catalog/file/info":
			responseData = map[string]any{"file_id": "file-1", "file_name": "orders.csv", "size": 16}
		case "/newmoi/catalog/file/preview_stream":
			_, _ = w.Write([]byte("preview-body"))
			return
		case "/newmoi/catalog/file/trigger_workflow":
			responseData = map[string]any{"triggered": 1}
		case "/newmoi/catalog/file/delete_ref":
			responseData = nil
		case "/newmoi/catalog/file/delete":
			responseData = nil
		default:
			t.Fatalf("path = %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	volume, err := workspace.PrepareDataVolume(context.Background(), "orders", "raw", "landing")
	if err != nil {
		t.Fatal(err)
	}
	if volume.ID() != 30 || volume.Name() != "landing" {
		t.Fatalf("volume = %#v", volume)
	}
	if volume.database.ID() != 20 || volume.database.Name() != "raw" {
		t.Fatalf("database = %#v", volume.database)
	}
	if volume.database.catalog.ID() != 10 || volume.database.catalog.Name() != "orders" {
		t.Fatalf("catalog = %#v", volume.database.catalog)
	}
	uploaded, err := volume.UploadBytes(context.Background(), "orders.csv", []byte("id,name\n1,alice\n"))
	if err != nil {
		t.Fatal(err)
	}
	if uploaded.GetFileId() != "file-1" {
		t.Fatalf("uploaded = %#v", uploaded)
	}
	localPath := filepath.Join(t.TempDir(), "local-orders.csv")
	if err := os.WriteFile(localPath, []byte("id,name\n2,bob\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	localUploaded, err := volume.UploadFile(context.Background(), localPath)
	if err != nil {
		t.Fatal(err)
	}
	if localUploaded.GetFileId() != "file-1" {
		t.Fatalf("local uploaded = %#v", localUploaded)
	}
	file, err := volume.UploadBytesHandle(context.Background(), "orders.csv", []byte("id,name\n1,alice\n"))
	if err != nil {
		t.Fatal(err)
	}
	if file.ID() != "file-1" {
		t.Fatalf("file = %#v", file)
	}
	info, err := file.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.GetFileName() != "orders.csv" {
		t.Fatalf("file info = %#v", info)
	}
	triggered, err := file.TriggerWorkflows(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if triggered.GetTriggered() != 1 {
		t.Fatalf("triggered = %#v", triggered)
	}
	if err := file.DeleteRef(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := file.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}

	localFile, err := volume.UploadFileHandle(context.Background(), localPath)
	if err != nil {
		t.Fatal(err)
	}
	if localFile.ID() != "file-1" {
		t.Fatalf("local file = %#v", localFile)
	}
	volumePreview, err := volume.UploadBytesPreview(context.Background(), "preview.csv", []byte("id,name\n3,carol\n"))
	if err != nil {
		t.Fatal(err)
	}
	if volumePreview.File.ID() != "file-1" || volumePreview.Preview == nil || volumePreview.Preview.Body == nil {
		t.Fatalf("volume preview = %#v", volumePreview)
	}
	_ = volumePreview.Preview.Body.Close()
	localVolumePreview, err := volume.UploadFilePreview(context.Background(), localPath)
	if err != nil {
		t.Fatal(err)
	}
	if localVolumePreview.File.ID() != "file-1" || localVolumePreview.Preview == nil || localVolumePreview.Preview.Body == nil {
		t.Fatalf("local volume preview = %#v", localVolumePreview)
	}
	_ = localVolumePreview.Preview.Body.Close()

	if len(requests) != 22 {
		t.Fatalf("request count = %d", len(requests))
	}
	for _, request := range requests {
		if request.workspace != "ws-1" {
			t.Fatalf("X-Workspace-ID = %q for %#v", request.workspace, request)
		}
	}
	if requests[0].path != "/newmoi/catalog/list" {
		t.Fatalf("catalog list request = %#v", requests[0])
	}
	if requests[1].path != "/newmoi/catalog/database/list" || requests[1].body["id"] != float64(10) {
		t.Fatalf("database list request = %#v", requests[1])
	}
	if requests[2].path != "/newmoi/catalog/database/create" || requests[2].body["catalog_id"] != float64(10) ||
		requests[2].body["name"] != "raw" {
		t.Fatalf("database create request = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/catalog/database/children" || requests[3].body["id"] != float64(20) {
		t.Fatalf("database children request = %#v", requests[3])
	}
	if requests[4].path != "/newmoi/catalog/file/upload" || requests[4].filename != "orders.csv" || requests[4].fileData != "id,name\n1,alice\n" {
		t.Fatalf("file upload request = %#v", requests[4])
	}
	if requests[5].path != "/newmoi/catalog/file/create" || requests[5].body["volume_id"] != float64(30) {
		t.Fatalf("file create request = %#v", requests[5])
	}
	fileIDs, ok := requests[5].body["file_ids"].([]any)
	if !ok || len(fileIDs) != 1 || fileIDs[0] != "file-1" {
		t.Fatalf("file create file_ids = %#v", requests[5].body["file_ids"])
	}
	if requests[6].path != "/newmoi/catalog/file/upload" || requests[6].filename != "local-orders.csv" || requests[6].fileData != "id,name\n2,bob\n" {
		t.Fatalf("local file upload request = %#v", requests[6])
	}
	if requests[7].path != "/newmoi/catalog/file/create" || requests[7].body["volume_id"] != float64(30) {
		t.Fatalf("local file create request = %#v", requests[7])
	}
	if requests[8].path != "/newmoi/catalog/file/upload" || requests[8].filename != "orders.csv" || requests[8].fileData != "id,name\n1,alice\n" {
		t.Fatalf("file handle upload request = %#v", requests[8])
	}
	if requests[9].path != "/newmoi/catalog/file/create" || requests[9].body["volume_id"] != float64(30) {
		t.Fatalf("file handle create request = %#v", requests[9])
	}
	if requests[10].path != "/newmoi/catalog/file/info" || requests[10].body["file_id"] != "file-1" {
		t.Fatalf("file info request = %#v", requests[10])
	}
	if requests[11].path != "/newmoi/catalog/file/trigger_workflow" || requests[11].body["volume_id"] != float64(30) {
		t.Fatalf("file trigger request = %#v", requests[11])
	}
	triggerFileIDs, ok := requests[11].body["file_ids"].([]any)
	if !ok || len(triggerFileIDs) != 1 || triggerFileIDs[0] != "file-1" {
		t.Fatalf("file trigger file_ids = %#v", requests[11].body["file_ids"])
	}
	if requests[12].path != "/newmoi/catalog/file/delete_ref" || requests[12].body["volume_id"] != float64(30) {
		t.Fatalf("file delete ref request = %#v", requests[12])
	}
	deleteFileIDs, ok := requests[12].body["file_ids"].([]any)
	if !ok || len(deleteFileIDs) != 1 || deleteFileIDs[0] != "file-1" {
		t.Fatalf("file delete ref file_ids = %#v", requests[12].body["file_ids"])
	}
	if requests[13].path != "/newmoi/catalog/file/delete" ||
		requests[13].body["id"] != "file-1" || requests[13].body["volume_id"] != "30" {
		t.Fatalf("file delete request = %#v", requests[13])
	}
	if requests[14].path != "/newmoi/catalog/file/upload" || requests[14].filename != "local-orders.csv" || requests[14].fileData != "id,name\n2,bob\n" {
		t.Fatalf("local file handle upload request = %#v", requests[14])
	}
	if requests[15].path != "/newmoi/catalog/file/create" || requests[15].body["volume_id"] != float64(30) {
		t.Fatalf("local file handle create request = %#v", requests[15])
	}
	if requests[16].path != "/newmoi/catalog/file/upload" || requests[16].filename != "preview.csv" || requests[16].fileData != "id,name\n3,carol\n" {
		t.Fatalf("volume preview upload request = %#v", requests[16])
	}
	if requests[17].path != "/newmoi/catalog/file/create" || requests[17].body["volume_id"] != float64(30) {
		t.Fatalf("volume preview create request = %#v", requests[17])
	}
	if requests[18].path != "/newmoi/catalog/file/preview_stream" || requests[18].body["file_id"] != "file-1" {
		t.Fatalf("volume preview stream request = %#v", requests[18])
	}
	if requests[19].path != "/newmoi/catalog/file/upload" || requests[19].filename != "local-orders.csv" || requests[19].fileData != "id,name\n2,bob\n" {
		t.Fatalf("local volume preview upload request = %#v", requests[19])
	}
	if requests[20].path != "/newmoi/catalog/file/create" || requests[20].body["volume_id"] != float64(30) {
		t.Fatalf("local volume preview create request = %#v", requests[20])
	}
	if requests[21].path != "/newmoi/catalog/file/preview_stream" || requests[21].body["file_id"] != "file-1" {
		t.Fatalf("local volume preview stream request = %#v", requests[21])
	}
}

func TestWorkspaceHandleUploadsToDataVolume(t *testing.T) {
	var requests []struct {
		path      string
		workspace string
		body      map[string]any
		filename  string
		fileData  string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		var filename, fileData string
		if strings.HasPrefix(r.Header.Get(headerContentType), "multipart/form-data; boundary=") {
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			filename = header.Filename
			data, err := io.ReadAll(file)
			if err != nil {
				t.Fatal(err)
			}
			fileData = string(data)
		} else if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path      string
			workspace string
			body      map[string]any
			filename  string
			fileData  string
		}{path: r.URL.EscapedPath(), workspace: r.Header.Get(headerWorkspaceID), body: body, filename: filename, fileData: fileData})

		var responseData any
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/list":
			responseData = map[string]any{"list": []map[string]any{{"id": 10, "name": "orders"}}}
		case "/newmoi/catalog/database/list":
			responseData = map[string]any{"list": []map[string]any{{"id": 20, "name": "raw"}}}
		case "/newmoi/catalog/database/children":
			responseData = map[string]any{"list": []map[string]any{{"id": "30", "name": "landing", "type": "volume"}}}
		case "/newmoi/catalog/file/upload":
			responseData = map[string]any{"file_id": "file-1"}
		case "/newmoi/catalog/file/create":
			responseData = nil
		case "/newmoi/catalog/file/preview_stream":
			_, _ = w.Write([]byte("preview-body"))
			return
		case "/newmoi/catalog/volume/full_path":
			responseData = map[string]any{"path": "orders/raw/landing"}
		case "/newmoi/catalog/volume/ref_list":
			responseData = map[string]any{"list": []map[string]any{{"ref_id": "wf-1", "ref_type": "workflow"}}}
		case "/newmoi/catalog/update":
			responseData = nil
		case "/newmoi/catalog/database/update":
			responseData = map[string]any{"id": 20}
		case "/newmoi/catalog/volume/update":
			responseData = map[string]any{"id": 30}
		case "/newmoi/catalog/info":
			responseData = map[string]any{"catalog": map[string]any{"id": 10, "name": "orders-renamed", "description": "renamed catalog"}}
		case "/newmoi/catalog/database/info":
			responseData = map[string]any{"database": map[string]any{"id": 20, "name": "raw_renamed", "description": "renamed database"}}
		case "/newmoi/catalog/volume/info":
			responseData = map[string]any{"volume": map[string]any{"id": 30, "name": "landing_renamed", "description": "renamed volume"}}
		case "/newmoi/catalog/ref_list":
			responseData = map[string]any{"list": []map[string]any{{"ref_id": "db-1", "ref_type": "database"}}}
		case "/newmoi/catalog/database/ref_list":
			responseData = map[string]any{"list": []map[string]any{{"ref_id": "volume-1", "ref_type": "volume"}}}
		case "/newmoi/catalog/volume/delete":
			responseData = nil
		case "/newmoi/catalog/database/delete":
			responseData = nil
		case "/newmoi/catalog/delete":
			responseData = nil
		default:
			t.Fatalf("path = %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	file, err := workspace.UploadBytesToDataVolume(context.Background(), "orders", "raw", "landing", "orders.csv", []byte("id,name\n1,alice\n"))
	if err != nil {
		t.Fatal(err)
	}
	if file.ID() != "file-1" {
		t.Fatalf("file = %#v", file)
	}
	localPath := filepath.Join(t.TempDir(), "local-orders.csv")
	if err := os.WriteFile(localPath, []byte("id,name\n2,bob\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	localFile, err := workspace.UploadFileToDataVolume(context.Background(), "orders", "raw", "landing", localPath)
	if err != nil {
		t.Fatal(err)
	}
	if localFile.ID() != "file-1" {
		t.Fatalf("local file = %#v", localFile)
	}
	preview, err := workspace.UploadBytesPreviewToDataVolume(context.Background(), "orders", "raw", "landing", "preview.csv", []byte("id,name\n3,carol\n"))
	if err != nil {
		t.Fatal(err)
	}
	if preview.File.ID() != "file-1" || preview.Preview == nil || preview.Preview.Body == nil {
		t.Fatalf("preview = %#v", preview)
	}
	_ = preview.Preview.Body.Close()
	localPreview, err := workspace.UploadFilePreviewToDataVolume(context.Background(), "orders", "raw", "landing", localPath)
	if err != nil {
		t.Fatal(err)
	}
	if localPreview.File.ID() != "file-1" || localPreview.Preview == nil || localPreview.Preview.Body == nil {
		t.Fatalf("local preview = %#v", localPreview)
	}
	_ = localPreview.Preview.Body.Close()
	selected, err := workspace.PrepareDataVolume(context.Background(), "orders", "raw", "landing")
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID() != 30 {
		t.Fatalf("selected volume = %#v", selected)
	}
	fullPath, err := selected.FullPath(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fullPath.GetPath() != "orders/raw/landing" {
		t.Fatalf("full path = %#v", fullPath)
	}
	namedFullPath, err := workspace.DataVolumeFullPath(context.Background(), "orders", "raw", "landing")
	if err != nil {
		t.Fatal(err)
	}
	if namedFullPath.GetPath() != "orders/raw/landing" {
		t.Fatalf("named full path = %#v", namedFullPath)
	}
	renamedCatalog, err := selected.database.catalog.Rename(context.Background(), "orders-renamed", WithCatalogDescription("renamed catalog"))
	if err != nil {
		t.Fatal(err)
	}
	if renamedCatalog != selected.database.catalog || renamedCatalog.Name() != "orders-renamed" {
		t.Fatalf("renamed catalog = %#v", renamedCatalog)
	}
	renamedDatabase, err := selected.database.UpdateDescription(context.Background(), "renamed database")
	if err != nil {
		t.Fatal(err)
	}
	if renamedDatabase != selected.database {
		t.Fatalf("updated database = %#v", renamedDatabase)
	}
	renamedVolume, err := selected.Rename(context.Background(), "landing_renamed", WithCatalogDescription("renamed volume"))
	if err != nil {
		t.Fatal(err)
	}
	if renamedVolume != selected || renamedVolume.Name() != "landing_renamed" {
		t.Fatalf("renamed volume = %#v", renamedVolume)
	}
	catalogInfo, err := renamedCatalog.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if catalogInfo.GetCatalog().GetName() != "orders-renamed" {
		t.Fatalf("catalog info = %#v", catalogInfo)
	}
	databaseInfo, err := renamedDatabase.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if databaseInfo.GetDatabase().GetName() != "raw_renamed" {
		t.Fatalf("database info = %#v", databaseInfo)
	}
	volumeInfo, err := renamedVolume.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if volumeInfo.GetVolume().GetName() != "landing_renamed" {
		t.Fatalf("volume info = %#v", volumeInfo)
	}
	if err := selected.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := selected.database.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := selected.database.catalog.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 39 {
		t.Fatalf("request count = %d", len(requests))
	}
	for _, request := range requests {
		if request.workspace != "ws-1" {
			t.Fatalf("X-Workspace-ID = %q for %#v", request.workspace, request)
		}
	}
	if requests[0].path != "/newmoi/catalog/list" {
		t.Fatalf("catalog list request = %#v", requests[0])
	}
	if requests[1].path != "/newmoi/catalog/database/list" || requests[1].body["id"] != float64(10) {
		t.Fatalf("database list request = %#v", requests[1])
	}
	if requests[2].path != "/newmoi/catalog/database/children" || requests[2].body["id"] != float64(20) {
		t.Fatalf("database children request = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/catalog/file/upload" || requests[3].filename != "orders.csv" || requests[3].fileData != "id,name\n1,alice\n" {
		t.Fatalf("file upload request = %#v", requests[3])
	}
	if requests[4].path != "/newmoi/catalog/file/create" || requests[4].body["volume_id"] != float64(30) {
		t.Fatalf("file create request = %#v", requests[4])
	}
	if requests[5].path != "/newmoi/catalog/list" {
		t.Fatalf("local catalog list request = %#v", requests[5])
	}
	if requests[6].path != "/newmoi/catalog/database/list" || requests[6].body["id"] != float64(10) {
		t.Fatalf("local database list request = %#v", requests[6])
	}
	if requests[7].path != "/newmoi/catalog/database/children" || requests[7].body["id"] != float64(20) {
		t.Fatalf("local database children request = %#v", requests[7])
	}
	if requests[8].path != "/newmoi/catalog/file/upload" || requests[8].filename != "local-orders.csv" || requests[8].fileData != "id,name\n2,bob\n" {
		t.Fatalf("local file upload request = %#v", requests[8])
	}
	if requests[9].path != "/newmoi/catalog/file/create" || requests[9].body["volume_id"] != float64(30) {
		t.Fatalf("local file create request = %#v", requests[9])
	}
	if requests[10].path != "/newmoi/catalog/list" {
		t.Fatalf("preview catalog list request = %#v", requests[10])
	}
	if requests[11].path != "/newmoi/catalog/database/list" || requests[11].body["id"] != float64(10) {
		t.Fatalf("preview database list request = %#v", requests[11])
	}
	if requests[12].path != "/newmoi/catalog/database/children" || requests[12].body["id"] != float64(20) {
		t.Fatalf("preview database children request = %#v", requests[12])
	}
	if requests[13].path != "/newmoi/catalog/file/upload" || requests[13].filename != "preview.csv" || requests[13].fileData != "id,name\n3,carol\n" {
		t.Fatalf("preview file upload request = %#v", requests[13])
	}
	if requests[14].path != "/newmoi/catalog/file/create" || requests[14].body["volume_id"] != float64(30) {
		t.Fatalf("preview file create request = %#v", requests[14])
	}
	if requests[15].path != "/newmoi/catalog/file/preview_stream" || requests[15].body["file_id"] != "file-1" {
		t.Fatalf("preview stream request = %#v", requests[15])
	}
	if requests[16].path != "/newmoi/catalog/list" {
		t.Fatalf("local preview catalog list request = %#v", requests[16])
	}
	if requests[17].path != "/newmoi/catalog/database/list" || requests[17].body["id"] != float64(10) {
		t.Fatalf("local preview database list request = %#v", requests[17])
	}
	if requests[18].path != "/newmoi/catalog/database/children" || requests[18].body["id"] != float64(20) {
		t.Fatalf("local preview database children request = %#v", requests[18])
	}
	if requests[19].path != "/newmoi/catalog/file/upload" || requests[19].filename != "local-orders.csv" || requests[19].fileData != "id,name\n2,bob\n" {
		t.Fatalf("local preview file upload request = %#v", requests[19])
	}
	if requests[20].path != "/newmoi/catalog/file/create" || requests[20].body["volume_id"] != float64(30) {
		t.Fatalf("local preview file create request = %#v", requests[20])
	}
	if requests[21].path != "/newmoi/catalog/file/preview_stream" || requests[21].body["file_id"] != "file-1" {
		t.Fatalf("local preview stream request = %#v", requests[21])
	}
	if requests[22].path != "/newmoi/catalog/list" {
		t.Fatalf("selected catalog list request = %#v", requests[22])
	}
	if requests[23].path != "/newmoi/catalog/database/list" || requests[23].body["id"] != float64(10) {
		t.Fatalf("selected database list request = %#v", requests[23])
	}
	if requests[24].path != "/newmoi/catalog/database/children" || requests[24].body["id"] != float64(20) {
		t.Fatalf("selected database children request = %#v", requests[24])
	}
	if requests[25].path != "/newmoi/catalog/volume/full_path" || requests[25].body["id"] != float64(30) {
		t.Fatalf("volume full path request = %#v", requests[25])
	}
	if requests[26].path != "/newmoi/catalog/list" {
		t.Fatalf("named full path catalog list request = %#v", requests[26])
	}
	if requests[27].path != "/newmoi/catalog/database/list" || requests[27].body["id"] != float64(10) {
		t.Fatalf("named full path database list request = %#v", requests[27])
	}
	if requests[28].path != "/newmoi/catalog/database/children" || requests[28].body["id"] != float64(20) {
		t.Fatalf("named full path database children request = %#v", requests[28])
	}
	if requests[29].path != "/newmoi/catalog/volume/full_path" || requests[29].body["id"] != float64(30) {
		t.Fatalf("named full path request = %#v", requests[29])
	}
	if requests[30].path != "/newmoi/catalog/update" ||
		requests[30].body["id"] != float64(10) || requests[30].body["name"] != "orders-renamed" ||
		requests[30].body["description"] != "renamed catalog" {
		t.Fatalf("catalog rename request = %#v", requests[30])
	}
	if requests[31].path != "/newmoi/catalog/database/update" ||
		requests[31].body["id"] != float64(20) ||
		requests[31].body["description"] != "renamed database" {
		t.Fatalf("database description update request = %#v", requests[31])
	}
	if requests[32].path != "/newmoi/catalog/volume/update" ||
		requests[32].body["id"] != float64(30) || requests[32].body["name"] != "landing_renamed" ||
		requests[32].body["description"] != "renamed volume" {
		t.Fatalf("volume rename request = %#v", requests[32])
	}
	if requests[33].path != "/newmoi/catalog/info" || requests[33].body["id"] != float64(10) {
		t.Fatalf("catalog info request = %#v", requests[33])
	}
	if requests[34].path != "/newmoi/catalog/database/info" || requests[34].body["id"] != float64(20) {
		t.Fatalf("database info request = %#v", requests[34])
	}
	if requests[35].path != "/newmoi/catalog/volume/info" || requests[35].body["id"] != "30" {
		t.Fatalf("volume info request = %#v", requests[35])
	}
	if requests[36].path != "/newmoi/catalog/volume/delete" || requests[36].body["id"] != float64(30) {
		t.Fatalf("volume delete request = %#v", requests[36])
	}
	if requests[37].path != "/newmoi/catalog/database/delete" || requests[37].body["id"] != float64(20) {
		t.Fatalf("database delete request = %#v", requests[37])
	}
	if requests[38].path != "/newmoi/catalog/delete" || requests[38].body["id"] != float64(10) {
		t.Fatalf("catalog delete request = %#v", requests[38])
	}
}

func TestWorkspaceHandlePrepareCatalogCreatesMissingCatalog(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		var responseData any
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/list":
			responseData = map[string]any{"list": []any{}}
		case "/newmoi/catalog/create":
			responseData = map[string]any{"id": 30}
		default:
			t.Fatalf("path = %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := workspace.PrepareCatalog(context.Background(), "orders", WithCatalogDescription("lake"))
	if err != nil {
		t.Fatal(err)
	}
	if catalog.ID() != 30 || catalog.Name() != "orders" {
		t.Fatalf("catalog = %#v", catalog)
	}
	if len(requests) != 2 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/catalog/list" {
		t.Fatalf("catalog list request = %#v", requests[0])
	}
	if requests[1].path != "/newmoi/catalog/create" || requests[1].body["name"] != "orders" || requests[1].body["description"] != "lake" {
		t.Fatalf("catalog create request = %#v", requests[1])
	}
}

func TestDatabaseHandlePrepareVolumeCreatesMissingVolume(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		var responseData any
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/database/children":
			responseData = map[string]any{"list": []any{}}
		case "/newmoi/catalog/volume/create":
			responseData = map[string]any{"id": 40}
		default:
			t.Fatalf("path = %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	database := &DatabaseHandle{catalog: &CatalogHandle{workspace: workspace, id: 10, name: "orders"}, id: 20, name: "raw"}
	volume, err := database.PrepareVolume(context.Background(), "landing", WithCatalogDescription("files"))
	if err != nil {
		t.Fatal(err)
	}
	if volume.ID() != 40 || volume.Name() != "landing" {
		t.Fatalf("volume = %#v", volume)
	}
	if len(requests) != 2 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/catalog/database/children" || requests[0].body["id"] != float64(20) {
		t.Fatalf("database children request = %#v", requests[0])
	}
	if requests[1].path != "/newmoi/catalog/volume/create" || requests[1].body["database_id"] != float64(20) ||
		requests[1].body["name"] != "landing" || requests[1].body["description"] != "files" {
		t.Fatalf("volume create request = %#v", requests[1])
	}
}

func TestDatabaseHandlePrepareVolumeRejectsInvalidChildID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"list": []map[string]any{{"id": 0, "name": "landing", "type": "volume"}}},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	database := &DatabaseHandle{catalog: &CatalogHandle{workspace: workspace, id: 10, name: "orders"}, id: 20, name: "raw"}
	if _, err := database.PrepareVolume(context.Background(), "landing"); !IsCode(err, "ErrResponseInvalid") {
		t.Fatalf("err = %v", err)
	}
}

func TestWorkspaceCatalogBrowseHandlesScopeDiscoveredResources(t *testing.T) {
	var requests []struct {
		path        string
		workspaceID string
		body        map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path        string
			workspaceID string
			body        map[string]any
		}{path: r.URL.EscapedPath(), workspaceID: r.Header.Get(headerWorkspaceID), body: body})
		var responseData any
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/list":
			responseData = map[string]any{"list": []map[string]any{{"id": 10, "name": "orders", "display_name": "Orders", "description": "lake", "display_description": "Lake catalog", "allowed_actions": []string{"catalog.update_catalog"}}}}
		case "/newmoi/catalog/database/list":
			responseData = map[string]any{"list": []map[string]any{{"id": 20, "name": "raw", "description": "landing db"}}}
		case "/newmoi/catalog/database/children":
			responseData = map[string]any{"list": []map[string]any{
				{"id": "30", "name": "incoming", "type": "volume", "description": "files"},
				{"id": "31", "name": "orders", "type": "table", "description": "table"},
			}}
		case "/newmoi/catalog/volume/full_path":
			responseData = map[string]any{"path": "orders/raw/incoming"}
		case "/newmoi/catalog/table/full_path":
			responseData = map[string]any{"path": "orders/raw/orders"}
		default:
			t.Fatalf("path = %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}

	catalogs, catalogList, err := workspace.Catalogs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalogs) != 1 || catalogs[0].ID() != 10 || catalogs[0].Name() != "orders" || len(catalogList.GetList()) != 1 || catalogList.GetList()[0].GetDisplayName() != "Orders" || catalogList.GetList()[0].GetDisplayDescription() != "Lake catalog" || len(catalogList.GetList()[0].GetAllowedActions()) != 1 {
		t.Fatalf("catalogs = %#v result = %#v", catalogs, catalogList)
	}
	databases, databaseList, err := catalogs[0].Databases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(databases) != 1 || databases[0].ID() != 20 || databases[0].Name() != "raw" || len(databaseList.GetList()) != 1 {
		t.Fatalf("databases = %#v result = %#v", databases, databaseList)
	}
	children, err := databases[0].Children(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(children.GetList()) != 2 {
		t.Fatalf("children = %#v", children)
	}
	volumes, volumeChildren, err := databases[0].Volumes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(volumes) != 1 || volumes[0].ID() != 30 || volumes[0].Name() != "incoming" || len(volumeChildren.GetList()) != 2 {
		t.Fatalf("volumes = %#v children = %#v", volumes, volumeChildren)
	}
	tables, tableChildren, err := databases[0].Tables(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || tables[0].ID() != 31 || tables[0].Name() != "orders" || len(tableChildren.GetList()) != 2 {
		t.Fatalf("tables = %#v children = %#v", tables, tableChildren)
	}
	if path, err := volumes[0].FullPath(context.Background()); err != nil {
		t.Fatal(err)
	} else if path.GetPath() != "orders/raw/incoming" {
		t.Fatalf("volume path = %#v", path)
	}
	if path, err := tables[0].FullPath(context.Background()); err != nil {
		t.Fatal(err)
	} else if path.GetPath() != "orders/raw/orders" {
		t.Fatalf("table path = %#v", path)
	}

	expectedPaths := []string{
		"/newmoi/catalog/list",
		"/newmoi/catalog/database/list",
		"/newmoi/catalog/database/children",
		"/newmoi/catalog/database/children",
		"/newmoi/catalog/database/children",
		"/newmoi/catalog/volume/full_path",
		"/newmoi/catalog/table/full_path",
	}
	if len(requests) != len(expectedPaths) {
		t.Fatalf("request count = %d", len(requests))
	}
	for i, path := range expectedPaths {
		if requests[i].path != path || requests[i].workspaceID != "ws-1" {
			t.Fatalf("request %d = %#v", i, requests[i])
		}
	}
	if requests[1].body["id"] != float64(10) {
		t.Fatalf("database list body = %#v", requests[1].body)
	}
	for _, index := range []int{2, 3, 4} {
		if requests[index].body["id"] != float64(20) {
			t.Fatalf("children body %d = %#v", index, requests[index].body)
		}
	}
	if requests[5].body["id"] != float64(30) || requests[6].body["id"] != float64(31) {
		t.Fatalf("path bodies = %#v %#v", requests[5].body, requests[6].body)
	}
}

func TestWorkspaceHandlePrepareCatalogRejectsDuplicateName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"list": []map[string]any{
				{"id": 10, "name": "orders"},
				{"id": 11, "name": "orders"},
			}},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.PrepareCatalog(context.Background(), "orders"); !IsCode(err, "ErrDuplicateName") {
		t.Fatalf("err = %v", err)
	}
}

func TestWorkspaceClaimValidatesRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Workspaces().Claim(context.Background(), ""); !IsCode(err, ErrWorkspaceRequired.Code) {
		t.Fatalf("claim err = %v", err)
	}
}

func TestWorkspaceListUsesNameFilter(t *testing.T) {
	var gotMethod, gotPath, gotRawQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotRawQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"total": 1,
				"workspaces": []any{
					map[string]any{"id": "ws-1", "name": "orders prod", "status": "ACTIVE", "owner_id": "user-1", "created_at": "2026-06-24T00:00:00Z"},
				},
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspaces, err := client.Workspaces().List(context.Background(), WithWorkspaceNameFilter("orders prod"))
	if err != nil {
		t.Fatal(err)
	}
	if workspaces.GetTotal() != 1 || len(workspaces.GetWorkspaces()) != 1 || workspaces.GetWorkspaces()[0].GetId() != "ws-1" {
		t.Fatalf("workspaces = %#v", workspaces)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method = %s", gotMethod)
	}
	if gotPath != "/newmoi/workspaces" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotRawQuery != "name=orders+prod" {
		t.Fatalf("query = %s", gotRawQuery)
	}
}

func TestClientEncodesPathAndQueryOptions(t *testing.T) {
	var gotPath, gotRawQuery, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotRawQuery = r.URL.RawQuery
		gotMethod = r.Method
		responseData := map[string]any{"ok": true}
		if r.URL.EscapedPath() == "/newmoi/workflow/v2/workflow-apps/workflow%201/executions/execution%2F1/import-files" {
			responseData = map[string]any{"files": []any{}, "total": 0, "total_sum": 0, "total_success": 0, "total_failed": 0}
		} else if r.URL.EscapedPath() == "/newmoi/user/info" {
			responseData = map[string]any{
				"user":     "alice",
				"role":     "admin",
				"is_admin": true,
				"user_info": map[string]any{
					"id":       "user-1",
					"name":     "alice",
					"status":   "enabled",
					"reserved": true,
				},
				"iam_context":          map[string]any{"effective_role_id": "admin-role", "role_context_status": "ready"},
				"iam_capabilities":     map[string]any{"effective_role_id": "admin-role", "items": []any{}},
				"iam_snapshot_version": "snapshot-1",
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Workflows("ws-1").ListRunImportFiles(
		context.Background(),
		"workflow 1",
		"execution/1",
		WithPage(2),
		WithPageSize(50),
		WithQueryValues("status", "1", "2"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method = %s", gotMethod)
	}
	if gotPath != "/newmoi/workflow/v2/workflow-apps/workflow%201/executions/execution%2F1/import-files" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotRawQuery != "page=2&page_size=50&status=1&status=2" {
		t.Fatalf("query = %s", gotRawQuery)
	}
}

func TestIDRequired(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Compute("ws-1").Get(context.Background(), "")
	if !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("err = %v", err)
	}
}

func TestComputeDeleteReturnsErrorOnly(t *testing.T) {
	var gotMethod, gotPath, gotRawQuery, gotWorkspace string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotRawQuery = r.URL.RawQuery
		gotWorkspace = r.Header.Get(headerWorkspaceID)
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK", "msg": "OK",
			"data": map[string]any{"resource_id": "compute 1", "status": "TERMINATING"},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Compute("ws-1").Delete(context.Background(), "compute 1", WithForce(true)); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/newmoi/compute-resources/compute%201" || gotRawQuery != "force=true" || gotWorkspace != "ws-1" {
		t.Fatalf("request = method %s path %s query %s workspace %s", gotMethod, gotPath, gotRawQuery, gotWorkspace)
	}
	if err := client.Compute("ws-1").Delete(context.Background(), ""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("delete id err = %v", err)
	}
}

func TestComputeAdminListDecodesCatalogFieldsStrictly(t *testing.T) {
	var gotMethod, gotPath, gotRawQuery, gotWorkspace string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotRawQuery = r.URL.RawQuery
		gotWorkspace = r.Header.Get(headerWorkspaceID)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"items": []map[string]any{{
					"id":                     "compute-1",
					"workspace_id":           "ws-target",
					"name":                   "go-worker",
					"description":            "etl",
					"spec_id":                "spec-1",
					"cpu":                    2,
					"memory_gib":             8,
					"gpu":                    0,
					"min_replicas":           1,
					"max_replicas":           3,
					"current_replicas":       1,
					"go_worker_image_id":     "go-img-1",
					"python_worker_image_id": "py-img-1",
					"platform":               "linux/amd64",
					"auto_suspend_minutes":   0,
					"is_default":             false,
					"status":                 "ACTIVE",
					"status_message":         "",
					"created_by":             "user-1",
					"created_at":             "2026-01-01T00:00:00Z",
					"updated_at":             "2026-01-01T00:00:00Z",
					"workspace": map[string]any{
						"id":           "ws-target",
						"name":         "Workspace Target",
						"owner_id":     "owner-1",
						"account_name": "account_1",
					},
					"kind":               "task",
					"cpu_milli":          2000,
					"memory_mib":         8192,
					"gpu_count":          0,
					"gpu_memory_mib":     0,
					"gpu_cores":          0,
					"desired_replicas":   2,
					"worker_images":      []map[string]any{{"worker_type": "go", "image_id": "go-img-1", "platform": "linux/amd64"}},
					"scale_reason":       "activation",
					"last_activation_at": "2026-01-01T00:01:00Z",
					"last_active_at":     "2026-01-01T00:02:00Z",
				}},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Compute("ws-admin").AdminList(context.Background(), WithLimit(50), WithOffset(0))
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet || gotPath != "/newmoi/admin/compute-resources" || gotRawQuery != "limit=50&offset=0" || gotWorkspace != "ws-admin" {
		t.Fatalf("request = method %s path %s query %s workspace %s", gotMethod, gotPath, gotRawQuery, gotWorkspace)
	}
	if resp.GetTotal() != 1 || resp.GetLimit() != 50 || resp.GetOffset() != 0 || len(resp.GetItems()) != 1 {
		t.Fatalf("response envelope = %+v", resp)
	}
	item := resp.GetItems()[0]
	if item.GetKind() != "task" || item.GetCpuMilli() != 2000 || item.GetMemoryMib() != 8192 || item.GetDesiredReplicas() != 2 {
		t.Fatalf("catalog fields = %+v", item)
	}
	if len(item.GetWorkerImages()) != 1 || item.GetWorkerImages()[0].GetImageId() != "go-img-1" {
		t.Fatalf("worker images = %+v", item.GetWorkerImages())
	}
	if item.GetScaleReason() != "activation" || item.GetLastActivationAt() != "2026-01-01T00:01:00Z" || item.GetLastActiveAt() != "2026-01-01T00:02:00Z" {
		t.Fatalf("scale timestamps = %+v", item)
	}
	if item.GetWorkspace().GetId() != "ws-target" || item.GetWorkspace().GetAccountName() != "account_1" {
		t.Fatalf("workspace = %+v", item.GetWorkspace())
	}
}

func TestComputeRuntimeUsesCollectionContractAndStrictEnvelope(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.EscapedPath()+"?"+r.URL.RawQuery)
		if r.Header.Get(headerWorkspaceID) != "ws-1" {
			t.Fatalf("workspace header = %q", r.Header.Get(headerWorkspaceID))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"runtimes": []any{map[string]any{
				"compute_resource_id": "cr-1",
				"worker_type":         "python-worker",
				"status":              "ACTIVE",
				"desired_replicas":    1,
				"current_replicas":    1,
				"last_activation_at":  "2026-08-11T10:30:11Z",
				"last_active_at":      "2026-08-11T10:32:08Z",
				"created_at":          "2026-08-11T10:30:00Z",
				"updated_at":          "2026-08-11T10:32:08Z",
			}}},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	runtimes, err := client.Compute("ws-1").Runtime(ctx, "cr-1")
	if err != nil {
		t.Fatalf("collection runtime: %v", err)
	}
	if len(runtimes) != 1 || runtimes[0].GetWorkerType() != "python-worker" || runtimes[0].GetComputeResourceId() != "cr-1" {
		t.Fatalf("collection runtimes = %+v", runtimes)
	}
	runtimes, err = client.Compute("ws-1").Runtime(ctx, "cr-1")
	if err != nil {
		t.Fatalf("object runtime: %v", err)
	}
	if len(runtimes) != 1 {
		t.Fatalf("object runtimes = %+v", runtimes)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	handle, err := workspace.ComputeResource("cr-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Runtime(ctx); err != nil {
		t.Fatalf("handle runtime: %v", err)
	}
	if _, err := client.Compute("ws-1").AdminRuntime(ctx, "cr-1"); err != nil {
		t.Fatalf("admin runtime: %v", err)
	}
	if _, err := client.Compute("ws-1").Runtime(ctx, ""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty runtime id err = %v", err)
	}
	if _, err := client.Compute("ws-1").AdminRuntime(ctx, ""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty admin runtime id err = %v", err)
	}
	want := []string{
		"/newmoi/compute-resources/cr-1/runtime?",
		"/newmoi/compute-resources/cr-1/runtime?",
		"/newmoi/compute-resources/cr-1/runtime?",
		"/newmoi/admin/compute-resources/cr-1/runtime?",
	}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("runtime requests = %#v, want %#v", requests, want)
	}

	for _, body := range [][]byte{
		[]byte(`{"runtimes":[]}`),
		[]byte(`{"code":"OK","msg":"OK","data":{}}`),
		[]byte(`{"code":"OK","msg":"OK","data":{"runtimes":[{"worker_type":1}]}}`),
		[]byte(`{"code":"OK","msg":"OK","data":{"runtimes":[{"worker_type":"python-worker","status":"ACTIVE","desired_replicas":1,"current_replicas":1,"created_at":"2026-08-11T10:30:00Z","updated_at":"2026-08-11T10:32:08Z"}]}}`),
	} {
		var out strictComputeWorkerRuntimeList
		if err := parseProductResponse(body, &out, ""); err == nil {
			t.Fatalf("strict runtime decode accepted %s", body)
		}
	}
}

func TestComputeWorkerImagesRejectInvalidCanonicalIdentity(t *testing.T) {
	tests := []struct {
		name string
		item map[string]any
	}{
		{
			name: "missing worker type",
			item: map[string]any{"id": "img-1", "type": "gpu-v2", "image_key": "gpu-v2"},
		},
		{
			name: "conflicting legacy type",
			item: map[string]any{"id": "img-1", "worker_type": "document-worker", "type": "gpu-v1", "image_key": "gpu-v2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/newmoi/compute-resources/worker-images" {
					t.Fatalf("path = %s", r.URL.Path)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": []any{tt.item}})
			}))
			defer server.Close()

			client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Compute("ws-1").WorkerImages(context.Background()); !IsCode(err, "ErrResponseInvalid") {
				t.Fatalf("worker images error = %v, want ErrResponseInvalid", err)
			}
		})
	}
}

func TestComputeRuntimePreservesHTTPErrorStatusAndCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := map[string]int{
			"400": http.StatusBadRequest,
			"403": http.StatusForbidden,
			"404": http.StatusNotFound,
			"500": http.StatusInternalServerError,
		}[strings.TrimSuffix(r.URL.Path, "/runtime")[strings.LastIndex(strings.TrimSuffix(r.URL.Path, "/runtime"), "/")+1:]]
		if status == 0 {
			t.Fatalf("missing runtime error status selector: %s", r.URL.String())
		}
		if r.Header.Get(headerWorkspaceID) != "ws-1" {
			t.Fatalf("workspace header = %q", r.Header.Get(headerWorkspaceID))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": fmt.Sprintf("RUNTIME_%d", status),
			"msg":  "runtime request failed",
			"data": nil,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	service := client.Compute("ws-1")
	tests := []struct {
		name   string
		status int
		call   func() error
	}{
		{
			name:   "workspace bad request",
			status: http.StatusBadRequest,
			call: func() error {
				_, err := service.Runtime(context.Background(), "400")
				return err
			},
		},
		{
			name:   "workspace forbidden",
			status: http.StatusForbidden,
			call: func() error {
				_, err := service.Runtime(context.Background(), "403")
				return err
			},
		},
		{
			name:   "admin not found",
			status: http.StatusNotFound,
			call: func() error {
				_, err := service.AdminRuntime(context.Background(), "404")
				return err
			},
		},
		{
			name:   "admin server error",
			status: http.StatusInternalServerError,
			call: func() error {
				_, err := service.AdminRuntime(context.Background(), "500")
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			var sdkErr *Error
			if !errors.As(err, &sdkErr) {
				t.Fatalf("runtime error = %T %v, want SDK error", err, err)
			}
			if sdkErr.Status != tt.status || sdkErr.Code != fmt.Sprintf("RUNTIME_%d", tt.status) {
				t.Fatalf("runtime error = %+v, want status %d code RUNTIME_%d", sdkErr, tt.status, tt.status)
			}
		})
	}
}

func TestComputeWorkerImageSelectionsBodyCanonicalizesLegacyKind(t *testing.T) {
	body, err := computeWorkerImageSelectionsBody([]*WorkerImageSelection{
		{Kind: "legacy-worker", ImageId: "legacy-image:v1", Platform: "linux/amd64"},
		{WorkerType: "document-worker", ImageId: "gpu-v2:v1", Platform: "linux/amd64"},
		{Kind: "same-worker", WorkerType: "same-worker", ImageId: "same-image:v1", Platform: "linux/amd64"},
	})
	if err != nil {
		t.Fatalf("canonicalize worker image selections: %v", err)
	}
	want := []map[string]string{
		{"worker_type": "legacy-worker", "image_id": "legacy-image:v1", "platform": "linux/amd64"},
		{"worker_type": "document-worker", "image_id": "gpu-v2:v1", "platform": "linux/amd64"},
		{"worker_type": "same-worker", "image_id": "same-image:v1", "platform": "linux/amd64"},
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("worker image body = %#v, want %#v", body, want)
	}
	if _, err := computeWorkerImageSelectionsBody([]*WorkerImageSelection{{Kind: "legacy-worker", WorkerType: "document-worker"}}); !IsCode(err, ErrWorkerTypeConflict.Code) {
		t.Fatalf("conflicting worker image selection error = %v", err)
	}
}

func TestWorkspaceMemberCurrentPrincipalUsesBackendRequestShape(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		workspace string
		rawBody   string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		if len(data) > 0 {
			if err := json.Unmarshal(data, &body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method    string
			path      string
			workspace string
			rawBody   string
			body      map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), workspace: r.Header.Get(headerWorkspaceID), rawBody: string(data), body: body})
		responseData := any(nil)
		if r.URL.EscapedPath() == "/newmoi/user/info" {
			responseData = map[string]any{
				"user":     "alice",
				"role":     "admin",
				"is_admin": true,
				"user_info": map[string]any{
					"id":       "user-1",
					"name":     "alice",
					"status":   "enabled",
					"reserved": true,
				},
				"iam_context": map[string]any{
					"effective_role_id": "admin-role", "role_context_status": "ready",
					"default_role_id": "admin-role", "default_role_version": 3, "explicit_role_id": "",
					"roles": []any{map[string]any{"role_id": "admin-role", "role_name": "admin", "lifecycle_version": 4, "selectable": true, "disabled_reason": ""}},
				},
				"iam_capabilities": map[string]any{
					"effective_role_id": "admin-role", "request_id": "req1", "trace_id": "trace1",
					"items": []any{map[string]any{"capability_id": "action.model.update", "allowed": true, "reason_code": "ALLOW", "all_resources": false, "resource_ids": []string{"llm:7"}}},
				},
				"iam_snapshot_version": "snapshot-1",
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	members := client.WorkspaceMembers("workspace 1")
	currentUser, err := members.CurrentPrincipal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if currentUser.GetUserInfo().GetId() != "user-1" || !currentUser.GetIsAdmin() {
		t.Fatalf("user info result = %#v", currentUser)
	}
	if currentUser.GetIamSnapshotVersion() != "snapshot-1" || currentUser.GetIamContext().GetEffectiveRoleId() != "admin-role" {
		t.Fatalf("canonical IAM bootstrap = %#v", currentUser)
	}
	roles := currentUser.GetIamContext().GetRoles()
	if len(roles) != 1 || roles[0].GetRoleId() != "admin-role" || roles[0].GetLifecycleVersion() != 4 || !roles[0].GetSelectable() || currentUser.GetIamContext().GetDefaultRoleVersion() != 3 {
		t.Fatalf("bootstrap role selection = %+v", currentUser.GetIamContext())
	}
	capabilities := currentUser.GetIamCapabilities()
	items := capabilities.GetItems()
	if len(items) != 1 || items[0].GetCapabilityId() != "action.model.update" || !items[0].GetAllowed() || items[0].GetAllResources() || !slices.Equal(items[0].GetResourceIds(), []string{"llm:7"}) || capabilities.GetRequestId() != "req1" || capabilities.GetTraceId() != "trace1" {
		t.Fatalf("bootstrap capability scope = %+v", capabilities)
	}
	if err := members.Update(context.Background(), "user-1", "Alice", "workspace owner",
		WithUserTag(101, "research"),
		WithUserTag(102, "owner"),
	); err != nil {
		t.Fatal(err)
	}
	if err := members.Update(context.Background(), "user-1", "Alice", ""); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 3 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].method != http.MethodPost || requests[0].path != "/newmoi/user/info" ||
		requests[0].workspace != "workspace 1" || requests[0].rawBody != "" {
		t.Fatalf("user info request = %#v", requests[0])
	}
	if requests[1].method != http.MethodPost || requests[1].path != "/newmoi/user/update_info" ||
		requests[1].workspace != "workspace 1" || requests[1].body["id"] != "user-1" ||
		requests[1].body["name"] != "Alice" || requests[1].body["description"] != "workspace owner" {
		t.Fatalf("update user info request = %#v", requests[1])
	}
	tags, ok := requests[1].body["tag_list"].([]any)
	if !ok || len(tags) != 2 {
		t.Fatalf("tag list = %#v", requests[1].body["tag_list"])
	}
	tag0, ok := tags[0].(map[string]any)
	if !ok || tag0["id"] != float64(101) || tag0["value"] != "research" {
		t.Fatalf("tag 0 = %#v", tags[0])
	}
	tag1, ok := tags[1].(map[string]any)
	if !ok || tag1["id"] != float64(102) || tag1["value"] != "owner" {
		t.Fatalf("tag 1 = %#v", tags[1])
	}
	emptyTags, ok := requests[2].body["tag_list"].([]any)
	if !ok || len(emptyTags) != 0 {
		t.Fatalf("empty tag list = %#v", requests[2].body["tag_list"])
	}
}

func TestWorkspaceMemberCurrentPrincipalMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.WorkspaceMembers("ws-1").Update(context.Background(), "", "Alice", ""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("update user info err = %v", err)
	}
	if err := client.WorkspaceMembers("ws-1").Update(context.Background(), "user-1", "", ""); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("update user info without name err = %v", err)
	}
}

func TestWorkspaceMemberMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method    string
			path      string
			workspace string
			body      map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), workspace: r.Header.Get(headerWorkspaceID), body: body})
		responseData := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/user/detail_info":
			responseData = map[string]any{
				"id":          "user-1",
				"name":        "alice",
				"description": "analyst",
				"status":      "enabled",
				"created_at":  "2026-06-25T00:00:00Z",
				"updated_at":  "2026-06-25T00:00:00Z",
				"reserved":    false,
				"role_list":   []any{map[string]any{"id": 10, "name": "viewer", "status": "enabled"}},
				"tag_list":    []any{map[string]any{"id": 101, "value": "research"}},
			}
		case "/newmoi/workspaces/ws-1/iam/users/canonical-user-2/role-bindings":
			if r.Method == http.MethodGet {
				responseData = map[string]any{
					"user_id": "canonical-user-2", "roles": []any{map[string]any{"role_id": "role-20", "role_name": "Editor", "selectable": true}},
					"default_role_id": "role-20", "default_role_version": 4, "binding_version": "binding-v4",
				}
			} else {
				responseData = map[string]any{
					"user_id": "canonical-user-2", "assigned_role_ids": []string{"role-20"}, "default_role_id": "role-20",
					"binding_version": 5, "default_role_version": 5, "audit_event_id": "audit-1", "idempotent_replay": false,
				}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	members := client.WorkspaceMembers("workspace 1")
	userDetail, err := members.Member(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if userDetail.GetId() != "user-1" || len(userDetail.GetRoleList()) != 1 || userDetail.GetRoleList()[0].GetName() != "viewer" {
		t.Fatalf("user detail result = %#v", userDetail)
	}
	if err := members.Delete(context.Background(), "user-1"); err != nil {
		t.Fatal(err)
	}
	if err := members.UpdateStatus(context.Background(), "user-1", "disable"); err != nil {
		t.Fatal(err)
	}
	assertRequest := func(idx int, path string, want map[string]any) {
		t.Helper()
		if len(requests) <= idx {
			t.Fatalf("missing request %d; count = %d", idx, len(requests))
		}
		if requests[idx].method != http.MethodPost || requests[idx].path != path || requests[idx].workspace != "workspace 1" {
			t.Fatalf("request %d = %#v", idx, requests[idx])
		}
		for key, value := range want {
			if requests[idx].body[key] != value {
				t.Fatalf("request %d body[%s] = %#v, want %#v; body = %#v", idx, key, requests[idx].body[key], value, requests[idx].body)
			}
		}
	}
	assertRequest(0, "/newmoi/user/detail_info", map[string]any{"id": "user-1"})
	assertRequest(1, "/newmoi/user/delete", map[string]any{"id": "user-1"})
	assertRequest(2, "/newmoi/user/update_status", map[string]any{"id": "user-1", "action": "disable"})
}

func TestWorkspaceMemberMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	members := client.WorkspaceMembers("ws-1")
	ctx := context.Background()
	if _, err := members.Member(ctx, ""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("user detail err = %v", err)
	}
	if err := members.Delete(ctx, ""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("delete user err = %v", err)
	}
	if err := members.UpdateStatus(ctx, "", "disable"); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("update user status id err = %v", err)
	}
	if err := members.UpdateStatus(ctx, "user-1", ""); !IsCode(err, ErrActionRequired.Code) {
		t.Fatalf("update user status action err = %v", err)
	}
}

func TestPostChannelCallbackUsesPublicProviderPathAndRequestHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/newmoi/callbacks/github/ws-1/credential-1" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get(headerAPIKey); got != "" {
			t.Fatalf("public callback unexpectedly carried API key %q", got)
		}
		if got := r.Header.Get(headerWorkspaceID); got != "" {
			t.Fatalf("public callback unexpectedly carried workspace %q", got)
		}
		if got := r.Header.Get("X-Hub-Signature-256"); got != "sha256=test" {
			t.Fatalf("signature header = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"action":"opened"}` {
			t.Fatalf("body = %q", body)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client, err := New(
		server.URL+"/newmoi", WithAPIKey("should-not-be-sent"),
		WithRequestModifier(func(req *http.Request) {
			req.Header.Set("X-Hub-Signature-256", "sha256=test")
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	response, err := client.PostChannelCallback(
		context.Background(), "github", "ws-1", "credential-1", []byte(`{"action":"opened"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(response) != "ok" {
		t.Fatalf("response = %q", response)
	}
}

func TestWorkspaceMemberListAndInviteMethodsUseOptionsBody(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		responseData := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/user/list":
			responseData = map[string]any{
				"total": 1,
				"user_list": []any{map[string]any{
					"id":          "user-1",
					"iam_user_id": "moi-user-1",
					"name":        "alice",
					"status":      "enabled",
					"created_at":  "2026-06-25T00:00:00Z",
					"updated_at":  "2026-06-25T00:00:00Z",
					"reserved":    false,
					"role_list":   []any{map[string]any{"id": 10, "name": "viewer", "status": "enabled"}},
					"tag_list":    []any{},
				}},
			}
		case "/newmoi/user/create":
			responseData = map[string]any{"id": "canonical-user-2"}
		case "/newmoi/catalog/ref_list":
			responseData = map[string]any{"list": []map[string]any{{"ref_id": "wf-1", "ref_type": "workflow", "volume_id": "10", "volume_name": "catalog", "table_id": "", "table_name": ""}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	members := client.WorkspaceMembers("ws-1")
	users, err := members.Members(context.Background(),
		WithMemberPage(1),
		WithMemberPageSize(20),
		WithMemberFilter("name", "alice", "bob"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if users.GetTotal() != 1 || len(users.GetUserList()) != 1 || users.GetUserList()[0].GetName() != "alice" || users.GetUserList()[0].GetIamUserId() != "moi-user-1" {
		t.Fatalf("users result = %#v", users)
	}
	invited, err := members.Invite(context.Background(), "alice", 20,
		WithUserMobileNumber("13800138000"),
		WithUserDescription("analyst"),
		WithUserRoleIDs(10, 20),
		WithUserAutoAccept(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	if invited.GetId() != "canonical-user-2" {
		t.Fatalf("invited user id = %q", invited.GetId())
	}
	if len(requests) != 2 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/user/list" || requests[0].body["page"] != float64(1) || requests[0].body["page_size"] != float64(20) {
		t.Fatalf("user list request = %#v", requests[0])
	}
	userFilters, ok := requests[0].body["filters"].([]any)
	if !ok || len(userFilters) != 1 {
		t.Fatalf("user filters = %#v", requests[0].body["filters"])
	}
	userFilter, ok := userFilters[0].(map[string]any)
	if !ok || userFilter["name"] != "name" {
		t.Fatalf("user filter = %#v", userFilters[0])
	}
	userFilterValues, ok := userFilter["values"].([]any)
	if !ok || len(userFilterValues) != 2 || userFilterValues[0] != "alice" || userFilterValues[1] != "bob" {
		t.Fatalf("user filter values = %#v", userFilter["values"])
	}
	if requests[1].path != "/newmoi/user/create" ||
		requests[1].body["name"] != "alice" ||
		requests[1].body["default_role_id"] != float64(20) ||
		requests[1].body["mobile_number"] != "13800138000" ||
		requests[1].body["description"] != "analyst" ||
		requests[1].body["auto_accept"] != true {
		t.Fatalf("create user request = %#v", requests[1])
	}
	if _, exists := requests[1].body["id"]; exists {
		t.Fatalf("create user request must not contain caller-supplied id: %#v", requests[1].body)
	}
	userRoles, ok := requests[1].body["role_id_list"].([]any)
	if !ok || len(userRoles) != 2 || userRoles[0] != float64(10) || userRoles[1] != float64(20) {
		t.Fatalf("user roles = %#v", requests[1].body["role_id_list"])
	}
}

func TestWorkspaceMemberInviteValidatesRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	members := client.WorkspaceMembers("ws-1")
	ctx := context.Background()
	if _, err := members.Invite(ctx, "", 10); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("create user err = %v", err)
	}
	if _, err := members.Invite(ctx, "alice", 10); !IsCode(err, ErrInvitationContactRequired.Code) {
		t.Fatalf("missing invitation contact err = %v", err)
	}
	if _, err := members.Invite(ctx, "alice", 10, WithUserEmail("alice@example.test"), WithUserMobileNumber("13800138000")); !IsCode(err, ErrInvitationContactConflict.Code) {
		t.Fatalf("ambiguous invitation contact err = %v", err)
	}
	if _, err := members.Invite(ctx, "alice", 10, WithUserMobileNumber("+8613800138000")); !IsCode(err, ErrInvitationMobileNumberInvalid.Code) {
		t.Fatalf("invalid mainland China invitation mobile err = %v", err)
	}
	if _, err := members.Invite(ctx, "alice", 0, WithUserEmail("alice@example.test"), WithUserRoleIDs(10)); !IsCode(err, ErrDefaultRoleRequired.Code) {
		t.Fatalf("missing default role err = %v", err)
	}
	if _, err := members.Invite(ctx, "alice", 20, WithUserEmail("alice@example.test"), WithUserRoleIDs(10)); !IsCode(err, ErrDefaultRoleNotAssigned.Code) {
		t.Fatalf("unassigned default role err = %v", err)
	}
	if _, err := members.Invite(ctx, "alice", 10, WithUserEmail("alice@example.test"), WithUserGithubLogin("octocat")); !IsCode(err, ErrInvitationContactConflict.Code) {
		t.Fatalf("email+github invitation contact err = %v", err)
	}
	if _, err := members.Invite(ctx, "alice", 0, WithUserGithubLogin("-octocat")); !IsCode(err, ErrInvitationGitHubLoginInvalid.Code) {
		t.Fatalf("invalid github login err = %v", err)
	}
	if _, err := members.Invite(ctx, "alice", 0, WithUserGithubLogin("@@octocat")); !IsCode(err, ErrInvitationGitHubLoginInvalid.Code) {
		t.Fatalf("double-prefix github login err = %v", err)
	}
}

func TestWorkspaceMemberInviteGitHubLoginTransport(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/newmoi/user/create" {
			t.Fatalf("unexpected path %s", r.URL.EscapedPath())
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, body)
		if name, _ := body["name"].(string); name == "unknown" {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "ErrUserNotFound", "msg": "Invitation account not found"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": map[string]any{"id": "canonical-user-2"}})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	members := client.WorkspaceMembers("ws-1")
	seated, err := members.Invite(context.Background(), "alice", 10, WithUserGithubLogin("@OctoCat"), WithUserRoleIDs(10))
	if err != nil {
		t.Fatal(err)
	}
	if seated.GetId() != "canonical-user-2" {
		t.Fatalf("seated github invite id = %q", seated.GetId())
	}
	_, invalidErr := members.Invite(t.Context(), "no-roles", 0, WithUserGithubLogin("octocat"))
	if !errors.Is(invalidErr, ErrDefaultRoleRequired) {
		t.Fatalf("empty GitHub roles: %v", invalidErr)
	}
	rejected, err := members.Invite(t.Context(), "unknown", 10, WithUserGithubLogin("new-user"), WithUserRoleIDs(10))
	var apiError *Error
	if !errors.As(err, &apiError) || apiError.Code != "ErrUserNotFound" || apiError.Status != http.StatusNotFound || apiError.Message != "Invitation account not found" || rejected.GetId() != "" {
		t.Fatalf("unknown GitHub account: result = %#v, error = %v", rejected, err)
	}
	wantRequests := []map[string]any{
		{"name": "alice", "github_login": "octocat", "role_id_list": []any{float64(10)}, "default_role_id": float64(10)},
		{"name": "unknown", "github_login": "new-user", "role_id_list": []any{float64(10)}, "default_role_id": float64(10)},
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("GitHub invitation requests = %#v, want %#v", requests, wantRequests)
	}
}

func TestWorkspaceMemberListDecodesPendingInvitation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/newmoi/user/list" {
			t.Fatalf("unexpected path %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"total": 2,
				"user_list": []any{
					map[string]any{
						"id":          "user-1",
						"iam_user_id": "moi-user-1",
						"name":        "alice",
						"status":      "enabled",
						"created_at":  "2026-08-20T00:00:00Z",
						"updated_at":  "2026-08-20T00:00:00Z",
						"reserved":    false,
						"role_list":   []any{},
						"tag_list":    []any{},
					},
					map[string]any{
						"id":          "user-2",
						"iam_user_id": "moi-user-2",
						"name":        "bob",
						"status":      "pending",
						"created_at":  "2026-08-20T00:00:00Z",
						"updated_at":  "2026-08-20T00:00:00Z",
						"reserved":    false,
						"role_list":   []any{},
						"tag_list":    []any{},
					},
				},
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	users, err := client.WorkspaceMembers("ws-1").Members(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if users.GetTotal() != 2 || len(users.GetUserList()) != 2 {
		t.Fatalf("users result = %#v", users)
	}
	if users.GetUserList()[0].GetName() != "alice" || users.GetUserList()[0].GetStatus() != "enabled" {
		t.Fatalf("active member = %#v", users.GetUserList()[0])
	}
	pending := users.GetUserList()[1]
	if pending.GetId() != "user-2" || pending.GetIamUserId() != "moi-user-2" || pending.GetStatus() != "pending" || pending.GetName() != "bob" {
		t.Fatalf("pending invitation = %#v", pending)
	}
}

func TestWorkspaceMemberAuditMethodsUseOptionsAndTypedResults(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		rawQuery  string
		workspace string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, struct {
			method    string
			path      string
			rawQuery  string
			workspace string
		}{method: r.Method, path: r.URL.EscapedPath(), rawQuery: r.URL.RawQuery, workspace: r.Header.Get(headerWorkspaceID)})
		var data map[string]any
		switch r.URL.EscapedPath() {
		case "/newmoi/audit/logs":
			data = map[string]any{
				"items": []any{
					map[string]any{
						"id":                     "log-1",
						"workspace_id":           "ws-1",
						"operator_user_id":       "user-1",
						"operator_name_snapshot": "Ada",
						"method":                 "POST",
						"path":                   "/newmoi/user/create",
						"route_pattern":          "/newmoi/user/create",
						"status_code":            200,
						"result":                 "success",
						"duration_ms":            15,
						"request_id":             "req-1",
						"created_at":             "2026-06-25T00:00:00Z",
					},
				},
				"total":     1,
				"page":      2,
				"page_size": 50,
			}
		case "/newmoi/audit/logs/overview":
			data = map[string]any{"total": 3, "success": 2, "failed": 1}
		default:
			t.Fatalf("unexpected request %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	members := client.WorkspaceMembers("ws-1")
	start := time.Date(2026, 6, 25, 8, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 25, 9, 0, 0, 0, time.UTC)
	logs, err := members.AuditLogs(
		context.Background(),
		WithAuditMethod("POST"),
		WithAuditResult("success"),
		WithAuditStatusCode(200),
		WithAuditStartTime(start),
		WithAuditEndTime(end),
		WithPage(2),
		WithPageSize(50),
	)
	if err != nil {
		t.Fatal(err)
	}
	if logs.GetTotal() != 1 || logs.GetPage() != 2 || logs.GetPageSize() != 50 || len(logs.GetItems()) != 1 {
		t.Fatalf("audit logs = %#v", logs)
	}
	if item := logs.GetItems()[0]; item.GetId() != "log-1" || item.GetMethod() != "POST" || item.GetResult() != "success" || item.GetStatusCode() != 200 {
		t.Fatalf("audit log item = %#v", item)
	}
	overview, err := members.AuditLogOverview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if overview.GetTotal() != 3 || overview.GetSuccess() != 2 || overview.GetFailed() != 1 {
		t.Fatalf("audit overview = %#v", overview)
	}
	if len(requests) != 2 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].method != http.MethodGet || requests[0].path != "/newmoi/audit/logs" || requests[0].workspace != "ws-1" {
		t.Fatalf("audit list request = %#v", requests[0])
	}
	wantQuery := "end_time=2026-06-25T09%3A00%3A00Z&method=POST&page=2&page_size=50&result=success&start_time=2026-06-25T08%3A00%3A00Z&status_code=200"
	if requests[0].rawQuery != wantQuery {
		t.Fatalf("audit list query = %s", requests[0].rawQuery)
	}
	if requests[1].method != http.MethodGet || requests[1].path != "/newmoi/audit/logs/overview" || requests[1].workspace != "ws-1" {
		t.Fatalf("audit overview request = %#v", requests[1])
	}
}

func TestDashboardTaskExecutionOverviewUsesWorkspaceScope(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		workspace string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, struct {
			method    string
			path      string
			workspace string
		}{method: r.Method, path: r.URL.EscapedPath(), workspace: r.Header.Get(headerWorkspaceID)})
		if r.URL.EscapedPath() != "/newmoi/dashboard/task-execution-overview" {
			t.Fatalf("unexpected path %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"running":     1,
				"success":     2,
				"failed":      3,
				"pending":     4,
				"total":       10,
				"window_days": 7,
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	overview, err := client.Dashboard("ws-1").TaskExecutionOverview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if overview.Running != 1 || overview.Success != 2 || overview.Failed != 3 || overview.Pending != 4 || overview.Total != 10 || overview.WindowDays != 7 {
		t.Fatalf("dashboard overview = %#v", overview)
	}
	if len(requests) != 1 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].method != http.MethodGet || requests[0].path != "/newmoi/dashboard/task-execution-overview" || requests[0].workspace != "ws-1" {
		t.Fatalf("dashboard request = %#v", requests[0])
	}
}

func TestWorkspaceMemberHandleScopesRequests(t *testing.T) {
	var requests []struct {
		path      string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path      string
			workspace string
			body      map[string]any
		}{path: r.URL.EscapedPath(), workspace: r.Header.Get(headerWorkspaceID), body: body})
		data := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/user/create":
			data = map[string]any{"id": "canonical-user-2"}
		case "/newmoi/user/detail_info":
			data = map[string]any{"id": "canonical-user-2", "name": "alice", "status": "enabled"}
		case "/newmoi/workspaces/ws-1/iam/users/canonical-user-2/role-bindings":
			if r.Method == http.MethodGet {
				data = map[string]any{
					"user_id": "canonical-user-2", "roles": []any{map[string]any{"role_id": "role-20", "role_name": "Editor", "selectable": true}},
					"default_role_id": "role-20", "default_role_version": 4, "binding_version": "binding-v4",
				}
			} else {
				data = map[string]any{
					"user_id": "canonical-user-2", "assigned_role_ids": []string{"role-20"}, "default_role_id": "role-20",
					"binding_version": 5, "default_role_version": 5, "audit_event_id": "audit-1", "idempotent_replay": false,
				}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	invitation, err := workspace.InviteMember(context.Background(), "alice", 10, WithUserEmail("alice@example.test"), WithUserRoleIDs(10))
	if err != nil {
		t.Fatal(err)
	}
	if invitation.GetId() != "canonical-user-2" {
		t.Fatalf("invitation target id = %s", invitation.GetId())
	}
	if len(requests) != 1 || requests[0].path != "/newmoi/user/create" {
		t.Fatalf("invitation requests = %#v", requests)
	}
	// Acceptance is out of band; bind the handle only after active membership is observed.
	user, err := workspace.Member("canonical-user-2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := user.Info(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := user.Update(context.Background(), "Alice", "Finance", WithUserTag(1, "finance")); err != nil {
		t.Fatal(err)
	}
	bindings, err := workspace.IAM().UserRoleBindings(context.Background(), "canonical-user-2")
	if err != nil || bindings.GetDefaultRoleId() != "role-20" {
		t.Fatalf("role bindings = %#v, err = %v", bindings, err)
	}
	expectedBindingVersion, err := IAMBindingVersionNumber(bindings)
	if err != nil {
		t.Fatal(err)
	}
	expectedDefaultRoleVersion := bindings.GetDefaultRoleVersion()
	updatedBindings, err := workspace.IAM().PutUserRoleBindings(context.Background(), "canonical-user-2", &IAMUserRoleBindingsRequest{
		AssignedRoleIds: []string{"role-20"}, DefaultRoleId: "role-20", ExpectedBindingVersion: expectedBindingVersion,
		ExpectedDefaultRoleVersion: expectedDefaultRoleVersion, RequestId: "write-1",
	})
	if err != nil || updatedBindings.GetBindingVersion() != 5 {
		t.Fatalf("updated role bindings = %#v, err = %v", updatedBindings, err)
	}
	if err := user.UpdateStatus(context.Background(), "disable"); err != nil {
		t.Fatal(err)
	}
	if err := user.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.Member(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("workspace member err = %v", err)
	}

	want := []string{
		"/newmoi/user/create",
		"/newmoi/user/detail_info",
		"/newmoi/user/update_info",
		"/newmoi/workspaces/ws-1/iam/users/canonical-user-2/role-bindings",
		"/newmoi/workspaces/ws-1/iam/users/canonical-user-2/role-bindings",
		"/newmoi/user/update_status",
		"/newmoi/user/delete",
	}
	if len(requests) != len(want) {
		t.Fatalf("request count = %d, want %d", len(requests), len(want))
	}
	for i, path := range want {
		if requests[i].workspace != "ws-1" || requests[i].path != path {
			t.Fatalf("request %d = %#v, want %s", i, requests[i], path)
		}
		if i > 0 && i < 3 && requests[i].body["id"] != "canonical-user-2" {
			t.Fatalf("request %d user id = %#v, want canonical-user-2", i, requests[i].body["id"])
		}
	}
}
func TestAgentA2AUsesProductRequestShape(t *testing.T) {
	var gotMethod, gotPath, gotWorkspace string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotWorkspace = r.Header.Get(headerWorkspaceID)
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"kind": "task", "id": "task-1", "turnId": "turn-1"},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Agents("workspace 1").A2A(context.Background(), AgentA2ARequest{
		AgentSelector: AgentSelector{AgentCode: "explore"},
		A2ARequest: A2ARequest{
			JSONRPC: "2.0",
			ID:      "1",
			Method:  A2AMethodSend,
			Params:  json.RawMessage(`{"message":{"parts":[{"kind":"text","text":"hi"}]}}`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetJson() != `{"id":"task-1","kind":"task","turnId":"turn-1"}` {
		t.Fatalf("a2a response json = %q", resp.GetJson())
	}
	if gotMethod != http.MethodPost || gotPath != "/newmoi/agents/a2a" || gotWorkspace != "workspace 1" {
		t.Fatalf("request = method:%s path:%s workspace:%s", gotMethod, gotPath, gotWorkspace)
	}
	if gotBody["agent_code"] != "explore" || gotBody["jsonrpc"] != "2.0" || gotBody["id"] != "1" || gotBody["method"] != A2AMethodSend {
		t.Fatalf("body = %#v", gotBody)
	}
	params, ok := gotBody["params"].(map[string]any)
	if !ok {
		t.Fatalf("params = %#v", gotBody["params"])
	}
	message, ok := params["message"].(map[string]any)
	if !ok {
		t.Fatalf("message = %#v", params["message"])
	}
	parts, ok := message["parts"].([]any)
	if !ok || len(parts) != 1 {
		t.Fatalf("parts = %#v", message["parts"])
	}
}

func TestAgentA2AValidatesMethodMode(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := client.Agents("ws-1").A2A(ctx, AgentA2ARequest{
		A2ARequest: A2ARequest{JSONRPC: "2.0", Method: A2AMethodSend},
	}); !IsCode(err, ErrAgentRequired.Code) {
		t.Fatalf("agent err = %v", err)
	}
	if _, err := client.Agents("ws-1").A2A(ctx, AgentA2ARequest{
		AgentSelector: AgentSelector{AgentCode: "explore"},
		A2ARequest:    A2ARequest{JSONRPC: "1.0", Method: A2AMethodSend},
	}); !IsCode(err, ErrJSONRPCInvalid.Code) {
		t.Fatalf("jsonrpc err = %v", err)
	}
	if _, err := client.Agents("ws-1").A2A(ctx, AgentA2ARequest{
		AgentSelector: AgentSelector{AgentCode: "explore"},
		A2ARequest:    A2ARequest{JSONRPC: "2.0"},
	}); !IsCode(err, ErrMethodRequired.Code) {
		t.Fatalf("method err = %v", err)
	}
	if _, err := client.Agents("ws-1").A2A(ctx, AgentA2ARequest{
		AgentSelector: AgentSelector{AgentCode: "explore"},
		A2ARequest:    A2ARequest{JSONRPC: "2.0", Method: A2AMethodStream},
	}); !IsCode(err, ErrMethodInvalid.Code) {
		t.Fatalf("stream method err = %v", err)
	}
	if _, err := client.Agents("ws-1").A2AStream(ctx, AgentA2ARequest{
		AgentSelector: AgentSelector{AgentCode: "explore"},
		A2ARequest:    A2ARequest{JSONRPC: "2.0", Method: A2AMethodSend},
	}); !IsCode(err, ErrMethodInvalid.Code) {
		t.Fatalf("send stream err = %v", err)
	}
}

func TestWorkspaceAgentHandleScopesA2ASelector(t *testing.T) {
	var requests []map[string]any
	var workspaces []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, body)
		workspaces = append(workspaces, r.Header.Get(headerWorkspaceID))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"kind": "task", "id": "task-1", "turnId": "turn-1"},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("workspace 1")
	if err != nil {
		t.Fatal(err)
	}
	agentByCode, err := workspace.AgentByCode("explore")
	if err != nil {
		t.Fatalf("agent by code err = %v", err)
	}
	if agentByCode.Selector().AgentCode != "explore" {
		t.Fatalf("agent code selector = %#v", agentByCode.Selector())
	}
	resp, err := agentByCode.A2A(context.Background(), A2ARequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  A2AMethodSend,
		Params:  json.RawMessage(`{"message":{"parts":[{"kind":"text","text":"hi"}]}}`),
	})
	if err != nil {
		t.Fatalf("a2a by code err = %v", err)
	}
	if resp.GetJson() != `{"id":"task-1","kind":"task","turnId":"turn-1"}` {
		t.Fatalf("a2a response json = %q", resp.GetJson())
	}
	sentText, err := agentByCode.SendText(context.Background(), AgentTextMessage{
		RequestID:      "req-2",
		MessageID:      "msg-1",
		SessionID:      "9",
		Text:           "show revenue",
		IdempotencyKey: "idem-1",
		Metadata:       map[string]any{"source": "sdk-test"},
	})
	if err != nil {
		t.Fatalf("send text err = %v", err)
	}
	sentTaskID, err := AgentA2ATaskID(sentText)
	if err != nil {
		t.Fatalf("sent task id err = %v", err)
	}
	if sentTaskID != "task-1" {
		t.Fatalf("sent task id = %q", sentTaskID)
	}
	sentTurnID, err := AgentA2ATurnID(sentText)
	if err != nil {
		t.Fatalf("sent turn id err = %v", err)
	}
	if sentTurnID != "turn-1" {
		t.Fatalf("sent turn id = %q", sentTurnID)
	}
	stream, err := agentByCode.StreamText(context.Background(), AgentTextMessage{
		RequestID: "req-stream",
		MessageID: "msg-stream",
		SessionID: "10",
		Text:      "stream revenue",
	})
	if err != nil {
		t.Fatalf("stream text err = %v", err)
	}
	if _, err := io.ReadAll(stream.Body); err != nil {
		t.Fatalf("read stream body err = %v", err)
	}
	_ = stream.Body.Close()
	agentByID, err := workspace.AgentByID("agent-1")
	if err != nil {
		t.Fatalf("agent by id err = %v", err)
	}
	if agentByID.Selector().AgentID != "agent-1" {
		t.Fatalf("agent id selector = %#v", agentByID.Selector())
	}
	if _, err := agentByID.GetTask(context.Background(), AgentTaskRequest{RequestID: "2", TaskID: "task-1"}); err != nil {
		t.Fatalf("get task err = %v", err)
	}
	if _, err := agentByID.CancelTask(context.Background(), AgentTaskRequest{RequestID: "3", TaskID: "task-1"}); err != nil {
		t.Fatalf("cancel task err = %v", err)
	}
	afterSeq := 7
	resubscribe, err := agentByID.ResubscribeTask(context.Background(), AgentTaskRequest{RequestID: "4", TaskID: "task-1", AfterSeq: &afterSeq})
	if err != nil {
		t.Fatalf("resubscribe task err = %v", err)
	}
	if _, err := io.ReadAll(resubscribe.Body); err != nil {
		t.Fatalf("read resubscribe body err = %v", err)
	}
	_ = resubscribe.Body.Close()
	if _, err := agentByID.SubmitInput(context.Background(), AgentInputSubmission{
		RequestID: "5",
		TaskID:    "task-1",
		TurnID:    "turn-1",
		CallID:    "call-1",
		Answers:   map[string][]string{"approval": {"yes"}, "regions": {"us", "eu"}},
	}); err != nil {
		t.Fatalf("submit input err = %v", err)
	}
	if _, err := workspace.AgentByCode(""); !IsCode(err, ErrAgentRequired.Code) {
		t.Fatalf("empty agent code err = %v", err)
	}
	if _, err := workspace.AgentByID(""); !IsCode(err, ErrAgentRequired.Code) {
		t.Fatalf("empty agent id err = %v", err)
	}
	if _, err := agentByID.SubmitInput(context.Background(), AgentInputSubmission{}); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("empty input task err = %v", err)
	}
	if _, err := agentByID.SubmitInput(context.Background(), AgentInputSubmission{TaskID: "task-1"}); !IsCode(err, ErrTurnIDRequired.Code) {
		t.Fatalf("empty input turn err = %v", err)
	}
	if _, err := agentByID.SubmitInput(context.Background(), AgentInputSubmission{TaskID: "task-1", TurnID: "turn-1"}); !IsCode(err, ErrAnswersRequired.Code) {
		t.Fatalf("empty input answers err = %v", err)
	}
	missingTaskID := &AgentA2AResult{Json: `{"kind":"task"}`}
	if _, err := AgentA2ATaskID(missingTaskID); !IsCode(err, "ErrResponseInvalid") {
		t.Fatalf("missing task id err = %v", err)
	}
	missingTurnID := &AgentA2AResult{Json: `{"kind":"task","id":"task-1"}`}
	if _, err := AgentA2ATurnID(missingTurnID); !IsCode(err, "ErrResponseInvalid") {
		t.Fatalf("missing turn id err = %v", err)
	}
	if len(requests) != 7 {
		t.Fatalf("request count = %d: %#v", len(requests), requests)
	}
	for i, workspaceID := range workspaces {
		if workspaceID != "workspace 1" {
			t.Fatalf("request %d workspace = %q", i, workspaceID)
		}
	}
	if requests[0]["agent_code"] != "explore" || requests[0]["agent_id"] != nil || requests[0]["method"] != A2AMethodSend {
		t.Fatalf("request 0 = %#v", requests[0])
	}
	if requests[1]["agent_code"] != "explore" || requests[1]["method"] != A2AMethodSend {
		t.Fatalf("request 1 = %#v", requests[1])
	}
	if requests[2]["agent_code"] != "explore" || requests[2]["method"] != A2AMethodStream {
		t.Fatalf("request 2 = %#v", requests[2])
	}
	for index, sessionID := range []string{"9", "10"} {
		params, ok := requests[index+1]["params"].(map[string]any)
		if !ok {
			t.Fatalf("request %d params = %#v", index+1, requests[index+1]["params"])
		}
		metadata, ok := params["metadata"].(map[string]any)
		if !ok || metadata["session_id"] != sessionID {
			t.Fatalf("request %d metadata = %#v", index+1, params["metadata"])
		}
	}
	if requests[3]["agent_id"] != "agent-1" || requests[3]["agent_code"] != nil || requests[3]["method"] != A2AMethodTasksGet {
		t.Fatalf("request 3 = %#v", requests[3])
	}
	if requests[4]["agent_id"] != "agent-1" || requests[4]["agent_code"] != nil || requests[4]["method"] != A2AMethodTasksCancel {
		t.Fatalf("request 4 = %#v", requests[4])
	}
	if requests[5]["agent_id"] != "agent-1" || requests[5]["method"] != A2AMethodTasksResubscribe {
		t.Fatalf("request 5 = %#v", requests[5])
	}
	if requests[6]["agent_id"] != "agent-1" || requests[6]["method"] != A2AMethodInputSubmit {
		t.Fatalf("request 6 = %#v", requests[6])
	}
	taskParams, ok := requests[3]["params"].(map[string]any)
	if !ok || taskParams["id"] != "task-1" {
		t.Fatalf("get task params = %#v", requests[3]["params"])
	}
	cancelParams, ok := requests[4]["params"].(map[string]any)
	if !ok || cancelParams["id"] != "task-1" {
		t.Fatalf("cancel task params = %#v", requests[4]["params"])
	}
	resubscribeParams, ok := requests[5]["params"].(map[string]any)
	if !ok || resubscribeParams["id"] != "task-1" || resubscribeParams["afterSeq"] != float64(7) {
		t.Fatalf("resubscribe params = %#v", requests[5]["params"])
	}
	inputParams, ok := requests[6]["params"].(map[string]any)
	if !ok || inputParams["taskId"] != "task-1" || inputParams["turnId"] != "turn-1" || inputParams["callId"] != "call-1" {
		t.Fatalf("input params = %#v", requests[6]["params"])
	}
	inputAnswers, ok := inputParams["answers"].(map[string]any)
	if !ok {
		t.Fatalf("input answers = %#v", inputParams["answers"])
	}
	approval, ok := inputAnswers["approval"].(map[string]any)
	if !ok {
		t.Fatalf("approval answer = %#v", inputAnswers["approval"])
	}
	approvalValues, ok := approval["answers"].([]any)
	if !ok || len(approvalValues) != 1 || approvalValues[0] != "yes" {
		t.Fatalf("approval values = %#v", approval["answers"])
	}
	regions, ok := inputAnswers["regions"].(map[string]any)
	if !ok {
		t.Fatalf("regions answer = %#v", inputAnswers["regions"])
	}
	regionValues, ok := regions["answers"].([]any)
	if !ok || len(regionValues) != 2 || regionValues[0] != "us" || regionValues[1] != "eu" {
		t.Fatalf("region values = %#v", regions["answers"])
	}
	params, ok := requests[1]["params"].(map[string]any)
	if !ok || params["idempotencyKey"] != "idem-1" {
		t.Fatalf("send text params = %#v", requests[1]["params"])
	}
	message, ok := params["message"].(map[string]any)
	if !ok || message["messageId"] != "msg-1" || message["role"] != "user" {
		t.Fatalf("send text message = %#v", params["message"])
	}
	parts, ok := message["parts"].([]any)
	if !ok || len(parts) != 1 {
		t.Fatalf("send text parts = %#v", message["parts"])
	}
	part, ok := parts[0].(map[string]any)
	if !ok || part["kind"] != "text" || part["text"] != "show revenue" {
		t.Fatalf("send text part = %#v", parts[0])
	}
}

func TestWorkspaceAgentByCodeInWorkspaceScopesA2ASelector(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"kind": "task", "id": "task-1"},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("workspace-1")
	if err != nil {
		t.Fatal(err)
	}
	agent, err := workspace.AgentByCodeInWorkspace("workflow", "system")
	if err != nil {
		t.Fatal(err)
	}
	if selector := agent.Selector(); selector.AgentCode != "workflow" || selector.AgentWorkspaceID != "system" {
		t.Fatalf("selector = %#v", selector)
	}
	if _, err := agent.SendText(context.Background(), AgentTextMessage{
		RequestID: "request-1",
		MessageID: "message-1",
		Model:     "glm-5.2",
		Text:      "create a workflow",
	}); err != nil {
		t.Fatal(err)
	}
	if gotBody["agent_code"] != "workflow" || gotBody["agent_workspace_id"] != "system" || gotBody["model"] != "glm-5.2" {
		t.Fatalf("body = %#v", gotBody)
	}
	if _, err := workspace.AgentByCodeInWorkspace("workflow", ""); !IsCode(err, ErrWorkspaceRequired.Code) {
		t.Fatalf("empty agent workspace err = %v", err)
	}
}

func TestAgentResourceAnyUsesProductWorkspaceRoute(t *testing.T) {
	var gotMethod, gotPath, gotRawQuery, gotWorkspace string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotRawQuery = r.URL.RawQuery
		gotWorkspace = r.Header.Get(headerWorkspaceID)
		data := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/connectors/get":
			data = map[string]any{"id": "conn-1"}
		case "/newmoi/connectors":
			data = map[string]any{"id": "conn-1"}
		case "/newmoi/connectors/validate":
			data = map[string]any{"valid": true}
		case "/newmoi/connectors/db/database/list":
			data = map[string]any{"databases": []string{"db1"}}
		case "/newmoi/connectors/db/database/schema/list":
			data = map[string]any{"schemas": []string{"public"}}
		case "/newmoi/connectors/db/database/table/list":
			data = map[string]any{"tables": []string{"orders"}}
		case "/newmoi/connectors/db/schema":
			data = map[string]any{"columns": []map[string]any{{
				"name":           "id",
				"data_type":      "INT",
				"mapped_type":    "INT64",
				"is_primary_key": true,
				"is_nullable":    false,
				"is_auto_incr":   true,
			}}}
		case "/newmoi/connectors/file/download":
			data = map[string]any{"url": "https://example.test/cf-1"}
		case "/newmoi/connectors/file/preview":
			data = map[string]any{"conn_file_id": "cf-1", "file_type": 1, "rows": []map[string]any{}}
		case "/newmoi/connectors/mo/database/list":
			data = map[string]any{"databases": []string{"analytics"}}
		case "/newmoi/connectors/mo/database/table/list":
			data = map[string]any{"tables": []string{"orders"}}
		case "/newmoi/connectors/mo/check_table_name":
			data = map[string]any{"exists": false}
		case "/newmoi/connectors/mo/column/mapping":
			data = map[string]any{"column_map": map[string]any{"src_id": []any{"id"}}, "need_mapping_columns": []string{"src_id"}}
		case "/newmoi/task":
			data = map[string]any{"task_id": "task-1"}
		case "/newmoi/task/get":
			data = map[string]any{"task": map[string]any{"id": "task-1"}}
		case "/newmoi/task/runs":
			data = map[string]any{"runs": []map[string]any{}}
		case "/newmoi/task/files":
			data = map[string]any{"files": []map[string]any{}}
		case "/newmoi/export/task/create":
			data = map[string]any{"id": "export-1"}
		case "/newmoi/export/task/list":
			data = map[string]any{"tasks": []map[string]any{}, "total": 0}
		case "/newmoi/export/task/info":
			data = map[string]any{"id": "export-1"}
		case "/newmoi/export/task/files":
			data = map[string]any{"files": []map[string]any{}}
		case "/newmoi/export/task/export-1/state":
			data = map[string]any{"total": 0, "pending": 0, "running": 0, "failed": 0, "completed": 0}
		case "/newmoi/export/task/task-1/state":
			data = map[string]any{"total": 0, "pending": 0, "running": 0, "failed": 0, "completed": 0}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": data,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Agents("workspace 1").ResourceAny(
		context.Background(),
		http.MethodPatch,
		"agent-runtime-tasks",
		"/task 1",
		map[string]any{"status": "cancelled"},
		WithQueryParam("include", "events"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPatch {
		t.Fatalf("method = %s", gotMethod)
	}
	if gotPath != "/newmoi/workspaces/workspace%201/agent-runtime-tasks/task%201" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotRawQuery != "include=events" {
		t.Fatalf("query = %s", gotRawQuery)
	}
	if gotWorkspace != "workspace 1" {
		t.Fatalf("workspace = %q", gotWorkspace)
	}
}

func TestWeComCallbackSDKUsesWorkspaceRoutesAndPublicCallback(t *testing.T) {
	var publicCallbackSawAuth bool
	var callbackBody string
	var listedQuery string
	channelBodies := make(map[string]map[string]any)
	requests := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.EscapedPath())
		if strings.HasPrefix(r.URL.EscapedPath(), "/newmoi/callbacks/wecom/") {
			publicCallbackSawAuth = r.Header.Get(headerAPIKey) != "" || r.Header.Get(headerWorkspaceID) != ""
			body, _ := io.ReadAll(r.Body)
			callbackBody = string(body)
			_, _ = w.Write([]byte("success"))
			return
		}
		if r.Header.Get(headerAPIKey) != "user-key" {
			t.Fatalf("authenticated route %s missing api key: %q", r.URL.EscapedPath(), r.Header.Get(headerAPIKey))
		}
		if r.Header.Get(headerWorkspaceID) != "workspace 1" {
			t.Fatalf("authenticated route %s workspace header = %q", r.URL.EscapedPath(), r.Header.Get(headerWorkspaceID))
		}
		if (r.Method == http.MethodPost || r.Method == http.MethodPatch) && strings.HasPrefix(r.URL.EscapedPath(), "/newmoi/workspaces/workspace%201/channels/wecom/instances") {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			channelBodies[r.Method+" "+r.URL.EscapedPath()] = body
		}
		var data any = map[string]any{"ok": true}
		switch {
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/workspaces/workspace%201/channels/wecom/instances":
			data = map[string]any{"id": "inst_1", "workspace_id": "workspace 1", "provider": "wecom", "channel_type": "wecom_app", "name": "WeCom App", "status": "active", "credential_ref": "wecom_cred_1", "callback_path": "/newmoi/callbacks/wecom/workspace%201/wecom_cred_1", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"}
		case r.Method == http.MethodPatch && r.URL.EscapedPath() == "/newmoi/workspaces/workspace%201/channels/wecom/instances/inst_1":
			data = map[string]any{"id": "inst_1", "workspace_id": "workspace 1", "provider": "wecom", "channel_type": "wecom_app", "name": "WeCom App", "status": "active", "credential_ref": "wecom_cred_1", "callback_path": "/newmoi/callbacks/wecom/workspace%201/wecom_cred_1", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/workspaces/workspace%201/channels/wecom/instances/test":
			data = map[string]any{"instance": map[string]any{"id": "inst_1", "workspace_id": "workspace 1", "provider": "wecom", "channel_type": "wecom_app", "name": "WeCom App", "status": "active", "credential_ref": "wecom_cred_1", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"}, "ok": true, "status": "succeeded", "error_stage": "credentials"}
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/workspaces/workspace%201/connector-catalog":
			data = map[string]any{"items": []any{map[string]any{"provider": "bocha", "name": "博查 Web Search", "description": "配置博查 API Key 后使用网页搜索。", "configured": true}}}
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/workspaces/workspace%201/channels/wecom/instances":
			data = map[string]any{"items": []any{map[string]any{"id": "inst_1", "workspace_id": "workspace 1", "provider": "wecom", "channel_type": "wecom_app", "name": "WeCom App", "status": "active", "credential_ref": "wecom_cred_1", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"}}, "total": 1, "limit": 20, "offset": 0}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/workspaces/workspace%201/callback-rules":
			data = map[string]any{"id": "rule_1", "workspace_id": "workspace 1", "provider": "wecom", "credential_ref": "wecom_cred_1", "name": "rule", "enabled": true, "priority": 10, "match": map[string]any{}, "target_type": "agent_automation_task", "target_config": map[string]any{"automation_task_id": "auto_1"}, "stop_after_match": true, "version": 1}
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/workspaces/workspace%201/callback-rules":
			listedQuery = r.URL.RawQuery
			data = map[string]any{"items": []any{}, "total": 0, "limit": 20, "offset": 0}
		case r.Method == http.MethodPatch && r.URL.EscapedPath() == "/newmoi/workspaces/workspace%201/callback-rules/rule%201":
			data = map[string]any{"id": "rule 1", "workspace_id": "workspace 1", "provider": "wecom", "credential_ref": "wecom_cred_1", "name": "rule updated", "enabled": true, "priority": 10, "match": map[string]any{}, "target_type": "agent_automation_task", "target_config": map[string]any{"automation_task_id": "auto_1"}, "stop_after_match": true, "version": 2}
		case r.Method == http.MethodDelete && r.URL.EscapedPath() == "/newmoi/workspaces/workspace%201/callback-rules/rule%201":
			data = map[string]any{"deleted": true}
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	instance, err := client.Agents("workspace 1").CreateChannelInstance(ctx, callbackProviderWeCom, ChannelInstanceCreateRequest{
		ChannelType: "wecom_app",
		Name:        "WeCom App",
		Config: map[string]string{
			"corp_id":  "corp_1",
			"agent_id": "1000001",
		},
		Secrets: map[string]string{
			"secret":         "secret",
			"callback_token": "token",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if instance.CredentialRef != "wecom_cred_1" || instance.Status != "active" {
		t.Fatalf("instance = %+v", instance)
	}
	secret := "rotated-secret"
	if _, err := client.Agents("workspace 1").UpdateChannelInstance(ctx, callbackProviderWeCom, "inst_1", ChannelInstanceUpdateRequest{Secrets: map[string]string{"secret": secret}, ExpectedVersion: 2}); err != nil {
		t.Fatal(err)
	}
	tested, err := client.Agents("workspace 1").TestChannelConfiguration(ctx, callbackProviderWeCom, ChannelInstanceCreateRequest{
		ChannelType: "wecom_app",
		Name:        "WeCom App",
		Config: map[string]string{
			"corp_id":  "corp_1",
			"agent_id": "1000001",
		},
		Secrets: map[string]string{
			"secret":         "secret",
			"callback_token": "token",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !tested.OK || tested.ErrorStage != "credentials" {
		t.Fatalf("test result = %+v", tested)
	}
	if got, want := channelBodies["POST /newmoi/workspaces/workspace%201/channels/wecom/instances"], map[string]any{"channel_type": "wecom_app", "name": "WeCom App", "config": map[string]any{"corp_id": "corp_1", "agent_id": "1000001"}, "secrets": map[string]any{"secret": "secret", "callback_token": "token"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("create body = %#v, want %#v", got, want)
	}
	if got, want := channelBodies["PATCH /newmoi/workspaces/workspace%201/channels/wecom/instances/inst_1"], map[string]any{"secrets": map[string]any{"secret": secret}, "expected_version": float64(2)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("update body = %#v, want %#v", got, want)
	}
	if got, want := channelBodies["POST /newmoi/workspaces/workspace%201/channels/wecom/instances/test"], map[string]any{"channel_type": "wecom_app", "name": "WeCom App", "config": map[string]any{"corp_id": "corp_1", "agent_id": "1000001"}, "secrets": map[string]any{"secret": "secret", "callback_token": "token"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("configuration test body = %#v, want %#v", got, want)
	}
	if instances, err := client.Agents("workspace 1").ListChannelInstances(ctx, callbackProviderWeCom); err != nil {
		t.Fatal(err)
	} else if instances.Total != 1 || len(instances.Items) != 1 {
		t.Fatalf("instances = %+v", instances)
	}
	catalog, err := client.Agents("workspace 1").ListConnectorCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Items) != 1 || catalog.Items[0].Provider != "bocha" || !catalog.Items[0].Configured || catalog.Items[0].Name == "" || catalog.Items[0].Description == "" {
		t.Fatalf("catalog = %+v", catalog)
	}
	rule, err := client.Agents("workspace 1").CreateCallbackRule(ctx, CallbackRuleCreateRequest{
		Provider:      callbackProviderWeCom,
		CredentialRef: "wecom_cred_1",
		Name:          "rule",
		Priority:      10,
		Match:         CallbackRuleMatch{EventTypes: []string{"event"}},
		TargetType:    "agent_automation_task",
		TargetConfig:  map[string]any{"automation_task_id": "auto_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rule.ID != "rule_1" || rule.Provider != callbackProviderWeCom {
		t.Fatalf("rule = %+v", rule)
	}
	if _, err := client.Agents("workspace 1").ListCallbackRules(ctx, WithQueryParam("provider", "wecom"), WithQueryParam("credential_ref", "wecom_cred_1")); err != nil {
		t.Fatal(err)
	}
	if listedQuery != "credential_ref=wecom_cred_1&provider=wecom" {
		t.Fatalf("list query = %q", listedQuery)
	}
	name := "rule updated"
	if _, err := client.Agents("workspace 1").UpdateCallbackRule(ctx, "rule 1", CallbackRuleUpdateRequest{Name: &name, Version: 1}); err != nil {
		t.Fatal(err)
	}
	deleted, err := client.Agents("workspace 1").DeleteCallbackRule(ctx, "rule 1")
	if err != nil {
		t.Fatal(err)
	}
	if !deleted.Deleted {
		t.Fatalf("delete result = %+v", deleted)
	}
	body, err := client.PostWeComCallback(ctx, "workspace 1", "wecom cred 1", []byte("<xml/>"), WithQueryParam("timestamp", "100"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "success" || callbackBody != "<xml/>" {
		t.Fatalf("callback body response=%q request=%q", string(body), callbackBody)
	}
	if publicCallbackSawAuth {
		t.Fatal("public callback request must not send product auth or workspace headers")
	}
	if len(requests) != 10 {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestAgentResourceRejectsRelativeTailPath(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Agents("ws-1").ResourceGet(context.Background(), "agents", "relative")
	if !IsCode(err, ErrPathInvalid.Code) {
		t.Fatalf("err = %v", err)
	}
}

func TestCatalogTopLevelMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		var responseData any
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/create":
			responseData = map[string]any{"id": 10}
		case "/newmoi/catalog/info":
			responseData = map[string]any{"catalog": map[string]any{"id": 10, "name": "catalog-1", "description": "primary"}}
		case "/newmoi/catalog/ref_list":
			responseData = map[string]any{"list": []map[string]any{{"ref_id": "wf-1", "ref_type": "workflow", "volume_id": "10", "volume_name": "catalog", "table_id": "", "table_name": ""}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	createdCatalog, err := catalog.Create(context.Background(), "catalog-1", WithCatalogDescription("primary"))
	if err != nil {
		t.Fatal(err)
	}
	if createdCatalog.GetId() != 10 {
		t.Fatalf("created catalog = %#v", createdCatalog)
	}
	catalogInfo, err := catalog.Info(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if catalogInfo.GetCatalog().GetId() != 10 || catalogInfo.GetCatalog().GetName() != "catalog-1" {
		t.Fatalf("catalog info result = %#v", catalogInfo)
	}
	if err := catalog.Update(context.Background(), 10, "catalog-2", WithCatalogDescription("updated")); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Delete(context.Background(), 10); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 4 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/catalog/create" || requests[0].body["name"] != "catalog-1" || requests[0].body["description"] != "primary" {
		t.Fatalf("create catalog = %#v", requests[0])
	}
	if requests[1].path != "/newmoi/catalog/info" || requests[1].body["id"] != float64(10) {
		t.Fatalf("catalog info = %#v", requests[1])
	}
	if requests[2].path != "/newmoi/catalog/update" || requests[2].body["id"] != float64(10) ||
		requests[2].body["name"] != "catalog-2" || requests[2].body["description"] != "updated" {
		t.Fatalf("update catalog = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/catalog/delete" || requests[3].body["id"] != float64(10) {
		t.Fatalf("delete catalog = %#v", requests[3])
	}
}

func TestCatalogTopLevelMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	ctx := context.Background()
	if _, err := catalog.Create(ctx, ""); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("create err = %v", err)
	}
	if _, err := catalog.Info(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("info err = %v", err)
	}
	if err := catalog.Update(ctx, 0, "catalog"); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("update id err = %v", err)
	}
	if err := catalog.Update(ctx, 1, ""); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("update name err = %v", err)
	}
	if err := catalog.Delete(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("delete err = %v", err)
	}
}

func TestCatalogDatabaseMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		var responseData any
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/database/list":
			responseData = map[string]any{"list": []any{map[string]any{"id": 20, "name": "db_1", "display_name": "DB One", "description": "primary db", "display_description": "Primary DB", "created_at": "t", "updated_at": "t", "created_by": "u", "updated_by": "u", "allowed_actions": []string{"database.update_database"}}}}
		case "/newmoi/catalog/database/create", "/newmoi/catalog/database/update":
			responseData = map[string]any{"id": 20}
		case "/newmoi/catalog/database/info":
			responseData = map[string]any{"database": map[string]any{"id": 20, "name": "db_1", "display_name": "DB One", "description": "primary db", "display_description": "Primary DB"}}
		case "/newmoi/catalog/database/children":
			responseData = map[string]any{"list": []any{map[string]any{"id": "30", "name": "volume-1", "display_name": "Landing", "type": "volume", "children_count": 0, "size": 0, "created_at": "t", "created_by": "u", "updated_at": "t", "updated_by": "u", "description": "primary volume", "display_description": "Primary volume", "reserved": false, "allowed_actions": []string{"volume.update"}}}}
		case "/newmoi/catalog/database/ref_list":
			responseData = map[string]any{"list": []map[string]any{{"ref_id": "wf-2", "ref_type": "workflow", "volume_id": "20", "volume_name": "database", "table_id": "", "table_name": ""}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	databases, err := catalog.ListDatabases(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(databases.GetList()) != 1 || databases.GetList()[0].GetId() != 20 ||
		databases.GetList()[0].GetDisplayName() != "DB One" ||
		len(databases.GetList()[0].GetAllowedActions()) != 1 {
		t.Fatalf("databases result = %#v", databases)
	}
	createdDatabase, err := catalog.CreateDatabase(context.Background(), 10, "db_1", WithCatalogDescription("primary db"))
	if err != nil {
		t.Fatal(err)
	}
	if createdDatabase.GetId() != 20 {
		t.Fatalf("created database = %#v", createdDatabase)
	}
	databaseInfo, err := catalog.DatabaseInfo(context.Background(), 20)
	if err != nil {
		t.Fatal(err)
	}
	if databaseInfo.GetDatabase().GetName() != "db_1" ||
		databaseInfo.GetDatabase().GetDisplayDescription() != "Primary DB" {
		t.Fatalf("database info result = %#v", databaseInfo)
	}
	updatedDatabase, err := catalog.UpdateDatabaseDescription(context.Background(), 20, WithCatalogDescription("updated db"))
	if err != nil {
		t.Fatal(err)
	}
	if updatedDatabase.GetId() != 20 {
		t.Fatalf("updated database = %#v", updatedDatabase)
	}
	if err := catalog.DeleteDatabase(context.Background(), 20); err != nil {
		t.Fatal(err)
	}
	children, err := catalog.DatabaseChildren(context.Background(), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(children.GetList()) != 1 || children.GetList()[0].GetType() != "volume" ||
		children.GetList()[0].GetDisplayName() != "Landing" ||
		len(children.GetList()[0].GetAllowedActions()) != 1 {
		t.Fatalf("database children = %#v", children)
	}

	if len(requests) != 6 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/catalog/database/list" || requests[0].body["id"] != float64(10) {
		t.Fatalf("database list = %#v", requests[0])
	}
	if requests[1].path != "/newmoi/catalog/database/create" || requests[1].body["catalog_id"] != float64(10) ||
		requests[1].body["name"] != "db_1" || requests[1].body["description"] != "primary db" {
		t.Fatalf("database create = %#v", requests[1])
	}
	if requests[2].path != "/newmoi/catalog/database/info" || requests[2].body["id"] != float64(20) {
		t.Fatalf("database info = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/catalog/database/update" || requests[3].body["id"] != float64(20) ||
		requests[3].body["description"] != "updated db" {
		t.Fatalf("database update = %#v", requests[3])
	}
	if requests[4].path != "/newmoi/catalog/database/delete" || requests[4].body["id"] != float64(20) {
		t.Fatalf("database delete = %#v", requests[4])
	}
	if requests[5].path != "/newmoi/catalog/database/children" || requests[5].body["id"] != float64(20) {
		t.Fatalf("database children = %#v", requests[5])
	}
}

func TestCatalogDatabaseMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	ctx := context.Background()
	if _, err := catalog.ListDatabases(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("list err = %v", err)
	}
	if _, err := catalog.CreateDatabase(ctx, 0, "db_1"); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("create id err = %v", err)
	}
	if _, err := catalog.CreateDatabase(ctx, 10, ""); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("create name err = %v", err)
	}
	if _, err := catalog.DatabaseInfo(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("info err = %v", err)
	}
	if _, err := catalog.UpdateDatabase(ctx, 0, "db_2"); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("update id err = %v", err)
	}
	if _, err := catalog.UpdateDatabase(ctx, 20, "db_2"); !IsCode(err, ErrDatabaseRenameUnsupported.Code) {
		t.Fatalf("update rename unsupported err = %v", err)
	}
	if _, err := catalog.UpdateDatabaseDescription(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("update description id err = %v", err)
	}
	if err := catalog.DeleteDatabase(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("delete err = %v", err)
	}
	if _, err := catalog.DatabaseChildren(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("children err = %v", err)
	}
}

func TestCatalogVolumeMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		var responseData any
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/volume/create", "/newmoi/catalog/volume/update":
			responseData = map[string]any{"id": 30}
		case "/newmoi/catalog/volume/info":
			responseData = map[string]any{"volume": map[string]any{"id": 30, "name": "volume-1", "description": "primary volume"}}
		case "/newmoi/catalog/volume/ref_list":
			responseData = map[string]any{"list": []map[string]any{{"ref_id": "wf-3", "ref_type": "workflow", "volume_id": "30", "volume_name": "volume", "table_id": "", "table_name": ""}}}
		case "/newmoi/catalog/volume/full_path":
			responseData = map[string]any{"path": "catalog/db/volume"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	createdVolume, err := catalog.CreateVolume(context.Background(), 20, "volume-1", WithCatalogDescription("primary volume"))
	if err != nil {
		t.Fatal(err)
	}
	if createdVolume.GetId() != 30 {
		t.Fatalf("created volume = %#v", createdVolume)
	}
	volumeInfo, err := catalog.VolumeInfo(context.Background(), "volume-30")
	if err != nil {
		t.Fatal(err)
	}
	if volumeInfo.GetVolume().GetName() != "volume-1" {
		t.Fatalf("volume info result = %#v", volumeInfo)
	}
	updatedVolume, err := catalog.UpdateVolume(context.Background(), 30, "volume-2", WithCatalogDescription("updated volume"))
	if err != nil {
		t.Fatal(err)
	}
	if updatedVolume.GetId() != 30 {
		t.Fatalf("updated volume = %#v", updatedVolume)
	}
	if err := catalog.DeleteVolume(context.Background(), 30); err != nil {
		t.Fatal(err)
	}
	if fullPath, err := catalog.VolumeFullPath(context.Background(), 30); err != nil {
		t.Fatal(err)
	} else if fullPath.GetPath() != "catalog/db/volume" {
		t.Fatalf("volume full path = %#v", fullPath)
	}
	if len(requests) != 5 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/catalog/volume/create" || requests[0].body["database_id"] != float64(20) ||
		requests[0].body["name"] != "volume-1" || requests[0].body["description"] != "primary volume" {
		t.Fatalf("volume create = %#v", requests[0])
	}
	if requests[1].path != "/newmoi/catalog/volume/info" || requests[1].body["id"] != "volume-30" {
		t.Fatalf("volume info = %#v", requests[1])
	}
	if requests[2].path != "/newmoi/catalog/volume/update" || requests[2].body["id"] != float64(30) ||
		requests[2].body["name"] != "volume-2" || requests[2].body["description"] != "updated volume" {
		t.Fatalf("volume update = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/catalog/volume/delete" || requests[3].body["id"] != float64(30) {
		t.Fatalf("volume delete = %#v", requests[3])
	}
	if requests[4].path != "/newmoi/catalog/volume/full_path" || requests[4].body["id"] != float64(30) {
		t.Fatalf("volume full path = %#v", requests[4])
	}
}

func TestCatalogVolumeMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	ctx := context.Background()
	if _, err := catalog.CreateVolume(ctx, 0, "volume"); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("create id err = %v", err)
	}
	if _, err := catalog.CreateVolume(ctx, 20, ""); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("create name err = %v", err)
	}
	if _, err := catalog.VolumeInfo(ctx, ""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("info err = %v", err)
	}
	if _, err := catalog.UpdateVolume(ctx, 0, "volume"); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("update id err = %v", err)
	}
	if _, err := catalog.UpdateVolume(ctx, 30, ""); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("update name err = %v", err)
	}
	if err := catalog.DeleteVolume(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("delete err = %v", err)
	}
	if _, err := catalog.VolumeFullPath(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("full path err = %v", err)
	}
}

func TestCatalogTableSimpleMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		responseData := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/table/multi_info":
			responseData = map[string]any{"tables": []any{map[string]any{"name": "table_1", "columns": []any{map[string]any{"name": "id", "type": "INT"}}}}}
		case "/newmoi/catalog/table/ref_list":
			responseData = map[string]any{"list": []map[string]any{{"ref_id": "wf-4", "ref_type": "workflow", "volume_id": "30", "volume_name": "volume", "table_id": "31", "table_name": "table"}}}
		case "/newmoi/catalog/table/exist":
			responseData = map[string]any{"data": true}
		case "/newmoi/catalog/table/full_path":
			responseData = map[string]any{"path": "catalog/db/table"}
		case "/newmoi/catalog/table/overview":
			responseData = map[string]any{"tables": []any{map[string]any{"name": "table_1", "columns": []any{"id"}}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	if _, err := catalog.MultiTableInfo(context.Background(), 31, 32); err != nil {
		t.Fatal(err)
	}
	if err := catalog.DeleteTable(context.Background(), 31); err != nil {
		t.Fatal(err)
	}
	resp, err := catalog.DownloadTable(context.Background(), 31)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if exists, err := catalog.TableExists(context.Background(), 20, "table_1"); err != nil {
		t.Fatal(err)
	} else if !exists.GetData() {
		t.Fatalf("table exists = %#v", exists)
	}
	if fullPath, err := catalog.TableFullPath(context.Background(), 31); err != nil {
		t.Fatal(err)
	} else if fullPath.GetPath() != "catalog/db/table" {
		t.Fatalf("table full path = %#v", fullPath)
	}
	if err := catalog.TruncateTable(context.Background(), 31); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.TableOverview(context.Background(), 20); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 7 {
		t.Fatalf("request count = %d", len(requests))
	}
	tableIDs, ok := requests[0].body["ids"].([]any)
	if requests[0].path != "/newmoi/catalog/table/multi_info" || !ok || len(tableIDs) != 2 ||
		tableIDs[0] != float64(31) || tableIDs[1] != float64(32) {
		t.Fatalf("multi table info = %#v", requests[0])
	}
	if requests[1].path != "/newmoi/catalog/table/delete" || requests[1].body["id"] != float64(31) {
		t.Fatalf("delete table = %#v", requests[1])
	}
	if requests[2].path != "/newmoi/catalog/table/download" || requests[2].body["id"] != float64(31) {
		t.Fatalf("download table = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/catalog/table/exist" || requests[3].body["database_id"] != float64(20) ||
		requests[3].body["name"] != "table_1" {
		t.Fatalf("table exists = %#v", requests[3])
	}
	if requests[4].path != "/newmoi/catalog/table/full_path" || requests[4].body["id"] != float64(31) {
		t.Fatalf("table full path = %#v", requests[4])
	}
	if requests[5].path != "/newmoi/catalog/table/truncate" || requests[5].body["id"] != float64(31) {
		t.Fatalf("truncate table = %#v", requests[5])
	}
	if requests[6].path != "/newmoi/catalog/table/overview" || requests[6].body["id"] != float64(20) {
		t.Fatalf("table overview = %#v", requests[6])
	}
}

func TestCatalogTableSimpleMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	ctx := context.Background()
	if err := catalog.DeleteTable(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("delete err = %v", err)
	}
	if _, err := catalog.DownloadTable(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("download err = %v", err)
	}
	if _, err := catalog.TableExists(ctx, 0, "table"); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("exists id err = %v", err)
	}
	if _, err := catalog.TableExists(ctx, 20, ""); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("exists name err = %v", err)
	}
	if _, err := catalog.TableFullPath(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("full path err = %v", err)
	}
	if err := catalog.TruncateTable(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("truncate err = %v", err)
	}
	if _, err := catalog.TableOverview(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("overview err = %v", err)
	}
}

func TestCatalogTableComplexMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		responseData := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/table/create":
			responseData = map[string]any{"id": 31}
		case "/newmoi/catalog/table/info":
			responseData = map[string]any{"name": "orders", "columns": []any{map[string]any{"name": "id", "type": "INT", "is_pk": true, "comment": "primary id", "precision": []any{10, 0}, "default": "0"}}}
		case "/newmoi/catalog/table/data", "/newmoi/catalog/table/preview":
			responseData = map[string]any{"data": []any{[]any{"1"}}, "columns": []any{map[string]any{"name": "id", "type": "INT"}}, "total_rows": 1, "page": 2, "page_size": 50}
		case "/newmoi/catalog/table/download_data":
			responseData = map[string]any{"url": "https://example.test/table.csv"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	isPK := true
	defaultValue := "0"
	columns := []TableColumn{{
		Name:      "id",
		Type:      "INT",
		IsPK:      &isPK,
		Comment:   "primary id",
		Precision: []int{10, 0},
		Default:   &defaultValue,
	}}
	if _, err := catalog.CreateTable(
		context.Background(),
		20,
		"orders",
		columns,
		WithTableComment("order table"),
		WithTableDataFileURL("s3://bucket/orders.csv"),
		WithColumnLoadOption("id", "0"),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.TableInfo(
		context.Background(),
		31,
		WithTableLookupName("orders"),
		WithTableLookupDatabaseID(20),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.TableData(
		context.Background(),
		31,
		WithTableDataPage(2),
		WithTableDataPageSize(50),
		WithTableDataName("orders"),
		WithTableDataDatabaseID(20),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.PreviewTable(context.Background(), 31, WithTablePreviewLines(25)); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 4 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/catalog/table/create" ||
		requests[0].body["database_id"] != float64(20) ||
		requests[0].body["name"] != "orders" ||
		requests[0].body["comment"] != "order table" ||
		requests[0].body["data_file_url"] != "s3://bucket/orders.csv" {
		t.Fatalf("create table = %#v", requests[0])
	}
	gotColumns, ok := requests[0].body["columns"].([]any)
	if !ok || len(gotColumns) != 1 {
		t.Fatalf("columns = %#v", requests[0].body["columns"])
	}
	gotColumn, ok := gotColumns[0].(map[string]any)
	if !ok ||
		gotColumn["name"] != "id" ||
		gotColumn["type"] != "INT" ||
		gotColumn["is_pk"] != true ||
		gotColumn["comment"] != "primary id" ||
		gotColumn["default"] != "0" {
		t.Fatalf("column = %#v", gotColumns[0])
	}
	gotPrecision, ok := gotColumn["precision"].([]any)
	if !ok || len(gotPrecision) != 2 || gotPrecision[0] != float64(10) || gotPrecision[1] != float64(0) {
		t.Fatalf("precision = %#v", gotColumn["precision"])
	}
	loadOptions, ok := requests[0].body["column_load_options"].([]any)
	if !ok || len(loadOptions) != 1 {
		t.Fatalf("load options = %#v", requests[0].body["column_load_options"])
	}
	loadOption, ok := loadOptions[0].(map[string]any)
	if !ok || loadOption["column"] != "id" || loadOption["data_from"] != "0" {
		t.Fatalf("load option = %#v", loadOptions[0])
	}
	if requests[1].path != "/newmoi/catalog/table/info" ||
		requests[1].body["id"] != float64(31) ||
		requests[1].body["name"] != "orders" ||
		requests[1].body["database_id"] != float64(20) {
		t.Fatalf("table info = %#v", requests[1])
	}
	if requests[2].path != "/newmoi/catalog/table/data" ||
		requests[2].body["id"] != float64(31) ||
		requests[2].body["page"] != float64(2) ||
		requests[2].body["page_size"] != float64(50) ||
		requests[2].body["name"] != "orders" ||
		requests[2].body["database_id"] != float64(20) {
		t.Fatalf("table data = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/catalog/table/preview" ||
		requests[3].body["id"] != float64(31) ||
		requests[3].body["preview_lines"] != float64(25) {
		t.Fatalf("preview table = %#v", requests[3])
	}
}

func TestCatalogCreateTableValidatesRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	ctx := context.Background()
	columns := []TableColumn{{Name: "id", Type: "INT"}}
	if _, err := catalog.CreateTable(ctx, 0, "orders", columns); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("database id err = %v", err)
	}
	if _, err := catalog.CreateTable(ctx, 20, "", columns); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("name err = %v", err)
	}
	if _, err := catalog.CreateTable(ctx, 20, "orders", nil); !IsCode(err, ErrColumnsRequired.Code) {
		t.Fatalf("columns err = %v", err)
	}
}

func TestWorkspaceCatalogTableHandleScopesDatabaseAndTable(t *testing.T) {
	var requests []struct {
		path        string
		workspaceID string
		body        map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path        string
			workspaceID string
			body        map[string]any
		}{path: r.URL.EscapedPath(), workspaceID: r.Header.Get("X-Workspace-ID"), body: body})
		responseData := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/table/create":
			responseData = map[string]any{"id": 31}
		case "/newmoi/catalog/table/exist":
			responseData = map[string]any{"data": true}
		case "/newmoi/catalog/table/overview":
			responseData = map[string]any{"tables": []any{map[string]any{"name": "orders", "columns": []any{"id"}}}}
		case "/newmoi/catalog/table/info":
			responseData = map[string]any{"name": "orders", "columns": []any{map[string]any{"name": "id", "type": "INT"}}}
		case "/newmoi/catalog/table/data", "/newmoi/catalog/table/preview":
			responseData = map[string]any{"data": []any{[]any{"1"}}, "columns": []any{map[string]any{"name": "id", "type": "INT"}}, "total_rows": 1, "page": 2, "page_size": 50}
		case "/newmoi/catalog/table/download_data":
			responseData = map[string]any{"url": "https://example.test/table.csv"}
		case "/newmoi/catalog/table/full_path":
			responseData = map[string]any{"path": "orders/raw/orders"}
		case "/newmoi/catalog/table/ref_list":
			responseData = map[string]any{"list": []any{}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	database := &DatabaseHandle{catalog: &CatalogHandle{workspace: workspace, id: 10, name: "orders"}, id: 20, name: "raw"}
	columns := []TableColumn{{Name: "id", Type: "INT"}}
	table, created, err := database.CreateTable(
		context.Background(),
		"orders",
		columns,
		WithTableComment("order table"),
		WithTableDataFileURL("s3://bucket/orders.csv"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if table.ID() != 31 || table.Name() != "orders" || created == nil {
		t.Fatalf("table = %#v created = %#v", table, created)
	}
	known, err := database.Table(31)
	if err != nil {
		t.Fatal(err)
	}
	if known.ID() != 31 {
		t.Fatalf("known table = %#v", known)
	}
	if exists, err := database.TableExists(context.Background(), "orders"); err != nil {
		t.Fatal(err)
	} else if !exists.GetData() {
		t.Fatalf("exists = %#v", exists)
	}
	if _, err := database.TableOverview(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Info(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Data(context.Background(), WithTableDataPage(2), WithTableDataPageSize(50)); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Preview(context.Background(), WithTablePreviewLines(25)); err != nil {
		t.Fatal(err)
	}
	if path, err := table.FullPath(context.Background()); err != nil {
		t.Fatal(err)
	} else if path.GetPath() != "orders/raw/orders" {
		t.Fatalf("path = %#v", path)
	}
	if err := table.Truncate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := table.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 9 {
		t.Fatalf("request count = %d", len(requests))
	}
	for i, req := range requests {
		if req.workspaceID != "ws-1" {
			t.Fatalf("request %d workspace = %q", i, req.workspaceID)
		}
	}
	expectedPaths := []string{
		"/newmoi/catalog/table/create",
		"/newmoi/catalog/table/exist",
		"/newmoi/catalog/table/overview",
		"/newmoi/catalog/table/info",
		"/newmoi/catalog/table/data",
		"/newmoi/catalog/table/preview",
		"/newmoi/catalog/table/full_path",
		"/newmoi/catalog/table/truncate",
		"/newmoi/catalog/table/delete",
	}
	for i, path := range expectedPaths {
		if requests[i].path != path {
			t.Fatalf("request %d path = %s body = %#v", i, requests[i].path, requests[i].body)
		}
	}
	if requests[0].body["database_id"] != float64(20) ||
		requests[0].body["name"] != "orders" ||
		requests[0].body["comment"] != "order table" ||
		requests[0].body["data_file_url"] != "s3://bucket/orders.csv" {
		t.Fatalf("create body = %#v", requests[0].body)
	}
	if requests[1].body["database_id"] != float64(20) || requests[1].body["name"] != "orders" {
		t.Fatalf("exists body = %#v", requests[1].body)
	}
	if requests[2].body["id"] != float64(20) {
		t.Fatalf("overview body = %#v", requests[2].body)
	}
	if requests[4].body["id"] != float64(31) ||
		requests[4].body["page"] != float64(2) ||
		requests[4].body["page_size"] != float64(50) {
		t.Fatalf("data body = %#v", requests[4].body)
	}
	if requests[5].body["id"] != float64(31) || requests[5].body["preview_lines"] != float64(25) {
		t.Fatalf("preview body = %#v", requests[5].body)
	}
}

func TestWorkspaceCatalogTableHandleValidatesRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	database := &DatabaseHandle{catalog: &CatalogHandle{workspace: workspace, id: 10, name: "orders"}, id: 20, name: "raw"}
	ctx := context.Background()
	columns := []TableColumn{{Name: "id", Type: "INT"}}
	if _, err := database.Table(0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("table id err = %v", err)
	}
	if _, _, err := database.CreateTable(ctx, "", columns); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("create table name err = %v", err)
	}
	if _, _, err := database.CreateTable(ctx, "orders", nil); !IsCode(err, ErrColumnsRequired.Code) {
		t.Fatalf("create table columns err = %v", err)
	}
}

func TestWorkspaceFolderHandleScopesVolumeFolderLifecycle(t *testing.T) {
	var requests []struct {
		path        string
		workspaceID string
		body        map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path        string
			workspaceID string
			body        map[string]any
		}{path: r.URL.EscapedPath(), workspaceID: r.Header.Get("X-Workspace-ID"), body: body})
		responseData := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/folder/create":
			responseData = map[string]any{"id": 31}
		case "/newmoi/catalog/folder/ref_list":
			responseData = map[string]any{"list": []map[string]any{{"ref_id": "wf-1", "ref_type": "workflow"}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	volume := &VolumeHandle{
		database: &DatabaseHandle{catalog: &CatalogHandle{workspace: workspace, id: 10, name: "orders"}, id: 20, name: "raw"},
		id:       30,
		name:     "landing",
	}

	folder, created, err := volume.CreateFolder(context.Background(), "staging", WithFolderDescription("temporary files"))
	if err != nil {
		t.Fatal(err)
	}
	if folder.ID() != 31 || folder.Name() != "staging" || created.GetId() != 31 {
		t.Fatalf("folder = %#v created = %#v", folder, created)
	}
	known, err := volume.Folder(31)
	if err != nil {
		t.Fatal(err)
	}
	if known.ID() != 31 {
		t.Fatalf("known folder = %#v", known)
	}
	if renamed, err := folder.Rename(context.Background(), "staging-2", WithUpdatedFolderDescription("updated")); err != nil {
		t.Fatal(err)
	} else if renamed.Name() != "staging-2" {
		t.Fatalf("renamed folder = %#v", renamed)
	}
	if err := folder.Clean(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := folder.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 4 {
		t.Fatalf("request count = %d", len(requests))
	}
	for i, req := range requests {
		if req.workspaceID != "ws-1" {
			t.Fatalf("request %d workspace = %q", i, req.workspaceID)
		}
	}
	expectedPaths := []string{
		"/newmoi/catalog/folder/create",
		"/newmoi/catalog/folder/update",
		"/newmoi/catalog/folder/clean",
		"/newmoi/catalog/folder/delete",
	}
	for i, path := range expectedPaths {
		if requests[i].path != path {
			t.Fatalf("request %d path = %s body = %#v", i, requests[i].path, requests[i].body)
		}
	}
	if requests[0].body["parent_volume_id"] != float64(30) ||
		requests[0].body["database_id"] != float64(20) ||
		requests[0].body["name"] != "staging" ||
		requests[0].body["description"] != "temporary files" {
		t.Fatalf("create body = %#v", requests[0].body)
	}
	if requests[1].body["id"] != float64(31) ||
		requests[1].body["name"] != "staging-2" ||
		requests[1].body["description"] != "updated" {
		t.Fatalf("update body = %#v", requests[1].body)
	}
	if requests[2].body["id"] != float64(31) ||
		requests[3].body["id"] != float64(31) {
		t.Fatalf("folder id bodies = %#v %#v", requests[2].body, requests[3].body)
	}
	if _, err := volume.Folder(0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("folder id err = %v", err)
	}
}

func TestWorkspaceFileHandleScopesVolumeFileLifecycle(t *testing.T) {
	var requests []struct {
		path        string
		workspaceID string
		body        map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path        string
			workspaceID string
			body        map[string]any
		}{path: r.URL.EscapedPath(), workspaceID: r.Header.Get("X-Workspace-ID"), body: body})
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/file/download":
			_, _ = w.Write([]byte("download-body"))
			return
		case "/newmoi/catalog/file/preview_stream":
			_, _ = w.Write([]byte("preview-body"))
			return
		}
		responseData := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/file/list":
			responseData = map[string]any{"total": 1, "list": []any{map[string]any{"id": "file-1", "name": "orders.csv", "file_type": "file", "volume_id": "30"}}}
		case "/newmoi/catalog/file/info":
			responseData = map[string]any{"file_id": "file-1", "file_name": "orders.csv", "size": 16}
		case "/newmoi/catalog/file/preview_link":
			responseData = map[string]any{"url": "https://example.test/preview/file-1"}
		case "/newmoi/catalog/file/preview_excel_stream":
			responseData = map[string]any{"link": "https://example.test/preview/file-1.xlsx"}
		case "/newmoi/catalog/file/trigger_workflow":
			responseData = map[string]any{"triggered": 1}
		case "/newmoi/catalog/files/file-1/data-asset":
			responseData = map[string]any{
				"asset": map[string]any{
					"asset_id":    "asset-1",
					"name":        "orders.csv",
					"raw_file_id": "file-1",
					"volume_id":   30,
					"asset_type":  "file",
					"asset_ref":   "file-1",
				},
				"derivations": []any{map[string]any{"kind": "parsed", "target_asset_id": "asset-parsed-1"}},
				"manifest":    map[string]any{"parsed_file_id": "parsed-file-1", "source_file_id": "file-1"},
			}
		case "/newmoi/catalog/file-artifacts/file-1":
			responseData = map[string]any{"file_id": "file-1", "file_exists": true, "has_artifact": true, "source_file_id": "file-1", "root_asset": map[string]any{"asset_id": "asset-1"}}
		case "/newmoi/export/volumes/30/files":
			responseData = map[string]any{"items": []any{}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	volume := &VolumeHandle{
		database: &DatabaseHandle{catalog: &CatalogHandle{workspace: workspace, id: 10, name: "orders"}, id: 20, name: "raw"},
		id:       30,
		name:     "landing",
	}
	file, err := volume.File("file-1")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := volume.Files(context.Background(), WithFileListPageSize(25)); err != nil {
		t.Fatal(err)
	}
	if info, err := file.Info(context.Background()); err != nil {
		t.Fatal(err)
	} else if info.GetFileName() != "orders.csv" {
		t.Fatalf("info = %#v", info)
	}
	download, err := file.Download(context.Background(), WithFileAsZip(true))
	if err != nil {
		t.Fatal(err)
	}
	downloadBody, err := io.ReadAll(download.Body)
	_ = download.Body.Close()
	if err != nil || string(downloadBody) != "download-body" {
		t.Fatalf("download body = %q err = %v", string(downloadBody), err)
	}
	preview, err := file.Preview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	previewBody, err := io.ReadAll(preview.Body)
	_ = preview.Body.Close()
	if err != nil || string(previewBody) != "preview-body" {
		t.Fatalf("preview body = %q err = %v", string(previewBody), err)
	}
	if triggered, err := file.TriggerWorkflows(context.Background()); err != nil {
		t.Fatal(err)
	} else if triggered.GetTriggered() != 1 {
		t.Fatalf("triggered = %#v", triggered)
	}
	if asset, err := file.DataAsset(context.Background()); err != nil {
		t.Fatal(err)
	} else if asset.GetAsset().GetAssetId() != "asset-1" ||
		asset.GetAsset().GetRawFileId() != "file-1" ||
		len(asset.GetDerivations()) != 1 ||
		asset.GetManifest().GetParsedFileId() != "parsed-file-1" {
		t.Fatalf("asset = %#v", asset)
	}
	if artifact, err := file.Artifact(context.Background()); err != nil {
		t.Fatal(err)
	} else if artifact.GetFileId() != "file-1" || !artifact.GetHasArtifact() {
		t.Fatalf("artifact = %#v", artifact)
	}
	if err := file.DeleteRef(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := file.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 9 {
		t.Fatalf("request count = %d", len(requests))
	}
	for i, req := range requests {
		if req.workspaceID != "ws-1" {
			t.Fatalf("request %d workspace = %q", i, req.workspaceID)
		}
	}
	expectedPaths := []string{
		"/newmoi/catalog/file/list",
		"/newmoi/catalog/file/info",
		"/newmoi/catalog/file/download",
		"/newmoi/catalog/file/preview_stream",
		"/newmoi/catalog/file/trigger_workflow",
		"/newmoi/catalog/files/file-1/data-asset",
		"/newmoi/catalog/file-artifacts/file-1",
		"/newmoi/catalog/file/delete_ref",
		"/newmoi/catalog/file/delete",
	}
	for i, path := range expectedPaths {
		if requests[i].path != path {
			t.Fatalf("request %d path = %s body = %#v", i, requests[i].path, requests[i].body)
		}
	}
	filters, ok := requests[0].body["filters"].([]any)
	if !ok || len(filters) != 1 {
		t.Fatalf("filters = %#v", requests[0].body["filters"])
	}
	filter, ok := filters[0].(map[string]any)
	if !ok || filter["name"] != "volume_id" {
		t.Fatalf("volume filter = %#v", filters[0])
	}
	values, ok := filter["values"].([]any)
	if !ok || len(values) != 1 || values[0] != "30" {
		t.Fatalf("volume filter values = %#v", filter["values"])
	}
	if requests[0].body["page_size"] != float64(25) {
		t.Fatalf("file list body = %#v", requests[0].body)
	}
	if requests[2].body["volume_id"] != "30" || requests[2].body["file_id"] != "file-1" || requests[2].body["as_zip"] != true {
		t.Fatalf("download body = %#v", requests[2].body)
	}
	if requests[3].body["volume_id"] != "30" || requests[3].body["file_id"] != "file-1" {
		t.Fatalf("preview body = %#v", requests[3].body)
	}
	if requests[4].body["volume_id"] != float64(30) {
		t.Fatalf("trigger body = %#v", requests[4].body)
	}
	if requests[7].body["volume_id"] != float64(30) || requests[8].body["volume_id"] != "30" {
		t.Fatalf("delete bodies = %#v %#v", requests[7].body, requests[8].body)
	}
}

func TestCatalogFileMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		responseData := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/file/list":
			responseData = map[string]any{"total": 1, "list": []any{map[string]any{"id": "f1", "name": "report.csv", "file_type": "file", "volume_id": "30"}}}
		case "/newmoi/catalog/file/info":
			responseData = map[string]any{"file_id": "f1", "file_name": "report.csv", "size": 128}
		case "/newmoi/catalog/file/trigger_workflow":
			responseData = map[string]any{"triggered": 2}
		case "/newmoi/catalog/file/preview_link":
			responseData = map[string]any{"url": "http://example.test/preview"}
		case "/newmoi/catalog/file/preview_excel_stream":
			responseData = map[string]any{"link": "http://example.test/report.xlsx"}
		case "/newmoi/catalog/data-assets/batch-get":
			responseData = map[string]any{
				"items": []any{map[string]any{
					"asset_id":    "asset-1",
					"raw_file_id": "f1",
					"volume_id":   30,
					"asset_type":  "file",
					"asset_ref":   "f1",
				}},
				"total": 1,
			}
		case "/newmoi/catalog/file-artifacts:batchResolve":
			responseData = map[string]any{"items": []any{map[string]any{"file_id": "f1", "file_exists": true, "has_artifact": true, "source_file_id": "f1", "root_asset_id": "asset-1"}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	if _, err := catalog.Files(
		context.Background(),
		WithFileListPage(2),
		WithFileListPageSize(50),
		WithFileListOrder("desc"),
		WithFileListOrderBy("created_at"),
		WithFileListFilter("volume_id", "30"),
		WithFuzzyFileListFilter("file_name", true, "report"),
	); err != nil {
		t.Fatal(err)
	}
	if err := catalog.CreateFile(context.Background(), 30, "f1", "f2"); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.FileInfo(context.Background(), "f1"); err != nil {
		t.Fatal(err)
	}
	if err := catalog.DeleteFile(context.Background(), "f1", "30"); err != nil {
		t.Fatal(err)
	}
	if err := catalog.DeleteFileRef(context.Background(), 30, "f1", "f2"); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.TriggerFileWorkflow(context.Background(), 30, "f1", "f2"); err != nil {
		t.Fatal(err)
	}
	download, err := catalog.DownloadFile(context.Background(), "30", "f1", WithFileAsZip(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = download.Body.Close()
	preview, err := catalog.PreviewFile(context.Background(), "30", "f1")
	if err != nil {
		t.Fatal(err)
	}
	_ = preview.Body.Close()
	if assets, err := catalog.BatchGetDataAssets(context.Background(), "f1", "f2"); err != nil {
		t.Fatal(err)
	} else if assets.GetTotal() != 1 || len(assets.GetItems()) != 1 || assets.GetItems()[0].GetRawFileId() != "f1" {
		t.Fatalf("batch data assets = %#v", assets)
	}
	if _, err := catalog.BatchResolveFileArtifacts(context.Background(), "f1", "f2"); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 10 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/catalog/file/list" ||
		requests[0].body["page"] != float64(2) ||
		requests[0].body["page_size"] != float64(50) ||
		requests[0].body["order"] != "desc" ||
		requests[0].body["order_by"] != "created_at" {
		t.Fatalf("file list = %#v", requests[0])
	}
	filters, ok := requests[0].body["filters"].([]any)
	if !ok || len(filters) != 2 {
		t.Fatalf("filters = %#v", requests[0].body["filters"])
	}
	filter0, ok := filters[0].(map[string]any)
	if !ok || filter0["name"] != "volume_id" {
		t.Fatalf("filter0 = %#v", filters[0])
	}
	values0, ok := filter0["values"].([]any)
	if !ok || len(values0) != 1 || values0[0] != "30" {
		t.Fatalf("filter0 values = %#v", filter0["values"])
	}
	filter1, ok := filters[1].(map[string]any)
	if !ok || filter1["name"] != "file_name" || filter1["fuzzy"] != true {
		t.Fatalf("filter1 = %#v", filters[1])
	}
	if requests[1].path != "/newmoi/catalog/file/create" ||
		requests[1].body["volume_id"] != float64(30) {
		t.Fatalf("create file = %#v", requests[1])
	}
	fileIDs, ok := requests[1].body["file_ids"].([]any)
	if !ok || len(fileIDs) != 2 || fileIDs[0] != "f1" || fileIDs[1] != "f2" {
		t.Fatalf("create file ids = %#v", requests[1].body["file_ids"])
	}
	if requests[2].path != "/newmoi/catalog/file/info" || requests[2].body["file_id"] != "f1" {
		t.Fatalf("file info = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/catalog/file/delete" ||
		requests[3].body["id"] != "f1" ||
		requests[3].body["volume_id"] != "30" {
		t.Fatalf("delete file = %#v", requests[3])
	}
	if requests[4].path != "/newmoi/catalog/file/delete_ref" ||
		requests[4].body["volume_id"] != float64(30) {
		t.Fatalf("delete file ref = %#v", requests[4])
	}
	deleteRefIDs, ok := requests[4].body["file_ids"].([]any)
	if !ok || len(deleteRefIDs) != 2 || deleteRefIDs[0] != "f1" || deleteRefIDs[1] != "f2" {
		t.Fatalf("delete ref file ids = %#v", requests[4].body["file_ids"])
	}
	if requests[5].path != "/newmoi/catalog/file/trigger_workflow" ||
		requests[5].body["volume_id"] != float64(30) {
		t.Fatalf("trigger file workflow = %#v", requests[5])
	}
	triggerFileIDs, ok := requests[5].body["file_ids"].([]any)
	if !ok || len(triggerFileIDs) != 2 || triggerFileIDs[0] != "f1" || triggerFileIDs[1] != "f2" {
		t.Fatalf("trigger file ids = %#v", requests[5].body["file_ids"])
	}
	if requests[6].path != "/newmoi/catalog/file/download" ||
		requests[6].body["volume_id"] != "30" ||
		requests[6].body["file_id"] != "f1" ||
		requests[6].body["as_zip"] != true {
		t.Fatalf("download file = %#v", requests[6])
	}
	if requests[7].path != "/newmoi/catalog/file/preview_stream" ||
		requests[7].body["volume_id"] != "30" ||
		requests[7].body["file_id"] != "f1" {
		t.Fatalf("preview file = %#v", requests[7])
	}
	if requests[8].path != "/newmoi/catalog/data-assets/batch-get" {
		t.Fatalf("batch data assets = %#v", requests[8])
	}
	rawFileIDs, ok := requests[8].body["raw_file_ids"].([]any)
	if !ok || len(rawFileIDs) != 2 || rawFileIDs[0] != "f1" || rawFileIDs[1] != "f2" {
		t.Fatalf("raw file ids = %#v", requests[8].body["raw_file_ids"])
	}
	if requests[9].path != "/newmoi/catalog/file-artifacts:batchResolve" {
		t.Fatalf("batch artifacts = %#v", requests[9])
	}
	artifactFileIDs, ok := requests[9].body["file_ids"].([]any)
	if !ok || len(artifactFileIDs) != 2 || artifactFileIDs[0] != "f1" || artifactFileIDs[1] != "f2" {
		t.Fatalf("artifact file ids = %#v", requests[9].body["file_ids"])
	}
}

func TestCatalogFileMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	ctx := context.Background()
	if err := catalog.CreateFile(ctx, 0, "f1"); !IsCode(err, ErrVolumeIDRequired.Code) {
		t.Fatalf("create volume err = %v", err)
	}
	if err := catalog.CreateFile(ctx, 30); !IsCode(err, ErrFileIDsRequired.Code) {
		t.Fatalf("create file ids err = %v", err)
	}
	if _, err := catalog.FileInfo(ctx, ""); !IsCode(err, ErrFileIDRequired.Code) {
		t.Fatalf("info err = %v", err)
	}
	if err := catalog.DeleteFile(ctx, "", "30"); !IsCode(err, ErrFileIDRequired.Code) {
		t.Fatalf("delete err = %v", err)
	}
	if err := catalog.DeleteFileRef(ctx, 0, "f1"); !IsCode(err, ErrVolumeIDRequired.Code) {
		t.Fatalf("delete ref volume err = %v", err)
	}
	if err := catalog.DeleteFileRef(ctx, 30); !IsCode(err, ErrFileIDsRequired.Code) {
		t.Fatalf("delete ref ids err = %v", err)
	}
	if _, err := catalog.TriggerFileWorkflow(ctx, 0, "f1"); !IsCode(err, ErrVolumeIDRequired.Code) {
		t.Fatalf("trigger volume err = %v", err)
	}
	if _, err := catalog.TriggerFileWorkflow(ctx, 30); !IsCode(err, ErrFileIDsRequired.Code) {
		t.Fatalf("trigger ids err = %v", err)
	}
	if _, err := catalog.DownloadFile(ctx, "", "f1"); !IsCode(err, ErrVolumeIDRequired.Code) {
		t.Fatalf("download volume err = %v", err)
	}
	if _, err := catalog.DownloadFile(ctx, "30", ""); !IsCode(err, ErrFileIDRequired.Code) {
		t.Fatalf("download file err = %v", err)
	}
	if _, err := catalog.PreviewFile(ctx, "", "f1"); !IsCode(err, ErrVolumeIDRequired.Code) {
		t.Fatalf("preview volume err = %v", err)
	}
	if _, err := catalog.PreviewFile(ctx, "30", ""); !IsCode(err, ErrFileIDRequired.Code) {
		t.Fatalf("preview file err = %v", err)
	}
	if _, err := catalog.BatchGetDataAssets(ctx); !IsCode(err, ErrFileIDsRequired.Code) {
		t.Fatalf("batch assets err = %v", err)
	}
	if _, err := catalog.BatchResolveFileArtifacts(ctx); !IsCode(err, ErrFileIDsRequired.Code) {
		t.Fatalf("batch artifacts err = %v", err)
	}
}

func TestCatalogDataAssetMethodsReturnTypedResults(t *testing.T) {
	var requests []struct {
		method string
		path   string
		body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method string
			path   string
			body   map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), body: body})
		var responseData any
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/data-assets/asset-1":
			responseData = map[string]any{
				"asset": map[string]any{
					"asset_id":    "asset-1",
					"name":        "orders.csv",
					"raw_file_id": "file-1",
					"volume_id":   30,
					"asset_type":  "file",
					"asset_ref":   "file-1",
					"meta":        map[string]any{"source": "workflow"},
				},
				"derivations":  []any{map[string]any{"kind": "parsed", "source_asset_id": "asset-1", "target_asset_id": "asset-parsed-1"}},
				"manifest":     map[string]any{"root_asset_id": "asset-1", "source_file_id": "file-1", "parsed_asset_id": "asset-parsed-1"},
				"assets_by_id": map[string]any{"asset-1": map[string]any{"asset_id": "asset-1", "asset_ref": "file-1"}},
			}
		case "/newmoi/catalog/files/file-1/data-asset":
			responseData = map[string]any{"asset": map[string]any{"asset_id": "asset-1", "raw_file_id": "file-1"}}
		case "/newmoi/catalog/data-assets/batch-get":
			responseData = map[string]any{
				"items": []any{
					map[string]any{"asset_id": "asset-1", "raw_file_id": "file-1", "asset_ref": "file-1"},
				},
				"total": 1,
			}
		default:
			t.Fatalf("path = %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": responseData})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	asset, err := catalog.DataAsset(context.Background(), "asset-1")
	if err != nil {
		t.Fatal(err)
	}
	if asset.GetAsset().GetAssetId() != "asset-1" ||
		asset.GetAsset().GetMeta().GetFields()["source"].GetStringValue() != "workflow" ||
		len(asset.GetDerivations()) != 1 ||
		asset.GetManifest().GetParsedAssetId() != "asset-parsed-1" ||
		asset.GetAssetsById()["asset-1"].GetAssetRef() != "file-1" {
		t.Fatalf("asset = %#v", asset)
	}
	fileAsset, err := catalog.FileDataAsset(context.Background(), "file-1")
	if err != nil {
		t.Fatal(err)
	}
	if fileAsset.GetAsset().GetRawFileId() != "file-1" {
		t.Fatalf("file asset = %#v", fileAsset)
	}
	batch, err := catalog.BatchGetDataAssets(context.Background(), "file-1", "file-2")
	if err != nil {
		t.Fatal(err)
	}
	if batch.GetTotal() != 1 || batch.GetItems()[0].GetRawFileId() != "file-1" {
		t.Fatalf("batch = %#v", batch)
	}
	if len(requests) != 3 ||
		requests[0].method != http.MethodGet ||
		requests[0].path != "/newmoi/catalog/data-assets/asset-1" ||
		requests[1].path != "/newmoi/catalog/files/file-1/data-asset" ||
		requests[2].path != "/newmoi/catalog/data-assets/batch-get" {
		t.Fatalf("requests = %#v", requests)
	}
	rawFileIDs, ok := requests[2].body["raw_file_ids"].([]any)
	if !ok || len(rawFileIDs) != 2 || rawFileIDs[0] != "file-1" || rawFileIDs[1] != "file-2" {
		t.Fatalf("raw file ids = %#v", requests[2].body["raw_file_ids"])
	}
}

func TestCatalogFolderMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		responseData := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/catalog/folder/create":
			responseData = map[string]any{"id": 31}
		case "/newmoi/catalog/folder/ref_list":
			responseData = map[string]any{"list": []map[string]any{{"ref_id": "wf-1", "ref_type": "workflow", "volume_id": "31", "volume_name": "folder", "table_id": "", "table_name": ""}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	if folder, err := catalog.CreateFolder(context.Background(), 30, 20, "folder", WithFolderDescription("folder description")); err != nil {
		t.Fatal(err)
	} else if folder.GetId() != 31 {
		t.Fatalf("created folder = %#v", folder)
	}
	if err := catalog.UpdateFolder(context.Background(), 31, WithFolderName("folder-2"), WithUpdatedFolderDescription("updated")); err != nil {
		t.Fatal(err)
	}
	if err := catalog.DeleteFolder(context.Background(), 31); err != nil {
		t.Fatal(err)
	}
	if err := catalog.CleanFolder(context.Background(), 31); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 4 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/catalog/folder/create" ||
		requests[0].body["parent_volume_id"] != float64(30) ||
		requests[0].body["database_id"] != float64(20) ||
		requests[0].body["name"] != "folder" ||
		requests[0].body["description"] != "folder description" {
		t.Fatalf("create folder = %#v", requests[0])
	}
	if requests[1].path != "/newmoi/catalog/folder/update" ||
		requests[1].body["id"] != float64(31) ||
		requests[1].body["name"] != "folder-2" ||
		requests[1].body["description"] != "updated" {
		t.Fatalf("update folder = %#v", requests[1])
	}
	if requests[2].path != "/newmoi/catalog/folder/delete" || requests[2].body["id"] != float64(31) {
		t.Fatalf("delete folder = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/catalog/folder/clean" || requests[3].body["id"] != float64(31) {
		t.Fatalf("clean folder = %#v", requests[3])
	}
}

func TestCatalogFolderMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	ctx := context.Background()
	if _, err := catalog.CreateFolder(ctx, 0, 20, "folder"); !IsCode(err, ErrVolumeIDRequired.Code) {
		t.Fatalf("create parent err = %v", err)
	}
	if _, err := catalog.CreateFolder(ctx, 30, 20, ""); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("create name err = %v", err)
	}
	if err := catalog.UpdateFolder(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("update err = %v", err)
	}
	if err := catalog.DeleteFolder(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("delete err = %v", err)
	}
	if err := catalog.CleanFolder(ctx, 0); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("clean err = %v", err)
	}
}

func TestWorkspaceMemberServiceUserTagMethodsUseCanonicalRoutes(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		responseData := map[string]any{"id": 7}
		if r.URL.EscapedPath() == "/newmoi/user/tag/list" {
			responseData = map[string]any{"total": 1, "tag_list": []map[string]any{{"id": 7, "name": "tag1", "description": "canonical subject attribute"}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": responseData,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	members := client.WorkspaceMembers("ws-1")
	ctx := context.Background()
	if result, err := members.CreateUserTag(ctx, "tag1"); err != nil {
		t.Fatal(err)
	} else if result.GetId() != 7 {
		t.Fatalf("create user tag id = %d, want 7", result.GetId())
	}
	if tags, err := members.UserTags(ctx); err != nil {
		t.Fatal(err)
	} else if len(tags.GetTagList()) != 1 || tags.GetTagList()[0].GetName() != "tag1" || tags.GetTagList()[0].GetDescription() != "canonical subject attribute" {
		t.Fatalf("tags = %#v", tags)
	}
	if result, err := members.UpdateUserTag(ctx, 7, "tag2"); err != nil {
		t.Fatal(err)
	} else if result.GetId() != 7 {
		t.Fatalf("update user tag id = %d, want 7", result.GetId())
	}
	if result, err := members.DeleteUserTag(ctx, 7); err != nil {
		t.Fatal(err)
	} else if result.GetId() != 7 {
		t.Fatalf("delete user tag id = %d, want 7", result.GetId())
	}

	if len(requests) != 4 {
		t.Fatalf("request count = %d", len(requests))
	}
	for index, path := range []string{
		"/newmoi/user/tag/create",
		"/newmoi/user/tag/list",
		"/newmoi/user/tag/update",
		"/newmoi/user/tag/delete",
	} {
		if requests[index].path != path {
			t.Fatalf("request[%d] path = %s, want %s", index, requests[index].path, path)
		}
	}
	if requests[1].body == nil || len(requests[1].body) != 0 {
		t.Fatalf("user tag list body = %#v, want explicit empty JSON object", requests[1].body)
	}
}

func TestCatalogNL2SQLMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := client.Catalog("ws-1")
	ctx := context.Background()
	if _, err := catalog.RunNL2SQL(ctx, 0, "SELECT 1"); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("nl2sql id err = %v", err)
	}
	if _, err := catalog.RunNL2SQL(ctx, 20, ""); !IsCode(err, ErrSQLRequired.Code) {
		t.Fatalf("nl2sql sql err = %v", err)
	}
}

func TestCatalogRunNL2SQLReturnsTypedRows(t *testing.T) {
	var request struct {
		path      string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		request = struct {
			path      string
			workspace string
			body      map[string]any
		}{path: r.URL.EscapedPath(), workspace: r.Header.Get(headerWorkspaceID), body: body}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"columns": []string{"id", "amount"},
				"rows":    [][]any{{"1", "42.5"}},
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Catalog("ws-1").RunNL2SQL(context.Background(), 20, "select id, amount from orders")
	if err != nil {
		t.Fatal(err)
	}
	if request.path != "/newmoi/catalog/nl2sql/run_sql" || request.workspace != "ws-1" {
		t.Fatalf("request = %#v", request)
	}
	if request.body["database_id"] != float64(20) || request.body["sql"] != "select id, amount from orders" {
		t.Fatalf("body = %#v", request.body)
	}
	if len(result.GetColumns()) != 2 || result.GetColumns()[0] != "id" || len(result.GetRows()) != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestConnectorMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		method string
		path   string
		body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method string
			path   string
			body   map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), body: body})
		data := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/connectors/get":
			data = map[string]any{"id": "conn-1"}
		case "/newmoi/connectors":
			data = map[string]any{"id": "conn-1"}
		case "/newmoi/connectors/validate":
			data = map[string]any{"valid": true}
		case "/newmoi/connectors/db/database/list":
			data = map[string]any{"databases": []string{"db1"}}
		case "/newmoi/connectors/db/database/schema/list":
			data = map[string]any{"schemas": []string{"public"}}
		case "/newmoi/connectors/db/database/table/list":
			data = map[string]any{"tables": []string{"orders"}}
		case "/newmoi/connectors/db/schema":
			data = map[string]any{"columns": []map[string]any{{
				"name":           "id",
				"data_type":      "INT",
				"mapped_type":    "INT64",
				"is_primary_key": true,
				"is_nullable":    false,
				"is_auto_incr":   true,
			}}}
		case "/newmoi/connectors/file/download":
			data = map[string]any{"url": "https://example.test/cf-1"}
		case "/newmoi/connectors/file/preview":
			data = map[string]any{"conn_file_id": "cf-1", "file_type": 1, "rows": []map[string]any{}}
		case "/newmoi/connectors/mo/database/list":
			data = map[string]any{"databases": []string{"analytics"}}
		case "/newmoi/connectors/mo/database/table/list":
			data = map[string]any{"tables": []string{"orders"}}
		case "/newmoi/connectors/mo/check_table_name":
			data = map[string]any{"exists": false}
		case "/newmoi/connectors/mo/column/mapping":
			data = map[string]any{"column_map": map[string]any{"src_id": []any{"id"}}, "need_mapping_columns": []string{"src_id"}}
		case "/newmoi/task":
			data = map[string]any{"task_id": "task-1"}
		case "/newmoi/task/get":
			data = map[string]any{"task": map[string]any{"id": "task-1"}}
		case "/newmoi/task/runs":
			data = map[string]any{"runs": []map[string]any{}}
		case "/newmoi/task/files":
			data = map[string]any{"files": []map[string]any{}}
		case "/newmoi/export/task/create":
			data = map[string]any{"id": "export-1"}
		case "/newmoi/export/task/info":
			data = map[string]any{"id": "export-1"}
		case "/newmoi/export/task/files":
			data = map[string]any{"files": []map[string]any{}}
		case "/newmoi/export/task/export-1/state":
			data = map[string]any{"total": 0, "pending": 0, "running": 0, "failed": 0, "completed": 0}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": data,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	connectors := client.Connectors("ws-1")
	config := map[string]any{"mysql": map[string]any{"database": "db1"}}
	if _, err := connectors.Get(context.Background(), "conn-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := connectors.Create(context.Background(), "mysql-source", 1, 3, WithConnectorConfig(config)); err != nil {
		t.Fatal(err)
	}
	if err := connectors.Update(
		context.Background(),
		"conn-1",
		WithConnectorName("mysql-source-2"),
		WithConnectorSourceType(1),
		WithConnectorUsageType(1),
		WithConnectorUpdateConfig(config),
	); err != nil {
		t.Fatal(err)
	}
	validated, err := connectors.Validate(context.Background(), "mysql-source", 1, 3, WithConnectorConfig(config))
	if err != nil {
		t.Fatal(err)
	}
	if !validated.GetValid() {
		t.Fatalf("validated = %#v", validated)
	}
	if _, err := connectors.PreviewFile(
		context.Background(),
		"conn-1",
		"/data/file.csv",
		WithConnectorPreviewConnFileID("cf-1"),
		WithConnectorPreviewColumnName(true),
		WithConnectorPreviewRowStart(2),
		WithConnectorPreviewColumnNameRow(1),
		WithConnectorPreviewCSV(",", "\"", true),
		WithConnectorPreviewFileType(1),
	); err != nil {
		t.Fatal(err)
	}
	dbs, err := connectors.DBListDatabases(context.Background(), "conn-1")
	if err != nil {
		t.Fatal(err)
	}
	if got := dbs.GetDatabases(); len(got) != 1 || got[0] != "db1" {
		t.Fatalf("source databases = %#v", got)
	}
	schemas, err := connectors.DBListSchemas(context.Background(), "conn-1", "db1")
	if err != nil {
		t.Fatal(err)
	}
	if got := schemas.GetSchemas(); len(got) != 1 || got[0] != "public" {
		t.Fatalf("source schemas = %#v", got)
	}
	tables, err := connectors.DBListTables(context.Background(), "conn-1", "db1", WithConnectorSchema("public"))
	if err != nil {
		t.Fatal(err)
	}
	if got := tables.GetTables(); len(got) != 1 || got[0] != "orders" {
		t.Fatalf("source tables = %#v", got)
	}
	sourceSchema, err := connectors.DBSourceSchema(context.Background(), "conn-1", "db1", "orders", WithConnectorSchema("public"))
	if err != nil {
		t.Fatal(err)
	}
	if got := sourceSchema.GetColumns(); len(got) != 1 || got[0].GetName() != "id" || got[0].GetMappedType() != "INT64" || !got[0].GetIsPrimaryKey() || got[0].GetIsNullable() || !got[0].GetIsAutoIncr() {
		t.Fatalf("source schema = %#v", got)
	}
	download, err := connectors.FileDownload(context.Background(), "cf-1")
	if err != nil {
		t.Fatal(err)
	}
	if download.GetUrl() != "https://example.test/cf-1" {
		t.Fatalf("download url = %q", download.GetUrl())
	}
	if err := connectors.FileDelete(context.Background(), "cf-1"); err != nil {
		t.Fatal(err)
	}
	moDatabases, err := connectors.MODatabaseList(context.Background(), "conn-1")
	if err != nil {
		t.Fatal(err)
	}
	if got := moDatabases.GetDatabases(); len(got) != 1 || got[0] != "analytics" {
		t.Fatalf("mo databases = %#v", got)
	}
	moTables, err := connectors.MODatabaseTableList(context.Background(), "conn-1", "db1")
	if err != nil {
		t.Fatal(err)
	}
	if got := moTables.GetTables(); len(got) != 1 || got[0] != "orders" {
		t.Fatalf("mo tables = %#v", got)
	}
	checkedTable, err := connectors.MOCheckTableName(context.Background(), "conn-1", "db1", "orders")
	if err != nil {
		t.Fatal(err)
	}
	if checkedTable.GetExists() {
		t.Fatalf("checked table = %#v", checkedTable)
	}
	columnMapping, err := connectors.MOColumnMapping(context.Background(), "conn-1", "db1", "orders", map[string]string{"id": "INT"})
	if err != nil {
		t.Fatal(err)
	}
	if got := columnMapping.GetColumnMap()["src_id"].GetValues(); len(got) != 1 || got[0].GetStringValue() != "id" || len(columnMapping.GetNeedMappingColumns()) != 1 || columnMapping.GetNeedMappingColumns()[0] != "src_id" {
		t.Fatalf("column mapping = %#v", columnMapping)
	}

	if len(requests) != 15 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/connectors/get" || requests[0].body["id"] != "conn-1" {
		t.Fatalf("connector get = %#v", requests[0])
	}
	if requests[1].path != "/newmoi/connectors" ||
		requests[1].body["name"] != "mysql-source" ||
		requests[1].body["source_type"] != float64(1) ||
		requests[1].body["usage_type"] != float64(3) {
		t.Fatalf("connector create = %#v", requests[1])
	}
	if _, ok := requests[1].body["config"].(map[string]any); !ok {
		t.Fatalf("connector config = %#v", requests[1].body["config"])
	}
	if requests[2].method != http.MethodPut ||
		requests[2].path != "/newmoi/connectors/conn-1" ||
		requests[2].body["name"] != "mysql-source-2" ||
		requests[2].body["source_type"] != float64(1) ||
		requests[2].body["usage_type"] != float64(1) {
		t.Fatalf("connector update = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/connectors/validate" ||
		requests[3].body["name"] != "mysql-source" ||
		requests[3].body["source_type"] != float64(1) ||
		requests[3].body["usage_type"] != float64(3) {
		t.Fatalf("connector validate = %#v", requests[3])
	}
	if requests[4].path != "/newmoi/connectors/file/preview" ||
		requests[4].body["connector_id"] != "conn-1" ||
		requests[4].body["uri"] != "/data/file.csv" ||
		requests[4].body["conn_file_id"] != "cf-1" ||
		requests[4].body["isColumnName"] != true ||
		requests[4].body["rowStart"] != float64(2) ||
		requests[4].body["columnNameRow"] != float64(1) ||
		requests[4].body["file_type"] != float64(1) {
		t.Fatalf("connector preview = %#v", requests[4])
	}
	csvConfig, ok := requests[4].body["csv"].(map[string]any)
	if !ok || csvConfig["separator"] != "," || csvConfig["delimiter"] != "\"" || csvConfig["isEscape"] != true {
		t.Fatalf("csv config = %#v", requests[4].body["csv"])
	}
	if requests[5].path != "/newmoi/connectors/db/database/list" || requests[5].body["connector_id"] != "conn-1" {
		t.Fatalf("db list = %#v", requests[5])
	}
	if requests[6].path != "/newmoi/connectors/db/database/schema/list" ||
		requests[6].body["connector_id"] != "conn-1" ||
		requests[6].body["database"] != "db1" {
		t.Fatalf("schema list = %#v", requests[6])
	}
	if requests[7].path != "/newmoi/connectors/db/database/table/list" ||
		requests[7].body["connector_id"] != "conn-1" ||
		requests[7].body["database"] != "db1" ||
		requests[7].body["schema"] != "public" {
		t.Fatalf("table list = %#v", requests[7])
	}
	if requests[8].path != "/newmoi/connectors/db/schema" ||
		requests[8].body["connector_id"] != "conn-1" ||
		requests[8].body["database"] != "db1" ||
		requests[8].body["schema"] != "public" ||
		requests[8].body["table"] != "orders" {
		t.Fatalf("source schema = %#v", requests[8])
	}
	if requests[9].path != "/newmoi/connectors/file/download" ||
		requests[9].body["conn_file_id"] != "cf-1" {
		t.Fatalf("file download = %#v", requests[9])
	}
	if requests[10].path != "/newmoi/connectors/file/delete" ||
		requests[10].body["conn_file_id"] != "cf-1" {
		t.Fatalf("file delete = %#v", requests[10])
	}
	if requests[11].path != "/newmoi/connectors/mo/database/list" ||
		requests[11].body["connector_id"] != "conn-1" {
		t.Fatalf("mo database list = %#v", requests[11])
	}
	if requests[12].path != "/newmoi/connectors/mo/database/table/list" ||
		requests[12].body["connector_id"] != "conn-1" ||
		requests[12].body["database"] != "db1" {
		t.Fatalf("mo table list = %#v", requests[12])
	}
	if requests[13].path != "/newmoi/connectors/mo/check_table_name" ||
		requests[13].body["connector_id"] != "conn-1" ||
		requests[13].body["database"] != "db1" ||
		requests[13].body["table"] != "orders" {
		t.Fatalf("mo check table = %#v", requests[13])
	}
	if requests[14].path != "/newmoi/connectors/mo/column/mapping" ||
		requests[14].body["connector_id"] != "conn-1" ||
		requests[14].body["database"] != "db1" ||
		requests[14].body["table"] != "orders" {
		t.Fatalf("mo column mapping = %#v", requests[14])
	}
	exportColumns, ok := requests[14].body["export_columns"].(map[string]any)
	if !ok || exportColumns["id"] != "INT" {
		t.Fatalf("export columns = %#v", requests[14].body["export_columns"])
	}
}

func TestConnectorMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	connectors := client.Connectors("ws-1")
	ctx := context.Background()
	if _, err := connectors.Get(ctx, ""); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("get err = %v", err)
	}
	if _, err := connectors.Create(ctx, "", 1, 1); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("create err = %v", err)
	}
	if err := connectors.Update(ctx, ""); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("update err = %v", err)
	}
	if err := connectors.Delete(ctx, ""); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("delete err = %v", err)
	}
	if _, err := connectors.Validate(ctx, "", 1, 1); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("validate err = %v", err)
	}
	if _, err := connectors.PreviewFile(ctx, "", "/file.csv"); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("preview err = %v", err)
	}
	if _, err := connectors.DBListDatabases(ctx, ""); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("db list err = %v", err)
	}
	if _, err := connectors.DBListSchemas(ctx, "", "db1"); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("schema connector err = %v", err)
	}
	if _, err := connectors.DBListSchemas(ctx, "conn-1", ""); !IsCode(err, ErrDatabaseRequired.Code) {
		t.Fatalf("schema database err = %v", err)
	}
	if _, err := connectors.DBListTables(ctx, "", "db1"); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("table connector err = %v", err)
	}
	if _, err := connectors.DBListTables(ctx, "conn-1", ""); !IsCode(err, ErrDatabaseRequired.Code) {
		t.Fatalf("table database err = %v", err)
	}
	if _, err := connectors.DBSourceSchema(ctx, "", "db1", "orders"); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("source schema connector err = %v", err)
	}
	if _, err := connectors.DBSourceSchema(ctx, "conn-1", "", "orders"); !IsCode(err, ErrDatabaseRequired.Code) {
		t.Fatalf("source schema database err = %v", err)
	}
	if _, err := connectors.DBSourceSchema(ctx, "conn-1", "db1", ""); !IsCode(err, ErrTableRequired.Code) {
		t.Fatalf("source schema table err = %v", err)
	}
	if _, err := connectors.FileDownload(ctx, ""); !IsCode(err, ErrConnectorFileIDRequired.Code) {
		t.Fatalf("file download err = %v", err)
	}
	if _, err := connectors.FileStream(ctx, ""); !IsCode(err, ErrConnectorFileIDRequired.Code) {
		t.Fatalf("file stream err = %v", err)
	}
	if err := connectors.FileDelete(ctx, ""); !IsCode(err, ErrConnectorFileIDRequired.Code) {
		t.Fatalf("file delete err = %v", err)
	}
	if _, err := connectors.MODatabaseList(ctx, ""); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("mo database err = %v", err)
	}
	if _, err := connectors.MODatabaseTableList(ctx, "", "db1"); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("mo table connector err = %v", err)
	}
	if _, err := connectors.MODatabaseTableList(ctx, "conn-1", ""); !IsCode(err, ErrDatabaseRequired.Code) {
		t.Fatalf("mo table database err = %v", err)
	}
	if _, err := connectors.MOCheckTableName(ctx, "", "db1", "orders"); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("mo check connector err = %v", err)
	}
	if _, err := connectors.MOCheckTableName(ctx, "conn-1", "", "orders"); !IsCode(err, ErrDatabaseRequired.Code) {
		t.Fatalf("mo check database err = %v", err)
	}
	if _, err := connectors.MOCheckTableName(ctx, "conn-1", "db1", ""); !IsCode(err, ErrTableRequired.Code) {
		t.Fatalf("mo check table err = %v", err)
	}
	if _, err := connectors.MOColumnMapping(ctx, "", "db1", "orders", nil); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("mo mapping connector err = %v", err)
	}
	if _, err := connectors.MOColumnMapping(ctx, "conn-1", "", "orders", nil); !IsCode(err, ErrDatabaseRequired.Code) {
		t.Fatalf("mo mapping database err = %v", err)
	}
	if _, err := connectors.MOColumnMapping(ctx, "conn-1", "db1", "", nil); !IsCode(err, ErrTableRequired.Code) {
		t.Fatalf("mo mapping table err = %v", err)
	}
}

func TestConnectorCreateTaskSendsStructuredLoadConfig(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/newmoi/task" || r.Header.Get(headerWorkspaceID) != "ws-1" {
			t.Errorf("unexpected structured create request: %s %s workspace=%s", r.Method, r.URL.Path, r.Header.Get(headerWorkspaceID))
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		if _, err := io.WriteString(w, `{"code":"OK","msg":"OK","data":{"task_id":"structured-task"}}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	config := map[string]any{"version": float64(1), "source": map[string]any{"connector_id": "conn-1", "source_type": "SOURCE_TYPE_MYSQL"}}
	created, err := client.Connectors("ws-1").CreateTask(t.Context(), ImportTaskCreateSpec{
		SourceConnectorID: "conn-1", ConfigType: 3, Name: "structured import", StructuredLoadConfig: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.GetTaskId() != "structured-task" {
		t.Fatalf("created task = %v", created)
	}
	want := map[string]any{"source_connector_id": "conn-1", "config_type": float64(3), "name": "structured import", "structured_load_config": config}
	if !reflect.DeepEqual(received, want) {
		t.Fatalf("structured create body = %#v, want %#v", received, want)
	}
}

func TestConnectorImportTaskMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		data := map[string]any{"ok": true}
		if r.URL.EscapedPath() == "/newmoi/connectors/db/database/list" {
			data = map[string]any{"databases": []string{"db1"}}
		} else if r.URL.EscapedPath() == "/newmoi/connectors/db/database/schema/list" {
			data = map[string]any{"schemas": []string{"public"}}
		} else if r.URL.EscapedPath() == "/newmoi/connectors/db/database/table/list" {
			data = map[string]any{"tables": []string{"orders"}}
		} else if r.URL.EscapedPath() == "/newmoi/connectors/db/schema" {
			data = map[string]any{"columns": []map[string]any{{
				"name":           "id",
				"data_type":      "INT",
				"mapped_type":    "INT64",
				"is_primary_key": true,
				"is_nullable":    false,
				"is_auto_incr":   true,
			}}}
		} else if r.URL.EscapedPath() == "/newmoi/task" {
			data = map[string]any{"task_id": "task-1"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": data,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	connectors := client.Connectors("ws-1")
	if _, err := connectors.CreateTask(context.Background(), ImportTaskCreateSpec{
		SourceConnectorID: "conn-1",
		ConfigType:        1,
		SourceConfig:      map[string]any{"tables": []any{"orders"}},
		Name:              "load orders",
		VolumeID:          "volume-1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := connectors.UpdateTask(context.Background(), "task-1", ImportTaskUpdateSpec{Name: "load orders v2"}); err != nil {
		t.Fatal(err)
	}
	if err := connectors.DeleteTask(context.Background(), "task-1"); err != nil {
		t.Fatal(err)
	}
	if err := connectors.PauseTask(context.Background(), "task-1"); err != nil {
		t.Fatal(err)
	}
	if err := connectors.ResumeTask(context.Background(), "task-1"); err != nil {
		t.Fatal(err)
	}
	if err := connectors.RunTaskNow(context.Background(), "task-1"); err != nil {
		t.Fatal(err)
	}
	if err := connectors.RunTaskFullRefresh(context.Background(), "task-1"); err != nil {
		t.Fatal(err)
	}
	if err := connectors.RetryTaskFiles(context.Background(), "task-1", "file-1", "file-2"); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 8 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/task" ||
		requests[0].body["source_connector_id"] != "conn-1" ||
		requests[0].body["config_type"] != float64(1) ||
		requests[0].body["name"] != "load orders" ||
		requests[0].body["volume_id"] != "volume-1" {
		t.Fatalf("create task = %#v", requests[0])
	}
	if _, ok := requests[0].body["source_config"].(map[string]any); !ok {
		t.Fatalf("source config = %#v", requests[0].body["source_config"])
	}
	if requests[1].path != "/newmoi/task/update" ||
		requests[1].body["task_id"] != "task-1" ||
		requests[1].body["name"] != "load orders v2" {
		t.Fatalf("update task = %#v", requests[1])
	}
	if requests[2].path != "/newmoi/task/delete" || requests[2].body["task_id"] != "task-1" {
		t.Fatalf("delete task = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/task/pause" || requests[3].body["task_id"] != "task-1" {
		t.Fatalf("pause task = %#v", requests[3])
	}
	if requests[4].path != "/newmoi/task/resume" || requests[4].body["task_id"] != "task-1" {
		t.Fatalf("resume task = %#v", requests[4])
	}
	if requests[5].path != "/newmoi/task/run-now" || requests[5].body["task_id"] != "task-1" {
		t.Fatalf("run task now = %#v", requests[5])
	}
	if requests[6].path != "/newmoi/task/run-full-refresh" || requests[6].body["task_id"] != "task-1" {
		t.Fatalf("run task full refresh = %#v", requests[6])
	}
	if requests[7].path != "/newmoi/task/retry" || requests[7].body["task_id"] != "task-1" {
		t.Fatalf("retry task = %#v", requests[7])
	}
	ids, ok := requests[7].body["ids"].([]any)
	if !ok || len(ids) != 2 || ids[0] != "file-1" || ids[1] != "file-2" {
		t.Fatalf("retry ids = %#v", requests[7].body["ids"])
	}
}

func TestConnectorImportTaskStructuredLoadConfigDecodesStrictly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		task := map[string]any{
			"id":       "task-1",
			"mapping":  []any{map[string]any{"target_column": "order_id"}},
			"schedule": map[string]any{"kind": "hourly"},
			"backfill": map[string]any{"enabled": true},
			// Backend dataconn.Task.SourceFiles is [][]string, not [{values:[...]}].
			"source_files": []any{[]any{"conn-name", "s3://bucket/a.csv"}},
			"structured_load_config": map[string]any{
				"source": map[string]any{"database": "analytics"},
				"tables": []any{"orders"},
			},
		}
		var data map[string]any
		switch r.URL.EscapedPath() {
		case "/newmoi/task/get":
			data = map[string]any{"task": task}
		case "/newmoi/task/list":
			data = map[string]any{"tasks": []any{task}, "total": 1}
		default:
			t.Fatalf("unexpected path %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	connectors := client.Connectors("ws-1")
	info, err := connectors.GetTask(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	list, err := connectors.Tasks(context.Background())
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	for _, task := range []*ImportTask{info.GetTask(), list.GetTasks()[0]} {
		config := task.GetStructuredLoadConfig().AsMap()
		if config["source"].(map[string]any)["database"] != "analytics" {
			t.Fatalf("structured_load_config = %#v", config)
		}
		if task.GetMapping().GetValues()[0].GetStructValue().GetFields()["target_column"].GetStringValue() != "order_id" {
			t.Fatalf("mapping = %#v", task.GetMapping())
		}
		if task.GetSchedule().GetFields()["kind"].GetStringValue() != "hourly" {
			t.Fatalf("schedule = %#v", task.GetSchedule())
		}
		if !task.GetBackfill().GetFields()["enabled"].GetBoolValue() {
			t.Fatalf("backfill = %#v", task.GetBackfill())
		}
		sf := task.GetSourceFiles()
		if len(sf) != 1 || len(sf[0].GetValues()) != 2 ||
			sf[0].GetValues()[0].GetStringValue() != "conn-name" ||
			sf[0].GetValues()[1].GetStringValue() != "s3://bucket/a.csv" {
			t.Fatalf("source_files = %#v", sf)
		}
	}
}

func TestConnectorImportTaskMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	connectors := client.Connectors("ws-1")
	ctx := context.Background()
	if _, err := connectors.CreateTask(ctx, ImportTaskCreateSpec{ConfigType: 1, SourceConfig: map[string]any{}}); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("create connector err = %v", err)
	}
	if _, err := connectors.CreateTask(ctx, ImportTaskCreateSpec{SourceConnectorID: "conn-1", ConfigType: 1}); !IsCode(err, ErrSourceConfigRequired.Code) {
		t.Fatalf("create source config err = %v", err)
	}
	if err := connectors.UpdateTask(ctx, "", ImportTaskUpdateSpec{}); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("update err = %v", err)
	}
	if err := connectors.DeleteTask(ctx, ""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("delete err = %v", err)
	}
	if err := connectors.PauseTask(ctx, ""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("pause err = %v", err)
	}
	if err := connectors.ResumeTask(ctx, ""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("resume err = %v", err)
	}
	if err := connectors.RunTaskNow(ctx, ""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("run now err = %v", err)
	}
	if err := connectors.RunTaskFullRefresh(ctx, ""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("run full refresh err = %v", err)
	}
	if _, err := connectors.GetTask(ctx, ""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("get err = %v", err)
	}
	if _, err := connectors.TaskRuns(ctx, ""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("runs err = %v", err)
	}
	if err := connectors.RetryTaskFiles(ctx, ""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("retry err = %v", err)
	}
}

func TestConnectorExportTaskMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		method string
		path   string
		body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method string
			path   string
			body   map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), body: body})
		data := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/connectors/get":
			data = map[string]any{"id": "conn-1"}
		case "/newmoi/connectors":
			data = map[string]any{"id": "conn-1"}
		case "/newmoi/connectors/db/database/list":
			data = map[string]any{"databases": []string{"db1"}}
		case "/newmoi/connectors/db/database/schema/list":
			data = map[string]any{"schemas": []string{"public"}}
		case "/newmoi/connectors/db/database/table/list":
			data = map[string]any{"tables": []string{"orders"}}
		case "/newmoi/connectors/db/schema":
			data = map[string]any{"columns": []map[string]any{{
				"name":           "id",
				"data_type":      "INT",
				"mapped_type":    "INT64",
				"is_primary_key": true,
				"is_nullable":    false,
				"is_auto_incr":   true,
			}}}
		case "/newmoi/connectors/mo/database/list":
			data = map[string]any{"databases": []string{"analytics"}}
		case "/newmoi/connectors/mo/database/table/list":
			data = map[string]any{"tables": []string{"orders"}}
		case "/newmoi/connectors/mo/check_table_name":
			data = map[string]any{"exists": false}
		case "/newmoi/connectors/mo/column/mapping":
			data = map[string]any{"column_map": map[string]any{"src_id": []any{"id"}}, "need_mapping_columns": []string{"src_id"}}
		case "/newmoi/task":
			data = map[string]any{"task_id": "task-1"}
		case "/newmoi/task/get":
			data = map[string]any{"task": map[string]any{"id": "task-1"}}
		case "/newmoi/task/runs":
			data = map[string]any{"runs": []map[string]any{}}
		case "/newmoi/task/files":
			data = map[string]any{"files": []map[string]any{}}
		case "/newmoi/export/task/create":
			data = map[string]any{"id": "export-1"}
		case "/newmoi/export/task/list":
			data = map[string]any{"tasks": []map[string]any{}, "total": 0}
		case "/newmoi/export/task/info":
			data = map[string]any{"id": "export-1"}
		case "/newmoi/export/task/files":
			data = map[string]any{"files": []map[string]any{}}
		case "/newmoi/export/task/export-1/state":
			data = map[string]any{"total": 0, "pending": 0, "running": 0, "failed": 0, "completed": 0}
		case "/newmoi/export/task/task-1/state":
			data = map[string]any{"total": 0, "pending": 0, "running": 0, "failed": 0, "completed": 0}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": data,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	connectors := client.Connectors("ws-1")
	if _, err := connectors.ExportTasks(
		context.Background(),
		WithExportTaskLimit(20),
		WithExportTaskOffset(40),
		WithExportTaskConnectorName("oss"),
		WithExportTaskID("task-1"),
		WithExportTaskOrderBy("created_at"),
		WithExportTaskOrderDirection("desc"),
		WithExportTaskStatuses(1, 2),
		WithExportTaskCreator("alice"),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := connectors.ExportTaskInfo(context.Background(), "task-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := connectors.ExportTaskFiles(
		context.Background(),
		"task-1",
		WithExportTaskFilesLimit(10),
		WithExportTaskFilesOffset(5),
		WithExportTaskFileStatuses(3),
		WithExportTaskFilesOrderBy("created_at"),
		WithExportTaskFilesOrderDirection("asc"),
	); err != nil {
		t.Fatal(err)
	}
	if err := connectors.DeleteExportTask(context.Background(), "task-1"); err != nil {
		t.Fatal(err)
	}
	isRaw := true
	if _, err := connectors.CreateExportTask(context.Background(), ExportTaskSpec{
		ConnectorID:   "conn-1",
		Name:          "export task",
		ConnectorName: "oss",
		Type:          2,
		Config:        ExportTaskConfig{S3Config: map[string]any{"path": "/out"}},
		Files:         []ExportTaskFile{{FileID: "file-1", FullPath: []string{"db", "table"}, IsRaw: &isRaw}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := connectors.RetryExportTask(context.Background(), "task-1", "file-1", "file-2"); err != nil {
		t.Fatal(err)
	}
	if _, err := connectors.ExportTaskState(context.Background(), "task-1"); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 7 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/export/task/list" ||
		requests[0].body["limit"] != float64(20) ||
		requests[0].body["offset"] != float64(40) ||
		requests[0].body["connector_name"] != "oss" ||
		requests[0].body["task_id"] != "task-1" ||
		requests[0].body["order_by"] != "created_at" ||
		requests[0].body["order_direction"] != "desc" ||
		requests[0].body["creator"] != "alice" {
		t.Fatalf("export tasks = %#v", requests[0])
	}
	statuses, ok := requests[0].body["statuses"].([]any)
	if !ok || len(statuses) != 2 || statuses[0] != float64(1) || statuses[1] != float64(2) {
		t.Fatalf("statuses = %#v", requests[0].body["statuses"])
	}
	if requests[1].path != "/newmoi/export/task/info" || requests[1].body["id"] != "task-1" {
		t.Fatalf("export task info = %#v", requests[1])
	}
	if requests[2].path != "/newmoi/export/task/files" ||
		requests[2].body["task_id"] != "task-1" ||
		requests[2].body["limit"] != float64(10) ||
		requests[2].body["offset"] != float64(5) ||
		requests[2].body["order_by"] != "created_at" ||
		requests[2].body["order_direction"] != "asc" {
		t.Fatalf("export task files = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/export/task/delete" || requests[3].body["id"] != "task-1" {
		t.Fatalf("delete export task = %#v", requests[3])
	}
	if requests[4].path != "/newmoi/export/task/create" ||
		requests[4].body["task_name"] != "export task" ||
		requests[4].body["connector_id"] != "conn-1" ||
		requests[4].body["connector_name"] != "oss" ||
		requests[4].body["type"] != float64(2) {
		t.Fatalf("create export task = %#v", requests[4])
	}
	if _, ok := requests[4].body["config"].(map[string]any); !ok {
		t.Fatalf("export config = %#v", requests[4].body["config"])
	}
	gotFiles, ok := requests[4].body["files"].([]any)
	if !ok || len(gotFiles) != 1 {
		t.Fatalf("export files = %#v", requests[4].body["files"])
	}
	if requests[5].path != "/newmoi/export/task/task-1/rerun" {
		t.Fatalf("retry export task = %#v", requests[5])
	}
	retryFiles, ok := requests[5].body["files"].([]any)
	if !ok || len(retryFiles) != 2 || retryFiles[0] != "file-1" || retryFiles[1] != "file-2" {
		t.Fatalf("retry files = %#v", requests[5].body["files"])
	}
	if requests[6].method != http.MethodGet || requests[6].path != "/newmoi/export/task/task-1/state" {
		t.Fatalf("export state = %#v", requests[6])
	}
}

func TestConnectorExportTaskMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	connectors := client.Connectors("ws-1")
	ctx := context.Background()
	if _, err := connectors.ExportTaskInfo(ctx, ""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("info err = %v", err)
	}
	if _, err := connectors.ExportTaskFiles(ctx, ""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("files err = %v", err)
	}
	if err := connectors.DeleteExportTask(ctx, ""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("delete err = %v", err)
	}
	if err := (ExportTaskSpec{
		ConnectorID: "conn-1", ConnectorName: "oss", Type: 1,
		Config: ExportTaskConfig{S3Config: map[string]any{"path": "/out"}},
		Files:  []ExportTaskFile{{FileID: "file-1", FullPath: []string{"db"}}},
	}).Validate(); !IsCode(err, ErrTaskNameRequired.Code) {
		t.Fatalf("create empty name err = %v", err)
	}
	if _, err := connectors.CreateExportTask(ctx, ExportTaskSpec{Name: "task", ConnectorName: "oss", Type: 1}); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("create connector err = %v", err)
	}
	if err := connectors.RetryExportTask(ctx, ""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("retry err = %v", err)
	}
	if _, err := connectors.ExportTaskState(ctx, ""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("state err = %v", err)
	}
}

func TestSQLWorkbookMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		data := any(nil)
		switch r.URL.EscapedPath() {
		case "/newmoi/workbook/create":
			data = map[string]any{"workbook_id": "wb-1", "name": "analysis"}
		case "/newmoi/workbook/list":
			data = map[string]any{"total": 1, "result": []any{map[string]any{"workbook_id": "wb-1", "name": "analysis"}}, "next_page_token": "workbooks-next"}
		case "/newmoi/workbook/detail":
			data = map[string]any{"sql_content": "select 1", "created_at": "2026-06-01T00:00:00Z", "updated_at": "2026-06-01T00:00:00Z"}
		case "/newmoi/workbook/version/list":
			data = map[string]any{"total": 1, "result": []any{map[string]any{"id": "ver-1", "name": "v1", "status": "draft"}}, "next_page_token": "versions-next"}
		case "/newmoi/workbook/version/create":
			data = map[string]any{"id": "ver-1", "version": "v1", "status": "draft"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	sql := client.SQL("ws-1")
	if _, err := sql.CreateWorkbook(context.Background(), "analysis"); err != nil {
		t.Fatal(err)
	}
	workbooks, err := sql.Workbooks(context.Background(), WithWorkbookSearch("ana"), WithWorkbookPageSize(20), WithWorkbookPageToken("cursor=="))
	if err != nil {
		t.Fatal(err)
	}
	if workbooks.GetNextPageToken() != "workbooks-next" {
		t.Fatalf("workbook next page token = %q", workbooks.GetNextPageToken())
	}
	if err := sql.UpdateWorkbook(context.Background(), "wb-1", "analysis v2"); err != nil {
		t.Fatal(err)
	}
	if err := sql.DeleteWorkbook(context.Background(), "wb-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sql.WorkbookDetail(context.Background(), "wb-1", "ver-1"); err != nil {
		t.Fatal(err)
	}
	versions, err := sql.WorkbookVersions(context.Background(), "wb-1", WithWorkbookVersionPageSize(10), WithWorkbookVersionPageToken("versions-cursor=="))
	if err != nil {
		t.Fatal(err)
	}
	if versions.GetNextPageToken() != "versions-next" {
		t.Fatalf("workbook version next page token = %q", versions.GetNextPageToken())
	}
	if _, err := sql.CreateWorkbookVersion(context.Background(), "wb-1", "select 1"); err != nil {
		t.Fatal(err)
	}
	if err := sql.UpdateWorkbookVersion(context.Background(), "wb-1", "ver-1", "select 2"); err != nil {
		t.Fatal(err)
	}
	sqlContent := "select 3"
	if err := sql.SaveWorkbookVersion(context.Background(), "wb-1", "ver-1", &sqlContent); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 9 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/workbook/create" || requests[0].body["name"] != "analysis" {
		t.Fatalf("create workbook = %#v", requests[0])
	}
	if requests[1].path != "/newmoi/workbook/list" ||
		requests[1].body["search"] != "ana" ||
		requests[1].body["page_size"] != float64(20) ||
		requests[1].body["page_token"] != "cursor==" {
		t.Fatalf("workbooks = %#v", requests[1])
	}
	if requests[2].path != "/newmoi/workbook/update" ||
		requests[2].body["workbook_id"] != "wb-1" ||
		requests[2].body["name"] != "analysis v2" {
		t.Fatalf("update workbook = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/workbook/delete" || requests[3].body["workbook_id"] != "wb-1" {
		t.Fatalf("delete workbook = %#v", requests[3])
	}
	if requests[4].path != "/newmoi/workbook/detail" ||
		requests[4].body["workbook_id"] != "wb-1" ||
		requests[4].body["version_id"] != "ver-1" {
		t.Fatalf("workbook detail = %#v", requests[4])
	}
	if requests[5].path != "/newmoi/workbook/version/list" ||
		requests[5].body["workbook_id"] != "wb-1" ||
		requests[5].body["page_size"] != float64(10) ||
		requests[5].body["page_token"] != "versions-cursor==" {
		t.Fatalf("workbook versions = %#v", requests[5])
	}
	if requests[6].path != "/newmoi/workbook/version/create" ||
		requests[6].body["workbook_id"] != "wb-1" ||
		requests[6].body["sql_content"] != "select 1" {
		t.Fatalf("create workbook version = %#v", requests[6])
	}
	if requests[7].path != "/newmoi/workbook/version/update" ||
		requests[7].body["workbook_id"] != "wb-1" ||
		requests[7].body["version_id"] != "ver-1" ||
		requests[7].body["sql_content"] != "select 2" {
		t.Fatalf("update workbook version = %#v", requests[7])
	}
	if requests[8].path != "/newmoi/workbook/version/save" ||
		requests[8].body["workbook_id"] != "wb-1" ||
		requests[8].body["version_id"] != "ver-1" ||
		requests[8].body["sql_content"] != "select 3" {
		t.Fatalf("save workbook version = %#v", requests[8])
	}
}

func TestSQLWorkbookMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	sql := client.SQL("ws-1")
	ctx := context.Background()
	if err := sql.UpdateWorkbook(ctx, "", "name"); !IsCode(err, ErrWorkbookIDRequired.Code) {
		t.Fatalf("update workbook id err = %v", err)
	}
	if err := sql.UpdateWorkbook(ctx, "wb-1", ""); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("update workbook name err = %v", err)
	}
	if err := sql.DeleteWorkbook(ctx, ""); !IsCode(err, ErrWorkbookIDRequired.Code) {
		t.Fatalf("delete workbook err = %v", err)
	}
	if _, err := sql.WorkbookDetail(ctx, "", "ver-1"); !IsCode(err, ErrWorkbookIDRequired.Code) {
		t.Fatalf("detail workbook err = %v", err)
	}
	if _, err := sql.WorkbookDetail(ctx, "wb-1", ""); !IsCode(err, ErrVersionIDRequired.Code) {
		t.Fatalf("detail version err = %v", err)
	}
	if _, err := sql.WorkbookVersions(ctx, ""); !IsCode(err, ErrWorkbookIDRequired.Code) {
		t.Fatalf("versions err = %v", err)
	}
	if _, err := sql.CreateWorkbookVersion(ctx, "", "select 1"); !IsCode(err, ErrWorkbookIDRequired.Code) {
		t.Fatalf("create version err = %v", err)
	}
	if err := sql.UpdateWorkbookVersion(ctx, "", "ver-1", "select 1"); !IsCode(err, ErrWorkbookIDRequired.Code) {
		t.Fatalf("update version workbook err = %v", err)
	}
	if err := sql.UpdateWorkbookVersion(ctx, "wb-1", "", "select 1"); !IsCode(err, ErrVersionIDRequired.Code) {
		t.Fatalf("update version err = %v", err)
	}
	if err := sql.SaveWorkbookVersion(ctx, "", "ver-1", nil); !IsCode(err, ErrWorkbookIDRequired.Code) {
		t.Fatalf("save workbook err = %v", err)
	}
	if err := sql.SaveWorkbookVersion(ctx, "wb-1", "", nil); !IsCode(err, ErrVersionIDRequired.Code) {
		t.Fatalf("save version err = %v", err)
	}
}

func TestSQLMetadataQueryAndHistoryMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.EscapedPath(), body: body})
		if r.URL.EscapedPath() == "/newmoi/query/result/download" {
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", `attachment; filename="result.csv"`)
			_, _ = w.Write([]byte("id\n1\n"))
			return
		}
		data := any(nil)
		switch r.URL.EscapedPath() {
		case "/newmoi/meta/db/table":
			data = map[string]any{"db_name": "db1", "tables": map[string]any{"tables": []any{map[string]any{"name": "tbl1"}}, "total": 1}, "views": map[string]any{"tables": []any{}, "total": 0}}
		case "/newmoi/meta/table/info":
			data = map[string]any{"columns": []any{map[string]any{"name": "id", "data_type": "INT"}}, "total": 1, "rows": 2}
		case "/newmoi/meta/table/column", "/newmoi/meta/table/column_stat":
			data = map[string]any{"columns": []any{map[string]any{"name": "id", "data_type": "INT"}}, "total": 1}
		case "/newmoi/meta/table/ddl":
			data = map[string]any{"create_sql": "CREATE TABLE tbl1 (id INT)"}
		case "/newmoi/query/execute":
			data = map[string]any{"id": "query-1", "query_id": "query-1"}
		case "/newmoi/query/describe":
			data = map[string]any{"query_id": "query-1", "created_at": "2026-06-01T00:00:00Z", "statement_id": "stmt-1", "db_name": "db1", "status": "running"}
		case "/newmoi/query/result":
			data = map[string]any{"statement_id": "stmt-1", "status": "success", "offset": 1, "limit": 2, "total": 1, "columns": []any{map[string]any{"name": "id", "type": "INT"}}, "result": []any{[]any{map[string]any{"String": "1", "Valid": true}}}}
		case "/newmoi/query/history/overview":
			data = map[string]any{"total": 1, "success": 1, "running": 0, "failed": 0}
		case "/newmoi/query/history":
			data = map[string]any{"total": 1, "offset": 4, "limit": 5, "query_list": []any{map[string]any{"statement_id": "stmt-1", "status": "Success"}}}
		case "/newmoi/query/detail":
			data = map[string]any{"statement_id": "stmt-1", "status": "Success"}
		case "/newmoi/query/profile":
			data = map[string]any{"statement_id": "stmt-1", "profile": map[string]any{"code": 200, "message": "OK", "success": true}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	sql := client.SQL("ws-1")
	ctx := context.Background()
	start := time.Date(2026, 6, 1, 2, 3, 4, 0, time.UTC)
	end := time.Date(2026, 6, 2, 2, 3, 4, 0, time.UTC)
	if _, err := sql.MetaDBTables(ctx, "db1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sql.MetaTableInfo(ctx, "db1", "tbl1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sql.MetaTableColumns(ctx, "db1", "tbl1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sql.MetaTableColumnStats(ctx, "db1", "tbl1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sql.MetaTableDDL(ctx, "db1", "tbl1"); err != nil {
		t.Fatal(err)
	}
	if err := sql.DropMetaDatabase(ctx, "db1"); err != nil {
		t.Fatal(err)
	}
	if err := sql.DropMetaTable(ctx, "db1", "tbl1"); err != nil {
		t.Fatal(err)
	}
	if err := sql.DropMetaView(ctx, "db1", "view1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sql.ExecuteQuery(ctx, "select 1", WithSQLQueryDBName("db1"), WithSQLQueryType("DQL"), WithSQLQueryOffset(7), WithSQLQueryLimit(8)); err != nil {
		t.Fatal(err)
	}
	if _, err := sql.DescribeQuery(ctx, "query-1"); err != nil {
		t.Fatal(err)
	}
	if err := sql.KillQuery(ctx, "query-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sql.QueryResult(ctx, "stmt-1", WithSQLResultOffset(1), WithSQLResultLimit(2)); err != nil {
		t.Fatal(err)
	}
	if _, err := sql.QueryHistoryOverview(ctx,
		WithQueryHistoryOverviewScope(QueryHistoryScopeWorkspace),
		WithQueryHistoryOverviewNonUser(true),
		WithQueryHistoryOverviewStart(start),
		WithQueryHistoryOverviewEnd(end),
		WithQueryHistoryOverviewSQLSourceTypes("cloud_user_sql"),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := sql.QueryHistory(ctx,
		WithQueryHistoryScope(QueryHistoryScopeWorkspace),
		WithQueryHistoryNonUser(true),
		WithQueryHistoryStatus("Success"),
		WithQueryHistoryUsers("u1", "u2"),
		WithQueryHistoryDatabases("db1"),
		WithQueryHistoryStatement("select"),
		WithQueryHistoryTypes("DQL"),
		WithQueryHistoryMinDuration(123),
		WithQueryHistoryStart(start),
		WithQueryHistoryEnd(end),
		WithQueryHistoryStatementID("stmt-1"),
		WithQueryHistoryTransactionID("txn-1"),
		WithQueryHistorySessionID("session-1"),
		WithQueryHistorySelectedFields("statement_id", "duration"),
		WithQueryHistorySQLSourceTypes("cloud_user_sql"),
		WithQueryHistoryOffset(4),
		WithQueryHistoryLimit(5),
		WithQueryHistoryOrderBy("duration"),
		WithQueryHistoryOrder("DESC"),
		WithQueryHistoryCU(9),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := sql.QueryDetail(ctx, "stmt-1", WithQueryRecordScope(QueryHistoryScopeWorkspace), WithQueryRecordStart("2026-06-01 00:00:00"), WithQueryRecordEnd("2026-06-02 00:00:00")); err != nil {
		t.Fatal(err)
	}
	if _, err := sql.QueryProfile(ctx, "stmt-1", WithQueryRecordScope(QueryHistoryScopeWorkspace), WithQueryRecordStart("2026-06-01 00:00:00"), WithQueryRecordEnd("2026-06-02 00:00:00")); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 16 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].path != "/newmoi/meta/db/table" || requests[0].body["name"] != "db1" {
		t.Fatalf("meta db tables = %#v", requests[0])
	}
	for i, path := range []string{
		"/newmoi/meta/table/info",
		"/newmoi/meta/table/column",
		"/newmoi/meta/table/column_stat",
		"/newmoi/meta/table/ddl",
	} {
		req := requests[i+1]
		if req.path != path || req.body["db_name"] != "db1" || req.body["table_name"] != "tbl1" {
			t.Fatalf("meta table request %d = %#v", i, req)
		}
	}
	if requests[5].path != "/newmoi/meta/db/drop" || requests[5].body["db_name"] != "db1" {
		t.Fatalf("drop db = %#v", requests[5])
	}
	if requests[6].path != "/newmoi/meta/table/drop" || requests[6].body["db_name"] != "db1" || requests[6].body["table_name"] != "tbl1" {
		t.Fatalf("drop table = %#v", requests[6])
	}
	if requests[7].path != "/newmoi/meta/view/drop" || requests[7].body["db_name"] != "db1" || requests[7].body["table_name"] != "view1" {
		t.Fatalf("drop view = %#v", requests[7])
	}
	if requests[8].path != "/newmoi/query/execute" ||
		requests[8].body["query"] != "select 1" ||
		requests[8].body["db_name"] != "db1" ||
		requests[8].body["sql_type"] != "DQL" ||
		requests[8].body["offset"] != float64(7) ||
		requests[8].body["limit"] != float64(8) {
		t.Fatalf("execute query = %#v", requests[8])
	}
	if requests[9].path != "/newmoi/query/describe" || requests[9].body["query_id"] != "query-1" {
		t.Fatalf("describe query = %#v", requests[9])
	}
	if requests[10].path != "/newmoi/query/kill" || requests[10].body["query_id"] != "query-1" {
		t.Fatalf("kill query = %#v", requests[10])
	}
	if requests[11].path != "/newmoi/query/result" ||
		requests[11].body["statement_id"] != "stmt-1" ||
		requests[11].body["offset"] != float64(1) ||
		requests[11].body["limit"] != float64(2) {
		t.Fatalf("query result = %#v", requests[11])
	}
	if requests[12].path != "/newmoi/query/history/overview" ||
		requests[12].body["scope"] != "workspace" ||
		requests[12].body["non_user"] != true ||
		requests[12].body["start"] != "2026-06-01T02:03:04Z" ||
		requests[12].body["end"] != "2026-06-02T02:03:04Z" {
		t.Fatalf("history overview = %#v", requests[12])
	}
	if got := requests[12].body["sql_source_type"].([]any); len(got) != 1 || got[0] != "cloud_user_sql" {
		t.Fatalf("overview sql_source_type = %#v", requests[12].body["sql_source_type"])
	}
	history := requests[13].body
	if requests[13].path != "/newmoi/query/history" ||
		history["scope"] != "workspace" ||
		history["non_user"] != true ||
		history["status"] != "Success" ||
		history["statement"] != "select" ||
		history["duration"] != float64(123) ||
		history["start"] != "2026-06-01T02:03:04Z" ||
		history["end"] != "2026-06-02T02:03:04Z" ||
		history["statement_id"] != "stmt-1" ||
		history["transaction_id"] != "txn-1" ||
		history["session_id"] != "session-1" ||
		history["offset"] != float64(4) ||
		history["limit"] != float64(5) ||
		history["order"] != "DESC" ||
		history["cu"] != float64(9) {
		t.Fatalf("history = %#v", requests[13])
	}
	for key, want := range map[string]string{
		"user":            "u1",
		"databases":       "db1",
		"query_type":      "DQL",
		"selected_field":  "statement_id",
		"sql_source_type": "cloud_user_sql",
		"order_by":        "duration",
	} {
		got := history[key].([]any)
		if len(got) == 0 || got[0] != want {
			t.Fatalf("history %s = %#v", key, history[key])
		}
	}
	if requests[14].path != "/newmoi/query/detail" ||
		requests[14].body["scope"] != "workspace" ||
		requests[14].body["statement_id"] != "stmt-1" ||
		requests[14].body["start"] != "2026-06-01 00:00:00" ||
		requests[14].body["end"] != "2026-06-02 00:00:00" {
		t.Fatalf("query detail = %#v", requests[14])
	}
	if requests[15].path != "/newmoi/query/profile" ||
		requests[15].body["scope"] != "workspace" ||
		requests[15].body["statement_id"] != "stmt-1" ||
		requests[15].body["start"] != "2026-06-01 00:00:00" ||
		requests[15].body["end"] != "2026-06-02 00:00:00" {
		t.Fatalf("query profile = %#v", requests[15])
	}
}

func TestQueryHistoryBodiesDefaultToFiveMinuteRange(t *testing.T) {
	for name, body := range map[string]map[string]any{
		"list":     queryHistoryBody(),
		"overview": queryHistoryOverviewBody(),
	} {
		t.Run(name, func(t *testing.T) {
			start, ok := body["start"].(time.Time)
			if !ok {
				t.Fatalf("start = %#v, want time.Time", body["start"])
			}
			end, ok := body["end"].(time.Time)
			if !ok {
				t.Fatalf("end = %#v, want time.Time", body["end"])
			}
			if end.Sub(start) != DefaultQueryHistoryLookback {
				t.Fatalf("range = %s, want %s", end.Sub(start), DefaultQueryHistoryLookback)
			}
		})
	}

	explicitStart := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	body := queryHistoryBody(WithQueryHistoryStart(explicitStart))
	if got := body["start"]; got != explicitStart {
		t.Fatalf("start = %#v, want %#v", got, explicitStart)
	}
	if _, ok := body["end"]; ok {
		t.Fatalf("end = %#v, want omitted when only start is configured", body["end"])
	}
}

func TestSQLMetadataQueryAndHistoryMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	sql := client.SQL("ws-1")
	ctx := context.Background()
	if _, err := sql.MetaDBTables(ctx, ""); !IsCode(err, ErrDatabaseRequired.Code) {
		t.Fatalf("meta db tables err = %v", err)
	}
	if _, err := sql.MetaTableInfo(ctx, "", "tbl1"); !IsCode(err, ErrDatabaseRequired.Code) {
		t.Fatalf("meta table info db err = %v", err)
	}
	if _, err := sql.MetaTableInfo(ctx, "db1", ""); !IsCode(err, ErrTableRequired.Code) {
		t.Fatalf("meta table info table err = %v", err)
	}
	if _, err := sql.MetaTableColumns(ctx, "", "tbl1"); !IsCode(err, ErrDatabaseRequired.Code) {
		t.Fatalf("meta table columns db err = %v", err)
	}
	if _, err := sql.MetaTableColumnStats(ctx, "db1", ""); !IsCode(err, ErrTableRequired.Code) {
		t.Fatalf("meta table column stats table err = %v", err)
	}
	if _, err := sql.MetaTableDDL(ctx, "", "tbl1"); !IsCode(err, ErrDatabaseRequired.Code) {
		t.Fatalf("meta table ddl db err = %v", err)
	}
	if err := sql.DropMetaDatabase(ctx, ""); !IsCode(err, ErrDatabaseRequired.Code) {
		t.Fatalf("drop db err = %v", err)
	}
	if err := sql.DropMetaTable(ctx, "", "tbl1"); !IsCode(err, ErrDatabaseRequired.Code) {
		t.Fatalf("drop table db err = %v", err)
	}
	if err := sql.DropMetaView(ctx, "db1", ""); !IsCode(err, ErrTableRequired.Code) {
		t.Fatalf("drop view table err = %v", err)
	}
	if _, err := sql.ExecuteQuery(ctx, ""); !IsCode(err, ErrQueryRequired.Code) {
		t.Fatalf("execute query err = %v", err)
	}
	if _, err := sql.DescribeQuery(ctx, ""); !IsCode(err, ErrQueryIDRequired.Code) {
		t.Fatalf("describe query err = %v", err)
	}
	if err := sql.KillQuery(ctx, ""); !IsCode(err, ErrQueryIDRequired.Code) {
		t.Fatalf("kill query err = %v", err)
	}
	if _, err := sql.QueryResult(ctx, ""); !IsCode(err, ErrStatementIDRequired.Code) {
		t.Fatalf("query result err = %v", err)
	}
	if _, err := sql.QueryDetail(ctx, ""); !IsCode(err, ErrStatementIDRequired.Code) {
		t.Fatalf("query detail err = %v", err)
	}
	if _, err := sql.QueryProfile(ctx, ""); !IsCode(err, ErrStatementIDRequired.Code) {
		t.Fatalf("query profile err = %v", err)
	}
}

func TestWorkflowMethodsUseExplicitArguments(t *testing.T) {
	var requests []struct {
		method string
		path   string
		body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method string
			path   string
			body   map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), body: body})
		data := any(nil)
		switch r.URL.EscapedPath() {
		case "/newmoi/meta/db/info":
			data = map[string]any{"total": 1, "result": []any{map[string]any{"db_name": "db1", "db_owner": "u"}}}
		case "/newmoi/meta/db/tree":
			data = map[string]any{"db_list": []any{map[string]any{"db_name": "db1", "db_owner": "u"}}}
		case "/newmoi/meta/db/table":
			data = map[string]any{"db_name": "db1", "tables": map[string]any{"tables": []any{map[string]any{"name": "orders"}}, "total": 1}, "views": map[string]any{"tables": []any{map[string]any{"name": "orders_view"}}, "total": 1}}
		case "/newmoi/meta/table/info":
			data = map[string]any{"columns": []any{map[string]any{"name": "id", "data_type": "INT"}}, "total": 1, "rows": 10}
		case "/newmoi/meta/table/column", "/newmoi/meta/table/column_stat":
			data = map[string]any{"columns": []any{map[string]any{"name": "id", "data_type": "INT"}}, "total": 1}
		case "/newmoi/meta/table/ddl":
			data = map[string]any{"create_sql": "CREATE TABLE orders (id INT)"}
		case "/newmoi/workflow/v2/workflow-deployments":
			data = map[string]any{"workflow": map[string]any{"id": "wf-1", "name": "wf", "source_type": "manual_dsl", "execution_mode": "cron", "status": "ready"}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1":
			if r.Method == http.MethodDelete {
				data = map[string]any{"deleted": true}
			} else {
				data = map[string]any{"workflow": map[string]any{"id": "wf-1", "name": "wf", "source_type": "manual_dsl", "status": "ready", "execution_mode": "cron", "dsl_yaml": "workflow:\n  name: wf"}}
			}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions":
			data = map[string]any{"workflow_run": map[string]any{"execution_id": "exec-1", "workflow_id": "wf-1", "status": "running", "execution_mode": "one_shot"}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1":
			if r.Method == http.MethodDelete {
				data = map[string]any{"deleted": true}
			} else {
				data = map[string]any{"execution": map[string]any{"execution_id": "exec-1", "workflow_id": "wf-1", "status": "running", "execution_mode": "one_shot"}}
			}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/retry":
			data = map[string]any{"workflow_run": map[string]any{"execution_id": "exec-2", "workflow_id": "wf-1", "status": "running", "execution_mode": "one_shot"}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/cancel":
			data = map[string]any{"execution": map[string]any{"execution_id": "exec-1", "workflow_id": "wf-1", "status": "cancelled", "execution_mode": "one_shot", "dispatch_job_id": "dispatch-1"}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/nodes/node-1/reruns":
			data = map[string]any{"rerun": map[string]any{"rerun_id": "rerun-1", "source_execution_id": "exec-1", "new_workflow_execution_id": "exec-3", "node_key": "node-1", "source_node_execution_id": "node-exec-1", "mode": "continue_from_node_with_overrides", "status": "created"}}
		case "/newmoi/workflow/v2/custom-operators":
			data = map[string]any{"id": 7, "node_id": "node-7", "name": "Text Counter", "identifier": "text_counter", "description": "Count text length", "language": 1, "version": "v1", "enabled": true}
		case "/newmoi/workflow/v2/custom-operators/7/test-run":
			data = map[string]any{"test_run": map[string]any{"operator_id": 7, "node_id": "node-7", "version": "v1", "task_id": "task-1", "status": "completed", "wait_timeout_seconds": 30}}
		case "/newmoi/workflow/v2/custom-operators/7":
			data = map[string]any{"id": 7, "node_id": "node-7", "name": "Counter", "identifier": "text_counter", "description": "new desc", "language": 1, "version": "v1", "enabled": true}
		case "/newmoi/workflow-templates":
			data = map[string]any{"id": 9, "workspace_id": "ws-1", "template_key": "tpl-key", "name": "tpl", "dsl_yaml": "workflow:\n  name: tpl", "input_form": `{"fields":[]}`, "is_builtin": false}
		case "/newmoi/workflow-templates/9":
			data = map[string]any{"id": 9, "workspace_id": "ws-1", "template_key": "tpl-key", "name": "tpl2", "description": "desc2", "dsl_yaml": "workflow:\n  name: tpl2", "input_form": `{"fields":[]}`, "is_builtin": false}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workflows := client.Workflows("ws-1")
	ctx := context.Background()
	enabled := true
	waitTimeout := 30
	configOverride := json.RawMessage(`{"node":{"input":"value"}}`)
	runtimeForm := map[string]any{"fields": []any{map[string]any{"field_id": "query"}}}
	runtimeLayout := map[string]any{"groups": []any{map[string]any{"group_id": "main"}}}
	if _, err := workflows.Deploy(ctx, "wf", "workflow:\n  name: wf", WithWorkflowID("wf-1"), WithWorkflowDescription("desc"), WithWorkflowExecutionMode("cron"), WithWorkflowCronExpression("* * * * *"), WithWorkflowSourceType("manual_dsl"), WithWorkflowInputForm(runtimeForm), WithWorkflowFormLayout(runtimeLayout), WithWorkflowDefaultValues(map[string]any{"k": "v"}), WithWorkflowComputeResourceID("cr-1"), WithWorkflowHTTPNotification("http://callback.test/completed", "COMPLETED", "FAILED", "CANCELLED")); err != nil {
		t.Fatal(err)
	}
	if _, err := workflows.Update(ctx, "wf-1", WithWorkflowUpdateName("wf2"), WithWorkflowUpdateDSLYAML("workflow:\n  name: wf2"), WithWorkflowUpdateInputForm(runtimeForm), WithWorkflowUpdateFormLayout(runtimeLayout)); err != nil {
		t.Fatal(err)
	}
	if _, err := workflows.CreateRunNodeRerun(ctx, "wf-1", "exec-1", "node-1", "continue_from_node_with_overrides", "node-exec-1", WithWorkflowRerunValues(map[string]any{"input": "value"}), WithWorkflowRerunConfigOverrideJSON(configOverride), WithExpectedWorkflowRerunContractHash("sha256:test-contract"), WithWorkflowRerunReason("fix input")); err != nil {
		t.Fatal(err)
	}
	if _, err := workflows.RetryRun(ctx, "wf-1", "exec-1", WithWorkflowRerunValues(map[string]any{"input": "value"}), WithWorkflowRerunTriggerNow(true)); err != nil {
		t.Fatal(err)
	}
	if cancelled, err := workflows.CancelRun(ctx, "wf-1", "exec-1", WithWorkflowCancelReason("stop")); err != nil {
		t.Fatal(err)
	} else if cancelled.GetExecution().GetDispatchJobId() != "dispatch-1" {
		t.Fatalf("cancelled execution dispatch_job_id = %q", cancelled.GetExecution().GetDispatchJobId())
	}
	if _, err := workflows.CreateCustomOperator(ctx,
		CustomOperatorSpec{
			Name:         "Text Counter",
			Identifier:   "text_counter",
			Description:  "Count text length",
			InputSchema:  map[string]any{"type": "object"},
			OutputSchema: map[string]any{"type": "object"},
		},
		WithCustomOperatorLanguage("python"),
		WithCustomOperatorCode("def handle(workspace_id, sdk, input):\n    return input"),
		WithCustomOperatorEnabled(enabled),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := workflows.TestCustomOperator(ctx, "7", CustomOperatorTestInput{Input: map[string]any{"text": "hello"}, WaitTimeoutSeconds: &waitTimeout}); err != nil {
		t.Fatal(err)
	}
	if _, err := workflows.UpdateCustomOperator(ctx, "7", WithCustomOperatorUpdateName("Counter"), WithCustomOperatorUpdateDescription("new desc"), WithCustomOperatorUpdateCode("def handle(workspace_id, sdk, input):\n    return input")); err != nil {
		t.Fatal(err)
	}
	if _, err := workflows.CreateWorkflowTemplate(ctx, "tpl", "workflow:\n  name: tpl", WithWorkflowTemplateKey("tpl-key"), WithWorkflowTemplateDescription("desc"), WithWorkflowTemplateInputForm(`{"fields":[]}`), WithWorkflowTemplateBuiltin(false)); err != nil {
		t.Fatal(err)
	}
	if _, err := workflows.UpdateWorkflowTemplate(ctx, "9", "tpl2", "workflow:\n  name: tpl2", WithWorkflowTemplateDescription("desc2")); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 10 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].method != http.MethodPost || requests[0].path != "/newmoi/workflow/v2/workflow-deployments" ||
		requests[0].body["name"] != "wf" ||
		requests[0].body["dsl_yaml"] != "workflow:\n  name: wf" ||
		requests[0].body["workflow_id"] != "wf-1" ||
		requests[0].body["description"] != "desc" ||
		requests[0].body["execution_mode"] != "cron" ||
		requests[0].body["cron_expression"] != "* * * * *" ||
		requests[0].body["source_type"] != "manual_dsl" ||
		requests[0].body["compute_resource_id"] != "cr-1" {
		t.Fatalf("deploy = %#v", requests[0])
	}
	if _, hasLegacy := requests[0].body["input_form"]; hasLegacy {
		t.Fatalf("deploy still uses legacy input_form: %#v", requests[0].body)
	}
	if !reflect.DeepEqual(requests[0].body["notification_config"], map[string]any{
		"type": "http", "url": "http://callback.test/completed", "notify_workflow_states": []any{"COMPLETED", "FAILED", "CANCELLED"},
	}) {
		t.Fatalf("deploy callback = %#v", requests[0].body["notification_config"])
	}
	if _, hasLegacy := requests[0].body["form_layout"]; hasLegacy {
		t.Fatalf("deploy still uses legacy form_layout: %#v", requests[0].body)
	}
	deployForm, ok := requests[0].body["runtime_fields"].(map[string]any)
	if !ok || deployForm["fields"] == nil {
		t.Fatalf("deploy runtime_fields = %#v", requests[0].body["runtime_fields"])
	}
	deployLayout, ok := requests[0].body["runtime_layout"].(map[string]any)
	if !ok || deployLayout["groups"] == nil {
		t.Fatalf("deploy runtime_layout = %#v", requests[0].body["runtime_layout"])
	}
	if requests[1].method != http.MethodPatch || requests[1].path != "/newmoi/workflow/v2/workflow-apps/wf-1" ||
		requests[1].body["name"] != "wf2" ||
		requests[1].body["dsl_yaml"] != "workflow:\n  name: wf2" {
		t.Fatalf("update = %#v", requests[1])
	}
	if _, hasLegacy := requests[1].body["input_form"]; hasLegacy {
		t.Fatalf("update still uses legacy input_form: %#v", requests[1].body)
	}
	updateForm, ok := requests[1].body["runtime_fields"].(map[string]any)
	if !ok || updateForm["fields"] == nil {
		t.Fatalf("update runtime_fields = %#v", requests[1].body["runtime_fields"])
	}
	updateLayout, ok := requests[1].body["runtime_layout"].(map[string]any)
	if !ok || updateLayout["groups"] == nil {
		t.Fatalf("update runtime_layout = %#v", requests[1].body["runtime_layout"])
	}
	if requests[2].path != "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/nodes/node-1/reruns" ||
		requests[2].body["mode"] != "continue_from_node_with_overrides" ||
		requests[2].body["source_node_execution_id"] != "node-exec-1" ||
		requests[2].body["expected_rerun_contract_hash"] != "sha256:test-contract" ||
		requests[2].body["reason"] != "fix input" {
		t.Fatalf("node rerun = %#v", requests[2])
	}
	if requests[3].path != "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/retry" ||
		requests[3].body["trigger_now"] != true {
		t.Fatalf("retry = %#v", requests[3])
	}
	if requests[4].path != "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/cancel" ||
		requests[4].body["reason"] != "stop" {
		t.Fatalf("cancel = %#v", requests[4])
	}
	if requests[5].path != "/newmoi/workflow/v2/custom-operators" ||
		requests[5].body["name"] != "Text Counter" ||
		requests[5].body["identifier"] != "text_counter" ||
		requests[5].body["description"] != "Count text length" ||
		requests[5].body["language"] != "python" ||
		requests[5].body["enabled"] != true {
		t.Fatalf("create custom operator = %#v", requests[5])
	}
	if requests[6].path != "/newmoi/workflow/v2/custom-operators/7/test-run" ||
		requests[6].body["wait_timeout_seconds"] != float64(30) {
		t.Fatalf("test custom operator = %#v", requests[6])
	}
	if requests[7].path != "/newmoi/workflow/v2/custom-operators/7" ||
		requests[7].body["name"] != "Counter" ||
		requests[7].body["description"] != "new desc" {
		t.Fatalf("update custom operator = %#v", requests[7])
	}
	if requests[8].path != "/newmoi/workflow-templates" ||
		requests[8].body["name"] != "tpl" ||
		requests[8].body["template_key"] != "tpl-key" ||
		requests[8].body["runtime_fields"] != `{"fields":[]}` ||
		requests[8].body["is_builtin"] != false {
		t.Fatalf("create template = %#v", requests[8])
	}
	if requests[9].path != "/newmoi/workflow-templates/9" ||
		requests[9].body["name"] != "tpl2" ||
		requests[9].body["dsl_yaml"] != "workflow:\n  name: tpl2" ||
		requests[9].body["description"] != "desc2" {
		t.Fatalf("update template = %#v", requests[9])
	}
}

func TestWorkflowEnableUsesLifecyclePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/newmoi/workflow/v2/workflow-apps/wf-1/enable" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"workflow": map[string]any{"id": "wf-1", "status": "ready", "execution_mode": "one_shot"}},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := workspace.Workflow("wf-1")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := workflow.Enable(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Workflow.Status != "ready" {
		t.Fatalf("status = %q", resp.Workflow.Status)
	}
}

func lineageTestResponse(method, path string) map[string]any {
	if method == http.MethodGet && strings.HasSuffix(path, "/overview") {
		return map[string]any{
			"artifact":            map[string]any{"artifact_id": "artifact-1", "root_asset_id": "root-1", "case_id": "case-1", "parsed_file_available": true},
			"workflow":            map[string]any{"workflow_id": "wf-1", "name": "workflow"},
			"workflow_invocation": map[string]any{"input": map[string]any{"file_id": "file-1"}, "vars": map[string]any{}},
			"topology":            map[string]any{"nodes": []any{map[string]any{"node_id": "node-1", "label": "Parse"}}, "edges": []any{}},
			"entry":               map[string]any{"catalog_file_id": "file-1"},
		}
	}
	if method == http.MethodGet && strings.HasSuffix(path, "/output-blocks") {
		nodeID := ""
		if strings.Contains(path, "/nodes/") {
			nodeID = "node-1"
		}
		return map[string]any{"artifact_id": "artifact-1", "node_id": nodeID, "blocks": []any{map[string]any{"block_id": "block-1"}}}
	}
	if method == http.MethodGet && strings.HasSuffix(path, "/revisions") {
		nodeID := ""
		if strings.Contains(path, "/nodes/") {
			nodeID = "node-1"
		}
		return map[string]any{"artifact_id": "artifact-1", "node_id": nodeID, "block_id": "block-1", "revisions": []any{map[string]any{"revision_id": "rev-1"}}}
	}
	if method == http.MethodPost && strings.HasSuffix(path, "/revisions") {
		nodeID := ""
		if strings.Contains(path, "/nodes/") {
			nodeID = "node-1"
		}
		return map[string]any{"artifact_id": "artifact-1", "node_id": nodeID, "block_id": "block-1", "revision": map[string]any{"revision_id": "rev-2"}}
	}
	if method == http.MethodPost && strings.HasSuffix(path, "/effective-revisions") {
		nodeID := ""
		if strings.Contains(path, "/nodes/") {
			nodeID = "node-1"
		}
		return map[string]any{"artifact_id": "artifact-1", "node_id": nodeID, "effective_set_version": 8}
	}
	if method == http.MethodGet && strings.Contains(path, "/nodes/") {
		return map[string]any{
			"workflow": map[string]any{"workflow_id": "wf-1"},
			"node":     map[string]any{"node_id": "node-1", "label": "Parse"},
			"run":      map[string]any{"status": "succeeded", "runtime_input": map[string]any{}, "runtime_output": map[string]any{}, "config": map[string]any{}},
		}
	}
	if method == http.MethodPost && strings.Contains(path, "/rerun") {
		return map[string]any{"artifact_id": "artifact-1", "node_id": "node-1", "node_execution_id": "node-exec-1", "rerun_id": "rerun-1", "branch_id": "branch-1", "case_id": "case-2", "status": "running"}
	}
	return map[string]any{}
}

func TestWorkspaceArtifactHandlesScopeLineageReview(t *testing.T) {
	var requests []struct {
		method      string
		path        string
		rawQuery    string
		workspaceID string
		body        map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method      string
			path        string
			rawQuery    string
			workspaceID string
			body        map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), rawQuery: r.URL.RawQuery, workspaceID: r.Header.Get("X-Workspace-ID"), body: body})
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": lineageTestResponse(r.Method, r.URL.EscapedPath())})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	file := &FileHandle{volume: &VolumeHandle{database: &DatabaseHandle{catalog: &CatalogHandle{workspace: workspace, id: 10, name: "orders"}, id: 20, name: "raw"}, id: 30, name: "landing"}, id: "file-1"}
	if _, err := file.LineageOverview(context.Background(), WithQueryParam("scope", "latest")); err != nil {
		t.Fatal(err)
	}
	artifact, err := workspace.Artifact("artifact-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := artifact.Overview(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := artifact.OutputBlocks(context.Background()); err != nil {
		t.Fatal(err)
	}
	block, err := artifact.Block("block-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := block.Revisions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := block.CreateRevision(context.Background(), WithLineageRevisionContentPayload("fixed"), WithLineageExpectedEffectiveSetVersion(7)); err != nil {
		t.Fatal(err)
	}
	switches := []LineageRevisionSwitch{{BlockID: "block-1", EffectiveRevisionID: "rev-2"}}
	if _, err := artifact.SwitchEffectiveRevisions(context.Background(), switches, WithLineageSwitchExpectedEffectiveSetVersion(8)); err != nil {
		t.Fatal(err)
	}
	node, err := artifact.Node("node-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := node.Info(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := node.OutputBlocks(context.Background()); err != nil {
		t.Fatal(err)
	}
	nodeBlock, err := node.Block("node-block-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nodeBlock.Revisions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := nodeBlock.CreateRevision(context.Background(), WithLineageUseCurrentEffective(true)); err != nil {
		t.Fatal(err)
	}
	if _, err := node.SwitchEffectiveRevisions(context.Background(), switches); err != nil {
		t.Fatal(err)
	}
	if _, err := node.Rerun(context.Background(), WithLineageRerunMode("from_node"), WithLineageRerunInputPayloadOverride(json.RawMessage(`{"id":1}`))); err != nil {
		t.Fatal(err)
	}
	if _, err := node.RerunTargetWithRevisions(context.Background(), "target-1", WithLineageRevisionRerunMode("selected"), WithLineageSelectedRevisionsJSON(json.RawMessage(`{"block-1":"rev-2"}`)), WithLineageRevisionRerunSourceEffectiveSetVersion(9)); err != nil {
		t.Fatal(err)
	}

	expected := []struct {
		method   string
		path     string
		rawQuery string
	}{
		{http.MethodGet, "/newmoi/lineage/catalog-files/file-1/overview", "scope=latest"},
		{http.MethodGet, "/newmoi/lineage/artifacts/artifact-1/overview", ""},
		{http.MethodGet, "/newmoi/lineage/artifacts/artifact-1/output-blocks", ""},
		{http.MethodGet, "/newmoi/lineage/artifacts/artifact-1/blocks/block-1/revisions", ""},
		{http.MethodPost, "/newmoi/lineage/artifacts/artifact-1/blocks/block-1/revisions", ""},
		{http.MethodPost, "/newmoi/lineage/artifacts/artifact-1/effective-revisions", ""},
		{http.MethodGet, "/newmoi/lineage/artifacts/artifact-1/nodes/node-1", ""},
		{http.MethodGet, "/newmoi/lineage/artifacts/artifact-1/nodes/node-1/output-blocks", ""},
		{http.MethodGet, "/newmoi/lineage/artifacts/artifact-1/nodes/node-1/blocks/node-block-1/revisions", ""},
		{http.MethodPost, "/newmoi/lineage/artifacts/artifact-1/nodes/node-1/blocks/node-block-1/revisions", ""},
		{http.MethodPost, "/newmoi/lineage/artifacts/artifact-1/nodes/node-1/effective-revisions", ""},
		{http.MethodPost, "/newmoi/lineage/artifacts/artifact-1/nodes/node-1/rerun", ""},
		{http.MethodPost, "/newmoi/lineage/artifacts/artifact-1/nodes/node-1/output-emissions/target-1/rerun-with-revisions", ""},
	}
	if len(requests) != len(expected) {
		t.Fatalf("request count = %d", len(requests))
	}
	for i, want := range expected {
		if requests[i].method != want.method || requests[i].path != want.path || requests[i].rawQuery != want.rawQuery || requests[i].workspaceID != "ws-1" {
			t.Fatalf("request %d = %#v", i, requests[i])
		}
	}
	if requests[4].body["revision_content_payload"] != "fixed" || requests[4].body["expected_effective_set_version"] != float64(7) {
		t.Fatalf("block revision body = %#v", requests[4].body)
	}
	if requests[5].body["expected_effective_set_version"] != float64(8) {
		t.Fatalf("artifact switch body = %#v", requests[5].body)
	}
	gotSwitches, ok := requests[5].body["switches"].([]any)
	if !ok || len(gotSwitches) != 1 {
		t.Fatalf("switches = %#v", requests[5].body["switches"])
	}
	if requests[9].body["use_current_effective"] != true {
		t.Fatalf("node block revision body = %#v", requests[9].body)
	}
	if stringBody, ok := requests[11].body["input_payload_override"].(map[string]any); !ok || stringBody["id"] != float64(1) {
		t.Fatalf("node rerun body = %#v", requests[11].body)
	}
	if requests[12].body["mode"] != "selected" || requests[12].body["source_effective_set_version"] != float64(9) {
		t.Fatalf("target rerun body = %#v", requests[12].body)
	}
}

func TestWorkspaceArtifactHandlesValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.Artifact(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("artifact err = %v", err)
	}
	artifact, err := workspace.Artifact("artifact-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := artifact.Block(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("block err = %v", err)
	}
	if _, err := artifact.Node(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("node err = %v", err)
	}
	node, err := artifact.Node("node-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := node.Block(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("node block err = %v", err)
	}
}

func TestWorkflowMethodsValidateRequiredArguments(t *testing.T) {
	client, err := New("http://example.invalid", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	workflows := client.Workflows("ws-1")
	ctx := context.Background()
	if _, err := workflows.Deploy(ctx, "", "dsl"); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("deploy name err = %v", err)
	}
	if _, err := workflows.Deploy(ctx, "wf", ""); !IsCode(err, ErrDSLYAMLRequired.Code) {
		t.Fatalf("deploy dsl err = %v", err)
	}
	if _, err := workflows.CreateRunNodeRerun(ctx, "wf", "exec", "node", "", "node-exec"); !IsCode(err, ErrModeRequired.Code) {
		t.Fatalf("rerun mode err = %v", err)
	}
	if _, err := workflows.CreateRunNodeRerun(ctx, "wf", "exec", "node", "continue_from_node", ""); !IsCode(err, ErrSourceNodeRequired.Code) {
		t.Fatalf("rerun source err = %v", err)
	}
	if _, err := workflows.CreateRunNodeRerun(ctx, "wf", "exec", "node", "continue_from_node", "node-exec"); !IsCode(err, ErrRerunContractHashRequired.Code) {
		t.Fatalf("rerun contract hash err = %v", err)
	}
	if _, err := workflows.CreateRunNodeRerun(ctx, "wf", "exec", "node", "continue_from_node", "node-exec", WithExpectedWorkflowRerunContractHash(" \t")); !IsCode(err, ErrRerunContractHashRequired.Code) {
		t.Fatalf("blank rerun contract hash err = %v", err)
	}
	if _, err := workflows.CreateCustomOperator(ctx, CustomOperatorSpec{Identifier: "id", Description: "desc", InputSchema: map[string]any{}, OutputSchema: map[string]any{}}); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("custom operator name err = %v", err)
	}
	if _, err := workflows.CreateCustomOperator(ctx, CustomOperatorSpec{Name: "name", Description: "desc", InputSchema: map[string]any{}, OutputSchema: map[string]any{}}); !IsCode(err, ErrIdentifierRequired.Code) {
		t.Fatalf("custom operator identifier err = %v", err)
	}
	if _, err := workflows.CreateCustomOperator(ctx, CustomOperatorSpec{Name: "name", Identifier: "id", InputSchema: map[string]any{}, OutputSchema: map[string]any{}}); !IsCode(err, ErrDescriptionRequired.Code) {
		t.Fatalf("custom operator description err = %v", err)
	}
	if _, err := workflows.CreateCustomOperator(ctx, CustomOperatorSpec{Name: "name", Identifier: "id", Description: "desc", OutputSchema: map[string]any{}}); !IsCode(err, ErrInputSchemaRequired.Code) {
		t.Fatalf("custom operator input schema err = %v", err)
	}
	if _, err := workflows.CreateCustomOperator(ctx, CustomOperatorSpec{Name: "name", Identifier: "id", Description: "desc", InputSchema: map[string]any{}}); !IsCode(err, ErrOutputSchemaRequired.Code) {
		t.Fatalf("custom operator output schema err = %v", err)
	}
	if _, err := workflows.GetRunResult(ctx, "", "exec-1"); !IsCode(err, ErrWorkflowIDRequired.Code) {
		t.Fatalf("get run result workflow id err = %v", err)
	}
	if _, err := workflows.GetRunResult(ctx, "wf-1", ""); !IsCode(err, ErrExecutionIDRequired.Code) {
		t.Fatalf("get run result execution id err = %v", err)
	}
	if _, err := workflows.GetRunResultByID(ctx, ""); !IsCode(err, ErrExecutionIDRequired.Code) {
		t.Fatalf("get run result by id execution id err = %v", err)
	}
	if _, err := workflows.CreateWorkflowTemplate(ctx, "", "dsl"); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("template name err = %v", err)
	}
	if _, err := workflows.CreateWorkflowTemplate(ctx, "tpl", ""); !IsCode(err, ErrDSLYAMLRequired.Code) {
		t.Fatalf("template dsl err = %v", err)
	}
	if _, err := workflows.UpdateWorkflowTemplate(ctx, "9", "", "dsl"); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("update template name err = %v", err)
	}
	if _, err := workflows.UpdateWorkflowTemplate(ctx, "9", "tpl", ""); !IsCode(err, ErrDSLYAMLRequired.Code) {
		t.Fatalf("update template dsl err = %v", err)
	}
}

func TestCustomOperatorSpecBodyIncludesCompleteCreateContract(t *testing.T) {
	nodeID := "node-spec"
	kind := "python"
	language := "python"
	handler := "handle"
	version := "v1"
	isolationLevel := "process"
	enabled := false
	code := "def handle(workspace_id, sdk, input): return input"
	sourceFileID := "source-spec"
	baseNodeID := "base-spec"
	baseNodeVersion := "v0"
	catalogID := int64(11)
	databaseID := int64(22)
	spec := CustomOperatorSpec{
		Name:            "Text Counter",
		Identifier:      "text_counter",
		Description:     "Count text length",
		InputSchema:     map[string]any{"type": "object"},
		OutputSchema:    map[string]any{"type": "object"},
		NodeID:          &nodeID,
		Kind:            &kind,
		Language:        &language,
		Handler:         &handler,
		Version:         &version,
		IsolationLevel:  &isolationLevel,
		Enabled:         &enabled,
		Code:            &code,
		SourceFileID:    &sourceFileID,
		BaseNodeID:      &baseNodeID,
		BaseNodeVersion: &baseNodeVersion,
		BindingConfig:   map[string]any{"runtime": "sandbox"},
		CatalogID:       &catalogID,
		DatabaseID:      &databaseID,
	}
	body, err := spec.Body()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"name": "Text Counter", "identifier": "text_counter", "description": "Count text length",
		"input_schema": map[string]any{"type": "object"}, "output_schema": map[string]any{"type": "object"},
		"node_id": "node-spec", "kind": "python", "language": "python", "handler": "handle",
		"version": "v1", "isolation_level": "process", "enabled": false, "code": code,
		"source_file_id": "source-spec", "base_node_id": "base-spec", "base_node_version": "v0",
		"binding_config": map[string]any{"runtime": "sandbox"}, "catalog_id": int64(11), "database_id": int64(22),
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("custom operator body = %#v, want %#v", body, want)
	}

	body, err = spec.Body(
		WithCustomOperatorLanguage("python-option"),
		WithCustomOperatorEnabled(true),
		WithCustomOperatorCatalogID(33),
	)
	if err != nil {
		t.Fatal(err)
	}
	if body["language"] != "python-option" || body["enabled"] != true || body["catalog_id"] != int64(33) {
		t.Fatalf("custom operator options did not override spec = %#v", body)
	}
}

func TestWorkspaceWorkflowHandleScopesLifecycle(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		query     string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method    string
			path      string
			query     string
			workspace string
			body      map[string]any
		}{
			method:    r.Method,
			path:      r.URL.EscapedPath(),
			query:     r.URL.RawQuery,
			workspace: r.Header.Get(headerWorkspaceID),
			body:      body,
		})
		data := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/connectors/db/database/list":
			data = map[string]any{"databases": []string{"db1"}}
		case "/newmoi/workflow/v2/workflow-deployments":
			data = map[string]any{"workflow": map[string]any{"id": "wf-1", "name": "wf", "source_type": "manual_dsl", "execution_mode": "one_shot", "status": "ready"}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1":
			if r.Method == http.MethodDelete {
				data = map[string]any{"deleted": true}
			} else {
				data = map[string]any{"workflow": map[string]any{"id": "wf-1", "name": "wf", "source_type": "manual_dsl", "status": "ready", "execution_mode": "one_shot", "dsl_yaml": "workflow:\n  name: wf"}}
			}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions":
			if r.Method == http.MethodGet {
				data = map[string]any{"total": 1, "executions": []any{map[string]any{"execution_id": "exec-1", "workflow_id": "wf-1", "status": "running", "execution_mode": "one_shot"}}}
			} else {
				data = map[string]any{"workflow_run": map[string]any{"execution_id": "exec-1", "workflow_id": "wf-1", "status": "running", "execution_mode": "one_shot"}}
			}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1":
			if r.Method == http.MethodDelete {
				data = map[string]any{"deleted": true}
			} else {
				data = map[string]any{"execution": map[string]any{"execution_id": "exec-1", "workflow_id": "wf-1", "status": "running", "execution_mode": "one_shot"}}
			}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/refresh":
			data = map[string]any{"execution": map[string]any{"execution_id": "exec-1", "workflow_id": "wf-1", "status": "succeeded", "execution_mode": "one_shot"}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/result":
			data = map[string]any{"result": map[string]any{"execution_id": "exec-1", "workflow_id": "wf-1", "status": "succeeded", "case_result": "done"}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/import-context":
			data = map[string]any{"import_context": map[string]any{"task_id": "task-1", "run_id": "run-1", "workflow_execution_id": "exec-1", "status": 1}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/import-files":
			data = map[string]any{"files": []any{}, "total": 0, "total_sum": 0, "total_success": 0, "total_failed": 0}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/nodes/node-1":
			data = map[string]any{
				"node": map[string]any{"flow_id": "root", "name": "node-1", "node_key": "node-1", "node_type": "custom", "display_name": "Node 1"},
				"run":  map[string]any{"status": "succeeded", "runtime_input": map[string]any{"file_id": "file-1"}, "runtime_output": map[string]any{"ok": true}, "config": map[string]any{}},
			}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/nodes/node-1/rerun-plan":
			data = map[string]any{"plan": map[string]any{
				"workflow_id":              "wf-1",
				"source_execution_id":      "exec-1",
				"source_case_id":           "case-1",
				"node_key":                 "node-1",
				"source_node_execution_id": "node-exec-1",
				"rerun_contract_hash":      "sha256:test-contract",
				"available_modes":          []string{"continue_from_node"},
				"validation":               map[string]any{"ok": true},
			}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/nodes/node-1/reruns":
			data = map[string]any{"rerun": map[string]any{"rerun_id": "rerun-1", "branch_id": "branch-1", "case_id": "case-2", "mode": "continue_from_node", "status": "running"}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/rerun-branches":
			data = map[string]any{"reruns": []any{map[string]any{"rerun_id": "rerun-1", "branch_id": "branch-1", "case_id": "case-2", "mode": "continue_from_node", "status": "running"}}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/reruns/rerun-1":
			data = map[string]any{"rerun": map[string]any{"rerun_id": "rerun-1", "branch_id": "branch-1", "case_id": "case-2", "mode": "continue_from_node", "status": "running"}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/reruns/rerun-1/cancel":
			data = map[string]any{"rerun": map[string]any{"rerun_id": "rerun-1", "branch_id": "branch-1", "case_id": "case-2", "mode": "continue_from_node", "status": "cancelled"}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/retry":
			data = map[string]any{"workflow_run": map[string]any{"execution_id": "exec-2", "workflow_id": "wf-1", "status": "running", "execution_mode": "one_shot"}}
		case "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/cancel":
			data = map[string]any{"execution": map[string]any{"execution_id": "exec-1", "workflow_id": "wf-1", "status": "cancelled", "execution_mode": "one_shot"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": data,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	workflow, deployed, err := workspace.DeployWorkflow(ctx, "wf", "workflow:\n  name: wf", "", WithWorkflowDescription("desc"))
	if err != nil {
		t.Fatalf("deploy workflow err = %v", err)
	}
	if workflow.ID() != "wf-1" {
		t.Fatalf("workflow id = %q", workflow.ID())
	}
	if deployed == nil {
		t.Fatal("missing deploy result")
	}
	knownWorkflow, err := workspace.Workflow("wf-1")
	if err != nil {
		t.Fatalf("workflow handle err = %v", err)
	}
	if knownWorkflow.ID() != "wf-1" {
		t.Fatalf("known workflow id = %q", knownWorkflow.ID())
	}
	if _, err := workflow.Info(ctx); err != nil {
		t.Fatalf("info err = %v", err)
	}
	if _, err := workflow.Update(ctx, WithWorkflowUpdateName("wf2")); err != nil {
		t.Fatalf("update err = %v", err)
	}
	if _, err := workflow.Run(ctx, WithWorkflowRunInput(map[string]any{"file_id": "file-1"})); err != nil {
		t.Fatalf("run err = %v", err)
	}
	if _, err := workflow.Runs(ctx, WithPage(2)); err != nil {
		t.Fatalf("runs err = %v", err)
	}
	run, err := workflow.RunHandle("exec-1")
	if err != nil {
		t.Fatalf("run handle err = %v", err)
	}
	if run.ID() != "exec-1" {
		t.Fatalf("run id = %q", run.ID())
	}
	if _, err := run.Info(ctx); err != nil {
		t.Fatalf("run info err = %v", err)
	}
	if _, err := run.Refresh(ctx); err != nil {
		t.Fatalf("refresh err = %v", err)
	}
	if _, err := run.Result(ctx); err != nil {
		t.Fatalf("result err = %v", err)
	}
	importContext, err := run.ImportContext(ctx)
	if err != nil {
		t.Fatalf("import context err = %v", err)
	}
	if importContext.ImportContext.GetTaskId() != "task-1" {
		t.Fatalf("import context task id = %q", importContext.ImportContext.GetTaskId())
	}
	importFiles, err := run.ImportFiles(ctx, WithQueryParam("kind", "output"))
	if err != nil {
		t.Fatalf("import files err = %v", err)
	}
	if importFiles.GetTotal() != 0 {
		t.Fatalf("import files total = %d", importFiles.GetTotal())
	}
	nodeDetail, err := run.Node(ctx, "node-1")
	if err != nil {
		t.Fatalf("node err = %v", err)
	}
	if nodeDetail.GetNode().GetNodeKey() != "node-1" {
		t.Fatalf("node key = %q", nodeDetail.GetNode().GetNodeKey())
	}
	node, err := run.NodeHandle("node-1")
	if err != nil {
		t.Fatalf("node handle err = %v", err)
	}
	if node.Key() != "node-1" {
		t.Fatalf("node key = %q", node.Key())
	}
	nodeInfo, err := node.Info(ctx)
	if err != nil {
		t.Fatalf("node info err = %v", err)
	}
	if nodeInfo.GetNode().GetNodeKey() != "node-1" {
		t.Fatalf("node info key = %q", nodeInfo.GetNode().GetNodeKey())
	}
	rerunPlan, err := node.RerunPlan(ctx, WithQueryParam("mode", "from_node"))
	if err != nil {
		t.Fatalf("node rerun plan err = %v", err)
	}
	if !rerunPlan.GetPlan().GetValidation().GetOk() {
		t.Fatal("rerun plan validation is not ok")
	}
	if rerunPlan.GetPlan().GetRerunContractHash() != "sha256:test-contract" {
		t.Fatalf("rerun contract hash = %q", rerunPlan.GetPlan().GetRerunContractHash())
	}
	nodeRerun, err := node.CreateRerun(ctx, "continue_from_node", "node-exec-1", WithExpectedWorkflowRerunContractHash(rerunPlan.GetPlan().GetRerunContractHash()), WithWorkflowRerunReason("fix input"))
	if err != nil {
		t.Fatalf("node create rerun err = %v", err)
	}
	if nodeRerun.GetRerun().GetRerunId() != "rerun-1" {
		t.Fatalf("node rerun id = %q", nodeRerun.GetRerun().GetRerunId())
	}
	rerunBranches, err := run.RerunBranches(ctx)
	if err != nil {
		t.Fatalf("rerun branches err = %v", err)
	}
	if len(rerunBranches.GetReruns()) != 1 || rerunBranches.GetReruns()[0].GetRerunId() != "rerun-1" {
		t.Fatalf("rerun branches = %#v", rerunBranches.GetReruns())
	}
	rerun, err := run.Rerun("rerun-1")
	if err != nil {
		t.Fatalf("rerun handle err = %v", err)
	}
	if rerun.ID() != "rerun-1" {
		t.Fatalf("rerun id = %q", rerun.ID())
	}
	rerunInfo, err := rerun.Info(ctx)
	if err != nil {
		t.Fatalf("rerun info err = %v", err)
	}
	if rerunInfo.GetRerun().GetRerunId() != "rerun-1" {
		t.Fatalf("rerun info id = %q", rerunInfo.GetRerun().GetRerunId())
	}
	rerunCancelled, err := rerun.Cancel(ctx)
	if err != nil {
		t.Fatalf("rerun cancel err = %v", err)
	}
	if rerunCancelled.GetRerun().GetStatus() != "cancelled" {
		t.Fatalf("rerun cancel status = %q", rerunCancelled.GetRerun().GetStatus())
	}
	if _, err := run.Retry(ctx, WithWorkflowRerunReason("retry")); err == nil {
		t.Fatal("retry with unsupported reason unexpectedly succeeded")
	}
	if _, err := run.Retry(ctx); err != nil {
		t.Fatalf("retry err = %v", err)
	}
	if _, err := run.Cancel(ctx, WithWorkflowCancelReason("stop")); err != nil {
		t.Fatalf("cancel err = %v", err)
	}
	if _, err := run.Delete(ctx); err != nil {
		t.Fatalf("run delete err = %v", err)
	}
	if _, err := workflow.Delete(ctx); err != nil {
		t.Fatalf("workflow delete err = %v", err)
	}
	if _, err := workspace.Workflow(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty workflow err = %v", err)
	}
	if _, err := workflow.RunHandle(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty run err = %v", err)
	}
	if _, err := run.NodeHandle(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty node err = %v", err)
	}
	if _, err := run.Rerun(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty rerun err = %v", err)
	}

	if len(requests) != 21 {
		t.Fatalf("request count = %d: %#v", len(requests), requests)
	}
	for i, request := range requests {
		if request.workspace != "ws-1" {
			t.Fatalf("request %d workspace = %q", i, request.workspace)
		}
	}
	if requests[0].method != http.MethodPost || requests[0].path != "/newmoi/workflow/v2/workflow-deployments" ||
		!reflect.DeepEqual(requests[0].body, map[string]any{"name": "wf", "dsl_yaml": "workflow:\n  name: wf", "description": "desc"}) {
		t.Fatalf("deploy request = %#v", requests[0])
	}
	expected := []struct {
		method string
		path   string
		query  string
	}{
		{http.MethodGet, "/newmoi/workflow/v2/workflow-apps/wf-1", ""},
		{http.MethodPatch, "/newmoi/workflow/v2/workflow-apps/wf-1", ""},
		{http.MethodPost, "/newmoi/workflow/v2/workflow-apps/wf-1/executions", ""},
		{http.MethodGet, "/newmoi/workflow/v2/workflow-apps/wf-1/executions", "page=2"},
		{http.MethodGet, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1", ""},
		{http.MethodPost, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/refresh", ""},
		{http.MethodGet, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/result", ""},
		{http.MethodGet, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/import-context", ""},
		{http.MethodGet, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/import-files", "kind=output"},
		{http.MethodGet, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/nodes/node-1", ""},
		{http.MethodGet, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/nodes/node-1", ""},
		{http.MethodGet, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/nodes/node-1/rerun-plan", "mode=from_node"},
		{http.MethodPost, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/nodes/node-1/reruns", ""},
		{http.MethodGet, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/rerun-branches", ""},
		{http.MethodGet, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/reruns/rerun-1", ""},
		{http.MethodPost, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/reruns/rerun-1/cancel", ""},
		{http.MethodPost, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/retry", ""},
		{http.MethodPost, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1/cancel", ""},
		{http.MethodDelete, "/newmoi/workflow/v2/workflow-apps/wf-1/executions/exec-1", ""},
		{http.MethodDelete, "/newmoi/workflow/v2/workflow-apps/wf-1", ""},
	}
	for i, want := range expected {
		got := requests[i+1]
		if got.method != want.method || got.path != want.path || got.query != want.query {
			t.Fatalf("request %d = %#v, want %#v", i+1, got, want)
		}
	}
	if requests[2].body["name"] != "wf2" {
		t.Fatalf("update body = %#v", requests[2].body)
	}
	values, ok := requests[3].body["values"].(map[string]any)
	if !ok || values["file_id"] != "file-1" {
		t.Fatalf("run body = %#v", requests[3].body)
	}
	if requests[13].body["mode"] != "continue_from_node" ||
		requests[13].body["source_node_execution_id"] != "node-exec-1" ||
		requests[13].body["expected_rerun_contract_hash"] != "sha256:test-contract" ||
		requests[13].body["reason"] != "fix input" {
		t.Fatalf("node rerun body = %#v", requests[13].body)
	}
	if _, ok := requests[17].body["reason"]; ok {
		t.Fatalf("retry body = %#v", requests[17].body)
	}
	if requests[18].body["reason"] != "stop" {
		t.Fatalf("cancel body = %#v", requests[18].body)
	}
}

func TestWorkspaceWorkflowDesignHandlesScopeOperatorsAndTemplates(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method    string
			path      string
			workspace string
			body      map[string]any
		}{
			method:    r.Method,
			path:      r.URL.EscapedPath(),
			workspace: r.Header.Get(headerWorkspaceID),
			body:      body,
		})
		data := map[string]any{}
		switch r.URL.EscapedPath() {
		case "/newmoi/workflow/v2/custom-operators":
			data = map[string]any{"id": 1, "node_id": "moi:custom.operator:workspace:text_counter", "name": "Text Counter", "identifier": "text_counter", "description": "Count text length", "input_schema": `{}`, "output_schema": `{}`, "enabled": true}
		case "/newmoi/workflow/v2/custom-operators/1":
			data = map[string]any{"id": 1, "node_id": "moi:custom.operator:workspace:text_counter", "name": "Text Counter", "identifier": "text_counter", "description": "Count text length", "input_schema": `{}`, "output_schema": `{}`, "enabled": true}
		case "/newmoi/workflow/v2/custom-operators/1/code":
			data = map[string]any{"source": "def handle(event): return event"}
		case "/newmoi/workflow/v2/custom-operators/1/test-run":
			data = map[string]any{"test_run": map[string]any{"operator_id": 1, "node_id": "moi:custom.operator:workspace:text_counter", "version": "v1", "status": "passed", "runtime_input": map[string]any{"text": "hello"}, "runtime_output": map[string]any{"length": 5}, "wait_timeout_seconds": 30}}
		case "/newmoi/workflow/v2/custom-operators/1/enable":
			data = map[string]any{"id": 1, "node_id": "moi:custom.operator:workspace:text_counter", "enabled": true}
		case "/newmoi/workflow/v2/custom-operators/1/disable":
			data = map[string]any{"id": 1, "node_id": "moi:custom.operator:workspace:text_counter", "enabled": false}
		case "/newmoi/workflow-templates":
			data = map[string]any{"id": 1, "template_key": "tpl-key", "name": "tpl", "dsl_yaml": "workflow:\n  name: tpl"}
		case "/newmoi/workflow-templates/1":
			data = map[string]any{"id": 1, "template_key": "tpl-key", "name": "tpl", "dsl_yaml": "workflow:\n  name: tpl"}
		}
		if r.Method == http.MethodDelete && r.URL.EscapedPath() == "/newmoi/workflow/v2/custom-operators/1" {
			data = map[string]any{"deleted": true}
		}
		if r.Method == http.MethodDelete && r.URL.EscapedPath() == "/newmoi/workflow-templates/1" {
			data = map[string]any{"message": "deleted"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": data,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	enabled := true
	operatorSpec := CustomOperatorSpec{
		Name:         "Text Counter",
		Identifier:   "text_counter",
		Description:  "Count text length",
		InputSchema:  map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string", "description": "Text to count"}}, "required": []any{"text"}},
		OutputSchema: map[string]any{"type": "object", "properties": map[string]any{"length": map[string]any{"type": "integer", "description": "Text length"}}},
	}
	operator, createdOperator, err := workspace.CreateCustomOperatorFromSpec(ctx, operatorSpec, WithCustomOperatorLanguage("python"), WithCustomOperatorCode("def handle(workspace_id, sdk, input): return {'length': len(input['text'])}"), WithCustomOperatorEnabled(enabled))
	if err != nil {
		t.Fatalf("create operator err = %v", err)
	}
	if operator.ID() != "1" || createdOperator == nil {
		t.Fatalf("operator = %q/%v", operator.ID(), createdOperator)
	}
	knownOperator, err := workspace.CustomOperator("1")
	if err != nil {
		t.Fatalf("known operator err = %v", err)
	}
	if knownOperator.ID() != "1" {
		t.Fatalf("known operator id = %q", knownOperator.ID())
	}
	if _, err := operator.Info(ctx); err != nil {
		t.Fatalf("operator info err = %v", err)
	}
	if _, err := operator.Code(ctx); err != nil {
		t.Fatalf("operator code err = %v", err)
	}
	waitTimeout := 30
	if _, err := operator.TestWithInput(ctx, CustomOperatorTestInput{Input: map[string]any{"text": "hello"}, WaitTimeoutSeconds: &waitTimeout}); err != nil {
		t.Fatalf("operator test err = %v", err)
	}
	if _, err := operator.Update(ctx, WithCustomOperatorUpdateName("Counter"), WithCustomOperatorUpdateDescription("new desc")); err != nil {
		t.Fatalf("operator update err = %v", err)
	}
	if _, err := operator.Enable(ctx); err != nil {
		t.Fatalf("operator enable err = %v", err)
	}
	if _, err := operator.Disable(ctx); err != nil {
		t.Fatalf("operator disable err = %v", err)
	}
	template, createdTemplate, err := workspace.CreateWorkflowTemplate(ctx, "tpl", "workflow:\n  name: tpl", WithWorkflowTemplateKey("tpl-key"))
	if err != nil {
		t.Fatalf("create template err = %v", err)
	}
	if template.ID() != "1" || createdTemplate == nil {
		t.Fatalf("template = %q/%v", template.ID(), createdTemplate)
	}
	knownTemplate, err := workspace.WorkflowTemplate("1")
	if err != nil {
		t.Fatalf("known template err = %v", err)
	}
	if knownTemplate.ID() != "1" {
		t.Fatalf("known template id = %q", knownTemplate.ID())
	}
	if _, err := template.Info(ctx); err != nil {
		t.Fatalf("template info err = %v", err)
	}
	if _, err := template.Update(ctx, "tpl2", "workflow:\n  name: tpl2", WithWorkflowTemplateDescription("desc2")); err != nil {
		t.Fatalf("template update err = %v", err)
	}
	if _, err := template.Delete(ctx); err != nil {
		t.Fatalf("template delete err = %v", err)
	}
	if _, err := operator.Delete(ctx); err != nil {
		t.Fatalf("operator delete err = %v", err)
	}
	if _, err := workspace.CustomOperator(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty operator err = %v", err)
	}
	if _, err := workspace.WorkflowTemplate(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty template err = %v", err)
	}

	if len(requests) != 12 {
		t.Fatalf("request count = %d: %#v", len(requests), requests)
	}
	for i, request := range requests {
		if request.workspace != "ws-1" {
			t.Fatalf("request %d workspace = %q", i, request.workspace)
		}
	}
	expected := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/newmoi/workflow/v2/custom-operators"},
		{http.MethodGet, "/newmoi/workflow/v2/custom-operators/1"},
		{http.MethodGet, "/newmoi/workflow/v2/custom-operators/1/code"},
		{http.MethodPost, "/newmoi/workflow/v2/custom-operators/1/test-run"},
		{http.MethodPut, "/newmoi/workflow/v2/custom-operators/1"},
		{http.MethodPost, "/newmoi/workflow/v2/custom-operators/1/enable"},
		{http.MethodPost, "/newmoi/workflow/v2/custom-operators/1/disable"},
		{http.MethodPost, "/newmoi/workflow-templates"},
		{http.MethodGet, "/newmoi/workflow-templates/1"},
		{http.MethodPut, "/newmoi/workflow-templates/1"},
		{http.MethodDelete, "/newmoi/workflow-templates/1"},
		{http.MethodDelete, "/newmoi/workflow/v2/custom-operators/1"},
	}
	for i, want := range expected {
		got := requests[i]
		if got.method != want.method || got.path != want.path {
			t.Fatalf("request %d = %#v, want %#v", i, got, want)
		}
	}
	if requests[0].body["name"] != "Text Counter" || requests[0].body["identifier"] != "text_counter" || requests[0].body["enabled"] != true || requests[0].body["code"] == "" {
		t.Fatalf("create operator body = %#v", requests[0].body)
	}
	if requests[3].body["wait_timeout_seconds"] != float64(30) {
		t.Fatalf("test operator body = %#v", requests[3].body)
	}
	if requests[7].body["template_key"] != "tpl-key" {
		t.Fatalf("create template body = %#v", requests[7].body)
	}
	if _, _, err := workspace.CreateCustomOperatorFromSpec(ctx, CustomOperatorSpec{Identifier: "id", Description: "desc", InputSchema: map[string]any{}, OutputSchema: map[string]any{}}); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("empty custom operator name err = %v", err)
	}
	if _, _, err := workspace.CreateCustomOperatorFromSpec(ctx, CustomOperatorSpec{Name: "name", Description: "desc", InputSchema: map[string]any{}, OutputSchema: map[string]any{}}); !IsCode(err, ErrIdentifierRequired.Code) {
		t.Fatalf("empty custom operator identifier err = %v", err)
	}
	if _, _, err := workspace.CreateCustomOperatorFromSpec(ctx, CustomOperatorSpec{Name: "name", Identifier: "id", InputSchema: map[string]any{}, OutputSchema: map[string]any{}}); !IsCode(err, ErrDescriptionRequired.Code) {
		t.Fatalf("empty custom operator description err = %v", err)
	}
	if _, _, err := workspace.CreateCustomOperatorFromSpec(ctx, CustomOperatorSpec{Name: "name", Identifier: "id", Description: "desc", OutputSchema: map[string]any{}}); !IsCode(err, ErrInputSchemaRequired.Code) {
		t.Fatalf("empty custom operator input schema err = %v", err)
	}
	if _, _, err := workspace.CreateCustomOperatorFromSpec(ctx, CustomOperatorSpec{Name: "name", Identifier: "id", Description: "desc", InputSchema: map[string]any{}}); !IsCode(err, ErrOutputSchemaRequired.Code) {
		t.Fatalf("empty custom operator output schema err = %v", err)
	}
	if _, err := operator.TestWithInput(ctx, CustomOperatorTestInput{}); !IsCode(err, ErrPayloadRequired.Code) {
		t.Fatalf("empty custom operator test input err = %v", err)
	}
}

func TestWorkspaceWorkitemAPIServiceHandleScopesLifecycle(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		query     string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method    string
			path      string
			query     string
			workspace string
			body      map[string]any
		}{
			method:    r.Method,
			path:      r.URL.EscapedPath(),
			query:     r.URL.RawQuery,
			workspace: r.Header.Get(headerWorkspaceID),
			body:      body,
		})
		data := map[string]any{"service": map[string]any{
			"api_service_id": "api_123",
			"service_name":   "operator-api-abc",
			"version":        "v1",
			"status":         "ready",
		}}
		if strings.HasSuffix(r.URL.Path, "/invoke") {
			data = map[string]any{"result": map[string]any{"status": "succeeded", "result": map[string]any{"text": "hello"}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": data,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	service, err := workspace.WorkitemAPIService("moi:files.read_text", "v1")
	if err != nil {
		t.Fatalf("api service handle err = %v", err)
	}
	if service.NodeID() != "moi:files.read_text" || service.Version() != "v1" {
		t.Fatalf("service scope = %q/%q", service.NodeID(), service.Version())
	}
	info, err := service.Info(ctx)
	if err != nil {
		t.Fatalf("service info err = %v", err)
	}
	if info.GetService().GetVersion() != "v1" || info.GetService().GetApiServiceId() != "api_123" {
		t.Fatalf("service info = %#v", info)
	}
	published, err := service.Publish(ctx, WorkitemAPIServicePublishSpec{
		Version:           "v1",
		ServiceName:       "operator-api-abc",
		ComputeResourceID: "compute-1",
		TimeoutSeconds:    45,
		MaxConcurrency:    3,
		RateLimitPerMin:   60,
	})
	if err != nil {
		t.Fatalf("service publish err = %v", err)
	}
	if published.GetService() == nil {
		t.Fatalf("service publish = %#v", published)
	}
	enabled, err := service.Enable(ctx)
	if err != nil {
		t.Fatalf("service enable err = %v", err)
	}
	if enabled.GetService() == nil {
		t.Fatalf("service enable = %#v", enabled)
	}
	invoked, err := service.Invoke(ctx, WorkitemAPIServiceInvokeSpec{
		ServiceName: "operator-api-abc",
		Type:        "operator",
		Version:     "v1",
		Payload:     json.RawMessage(`{"text":"hello"}`),
	})
	if err != nil {
		t.Fatalf("service invoke err = %v", err)
	}
	if invoked.GetResult() == nil {
		t.Fatalf("service invoke = %#v", invoked)
	}
	disabled, err := service.Disable(ctx)
	if err != nil {
		t.Fatalf("service disable err = %v", err)
	}
	if disabled.GetService() == nil {
		t.Fatalf("service disable = %#v", disabled)
	}
	if len(requests) != 5 {
		t.Fatalf("request count = %d: %#v", len(requests), requests)
	}
	expected := []struct {
		method string
		path   string
		query  string
	}{
		{http.MethodGet, "/newmoi/workflow/v2/workitems/catalog/moi:files.read_text/api-service", "version=v1"},
		{http.MethodPost, "/newmoi/workflow/v2/workitems/catalog/moi:files.read_text/api-service", ""},
		{http.MethodPost, "/newmoi/workflow/v2/workitems/catalog/moi:files.read_text/api-service/enable", "version=v1"},
		{http.MethodPost, "/newmoi/workflow/v2/workitems/catalog/moi:files.read_text/api-service/invoke", ""},
		{http.MethodPost, "/newmoi/workflow/v2/workitems/catalog/moi:files.read_text/api-service/disable", "version=v1"},
	}
	for i, want := range expected {
		got := requests[i]
		if got.method != want.method || got.path != want.path || got.query != want.query || got.workspace != "ws-1" {
			t.Fatalf("request %d = %#v, want %#v", i, got, want)
		}
	}
	if requests[1].body["version"] != "v1" ||
		requests[1].body["service_name"] != "operator-api-abc" ||
		requests[1].body["compute_resource_id"] != "compute-1" ||
		requests[1].body["timeout_seconds"] != float64(45) ||
		requests[1].body["max_concurrency"] != float64(3) ||
		requests[1].body["rate_limit_per_min"] != float64(60) {
		t.Fatalf("publish body = %#v", requests[1].body)
	}
	if requests[3].body["service_name"] != "operator-api-abc" || requests[3].body["type"] != "operator" || requests[3].body["version"] != "v1" {
		t.Fatalf("invoke body = %#v", requests[3].body)
	}
	payload, ok := requests[3].body["payload"].(map[string]any)
	if !ok || payload["text"] != "hello" {
		t.Fatalf("invoke payload = %#v", requests[3].body["payload"])
	}
	if _, err := workspace.WorkitemAPIService("", "v1"); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty node err = %v", err)
	}
	if _, err := workspace.WorkitemAPIService("moi:files.read_text", ""); !IsCode(err, ErrVersionRequired.Code) {
		t.Fatalf("empty version err = %v", err)
	}
	if _, err := workspace.Workflows().PublishWorkitemAPIService(ctx, "moi:files.read_text", WorkitemAPIServicePublishSpec{}); !IsCode(err, ErrVersionRequired.Code) {
		t.Fatalf("empty publish version err = %v", err)
	}
	if _, err := service.Invoke(ctx, WorkitemAPIServiceInvokeSpec{Type: "operator", Version: "v1", Payload: json.RawMessage(`{}`)}); !IsCode(err, ErrServiceNameRequired.Code) {
		t.Fatalf("empty invoke service name err = %v", err)
	}
	if _, err := service.Invoke(ctx, WorkitemAPIServiceInvokeSpec{ServiceName: "operator-api-abc", Version: "v1", Payload: json.RawMessage(`{}`)}); !IsCode(err, ErrTypeRequired.Code) {
		t.Fatalf("empty invoke type err = %v", err)
	}
	if _, err := service.Invoke(ctx, WorkitemAPIServiceInvokeSpec{ServiceName: "operator-api-abc", Type: "operator", Version: "v1"}); !IsCode(err, ErrPayloadRequired.Code) {
		t.Fatalf("empty invoke payload err = %v", err)
	}
}

func TestWorkitemAPIServiceInvokeValidationDetails(t *testing.T) {
	for _, tc := range []struct {
		name, code, field, reason, expected, message string
		spec                                         WorkitemAPIServiceInvokeSpec
	}{
		{name: "service name", code: ErrServiceNameRequired.Code, field: "service_name", reason: "required", message: "published service.service_name", spec: WorkitemAPIServiceInvokeSpec{Type: "operator", Version: "v1", Payload: json.RawMessage(`{}`)}},
		{name: "type", code: ErrTypeRequired.Code, field: "type", reason: "required", expected: "operator", message: `use "operator"`, spec: WorkitemAPIServiceInvokeSpec{ServiceName: "service", Version: "v1", Payload: json.RawMessage(`{}`)}},
		{name: "version", code: ErrVersionRequired.Code, field: "version", reason: "required", message: "published service.version", spec: WorkitemAPIServiceInvokeSpec{ServiceName: "service", Type: "operator", Payload: json.RawMessage(`{}`)}},
		{name: "payload", code: ErrPayloadRequired.Code, field: "payload", reason: "required", message: "not an input wrapper", spec: WorkitemAPIServiceInvokeSpec{ServiceName: "service", Type: "operator", Version: "v1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.spec.Validate()
			var productErr *Error
			if !errors.As(err, &productErr) || productErr.Code != tc.code {
				t.Fatalf("validation error = %v, want %s", err, tc.code)
			}
			if !strings.Contains(productErr.Message, tc.message) || productErr.Details["field"] != tc.field || productErr.Details["reason"] != tc.reason {
				t.Fatalf("validation error lacks actionable details: %#v", productErr)
			}
			if tc.expected != "" && productErr.Details["expected"] != tc.expected {
				t.Fatalf("validation expected = %#v, want %q", productErr.Details["expected"], tc.expected)
			}
		})
	}
}

func TestWorkspaceConnectorHandleScopesTasks(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		query     string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method    string
			path      string
			query     string
			workspace string
			body      map[string]any
		}{
			method:    r.Method,
			path:      r.URL.EscapedPath(),
			query:     r.URL.RawQuery,
			workspace: r.Header.Get(headerWorkspaceID),
			body:      body,
		})
		data := map[string]any{"ok": true}
		switch r.URL.EscapedPath() {
		case "/newmoi/connectors/get":
			data = map[string]any{"id": "conn-1"}
		case "/newmoi/connectors":
			data = map[string]any{"id": "conn-1"}
		case "/newmoi/connectors/db/database/list":
			data = map[string]any{"databases": []string{"db1"}}
		case "/newmoi/connectors/db/database/schema/list":
			data = map[string]any{"schemas": []string{"public"}}
		case "/newmoi/connectors/db/database/table/list":
			data = map[string]any{"tables": []string{"orders"}}
		case "/newmoi/connectors/db/schema":
			data = map[string]any{"columns": []map[string]any{{
				"name":           "id",
				"data_type":      "INT",
				"mapped_type":    "INT64",
				"is_primary_key": true,
				"is_nullable":    false,
				"is_auto_incr":   true,
			}}}
		case "/newmoi/connectors/mo/database/list":
			data = map[string]any{"databases": []string{"analytics"}}
		case "/newmoi/connectors/mo/database/table/list":
			data = map[string]any{"tables": []string{"orders"}}
		case "/newmoi/connectors/mo/check_table_name":
			data = map[string]any{"exists": false}
		case "/newmoi/connectors/mo/column/mapping":
			data = map[string]any{"column_map": map[string]any{"src_id": []any{"id"}}, "need_mapping_columns": []string{"src_id"}}
		case "/newmoi/task":
			data = map[string]any{"task_id": "task-1"}
		case "/newmoi/task/get":
			data = map[string]any{"task": map[string]any{"id": "task-1"}}
		case "/newmoi/task/runs":
			data = map[string]any{"runs": []map[string]any{}}
		case "/newmoi/task/files":
			data = map[string]any{"files": []map[string]any{}}
		case "/newmoi/export/task/create":
			data = map[string]any{"id": "export-1"}
		case "/newmoi/export/task/info":
			data = map[string]any{"id": "export-1"}
		case "/newmoi/export/task/files":
			data = map[string]any{"files": []map[string]any{}}
		case "/newmoi/export/task/export-1/state":
			data = map[string]any{"total": 0, "pending": 0, "running": 0, "failed": 0, "completed": 0}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": data,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	connector, created, err := workspace.CreateConnector(ctx, "oss", 1, 2, WithConnectorConfig(map[string]any{"bucket": "b"}))
	if err != nil {
		t.Fatalf("create connector err = %v", err)
	}
	if connector.ID() != "conn-1" {
		t.Fatalf("connector id = %q", connector.ID())
	}
	if created == nil {
		t.Fatal("missing connector create result")
	}
	if created.GetId() != "conn-1" {
		t.Fatalf("created connector id = %q", created.GetId())
	}
	known, err := workspace.Connector("conn-1")
	if err != nil {
		t.Fatalf("known connector err = %v", err)
	}
	if known.ID() != "conn-1" {
		t.Fatalf("known connector id = %q", known.ID())
	}
	if _, err := connector.Info(ctx); err != nil {
		t.Fatalf("info err = %v", err)
	}
	if err := connector.Update(ctx, WithConnectorName("oss-prod")); err != nil {
		t.Fatalf("update err = %v", err)
	}
	sourceDatabases, err := connector.SourceDatabases(ctx)
	if err != nil {
		t.Fatalf("source db err = %v", err)
	}
	if got := sourceDatabases.GetDatabases(); len(got) != 1 || got[0] != "db1" {
		t.Fatalf("source dbs = %#v", got)
	}
	sourceSchemas, err := connector.SourceSchemas(ctx, "db1")
	if err != nil {
		t.Fatalf("source schemas err = %v", err)
	}
	if got := sourceSchemas.GetSchemas(); len(got) != 1 || got[0] != "public" {
		t.Fatalf("source schemas = %#v", got)
	}
	sourceTables, err := connector.SourceTables(ctx, "db1", WithConnectorSchema("public"))
	if err != nil {
		t.Fatalf("source tables err = %v", err)
	}
	if got := sourceTables.GetTables(); len(got) != 1 || got[0] != "orders" {
		t.Fatalf("source tables = %#v", got)
	}
	sourceSchema, err := connector.SourceSchema(ctx, "db1", "orders", WithConnectorSchema("public"))
	if err != nil {
		t.Fatalf("source schema err = %v", err)
	}
	if got := sourceSchema.GetColumns(); len(got) != 1 || got[0].GetName() != "id" || got[0].GetMappedType() != "INT64" || !got[0].GetIsPrimaryKey() || got[0].GetIsNullable() || !got[0].GetIsAutoIncr() {
		t.Fatalf("source schema = %#v", got)
	}
	moDatabases, err := connector.MODatabases(ctx)
	if err != nil {
		t.Fatalf("mo databases err = %v", err)
	}
	if got := moDatabases.GetDatabases(); len(got) != 1 || got[0] != "analytics" {
		t.Fatalf("mo databases = %#v", got)
	}
	moTables, err := connector.MODatabaseTables(ctx, "analytics")
	if err != nil {
		t.Fatalf("mo database tables err = %v", err)
	}
	if got := moTables.GetTables(); len(got) != 1 || got[0] != "orders" {
		t.Fatalf("mo tables = %#v", got)
	}
	checkedTable, err := connector.CheckMOTableName(ctx, "analytics", "orders")
	if err != nil {
		t.Fatalf("check mo table name err = %v", err)
	}
	if checkedTable.GetExists() {
		t.Fatalf("checked table = %#v", checkedTable)
	}
	columnMapping, err := connector.MOColumnMapping(ctx, "analytics", "orders", map[string]string{"src_id": "id"})
	if err != nil {
		t.Fatalf("mo column mapping err = %v", err)
	}
	if got := columnMapping.GetColumnMap()["src_id"].GetValues(); len(got) != 1 || got[0].GetStringValue() != "id" || len(columnMapping.GetNeedMappingColumns()) != 1 || columnMapping.GetNeedMappingColumns()[0] != "src_id" {
		t.Fatalf("column mapping = %#v", columnMapping)
	}
	importTask, imported, err := connector.CreateImportTask(ctx, 1, map[string]any{"tables": []any{"orders"}}, WithImportTaskName("load orders"))
	if err != nil {
		t.Fatalf("create import task err = %v", err)
	}
	if importTask.ID() != "task-1" {
		t.Fatalf("import task id = %q", importTask.ID())
	}
	if imported == nil {
		t.Fatal("missing import task result")
	}
	if imported.GetTaskId() != "task-1" {
		t.Fatalf("imported task id = %q", imported.GetTaskId())
	}
	keepStructure := true
	fileImportTask, _, err := connector.CreateFileImportTask(ctx, FileImportTaskSpec{
		ConfigType:         1,
		Name:               "load files",
		VolumeID:           "volume-1",
		URIs:               []string{"oss://bucket/orders.csv"},
		LoadModeConfig:     map[string]any{"mode": 1},
		FileFilterConfig:   map[string]any{"file_types": []any{1}},
		UnzipKeepStructure: &keepStructure,
		Dedup:              map[string]any{"by": []any{"path"}, "strategy": "keep"},
	})
	if err != nil {
		t.Fatalf("create file import task err = %v", err)
	}
	if fileImportTask.ID() != "task-1" {
		t.Fatalf("file import task id = %q", fileImportTask.ID())
	}
	knownImport, err := connector.ImportTask("task-1")
	if err != nil {
		t.Fatalf("known import task err = %v", err)
	}
	if knownImport.ID() != "task-1" {
		t.Fatalf("known import id = %q", knownImport.ID())
	}
	if _, err := importTask.Info(ctx); err != nil {
		t.Fatalf("import info err = %v", err)
	}
	if err := importTask.Update(ctx, ImportTaskUpdateSpec{Name: "load orders v2"}); err != nil {
		t.Fatalf("import update err = %v", err)
	}
	if err := importTask.UpdateConfig(ctx, ImportTaskUpdateSpec{
		Name:             "load orders typed",
		LoadModeConfig:   map[string]any{"mode": 2, "interval": 30},
		FileFilterConfig: map[string]any{"file_types": []any{7}},
		URIs:             []string{"oss://bucket/orders-v2.csv"},
	}); err != nil {
		t.Fatalf("import update config err = %v", err)
	}
	if err := importTask.Pause(ctx); err != nil {
		t.Fatalf("pause err = %v", err)
	}
	if err := importTask.Resume(ctx); err != nil {
		t.Fatalf("resume err = %v", err)
	}
	if _, err := importTask.Runs(ctx); err != nil {
		t.Fatalf("runs err = %v", err)
	}
	if _, err := importTask.Files(ctx, WithQueryParam("status", "failed")); err != nil {
		t.Fatalf("files err = %v", err)
	}
	if err := importTask.RetryFiles(ctx, "file-1"); err != nil {
		t.Fatalf("retry import files err = %v", err)
	}
	isRaw := true
	exportTask, exported, err := connector.CreateExportTask(ctx, "export orders", "oss", 2, ExportTaskConfig{S3Config: map[string]any{"path": "/out"}}, []ExportTaskFile{{FileID: "file-1", FullPath: []string{"db", "orders"}, IsRaw: &isRaw}})
	if err != nil {
		t.Fatalf("create export task err = %v", err)
	}
	if exportTask.ID() != "export-1" {
		t.Fatalf("export task id = %q", exportTask.ID())
	}
	if exported == nil {
		t.Fatal("missing export task result")
	}
	mergeTitle := true
	specExportTask, _, err := connector.CreateExportTaskFromSpec(ctx, ExportTaskSpec{
		Name:          "export spec orders",
		ConnectorName: "oss",
		Type:          2,
		Config: ExportTaskConfig{
			MergeTitleToText: &mergeTitle,
			S3Config:         map[string]any{"path": "/out/spec", "need_compress": true, "compress_method": "zip"},
		},
		Files: []ExportTaskFile{{
			FileID:   "file-2",
			FullPath: []string{"db", "orders", "parsed"},
			IsRaw:    &keepStructure,
		}},
	})
	if err != nil {
		t.Fatalf("create export task from spec err = %v", err)
	}
	if specExportTask.ID() != "export-1" {
		t.Fatalf("spec export task id = %q", specExportTask.ID())
	}
	knownExport, err := connector.ExportTask("export-1")
	if err != nil {
		t.Fatalf("known export task err = %v", err)
	}
	if knownExport.ID() != "export-1" {
		t.Fatalf("known export id = %q", knownExport.ID())
	}
	if _, err := exportTask.Info(ctx); err != nil {
		t.Fatalf("export info err = %v", err)
	}
	if _, err := exportTask.Files(ctx, WithExportTaskFilesLimit(10)); err != nil {
		t.Fatalf("export files err = %v", err)
	}
	if err := exportTask.Retry(ctx, "file-1"); err != nil {
		t.Fatalf("export retry err = %v", err)
	}
	if _, err := exportTask.State(ctx); err != nil {
		t.Fatalf("export state err = %v", err)
	}
	if err := exportTask.Delete(ctx); err != nil {
		t.Fatalf("export delete err = %v", err)
	}
	if err := importTask.Delete(ctx); err != nil {
		t.Fatalf("import delete err = %v", err)
	}
	if err := connector.Delete(ctx); err != nil {
		t.Fatalf("connector delete err = %v", err)
	}
	if _, err := workspace.Connector(""); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("empty connector err = %v", err)
	}
	if _, err := connector.ImportTask(""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("empty import task err = %v", err)
	}
	if _, err := connector.ExportTask(""); !IsCode(err, ErrTaskIDRequired.Code) {
		t.Fatalf("empty export task err = %v", err)
	}
	if _, _, err := connector.CreateFileImportTask(ctx, FileImportTaskSpec{Name: "load", VolumeID: "volume-1", URIs: []string{"oss://bucket/orders.csv"}, LoadModeConfig: map[string]any{"mode": 1}, FileFilterConfig: map[string]any{"file_types": []any{1}}}); !IsCode(err, ErrTypeRequired.Code) {
		t.Fatalf("empty file import type err = %v", err)
	}
	if err := (FileImportTaskSpec{ConfigType: 1, VolumeID: "volume-1", URIs: []string{"oss://bucket/orders.csv"}, LoadModeConfig: map[string]any{"mode": 1}, FileFilterConfig: map[string]any{"file_types": []any{1}}}).Validate(); err != nil {
		t.Fatalf("optional file import name err = %v", err)
	}
	if err := (FileImportTaskSpec{ConfigType: 1, Name: "load", URIs: []string{"oss://bucket/orders.csv"}, LoadModeConfig: map[string]any{"mode": 1}, FileFilterConfig: map[string]any{"file_types": []any{1}}}).Validate(); err != nil {
		t.Fatalf("optional file import volume err = %v", err)
	}
	if _, _, err := connector.CreateFileImportTask(ctx, FileImportTaskSpec{ConfigType: 1, Name: "load", VolumeID: "volume-1", LoadModeConfig: map[string]any{"mode": 1}, FileFilterConfig: map[string]any{"file_types": []any{1}}}); !IsCode(err, ErrURLRequired.Code) {
		t.Fatalf("empty file import uri err = %v", err)
	}
	if _, _, err := connector.CreateFileImportTask(ctx, FileImportTaskSpec{ConfigType: 1, Name: "load", VolumeID: "volume-1", URIs: []string{"oss://bucket/orders.csv"}, FileFilterConfig: map[string]any{"file_types": []any{1}}}); !IsCode(err, ErrModeRequired.Code) {
		t.Fatalf("empty file import load mode err = %v", err)
	}
	if _, _, err := connector.CreateFileImportTask(ctx, FileImportTaskSpec{ConfigType: 1, Name: "load", VolumeID: "volume-1", URIs: []string{"oss://bucket/orders.csv"}, LoadModeConfig: map[string]any{"mode": 1}}); !IsCode(err, ErrSourceConfigRequired.Code) {
		t.Fatalf("empty file import filter err = %v", err)
	}
	if err := importTask.UpdateConfig(ctx, ImportTaskUpdateSpec{}); !IsCode(err, ErrSourceConfigRequired.Code) {
		t.Fatalf("empty import update spec err = %v", err)
	}
	if err := importTask.UpdateConfig(ctx, ImportTaskUpdateSpec{URIs: []string{}}); !IsCode(err, ErrURLRequired.Code) {
		t.Fatalf("empty import update uris err = %v", err)
	}
	if err := (ExportTaskSpec{ConnectorID: "conn-1", ConnectorName: "oss", Type: 2, Config: ExportTaskConfig{S3Config: map[string]any{"path": "/out"}}, Files: []ExportTaskFile{{FileID: "file-1", FullPath: []string{"db"}}}}).Validate(); !IsCode(err, ErrTaskNameRequired.Code) {
		t.Fatalf("empty export task name err = %v", err)
	}
	if _, _, err := connector.CreateExportTaskFromSpec(ctx, ExportTaskSpec{Name: "export", Type: 2, Config: ExportTaskConfig{S3Config: map[string]any{"path": "/out"}}, Files: []ExportTaskFile{{FileID: "file-1", FullPath: []string{"db"}}}}); !IsCode(err, ErrConnectorIDRequired.Code) {
		t.Fatalf("empty export task connector err = %v", err)
	}
	if _, _, err := connector.CreateExportTaskFromSpec(ctx, ExportTaskSpec{Name: "export", ConnectorName: "oss", Config: ExportTaskConfig{S3Config: map[string]any{"path": "/out"}}, Files: []ExportTaskFile{{FileID: "file-1", FullPath: []string{"db"}}}}); !IsCode(err, ErrTypeRequired.Code) {
		t.Fatalf("empty export task type err = %v", err)
	}
	if _, _, err := connector.CreateExportTaskFromSpec(ctx, ExportTaskSpec{Name: "export", ConnectorName: "oss", Type: 2, Files: []ExportTaskFile{{FileID: "file-1", FullPath: []string{"db"}}}}); !IsCode(err, ErrConfigRequired.Code) {
		t.Fatalf("empty export task config err = %v", err)
	}
	if _, _, err := connector.CreateExportTaskFromSpec(ctx, ExportTaskSpec{Name: "export", ConnectorName: "oss", Type: 2, Config: ExportTaskConfig{S3Config: map[string]any{"path": "/out"}}}); !IsCode(err, ErrFileIDsRequired.Code) {
		t.Fatalf("empty export task files err = %v", err)
	}
	if _, _, err := connector.CreateExportTaskFromSpec(ctx, ExportTaskSpec{Name: "export", ConnectorName: "oss", Type: 2, Config: ExportTaskConfig{S3Config: map[string]any{"path": "/out"}}, Files: []ExportTaskFile{{FullPath: []string{"db"}}}}); !IsCode(err, ErrFileIDRequired.Code) {
		t.Fatalf("empty export file id err = %v", err)
	}
	if _, _, err := connector.CreateExportTaskFromSpec(ctx, ExportTaskSpec{Name: "export", ConnectorName: "oss", Type: 2, Config: ExportTaskConfig{S3Config: map[string]any{"path": "/out"}}, Files: []ExportTaskFile{{FileID: "file-1"}}}); !IsCode(err, ErrPathInvalid.Code) {
		t.Fatalf("empty export file path err = %v", err)
	}

	if len(requests) != 30 {
		t.Fatalf("request count = %d: %#v", len(requests), requests)
	}
	for i, request := range requests {
		if request.workspace != "ws-1" {
			t.Fatalf("request %d workspace = %q", i, request.workspace)
		}
	}
	expected := []struct {
		method string
		path   string
		query  string
	}{
		{http.MethodPost, "/newmoi/connectors", ""},
		{http.MethodPost, "/newmoi/connectors/get", ""},
		{http.MethodPut, "/newmoi/connectors/conn-1", ""},
		{http.MethodPost, "/newmoi/connectors/db/database/list", ""},
		{http.MethodPost, "/newmoi/connectors/db/database/schema/list", ""},
		{http.MethodPost, "/newmoi/connectors/db/database/table/list", ""},
		{http.MethodPost, "/newmoi/connectors/db/schema", ""},
		{http.MethodPost, "/newmoi/connectors/mo/database/list", ""},
		{http.MethodPost, "/newmoi/connectors/mo/database/table/list", ""},
		{http.MethodPost, "/newmoi/connectors/mo/check_table_name", ""},
		{http.MethodPost, "/newmoi/connectors/mo/column/mapping", ""},
		{http.MethodPost, "/newmoi/task", ""},
		{http.MethodPost, "/newmoi/task", ""},
		{http.MethodGet, "/newmoi/task/get", "task_id=task-1"},
		{http.MethodPost, "/newmoi/task/update", ""},
		{http.MethodPost, "/newmoi/task/update", ""},
		{http.MethodPost, "/newmoi/task/pause", ""},
		{http.MethodPost, "/newmoi/task/resume", ""},
		{http.MethodGet, "/newmoi/task/runs", "task_id=task-1"},
		{http.MethodGet, "/newmoi/task/files", "status=failed&task_id=task-1"},
		{http.MethodPost, "/newmoi/task/retry", ""},
		{http.MethodPost, "/newmoi/export/task/create", ""},
		{http.MethodPost, "/newmoi/export/task/create", ""},
		{http.MethodPost, "/newmoi/export/task/info", ""},
		{http.MethodPost, "/newmoi/export/task/files", ""},
		{http.MethodPost, "/newmoi/export/task/export-1/rerun", ""},
		{http.MethodGet, "/newmoi/export/task/export-1/state", ""},
		{http.MethodPost, "/newmoi/export/task/delete", ""},
		{http.MethodPost, "/newmoi/task/delete", ""},
		{http.MethodDelete, "/newmoi/connectors/conn-1", ""},
	}
	for i, want := range expected {
		got := requests[i]
		if got.method != want.method || got.path != want.path || got.query != want.query {
			t.Fatalf("request %d = %#v, want %#v", i, got, want)
		}
	}
	if requests[0].body["name"] != "oss" || requests[0].body["source_type"] != float64(1) || requests[0].body["usage_type"] != float64(2) {
		t.Fatalf("create connector body = %#v", requests[0].body)
	}
	if requests[7].body["connector_id"] != "conn-1" {
		t.Fatalf("mo databases body = %#v", requests[7].body)
	}
	if requests[8].body["connector_id"] != "conn-1" || requests[8].body["database"] != "analytics" {
		t.Fatalf("mo tables body = %#v", requests[8].body)
	}
	if requests[9].body["connector_id"] != "conn-1" || requests[9].body["database"] != "analytics" || requests[9].body["table"] != "orders" {
		t.Fatalf("check mo table body = %#v", requests[9].body)
	}
	if requests[10].body["connector_id"] != "conn-1" || requests[10].body["database"] != "analytics" || requests[10].body["table"] != "orders" {
		t.Fatalf("mo mapping body = %#v", requests[10].body)
	}
	if requests[11].body["source_connector_id"] != "conn-1" || requests[11].body["name"] != "load orders" {
		t.Fatalf("create import body = %#v", requests[11].body)
	}
	sourceConfig, ok := requests[12].body["source_config"].(map[string]any)
	if !ok {
		t.Fatalf("file import source_config = %#v", requests[12].body["source_config"])
	}
	commonConfig, ok := sourceConfig["common_file_task_config"].(map[string]any)
	if !ok || commonConfig["unzip_keep_structure"] != true {
		t.Fatalf("file import common config = %#v", sourceConfig["common_file_task_config"])
	}
	if requests[15].body["name"] != "load orders typed" {
		t.Fatalf("typed import update body = %#v", requests[15].body)
	}
	if _, present := requests[15].body["volume_id"]; present {
		t.Fatalf("typed import update sent unsupported volume_id: %#v", requests[15].body)
	}
	if requests[19].query != "status=failed&task_id=task-1" {
		t.Fatalf("import files query = %q", requests[19].query)
	}
	if requests[21].body["connector_id"] != "conn-1" || requests[21].body["task_name"] != "export orders" {
		t.Fatalf("create export body = %#v", requests[21].body)
	}
	config, ok := requests[22].body["config"].(map[string]any)
	if !ok {
		t.Fatalf("spec export config = %#v", requests[22].body["config"])
	}
	if config["merge_title_to_text"] != true {
		t.Fatalf("spec export config = %#v", config)
	}
	if requests[24].body["task_id"] != "export-1" || requests[24].body["limit"] != float64(10) {
		t.Fatalf("export files body = %#v", requests[24].body)
	}
}

func TestWorkspaceConnectorFileHandleScopesTemporaryFileLifecycle(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		query     string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() == "/newmoi/connectors/file/stream" {
			requests = append(requests, struct {
				method    string
				path      string
				query     string
				workspace string
				body      map[string]any
			}{
				method:    r.Method,
				path:      r.URL.EscapedPath(),
				query:     r.URL.RawQuery,
				workspace: r.Header.Get(headerWorkspaceID),
			})
			_, _ = w.Write([]byte("stream-body"))
			return
		}

		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method    string
			path      string
			query     string
			workspace string
			body      map[string]any
		}{
			method:    r.Method,
			path:      r.URL.EscapedPath(),
			query:     r.URL.RawQuery,
			workspace: r.Header.Get(headerWorkspaceID),
			body:      body,
		})
		data := map[string]any{"ok": true}
		if r.URL.EscapedPath() == "/newmoi/connectors/file/preview" {
			data = map[string]any{"conn_file_id": "cf-1", "file_type": 1, "rows": []map[string]any{}}
		} else if r.URL.EscapedPath() == "/newmoi/connectors/file/download" {
			data = map[string]any{"url": "https://example.test/cf-1"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": data,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	connector, err := workspace.Connector("conn-1")
	if err != nil {
		t.Fatal(err)
	}
	file, err := connector.File("cf-1")
	if err != nil {
		t.Fatalf("connector file err = %v", err)
	}
	if file.ID() != "cf-1" {
		t.Fatalf("file id = %q", file.ID())
	}
	known, err := workspace.ConnectorFile("cf-1")
	if err != nil {
		t.Fatalf("known file err = %v", err)
	}
	if known.ID() != "cf-1" {
		t.Fatalf("known file id = %q", known.ID())
	}
	ctx := context.Background()
	if _, err := file.Preview(ctx, WithConnectorPreviewRowStart(2)); err != nil {
		t.Fatalf("preview err = %v", err)
	}
	download, err := file.Download(ctx)
	if err != nil {
		t.Fatalf("download err = %v", err)
	}
	if download.GetUrl() != "https://example.test/cf-1" {
		t.Fatalf("download url = %q", download.GetUrl())
	}
	stream, err := known.Stream(ctx)
	if err != nil {
		t.Fatalf("stream err = %v", err)
	}
	data, err := io.ReadAll(stream.Body)
	if closeErr := stream.Body.Close(); closeErr != nil {
		t.Fatalf("close err = %v", closeErr)
	}
	if err != nil {
		t.Fatalf("read stream err = %v", err)
	}
	if string(data) != "stream-body" {
		t.Fatalf("stream body = %q", string(data))
	}
	if err := known.Delete(ctx); err != nil {
		t.Fatalf("delete err = %v", err)
	}
	if _, err := workspace.ConnectorFile(""); !IsCode(err, ErrConnectorFileIDRequired.Code) {
		t.Fatalf("empty workspace connector file err = %v", err)
	}
	if _, err := connector.File(""); !IsCode(err, ErrConnectorFileIDRequired.Code) {
		t.Fatalf("empty connector file err = %v", err)
	}
	if _, err := known.Preview(ctx); err != nil {
		t.Fatalf("workspace-only preview err = %v", err)
	}

	if len(requests) != 5 {
		t.Fatalf("request count = %d: %#v", len(requests), requests)
	}
	for i, request := range requests {
		if request.workspace != "ws-1" {
			t.Fatalf("request %d workspace = %q", i, request.workspace)
		}
	}
	expected := []struct {
		method string
		path   string
		query  string
	}{
		{http.MethodPost, "/newmoi/connectors/file/preview", ""},
		{http.MethodPost, "/newmoi/connectors/file/download", ""},
		{http.MethodGet, "/newmoi/connectors/file/stream", "conn_file_id=cf-1"},
		{http.MethodPost, "/newmoi/connectors/file/delete", ""},
		{http.MethodPost, "/newmoi/connectors/file/preview", ""},
	}
	for i, want := range expected {
		got := requests[i]
		if got.method != want.method || got.path != want.path || got.query != want.query {
			t.Fatalf("request %d = %#v, want %#v", i, got, want)
		}
	}
	if requests[0].body["connector_id"] != "conn-1" || requests[0].body["conn_file_id"] != "cf-1" || requests[0].body["rowStart"] != float64(2) {
		t.Fatalf("preview body = %#v", requests[0].body)
	}
	if requests[1].body["conn_file_id"] != "cf-1" {
		t.Fatalf("download body = %#v", requests[1].body)
	}
	if requests[3].body["conn_file_id"] != "cf-1" {
		t.Fatalf("delete body = %#v", requests[3].body)
	}
	if requests[4].body["connector_id"] != "" || requests[4].body["conn_file_id"] != "cf-1" {
		t.Fatalf("workspace preview body = %#v", requests[4].body)
	}
}

func TestWorkspaceSemanticModelHandleScopesEntries(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		query     string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method    string
			path      string
			query     string
			workspace string
			body      map[string]any
		}{
			method:    r.Method,
			path:      r.URL.EscapedPath(),
			query:     r.URL.RawQuery,
			workspace: r.Header.Get(headerWorkspaceID),
			body:      body,
		})
		data := map[string]any{"deleted": true}
		switch {
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/semantic-models":
			data = map[string]any{"id": 101, "name": "orders-model", "description": "orders", "tables": []any{"orders"}, "created_at": 11, "updated_at": 11}
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/semantic-models/101":
			data = map[string]any{"id": 101, "name": "orders-model", "description": "orders", "tables": []any{"orders"}, "created_at": 11, "updated_at": 11}
		case r.Method == http.MethodPut && r.URL.EscapedPath() == "/newmoi/semantic-models/101":
			data = map[string]any{"updated": true}
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/semantic-models/101/entries":
			data = map[string]any{"items": []any{}, "total": 0, "next_page_token": ""}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/semantic-models/101/entries":
			data = map[string]any{"id": 201, "kind": "metric", "key": "revenue", "tables": []any{"orders"}, "spec": map[string]any{"expr": "sum(amount)"}, "created_at": 12, "updated_at": 12}
		case r.Method == http.MethodPut && r.URL.EscapedPath() == "/newmoi/semantic-models/101/entries/201":
			data = map[string]any{"updated": true}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/semantic-models/101/import":
			data = map[string]any{"imported": 1, "model_id": 101}
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/semantic-models/101/export":
			data = map[string]any{
				"model":   map[string]any{"id": 101, "name": "orders-model", "tables": []any{"orders"}, "created_at": 11, "updated_at": 11},
				"entries": []any{},
			}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/semantic-models/101/validate":
			data = map[string]any{"valid": true}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	model, created, err := workspace.CreateSemanticModel(ctx, "orders-model", WithSemanticModelDescription("orders"), WithSemanticModelTables([]string{"orders"}))
	if err != nil {
		t.Fatalf("create semantic model err = %v", err)
	}
	if model.ID() != "101" || created == nil {
		t.Fatalf("model = %q/%v", model.ID(), created)
	}
	if created.GetId() != 101 || created.GetName() != "orders-model" {
		t.Fatalf("created semantic model = %#v", created)
	}
	knownModel, err := workspace.SemanticModel("101")
	if err != nil {
		t.Fatalf("known model err = %v", err)
	}
	if knownModel.ID() != "101" {
		t.Fatalf("known model id = %q", knownModel.ID())
	}
	info, err := model.Info(ctx)
	if err != nil {
		t.Fatalf("model info err = %v", err)
	}
	if info.GetId() != 101 || info.GetDescription() != "orders" {
		t.Fatalf("model info = %#v", info)
	}
	updated, err := model.Update(ctx, "orders-model", WithSemanticModelFiles([]string{"file-1"}))
	if err != nil {
		t.Fatalf("model update err = %v", err)
	}
	if !updated.GetUpdated() {
		t.Fatalf("model updated = %#v", updated)
	}
	entries, err := model.Entries(ctx, WithQueryParam("limit", "20"))
	if err != nil {
		t.Fatalf("model entries err = %v", err)
	}
	if entries.GetTotal() != 0 || len(entries.GetItems()) != 0 {
		t.Fatalf("model entries = %#v", entries)
	}
	entryInput := SemanticEntryInput{Kind: "metric", Key: "revenue", Spec: map[string]any{"expr": "sum(amount)"}, Tables: []string{"orders"}}
	entry, entryCreated, err := model.CreateEntry(ctx, entryInput)
	if err != nil {
		t.Fatalf("create entry err = %v", err)
	}
	if entry.ID() != "201" || entry.Model() != model || entryCreated == nil {
		t.Fatalf("entry = %q/%v", entry.ID(), entryCreated)
	}
	if entryCreated.GetId() != 201 || entryCreated.GetKey() != "revenue" {
		t.Fatalf("created semantic entry = %#v", entryCreated)
	}
	knownEntry, err := model.Entry("201")
	if err != nil {
		t.Fatalf("known entry err = %v", err)
	}
	if knownEntry.ID() != "201" {
		t.Fatalf("known entry id = %q", knownEntry.ID())
	}
	entryUpdated, err := entry.Update(ctx, SemanticEntryInput{Kind: "metric", Key: "margin", Spec: map[string]any{"expr": "sum(margin)"}})
	if err != nil {
		t.Fatalf("entry update err = %v", err)
	}
	if !entryUpdated.GetUpdated() {
		t.Fatalf("entry updated = %#v", entryUpdated)
	}
	imported, err := model.Import(ctx, []SemanticEntryInput{{Kind: "field_description", Key: "region", Spec: map[string]any{"column": "region"}}})
	if err != nil {
		t.Fatalf("model import err = %v", err)
	}
	if imported.GetImported() != 1 || imported.GetModelId() != 101 {
		t.Fatalf("imported = %#v", imported)
	}
	exported, err := model.Export(ctx)
	if err != nil {
		t.Fatalf("model export err = %v", err)
	}
	if exported.GetModel().GetId() != 101 || len(exported.GetEntries()) != 0 {
		t.Fatalf("exported = %#v", exported)
	}
	validated, err := model.Validate(ctx)
	if err != nil {
		t.Fatalf("model validate err = %v", err)
	}
	if !validated.GetValid() {
		t.Fatalf("validated = %#v", validated)
	}
	entryDeleted, err := entry.Delete(ctx)
	if err != nil {
		t.Fatalf("entry delete err = %v", err)
	}
	if !entryDeleted.GetDeleted() {
		t.Fatalf("entry deleted = %#v", entryDeleted)
	}
	modelDeleted, err := model.Delete(ctx)
	if err != nil {
		t.Fatalf("model delete err = %v", err)
	}
	if !modelDeleted.GetDeleted() {
		t.Fatalf("model deleted = %#v", modelDeleted)
	}
	if _, err := workspace.SemanticModel(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty model err = %v", err)
	}
	if _, _, err := model.CreateEntry(ctx, SemanticEntryInput{Key: "revenue", Spec: map[string]any{"expr": "sum(amount)"}}); !IsCode(err, ErrKindRequired.Code) {
		t.Fatalf("empty entry kind err = %v", err)
	}
	if _, _, err := model.CreateEntry(ctx, SemanticEntryInput{Kind: "metric", Spec: map[string]any{"expr": "sum(amount)"}}); !IsCode(err, ErrKeyRequired.Code) {
		t.Fatalf("empty entry key err = %v", err)
	}
	if _, _, err := model.CreateEntry(ctx, SemanticEntryInput{Kind: "metric", Key: "revenue"}); !IsCode(err, ErrSpecRequired.Code) {
		t.Fatalf("empty entry spec err = %v", err)
	}
	if _, err := entry.Update(ctx, SemanticEntryInput{Kind: "metric", Key: "margin"}); !IsCode(err, ErrSpecRequired.Code) {
		t.Fatalf("empty update entry spec err = %v", err)
	}
	if _, err := model.Import(ctx, nil); !IsCode(err, ErrEntriesRequired.Code) {
		t.Fatalf("empty import entries err = %v", err)
	}
	if _, err := model.Import(ctx, []SemanticEntryInput{{Kind: "field_description", Spec: map[string]any{"column": "region"}}}); !IsCode(err, ErrKeyRequired.Code) {
		t.Fatalf("invalid import entry err = %v", err)
	}
	if _, err := model.Entry(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty entry err = %v", err)
	}

	if len(requests) != 11 {
		t.Fatalf("request count = %d: %#v", len(requests), requests)
	}
	for i, request := range requests {
		if request.workspace != "ws-1" {
			t.Fatalf("request %d workspace = %q", i, request.workspace)
		}
	}
	expected := []struct {
		method string
		path   string
		query  string
	}{
		{http.MethodPost, "/newmoi/semantic-models", ""},
		{http.MethodGet, "/newmoi/semantic-models/101", ""},
		{http.MethodPut, "/newmoi/semantic-models/101", ""},
		{http.MethodGet, "/newmoi/semantic-models/101/entries", "limit=20"},
		{http.MethodPost, "/newmoi/semantic-models/101/entries", ""},
		{http.MethodPut, "/newmoi/semantic-models/101/entries/201", ""},
		{http.MethodPost, "/newmoi/semantic-models/101/import", ""},
		{http.MethodGet, "/newmoi/semantic-models/101/export", ""},
		{http.MethodPost, "/newmoi/semantic-models/101/validate", ""},
		{http.MethodDelete, "/newmoi/semantic-models/101/entries/201", ""},
		{http.MethodDelete, "/newmoi/semantic-models/101", ""},
	}
	for i, want := range expected {
		got := requests[i]
		if got.method != want.method || got.path != want.path || got.query != want.query {
			t.Fatalf("request %d = %#v, want %#v", i, got, want)
		}
	}
	if requests[0].body["name"] != "orders-model" || requests[0].body["description"] != "orders" {
		t.Fatalf("create body = %#v", requests[0].body)
	}
	if requests[2].body["name"] != "orders-model" {
		t.Fatalf("update body = %#v", requests[2].body)
	}
	if requests[4].body["kind"] != "metric" || requests[4].body["key"] != "revenue" {
		t.Fatalf("entry body = %#v", requests[4].body)
	}
	importEntries, ok := requests[6].body["entries"].([]any)
	if !ok || len(importEntries) != 1 {
		t.Fatalf("import body = %#v", requests[6].body)
	}
}

func TestKnowledgeListEntriesEncodesSemanticDomain(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"items": []any{}, "total": 0, "next_page_token": ""},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Knowledge("ws-1").ListEntries(context.Background(), "101", WithSemanticDomain("rag"))
	if err != nil {
		t.Fatalf("list entries err = %v", err)
	}
	if result.GetTotal() != 0 {
		t.Fatalf("result = %#v", result)
	}
	if gotQuery != "domain=rag" {
		t.Fatalf("query = %q", gotQuery)
	}
}

func TestKnowledgeCreateUpdateImportEntriesIncludeSemanticDomain(t *testing.T) {
	var requests []struct {
		method string
		path   string
		body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method string
			path   string
			body   map[string]any
		}{method: r.Method, path: r.URL.EscapedPath(), body: body})
		data := map[string]any{"updated": true}
		switch {
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/semantic-models/101/entries":
			data = map[string]any{"id": 201, "kind": "standard_qa", "key": "q1", "domain": "rag", "spec": map[string]any{"question": "GMV?"}, "created_at": 12, "updated_at": 12}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/semantic-models/101/import":
			data = map[string]any{"imported": 1, "model_id": 101}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	knowledge := client.Knowledge("ws-1")
	ctx := context.Background()
	created, err := knowledge.CreateEntry(ctx, "101", SemanticEntryInput{
		Kind: "standard_qa", Key: "q1", Domain: "rag", Spec: map[string]any{"question": "GMV?"},
	})
	if err != nil {
		t.Fatalf("create entry err = %v", err)
	}
	if created.GetDomain() != "rag" || created.GetKind() != "standard_qa" {
		t.Fatalf("created = %#v", created)
	}
	updated, err := knowledge.UpdateEntry(ctx, "101", "201", SemanticEntryInput{
		Kind: "standard_qa", Key: "q1", Domain: "rag", Spec: map[string]any{"question": "GMV total?"},
	})
	if err != nil || !updated.GetUpdated() {
		t.Fatalf("update entry result=%#v err=%v", updated, err)
	}
	imported, err := knowledge.Import(ctx, "101", []SemanticEntryInput{{
		Kind: "business_rule", Key: "paid", Domain: "rag", Spec: map[string]any{"content": "paid orders only"},
	}})
	if err != nil || imported.GetImported() != 1 {
		t.Fatalf("import result=%#v err=%v", imported, err)
	}
	if len(requests) != 3 {
		t.Fatalf("request count = %d: %#v", len(requests), requests)
	}
	if requests[0].body["domain"] != "rag" || requests[0].body["kind"] != "standard_qa" || requests[0].body["key"] != "q1" {
		t.Fatalf("create body = %#v", requests[0].body)
	}
	if requests[1].body["domain"] != "rag" || requests[1].body["kind"] != "standard_qa" {
		t.Fatalf("update body = %#v", requests[1].body)
	}
	importEntries, ok := requests[2].body["entries"].([]any)
	if !ok || len(importEntries) != 1 {
		t.Fatalf("import body = %#v", requests[2].body)
	}
	importedEntry, ok := importEntries[0].(map[string]any)
	if !ok || importedEntry["domain"] != "rag" || importedEntry["kind"] != "business_rule" {
		t.Fatalf("import entry = %#v", importEntries[0])
	}
}

func TestWorkspaceModelProviderHandleScopesBackendAndRouter(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		workspace string
		body      map[string]any
	}
	providerEndpoint := func(id, backendID int64, address string) map[string]any {
		return map[string]any{
			"id":         id,
			"backend_id": backendID,
			"address":    address,
			"status":     "ONLINE",
			"created_at": int64(1710000000),
			"updated_at": int64(1710000001),
		}
	}
	providerBackend := func(id int64, name, backendType string, models, mimeTypes []string) map[string]any {
		if models == nil {
			models = []string{}
		}
		if mimeTypes == nil {
			mimeTypes = []string{}
		}
		return map[string]any{
			"id":                         id,
			"name":                       name,
			"type":                       backendType,
			"reasoning_control_protocol": "NONE",
			"timeout_seconds":            30,
			"models":                     models,
			"supported_mime_types":       mimeTypes,
			"endpoints":                  []map[string]any{providerEndpoint(id+100, id, "https://provider.example.test")},
			"created_at":                 int64(1710000000),
			"updated_at":                 int64(1710000001),
		}
	}
	routerConfig := func(strategy string) map[string]any {
		return map[string]any{
			"strategy":              strategy,
			"health_check_interval": 30,
			"max_retries":           2,
			"session_affinity":      false,
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method    string
			path      string
			workspace string
			body      map[string]any
		}{
			method:    r.Method,
			path:      r.URL.EscapedPath(),
			workspace: r.Header.Get(headerWorkspaceID),
			body:      body,
		})
		var data any
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/llm/backends":
			data = map[string]any{"backends": []map[string]any{providerBackend(1, "openai", "OPENAI", []string{"gpt-4.1"}, nil)}}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/llm/probe-models":
			data = map[string]any{"models": []string{"gpt-4.1"}}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/llm/backends":
			data = providerBackend(1, "openai", "OPENAI", []string{"gpt-4.1"}, nil)
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/llm/backends/1":
			data = providerBackend(1, "openai", "OPENAI", []string{"gpt-4.1"}, nil)
		case r.Method == http.MethodPut && r.URL.EscapedPath() == "/newmoi/llm/backends/1":
			data = providerBackend(1, "openai-prod", "OPENAI", []string{"gpt-4.1-mini"}, nil)
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/llm/backends/1/endpoints":
			data = providerEndpoint(2, 1, "https://llm-2.example.test")
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/llm/backends/1/endpoints":
			data = map[string]any{"endpoints": []map[string]any{providerEndpoint(2, 1, "https://llm-2.example.test")}}
		case r.Method == http.MethodPut && r.URL.EscapedPath() == "/newmoi/llm/backends/1/endpoints/2/status":
			data = map[string]any{"success": true}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/llm/backends/1/probe-models":
			data = map[string]any{"models": []string{"gpt-4.1"}}
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/llm/router":
			data = routerConfig("round_robin")
		case r.Method == http.MethodPut && r.URL.EscapedPath() == "/newmoi/llm/router":
			data = routerConfig("round_robin")
		case r.Method == http.MethodDelete && r.URL.EscapedPath() == "/newmoi/llm/backends/1":
			data = map[string]any{"success": true}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/llm/setup":
			data = map[string]any{
				"backend":  providerBackend(5, "setup", "OPENAI", []string{"gpt-4.1"}, nil),
				"endpoint": providerEndpoint(6, 5, "https://setup.example.test"),
			}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/embedding/probe-models":
			data = map[string]any{"models": []string{"bge-m3"}}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/embedding/backends":
			data = providerBackend(3, "bge", "EMBEDDING", []string{"bge-m3"}, nil)
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/embedding/backends/3/probe-models":
			data = map[string]any{"models": []string{"bge-m3"}}
		case r.Method == http.MethodPut && r.URL.EscapedPath() == "/newmoi/embedding/router":
			data = routerConfig("sticky")
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/parser/backends":
			data = providerBackend(4, "doc-parser", "PARSER", nil, []string{"application/pdf"})
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/parser/router":
			data = routerConfig("single")
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/parser/backends/4":
			data = providerBackend(4, "doc-parser", "PARSER", nil, []string{"application/pdf"})
		default:
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": data,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	llm := workspace.LLMProvider()
	if llm.Kind() != "llm" {
		t.Fatalf("llm kind = %q", llm.Kind())
	}
	if _, err := llm.Backends(ctx); err != nil {
		t.Fatalf("llm backends err = %v", err)
	}
	if _, err := llm.ProbeAddress(ctx, "https://llm.example.test", WithProviderBackendAPIKey("sk")); err != nil {
		t.Fatalf("llm probe address err = %v", err)
	}
	backend, created, err := llm.CreateBackend(ctx, "openai", WithProviderBackendAddress("https://llm.example.test"), WithProviderBackendModels("gpt-4.1"))
	if err != nil {
		t.Fatalf("create llm backend err = %v", err)
	}
	if backend.ID() != "1" {
		t.Fatalf("backend id = %q", backend.ID())
	}
	if backend.Provider() != llm {
		t.Fatal("backend provider mismatch")
	}
	if created.GetId() != 1 {
		t.Fatalf("created id = %d", created.GetId())
	}
	known, err := workspace.LLMBackend("1")
	if err != nil {
		t.Fatalf("known llm backend err = %v", err)
	}
	if known.ID() != "1" {
		t.Fatalf("known backend id = %q", known.ID())
	}
	if _, err := backend.Info(ctx); err != nil {
		t.Fatalf("backend info err = %v", err)
	}
	if _, err := backend.Update(ctx, WithProviderBackendUpdateName("openai-prod"), WithProviderBackendUpdateModels("gpt-4.1-mini")); err != nil {
		t.Fatalf("backend update err = %v", err)
	}
	if _, err := backend.CreateEndpoint(ctx, "https://llm-2.example.test"); err != nil {
		t.Fatalf("endpoint create err = %v", err)
	}
	if _, err := backend.Endpoints(ctx); err != nil {
		t.Fatalf("endpoints err = %v", err)
	}
	if _, err := backend.SetEndpointStatus(ctx, "2", "disabled"); err != nil {
		t.Fatalf("endpoint status err = %v", err)
	}
	if _, err := backend.ProbeModels(ctx, WithProviderBackendAPIKey("sk")); err != nil {
		t.Fatalf("backend probe err = %v", err)
	}
	router := llm.Router()
	if router.Provider() != llm {
		t.Fatal("router provider mismatch")
	}
	if _, err := router.Info(ctx); err != nil {
		t.Fatalf("router info err = %v", err)
	}
	if _, err := router.Update(ctx, WithRouterStrategy("round_robin"), WithRouterMaxRetries(2)); err != nil {
		t.Fatalf("router update err = %v", err)
	}
	if _, err := backend.Delete(ctx); err != nil {
		t.Fatalf("backend delete err = %v", err)
	}
	setupBackend, setupResult, err := workspace.SetupLLMBackend(ctx, "setup", "https://setup.example.test", []string{"gpt-4.1"})
	if err != nil {
		t.Fatalf("setup llm err = %v", err)
	}
	if setupBackend.ID() != "5" || setupResult.GetBackend().GetId() != 5 {
		t.Fatalf("setup result = %q/%v", setupBackend.ID(), setupResult)
	}
	embedding := workspace.EmbeddingProvider()
	if _, err := embedding.ProbeAddress(ctx, "https://embedding.example.test", WithProviderBackendAPIKey("sk")); err != nil {
		t.Fatalf("embedding probe err = %v", err)
	}
	embeddingBackend, _, err := embedding.CreateBackend(ctx, "bge", WithProviderBackendModels("bge-m3"))
	if err != nil {
		t.Fatalf("embedding create err = %v", err)
	}
	if _, err := workspace.EmbeddingBackend("3"); err != nil {
		t.Fatalf("known embedding err = %v", err)
	}
	if _, err := embeddingBackend.ProbeModels(ctx, WithProviderBackendAPIKey("sk")); err != nil {
		t.Fatalf("embedding backend probe err = %v", err)
	}
	if _, err := embedding.Router().Update(ctx, WithRouterSessionAffinity(true)); err != nil {
		t.Fatalf("embedding router update err = %v", err)
	}
	parser := workspace.ParserProvider()
	parserBackend, _, err := parser.CreateBackend(ctx, "doc-parser", WithProviderBackendSupportedMimeTypes("application/pdf"))
	if err != nil {
		t.Fatalf("parser create err = %v", err)
	}
	if _, err := workspace.ParserBackend("4"); err != nil {
		t.Fatalf("known parser err = %v", err)
	}
	if _, err := parser.Router().Info(ctx); err != nil {
		t.Fatalf("parser router info err = %v", err)
	}
	if _, err := parserBackend.Info(ctx); err != nil {
		t.Fatalf("parser info err = %v", err)
	}
	if _, err := parser.ProbeAddress(ctx, "https://parser.example.test"); !IsCode(err, "ErrUnsupportedOperation") {
		t.Fatalf("parser probe err = %v", err)
	}
	if _, _, err := llm.CreateBackend(ctx, "", WithProviderBackendModels("gpt")); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("empty backend name err = %v", err)
	}
	if _, err := backend.CreateEndpoint(ctx, ""); !IsCode(err, ErrAddressRequired.Code) {
		t.Fatalf("empty endpoint address err = %v", err)
	}
	if _, err := backend.SetEndpointStatus(ctx, "", "disabled"); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty endpoint id err = %v", err)
	}
	if _, err := backend.SetEndpointStatus(ctx, "2", ""); !IsCode(err, ErrStatusRequired.Code) {
		t.Fatalf("empty endpoint status err = %v", err)
	}
	if _, err := workspace.LLMBackend(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty known backend err = %v", err)
	}
	if _, _, err := workspace.SetupLLMBackend(ctx, "", "https://setup.example.test", []string{"gpt"}); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("empty setup name err = %v", err)
	}
	if _, _, err := workspace.SetupLLMBackend(ctx, "setup", "", []string{"gpt"}); !IsCode(err, ErrAddressRequired.Code) {
		t.Fatalf("empty setup address err = %v", err)
	}
	if _, _, err := workspace.SetupLLMBackend(ctx, "setup", "https://setup.example.test", nil); !IsCode(err, ErrModelsRequired.Code) {
		t.Fatalf("empty setup models err = %v", err)
	}

	if len(requests) != 20 {
		t.Fatalf("request count = %d: %#v", len(requests), requests)
	}
	for i, request := range requests {
		if request.workspace != "ws-1" {
			t.Fatalf("request %d workspace = %q", i, request.workspace)
		}
	}
	expected := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/newmoi/llm/backends"},
		{http.MethodPost, "/newmoi/llm/probe-models"},
		{http.MethodPost, "/newmoi/llm/backends"},
		{http.MethodGet, "/newmoi/llm/backends/1"},
		{http.MethodPut, "/newmoi/llm/backends/1"},
		{http.MethodPost, "/newmoi/llm/backends/1/endpoints"},
		{http.MethodGet, "/newmoi/llm/backends/1/endpoints"},
		{http.MethodPut, "/newmoi/llm/backends/1/endpoints/2/status"},
		{http.MethodPost, "/newmoi/llm/backends/1/probe-models"},
		{http.MethodGet, "/newmoi/llm/router"},
		{http.MethodPut, "/newmoi/llm/router"},
		{http.MethodDelete, "/newmoi/llm/backends/1"},
		{http.MethodPost, "/newmoi/llm/setup"},
		{http.MethodPost, "/newmoi/embedding/probe-models"},
		{http.MethodPost, "/newmoi/embedding/backends"},
		{http.MethodPost, "/newmoi/embedding/backends/3/probe-models"},
		{http.MethodPut, "/newmoi/embedding/router"},
		{http.MethodPost, "/newmoi/parser/backends"},
		{http.MethodGet, "/newmoi/parser/router"},
		{http.MethodGet, "/newmoi/parser/backends/4"},
	}
	for i, want := range expected {
		got := requests[i]
		if got.method != want.method || got.path != want.path {
			t.Fatalf("request %d = %#v, want %#v", i, got, want)
		}
	}
	if requests[1].body["address"] != "https://llm.example.test" || requests[1].body["api_key"] != "sk" {
		t.Fatalf("llm probe body = %#v", requests[1].body)
	}
	if requests[2].body["name"] != "openai" || requests[2].body["address"] != "https://llm.example.test" {
		t.Fatalf("llm create body = %#v", requests[2].body)
	}
	if requests[4].body["name"] != "openai-prod" {
		t.Fatalf("llm update body = %#v", requests[4].body)
	}
	if requests[5].body["address"] != "https://llm-2.example.test" {
		t.Fatalf("endpoint create body = %#v", requests[5].body)
	}
	if requests[7].body["status"] != "disabled" {
		t.Fatalf("endpoint status body = %#v", requests[7].body)
	}
	if requests[8].body["api_key"] != "sk" {
		t.Fatalf("backend probe body = %#v", requests[8].body)
	}
	if requests[10].body["strategy"] != "round_robin" || requests[10].body["max_retries"] != float64(2) {
		t.Fatalf("router update body = %#v", requests[10].body)
	}
	if requests[12].body["name"] != "setup" || requests[12].body["address"] != "https://setup.example.test" {
		t.Fatalf("setup body = %#v", requests[12].body)
	}
	if requests[13].body["address"] != "https://embedding.example.test" {
		t.Fatalf("embedding probe body = %#v", requests[13].body)
	}
	if requests[16].body["session_affinity"] != true {
		t.Fatalf("embedding router body = %#v", requests[16].body)
	}
	if requests[17].body["name"] != "doc-parser" {
		t.Fatalf("parser create body = %#v", requests[17].body)
	}
}

func TestWorkspaceComputeResourceHandleScopesLifecycle(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		query     string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method    string
			path      string
			query     string
			workspace string
			body      map[string]any
		}{
			method:    r.Method,
			path:      r.URL.EscapedPath(),
			query:     r.URL.RawQuery,
			workspace: r.Header.Get(headerWorkspaceID),
			body:      body,
		})
		data := map[string]any{
			"id":                     "compute-1",
			"workspace_id":           "ws-1",
			"name":                   "go-worker",
			"description":            "etl",
			"spec_id":                "spec-1",
			"cpu":                    2,
			"memory_gib":             8,
			"gpu":                    0,
			"min_replicas":           1,
			"max_replicas":           3,
			"current_replicas":       1,
			"go_worker_image_id":     "go-img-1",
			"python_worker_image_id": "py-img-1",
			"platform":               "linux/amd64",
			"auto_suspend_minutes":   0,
			"is_default":             false,
			"status":                 "running",
			"status_message":         "",
			"created_by":             "user-1",
			"created_at":             "2026-01-01T00:00:00Z",
			"updated_at":             "2026-01-01T00:00:00Z",
		}
		switch r.URL.EscapedPath() {
		case "/newmoi/compute-resources":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": []any{data}})
				return
			}
		case "/newmoi/compute-resources/worker-images":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": "OK",
				"msg":  "OK",
				"data": []any{map[string]any{"id": "go-img-1", "worker_type": "go-worker", "image_key": "go-worker", "type": "go-worker", "tag": "v1", "repository": "repo/go-worker", "platform": "linux/amd64", "description": "go worker", "is_default": true, "status": "active", "registered_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"}},
			})
			return
		case "/newmoi/compute-resources/compute-1/preflight-delete":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": "OK",
				"msg":  "OK",
				"data": map[string]any{"active_tasks": 1, "active_task_ids": []string{"task-1"}, "bound_workflows": 1, "bound_workflow_details": []any{map[string]any{"id": "wf-1", "name": "workflow", "node_names": []string{"node-1"}}}},
			})
			return
		case "/newmoi/compute-resources/compute-1/adopt-legacy-ownership":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": map[string]any{"workspace_id": "ws-1", "resource_type": "compute_resource", "resource_id": "compute-1", "owner_role_id": "role-superadmin", "authorized_role_id": "role-superadmin", "status": "active", "ownership_version": 1}})
			return
		}
		if strings.HasSuffix(r.URL.EscapedPath(), "/suspend") {
			data["status"] = "suspended"
		}
		if strings.HasSuffix(r.URL.EscapedPath(), "/resume") {
			data["status"] = "IDLE"
		}
		if strings.HasSuffix(r.URL.EscapedPath(), "/retry") {
			data["status"] = "retrying"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	workerImages := []*WorkerImageSelection{
		{WorkerType: "go-worker", ImageId: "go-img-1", Platform: "linux/amd64"},
		{WorkerType: "python-worker", ImageId: "python-img-1", Platform: "linux/amd64"},
		{WorkerType: "java-worker", ImageId: "java-img-1", Platform: "linux/amd64"},
		{WorkerType: "custom-tool-worker", ImageId: "tool-img-1", Platform: "linux/amd64"},
	}
	resource, created, err := workspace.CreateComputeResource(ctx, "go-worker", "spec-1", 2, 8, WithComputeDescription("etl"), WithComputeMinReplicas(1), WithComputeMaxReplicas(3), WithComputeWorkerImages(workerImages...))
	if err != nil {
		t.Fatalf("create compute err = %v", err)
	}
	if resource.ID() != "compute-1" {
		t.Fatalf("compute id = %q", resource.ID())
	}
	if created.GetId() != "compute-1" || created.GetCpu() != 2 || created.GetMemoryGib() != 8 {
		t.Fatalf("created compute = %+v", created)
	}
	resources, err := workspace.Compute().List(ctx)
	if err != nil {
		t.Fatalf("list err = %v", err)
	}
	if len(resources) != 1 || resources[0].GetId() != "compute-1" {
		t.Fatalf("resources = %+v", resources)
	}
	images, err := workspace.Compute().WorkerImages(ctx, WithQueryParam("type", "go-worker"))
	if err != nil {
		t.Fatalf("worker images err = %v", err)
	}
	if len(images) != 1 || images[0].GetId() != "go-img-1" || images[0].GetType() != "go-worker" {
		t.Fatalf("images = %+v", images)
	}
	known, err := workspace.ComputeResource("compute-1")
	if err != nil {
		t.Fatalf("known compute err = %v", err)
	}
	if known.ID() != "compute-1" {
		t.Fatalf("known compute id = %q", known.ID())
	}
	info, err := resource.Info(ctx)
	if err != nil {
		t.Fatalf("info err = %v", err)
	}
	if info.GetWorkspaceId() != "ws-1" || info.GetPlatform() != "linux/amd64" {
		t.Fatalf("info = %+v", info)
	}
	updated, err := resource.Update(ctx, WithComputeUpdateName("go-worker-prod"), WithComputeUpdateCPU(4), WithComputeUpdateMemoryGiB(16), WithComputeUpdateAutoSuspendMinutes(20), WithComputeUpdateWorkerImages(workerImages...))
	if err != nil {
		t.Fatalf("update err = %v", err)
	}
	if updated.GetId() != "compute-1" {
		t.Fatalf("updated = %+v", updated)
	}
	ownership, err := resource.AdoptLegacyOwnership(ctx)
	if err != nil {
		t.Fatalf("adopt legacy ownership err = %v", err)
	}
	if ownership.GetResourceId() != "compute-1" || ownership.GetOwnerRoleId() != "role-superadmin" || ownership.GetOwnershipVersion() != 1 {
		t.Fatalf("ownership = %+v", ownership)
	}
	suspended, err := resource.Suspend(ctx)
	if err != nil {
		t.Fatalf("suspend err = %v", err)
	}
	if suspended.GetStatus() != "suspended" {
		t.Fatalf("suspended = %+v", suspended)
	}
	resumed, err := resource.Resume(ctx)
	if err != nil {
		t.Fatalf("resume err = %v", err)
	}
	if resumed.GetStatus() != "IDLE" {
		t.Fatalf("resumed = %+v", resumed)
	}
	retried, err := resource.Retry(ctx)
	if err != nil {
		t.Fatalf("retry err = %v", err)
	}
	if retried.GetStatus() != "retrying" {
		t.Fatalf("retried = %+v", retried)
	}
	preflight, err := resource.PreflightDelete(ctx)
	if err != nil {
		t.Fatalf("preflight err = %v", err)
	}
	if preflight.GetActiveTasks() != 1 || preflight.GetBoundWorkflows() != 1 || preflight.GetBoundWorkflowDetails()[0].GetNodeNames()[0] != "node-1" {
		t.Fatalf("preflight = %+v", preflight)
	}
	if err := resource.Delete(ctx, WithForce(true)); err != nil {
		t.Fatalf("delete err = %v", err)
	}
	if _, err := workspace.ComputeResource(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty compute err = %v", err)
	}
	if _, _, err := workspace.CreateComputeResource(ctx, "", "spec-1", 1, 1); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("empty create compute name err = %v", err)
	}

	if len(requests) != 11 {
		t.Fatalf("request count = %d: %#v", len(requests), requests)
	}
	for i, request := range requests {
		if request.workspace != "ws-1" {
			t.Fatalf("request %d workspace = %q", i, request.workspace)
		}
	}
	expected := []struct {
		method string
		path   string
		query  string
	}{
		{http.MethodPost, "/newmoi/compute-resources", ""},
		{http.MethodGet, "/newmoi/compute-resources", ""},
		{http.MethodGet, "/newmoi/compute-resources/worker-images", "type=go-worker"},
		{http.MethodGet, "/newmoi/compute-resources/compute-1", ""},
		{http.MethodPut, "/newmoi/compute-resources/compute-1", ""},
		{http.MethodPost, "/newmoi/compute-resources/compute-1/adopt-legacy-ownership", ""},
		{http.MethodPost, "/newmoi/compute-resources/compute-1/suspend", ""},
		{http.MethodPost, "/newmoi/compute-resources/compute-1/resume", ""},
		{http.MethodPost, "/newmoi/compute-resources/compute-1/retry", ""},
		{http.MethodGet, "/newmoi/compute-resources/compute-1/preflight-delete", ""},
		{http.MethodDelete, "/newmoi/compute-resources/compute-1", "force=true"},
	}
	for i, want := range expected {
		got := requests[i]
		if got.method != want.method || got.path != want.path || got.query != want.query {
			t.Fatalf("request %d = %#v, want %#v", i, got, want)
		}
	}
	if requests[0].body["name"] != "go-worker" || requests[0].body["spec_id"] != "spec-1" || requests[0].body["cpu"] != float64(2) || requests[0].body["memory_gib"] != float64(8) || requests[0].body["auto_suspend_minutes"] != float64(10) {
		t.Fatalf("create body = %#v", requests[0].body)
	}
	if requests[0].body["description"] != "etl" || requests[0].body["min_replicas"] != float64(1) || requests[0].body["max_replicas"] != float64(3) {
		t.Fatalf("create option body = %#v", requests[0].body)
	}
	if images, ok := requests[0].body["worker_images"].([]any); !ok || len(images) != 4 {
		t.Fatalf("create worker_images = %#v", requests[0].body["worker_images"])
	} else if first, ok := images[0].(map[string]any); !ok || first["worker_type"] != "go-worker" || first["kind"] != nil {
		t.Fatalf("create worker image contract = %#v", images[0])
	}
	if requests[4].body["name"] != "go-worker-prod" || requests[4].body["cpu"] != float64(4) || requests[4].body["memory_gib"] != float64(16) || requests[4].body["auto_suspend_minutes"] != float64(20) {
		t.Fatalf("update body = %#v", requests[4].body)
	}
	if images, ok := requests[4].body["worker_images"].([]any); !ok || len(images) != 4 {
		t.Fatalf("update worker_images = %#v", requests[4].body["worker_images"])
	} else if first, ok := images[0].(map[string]any); !ok || first["worker_type"] != "go-worker" || first["kind"] != nil {
		t.Fatalf("update worker image contract = %#v", images[0])
	}
}

func TestWorkspaceDataShareHandleScopesPublishAndSubscription(t *testing.T) {
	var requests []struct {
		method    string
		path      string
		query     string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			method    string
			path      string
			query     string
			workspace string
			body      map[string]any
		}{
			method:    r.Method,
			path:      r.URL.EscapedPath(),
			query:     r.URL.RawQuery,
			workspace: r.Header.Get(headerWorkspaceID),
			body:      body,
		})
		data := map[string]any{}
		if r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/data-share/publishes" {
			data = map[string]any{"list": []any{map[string]any{"id": "publish-1", "name": "orders-share", "source_database_id": "101", "mo_database_name": "orders", "table_scope": map[string]any{"mode": "selected", "object_ids": []any{"201"}}, "obj_display_name": "Orders", "obj_display_path": "Catalog/Orders", "targets": []any{map[string]any{"workspace_id": "ws-2", "workspace_name": "Target"}}, "permission": "owner", "remark": "daily", "created_by": "user-1", "created_at": "2026-06-01T00:00:00Z", "updated_at": "2026-06-01T00:00:00Z"}}, "total": 1, "page": 1, "page_size": 20}
		} else if r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/data-share/workspace-names" {
			data = map[string]any{"workspaces": []any{map[string]any{"id": "ws-2", "name": "Target"}}}
		} else if r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/data-share/workspaces/resolve-names" {
			data = map[string]any{"workspaces": []any{map[string]any{"id": "cross-user-ws", "name": "Cross User Workspace"}}}
		} else if r.URL.EscapedPath() == "/newmoi/data-share/publishes/summary" {
			data = map[string]any{"total": 2, "target_total": 5}
		} else if r.URL.EscapedPath() == "/newmoi/data-share/source-databases/101/tables" {
			data = map[string]any{"list": []any{map[string]any{"id": "201", "name": "orders"}}, "total": 1, "page": 1, "page_size": 20}
		} else if r.URL.EscapedPath() == "/newmoi/data-share/publishes/check-name" {
			data = map[string]any{"available": true}
		} else if r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/data-share/publishes" {
			data = map[string]any{"id": "publish-1", "name": "orders-share", "source_database_id": "101", "mo_database_name": "orders", "table_scope": map[string]any{"mode": "selected", "object_ids": []any{"201"}}, "obj_display_name": "Orders", "obj_display_path": "Catalog/Orders", "targets": []any{map[string]any{"workspace_id": "ws-2", "workspace_name": "Target"}}, "permission": "owner", "remark": "daily", "created_by": "user-1", "created_at": "2026-06-01T00:00:00Z", "updated_at": "2026-06-01T00:00:00Z"}
		} else if r.Method == http.MethodPut && r.URL.EscapedPath() == "/newmoi/data-share/publishes/publish-1" {
			data = map[string]any{"id": "publish-1", "name": "orders-share", "source_database_id": "101", "mo_database_name": "orders", "table_scope": map[string]any{"mode": "selected", "object_ids": []any{"201", "202"}}, "targets": []any{map[string]any{"workspace_id": "ws-2"}, map[string]any{"workspace_id": "ws-3"}}, "permission": "owner", "remark": "expanded", "created_by": "user-1", "created_at": "2026-06-01T00:00:00Z", "updated_at": "2026-06-02T00:00:00Z"}
		} else if r.Method == http.MethodGet && r.URL.EscapedPath() == "/newmoi/data-share/subscriptions" {
			data = map[string]any{"list": []any{map[string]any{"id": "sub-1", "pub_name": "orders-share", "source_database_id": "101", "mo_database_name": "orders", "table_scope": map[string]any{"mode": "selected", "object_ids": []any{"201"}}, "obj_display_name": "Orders", "obj_display_path": "Catalog/Orders", "source_workspace_id": "ws-source", "source_workspace_name": "Source", "publisher": "alice", "published_at": "2026-06-01T00:00:00Z", "status": "unsubscribed", "created_at": "2026-06-01T00:00:00Z"}}, "total": 1, "page": 1, "page_size": 20}
		} else if r.Method == http.MethodPost && r.URL.EscapedPath() == "/newmoi/data-share/subscriptions/sub-1/subscribe" {
			data = map[string]any{"id": "sub-1", "pub_name": "orders-share", "source_database_id": "101", "mo_database_name": "orders", "table_scope": map[string]any{"mode": "selected", "object_ids": []any{"201"}}, "source_workspace_id": "ws-source", "source_workspace_name": "Source", "publisher": "alice", "published_at": "2026-06-01T00:00:00Z", "status": "subscribed", "sub_name": "orders-copy", "target_database_id": "301", "subscribed_by": "user-1", "created_at": "2026-06-01T00:00:00Z"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": data,
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	publishes, err := workspace.DataShare().Publishes(ctx, WithQueryParam("name", "orders-share"))
	if err != nil {
		t.Fatalf("publishes err = %v", err)
	}
	if publishes.GetTotal() != 1 || publishes.GetList()[0].GetId() != "publish-1" || publishes.GetList()[0].GetObjDisplayName() != "Orders" || publishes.GetList()[0].GetObjDisplayPath() != "Catalog/Orders" || publishes.GetList()[0].GetTargets()[0].GetWorkspaceId() != "ws-2" {
		t.Fatalf("publishes = %#v", publishes)
	}
	workspaceNames, err := workspace.DataShare().WorkspaceNamesResult(ctx, []string{"ws-2"})
	if err != nil {
		t.Fatalf("workspace names err = %v", err)
	}
	if len(workspaceNames.GetWorkspaces()) != 1 || workspaceNames.GetWorkspaces()[0].GetName() != "Target" {
		t.Fatalf("workspace names = %#v", workspaceNames)
	}
	resolvedWorkspaceNames, err := workspace.DataShare().ResolveWorkspaceNames(ctx, []string{"cross-user-ws"})
	if err != nil {
		t.Fatalf("resolve workspace names err = %v", err)
	}
	if len(resolvedWorkspaceNames.GetWorkspaces()) != 1 || resolvedWorkspaceNames.GetWorkspaces()[0].GetName() != "Cross User Workspace" {
		t.Fatalf("resolved workspace names = %#v", resolvedWorkspaceNames)
	}
	summary, err := workspace.DataShareSummary(ctx)
	if err != nil {
		t.Fatalf("publish summary err = %v", err)
	}
	if summary.GetTotal() != 2 || summary.GetTargetTotal() != 5 {
		t.Fatalf("publish summary = %#v", summary)
	}
	sourceTables, err := workspace.DataShareSourceTables(ctx, "101")
	if err != nil {
		t.Fatalf("source tables err = %v", err)
	}
	if sourceTables.GetTotal() != 1 || sourceTables.GetList()[0].GetId() != "201" || sourceTables.GetList()[0].GetName() != "orders" {
		t.Fatalf("source tables = %#v", sourceTables)
	}
	checkedName, err := workspace.DataShare().CheckPublishName(ctx, WithQueryParam("name", "orders-share"))
	if err != nil {
		t.Fatalf("check publish name err = %v", err)
	}
	if !checkedName.GetAvailable() {
		t.Fatalf("check publish name = %#v", checkedName)
	}
	tableScope := &DataShareScope{Mode: "selected", ObjectIds: []string{"201"}}
	publish, created, err := workspace.CreateDataSharePublish(ctx, "orders-share", "101", tableScope, []string{"ws-2"}, WithDataShareRemark("daily"))
	if err != nil {
		t.Fatalf("create publish err = %v", err)
	}
	if publish.ID() != "publish-1" {
		t.Fatalf("publish id = %q", publish.ID())
	}
	if created == nil {
		t.Fatal("missing publish create result")
	}
	if created.GetId() != "publish-1" || created.GetName() != "orders-share" || created.GetSourceDatabaseId() != "101" || created.GetMoDatabaseName() != "orders" || created.GetTableScope().GetObjectIds()[0] != "201" || created.GetObjDisplayName() != "Orders" || created.GetObjDisplayPath() != "Catalog/Orders" || created.GetTargets()[0].GetWorkspaceId() != "ws-2" {
		t.Fatalf("created publish = %#v", created)
	}
	knownPublish, err := workspace.DataSharePublish("publish-1")
	if err != nil {
		t.Fatalf("known publish err = %v", err)
	}
	if knownPublish.ID() != "publish-1" {
		t.Fatalf("known publish id = %q", knownPublish.ID())
	}
	updated, err := publish.Update(ctx, WithDataShareUpdateTableScope(&DataShareScope{Mode: "selected", ObjectIds: []string{"201", "202"}}), WithDataShareUpdateTargetWorkspaceIDs("ws-2", "ws-3"), WithDataShareUpdateRemark("expanded"))
	if err != nil {
		t.Fatalf("update publish err = %v", err)
	}
	if updated.GetRemark() != "expanded" || len(updated.GetTargets()) != 2 {
		t.Fatalf("updated publish = %#v", updated)
	}
	subscriptions, err := workspace.DataShare().Subscriptions(ctx, WithQueryParam("status", "pending"))
	if err != nil {
		t.Fatalf("subscriptions err = %v", err)
	}
	if subscriptions.GetTotal() != 1 || subscriptions.GetList()[0].GetId() != "sub-1" || subscriptions.GetList()[0].GetObjDisplayName() != "Orders" || subscriptions.GetList()[0].GetObjDisplayPath() != "Catalog/Orders" {
		t.Fatalf("subscriptions = %#v", subscriptions)
	}
	subscription, err := workspace.DataShareSubscription("sub-1")
	if err != nil {
		t.Fatalf("subscription err = %v", err)
	}
	if subscription.ID() != "sub-1" {
		t.Fatalf("subscription id = %q", subscription.ID())
	}
	subscribed, err := subscription.Subscribe(ctx, "orders-copy")
	if err != nil {
		t.Fatalf("subscribe err = %v", err)
	}
	if subscribed.GetId() != "sub-1" || subscribed.GetStatus() != "subscribed" || subscribed.GetTargetDatabaseId() != "301" {
		t.Fatalf("subscribed = %#v", subscribed)
	}
	if err := subscription.Unsubscribe(ctx); err != nil {
		t.Fatalf("unsubscribe err = %v", err)
	}
	if err := publish.Delete(ctx); err != nil {
		t.Fatalf("delete publish err = %v", err)
	}
	if _, err := workspace.DataSharePublish(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty publish err = %v", err)
	}
	if _, _, err := workspace.CreateDataSharePublish(ctx, "", "101", tableScope, []string{"ws-2"}); !IsCode(err, ErrNameRequired.Code) {
		t.Fatalf("empty create publish name err = %v", err)
	}
	if _, err := workspace.DataShareSubscription(""); !IsCode(err, ErrIDRequired.Code) {
		t.Fatalf("empty subscription err = %v", err)
	}

	if len(requests) != 12 {
		t.Fatalf("request count = %d: %#v", len(requests), requests)
	}
	for i, request := range requests {
		if request.workspace != "ws-1" {
			t.Fatalf("request %d workspace = %q", i, request.workspace)
		}
	}
	expected := []struct {
		method string
		path   string
		query  string
	}{
		{http.MethodGet, "/newmoi/data-share/publishes", "name=orders-share"},
		{http.MethodPost, "/newmoi/data-share/workspace-names", ""},
		{http.MethodPost, "/newmoi/data-share/workspaces/resolve-names", ""},
		{http.MethodGet, "/newmoi/data-share/publishes/summary", ""},
		{http.MethodGet, "/newmoi/data-share/source-databases/101/tables", ""},
		{http.MethodGet, "/newmoi/data-share/publishes/check-name", "name=orders-share"},
		{http.MethodPost, "/newmoi/data-share/publishes", ""},
		{http.MethodPut, "/newmoi/data-share/publishes/publish-1", ""},
		{http.MethodGet, "/newmoi/data-share/subscriptions", "status=pending"},
		{http.MethodPost, "/newmoi/data-share/subscriptions/sub-1/subscribe", ""},
		{http.MethodPost, "/newmoi/data-share/subscriptions/sub-1/unsubscribe", ""},
		{http.MethodDelete, "/newmoi/data-share/publishes/publish-1", ""},
	}
	for i, want := range expected {
		got := requests[i]
		if got.method != want.method || got.path != want.path || got.query != want.query {
			t.Fatalf("request %d = %#v, want %#v", i, got, want)
		}
	}
	ids, ok := requests[1].body["ids"].([]any)
	if !ok || len(ids) != 1 || ids[0] != "ws-2" {
		t.Fatalf("workspace names body = %#v", requests[1].body)
	}
	resolvedIDs, ok := requests[2].body["ids"].([]any)
	if !ok || len(resolvedIDs) != 1 || resolvedIDs[0] != "cross-user-ws" {
		t.Fatalf("resolve workspace names body = %#v", requests[2].body)
	}
	if requests[6].body["name"] != "orders-share" || requests[6].body["source_database_id"] != "101" || requests[6].body["remark"] != "daily" {
		t.Fatalf("create publish body = %#v", requests[6].body)
	}
	if scope, ok := requests[6].body["table_scope"].(map[string]any); !ok || scope["mode"] != "selected" || len(scope["object_ids"].([]any)) != 1 {
		t.Fatalf("create publish table scope = %#v", requests[6].body["table_scope"])
	}
	for _, oldField := range []string{"obj_id", "obj_name", "obj_type", "obj_path", "tables"} {
		if _, exists := requests[6].body[oldField]; exists {
			t.Fatalf("create publish contains removed field %q: %#v", oldField, requests[6].body)
		}
	}
	if requests[7].body["remark"] != "expanded" {
		t.Fatalf("update publish body = %#v", requests[7].body)
	}
	if requests[9].body["sub_name"] != "orders-copy" {
		t.Fatalf("subscribe body = %#v", requests[9].body)
	}
	if _, exists := requests[9].body["location_id"]; exists {
		t.Fatalf("subscribe contains removed location_id: %#v", requests[9].body)
	}
}

func TestWorkspaceSQLHandlesScopeWorkbookQueryAndStatement(t *testing.T) {
	var requests []struct {
		path      string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path      string
			workspace string
			body      map[string]any
		}{path: r.URL.EscapedPath(), workspace: r.Header.Get(headerWorkspaceID), body: body})
		if r.URL.EscapedPath() == "/newmoi/query/result/download" {
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte("id\n1\n"))
			return
		}
		data := any(nil)
		switch r.URL.EscapedPath() {
		case "/newmoi/workbook/create":
			data = map[string]any{"workbook_id": "wb-1", "name": "analysis"}
		case "/newmoi/workbook/version/list":
			data = map[string]any{"total": 1, "result": []any{map[string]any{"id": "ver-1", "name": "v1", "status": "draft"}}}
		case "/newmoi/workbook/version/create":
			data = map[string]any{"id": "ver-1", "version": "v1", "status": "draft"}
		case "/newmoi/workbook/detail":
			data = map[string]any{"sql_content": "select 1", "created_at": "2026-06-01T00:00:00Z", "updated_at": "2026-06-01T00:00:00Z"}
		case "/newmoi/query/execute":
			data = map[string]any{"id": "query-1", "query_id": "query-1"}
		case "/newmoi/query/describe":
			data = map[string]any{"query_id": "query-1", "created_at": "2026-06-01T00:00:00Z", "statement_id": "stmt-1", "db_name": "db1", "status": "running"}
		case "/newmoi/query/result":
			data = map[string]any{"statement_id": "stmt-1", "status": "success", "offset": 0, "limit": 50, "total": 0, "result": []any{}}
		case "/newmoi/query/detail":
			data = map[string]any{"statement_id": "stmt-1", "status": "Success"}
		case "/newmoi/query/profile":
			data = map[string]any{"statement_id": "stmt-1", "profile": map[string]any{"code": 200, "message": "OK", "success": true}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	workbook, created, err := workspace.CreateSQLWorkbook(ctx, "analysis")
	if err != nil {
		t.Fatalf("create workbook err = %v", err)
	}
	if workbook.ID() != "wb-1" || created == nil {
		t.Fatalf("workbook = %q/%v", workbook.ID(), created)
	}
	knownWorkbook, err := workspace.SQLWorkbook("wb-1")
	if err != nil {
		t.Fatalf("known workbook err = %v", err)
	}
	if knownWorkbook.ID() != "wb-1" {
		t.Fatalf("known workbook id = %q", knownWorkbook.ID())
	}
	if err := workbook.Rename(ctx, "analysis v2"); err != nil {
		t.Fatalf("rename err = %v", err)
	}
	if _, err := workbook.Versions(ctx, WithWorkbookVersionPageSize(10)); err != nil {
		t.Fatalf("versions err = %v", err)
	}
	version, versionCreated, err := workbook.CreateVersion(ctx, "select 1")
	if err != nil {
		t.Fatalf("create version err = %v", err)
	}
	if version.ID() != "ver-1" || version.Workbook() != workbook || versionCreated == nil {
		t.Fatalf("version = %q/%v", version.ID(), versionCreated)
	}
	knownVersion, err := workbook.Version("ver-1")
	if err != nil {
		t.Fatalf("known version err = %v", err)
	}
	if knownVersion.ID() != "ver-1" {
		t.Fatalf("known version id = %q", knownVersion.ID())
	}
	if _, err := version.Info(ctx); err != nil {
		t.Fatalf("version info err = %v", err)
	}
	if err := version.Update(ctx, "select 2"); err != nil {
		t.Fatalf("version update err = %v", err)
	}
	sqlContent := "select 3"
	if err := version.Save(ctx, sqlContent); err != nil {
		t.Fatalf("version save err = %v", err)
	}
	query, queryStarted, err := workspace.ExecuteSQL(ctx, "select 1", WithSQLQueryDBName("db1"), WithSQLQueryLimit(100))
	if err != nil {
		t.Fatalf("execute sql err = %v", err)
	}
	if query.ID() != "query-1" || queryStarted == nil {
		t.Fatalf("query = %q/%v", query.ID(), queryStarted)
	}
	knownQuery, err := workspace.SQLQuery("query-1")
	if err != nil {
		t.Fatalf("known query err = %v", err)
	}
	if knownQuery.ID() != "query-1" {
		t.Fatalf("known query id = %q", knownQuery.ID())
	}
	if _, err := query.Describe(ctx); err != nil {
		t.Fatalf("describe err = %v", err)
	}
	if err := query.Kill(ctx); err != nil {
		t.Fatalf("kill err = %v", err)
	}
	statement, err := workspace.SQLStatement("stmt-1")
	if err != nil {
		t.Fatalf("statement err = %v", err)
	}
	if statement.ID() != "stmt-1" {
		t.Fatalf("statement id = %q", statement.ID())
	}
	if _, err := statement.Result(ctx, WithSQLResultLimit(50)); err != nil {
		t.Fatalf("result err = %v", err)
	}
	if _, err := statement.Detail(ctx, WithQueryRecordStart("2026-06-01 00:00:00")); err != nil {
		t.Fatalf("detail err = %v", err)
	}
	if _, err := statement.Profile(ctx, WithQueryRecordEnd("2026-06-02 00:00:00")); err != nil {
		t.Fatalf("profile err = %v", err)
	}
	if err := workbook.Delete(ctx); err != nil {
		t.Fatalf("workbook delete err = %v", err)
	}
	if _, err := workspace.SQLWorkbook(""); !IsCode(err, ErrWorkbookIDRequired.Code) {
		t.Fatalf("empty workbook err = %v", err)
	}
	if _, err := workbook.Version(""); !IsCode(err, ErrVersionIDRequired.Code) {
		t.Fatalf("empty version err = %v", err)
	}
	if _, err := workspace.SQLQuery(""); !IsCode(err, ErrQueryIDRequired.Code) {
		t.Fatalf("empty query err = %v", err)
	}
	if _, err := workspace.SQLStatement(""); !IsCode(err, ErrStatementIDRequired.Code) {
		t.Fatalf("empty statement err = %v", err)
	}

	if len(requests) != 14 {
		t.Fatalf("request count = %d: %#v", len(requests), requests)
	}
	for i, request := range requests {
		if request.workspace != "ws-1" {
			t.Fatalf("request %d workspace = %q", i, request.workspace)
		}
	}
	expectedPaths := []string{
		"/newmoi/workbook/create",
		"/newmoi/workbook/update",
		"/newmoi/workbook/version/list",
		"/newmoi/workbook/version/create",
		"/newmoi/workbook/detail",
		"/newmoi/workbook/version/update",
		"/newmoi/workbook/version/save",
		"/newmoi/query/execute",
		"/newmoi/query/describe",
		"/newmoi/query/kill",
		"/newmoi/query/result",
		"/newmoi/query/detail",
		"/newmoi/query/profile",
		"/newmoi/workbook/delete",
	}
	for i, path := range expectedPaths {
		if requests[i].path != path {
			t.Fatalf("request %d path = %s, want %s", i, requests[i].path, path)
		}
	}
	if requests[0].body["name"] != "analysis" {
		t.Fatalf("create workbook body = %#v", requests[0].body)
	}
	if requests[1].body["workbook_id"] != "wb-1" || requests[1].body["name"] != "analysis v2" {
		t.Fatalf("rename body = %#v", requests[1].body)
	}
	if requests[2].body["workbook_id"] != "wb-1" || requests[2].body["page_size"] != float64(10) {
		t.Fatalf("versions body = %#v", requests[2].body)
	}
	if requests[3].body["workbook_id"] != "wb-1" || requests[3].body["sql_content"] != "select 1" {
		t.Fatalf("create version body = %#v", requests[3].body)
	}
	if requests[4].body["workbook_id"] != "wb-1" || requests[4].body["version_id"] != "ver-1" {
		t.Fatalf("version info body = %#v", requests[4].body)
	}
	if requests[5].body["sql_content"] != "select 2" || requests[6].body["sql_content"] != "select 3" {
		t.Fatalf("version update/save body = %#v/%#v", requests[5].body, requests[6].body)
	}
	if requests[7].body["query"] != "select 1" || requests[7].body["db_name"] != "db1" || requests[7].body["limit"] != float64(100) {
		t.Fatalf("execute body = %#v", requests[7].body)
	}
	if requests[8].body["query_id"] != "query-1" || requests[9].body["query_id"] != "query-1" {
		t.Fatalf("query body = %#v/%#v", requests[8].body, requests[9].body)
	}
	if requests[10].body["statement_id"] != "stmt-1" || requests[10].body["limit"] != float64(50) {
		t.Fatalf("statement result body = %#v", requests[10].body)
	}
	if requests[11].body["statement_id"] != "stmt-1" || requests[11].body["start"] != "2026-06-01 00:00:00" {
		t.Fatalf("statement detail body = %#v", requests[11].body)
	}
	if requests[12].body["statement_id"] != "stmt-1" || requests[12].body["end"] != "2026-06-02 00:00:00" {
		t.Fatalf("statement profile body = %#v", requests[12].body)
	}
	if requests[13].body["workbook_id"] != "wb-1" {
		t.Fatalf("delete workbook body = %#v", requests[13].body)
	}
}

func TestWorkspaceSQLMetadataHandleScopesDatabaseAndTable(t *testing.T) {
	var requests []struct {
		path      string
		workspace string
		body      map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path      string
			workspace string
			body      map[string]any
		}{path: r.URL.EscapedPath(), workspace: r.Header.Get(headerWorkspaceID), body: body})
		data := any(nil)
		switch r.URL.EscapedPath() {
		case "/newmoi/meta/db/info":
			data = map[string]any{"total": 1, "result": []any{map[string]any{"db_name": "db1", "db_owner": "u"}}}
		case "/newmoi/meta/db/tree":
			data = map[string]any{"db_list": []any{map[string]any{"db_name": "db1", "db_owner": "u"}}}
		case "/newmoi/meta/db/table":
			data = map[string]any{"db_name": "db1", "tables": map[string]any{"tables": []any{map[string]any{"name": "orders"}}, "total": 1}, "views": map[string]any{"tables": []any{map[string]any{"name": "orders_view"}}, "total": 1}}
		case "/newmoi/meta/table/info":
			data = map[string]any{"columns": []any{map[string]any{"name": "id", "data_type": "INT"}}, "total": 1, "rows": 10}
		case "/newmoi/meta/table/column", "/newmoi/meta/table/column_stat":
			data = map[string]any{"columns": []any{map[string]any{"name": "id", "data_type": "INT"}}, "total": 1}
		case "/newmoi/meta/table/ddl":
			data = map[string]any{"create_sql": "CREATE TABLE orders (id INT)"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": data})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	metadata := workspace.SQLMetadata()
	if _, err := metadata.Databases(ctx); err != nil {
		t.Fatalf("databases err = %v", err)
	}
	if _, err := metadata.Tree(ctx); err != nil {
		t.Fatalf("tree err = %v", err)
	}
	database, err := metadata.Database("db1")
	if err != nil {
		t.Fatalf("database err = %v", err)
	}
	if database.Name() != "db1" {
		t.Fatalf("database name = %q", database.Name())
	}
	if _, err := database.Tables(ctx); err != nil {
		t.Fatalf("tables err = %v", err)
	}
	table, err := database.Table("orders")
	if err != nil {
		t.Fatalf("table err = %v", err)
	}
	if table.Name() != "orders" {
		t.Fatalf("table name = %q", table.Name())
	}
	if _, err := table.Info(ctx); err != nil {
		t.Fatalf("info err = %v", err)
	}
	if _, err := table.Columns(ctx); err != nil {
		t.Fatalf("columns err = %v", err)
	}
	if _, err := table.ColumnStats(ctx); err != nil {
		t.Fatalf("stats err = %v", err)
	}
	if _, err := table.DDL(ctx); err != nil {
		t.Fatalf("ddl err = %v", err)
	}
	if err := table.DropTable(ctx); err != nil {
		t.Fatalf("drop table err = %v", err)
	}
	view, err := database.Table("orders_view")
	if err != nil {
		t.Fatalf("view err = %v", err)
	}
	if err := view.DropView(ctx); err != nil {
		t.Fatalf("drop view err = %v", err)
	}
	if err := database.Drop(ctx); err != nil {
		t.Fatalf("drop database err = %v", err)
	}
	if _, err := metadata.Database(""); !IsCode(err, ErrDatabaseRequired.Code) {
		t.Fatalf("empty database err = %v", err)
	}
	if _, err := database.Table(""); !IsCode(err, ErrTableRequired.Code) {
		t.Fatalf("empty table err = %v", err)
	}

	if len(requests) != 10 {
		t.Fatalf("request count = %d: %#v", len(requests), requests)
	}
	for i, request := range requests {
		if request.workspace != "ws-1" {
			t.Fatalf("request %d workspace = %q", i, request.workspace)
		}
	}
	expectedPaths := []string{
		"/newmoi/meta/db/info",
		"/newmoi/meta/db/tree",
		"/newmoi/meta/db/table",
		"/newmoi/meta/table/info",
		"/newmoi/meta/table/column",
		"/newmoi/meta/table/column_stat",
		"/newmoi/meta/table/ddl",
		"/newmoi/meta/table/drop",
		"/newmoi/meta/view/drop",
		"/newmoi/meta/db/drop",
	}
	for i, path := range expectedPaths {
		if requests[i].path != path {
			t.Fatalf("request %d path = %s, want %s", i, requests[i].path, path)
		}
	}
	if requests[0].body != nil || requests[1].body != nil {
		t.Fatalf("metadata list bodies = %#v/%#v", requests[0].body, requests[1].body)
	}
	if requests[2].body["name"] != "db1" {
		t.Fatalf("tables body = %#v", requests[2].body)
	}
	for i := 3; i <= 7; i++ {
		if requests[i].body["db_name"] != "db1" || requests[i].body["table_name"] != "orders" {
			t.Fatalf("table request %d body = %#v", i, requests[i].body)
		}
	}
	if requests[8].body["db_name"] != "db1" || requests[8].body["table_name"] != "orders_view" {
		t.Fatalf("drop view body = %#v", requests[8].body)
	}
	if requests[9].body["db_name"] != "db1" {
		t.Fatalf("drop database body = %#v", requests[9].body)
	}
}

func TestCatalogUploadUsesMultipartFileField(t *testing.T) {
	var gotWorkspace, gotFilename, gotBody string
	var gotForm *multipart.Form
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/newmoi/catalog/file/upload" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		gotWorkspace = r.Header.Get(headerWorkspaceID)
		if !strings.HasPrefix(r.Header.Get(headerContentType), "multipart/form-data; boundary=") {
			t.Fatalf("content-type = %q", r.Header.Get(headerContentType))
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		gotFilename = header.Filename
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		gotBody = string(data)
		gotForm = r.MultipartForm
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{"file_id": "file-1"},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Catalog("ws-1").UploadBytes(context.Background(), "data.txt", []byte("abc"))
	if err != nil {
		t.Fatal(err)
	}
	if gotWorkspace != "ws-1" {
		t.Fatalf("workspace = %q", gotWorkspace)
	}
	if gotFilename != "data.txt" {
		t.Fatalf("filename = %q", gotFilename)
	}
	if gotBody != "abc" {
		t.Fatalf("body = %q", gotBody)
	}
	if gotForm == nil {
		t.Fatal("multipart form was not parsed")
	}
	if resp.GetFileId() != "file-1" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestCatalogPrivateUploadUsesWorkspaceScopedRoute(t *testing.T) {
	var gotWorkspace, gotFilename, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/newmoi/workspaces/ws-1/catalog-files/uploads" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		gotWorkspace = r.Header.Get(headerWorkspaceID)
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		gotFilename = header.Filename
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		gotBody = string(data)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "OK",
			"msg":  "OK",
			"data": map[string]any{
				"file_id":       "file-1",
				"original_name": "resume.pdf",
				"size":          3,
				"md5":           "digest",
				"catalog_file": map[string]any{
					"workspace_id": "ws-1",
					"volume_id":    17,
					"file_id":      "file-1",
					"path":         "resume.pdf",
					"name":         "resume.pdf",
					"size":         3,
					"md5":          "digest",
				},
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Catalog("ws-1").UploadPrivateBytes(context.Background(), "resume.pdf", []byte("pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if gotWorkspace != "ws-1" {
		t.Fatalf("workspace = %q", gotWorkspace)
	}
	if gotFilename != "resume.pdf" {
		t.Fatalf("filename = %q", gotFilename)
	}
	if gotBody != "pdf" {
		t.Fatalf("body = %q", gotBody)
	}
	if resp.GetFileId() != "file-1" || resp.GetCatalogFile().GetVolumeId() != 17 {
		t.Fatalf("response = %#v", resp)
	}
}

func TestCatalogRuntimeArtifactUsesChildRuntimeContract(t *testing.T) {
	var gotWorkspace, gotRuntimeAuthorization, gotFilename, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/newmoi/workspaces/workspace%201/runtime-artifacts" {
			t.Fatalf("request = %s %s", r.Method, r.URL.EscapedPath())
		}
		gotWorkspace = r.Header.Get(headerWorkspaceID)
		gotRuntimeAuthorization = r.Header.Get(headerRuntimeAuthorization)
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		gotFilename = header.Filename
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		gotBody = string(data)
		_, _ = w.Write([]byte(`{"file_id":"file-1","original_name":"report.txt","size":3,"md5":"md5","artifact_id":"artifact-1","sha256":"sha256:abc"}`))
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithRuntimeAuthorization("Bearer runtime-proof"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Catalog("workspace 1").UploadRuntimeArtifact(context.Background(), "report.txt", strings.NewReader("abc"))
	if err != nil {
		t.Fatal(err)
	}
	if gotWorkspace != "workspace 1" || gotRuntimeAuthorization != "Bearer runtime-proof" || gotFilename != "report.txt" || gotBody != "abc" {
		t.Fatalf("request headers/body = workspace=%q runtime=%q filename=%q body=%q", gotWorkspace, gotRuntimeAuthorization, gotFilename, gotBody)
	}
	if result.GetFileId() != "file-1" || result.GetArtifactId() != "artifact-1" || result.GetSha256() != "sha256:abc" {
		t.Fatalf("result = %#v", result)
	}
}

func TestCatalogRuntimeImageAnalysisUsesChildRuntimeContract(t *testing.T) {
	var gotWorkspace, gotRuntimeAuthorization, gotFilename, gotQuestion, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/newmoi/workspaces/workspace%201/runtime-image-analysis" {
			t.Fatalf("request = %s %s", r.Method, r.URL.EscapedPath())
		}
		gotWorkspace = r.Header.Get(headerWorkspaceID)
		gotRuntimeAuthorization = r.Header.Get(headerRuntimeAuthorization)
		gotQuestion = r.FormValue("question")
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		gotFilename = header.Filename
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		gotBody = string(data)
		_, _ = w.Write([]byte(`{"answer":"a cat"}`))
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithRuntimeAuthorization("Bearer runtime-proof"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Catalog("workspace 1").AnalyzeRuntimeImage(
		context.Background(), "image.jpg", strings.NewReader("jpeg bytes"), "what is shown?",
	)
	if err != nil {
		t.Fatal(err)
	}
	if gotWorkspace != "workspace 1" || gotRuntimeAuthorization != "Bearer runtime-proof" ||
		gotFilename != "image.jpg" || gotQuestion != "what is shown?" || gotBody != "jpeg bytes" {
		t.Fatalf("request = workspace=%q runtime=%q filename=%q question=%q body=%q", gotWorkspace, gotRuntimeAuthorization, gotFilename, gotQuestion, gotBody)
	}
	if result.GetAnswer() != "a cat" {
		t.Fatalf("result = %#v", result)
	}
}

func TestCatalogDownloadWorkspaceFileUsesTaskVisibleRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.EscapedPath() != "/newmoi/workspaces/workspace%201/files/file%2F1/download" {
			t.Fatalf("request = %s %s", r.Method, r.URL.EscapedPath())
		}
		if got := r.Header.Get(headerRuntimeAuthorization); got != "Bearer runtime-proof" {
			t.Fatalf("runtime authorization = %q", got)
		}
		_, _ = w.Write([]byte("file bytes"))
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithRuntimeAuthorization("Bearer runtime-proof"))
	if err != nil {
		t.Fatal(err)
	}
	stream, err := client.Catalog("workspace 1").DownloadWorkspaceFile(context.Background(), "file/1")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	data, err := io.ReadAll(stream.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "file bytes" {
		t.Fatalf("download = %q", data)
	}
}

func TestWorkspaceConsumerMemoryHandleScopesRetrieveAndProfile(t *testing.T) {
	type recorded struct {
		method    string
		path      string
		workspace string
		body      map[string]any
	}
	var requests []recorded
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		requests = append(requests, recorded{
			method:    r.Method,
			path:      r.URL.EscapedPath(),
			workspace: r.Header.Get(headerWorkspaceID),
			body:      body,
		})
		switch r.URL.EscapedPath() {
		case "/newmoi/memories/retrieve", "/newmoi/memories/search", "/newmoi/memories/fulltext-search", "/newmoi/memories/query":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": map[string]any{
				"memories": []any{map[string]any{
					"memory_id": "m1", "subject_id": "subject-1", "memory_type": "semantic",
					"content": "likes seafood", "trust_tier": "T1", "initial_confidence": 0.9, "is_active": true,
				}},
			}})
			return
		case "/newmoi/consumer-profiles/subject-1/render":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": map[string]any{
				"subject_id": "subject-1", "text": "profile text", "token_estimate": 12, "pending_conflicts": []any{},
			}})
			return
		}
		http.Error(w, "unexpected path", http.StatusNotFound)
	}))
	defer server.Close()

	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := client.Workspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	handle, err := workspace.ConsumerMemory("subject-1")
	if err != nil {
		t.Fatalf("consumer memory handle err = %v", err)
	}
	if handle.ID() != "subject-1" {
		t.Fatalf("subject id = %q", handle.ID())
	}

	ctx := context.Background()
	got, err := handle.Retrieve(ctx, "seafood allergy")
	if err != nil {
		t.Fatalf("retrieve err = %v", err)
	}
	if len(got.GetMemories()) != 1 || got.GetMemories()[0].GetMemoryId() != "m1" {
		t.Fatalf("retrieve result = %+v", got)
	}
	if _, err := handle.FulltextSearch(ctx, ConsumerMemoryFulltextSearchInput{
		Query: "seafood", ExtraMetadataFilter: map[string]any{"department": "sales"},
	}); err != nil {
		t.Fatalf("fulltext search err = %v", err)
	}
	if _, err := handle.Query(ctx, ConsumerMemoryStructuredQueryInput{
		ExtraMetadataFilter: map[string]any{"urgent": true},
	}); err != nil {
		t.Fatalf("structured query err = %v", err)
	}
	profile, err := handle.RenderProfile(ctx)
	if err != nil {
		t.Fatalf("render profile err = %v", err)
	}
	if profile.GetText() != "profile text" || profile.GetTokenEstimate() != 12 {
		t.Fatalf("profile = %+v", profile)
	}

	if len(requests) != 4 {
		t.Fatalf("request count = %d", len(requests))
	}
	wantPaths := []string{
		"/newmoi/memories/retrieve",
		"/newmoi/memories/fulltext-search",
		"/newmoi/memories/query",
		"/newmoi/consumer-profiles/subject-1/render",
	}
	for i, want := range wantPaths {
		if requests[i].path != want {
			t.Fatalf("request[%d] path = %q, want %q", i, requests[i].path, want)
		}
		if requests[i].workspace != "ws-1" {
			t.Fatalf("request[%d] workspace = %q", i, requests[i].workspace)
		}
	}
	if requests[0].body["subject_id"] != "subject-1" || requests[0].body["query"] != "seafood allergy" {
		t.Fatalf("retrieve body = %+v", requests[0].body)
	}
	if requests[1].body["subject_id"] != "subject-1" || requests[1].body["query"] != "seafood" {
		t.Fatalf("fulltext body = %+v", requests[1].body)
	}
	if requests[2].body["subject_id"] != "subject-1" {
		t.Fatalf("structured query body = %+v", requests[2].body)
	}
}

func TestConsumerMemoryInputsRejectInvalidMetadataBeforeRequest(t *testing.T) {
	if err := (ConsumerMemoryFulltextSearchInput{Query: "x", ExtraMetadataFilter: map[string]any{"nested": map[string]any{"x": 1}}}).Validate(); !IsCode(err, "ErrInvalidArgument") {
		t.Fatalf("fulltext validation err = %v", err)
	}
	if err := (ConsumerMemoryStructuredQueryInput{ExtraMetadataFilter: map[string]any{"bad.key": true}}).Validate(); !IsCode(err, "ErrInvalidArgument") {
		t.Fatalf("structured validation err = %v", err)
	}
	if err := (MemoryQuery{Query: "x", SubjectID: "subject-1", TopK: 101}).validate(); !IsCode(err, "ErrInvalidArgument") {
		t.Fatalf("semantic retrieval validation err = %v", err)
	}
}
