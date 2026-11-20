package sdk

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestProductErrorsPreserveOwnerRepairData(t *testing.T) {
	const body = `{"code":"ErrValidation","message":"A value is required","data":{"diagnostics":[{"field":"name","reason":"required"}]}}`
	for _, status := range []int{200, 422} {
		err := parseProductHTTPResponse(status, []byte(body), nil, "request-1")
		var failure *Error
		if !errors.As(err, &failure) || failure.Code != "ErrValidation" || failure.Message != "A value is required" || failure.RequestID != "request-1" {
			t.Fatalf("error contract changed: %v", err)
		}
		var actual map[string]any
		if err := json.Unmarshal(failure.Data, &actual); err != nil {
			t.Fatalf("HTTP %d discarded the owner's repair data: %v", status, err)
		}
		expected := map[string]any{"diagnostics": []any{map[string]any{"field": "name", "reason": "required"}}}
		if !reflect.DeepEqual(expected, actual) {
			t.Fatalf("repair data changed: %#v", actual)
		}
	}
}

type envelopeRequiredResponse struct{}

func (*envelopeRequiredResponse) requireProductEnvelope() {}

func TestParseProductResponseBusinessErrorUsesHeaderRequestID(t *testing.T) {
	err := parseProductResponse([]byte(`{"code":"ErrConflict","msg":"name already exists","details":{"request_id":"body-id"}}`), nil, "header-id")
	var productErr *Error
	if !errors.As(err, &productErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if productErr.RequestID != "header-id" {
		t.Fatalf("expected header request ID, got %q", productErr.RequestID)
	}
}

func TestParseProductResponseDoesNotUseBodyRequestIDAsTransportID(t *testing.T) {
	err := parseProductResponse([]byte(`{"code":"ErrConflict","msg":"name already exists","details":{"request_id":"body-id"}}`), nil, "")
	var productErr *Error
	if !errors.As(err, &productErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if productErr.RequestID != "" {
		t.Fatalf("expected no transport request ID, got %q", productErr.RequestID)
	}
}

func TestParseProductResponseInvalidRequiredEnvelopeUsesHeaderRequestID(t *testing.T) {
	err := parseProductResponse([]byte(`{"unexpected":"response"}`), &envelopeRequiredResponse{}, "header-id")
	var productErr *Error
	if !errors.As(err, &productErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if productErr.Code != "ErrResponseInvalid" {
		t.Fatalf("expected ErrResponseInvalid, got %q", productErr.Code)
	}
	if productErr.RequestID != "header-id" {
		t.Fatalf("expected header request ID, got %q", productErr.RequestID)
	}
}

func TestParseHTTPErrorPreservesLegacyErrorCodeEnvelope(t *testing.T) {
	err := parseHTTPError(403, []byte(`{"error_code":"runtime_artifact_upload_denied"}`), "header-id")
	var productErr *Error
	if !errors.As(err, &productErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if productErr.Code != "runtime_artifact_upload_denied" || productErr.Message != "Forbidden" || productErr.RequestID != "header-id" {
		t.Fatalf("error = %#v", productErr)
	}
}
