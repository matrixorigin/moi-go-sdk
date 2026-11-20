package sdk

import (
	"encoding/json"
	"testing"
)

func TestProductDataJSONPreservesTheProductValue(t *testing.T) {
	for _, input := range []string{`{"tools":[{"tool_id":"published-tool"}],"dry_run":true}`, `["first","second"]`, `"value"`, `7`, `null`} {
		var result ProductData
		if err := json.Unmarshal([]byte(input), &result); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		var before, after any
		if err := json.Unmarshal([]byte(input), &before); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(encoded, &after); err != nil {
			t.Fatal(err)
		}
		left, _ := json.Marshal(before)
		right, _ := json.Marshal(after)
		if string(left) != string(right) {
			t.Fatalf("product JSON changed: %s -> %s", input, encoded)
		}
	}
}

func TestProductDataJSONPreservesNumberText(t *testing.T) {
	var result ProductData
	if err := json.Unmarshal([]byte(`{"count":7,"large_id":9007199254740993}`), &result); err != nil {
		t.Fatal(err)
	}
	data, ok := result.AsMap()
	if !ok || data["count"] != json.Number("7") || data["large_id"] != json.Number("9007199254740993") {
		t.Fatalf("product data = %#v", result.Value)
	}
}

func TestProductDataJSONRejectsTrailingValue(t *testing.T) {
	var result ProductData
	if err := json.Unmarshal([]byte(`{"count":1} {}`), &result); err == nil {
		t.Fatalf("trailing JSON was accepted: %#v", result.Value)
	}
}
