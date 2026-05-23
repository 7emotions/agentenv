package source

import (
	"testing"

	"github.com/7emotions/agentenv/pkg/types"
)

func TestGetHandler(t *testing.T) {
	tests := []struct {
		scheme string
		want   SourceHandler
	}{
		{"github", &GitHubSource{}},
		{"npm", &NPMSource{}},
		{"local", &LocalSource{}},
		{"git", &GitSource{}},
	}

	for _, tt := range tests {
		t.Run(tt.scheme, func(t *testing.T) {
			h, err := GetHandler(tt.scheme)
			if err != nil {
				t.Errorf("GetHandler(%q) unexpected error: %v", tt.scheme, err)
			}
			if h == nil {
				t.Errorf("GetHandler(%q) returned nil handler", tt.scheme)
			}
		})
	}
}

func TestGetHandlerUnknown(t *testing.T) {
	_, err := GetHandler("unknown")
	if err == nil {
		t.Error("GetHandler(unknown) expected error, got nil")
	}
}

func TestRegistry(t *testing.T) {
	expectedSchemes := []string{"github", "npm", "local", "git"}
	for _, scheme := range expectedSchemes {
		if _, ok := Registry[scheme]; !ok {
			t.Errorf("Registry missing scheme %q", scheme)
		}
	}
	if len(Registry) != 4 {
		t.Errorf("Registry has %d entries, want 4", len(Registry))
	}
}

func TestSourceHandlerInterface(t *testing.T) {
	// Compile-time check: ensure types implement the interface.
	var _ SourceHandler = &GitHubSource{}
	var _ SourceHandler = &NPMSource{}
	var _ SourceHandler = &LocalSource{}
	var _ SourceHandler = &GitSource{}

	// Verify interface methods exist.
	handlers := []SourceHandler{
		&GitHubSource{},
		&NPMSource{},
		&LocalSource{},
		&GitSource{},
	}

	src := types.SourceURL{Scheme: "local", Path: "/nonexistent"}
	for _, h := range handlers {
		_, err := h.ListVersions(src)
		if err == nil {
			continue
		}
		// Local handler will error on nonexistent path. That's fine.
		_ = err
	}
}

func TestGetHandlerNilOnBadScheme(t *testing.T) {
	h, err := GetHandler("docker")
	if err == nil {
		t.Error("expected error for docker scheme")
	}
	if h != nil {
		t.Error("expected nil handler for unknown scheme")
	}
}
