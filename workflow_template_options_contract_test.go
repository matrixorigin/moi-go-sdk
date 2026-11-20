package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestWorkflowTemplateOptionsMatchSharedHandlerContract(t *testing.T) {
	var cases []struct {
		Name, Command, ID, Method, Path string
		Request, Response               json.RawMessage
		Status                          int
	}
	data, err := os.ReadFile("testdata/workflow_template_options_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()
			var want any
			decoder := json.NewDecoder(bytes.NewReader(scenario.Request))
			decoder.UseNumber()
			if err := decoder.Decode(&want); err != nil {
				t.Fatal(err)
			}
			received := make(chan any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != scenario.Method || r.URL.Path != "/newmoi"+scenario.Path || r.URL.RawQuery != "" || r.Header.Get("X-API-Key") != "synthetic-key" || r.Header.Get("X-Workspace-ID") != "workspace-1" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				var got any
				if want == nil {
					if len(raw) != 0 {
						t.Errorf("unexpected body=%s", raw)
					}
				} else {
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					if err := decoder.Decode(&got); err != nil {
						t.Error(err)
					}
				}
				received <- got
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				_, _ = w.Write(scenario.Response)
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			var input struct {
				Name        string
				DSLYAML     string `json:"dsl_yaml"`
				Workflow    map[string]any
				TemplateKey *string `json:"template_key"`
				Description *string
				InputForm   *string `json:"runtime_fields"`
				IsBuiltin   *bool   `json:"is_builtin"`
			}
			if err := json.Unmarshal(scenario.Request, &input); err != nil {
				t.Fatal(err)
			}
			var opts []WorkflowTemplateOption
			if input.TemplateKey != nil {
				opts = append(opts, WithWorkflowTemplateKey(*input.TemplateKey))
			}
			if input.Description != nil {
				opts = append(opts, WithWorkflowTemplateDescription(*input.Description))
			}
			if input.InputForm != nil {
				opts = append(opts, WithWorkflowTemplateInputForm(*input.InputForm))
			}
			if input.IsBuiltin != nil {
				opts = append(opts, WithWorkflowTemplateBuiltin(*input.IsBuiltin))
			}
			service := client.Workflows("workspace-1")
			var result *WorkflowTemplate
			switch scenario.Command {
			case "get-workflow-template":
				result, err = service.GetWorkflowTemplate(t.Context(), scenario.ID)
			case "create-workflow-template":
				workspace, bindErr := client.Workspace("workspace-1")
				if bindErr != nil {
					t.Fatal(bindErr)
				}
				var handle *WorkflowTemplateHandle
				handle, result, err = workspace.CreateWorkflowTemplate(t.Context(), input.Name, input.DSLYAML, opts...)
				if err == nil && handle.ID() != scenario.ID {
					t.Fatalf("created handle ID=%s, want=%s", handle.ID(), scenario.ID)
				}
			case "create-mowl-workflow-template":
				result, err = service.CreateMOWLWorkflowTemplate(t.Context(), input.Name, input.Workflow, opts...)
			case "update-workflow-template":
				result, err = service.UpdateWorkflowTemplate(t.Context(), scenario.ID, input.Name, input.DSLYAML, opts...)
			case "update-mowl-workflow-template":
				workspace, bindErr := client.Workspace("workspace-1")
				if bindErr != nil {
					t.Fatal(bindErr)
				}
				template, bindErr := workspace.WorkflowTemplate(scenario.ID)
				if bindErr != nil {
					t.Fatal(bindErr)
				}
				result, err = template.UpdateMOWL(t.Context(), input.Name, input.Workflow, opts...)
			default:
				t.Fatalf("unknown fixture command %s", scenario.Command)
			}
			select {
			case got := <-received:
				if !reflect.DeepEqual(got, want) {
					t.Errorf("request=%v, want=%v", got, want)
				}
			default:
				t.Fatal("no HTTP request received")
			}
			var response struct {
				Code, Msg string
				Data      json.RawMessage
			}
			if decodeErr := json.Unmarshal(scenario.Response, &response); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if scenario.Status >= 400 {
				var failure *Error
				if !errors.As(err, &failure) || failure.Code != response.Code || failure.Message != response.Msg || failure.Status != scenario.Status {
					t.Fatalf("owner failure changed: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var expected WorkflowTemplate
			if err := protojson.Unmarshal(response.Data, &expected); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(result, &expected) {
				t.Errorf("response=%v, want=%v", result, &expected)
			}
		})
	}
}
