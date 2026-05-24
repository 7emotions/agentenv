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

// extractManifestFromTarGz reads agentpkg.yaml from a tar.gz archive.
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
