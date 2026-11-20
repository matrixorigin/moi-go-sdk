package product

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestComputeResourceAllowedActionsContract(t *testing.T) {
	field := (&ComputeResource{}).ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name("allowed_actions"))
	if field == nil {
		t.Fatal("ComputeResource.allowed_actions is missing")
	}
	if field.Number() != 33 || field.JSONName() != "allowed_actions" || !field.IsList() || field.Kind() != protoreflect.StringKind {
		t.Fatalf("unexpected allowed_actions descriptor: number=%d json=%q list=%t kind=%s", field.Number(), field.JSONName(), field.IsList(), field.Kind())
	}
	if adminField := (&AdminComputeResource{}).ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name("allowed_actions")); adminField != nil {
		t.Fatal("AdminComputeResource must not expose workspace allowed_actions")
	}
}
