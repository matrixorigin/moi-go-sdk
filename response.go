package sdk

import "encoding/json"

type apiEnvelope struct {
	Code      string          `json:"code"`
	Msg       string          `json:"msg"`
	Data      json.RawMessage `json:"data"`
	RequestID string          `json:"request_id"`
}
