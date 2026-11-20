package sdk

import (
	"context"
	"encoding/json"
	"golang.org/x/net/websocket"
	"net/url"
	"strconv"
	"sync"
)

// SandboxTerminal owns the authenticated terminal connection and its framing.
// One receiver and one sender may run concurrently.
type SandboxTerminal struct {
	connection      *websocket.Conn
	readMu, writeMu sync.Mutex
}

func (s *SandboxService) ConnectTerminal(ctx context.Context, id, token string, cols, rows uint32) (*SandboxTerminal, error) {
	path, err := pathID(id)
	if err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(s.client.productURL("/agent-sandboxes/" + path + "/terminal/ws"))
	if err != nil {
		return nil, err
	}
	origin := endpoint.Scheme + "://" + endpoint.Host
	if endpoint.Scheme == "https" {
		endpoint.Scheme = "wss"
	} else {
		endpoint.Scheme = "ws"
	}
	endpoint.RawQuery = url.Values{"token": {token}, "cols": {strconv.FormatUint(uint64(cols), 10)}, "rows": {strconv.FormatUint(uint64(rows), 10)}}.Encode()
	config, err := websocket.NewConfig(endpoint.String(), origin)
	if err != nil {
		return nil, err
	}
	connection, err := config.DialContext(ctx)
	if err != nil {
		return nil, err
	}
	return &SandboxTerminal{connection: connection}, nil
}
func (s *SandboxTerminal) Close() error { return s.connection.Close() }
func (s *SandboxTerminal) WriteInput(ctx context.Context, data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return webSocketOperation(ctx, s.connection.SetWriteDeadline, func() error { return websocket.Message.Send(s.connection, data) })
}
func (s *SandboxTerminal) Receive(ctx context.Context) ([]byte, error) {
	s.readMu.Lock()
	defer s.readMu.Unlock()
	var data []byte
	err := webSocketOperation(ctx, s.connection.SetReadDeadline, func() error { return websocket.Message.Receive(s.connection, &data) })
	return data, err
}
func (s *SandboxTerminal) Resize(ctx context.Context, cols, rows uint32) error {
	raw, err := json.Marshal(struct {
		Type string `json:"type"`
		Cols uint32 `json:"cols"`
		Rows uint32 `json:"rows"`
	}{"resize", cols, rows})
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return webSocketOperation(ctx, s.connection.SetWriteDeadline, func() error { return websocket.Message.Send(s.connection, string(raw)) })
}
