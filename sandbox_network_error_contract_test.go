package sdk

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSandboxNetworkErrorsMatchSharedContract(t *testing.T) {
	var cases []struct {
		Name            string
		Status          int
		DetailBytes     int `json:"detail_bytes"`
		Response        map[string]any
		ExpectedCode    string       `json:"expected_code"`
		ExpectedDetails ErrorDetails `json:"expected_details"`
	}
	data, err := os.ReadFile("testdata/sandbox_network_error_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			if scenario.DetailBytes != 0 {
				detail := strings.Repeat("x", scenario.DetailBytes)
				scenario.Response["detail"] = detail
				if scenario.ExpectedDetails["detail_omitted"] != true {
					scenario.ExpectedDetails["detail"] = detail
				}
			}
			body, err := json.Marshal(scenario.Response)
			if err != nil {
				t.Fatal(err)
			}
			var failure *Error
			if err := parseHTTPError(scenario.Status, body, "transport-request"); !errors.As(err, &failure) {
				t.Fatalf("unstructured error: %v", err)
			}
			if failure.Code != scenario.ExpectedCode || failure.Status != scenario.Status || failure.RequestID != "transport-request" || !reflect.DeepEqual(failure.Details, scenario.ExpectedDetails) && !(len(failure.Details) == 0 && len(scenario.ExpectedDetails) == 0) {
				t.Fatalf("error contract lost: %#v; want code=%s details=%#v", failure, scenario.ExpectedCode, scenario.ExpectedDetails)
			}
			if failure.Message == "" || failure.Message == scenario.Response["detail"] || strings.Contains(failure.Message, "RAW-SECRET") {
				t.Fatalf("unsafe summary: %q", failure.Message)
			}
		})
	}
}
