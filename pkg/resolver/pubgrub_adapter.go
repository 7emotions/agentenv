package resolver

import (
	"slices"

	"github.com/7emotions/agentenv/pkg/parser"
	"github.com/7emotions/agentenv/pkg/source"
	"github.com/7emotions/agentenv/pkg/types"
	"github.com/contriboss/pubgrub-go"
)

// agentenvSource adapts a source.SourceHandler, a types.SourceURL, and a
// depSource callback into the pubgrub.Source interface. Each instance wraps
// a single package, using the handler to list its versions and fetch its
// archive for dependency discovery.
type agentenvSource struct {
	handler   source.SourceHandler
	srcURL    *types.SourceURL
	depSource func(name, pkgType string) string
}

// NewAgentenvSource creates a pubgrub.Source adapter backed by the given
// handler and source URL, wrapped with a CachedSource to prevent re-fetching
// the same package's versions and dependencies.
//
// The depSource callback is used to construct source URLs for discovered
// transitive dependencies. If nil, defaults to "github:test/<name>".
func NewAgentenvSource(
	handler source.SourceHandler,
	srcURL *types.SourceURL,
	depSource func(name, pkgType string) string,
) *pubgrub.CachedSource {
	if depSource == nil {
		depSource = func(name, pkgType string) string {
			return "github:test/" + name
		}
	}
	return pubgrub.NewCachedSource(&agentenvSource{
		handler:   handler,
		srcURL:    srcURL,
		depSource: depSource,
	})
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
// It fetches the package tar.gz archive, extracts agentpkg.yaml, and parses
// the manifest to discover dependencies. Each dependency is encoded using
// EncodeName to namespace by type (e.g., "skill:code-reviewer").
//
// If the manifest is missing or unparseable, an empty list is returned with
// no error (graceful degradation). Fetch errors are propagated to the caller
// so the solver can report them properly.
func (s *agentenvSource) GetDependencies(name pubgrub.Name, version pubgrub.Version) ([]pubgrub.Term, error) {
	data, _, err := s.handler.Fetch(*s.srcURL, version.String())
	if err != nil {
		return nil, err
	}

	manifestData, manifestErr := extractManifestFromTarGz(data)
	if manifestErr != nil {
		return []pubgrub.Term{}, nil
	}

	spec, parseErr := parser.ParseAgentPkg(manifestData)
	if parseErr != nil {
		return []pubgrub.Term{}, nil
	}

	terms := make([]pubgrub.Term, 0, len(spec.Dependencies))
	for _, dep := range spec.Dependencies {
		depName := pubgrub.MakeName(EncodeName(string(dep.Type), dep.Name))

		constraint := dep.Constraint
		if constraint == "" {
			constraint = "*"
		}

		vs, err := pubgrub.ParseVersionRange(constraint)
		if err != nil {
			continue
		}

		terms = append(terms, pubgrub.NewTerm(depName, pubgrub.NewVersionSetCondition(vs)))
	}

	if terms == nil {
		return []pubgrub.Term{}, nil
	}

	return terms, nil
}

// Compile-time check that agentenvSource implements pubgrub.Source.
var _ pubgrub.Source = (*agentenvSource)(nil)
