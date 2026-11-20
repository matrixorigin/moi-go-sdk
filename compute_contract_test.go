package sdk

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sync/atomic"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestComputeRequestsAndResponsesMatchHandlerContract(t *testing.T) {
	var cases []struct {
		Name, Command, ID, Method, Path string
		Input                           map[string]any
		Query                           url.Values
		Request, Response               json.RawMessage
	}
	value, err := os.ReadFile("testdata/compute_options_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(value, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			var wantBody any
			decoder := json.NewDecoder(bytes.NewReader(scenario.Request))
			decoder.UseNumber()
			if err := decoder.Decode(&wantBody); err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != scenario.Method || r.URL.Path != "/newmoi"+scenario.Path || r.URL.Query().Encode() != scenario.Query.Encode() || r.Header.Get("X-Workspace-ID") != "workspace-1" || r.Header.Get("X-API-Key") != "synthetic-key" {
					t.Errorf("unexpected scoped request: %s %s", r.Method, r.URL.RequestURI())
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if wantBody == nil {
					if len(raw) != 0 {
						t.Errorf("unexpected body: %s", raw)
					}
				} else {
					var got any
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					if err := decoder.Decode(&got); err != nil {
						t.Error(err)
					}
					if !reflect.DeepEqual(got, wantBody) {
						t.Errorf("request=%s, expected=%s", raw, scenario.Request)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(append(append([]byte(`{"code":"OK","data":`), scenario.Response...), '}'))
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			service := client.Compute("workspace-1")
			var result proto.Message
			switch scenario.Command {
			case "create-spec":
				opts := computeContractSpecCreateOptions(scenario.Input)
				result, err = service.CreateSpec(t.Context(), scenario.ID, scenario.Input["kind"].(string), scenario.Input["family"].(string), scenario.Input["family_name"].(string), scenario.Input["family_name_en"].(string), int(scenario.Input["cpu_milli"].(float64)), int(scenario.Input["memory_mib"].(float64)), opts...)
			case "update-spec":
				result, err = service.UpdateSpec(t.Context(), scenario.ID, computeContractSpecUpdateOptions(scenario.Input)...)
			case "create":
				result, err = service.Create(t.Context(), scenario.Input["name"].(string), scenario.Input["spec_id"].(string), int(scenario.Input["cpu"].(float64)), int(scenario.Input["memory_gib"].(float64)), computeContractCreateOptions(t, scenario.Input)...)
			case "update":
				result, err = service.Update(t.Context(), scenario.ID, computeContractUpdateOptions(t, scenario.Input)...)
			case "admin-update":
				result, err = service.AdminUpdate(t.Context(), scenario.ID, computeContractUpdateOptions(t, scenario.Input)...)
			case "get":
				result, err = service.Get(t.Context(), scenario.ID)
			case "admin-list":
				var opts []ListOption
				for key, values := range scenario.Query {
					opts = append(opts, WithQueryValues(key, values...))
				}
				result, err = service.AdminList(t.Context(), opts...)
			default:
				t.Fatalf("unknown fixture operation: %s", scenario.Command)
			}
			if err != nil {
				t.Fatal(err)
			}
			want := result.ProtoReflect().Type().New().Interface()
			if err := protojson.Unmarshal(scenario.Response, want); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(result, want) {
				t.Errorf("result=%v, expected=%v", result, want)
			}
			if calls.Load() != 1 {
				t.Errorf("requests=%d; expected one operation", calls.Load())
			}
		})
	}
}

func computeContractPlacement(value any) map[string]any {
	if value == nil {
		return nil
	}
	return value.(map[string]any)
}

func computeContractSpecCreateOptions(input map[string]any) []ComputeSpecOption {
	var opts []ComputeSpecOption
	for key, value := range input {
		switch key {
		case "gpu_count":
			opts = append(opts, WithComputeSpecGPUCount(int(value.(float64))))
		case "gpu_memory_mib":
			opts = append(opts, WithComputeSpecGPUMemoryMiB(int(value.(float64))))
		case "gpu_cores":
			opts = append(opts, WithComputeSpecGPUCores(int(value.(float64))))
		case "credit_per_hour":
			opts = append(opts, WithComputeSpecCreditPerHour(value.(float64)))
		case "description":
			opts = append(opts, WithComputeSpecDescription(value.(string)))
		case "enabled":
			opts = append(opts, WithComputeSpecEnabled(value.(bool)))
		case "node_placement":
			opts = append(opts, WithComputeSpecNodePlacement(computeContractPlacement(value)))
		}
	}
	return opts
}

