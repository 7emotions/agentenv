package resolver

import (
	"slices"

	"github.com/7emotions/agentenv/pkg/source"
	"github.com/7emotions/agentenv/pkg/types"
	"github.com/contriboss/pubgrub-go"
)

// agentenvSource adapts a source.SourceHandler, a types.SourceURL, and a
// depSource callback into the pubgrub.Source interface. Each instance wraps
// a single package, using the handler to list its versions and the callback
// to construct dependency source URLs (used in a later task).
//
// The integration flow:
//   - GetVersions calls handler.ListVersions(srcURL) and converts each
//     semver string to a *pubgrub.SemanticVersion.
//   - GetDependencies currently returns an empty list (transitive dependency
//     resolution will be added in a future task).
type agentenvSource struct {
	handler   source.SourceHandler
	srcURL    *types.SourceURL
	depSource func(name, pkgType string) string
}

// NewAgentenvSource creates a pubgrub.Source adapter backed by the given
// handler and source URL. The depSource callback will be used in a future
// task to construct source URLs for transitive dependencies.
func NewAgentenvSource(
	handler source.SourceHandler,
	srcURL *types.SourceURL,
	depSource func(name, pkgType string) string,
) *agentenvSource {
	return &agentenvSource{
		handler:   handler,
		srcURL:    srcURL,
		depSource: depSource,
	}
}

// GetVersions returns all available versions of the package in ascending
// order (lowest first), as required by the pubgrub.Source contract.
//
// It delegates to handler.ListVersions(srcURL) and parses the returned
// semver strings. Invalid versions are silently skipped. An empty result
// is returned as a non-nil empty slice.
func (s *agentenvSource) GetVersions(name pubgrub.Name) ([]pubgrub.Version, error) {
	rawVersions, err := s.handler.ListVersions(*s.srcURL)
	if err != nil {
		return nil, err
	}

	if len(rawVersions) == 0 {
		return []pubgrub.Version{}, nil
	}

	result := make([]pubgrub.Version, 0, len(rawVersions))
	for _, raw := range rawVersions {
		sv, err := pubgrub.ParseSemanticVersion(raw)
		if err != nil {
			continue // match SortVersions behaviour: skip invalid silently
		}
		result = append(result, sv)
	}

	// Sort ascending (lowest first) as required by Source.GetVersions.
	slices.SortFunc(result, func(a, b pubgrub.Version) int {
		return a.Sort(b)
	})

	return result, nil
}

// GetDependencies returns the dependency terms for a specific package version.
//
// Currently returns an empty list. Full transitive dependency resolution will
// be implemented in a follow-up task (Task 5) where DecodeName from namespace.go
// is used to extract the package type and name from the pubgrub Name.
func (s *agentenvSource) GetDependencies(name pubgrub.Name, version pubgrub.Version) ([]pubgrub.Term, error) {
	return []pubgrub.Term{}, nil
}

// Compile-time check that agentenvSource implements pubgrub.Source.
var _ pubgrub.Source = (*agentenvSource)(nil)
