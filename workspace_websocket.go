package sdk

import (
	"fmt"
	"net/http"
	"net/url"
)

// webSocketBase 与 HTTP 请求使用同一产品地址和工作区边界。
func (c *Client) webSocketBase(workspaceID string) (*url.URL, error) {
	if workspaceID == "" {
		return nil, ErrWorkspaceRequired
	}
	base, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse product endpoint: %w", err)
	}
	switch base.Scheme {
	case "https":
		base.Scheme = "wss"
	case "http":
		base.Scheme = "ws"
	default:
		return nil, &Error{Code: "ErrEndpointSchemeUnsupported", Message: "product endpoint must use http or https"}
	}
	base.RawQuery = ""
	return base, nil
}

func (c *Client) webSocketHeaders(workspaceID string) http.Header {
	headers := make(http.Header)
	if apiKey := load(&c.apiKey); apiKey != "" {
		headers.Set(headerAPIKey, apiKey)
	}
	if token := load(&c.token); token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}
	if cookie := load(&c.sessionCookie); cookie != "" {
		headers.Set("Cookie", cookie)
	}
	for key, value := range c.customHeaders {
		headers.Set(key, value)
	}
	c.applyWorkspaceScope(headers, workspaceID)
	return headers
}

func (c *Client) webSocketSubprotocols() []string {
	if apiKey := load(&c.apiKey); apiKey != "" {
		return []string{"moi-api-key", "moi-api-key." + apiKey}
	}
	if token := load(&c.token); token != "" {
		return []string{"moi-bearer", "moi-bearer." + token}
	}
	return nil
}
