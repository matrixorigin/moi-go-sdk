package sdk

import (
	"errors"
	"fmt"
)

var (
	// ErrBaseURLRequired indicates that NewSDKClient was called without a base URL.
	ErrBaseURLRequired = errors.New("sdk: baseURL is required")
	// ErrAPIKeyRequired indicates that NewSDKClient was called without an API key.
	ErrAPIKeyRequired = errors.New("sdk: apiKey is required")
	// ErrNilRequest indicates that a required request payload was nil.
	ErrNilRequest = errors.New("sdk: request payload cannot be nil")
)

// APIError captures an application-level error returned by the catalog service envelope.
type APIError struct {
	Code       string
	Message    string
	RequestID  string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("catalog service error: code=%s msg=%s request_id=%s status=%d", e.Code, e.Message, e.RequestID, e.HTTPStatus)
}

// HTTPError represents a non-2xx HTTP response that occurred before the SDK could parse the envelope.
type HTTPError struct {
	StatusCode int
	Body       []byte
}

func (e *HTTPError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if len(e.Body) == 0 {
		return fmt.Sprintf("http error: status=%d", e.StatusCode)
	}
	return fmt.Sprintf("http error: status=%d body=%s", e.StatusCode, string(e.Body))
}
