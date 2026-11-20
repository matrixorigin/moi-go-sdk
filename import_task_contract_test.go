package sdk

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestStructuredImportTaskCreatePayload(t *testing.T) {
	config := map[string]any{
		"version":     1,
		"source":      map[string]any{"database": "orders", "table": "events"},
		"load_policy": map[string]any{"run_mode": "once", "conflict_policy": "fail"},
	}
	body, err := (ImportTaskCreateSpec{
		SourceConnectorID: "connector-1", ConfigType: 3, Name: "events",
		StructuredLoadConfig: config,
	}).Body()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"source_connector_id": "connector-1", "config_type": 3, "name": "events",
		"structured_load_config": config,
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("structured create payload = %#v, want %#v", body, want)
	}
}

func TestIssue13504ImportTaskListAndGetStrictContract(t *testing.T) {
	for _, fixture := range []string{
		"issue13504_local_task.json",
		"issue13504_structured_task.json",
	} {
		taskJSON := readIssue13504TaskFixture(t, fixture)
		for _, endpoint := range []struct {
			name string
			body func(json.RawMessage) []byte
			out  func() any
			task func(any) *product.ImportTask
		}{
			{
				name: "list",
				body: func(task json.RawMessage) []byte {
					return []byte(`{"tasks":[` + string(task) + `],"total":1}`)
				},
				out: func() any { return &product.ImportTaskListResult{} },
				task: func(out any) *product.ImportTask {
					return out.(*product.ImportTaskListResult).GetTasks()[0]
				},
			},
			{
				name: "get",
				body: func(task json.RawMessage) []byte {
					return []byte(`{"task":` + string(task) + `}`)
				},
				out: func() any { return &product.ImportTaskInfoResult{} },
				task: func(out any) *product.ImportTask {
					return out.(*product.ImportTaskInfoResult).GetTask()
				},
			},
		} {
			t.Run(fixture+"/"+endpoint.name, func(t *testing.T) {
				out := endpoint.out()
				if err := parseProductResponse(endpoint.body(taskJSON), out, ""); err != nil {
					t.Fatalf("strict decode: %v", err)
				}
				assertIssue13504ImportTask(t, endpoint.task(out), fixture)
			})
		}
	}
}

func TestIssue13504ImportTaskStrictContractRejectsUnknownField(t *testing.T) {
	taskJSON := strings.TrimSuffix(string(readIssue13504TaskFixture(t, "issue13504_local_task.json")), "\n")
	taskJSON = strings.TrimSuffix(taskJSON, "}") + `,"unregistered_field":true}`

	cases := []struct {
		name string
		body []byte
		out  any
	}{
		{
			name: "list",
			body: []byte(`{"tasks":[` + taskJSON + `],"total":1}`),
			out:  &product.ImportTaskListResult{},
		},
		{
			name: "get",
			body: []byte(`{"task":` + taskJSON + `}`),
			out:  &product.ImportTaskInfoResult{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := parseProductResponse(tc.body, tc.out, "")
			if err == nil || !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("expected unknown field error, got %v", err)
			}
		})
	}
}

func TestIssue13504ImportTaskProtoFieldNumbers(t *testing.T) {
	expected := map[string]int{
		"source_files":              22,
		"load_type":                 23,
		"config_type":               24,
		"total_rows":                25,
		"imported_rows":             26,
		"success_file_count":        27,
		"failed_file_count":         28,
		"table_paths":               29,
		"runtime_init_status":       30,
		"priority":                  31,
		"config_hash":               32,
		"structured_config_hash":    33,
		"structured_load_summary":   34,
		"structured_runtime":        35,
		"structured_load_config":    36,
		"mapping":                   37,
		"schedule":                  38,
		"backfill":                  39,
		"latest_rows":               40,
		"cumulative_rows":           41,
		"error_code":                42,
		"error_summary":             43,
		"target_path_state":         44,
		"target_path_error_code":    45,
		"target_path_error_summary": 46,
	}
	fields := (&product.ImportTask{}).ProtoReflect().Descriptor().Fields()
	for name, number := range expected {
		field := fields.ByName(protoreflect.Name(name))
		if field == nil || int(field.Number()) != number {
			t.Fatalf("field %s number = %v, want %d", name, field, number)
		}
	}
	sourceFiles := fields.ByName("source_files")
	if sourceFiles.Message().FullName() != "google.protobuf.ListValue" {
		t.Fatalf("source_files type = %s", sourceFiles.Message().FullName())
	}
}

func readIssue13504TaskFixture(t *testing.T, name string) json.RawMessage {
	t.Helper()
	value, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return value
}

