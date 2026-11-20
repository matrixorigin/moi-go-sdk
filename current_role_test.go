package sdk

import (
	"net/http"
	"testing"
)

func TestValidateCurrentRoleID(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "minimum", value: "1", valid: true},
		{name: "maximum int64", value: "9223372036854775807", valid: true},
		{name: "empty", value: ""},
		{name: "zero", value: "0"},
		{name: "leading zero", value: "01"},
		{name: "leading whitespace", value: " 1"},
		{name: "trailing whitespace", value: "1 "},
		{name: "positive sign", value: "+1"},
		{name: "negative", value: "-1"},
		{name: "non decimal", value: "role-1"},
		{name: "over int64", value: "9223372036854775808"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCurrentRoleID(tt.value)
			if tt.valid && err != nil {
				t.Fatalf("ValidateCurrentRoleID(%q) error = %v", tt.value, err)
			}
			if !tt.valid && err != ErrCurrentRoleInvalid {
				t.Fatalf("ValidateCurrentRoleID(%q) error = %v, want %v", tt.value, err, ErrCurrentRoleInvalid)
			}
		})
	}
}

func TestWorkspaceCurrentRoleConfiguration(t *testing.T) {
	client, err := New(
		"https://product.example.test",
		WithAPIKey("api-key"),
		WithWorkspaceCurrentRoleID("ws-a", "12"),
		WithWorkspaceCurrentRoleID("ws-a", "12"),
		WithWorkspaceCurrentRoleID("ws-b", "27"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := client.workspaceCurrentRoleIDs["ws-a"]; got != "12" {
		t.Fatalf("workspace ws-a role = %q, want 12", got)
	}
	if got := client.workspaceCurrentRoleIDs["ws-b"]; got != "27" {
		t.Fatalf("workspace ws-b role = %q, want 27", got)
	}
}

func TestWorkspaceCurrentRoleConfigurationRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want error
	}{
		{
			name: "workspace required",
			opts: []Option{WithWorkspaceCurrentRoleID("", "12")},
			want: ErrWorkspaceRequired,
		},
		{
			name: "role invalid",
			opts: []Option{WithWorkspaceCurrentRoleID("ws-a", "01")},
			want: ErrCurrentRoleInvalid,
		},
		{
			name: "workspace role conflict",
			opts: []Option{
				WithWorkspaceCurrentRoleID("ws-a", "12"),
				WithWorkspaceCurrentRoleID("ws-a", "27"),
			},
			want: ErrWorkspaceCurrentRoleConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := append([]Option{WithAPIKey("api-key")}, tt.opts...)
			client, err := New("https://product.example.test", opts...)
			if client != nil || err != tt.want {
				t.Fatalf("client=%v error=%v, want nil client and %v", client, err, tt.want)
			}
		})
	}
}

func TestWorkspaceCurrentRoleClientSnapshot(t *testing.T) {
	first, err := New("https://product.example.test", WithAPIKey("api-key"), WithWorkspaceCurrentRoleID("ws-a", "12"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := New("https://product.example.test", WithAPIKey("api-key"), WithWorkspaceCurrentRoleID("ws-a", "27"))
	if err != nil {
		t.Fatal(err)
	}

	firstHeader := make(http.Header)
	first.applyWorkspaceScope(firstHeader, "ws-a")
	secondHeader := make(http.Header)
	second.applyWorkspaceScope(secondHeader, "ws-a")
	if got := firstHeader.Get(headerMOIRoleID); got != "12" {
		t.Fatalf("first client role = %q, want 12", got)
	}
	if got := secondHeader.Get(headerMOIRoleID); got != "27" {
		t.Fatalf("second client role = %q, want 27", got)
	}
}
