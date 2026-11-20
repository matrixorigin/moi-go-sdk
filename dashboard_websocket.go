package sdk

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/encoding/protojson"
)

type DataDashboardWebSocketMessage = product.DataDashboardWebSocketMessage
type DataDashboardWebSocketError = product.DataDashboardWebSocketError

const (
	DashboardMessagePing              = product.DataDashboardWebSocketMessage_ping
	DashboardMessagePong              = product.DataDashboardWebSocketMessage_pong
	DashboardMessagePreviewResult     = product.DataDashboardWebSocketMessage_dashboard_preview_result
	DashboardMessageChartCreateResult = product.DataDashboardWebSocketMessage_dashboard_chart_create_result
	DashboardMessageChartUpdateResult = product.DataDashboardWebSocketMessage_dashboard_chart_update_result
	DashboardMessageError             = product.DataDashboardWebSocketMessage_error
)

// DashboardWebSocket 管理看板消息；允许一个读取者与一个写入者并发。
type DashboardWebSocket struct {
	connection      *websocket.Conn
	readMu, writeMu sync.Mutex
}

// ConnectWebSocket 建立固定工作区与当前角色的看板连接，不重连或重放写入。
func (s *DashboardService) ConnectWebSocket(ctx context.Context) (*DashboardWebSocket, error) {
	base, err := s.client.webSocketBase(s.workspaceID)
	if err != nil {
		return nil, err
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/data-dashboards/ws"
	query := base.Query()
	query.Set("workspace_id", s.workspaceID)
	base.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header = s.client.webSocketHeaders(s.workspaceID)
	request.Header.Set("Origin", "http"+strings.TrimPrefix(base.Scheme, "ws")+"://"+base.Host)
	for _, modifier := range s.client.requestModifier {
		modifier(request)
	}
	query = request.URL.Query()
	query.Set("workspace_id", s.workspaceID)
	request.URL.RawQuery = query.Encode()
	s.client.applyWorkspaceScope(request.Header, s.workspaceID)
	protocols := s.client.webSocketSubprotocols()
	if roleID := request.Header.Get(headerMOIRoleID); roleID != "" {
		protocols = append(protocols, "moi-role-id."+roleID)
	}
	dialer := websocket.Dialer{Subprotocols: protocols, HandshakeTimeout: s.client.timeout, Proxy: http.ProxyFromEnvironment}
	if transport, ok := s.client.httpClient.Transport.(*http.Transport); ok {
		dialer.TLSClientConfig, dialer.Proxy = transport.TLSClientConfig, transport.Proxy
	}
	connection, response, err := dialer.DialContext(ctx, request.URL.String(), request.Header)
	if err != nil {
		if response == nil {
			return nil, productTransportError(ctx, err)
		}
		defer response.Body.Close()
		body, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return nil, productResponseReadError(ctx, response.StatusCode, response.Header.Get(headerRequestID), readErr)
		}
		return nil, parseHTTPError(response.StatusCode, body, response.Header.Get(headerRequestID))
	}
	return &DashboardWebSocket{connection: connection}, nil
}

func (session *DashboardWebSocket) Close() error { return session.connection.Close() }

func (session *DashboardWebSocket) Ping(ctx context.Context, id string) error {
	return session.send(ctx, id, product.DataDashboardWebSocketMessage_ping, nil)
}

func (session *DashboardWebSocket) Preclose(ctx context.Context, id string) error {
	return session.send(ctx, id, product.DataDashboardWebSocketMessage_preclose, nil)
}

func (session *DashboardWebSocket) PreviewSQL(ctx context.Context, id, dashboardID, chartID, sqlText string) error {
	return session.send(ctx, id, product.DataDashboardWebSocketMessage_dashboard_preview_execute, map[string]string{"dashboard_id": dashboardID, "chart_id": chartID, "sql_text": sqlText})
}

func (session *DashboardWebSocket) CreateChart(ctx context.Context, id, dashboardID string, request *DataDashboardChartMutationRequest) error {
	if request == nil {
		return &Error{Code: "ErrRequestRequired", Message: "dashboard request is required"}
	}
	return session.send(ctx, id, product.DataDashboardWebSocketMessage_dashboard_chart_create, struct {
		DashboardID string `json:"dashboard_id"`
		*DataDashboardChartMutationRequest
	}{dashboardID, request})
}

func (session *DashboardWebSocket) UpdateChart(ctx context.Context, id, dashboardID, chartID string, request *DataDashboardChartMutationRequest) error {
	if request == nil {
		return &Error{Code: "ErrRequestRequired", Message: "dashboard request is required"}
	}
	return session.send(ctx, id, product.DataDashboardWebSocketMessage_dashboard_chart_update, struct {
		DashboardID string `json:"dashboard_id"`
		ChartID     string `json:"chart_id"`
		*DataDashboardChartMutationRequest
	}{dashboardID, chartID, request})
}

func (session *DashboardWebSocket) send(ctx context.Context, id string, kind product.DataDashboardWebSocketMessage_Type, data any) error {
	if id == "" {
		return ErrIDRequired
	}
	payload, err := json.Marshal(struct {
		ID   string `json:"id"`
		Type string `json:"message_type"`
		Data any    `json:"data,omitempty"`
	}{id, kind.String(), data})
	if err != nil {
		return err
	}
	session.writeMu.Lock()
	defer session.writeMu.Unlock()
	err = webSocketOperation(ctx, session.connection.SetWriteDeadline, func() error { return session.connection.WriteMessage(websocket.TextMessage, payload) })
	if err != nil {
		return productTransportError(ctx, err)
	}
	return nil
}

// Receive 同时保留关联消息 ID 与服务端错误的状态、机器码及本地化消息。
func (session *DashboardWebSocket) Receive(ctx context.Context) (*DataDashboardWebSocketMessage, error) {
	session.readMu.Lock()
	defer session.readMu.Unlock()
	var payload []byte
	err := webSocketOperation(ctx, session.connection.SetReadDeadline, func() error {
		kind, data, err := session.connection.ReadMessage()
		if err != nil {
			return productTransportError(ctx, err)
		}
		if kind != websocket.TextMessage {
			return &Error{Code: "ErrResponseInvalid", Message: "dashboard response must be text"}
		}
		payload = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	frame := &DataDashboardWebSocketMessage{}
	if err := protojson.Unmarshal(payload, frame); err != nil {
		return nil, &Error{Code: "ErrResponseInvalid", Message: "dashboard response is invalid", cause: err}
	}
	if frame.GetMessageType() == DashboardMessageError {
		failure := &DataDashboardWebSocketError{}
		data, err := protojson.Marshal(frame.GetData())
		if err == nil {
			err = protojson.Unmarshal(data, failure)
		}
		if err != nil || failure.GetStatus() < 400 || failure.GetStatus() > 599 || failure.GetCode() == "" || failure.GetMsg() == "" {
			return frame, &Error{Code: "ErrResponseInvalid", Message: "dashboard error response is invalid", cause: err}
		}
		return frame, &Error{Status: int(failure.GetStatus()), Code: failure.GetCode(), Message: failure.GetMsg(), Data: payload}
	}
	switch frame.GetMessageType() {
	case DashboardMessagePong, DashboardMessagePreviewResult, DashboardMessageChartCreateResult, DashboardMessageChartUpdateResult:
		return frame, nil
	default:
		return frame, &Error{Code: "ErrResponseInvalid", Message: "dashboard response has an unknown message type"}
	}
}
