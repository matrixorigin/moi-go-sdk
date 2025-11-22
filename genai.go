package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

// PipelineFile represents a single file to be uploaded when creating a GenAI pipeline.
type PipelineFile struct {
	FileName string
	Reader   io.Reader
}

func (c *RawClient) CreateGenAIPipeline(ctx context.Context, req *GenAICreatePipelineRequest, files []PipelineFile, opts ...CallOption) (*GenAICreatePipelineResponse, error) {
	if len(files) == 0 {
		if req == nil {
			return nil, ErrNilRequest
		}
		var resp GenAICreatePipelineResponse
		if err := c.postJSON(ctx, "/v1/genai/pipeline", req, &resp, opts...); err != nil {
			return nil, err
		}
		return &resp, nil
	}

	if req == nil {
		return nil, ErrNilRequest
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	contentType := writer.FormDataContentType()

	go func() {
		defer pw.Close()
		defer writer.Close()

		payload, err := json.Marshal(req)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := writer.WriteField("payload", string(payload)); err != nil {
			pw.CloseWithError(err)
			return
		}
		if len(req.FileNames) > 0 {
			for _, name := range req.FileNames {
				if err := writer.WriteField("file_names", name); err != nil {
					pw.CloseWithError(err)
					return
				}
			}
		}

		for i, file := range files {
			if file.Reader == nil {
				pw.CloseWithError(fmt.Errorf("file reader at index %d is nil", i))
				return
			}
			filename := file.FileName
			if strings.TrimSpace(filename) == "" {
				filename = fmt.Sprintf("file_%d", i)
			}
			part, err := writer.CreateFormFile("files", filename)
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			if _, err := io.Copy(part, file.Reader); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
	}()

	callOpts := newCallOptions(opts...)
	resp, err := c.doRaw(ctx, http.MethodPost, "/v1/genai/pipeline", pr, callOpts, func(r *http.Request) {
		r.Header.Set(headerContentType, contentType)
		r.Header.Set(headerAccept, mimeJSON)
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var envelope apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	if envelope.Code != "" && envelope.Code != "OK" {
		return nil, &APIError{
			Code:       envelope.Code,
			Message:    envelope.Msg,
			RequestID:  envelope.RequestID,
			HTTPStatus: resp.StatusCode,
		}
	}
	var pipelineResp GenAICreatePipelineResponse
	if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, &pipelineResp); err != nil {
			return nil, err
		}
	}
	return &pipelineResp, nil
}

func (c *RawClient) GetGenAIJob(ctx context.Context, jobID string, opts ...CallOption) (*GenAIGetJobDetailResponse, error) {
	if strings.TrimSpace(jobID) == "" {
		return nil, fmt.Errorf("jobID cannot be empty")
	}
	var resp GenAIGetJobDetailResponse
	path := fmt.Sprintf("/v1/genai/jobs/%s", url.PathEscape(jobID))
	if err := c.getJSON(ctx, path, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RawClient) DownloadGenAIResult(ctx context.Context, fileID string, opts ...CallOption) (*FileStream, error) {
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("fileID cannot be empty")
	}
	callOpts := newCallOptions(opts...)
	path := fmt.Sprintf("/v1/genai/results/file/%s", url.PathEscape(fileID))
	resp, err := c.doRaw(ctx, http.MethodGet, path, nil, callOpts, nil)
	if err != nil {
		return nil, err
	}
	return &FileStream{
		Body:       resp.Body,
		Header:     resp.Header.Clone(),
		StatusCode: resp.StatusCode,
	}, nil
}
