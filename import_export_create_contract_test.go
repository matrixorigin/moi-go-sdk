package sdk

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestImportExportCreateContract(t *testing.T) {
	var fixture struct {
		FileImports        []importExportCreateCase `json:"file_imports"`
		ExplicitTaskImport importExportCreateCase   `json:"explicit_task_import"`
		Exports            []importExportCreateCase `json:"exports"`
		OwnerDenial        struct {
			Status int `json:"status"`
			Body   struct {
				Code    string            `json:"code"`
				Message string            `json:"msg"`
				Details map[string]string `json:"details"`
			} `json:"body"`
		} `json:"owner_denial"`
	}
	value, err := os.ReadFile("testdata/import_export_create_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(value, &fixture); err != nil {
		t.Fatal(err)
	}
	fixture.FileImports = append(fixture.FileImports, fixture.ExplicitTaskImport)
	for _, group := range []struct {
		kind  string
		path  string
		cases []importExportCreateCase
	}{
		{"import", "/newmoi/task", fixture.FileImports},
		{"export", "/newmoi/export/task/create", fixture.Exports},
	} {
		for _, scenario := range group.cases {
			for _, denied := range []bool{false, true} {
				name := group.kind + "/" + scenario.Name
				if denied {
					name += "/owner denial"
				}
				t.Run(name, func(t *testing.T) {
					var calls atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls.Add(1)
						if r.Method != http.MethodPost || r.URL.Path != group.path || r.Header.Get("X-Workspace-ID") != "workspace-1" {
							t.Errorf("unexpected target: %s %s workspace=%s", r.Method, r.URL.Path, r.Header.Get("X-Workspace-ID"))
						}
						var got, want map[string]any
						if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
							t.Error(err)
						}
						if err := json.Unmarshal(scenario.Request, &want); err != nil {
							t.Error(err)
						}
						if !reflect.DeepEqual(got, want) {
							t.Errorf("request = %#v, want %#v", got, want)
						}
						if denied {
							w.WriteHeader(fixture.OwnerDenial.Status)
							_ = json.NewEncoder(w).Encode(fixture.OwnerDenial.Body)
							return
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": scenario.Response})
					}))
					t.Cleanup(server.Close)
					client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = client.Close() })
					workspace, err := client.Workspace("workspace-1")
					if err != nil {
						t.Fatal(err)
					}
					connector, err := workspace.Connector("connector-1")
					if err != nil {
						t.Fatal(err)
					}
					var callErr error
					if group.kind == "import" {
						spec := importSpecFromCreateCase(t, scenario)
						handle, result, err := connector.CreateFileImportTask(t.Context(), spec)
						callErr = err
						if err == nil {
							var want ImportTaskCreateResult
							if err := json.Unmarshal(scenario.Response, &want); err != nil {
								t.Fatal(err)
							}
							if handle.ID() != want.GetTaskId() || result.GetTaskId() != want.GetTaskId() {
								t.Errorf("returned IDs = %s, %s, want %s", handle.ID(), result.GetTaskId(), want.GetTaskId())
							}
						}
					} else {
						spec := exportSpecFromCreateCase(t, scenario)
						handle, result, err := connector.CreateExportTaskFromSpec(t.Context(), spec)
						callErr = err
						if err == nil && scenario.LocalError == "" {
							var want struct{ ID, Name string }
							if err := json.Unmarshal(scenario.Response, &want); err != nil {
								t.Fatal(err)
							}
							if handle.ID() != want.ID || result.GetId() != want.ID || result.GetName() != want.Name {
								t.Errorf("returned export = %s, %v, want %v", handle.ID(), result, want)
							}
						}
					}
					if scenario.LocalError != "" {
						if !IsCode(callErr, scenario.LocalError) || calls.Load() != 0 {
							t.Fatalf("invalid input reached owner: err=%v calls=%d", callErr, calls.Load())
						}
					} else if denied {
						var ownerError *Error
						if !errors.As(callErr, &ownerError) || ownerError.Code != fixture.OwnerDenial.Body.Code || ownerError.Message != fixture.OwnerDenial.Body.Message || ownerError.Status != fixture.OwnerDenial.Status {
							t.Fatalf("owner denial was changed: %v", callErr)
						}
					} else if callErr != nil {
						t.Fatal(callErr)
					}
					if scenario.LocalError == "" && calls.Load() != 1 {
						t.Errorf("requests = %d, want one create without retry", calls.Load())
					}
				})
			}
		}
	}
}

type importExportCreateCase struct {
	Name       string          `json:"name"`
	Request    json.RawMessage `json:"request"`
	Response   json.RawMessage `json:"response"`
	LocalError string          `json:"local_error"`
}

func importSpecFromCreateCase(t *testing.T, scenario importExportCreateCase) FileImportTaskSpec {
	t.Helper()
	var body struct {
		TaskID       string `json:"task_id"`
		ConfigType   int    `json:"config_type"`
		Name         string `json:"name"`
		VolumeID     string `json:"volume_id"`
		SourceConfig struct {
			Common struct {
				URIs               []string       `json:"uris"`
				LoadModeConfig     map[string]any `json:"load_mode_config"`
				FileFilterConfig   map[string]any `json:"file_filter_config"`
				UnzipKeepStructure *bool          `json:"unzip_keep_structure"`
				FileStorageMode    string         `json:"file_storage_mode"`
			} `json:"common_file_task_config"`
		} `json:"source_config"`
	}
	if err := json.Unmarshal(scenario.Request, &body); err != nil {
		t.Fatal(err)
	}
	common := body.SourceConfig.Common
	return FileImportTaskSpec{TaskID: body.TaskID, ConfigType: body.ConfigType, Name: body.Name, VolumeID: body.VolumeID,
		URIs: common.URIs, LoadModeConfig: common.LoadModeConfig, FileFilterConfig: common.FileFilterConfig,
		UnzipKeepStructure: common.UnzipKeepStructure, FileStorageMode: common.FileStorageMode}
}

func exportSpecFromCreateCase(t *testing.T, scenario importExportCreateCase) ExportTaskSpec {
	t.Helper()
	var body struct {
		Name          string `json:"task_name"`
		Creator       string `json:"creator"`
		ConnectorID   string `json:"connector_id"`
		ConnectorName string `json:"connector_name"`
		Type          int    `json:"type"`
		Config        struct {
			MergeTitleToText *bool          `json:"merge_title_to_text"`
			S3Config         map[string]any `json:"s3_config"`
		} `json:"config"`
		Files []ExportFileParam `json:"files"`
	}
	if err := json.Unmarshal(scenario.Request, &body); err != nil {
		t.Fatal(err)
	}
	files := make([]ExportTaskFile, 0, len(body.Files))
	for _, file := range body.Files {
		files = append(files, ExportTaskFile{FileID: file.FileID, FullPath: file.FullPath, IsRaw: &file.IsRaw})
	}
	return ExportTaskSpec{Name: body.Name, Creator: body.Creator, ConnectorID: body.ConnectorID, ConnectorName: body.ConnectorName,
		Type: body.Type, Config: ExportTaskConfig{MergeTitleToText: body.Config.MergeTitleToText, S3Config: body.Config.S3Config}, Files: files}
}
