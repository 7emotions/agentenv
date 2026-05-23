package resolver

import (
	"context"
	"errors"
	"log"
)

// Logger is the minimal interface used by PubGrubResolver for logging.
// Standard *log.Logger satisfies this interface.
type Logger interface {
	Printf(format string, v ...interface{})
}

// PubGrubResolver resolves dependencies using the PubGrub algorithm.
// It implements the DependencyResolver interface.
type PubGrubResolver struct {
	depSource func(name, pkgType string) string
	maxSteps  int
	logger    Logger
}

// PubGrubResolverOption configures a PubGrubResolver.
type PubGrubResolverOption func(*PubGrubResolver)

// WithDepSource sets the function that constructs a source URL from a
// dependency name and type. If nil, defaults to "github:test/<name>".
func WithDepSource(fn func(name, pkgType string) string) PubGrubResolverOption {
	return func(r *PubGrubResolver) {
		r.depSource = fn
	}
}

// WithMaxSteps sets the maximum number of resolution steps before aborting.
func WithMaxSteps(n int) PubGrubResolverOption {
	return func(r *PubGrubResolver) {
		r.maxSteps = n
	}
}

// WithLogger sets the logger for resolution progress and warnings.
func WithLogger(l Logger) PubGrubResolverOption {
	return func(r *PubGrubResolver) {
		r.logger = l
	}
}

const defaultMaxSteps = 10000

// NewPubGrubResolver creates a new PubGrubResolver with the given options.
func NewPubGrubResolver(opts ...PubGrubResolverOption) *PubGrubResolver {
	r := &PubGrubResolver{
		maxSteps: defaultMaxSteps,
		logger:   log.Default(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Resolve resolves the given package requests into a complete dependency graph.
func (r *PubGrubResolver) Resolve(ctx context.Context, requests []PackageRequest, opts ResolveOptions) (*ResolutionResult, error) {
	return nil, errors.New("PubGrubResolver: Resolve not yet implemented")
}

// Compile-time check that PubGrubResolver satisfies the DependencyResolver interface.
var _ DependencyResolver = (*PubGrubResolver)(nil)
