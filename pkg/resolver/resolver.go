// Package resolver provides version constraint parsing and dependency graph
// resolution for agentenv packages.
package resolver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/7emotions/agentenv/pkg/parser"
	"github.com/7emotions/agentenv/pkg/source"
	"github.com/7emotions/agentenv/pkg/types"
)

// ResolvedPackage is the output of resolution for a single package.
type ResolvedPackage struct {
	Name         string
	Type         string
	Source       string
	Version      string
	Resolved     string
	SHA256       string
	Dependencies []ResolvedDep
	ResolvedBy   string
}

// ResolvedDep represents a resolved dependency within a package.
type ResolvedDep struct {
	Name     string
	Type     string
	Version  string
	Resolved string
	Source   string
}

// ResolutionResult holds the output of dependency resolution.
type ResolutionResult struct {
	Packages []ResolvedPackage
	Warnings []string
	Duration time.Duration
}

// DependencyResolver resolves a list of top-level package requests into a
// complete dependency graph.
type DependencyResolver interface {
	Resolve(ctx context.Context, requests []PackageRequest, opts ResolveOptions) (*ResolutionResult, error)
}

// PackageRequest describes a single package to resolve.
type PackageRequest struct {
	Name       string
	Type       string
	Source     string
	Constraint string
}

// ResolveOptions controls resolution behavior.
type ResolveOptions struct {
	MaxDepth    int
	MaxPackages int
	Strict      bool
}

// DefaultResolveOptions returns sensible defaults.
func DefaultResolveOptions() ResolveOptions {
	return ResolveOptions{
		MaxDepth:    3,
		MaxPackages: 50,
	}
}

// TopologicalBacktrackResolver resolves dependencies using a topological
// queue with backtracking for conflict detection.
//
// It processes packages breadth-first: resolving top-level requests, then
// their dependencies, then transitive dependencies. It detects cycles by
// tracking the resolution path and version conflicts by checking constraint
// compatibility when a package is requested again.
type TopologicalBacktrackResolver struct {
	handlers map[string]source.SourceHandler

	// DepSource constructs a source URL string for a dependency from its
	// name and type. If nil, defaults to "github:test/<name>".
	DepSource func(name, pkgType string) string
}

// NewResolver creates a new TopologicalBacktrackResolver.
func NewResolver() *TopologicalBacktrackResolver {
	return &TopologicalBacktrackResolver{
		handlers: make(map[string]source.SourceHandler),
	}
}

// RegisterHandler registers a custom source handler for a scheme, overriding
// the global source registry for that scheme during resolution.
func (r *TopologicalBacktrackResolver) RegisterHandler(scheme string, h source.SourceHandler) {
	r.handlers[scheme] = h
}

func (r *TopologicalBacktrackResolver) getHandler(scheme string) (source.SourceHandler, error) {
	if h, ok := r.handlers[scheme]; ok {
		return h, nil
	}
	return source.GetHandler(scheme)
}

func (r *TopologicalBacktrackResolver) depSource(name, pkgType string) string {
	if r.DepSource != nil {
		return r.DepSource(name, pkgType)
	}
	return "github:test/" + name
}

func extractManifestFromTarGz(data []byte) ([]byte, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("not gzip data: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(header.Name, "agentpkg.yaml") ||
			filepath.Base(header.Name) == "agentpkg.yaml" {
			return io.ReadAll(tarReader)
		}
	}
	return nil, fmt.Errorf("agentpkg.yaml not found in archive")
}

type queuedRequest struct {
	request PackageRequest
	depth   int
	path    []string
	parent  string
}

