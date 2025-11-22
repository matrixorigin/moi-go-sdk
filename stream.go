package sdk

import (
	"io"
	"net/http"
)

// FileStream wraps a streaming HTTP response body that callers must close.
type FileStream struct {
	Body       io.ReadCloser
	Header     http.Header
	StatusCode int
}

// Close releases the underlying HTTP response body.
func (s *FileStream) Close() error {
	if s == nil || s.Body == nil {
		return nil
	}
	return s.Body.Close()
}
