package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
)

// IngressVerificationResult preserves the two owner-defined success shapes:
// an updated connection or a rejected channel probe. Exactly one is present.
type IngressVerificationResult struct {
	Connection *IngressConnection
	TestResult *ChannelInstanceTestResult
	raw        json.RawMessage
}

func (result *IngressVerificationResult) UnmarshalJSON(data []byte) error {
	result.raw = bytes.Clone(data)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	probeKeys := []string{"ok", "status", "last_test_status", "last_tested_at"}
	present := 0
	for _, key := range probeKeys {
		if _, exists := fields[key]; exists {
			present++
		}
	}
	if present == len(probeKeys) {
		var tested ChannelInstanceTestResult
		if err := json.Unmarshal(data, &tested); err != nil {
			return err
		}
		result.Connection = nil
		result.TestResult = &tested
		return nil
	}
	if present != 0 {
		return fmt.Errorf("ingress verification probe response is missing required fields")
	}
	connection := &IngressConnection{}
	if err := protojson.Unmarshal(data, connection); err != nil {
		return err
	}
	result.Connection = connection
	result.TestResult = nil
	return nil
}

func (result IngressVerificationResult) MarshalJSON() ([]byte, error) {
	if len(result.raw) != 0 {
		return bytes.Clone(result.raw), nil
	}
	if result.TestResult != nil {
		if result.Connection != nil {
			return nil, fmt.Errorf("ingress verification result cannot contain both connection and probe")
		}
		return json.Marshal(result.TestResult)
	}
	if result.Connection == nil {
		return nil, fmt.Errorf("ingress verification result is empty")
	}
	return protojson.Marshal(result.Connection)
}