// Resolve resolves the given package requests into a complete dependency graph.
func (r *TopologicalBacktrackResolver) Resolve(ctx context.Context, requests []PackageRequest, opts ResolveOptions) (*ResolutionResult, error) {
	start := time.Now()

	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 3
	}
	if opts.MaxPackages <= 0 {
		opts.MaxPackages = 50
	}

	resolved := make(map[string]ResolvedPackage)
	versionByConstraint := make(map[string]string)
	queue := make([]queuedRequest, 0, len(requests))
	resolvedOrder := make([]string, 0)
	seenTypes := make(map[string]map[string]bool)
	warnings := make([]string, 0)

	for _, req := range requests {
		if req.Constraint == "" {
			req.Constraint = "*"
		}
		queue = append(queue, queuedRequest{
			request: req,
			depth:   0,
			parent:  "(root)",
		})
	}

	for len(queue) > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		q := queue[0]
		queue = queue[1:]

		name := q.request.Name

		if _, ok := seenTypes[name]; !ok {
			seenTypes[name] = make(map[string]bool)
		}
		seenTypes[name][q.request.Type] = true

		if q.depth > opts.MaxDepth {
			return nil, fmt.Errorf("dependency depth %d exceeds max %d for package %s",
				q.depth, opts.MaxDepth, name)
		}

		if len(resolved) >= opts.MaxPackages {
			return nil, fmt.Errorf("%d packages exceeds max %d",
				len(resolved)+1, opts.MaxPackages)
		}

		for i, p := range q.path {
			if p == name {
				cyclePath := append(q.path[i:], name)
				return nil, fmt.Errorf("circular dependency: %s",
					strings.Join(cyclePath, " → "))
			}
		}

		if existingVersion, ok := versionByConstraint[name]; ok {
			vc, err := ParseConstraint(q.request.Constraint)
			if err != nil {
				return nil, fmt.Errorf("package %s: %w", name, err)
			}
			if vc.Satisfies(existingVersion) {
				continue
			}

			existingConstraint := resolved[name].Version
			if existingConstraint == "" {
				existingConstraint = "*"
			}
			intersection, err := ConstraintIntersection(existingConstraint, q.request.Constraint)
			if err != nil {
				return nil, fmt.Errorf("version conflict: %s requires %s@%s, but %s already resolved %s@%s",
					q.parent, name, q.request.Constraint, name, name, existingVersion)
			}

			existingPkg := resolved[name]
			srcURL, err := types.ParseSourceURL(existingPkg.Source)
			if err != nil {
				return nil, fmt.Errorf("package %s: %w", name, err)
			}
			handler, err := r.getHandler(srcURL.Scheme)
			if err != nil {
				return nil, fmt.Errorf("package %s: %w", name, err)
			}
			versions, err := handler.ListVersions(srcURL)
			if err != nil {
				return nil, fmt.Errorf("package %s: listing versions: %w", name, err)
			}
			intersectVC, err := ParseConstraint(intersection)
			if err != nil {
				return nil, fmt.Errorf("package %s: %w", name, err)
			}
			newVersion, err := HighestCompatible(versions, intersectVC)
			if err != nil {
				return nil, fmt.Errorf("version conflict: %s requires %s@%s, but %s already resolved %s@%s (%s and %s are incompatible with available versions)",
					q.parent, name, q.request.Constraint, name, name, existingVersion, existingConstraint, q.request.Constraint)
			}

			_, sha256, err := handler.Fetch(srcURL, newVersion)
			if err != nil {
				return nil, fmt.Errorf("package %s: fetching version %s: %w", name, newVersion, err)
			}

			existingPkg.Resolved = newVersion
			existingPkg.SHA256 = sha256
			resolved[name] = existingPkg
			versionByConstraint[name] = newVersion
			continue
		}

		srcURL, err := types.ParseSourceURL(q.request.Source)
		if err != nil {
			return nil, fmt.Errorf("package %s: invalid source %q: %w", name, q.request.Source, err)
		}

		handler, err := r.getHandler(srcURL.Scheme)
		if err != nil {
			return nil, fmt.Errorf("package %s: %w", name, err)
		}

		versions, err := handler.ListVersions(srcURL)
		if err != nil {
			return nil, fmt.Errorf("package %s: listing versions: %w", name, err)
		}

		constraint := q.request.Constraint
		vc, err := ParseConstraint(constraint)
		if err != nil {
			return nil, fmt.Errorf("package %s: %w", name, err)
		}

		resolvedVersion, err := HighestCompatible(versions, vc)
		if err != nil {
			if len(versions) == 0 && vc.IsLatest {
				// No versions found (e.g., GitHub repo without tags).
				// Treat as "latest" — fetch default branch content.
				resolvedVersion = "latest"
			} else {
				return nil, fmt.Errorf("package %s: no version satisfies %q among %v: %w",
					name, constraint, versions, err)
			}
		}

		data, sha256, err := handler.Fetch(srcURL, resolvedVersion)
		if err != nil {
			return nil, fmt.Errorf("package %s: fetching version %s: %w",
				name, resolvedVersion, err)
		}

		// Fetch returns raw package data (tar.gz for GitHub/npm),
		// not agentpkg.yaml content. Extract the manifest from
		// the archive, then parse it. If extraction fails, skip
		// dependency resolution with a warning.
		var spec *types.AgentSpec
		manifestData, manifestErr := extractManifestFromTarGz(data)
		if manifestErr != nil {
			warnings = append(warnings,
				fmt.Sprintf("package %s: no agentpkg.yaml found (will skip transitive deps)", name))
		} else {
			var parseErr error
			spec, parseErr = parser.ParseAgentPkg(manifestData)
			if parseErr != nil {
				warnings = append(warnings,
					fmt.Sprintf("package %s: cannot parse agentpkg.yaml (will skip transitive deps): %v", name, parseErr))
			}
		}

		versionByConstraint[name] = resolvedVersion

		rp := ResolvedPackage{
			Name:     name,
			Type:     q.request.Type,
			Source:   q.request.Source,
			Version:  constraint,
			Resolved: resolvedVersion,
			SHA256:   sha256,
		}

		newPath := make([]string, len(q.path)+1)
		copy(newPath, q.path)
		newPath[len(q.path)] = name

		if spec != nil {
			for _, dep := range spec.Dependencies {
				depConstraint := dep.Constraint
				if depConstraint == "" {
					depConstraint = "*"
				}
				depSource := r.depSource(dep.Name, string(dep.Type))
				if dep.Source != "" {
					depSource = dep.Source
				}
				queue = append(queue, queuedRequest{
					request: PackageRequest{
						Name:       dep.Name,
						Type:       string(dep.Type),
						Source:     depSource,
						Constraint: depConstraint,
					},
					depth:  q.depth + 1,
					path:   newPath,
					parent: name,
				})
		rp.Dependencies = append(rp.Dependencies, ResolvedDep{
				Name:    dep.Name,
				Type:    string(dep.Type),
				Version: depConstraint,
				Source:  depSource,
			})
			}
		}

		resolved[name] = rp
		resolvedOrder = append(resolvedOrder, name)
	}

	for _, name := range resolvedOrder {
		rp := resolved[name]
		for j, dep := range rp.Dependencies {
			if v, ok := versionByConstraint[dep.Name]; ok {
				rp.Dependencies[j].Resolved = v
			}
		}
	}

	result := &ResolutionResult{Warnings: warnings}

	for name, types := range seenTypes {
		if len(types) > 1 {
			typeList := make([]string, 0, len(types))
			for t := range types {
				typeList = append(typeList, t)
			}
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("tool name %q collision: multiple packages with types %v; consider auto-prefixing",
					name, typeList))
		}
	}

	for _, name := range resolvedOrder {
		result.Packages = append(result.Packages, resolved[name])
	}

	result.Duration = time.Since(start)
	return result, nil
}
