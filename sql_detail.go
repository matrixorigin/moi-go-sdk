package sdk

import (
	"bytes"
	"google.golang.org/protobuf/encoding/protojson"
)

// The query-detail contract uses an explicit null for a missing record.
// Missing data and malformed envelopes retain the shared transport failure.
type sqlStatementDetail struct{ statement *SQLStatementRecord }

func (*sqlStatementDetail) acceptsProductNull() {}
func (detail *sqlStatementDetail) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		detail.statement = nil
		return nil
	}
	detail.statement = &SQLStatementRecord{}
	return (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(data, detail.statement)
}
