package sdk

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// FileStream wraps a streaming HTTP response body that callers must close.
//
// FileStream is returned by methods that download files or stream content.
// The caller is responsible for closing the Body to release resources.
//
// Example:
//
//	stream, err := client.DownloadGenAIResult(ctx, "file-id-123")
//	if err != nil {
//		return err
//	}
//	defer stream.Close()
//
//	data, err := io.ReadAll(stream.Body)
type FileStream struct {
	// Body is the response body that must be closed by the caller
	Body io.ReadCloser
	// Header contains the HTTP response headers
	Header http.Header
	// StatusCode is the HTTP status code
	StatusCode int
}

// Close releases the underlying HTTP response body.
//
// This should always be called when done with the FileStream to prevent
// resource leaks. It's safe to call Close multiple times.
func (s *FileStream) Close() error {
	if s == nil || s.Body == nil {
		return nil
	}
	return s.Body.Close()
}

// WriteToFile writes the stream content to a file at the specified path.
//
// The method creates the file and any necessary parent directories.
// It returns the number of bytes written and any error encountered.
//
// Example:
//
//	stream, err := client.DownloadTableData(ctx, &sdk.TableDownloadDataRequest{
//		ID: 1,
//	})
//	if err != nil {
//		return err
//	}
//	defer stream.Close()
//
//	written, err := stream.WriteToFile("/path/to/output.csv")
//	if err != nil {
//		return err
//	}
//	fmt.Printf("Wrote %d bytes to file\n", written)
func (s *FileStream) WriteToFile(filePath string) (int64, error) {
	if s == nil || s.Body == nil {
		return 0, io.ErrUnexpectedEOF
	}

	// Create parent directories if they don't exist
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return 0, err
		}
	}

	// Create the file
	file, err := os.Create(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Copy the stream content to the file
	written, err := io.Copy(file, s.Body)
	if err != nil {
		return written, err
	}

	return written, nil
}