func assertIssue13504ImportTask(t *testing.T, task *product.ImportTask, fixture string) {
	t.Helper()
	if task == nil {
		t.Fatal("decoded task is nil")
	}
	if task.GetConfigType() == 0 || task.GetTotalRows() != 4 || task.GetImportedRows() == 0 {
		t.Fatalf("missing required task counters: %+v", task)
	}
	if task.GetLatestRows() == nil || task.GetCumulativeRows() == nil {
		t.Fatal("row stats were not decoded")
	}
	for name, stats := range map[string]*product.ImportTaskRowStats{
		"latest":     task.GetLatestRows(),
		"cumulative": task.GetCumulativeRows(),
	} {
		if stats.ReadRows == nil || stats.SucceededRows == nil || stats.FailedRows == nil || stats.SkippedRows == nil {
			t.Fatalf("%s row stats lost optional presence: %+v", name, stats)
		}
		if !stats.GetComplete() || stats.GetFailedRows() != 0 {
			t.Fatalf("%s row stats lost complete or exact zero: %+v", name, stats)
		}
	}
	if task.GetTargetPathState() != "resolved" {
		t.Fatalf("target_path_state = %q", task.GetTargetPathState())
	}

	if fixture == "issue13504_local_task.json" {
		if task.GetConfigType() != 1 {
			t.Fatalf("local config_type = %d, want 1", task.GetConfigType())
		}
		if task.GetErrorCode() != "LOCAL_IMPORT_EXECUTION_FAILED" {
			t.Fatalf("local error_code = %q", task.GetErrorCode())
		}
		if len(task.GetSourceFiles()) != 2 {
			t.Fatalf("source_files count = %d", len(task.GetSourceFiles()))
		}
		wantFilenames := []string{"orders.csv", "orders-2.csv"}
		for i, file := range task.GetSourceFiles() {
			if len(file.GetValues()) != 2 {
				t.Fatalf("source_files[%d] length = %d", i, len(file.GetValues()))
			}
			for j, value := range file.GetValues() {
				if _, ok := value.GetKind().(*structpb.Value_StringValue); !ok {
					t.Fatalf("source_files[%d][%d] is not a string: %v", i, j, value)
				}
			}
			if file.GetValues()[0].GetStringValue() != task.GetConnectorName() {
				t.Fatalf("source_files[%d] connector = %q, want %q", i, file.GetValues()[0].GetStringValue(), task.GetConnectorName())
			}
			if file.GetValues()[1].GetStringValue() != wantFilenames[i] {
				t.Fatalf("source_files[%d] filename = %q, want %q", i, file.GetValues()[1].GetStringValue(), wantFilenames[i])
			}
		}
		assertIssue13504SourceFilesJSONShape(t, task)
		return
	}

	if task.GetConfigType() != 3 {
		t.Fatalf("structured config_type = %d, want 3", task.GetConfigType())
	}
	if task.GetErrorCode() != "" {
		t.Fatalf("structured error_code = %q, want empty", task.GetErrorCode())
	}
	if task.GetStructuredLoadConfig() == nil ||
		task.GetStructuredLoadSummary() == nil ||
		task.GetStructuredRuntime() == nil ||
		task.GetMapping() == nil ||
		task.GetSchedule() == nil ||
		task.GetBackfill() == nil {
		t.Fatal("structured task dynamic or typed fields were not decoded")
	}
	summary := task.GetStructuredLoadSummary().AsMap()
	for _, field := range []string{"source", "target", "progress", "checkpoint", "reconcile"} {
		if _, ok := summary[field]; !ok {
			t.Fatalf("structured summary field %q was not decoded", field)
		}
	}
	runtime := task.GetStructuredRuntime().AsMap()
	for _, field := range []string{"progress", "checkpoint", "reconcile", "last_run"} {
		if _, ok := runtime[field]; !ok {
			t.Fatalf("structured runtime field %q was not decoded", field)
		}
	}
}

func assertIssue13504SourceFilesJSONShape(t *testing.T, task *product.ImportTask) {
	t.Helper()
	encoded, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(task)
	if err != nil {
		t.Fatalf("encode task: %v", err)
	}
	var value map[string]any
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatalf("decode encoded task: %v", err)
	}
	files, ok := value["source_files"].([]any)
	if !ok || len(files) != 2 {
		t.Fatalf("source_files is not a bare list: %s", encoded)
	}
	for i, file := range files {
		values, ok := file.([]any)
		if !ok {
			t.Fatalf("source_files[%d] has wrapper shape: %T", i, file)
		}
		for j, item := range values {
			if _, ok := item.(string); !ok {
				t.Fatalf("source_files[%d][%d] is not string: %T", i, j, item)
			}
		}
	}
}
