package sdk

import (
	"testing"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
	"google.golang.org/protobuf/encoding/protojson"
)

// Product SDK decodes responses with strict protojson (DiscardUnknown=false).
// The degraded lineage overview for a deleted producer workflow must decode:
// source_status/source_unavailable_reason were added to LineageArtifactOverview
// so SDK consumers can read the machine-readable degradation instead of failing.
func TestLineageOverviewAcceptsSourceUnavailableDegradation(t *testing.T) {
	raw := []byte(`{
		"code": "OK",
		"msg": "OK",
		"data": {
			"artifact": {
				"artifact_id": "root-1:case-1",
				"root_asset_id": "root-1",
				"stage_asset_id": "",
				"case_id": "case-1",
				"workflow_version_id": "",
				"raw_file_id": "raw-1",
				"parsed_file_id": "parsed-1",
				"parsed_file_available": true,
				"origin_file_name": "doc.md",
				"origin_file_ext": "md",
				"selected_by": "entry_case_id",
				"selection_metadata": {"entry_case_id": "case-1"},
				"source_status": "unavailable",
				"source_unavailable_reason": "execution_context_missing",
				"case_candidates": [
					{"case_id": "case-1", "created_at": "2026-10-17T16:00:00Z", "status": "source_unavailable", "role": "producer", "selected": true}
				]
			},
			"topology": {"nodes": [], "edges": []},
			"entry": {"catalog_file_id": "parsed-1", "volume_id": "8"}
		}
	}`)

	var overview product.LineageOverview
	if err := parseProductResponse(raw, &overview, ""); err != nil {
		t.Fatalf("parseProductResponse: %v", err)
	}
	artifact := overview.GetArtifact()
	if artifact.GetSourceStatus() != "unavailable" {
		t.Fatalf("source_status=%q", artifact.GetSourceStatus())
	}
	if artifact.GetSourceUnavailableReason() != "execution_context_missing" {
		t.Fatalf("source_unavailable_reason=%q", artifact.GetSourceUnavailableReason())
	}
	if artifact.GetSelectedBy() != "entry_case_id" {
		t.Fatalf("selected_by=%q", artifact.GetSelectedBy())
	}

	// Strict protojson without the envelope must also accept the degraded shape.
	strict := protojson.UnmarshalOptions{DiscardUnknown: false}
	var artifactOnly product.LineageArtifactOverview
	if err := strict.Unmarshal([]byte(`{
		"artifact_id": "root-1:case-1",
		"source_status": "unavailable",
		"source_unavailable_reason": "execution_context_missing",
		"case_candidates": []
	}`), &artifactOnly); err != nil {
		t.Fatalf("strict unmarshal: %v", err)
	}
	if artifactOnly.GetSourceStatus() != "unavailable" {
		t.Fatalf("strict source_status=%q", artifactOnly.GetSourceStatus())
	}
}