func computeContractSpecUpdateOptions(input map[string]any) []ComputeSpecUpdateOption {
	var opts []ComputeSpecUpdateOption
	for key, value := range input {
		switch key {
		case "kind":
			opts = append(opts, WithComputeSpecUpdateKind(value.(string)))
		case "family":
			opts = append(opts, WithComputeSpecUpdateFamily(value.(string)))
		case "family_name":
			opts = append(opts, WithComputeSpecUpdateFamilyName(value.(string)))
		case "family_name_en":
			opts = append(opts, WithComputeSpecUpdateFamilyNameEN(value.(string)))
		case "cpu_milli":
			opts = append(opts, WithComputeSpecUpdateCPUMilli(int(value.(float64))))
		case "memory_mib":
			opts = append(opts, WithComputeSpecUpdateMemoryMiB(int(value.(float64))))
		case "gpu_count":
			opts = append(opts, WithComputeSpecUpdateGPUCount(int(value.(float64))))
		case "gpu_memory_mib":
			opts = append(opts, WithComputeSpecUpdateGPUMemoryMiB(int(value.(float64))))
		case "gpu_cores":
			opts = append(opts, WithComputeSpecUpdateGPUCores(int(value.(float64))))
		case "credit_per_hour":
			opts = append(opts, WithComputeSpecUpdateCreditPerHour(value.(float64)))
		case "description":
			opts = append(opts, WithComputeSpecUpdateDescription(value.(string)))
		case "enabled":
			opts = append(opts, WithComputeSpecUpdateEnabled(value.(bool)))
		case "node_placement":
			opts = append(opts, WithComputeSpecUpdateNodePlacement(computeContractPlacement(value)))
		}
	}
	return opts
}

func computeContractImages(t *testing.T, value any) []*WorkerImageSelection {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var images []*WorkerImageSelection
	if err := json.Unmarshal(raw, &images); err != nil {
		t.Fatal(err)
	}
	return images
}

func computeContractCreateOptions(t *testing.T, input map[string]any) []ComputeCreateOption {
	t.Helper()
	var opts []ComputeCreateOption
	for key, value := range input {
		switch key {
		case "kind":
			opts = append(opts, WithComputeKind(value.(string)))
		case "description":
			opts = append(opts, WithComputeDescription(value.(string)))
		case "gpu":
			opts = append(opts, WithComputeGPU(int(value.(float64))))
		case "min_replicas":
			opts = append(opts, WithComputeMinReplicas(int(value.(float64))))
		case "max_replicas":
			opts = append(opts, WithComputeMaxReplicas(int(value.(float64))))
		case "auto_suspend_minutes":
			opts = append(opts, WithComputeAutoSuspendMinutes(int(value.(float64))))
		case "go_worker_image_id":
			opts = append(opts, WithComputeGoWorkerImageID(value.(string)))
		case "python_worker_image_id":
			opts = append(opts, WithComputePythonWorkerImageID(value.(string)))
		case "platform":
			opts = append(opts, WithComputePlatform(value.(string)))
		case "worker_images":
			opts = append(opts, WithComputeWorkerImages(computeContractImages(t, value)...))
		case "py_requirements_txt":
			opts = append(opts, WithComputePyRequirementsTxt(value.(string)))
		}
	}
	return opts
}

func computeContractUpdateOptions(t *testing.T, input map[string]any) []ComputeUpdateOption {
	t.Helper()
	var opts []ComputeUpdateOption
	for key, value := range input {
		switch key {
		case "name":
			opts = append(opts, WithComputeUpdateName(value.(string)))
		case "kind":
			opts = append(opts, WithComputeUpdateKind(value.(string)))
		case "description":
			opts = append(opts, WithComputeUpdateDescription(value.(string)))
		case "spec_id":
			opts = append(opts, WithComputeUpdateSpecID(value.(string)))
		case "cpu":
			opts = append(opts, WithComputeUpdateCPU(int(value.(float64))))
		case "memory_gib":
			opts = append(opts, WithComputeUpdateMemoryGiB(int(value.(float64))))
		case "gpu":
			opts = append(opts, WithComputeUpdateGPU(int(value.(float64))))
		case "min_replicas":
			opts = append(opts, WithComputeUpdateMinReplicas(int(value.(float64))))
		case "max_replicas":
			opts = append(opts, WithComputeUpdateMaxReplicas(int(value.(float64))))
		case "auto_suspend_minutes":
			opts = append(opts, WithComputeUpdateAutoSuspendMinutes(int(value.(float64))))
		case "go_worker_image_id":
			opts = append(opts, WithComputeUpdateGoWorkerImageID(value.(string)))
		case "python_worker_image_id":
			opts = append(opts, WithComputeUpdatePythonWorkerImageID(value.(string)))
		case "platform":
			opts = append(opts, WithComputeUpdatePlatform(value.(string)))
		case "worker_images":
			opts = append(opts, WithComputeUpdateWorkerImages(computeContractImages(t, value)...))
		case "py_requirements_txt":
			opts = append(opts, WithComputeUpdatePyRequirementsTxt(value.(string)))
		}
	}
	return opts
}
