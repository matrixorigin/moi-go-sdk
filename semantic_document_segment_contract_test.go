package sdk

import (
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

// GetSourceDocument returns SemanticModelDocumentSegment, including
// segment_type / start_ms / end_ms. Strict protojson must accept those keys.
func TestGetSourceDocumentAcceptsDocumentSegmentType(t *testing.T) {
	raw := []byte(`{
		"code": "OK",
		"msg": "OK",
		"data": {
			"etag": "etag-1",
			"segment_status": {"available": true, "total": 1},
			"segments": [
				{
					"segment_id": "seg-1",
					"segment_type": "image",
					"start_ms": 0,
					"end_ms": 1250,
					"level": "chunk",
					"content": "first chunk",
					"image_file_id": "image-file-1",
					"page_image_file_id": "page-image-1",
					"word_count": 11,
					"recall_count": 3,
					"enabled": true,
					"metadata": {"volume_id": 13}
				}
			]
		}
	}`)

	var doc product.SemanticModelSourceDocument
	if err := parseProductResponse(raw, &doc, ""); err != nil {
		t.Fatalf("parseProductResponse: %v", err)
	}
	if doc.GetEtag() != "etag-1" {
		t.Fatalf("etag=%q", doc.GetEtag())
	}
	if len(doc.GetSegments()) != 1 {
		t.Fatalf("segments len=%d", len(doc.GetSegments()))
	}
	seg := doc.GetSegments()[0]
	if seg.GetSegmentId() != "seg-1" {
		t.Fatalf("segment_id=%q", seg.GetSegmentId())
	}
	if seg.GetSegmentType() != "image" {
		t.Fatalf("segment_type=%q", seg.GetSegmentType())
	}
	if seg.StartMs == nil || *seg.StartMs != 0 {
		t.Fatalf("start_ms=%v", seg.StartMs)
	}
	if seg.EndMs == nil || *seg.EndMs != 1250 {
		t.Fatalf("end_ms=%v", seg.EndMs)
	}
	if seg.GetImageFileId() != "image-file-1" {
		t.Fatalf("image_file_id=%q", seg.GetImageFileId())
	}
}
