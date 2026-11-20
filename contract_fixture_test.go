package sdk

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// readContractFixture loads one wire-shaped list item from sdk/go-sdk/testdata.
func readContractFixture(t *testing.T, name string) json.RawMessage {
	t.Helper()
	value, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return value
}

// assertProductMessageFieldNumbers pins the wire contract numbering of a
// product message; every declared field is listed with its number.
func assertProductMessageFieldNumbers(t *testing.T, message proto.Message, expected map[string]int) {
	t.Helper()
	fields := message.ProtoReflect().Descriptor().Fields()
	for name, number := range expected {
		field := fields.ByName(protoreflect.Name(name))
		if field == nil || int(field.Number()) != number {
			t.Fatalf("field %s number = %v, want %d", name, field, number)
		}
	}
}

// injectUnknownContractField appends an unregistered key to a wire item so
// the strict decoder must reject it.
func injectUnknownContractField(t *testing.T, fixture string) string {
	t.Helper()
	raw := strings.TrimSpace(string(readContractFixture(t, fixture)))
	return strings.TrimSuffix(raw, "}") + `,"unregistered_field":true}`
}
