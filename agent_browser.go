package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	gorillawebsocket "github.com/gorilla/websocket"
	"golang.org/x/net/websocket"
	"io"
	"strings"
)

const runtimeBrowserProtocol = "moi-browser.v1"

// RuntimeBrowserFrame preserves the WebSocket frame type. Browser metadata is
// text; image payloads are binary and must not be decoded as UTF-8.
type RuntimeBrowserFrame struct {
	Text bool
	Data []byte
}

// RuntimeBrowser owns one authenticated browser stream. One receiver and one
// sender may run concurrently; the caller must close the connection.
type RuntimeBrowser struct {
	connection      *websocket.Conn
	readMu, writeMu sync.Mutex
}

// RuntimeBrowserToken obtains the short-lived proof used only by the browser
// WebSocket subprotocol handshake.
func (s *AgentService) RuntimeBrowserToken(ctx context.Context, taskID string) (*SandboxTerminalToken, error) {
	taskPath, err := pathID(taskID)
	if err != nil {
		return nil, err
	}
	path, err := agentResourcePath(s.workspaceID, "agent-runtime-tasks", "/"+taskPath+"/browser/token")
	if err != nil {
		return nil, err
	}
	var out SandboxTerminalToken
	err = s.do(ctx, http.MethodPost, path, struct{}{}, &out)
	return &out, err
}

// ConnectRuntimeBrowser opens the browser stream without placing its token in
// the URL or adding HTTP authentication headers to the WebSocket handshake.
func (s *AgentService) ConnectRuntimeBrowser(ctx context.Context, taskID, token, origin string) (*RuntimeBrowser, error) {
	if token == "" {
		return nil, ErrTokenRequired
	}
	if origin == "" {
		return nil, ErrOriginRequired
	}
	taskPath, err := pathID(taskID)
	if err != nil {
		return nil, err
	}
	path, err := agentResourcePath(s.workspaceID, "agent-runtime-tasks", "/"+taskPath+"/browser/stream")
	if err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(s.client.productURL(path))
	if err != nil {
		return nil, err
	}
	switch endpoint.Scheme {
	case "https":
		endpoint.Scheme = "wss"
	case "http":
		endpoint.Scheme = "ws"
	default:
		return nil, &Error{Code: "ErrEndpointSchemeUnsupported", Message: "product endpoint must use http or https"}
	}
	endpoint.RawQuery = ""
	endpoint.ForceQuery = false
	endpoint.Fragment = ""
	config, err := websocket.NewConfig(endpoint.String(), origin)
	if err != nil {
		return nil, err
	}
	config.Protocol = []string{runtimeBrowserProtocol, token}
	if transport, ok := s.client.httpClient.Transport.(*http.Transport); ok {
		config.TlsConfig = transport.TLSClientConfig
	}
	connection, err := config.DialContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect runtime browser WebSocket: %w", err)
	}
	return &RuntimeBrowser{connection: connection}, nil
}

func (session *RuntimeBrowser) Close() error { return session.connection.Close() }

// WriteInput sends one JSON value as a text WebSocket frame.
func (session *RuntimeBrowser) WriteInput(ctx context.Context, input any) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	session.writeMu.Lock()
	defer session.writeMu.Unlock()
	return webSocketOperation(ctx, session.connection.SetWriteDeadline, func() error {
		return websocket.Message.Send(session.connection, string(raw))
	})
}

var runtimeBrowserFrameCodec = websocket.Codec{
	Unmarshal: func(data []byte, payloadType byte, value any) error {
		frame, ok := value.(*RuntimeBrowserFrame)
		if !ok {
			return fmt.Errorf("runtime browser receive target must be *RuntimeBrowserFrame")
		}
		switch payloadType {
		case websocket.TextFrame:
			frame.Text = true
		case websocket.BinaryFrame:
			frame.Text = false
		default:
			return fmt.Errorf("unsupported runtime browser WebSocket frame type %d", payloadType)
		}
		frame.Data = bytes.Clone(data)
		return nil
	},
}

// Receive returns one metadata or image frame without changing its wire type.
func (session *RuntimeBrowser) Receive(ctx context.Context) (*RuntimeBrowserFrame, error) {
	session.readMu.Lock()
	defer session.readMu.Unlock()
	var frame RuntimeBrowserFrame
	if err := webSocketOperation(ctx, session.connection.SetReadDeadline, func() error {
		return runtimeBrowserFrameCodec.Receive(session.connection, &frame)
	}); err != nil {
		return nil, err
	}
	return &frame, nil
}

// AgentBrowser owns a task's browser handoff stream. One receiver and one sender
// may run concurrently. Receive keeps frame metadata paired with its image.
type AgentBrowser struct {
	connection      *gorillawebsocket.Conn
	compressed      bool
	readMu, writeMu sync.Mutex
}

// ConnectBrowser obtains a task-scoped proof with the client's product identity.
// Compression changes wire encoding only; received image bytes are unchanged.
func (s *AgentService) ConnectBrowser(ctx context.Context, taskID string, compression bool) (*AgentBrowser, error) {
	return s.connectBrowser(ctx, taskID, compression, false)
}

