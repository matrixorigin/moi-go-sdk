package sdk

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/matrixorigin/moi-go-sdk/v5/product"
	"golang.org/x/net/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

type SQLQueryWebSocketMessage = product.SQLQueryWebSocketMessage

const (
	SQLQueryMessagePing    = product.SQLQueryWebSocketMessage_ping
	SQLQueryMessagePong    = product.SQLQueryWebSocketMessage_pong
	SQLQueryMessageExecute = product.SQLQueryWebSocketMessage_query_execute
	SQLQueryMessageQueryID = product.SQLQueryWebSocketMessage_query_id
	SQLQueryMessageResult  = product.SQLQueryWebSocketMessage_query_result
	SQLQueryMessageKill    = product.SQLQueryWebSocketMessage_query_kill
	SQLQueryMessageError   = product.SQLQueryWebSocketMessage_error
)

// SQLQueryWebSocket owns SQL editor framing and cancellable reads/writes.
// Close releases the connection. One reader and one writer may run concurrently.
type SQLQueryWebSocket struct {
	connection *websocket.Conn
	readMu     sync.Mutex
	writeMu    sync.Mutex
}

// ConnectQueryWebSocket opens a SQL editor session with SDK-owned identity and workspace scope.
func (s *SQLService) ConnectQueryWebSocket(ctx context.Context) (*SQLQueryWebSocket, error) {
	endpoint, err := s.QueryWebSocketURL()
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	query := parsed.Query()
	query.Set("workspace_id", s.workspaceID)
	parsed.RawQuery = query.Encode()
	origin := *parsed
	origin.Path = ""
	origin.RawQuery = ""
	origin.Scheme = "http"
	if parsed.Scheme == "wss" {
		origin.Scheme = "https"
	}
	config, err := websocket.NewConfig(parsed.String(), origin.String())
	if err != nil {
		return nil, err
	}
	config.Header = s.QueryWebSocketHeaders()
	config.Protocol = s.QueryWebSocketSubprotocols()
	// Request modifiers also apply to the SDK-owned WebSocket handshake.
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header = config.Header
	for _, modifier := range s.client.requestModifier {
		modifier(request)
	}
	config.Header = request.Header
	config.Location = request.URL
	if transport, ok := s.client.httpClient.Transport.(*http.Transport); ok {
		config.TlsConfig = transport.TLSClientConfig
	}
	connection, err := config.DialContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect SQL editor WebSocket: %w", err)
	}
	return &SQLQueryWebSocket{connection: connection}, nil
}

func (session *SQLQueryWebSocket) Protocols() []string {
	return slices.Clone(session.connection.Config().Protocol)
}

func (session *SQLQueryWebSocket) Close() error { return session.connection.Close() }

func (session *SQLQueryWebSocket) ExecuteQuery(ctx context.Context, id, query string, opts ...SQLQueryOption) error {
	if query == "" {
		return ErrQueryRequired
	}
	return session.send(ctx, id, SQLQueryMessageExecute, sqlQueryBody(query, opts...))
}

func (session *SQLQueryWebSocket) KillQuery(ctx context.Context, id, queryID string) error {
	if queryID == "" {
		return ErrQueryIDRequired
	}
	return session.send(ctx, id, SQLQueryMessageKill, map[string]any{"query_id": queryID})
}

func (session *SQLQueryWebSocket) Ping(ctx context.Context, id string) error {
	return session.send(ctx, id, SQLQueryMessagePing, nil)
}

func (session *SQLQueryWebSocket) send(ctx context.Context, id string, messageType product.SQLQueryWebSocketMessage_Type, body map[string]any) error {
	message := &SQLQueryWebSocketMessage{Id: id, MessageType: messageType}
	if body != nil {
		data, err := structpb.NewStruct(body)
		if err != nil {
			return err
		}
		message.Data = data
	}
	raw, err := protojson.Marshal(message)
	if err != nil {
		return err
	}
	session.writeMu.Lock()
	defer session.writeMu.Unlock()
	// Sending a string makes the wire frame text, as required by the editor protocol.
	return webSocketOperation(ctx, session.connection.SetWriteDeadline, func() error {
		return websocket.Message.Send(session.connection, string(raw))
	})
}

func (session *SQLQueryWebSocket) Receive(ctx context.Context) (*SQLQueryWebSocketMessage, error) {
	session.readMu.Lock()
	defer session.readMu.Unlock()
	var raw []byte
	if err := webSocketOperation(ctx, session.connection.SetReadDeadline, func() error {
		return websocket.Message.Receive(session.connection, &raw)
	}); err != nil {
		return nil, err
	}
	message := &SQLQueryWebSocketMessage{}
	if err := protojson.Unmarshal(raw, message); err != nil {
		return nil, err
	}
	return message, nil
}

func webSocketOperation(ctx context.Context, setDeadline func(time.Time) error, operation func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline, _ := ctx.Deadline()
	if err := setDeadline(deadline); err != nil {
		return err
	}
	var cancellationErr error
	canceled := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		cancellationErr = setDeadline(time.Now())
		close(canceled)
	})
	operationErr := operation()
	if !stop() {
		<-canceled
	}
	contextErr := ctx.Err()
	var timeout net.Error
	if contextErr == nil && !deadline.IsZero() && !time.Now().Before(deadline) &&
		errors.As(operationErr, &timeout) && timeout.Timeout() {
		contextErr = context.DeadlineExceeded
	}
	return errors.Join(operationErr, contextErr, cancellationErr, setDeadline(time.Time{}))
}
