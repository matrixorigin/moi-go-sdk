package sdk

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultUserAgent   = "matrixflow-sdk-go/0.1.0"
	defaultHTTPTimeout = 30 * time.Second
)

type clientOptions struct {
	httpClient     *http.Client
	userAgent      string
	defaultHeaders http.Header
}

// ClientOption customizes the SDK client during construction.
type ClientOption func(*clientOptions)

// WithHTTPClient overrides the default http.Client used by the SDK.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(o *clientOptions) {
		if client != nil {
			o.httpClient = client
		}
	}
}

// WithHTTPTimeout configures the timeout on the underlying http.Client.
func WithHTTPTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		if timeout <= 0 {
			return
		}
		if o.httpClient == nil {
			o.httpClient = &http.Client{}
		}
		o.httpClient.Timeout = timeout
	}
}

// WithUserAgent overrides the default User-Agent header that is sent with every request.
func WithUserAgent(userAgent string) ClientOption {
	return func(o *clientOptions) {
		ua := strings.TrimSpace(userAgent)
		if ua != "" {
			o.userAgent = ua
		}
	}
}

// WithDefaultHeader adds a header that will be included on every request.
func WithDefaultHeader(key, value string) ClientOption {
	return func(o *clientOptions) {
		if key == "" {
			return
		}
		if o.defaultHeaders == nil {
			o.defaultHeaders = make(http.Header)
		}
		o.defaultHeaders.Add(key, value)
	}
}

// WithDefaultHeaders merges a set of headers that will be included on every request.
func WithDefaultHeaders(headers http.Header) ClientOption {
	return func(o *clientOptions) {
		if len(headers) == 0 {
			return
		}
		if o.defaultHeaders == nil {
			o.defaultHeaders = make(http.Header)
		}
		mergeHeaders(o.defaultHeaders, headers, false)
	}
}

// CallOption customizes individual SDK operations.
type CallOption func(*callOptions)

type callOptions struct {
	headers   http.Header
	query     url.Values
	requestID string
}

func newCallOptions(opts ...CallOption) callOptions {
	co := callOptions{
		headers: make(http.Header),
		query:   make(url.Values),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&co)
		}
	}
	return co
}

// WithRequestID sets the X-Request-ID header on the outgoing request.
func WithRequestID(id string) CallOption {
	return func(co *callOptions) {
		co.requestID = strings.TrimSpace(id)
	}
}

// WithHeader sets or overrides a header on the outgoing request.
func WithHeader(key, value string) CallOption {
	return func(co *callOptions) {
		if key == "" {
			return
		}
		if co.headers == nil {
			co.headers = make(http.Header)
		}
		co.headers.Set(key, value)
	}
}

// WithHeaders merges headers into the outgoing request.
func WithHeaders(headers http.Header) CallOption {
	return func(co *callOptions) {
		if len(headers) == 0 {
			return
		}
		if co.headers == nil {
			co.headers = make(http.Header)
		}
		mergeHeaders(co.headers, headers, false)
	}
}

// WithQueryParam appends a single query parameter to the request URL.
func WithQueryParam(key, value string) CallOption {
	return func(co *callOptions) {
		if key == "" {
			return
		}
		if co.query == nil {
			co.query = make(url.Values)
		}
		co.query.Add(key, value)
	}
}

// WithQuery merges an entire query parameter map into the request URL.
func WithQuery(values url.Values) CallOption {
	return func(co *callOptions) {
		if len(values) == 0 {
			return
		}
		if co.query == nil {
			co.query = make(url.Values)
		}
		for key, vv := range values {
			for _, v := range vv {
				co.query.Add(key, v)
			}
		}
	}
}

func cloneHeader(src http.Header) http.Header {
	if len(src) == 0 {
		return make(http.Header)
	}
	dst := make(http.Header, len(src))
	for k, vv := range src {
		copied := make([]string, len(vv))
		copy(copied, vv)
		dst[k] = copied
	}
	return dst
}

func mergeHeaders(dst, src http.Header, override bool) {
	if len(src) == 0 {
		return
	}
	for k, vv := range src {
		if override {
			dst.Del(k)
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