// WatchBrowser observes the current page without acquiring browser control.
// Acknowledge each rendered frame. Keyboard and mouse input are rejected by the server.
func (s *AgentService) WatchBrowser(ctx context.Context, taskID string, compression bool) (*AgentBrowser, error) {
	return s.connectBrowser(ctx, taskID, compression, true)
}

func (s *AgentService) connectBrowser(ctx context.Context, taskID string, compression, readOnly bool) (*AgentBrowser, error) {
	id, err := pathID(taskID)
	if err != nil {
		return nil, err
	}
	path, err := agentResourcePath(s.workspaceID, "agent-runtime-tasks", "/"+id+"/browser")
	if err != nil {
		return nil, err
	}
	var proof struct {
		Token string `json:"token"`
	}
	if err := s.do(ctx, http.MethodPost, path+"/token", map[string]any{}, &proof); err != nil {
		return nil, err
	}
	if proof.Token == "" {
		return nil, &Error{Code: "ErrResponseInvalid", Message: "browser token response has no token"}
	}
	endpoint, err := url.Parse(s.client.productURL(path + "/stream"))
	if err != nil {
		return nil, err
	}
	if readOnly {
		query := endpoint.Query()
		query.Set("mode", "view")
		endpoint.RawQuery = query.Encode()
	}
	origin := endpoint.Scheme + "://" + endpoint.Host
	if endpoint.Scheme == "https" {
		endpoint.Scheme = "wss"
	} else {
		endpoint.Scheme = "ws"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Origin", origin)
	for _, modify := range s.client.requestModifier {
		modify(request)
	}
	dialer := gorillawebsocket.Dialer{EnableCompression: compression, Subprotocols: []string{"moi-browser.v1", proof.Token}, HandshakeTimeout: s.client.timeout}
	if transport, ok := s.client.httpClient.Transport.(*http.Transport); ok {
		dialer.TLSClientConfig = transport.TLSClientConfig
	}
	connection, response, err := dialer.DialContext(ctx, request.URL.String(), request.Header)
	if err != nil {
		if response != nil {
			defer response.Body.Close()
			body, readErr := io.ReadAll(response.Body)
			if readErr != nil {
				return nil, productResponseReadError(ctx, response.StatusCode, response.Header.Get(headerRequestID), readErr)
			}
			return nil, parseHTTPError(response.StatusCode, body, response.Header.Get(headerRequestID))
		}
		return nil, productTransportError(ctx, err)
	}
	return &AgentBrowser{connection: connection, compressed: strings.Contains(response.Header.Get("Sec-WebSocket-Extensions"), "permessage-deflate")}, nil
}

func (browser *AgentBrowser) Close() error             { return browser.connection.Close() }
func (browser *AgentBrowser) CompressionEnabled() bool { return browser.compressed }

// Receive returns one browser event, plus its image for a frame event.
// ProductData preserves the browser process's event and metadata fields.
func (browser *AgentBrowser) Receive(ctx context.Context) (*ProductData, []byte, error) {
	browser.readMu.Lock()
	defer browser.readMu.Unlock()
	var event ProductData
	var image []byte
	err := webSocketOperation(ctx, browser.connection.SetReadDeadline, func() error {
		kind, payload, err := browser.connection.ReadMessage()
		if err != nil {
			return err
		}
		if kind != gorillawebsocket.TextMessage {
			return &Error{Code: "ErrResponseInvalid", Message: "browser event must be text"}
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			return &Error{Code: "ErrResponseInvalid", Message: "browser event must be valid JSON", cause: err}
		}
		value, ok := event.AsMap()
		if !ok {
			return &Error{Code: "ErrResponseInvalid", Message: "browser event must be an object"}
		}
		if value["type"] == "frame" {
			kind, image, err = browser.connection.ReadMessage()
			if err != nil {
				return err
			}
			if kind != gorillawebsocket.BinaryMessage {
				return &Error{Code: "ErrResponseInvalid", Message: "browser frame image must be binary"}
			}
		}
		return nil
	})
	return &event, image, err
}

// Acknowledge releases stream credit after the caller has processed a frame.
// Use the seq from that frame; the bridge accepts cumulative acknowledgements.
func (browser *AgentBrowser) Acknowledge(ctx context.Context, seq int64) error {
	return browser.send(ctx, map[string]any{"type": "ack", "seq": seq})
}

func (browser *AgentBrowser) InputKeyboard(ctx context.Context, eventType, text string) error {
	return browser.send(ctx, map[string]string{"type": "input_keyboard", "eventType": eventType, "text": text})
}

func (browser *AgentBrowser) Release(ctx context.Context) error {
	return browser.send(ctx, map[string]string{"type": "release"})
}

func (browser *AgentBrowser) send(ctx context.Context, event any) error {
	browser.writeMu.Lock()
	defer browser.writeMu.Unlock()
	return webSocketOperation(ctx, browser.connection.SetWriteDeadline, func() error { return browser.connection.WriteJSON(event) })
}
