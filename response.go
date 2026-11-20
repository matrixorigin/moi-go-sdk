package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type productResponse struct {
	Code      any             `json:"code"`
	ErrorCode string          `json:"error_code,omitempty"`
	Message   string          `json:"msg,omitempty"`
	Msg       string          `json:"message,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Details   ErrorDetails    `json:"details,omitempty"`
}

// productEnvelopeRequired is used by endpoints whose public contract is an
// explicit Product envelope rather than a legacy raw response.
type productEnvelopeRequired interface {
	requireProductEnvelope()
}

type requiredProductResult struct{ value any }

func (requiredProductResult) requireProductEnvelope() {}

// productNullableData declares an endpoint's explicit null result contract.
// It never permits an envelope that omits data.
type productNullableData interface {
	acceptsProductNull()
}

func decodeProductMessageList[T proto.Message](data []byte, newItem func() T) ([]T, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("product list data must be an array")
	}
	items := make([]T, 0, len(raw))
	for index, value := range raw {
		item := newItem()
		if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(value, item); err != nil {
			return nil, fmt.Errorf("product list item %d: %w", index, err)
		}
		items = append(items, item)
	}
	return items, nil
}

func parseProductHTTPResponse(status int, body []byte, result any, requestID string) error {
	// Raw resource operations can acknowledge success with HTTP 204. Typed
	// results still require their data, and an empty HTTP 200 stays invalid.
	if status == http.StatusNoContent && len(body) == 0 {
		if out, ok := result.(*ProductData); ok {
			out.Value = nil
			return nil
		}
	}
	err := parseProductResponse(body, result, requestID)
	var failure *Error
	if errors.As(err, &failure) {
		failure.Status = status
	}
	return err
}

func parseProductResponse(body []byte, result any, requestID string) error {
	_, requireEnvelope := result.(productEnvelopeRequired)
	wrapped, strictObject := result.(requiredProductResult)
	if strictObject {
		result = wrapped.value
	}
	invalid := func(cause error) error {
		return &Error{Code: "ErrResponseInvalid", Message: "product response is not a valid envelope", RequestID: requestID, cause: cause}
	}
	if len(body) == 0 {
		if requireEnvelope {
			return invalid(nil)
		}
		if result != nil {
			return &Error{Code: "ErrResponseInvalid", Message: "product response is empty", RequestID: requestID}
		}
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		if requireEnvelope {
			return &Error{Code: "ErrResponseInvalid", Message: "product response is not a valid envelope", RequestID: requestID}
		}
		return unmarshalRawProductResponse(body, result)
	}
	if !isProductEnvelope(fields) {
		if requireEnvelope {
			return &Error{Code: "ErrResponseInvalid", Message: "product response is missing envelope", RequestID: requestID}
		}
		return unmarshalRawProductResponse(body, result)
	}

	var envelope productResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return invalid(err)
	}
	if requireEnvelope {
		switch code := envelope.Code.(type) {
		case string:
			if code == "" {
				return invalid(nil)
			}
		case float64:
			if math.Trunc(code) != code {
				return invalid(nil)
			}
		default:
			return invalid(nil)
		}
		for _, key := range []string{"msg", "message"} {
			if raw, exists := fields[key]; exists {
				var value *string
				if json.Unmarshal(raw, &value) != nil || value == nil {
					return invalid(nil)
				}
			}
		}
	}
	if !isSuccessCode(envelope.Code) {
		return &Error{
			Code:      codeString(envelope.Code),
			Message:   firstNonEmpty(envelope.Message, envelope.Msg),
			RequestID: requestID,
			Details:   envelope.Details,
			Data:      envelope.Data,
		}
	}
	if result == nil {
		return nil
	}
	if string(envelope.Data) == "null" {
		if _, ok := result.(productNullableData); ok {
			return unmarshalProductResponseData(envelope.Data, result, "data")
		}
		return &Error{Code: "ErrResponseInvalid", Message: "product response data must not be null", RequestID: requestID, Details: envelope.Details}
	}
	if len(envelope.Data) == 0 {
		return &Error{
			Code:      "ErrResponseInvalid",
			Message:   "product response envelope missing data",
			RequestID: requestID,
			Details:   envelope.Details,
		}
	}
	if strictObject {
		var object map[string]json.RawMessage
		if json.Unmarshal(envelope.Data, &object) != nil || object == nil {
			return invalid(nil)
		}
	}
	err := unmarshalProductResponseData(envelope.Data, result, "data")
	if err != nil && requireEnvelope {
		return invalid(err)
	}
	return err
}

func isProductEnvelope(fields map[string]json.RawMessage) bool {
	if _, ok := fields["code"]; !ok {
		return false
	}
	for _, key := range []string{"data", "msg", "message", "details"} {
		if _, ok := fields[key]; ok {
			return true
		}
	}
	return false
}

func unmarshalRawProductResponse(body []byte, result any) error {
	if result == nil {
		return nil
	}
	return unmarshalProductResponseData(body, result, "response")
}

func unmarshalProductResponseData(data []byte, result any, label string) error {
	switch out := result.(type) {
	case *product.AgentCardResult:
		out.Json = string(data)
		return nil
	case *product.AgentA2AResult:
		out.Json = string(data)
		return nil
	case *product.WorkflowTemplateList:
		if err := unmarshalWorkflowTemplateList(data, out, label); err != nil {
			return err
		}
		return nil
	case *parserBackendList:
		if err := unmarshalParserBackendList(data, out, label); err != nil {
			return err
		}
		return nil
	case *parserEndpointList:
		if err := unmarshalParserEndpointList(data, out, label); err != nil {
			return err
		}
		return nil
	case *product.WorkitemCatalogDetail:
		wrapped := []byte(`{"item":` + string(data) + `}`)
		options := protojson.UnmarshalOptions{DiscardUnknown: false}
		if err := options.Unmarshal(wrapped, out); err != nil {
			return fmt.Errorf("unmarshal product response %s: %w", label, err)
		}
		return nil
	}
	if msg, ok := result.(proto.Message); ok {
		options := protojson.UnmarshalOptions{DiscardUnknown: false}
		if err := options.Unmarshal(data, msg); err != nil {
			return fmt.Errorf("unmarshal product response %s: %w", label, err)
		}
		return nil
	}
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("unmarshal product response %s: %w", label, err)
	}
	return nil
}

func unmarshalWorkflowTemplateList(data []byte, out *product.WorkflowTemplateList, label string) error {
	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return fmt.Errorf("unmarshal product response %s: %w", label, err)
	}
	out.Templates = make([]*product.WorkflowTemplate, 0, len(rawItems))
	options := protojson.UnmarshalOptions{DiscardUnknown: false}
	for i, raw := range rawItems {
		item := &product.WorkflowTemplate{}
		if err := options.Unmarshal(raw, item); err != nil {
			return fmt.Errorf("unmarshal product response %s[%d]: %w", label, i, err)
		}
		out.Templates = append(out.Templates, item)
	}
	return nil
}

func unmarshalParserBackendList(data []byte, out *parserBackendList, label string) error {
	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return fmt.Errorf("unmarshal product response %s: %w", label, err)
	}
	out.backends = make([]*ProviderBackend, 0, len(rawItems))
	options := protojson.UnmarshalOptions{DiscardUnknown: false}
	for i, raw := range rawItems {
		backend := &ProviderBackend{}
		if err := options.Unmarshal(raw, backend); err != nil {
			return fmt.Errorf("unmarshal product response %s[%d]: %w", label, i, err)
		}
		out.backends = append(out.backends, backend)
	}
	return nil
}

func unmarshalParserEndpointList(data []byte, out *parserEndpointList, label string) error {
	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return fmt.Errorf("unmarshal product response %s: %w", label, err)
	}
	out.endpoints = make([]*ProviderEndpoint, 0, len(rawItems))
	options := protojson.UnmarshalOptions{DiscardUnknown: false}
	for i, raw := range rawItems {
		endpoint := &ProviderEndpoint{}
		if err := options.Unmarshal(raw, endpoint); err != nil {
			return fmt.Errorf("unmarshal product response %s[%d]: %w", label, i, err)
		}
		out.endpoints = append(out.endpoints, endpoint)
	}
	return nil
}

func parseHTTPError(status int, body []byte, requestID string) error {
	invalid := &Error{Code: "ErrResponseInvalid", Message: "product error response is invalid", Status: status, RequestID: requestID}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil || fields == nil {
		return invalid
	}
	if _, exists := fields["jsonrpc"]; exists {
		var rpc struct {
			JSONRPC string `json:"jsonrpc"`
			Error   *struct {
				Code    *int            `json:"code"`
				Message string          `json:"message"`
				Data    json.RawMessage `json:"data"`
			} `json:"error"`
		}
		_, hasID := fields["id"]
		_, hasResult := fields["result"]
		var id any
		validID := false
		if json.Unmarshal(fields["id"], &id) == nil {
			switch id.(type) {
			case nil, string, float64:
				validID = true
			}
		}
		if json.Unmarshal(body, &rpc) != nil || rpc.JSONRPC != "2.0" || !hasID || !validID || hasResult || rpc.Error == nil || rpc.Error.Code == nil || rpc.Error.Message == "" {
			return invalid
		}
		return &Error{Code: strconv.Itoa(*rpc.Error.Code), Message: rpc.Error.Message, Status: status, RequestID: requestID, Data: rpc.Error.Data}
	}
	_, productCode := fields["code"]
	_, runtimeCode := fields["error_code"]
	var proxyCode string
	if !productCode && !runtimeCode && json.Unmarshal(fields["error"], &proxyCode) == nil && proxyCode == "policy_denied" {
		var detail string
		if status != http.StatusForbidden || json.Unmarshal(fields["detail"], &detail) != nil || detail == "" {
			return invalid
		}
		// OpenShell's proxy rejected the request before the Product API. Keep
		// its machine reason distinct from Catalog IAM and raw detail out of
		// the summary. Bound the single diagnostic field, not the error code.
		details := ErrorDetails{"domain": "sandbox_network", "reason": proxyCode}
		if len(detail) <= 16*1024 {
			details["detail"] = detail
		} else {
			details["detail_omitted"] = true
		}
		return &Error{Code: proxyCode, Message: "sandbox network policy denied the request", Status: status, RequestID: requestID, Details: details}
	}
	if raw, exists := fields["error"]; !productCode && exists && len(raw) > 0 && raw[0] == '{' {
		var nested struct {
			Error *struct {
				Code      any    `json:"code"`
				Message   string `json:"message"`
				RequestID string `json:"request_id"`
				Retryable *bool  `json:"retryable"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &nested) != nil || nested.Error == nil || nested.Error.Message == "" {
			return invalid
		}
		switch code := nested.Error.Code.(type) {
		case string:
			if code == "" {
				return invalid
			}
		case float64:
			if math.Trunc(code) != code {
				return invalid
			}
		default:
			return invalid
		}
		details := map[string]any{}
		if nested.Error.Retryable != nil {
			details["retryable"] = fmt.Sprint(*nested.Error.Retryable)
		}
		return &Error{Code: codeString(nested.Error.Code), Message: nested.Error.Message, Status: status, RequestID: firstNonEmpty(requestID, nested.Error.RequestID), Details: details}
	}
	var envelope productResponse
	if json.Unmarshal(body, &envelope) != nil {
		return invalid
	}
	for _, key := range []string{"msg", "message"} {
		if raw, exists := fields[key]; exists {
			var value *string
			if json.Unmarshal(raw, &value) != nil || value == nil {
				return invalid
			}
		}
	}
	message := firstNonEmpty(envelope.Message, envelope.Msg)
	// Some published runtime endpoints return error_code without a Product
	// code or message. Preserve that explicit contract, not arbitrary bodies.
	if envelope.ErrorCode != "" {
		return &Error{Code: envelope.ErrorCode, Message: firstNonEmpty(message, http.StatusText(status)), Status: status, RequestID: requestID, Details: envelope.Details, Data: envelope.Data}
	}
	if !isProductEnvelope(fields) || isSuccessCode(envelope.Code) {
		return invalid
	}
	switch envelope.Code.(type) {
	case string:
	case float64:
		code := envelope.Code.(float64)
		if math.Trunc(code) != code {
			return invalid
		}
	default:
		return invalid
	}
	if message == "" {
		return invalid
	}
	return &Error{Code: codeString(envelope.Code), Message: message, Status: status, RequestID: requestID, Details: envelope.Details, Data: envelope.Data}
}

func isSuccessCode(code any) bool {
	switch v := code.(type) {
	case nil:
		return true
	case string:
		return v == "" || v == "OK" || v == "0" || v == "200"
	case float64:
		return v == 0 || v == 200
	default:
		return false
	}
}

func codeString(code any) string {
	switch v := code.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func responseErrorCode(code string, status int) string {
	if code != "" {
		return code
	}
	return statusCode(status)
}

func statusCode(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "ErrUnauthorized"
	case http.StatusForbidden:
		return "ErrForbidden"
	case http.StatusNotFound:
		return "ErrNotFound"
	case http.StatusConflict:
		return "ErrConflict"
	default:
		return fmt.Sprintf("HTTP%d", status)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
