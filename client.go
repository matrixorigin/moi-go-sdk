package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const (
	headerAPIKey               = "X-API-Key"
	headerWorkspaceID          = "X-Workspace-ID"
	headerRequestID            = "X-Request-ID"
	headerContentType          = "Content-Type"
	headerRuntimeAuthorization = "X-MOI-Runtime-Authorization"
	contentTypeJSON            = "application/json"
	acceptAny                  = "*/*"
	acceptEventStream          = "text/event-stream"
)

// Client is the MOI product-level SDK client.
type Client struct {
	endpoint                string
	apiKey                  atomic.Pointer[string]
	token                   atomic.Pointer[string]
	sessionCookie           atomic.Pointer[string]
	timeout                 time.Duration
	a2aWaitTimeout          time.Duration
	httpClient              *http.Client
	requestModifier         []RequestModifier
	customHeaders           map[string]string
	workspaceCurrentRoleIDs map[string]string
	personalAccessToken     bool
}

// New creates a Product SDK client for the product API endpoint.
func New(productEndpoint string, opts ...Option) (*Client, error) {
	if productEndpoint == "" {
		return nil, ErrEndpointRequired
	}
	cfg := &config{
		timeout:        60 * time.Second,
		a2aWaitTimeout: defaultA2AWaitTimeout,
		httpClient:     &http.Client{},
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.apiKey == "" && cfg.token == "" && cfg.sessionCookie == "" && !cfg.allowUnauthenticated {
		return nil, ErrAuthRequired
	}
	workspaceCurrentRoleIDs, err := resolveWorkspaceCurrentRoles(cfg.workspaceCurrentRole)
	if err != nil {
		return nil, err
	}
	c := &Client{
		endpoint:                strings.TrimRight(productEndpoint, "/"),
		timeout:                 cfg.timeout,
		a2aWaitTimeout:          cfg.a2aWaitTimeout,
		httpClient:              cfg.httpClient,
		requestModifier:         cfg.requestModifiers,
		customHeaders:           cfg.customHeaders,
		workspaceCurrentRoleIDs: workspaceCurrentRoleIDs,
	}
	c.apiKey.Store(&cfg.apiKey)
	c.token.Store(&cfg.token)
	c.sessionCookie.Store(&cfg.sessionCookie)
	return c, nil
}

// NewWithPersonalAccessToken creates a product SDK client authenticated by a UC
// personal access token. The token is opaque and is sent only as X-API-Key.
func NewWithPersonalAccessToken(productEndpoint, personalAccessToken string, opts ...Option) (*Client, error) {
	if personalAccessToken == "" {
		return nil, ErrAPIKeyRequired
	}
	if strings.TrimSpace(personalAccessToken) != personalAccessToken || strings.ContainsAny(personalAccessToken, "\r\n\t ") {
		return nil, ErrMalformedPersonalAccessToken
	}
	client, err := New(productEndpoint, append(opts, WithAPIKey(personalAccessToken))...)
	if err != nil {
		return nil, err
	}
	if load(&client.token) != "" || load(&client.sessionCookie) != "" {
		return nil, ErrPersonalAccessTokenAuthConflict
	}
	client.personalAccessToken = true
	return client, nil
}

// SetAPIKey updates user-scoped API key authentication for future requests.
func (c *Client) SetAPIKey(apiKey string) error {
	if apiKey == "" {
		return ErrAPIKeyRequired
	}
	c.apiKey.Store(&apiKey)
	empty := ""
	c.token.Store(&empty)
	c.sessionCookie.Store(&empty)
	return nil
}

// Close releases client resources. It is currently a no-op.
func (c *Client) Close() error {
	return nil
}

// Workspaces returns the route-aligned escape hatch workspace service. Prefer typed EnsureWorkspace results and WorkspaceHandle for user-facing workflows.
func (c *Client) Workspaces() *WorkspaceService {
	return &WorkspaceService{client: c}
}

// Account returns the route-aligned escape hatch account service. Prefer typed WorkspaceHandle DB connection helpers for user-facing workflows.
func (c *Client) Account() *AccountService {
	return &AccountService{client: c}
}

// WorkspaceMembers returns the route-aligned escape hatch for canonical member APIs. Prefer typed WorkspaceHandle member methods for user-facing workflows.
func (c *Client) WorkspaceMembers(workspaceID string) *WorkspaceMemberService {
	return &WorkspaceMemberService{workspaceService{client: c, workspaceID: workspaceID}}
}

// IAM returns the route-aligned escape hatch IAM service. Prefer typed WorkspaceHandle IAM methods for user-facing workflows.
func (c *Client) IAM(workspaceID string) *IAMService {
	return &IAMService{workspaceService{client: c, workspaceID: workspaceID}}
}

// Catalog returns the route-aligned escape hatch catalog service. Prefer typed catalog, database, table, volume, folder, and file handles for user-facing workflows.
func (c *Client) Catalog(workspaceID string) *CatalogService {
	return &CatalogService{workspaceService{client: c, workspaceID: workspaceID}}
}

// Knowledge returns the route-aligned escape hatch knowledge service. Prefer typed semantic model and entry handles for user-facing workflows.
func (c *Client) Knowledge(workspaceID string) *KnowledgeService {
	return &KnowledgeService{workspaceService{client: c, workspaceID: workspaceID}}
}

// Workflows returns the route-aligned escape hatch workflow service. Prefer typed workflow, run, operator, template, and WorkItem API handles for user-facing workflows.
func (c *Client) Workflows(workspaceID string) *WorkflowService {
	return &WorkflowService{workspaceService{client: c, workspaceID: workspaceID}}
}

// Agents returns the route-aligned escape hatch agent service. Prefer typed AgentHandle A2A helpers for user-facing workflows.
func (c *Client) Agents(workspaceID string) *AgentService {
	return &AgentService{workspaceService{client: c, workspaceID: workspaceID}}
}

// Webapp returns the route-aligned escape hatch for the anonymous public
// Agent Webapp protocol. Prefer typed AgentHandle helpers for publisher
// user-facing workflows; never attach product credentials to public link calls.
func (c *Client) Webapp() *WebappService { return &WebappService{client: c} }

// Runners manages workspace execution runners.
func (c *Client) Runners(workspaceID string) *RunnerService {
	return &RunnerService{workspaceService{client: c, workspaceID: workspaceID}}
}

// Sandboxes manages workspace execution sandboxes.
func (c *Client) Sandboxes(workspaceID string) *SandboxService {
	return &SandboxService{workspaceService{client: c, workspaceID: workspaceID}}
}

// Memory returns the legacy route-aligned escape hatch consumer memory service.
// Prefer typed ConsumerMemoryHandle methods when using a legacy server.
//
// Deprecated: Current MOI does not serve the legacy consumer-memory/profile API.
func (c *Client) Memory(workspaceID string) *MemoryService {
	return &MemoryService{workspaceService{client: c, workspaceID: workspaceID}}
}

// Models returns the route-aligned escape hatch model/provider service. Prefer typed provider backend and router handles for user-facing workflows.
func (c *Client) Models(workspaceID string) *ModelService {
	return &ModelService{workspaceService{client: c, workspaceID: workspaceID}}
}

// Connectors returns the route-aligned escape hatch connector service. Prefer typed connector, import task, export task, and connector file handles for user-facing workflows.
func (c *Client) Connectors(workspaceID string) *ConnectorService {
	return &ConnectorService{workspaceService{client: c, workspaceID: workspaceID}}
}

// SQL returns the route-aligned escape hatch SQL service. Prefer typed SQL metadata, workbook, query, and statement handles for user-facing workflows.
func (c *Client) SQL(workspaceID string) *SQLService {
	return &SQLService{workspaceService{client: c, workspaceID: workspaceID}}
}

// Compute returns the route-aligned escape hatch compute service. Prefer typed compute resource handles for user-facing workflows.
func (c *Client) Compute(workspaceID string) *ComputeService {
	return &ComputeService{workspaceService{client: c, workspaceID: workspaceID}}
}

// Dashboard returns the route-aligned escape hatch dashboard service. Prefer typed workflow handles for user-facing workflows.
func (c *Client) Dashboard(workspaceID string) *DashboardService {
	return &DashboardService{workspaceService{client: c, workspaceID: workspaceID}}
}

// DataShare returns the route-aligned escape hatch data-share service. Prefer typed publish and subscription handles for user-facing workflows.
func (c *Client) DataShare(workspaceID string) *DataShareService {
	return &DataShareService{workspaceService{client: c, workspaceID: workspaceID}}
}

type workspaceService struct {
	client      *Client
	workspaceID string
}

func (s workspaceService) do(ctx context.Context, method, path string, body, result any) error {
	if s.workspaceID == "" {
		return ErrWorkspaceRequired
	}
	return s.client.do(ctx, method, path, s.workspaceID, body, result)
}

func (s workspaceService) doStream(ctx context.Context, method, path string, body any) (*StreamResponse, error) {
	if s.workspaceID == "" {
		return nil, ErrWorkspaceRequired
	}
	return s.client.doStream(ctx, method, path, s.workspaceID, body)
}

func (s workspaceService) doDownload(ctx context.Context, method, path string, body any) (*StreamResponse, error) {
	if s.workspaceID == "" {
		return nil, ErrWorkspaceRequired
	}
	return s.client.doStreamWithAccept(ctx, method, path, s.workspaceID, body, acceptAny)
}

func (s workspaceService) doRaw(ctx context.Context, method, path string, body io.Reader, contentType, accept string, result any) error {
	if s.workspaceID == "" {
		return ErrWorkspaceRequired
	}
	return s.client.doRaw(ctx, method, path, s.workspaceID, body, contentType, accept, result)
}

// do executes a single product HTTP request. The SDK never retries; callers
// that need retry must implement it at the business layer after confirming safety.
func (c *Client) do(ctx context.Context, method, path, workspaceID string, body, result any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.doOnce(ctx, method, path, workspaceID, body, result)
}

func (c *Client) doOnce(ctx context.Context, method, path, workspaceID string, body, result any) error {
	return c.doURL(ctx, method, c.productURL(path), workspaceID, body, result)
}

func (c *Client) doURL(ctx context.Context, method, endpointURL, workspaceID string, body, result any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal product request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpointURL, bodyReader)
	if err != nil {
		return fmt.Errorf("create product request: %w", err)
	}
	c.applyAuth(req)
	if workspaceID != "" {
		req.Header.Set(headerWorkspaceID, workspaceID)
	}
	req.Header.Set("Accept", contentTypeJSON)
	if body != nil {
		req.Header.Set(headerContentType, contentTypeJSON)
	}
	for k, v := range c.customHeaders {
		req.Header.Set(k, v)
	}
	for _, modifier := range c.requestModifier {
		modifier(req)
	}
	c.applyWorkspaceScope(req.Header, workspaceID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return productTransportError(ctx, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return productResponseReadError(ctx, resp.StatusCode, resp.Header.Get(headerRequestID), err)
	}
	if resp.StatusCode >= 400 {
		return parseHTTPError(resp.StatusCode, respBody, resp.Header.Get(headerRequestID))
	}
	return parseProductHTTPResponse(resp.StatusCode, respBody, result, resp.Header.Get(headerRequestID))
}

// StreamResponse is a live product streaming response. Callers own Body and must
// consume or explicitly hand off partial stream state, then close it after reading.
// It is not task completion signal, AI task-selection source, or copy anchor by itself.
type StreamResponse struct {
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
}

type streamBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *streamBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func (c *Client) doStream(ctx context.Context, method, path, workspaceID string, body any) (*StreamResponse, error) {
	return c.doStreamWithAccept(ctx, method, path, workspaceID, body, acceptEventStream)
}

func (c *Client) doStreamWithAccept(ctx context.Context, method, path, workspaceID string, body any, accept string) (*StreamResponse, error) {
	return c.doStreamURL(ctx, method, c.productURL(path), workspaceID, body, accept, false)
}

func (c *Client) doStreamURL(ctx context.Context, method, endpointURL, workspaceID string, body any, accept string, noRedirect bool) (*StreamResponse, error) {
	return c.doStreamURLWithAuthentication(ctx, method, endpointURL, workspaceID, body, accept, noRedirect, true)
}

func (c *Client) doStreamURLWithAuthentication(ctx context.Context, method, endpointURL, workspaceID string, body any, accept string, noRedirect bool, authenticate bool) (*StreamResponse, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal product request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	requestCtx := ctx
	var cancel context.CancelFunc
	if c.timeout > 0 {
		requestCtx, cancel = context.WithCancel(ctx)
	}
	req, err := http.NewRequestWithContext(requestCtx, method, endpointURL, bodyReader)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, fmt.Errorf("create product stream request: %w", err)
	}
	if authenticate {
		c.applyAuth(req)
	}
	if workspaceID != "" {
		req.Header.Set(headerWorkspaceID, workspaceID)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if body != nil {
		req.Header.Set(headerContentType, contentTypeJSON)
	}
	for k, v := range c.customHeaders {
		req.Header.Set(k, v)
	}
	for _, modifier := range c.requestModifier {
		modifier(req)
	}
	c.applyWorkspaceScope(req.Header, workspaceID)
	if !authenticate {
		clearPublicProductHeaders(req.Header)
	}
	httpClient := c.httpClient
	if !authenticate && httpClient.Jar != nil {
		httpClone := *httpClient
		httpClone.Jar = nil
		httpClient = &httpClone
	}
	resp, err := c.doStreamRequest(httpClient, req, cancel, noRedirect)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, productTransportError(ctx, err)
	}
	if resp.StatusCode >= 400 {
		if cancel != nil {
			defer cancel()
		}
		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, productResponseReadError(ctx, resp.StatusCode, resp.Header.Get(headerRequestID), err)
		}
		return nil, parseHTTPError(resp.StatusCode, respBody, resp.Header.Get(headerRequestID))
	}
	responseBody := resp.Body
	if cancel != nil {
		responseBody = &streamBody{ReadCloser: resp.Body, cancel: cancel}
	}
	return &StreamResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       responseBody,
	}, nil
}

