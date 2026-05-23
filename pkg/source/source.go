// Package source provides handlers for fetching package data from various
// source schemes (github, npm, local, git).
package source

import (
	"fmt"

	"github.com/7emotions/agentenv/pkg/types"
)

// VersionInfo describes a single version available from a source.
type VersionInfo struct {
	Version string
	SHA256  string
}

// SourceHandler is the interface that all source scheme handlers must implement.
type SourceHandler interface {
	// ListVersions returns available versions sorted highest-first. For
	// sources without versioning (e.g. local), it returns a single entry.
	ListVersions(src types.SourceURL) ([]string, error)

	// Fetch retrieves package data for the given version. It returns the
	// raw package bytes, its SHA-256 hex digest, and any error.
	Fetch(src types.SourceURL, version string) ([]byte, string, error)
}

// Registry maps source schemes to their handler implementations.
var Registry = map[string]SourceHandler{
	"github": &GitHubSource{},
	"npm":    &NPMSource{},
	"local":  &LocalSource{},
	"git":    &GitSource{},
}

// GetHandler returns the SourceHandler registered for the given scheme.
func GetHandler(scheme string) (SourceHandler, error) {
	h, ok := Registry[scheme]
	if !ok {
		return nil, fmt.Errorf("unknown source scheme: %s", scheme)
	}
	return h, nil
}