func (c *Client) doStreamRequest(httpClient *http.Client, req *http.Request, cancel context.CancelFunc, noRedirect bool) (*http.Response, error) {
	if noRedirect {
		redirected := *httpClient
		redirected.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		httpClient = &redirected
	}
	if c.timeout <= 0 {
		return httpClient.Do(req)
	}
	result := make(chan struct {
		resp *http.Response
		err  error
	}, 1)
	go func() {
		resp, err := httpClient.Do(req)
		result <- struct {
			resp *http.Response
			err  error
		}{resp: resp, err: err}
	}()
	timer := time.NewTimer(c.timeout)
	defer timer.Stop()
	select {
	case out := <-result:
		return out.resp, out.err
	case <-req.Context().Done():
		return nil, req.Context().Err()
	case <-timer.C:
		cancel()
		return nil, context.DeadlineExceeded
	}
}

func (c *Client) doRaw(ctx context.Context, method, path, workspaceID string, body io.Reader, contentType, accept string, result any) error {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, method, c.productURL(path), body)
	if err != nil {
		return fmt.Errorf("create product raw request: %w", err)
	}
	c.applyAuth(req)
	if workspaceID != "" {
		req.Header.Set(headerWorkspaceID, workspaceID)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if contentType != "" {
		req.Header.Set(headerContentType, contentType)
	}
	for k, v := range c.customHeaders {
		req.Header.Set(k, v)
	}
	for _, modifier := range c.requestModifier {
		modifier(req)
	}
	c.applyWorkspaceScope(req.Header, workspaceID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return productTransportError(ctx, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return productResponseReadError(ctx, resp.StatusCode, resp.Header.Get(headerRequestID), err)
	}
	if resp.StatusCode >= 400 {
		return parseHTTPError(resp.StatusCode, respBody, resp.Header.Get(headerRequestID))
	}
	return parseProductHTTPResponse(resp.StatusCode, respBody, result, resp.Header.Get(headerRequestID))
}

func (c *Client) applyAuth(req *http.Request) {
	apiKey := load(&c.apiKey)
	token := load(&c.token)
	sessionCookie := load(&c.sessionCookie)
	if c.personalAccessToken {
		req.Header.Del("Authorization")
		req.Header.Del("Cookie")
		req.Header.Set(headerAPIKey, apiKey)
		return
	}
	if apiKey != "" {
		req.Header.Set(headerAPIKey, apiKey)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if sessionCookie != "" {
		req.Header.Set("Cookie", sessionCookie)
	}
}

func clearPublicProductHeaders(header http.Header) {
	for _, key := range []string{headerAPIKey, "Authorization", "Cookie", headerRuntimeAuthorization, headerWorkspaceID, headerMOIRoleID} {
		deleteHeaderFold(header, key)
	}
}

func (c *Client) productURL(path string) string {
	return c.endpoint + path
}

func load(p *atomic.Pointer[string]) string {
	v := p.Load()
	if v == nil {
		return ""
	}
	return *v
}

func withListQuery(path string, opts []ListOption) string {
	return withQuery(path, opts)
}

func withQuery(path string, opts []QueryOption) string {
	options := &queryOptions{}
	for _, opt := range opts {
		opt(options)
	}
	if options.values == nil {
		return path
	}
	if encoded := options.values.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

func withListAndQuery(path string, listOpts []ListOption, queryOpts []QueryOption) string {
	options := &queryOptions{values: url.Values{}}
	for _, opt := range listOpts {
		opt(options)
	}
	for _, opt := range queryOpts {
		opt(options)
	}
	if encoded := options.values.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

func pathID(id string) (string, error) {
	if id == "" {
		return "", ErrIDRequired
	}
	return url.PathEscape(id), nil
}

// nativeURL keeps operational routes on the same MOI origin and deployment
// prefix as the configured /newmoi product gateway.
func (c *Client) nativeURL(path string) (string, error) {
	endpoint, err := url.Parse(c.endpoint)
	if err != nil {
		return "", fmt.Errorf("parse native MOI endpoint: %w", err)
	}
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/newmoi") + path
	endpoint.RawPath = ""
	endpoint.RawQuery = ""
	return endpoint.String(), nil
}
