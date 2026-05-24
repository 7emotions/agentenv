package resolver

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/7emotions/agentenv/pkg/parser"
	"github.com/7emotions/agentenv/pkg/source"
	"github.com/7emotions/agentenv/pkg/types"
	"github.com/contriboss/pubgrub-go"
)

// PubGrubResolver resolves dependencies using the PubGrub algorithm.
// It implements the DependencyResolver interface.
type PubGrubResolver struct {
	handlers  map[string]source.SourceHandler
	depSource func(name, pkgType string) string
	maxSteps  int
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

const defaultMaxSteps = 10000

// NewPubGrubResolver creates a new PubGrubResolver with the given options.
func NewPubGrubResolver(opts ...PubGrubResolverOption) *PubGrubResolver {
	r := &PubGrubResolver{
		maxSteps: defaultMaxSteps,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RegisterHandler registers a custom source handler for a scheme, overriding
// the global source registry for that scheme during resolution.
func (r *PubGrubResolver) RegisterHandler(scheme string, h source.SourceHandler) {
	if r.handlers == nil {
		r.handlers = make(map[string]source.SourceHandler)
	}
	r.handlers[scheme] = h
}

func (r *PubGrubResolver) getHandler(scheme string) (source.SourceHandler, error) {
	if h, ok := r.handlers[scheme]; ok {
		return h, nil
	}
	return source.GetHandler(scheme)
}

// Resolve resolves the given package requests into a complete dependency graph.
func (r *PubGrubResolver) Resolve(ctx context.Context, requests []PackageRequest, opts ResolveOptions) (*ResolutionResult, error) {
	start := time.Now()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Set up depSource with default
	depFn := r.depSource
	if depFn == nil {
		depFn = func(name, pkgType string) string {
			return "github:test/" + name
		}
	}

	// ---------- Phase 1: Pre-handle packages with no versions ----------
	// For "latest" and "*" constraints, if the handler returns zero versions
	// we resolve to the literal string "latest" (matching existing resolver behaviour).
	var remaining []PackageRequest
	preResolved := make(map[string]ResolvedPackage) // keyed by encoded name

	for _, req := range requests {
		constraint := req.Constraint
		if constraint == "" {
			constraint = "*"
		}
		req.Constraint = constraint

		if constraint != "latest" && constraint != "*" {
			remaining = append(remaining, req)
			continue
		}

		srcURL, err := types.ParseSourceURL(req.Source)
		if err != nil {
			return nil, fmt.Errorf("package %s: invalid source %q: %w", req.Name, req.Source, err)
		}
		handler, err := r.getHandler(srcURL.Scheme)
		if err != nil {
			return nil, fmt.Errorf("package %s: %w", req.Name, err)
		}
		versions, err := handler.ListVersions(srcURL)
		if err != nil {
			return nil, fmt.Errorf("package %s: listing versions: %w", req.Name, err)
		}
		if len(versions) == 0 {
			encoded := EncodeName(req.Type, req.Name)
			preResolved[encoded] = ResolvedPackage{
				Name:       req.Name,
				Type:       req.Type,
				Source:     req.Source,
				Version:    constraint,
				Resolved:   "latest",
				ResolvedBy: "(root)",
			}
			continue
		}
		remaining = append(remaining, req)
	}

	warnings := make([]string, 0)
	result := &ResolutionResult{
		Warnings: warnings,
		Duration: time.Since(start),
	}

	// Add pre-resolved packages to the result
	for _, pkg := range preResolved {
		result.Packages = append(result.Packages, pkg)
	}

	if len(remaining) == 0 {
		return result, nil
	}

	// ---------- Phase 2: Build pubgrub sources and solve ----------

	// RootSource wraps the top-level requirements as dependencies of $$root.
	root := pubgrub.NewRootSource()
	requestMap := make(map[string]PackageRequest) // encoded name -> request details

	// sourceURLMap stores the explicit source URL for root packages,
	// so resolverSource can use it instead of the default depSource.
	sourceURLMap := make(map[string]string)

	for _, req := range remaining {
		encoded := EncodeName(req.Type, req.Name)
		requestMap[encoded] = req
		sourceURLMap[encoded] = req.Source

		pkgName := pubgrub.MakeName(encoded)

		// Normalize the constraint to a pubgrub-compatible version range.
		normalized := normalizeConstraint(req.Constraint)
		vs, err := pubgrub.ParseVersionRange(normalized)
		if err != nil {
			return nil, fmt.Errorf("package %s: invalid constraint %q (normalized: %q): %w",
				req.Name, req.Constraint, normalized, err)
		}
		condition := pubgrub.NewVersionSetCondition(vs)
		root.AddPackage(pkgName, condition)
	}

	// The resolverSource dynamically dispatches GetVersions / GetDependencies
	// by decoding the encoded package name and using depSource (or explicit
	// sourceURLMap) to find the handler and source URL.
	resSrc := &resolverSource{
		getHandler:     r.getHandler,
		depSource:      depFn,
		sourceURLMap:   sourceURLMap,
		rootConstraint: requestMap,
	}

	// Build the solver. We enable incompatibility tracking for detailed
	// error messages.
	solver := pubgrub.NewSolver(root, resSrc)
	if r.maxSteps > 0 {
		solver = solver.Configure(pubgrub.WithMaxSteps(r.maxSteps))
	}
	solver = solver.EnableIncompatibilityTracking()

	solution, err := solver.Solve(root.Term())
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Try to extract a NoSolutionError for detailed messaging.
		var noSolErr *pubgrub.NoSolutionError
		if errors.As(err, &noSolErr) {
			return nil, wrapNoSolutionError(noSolErr)
		}
		return nil, fmt.Errorf("resolution failed: %w", err)
	}

	// ---------- Phase 3: Convert solution to ResolvedPackage slice ----------
	// Two-pass: collect packages and parent relationships from manifests,
	// then assign ResolvedBy.

	type pkgBuild struct {
		rp   ResolvedPackage
		deps []ResolvedDep
	}

	buildMap := make(map[string]*pkgBuild)
	parentOf := make(map[string]string)

	seenTypes := make(map[string]map[string]bool)

	for _, nv := range solution {
		encoded := nv.Name.Value()
		// Skip internal root package.
		if strings.HasPrefix(encoded, "$$") {
			continue
		}

		pkgType, pkgName, decErr := DecodeName(encoded)
		if decErr != nil {
			continue
		}

		// Track type collision warnings.
		if _, ok := seenTypes[pkgName]; !ok {
			seenTypes[pkgName] = make(map[string]bool)
		}
		seenTypes[pkgName][pkgType] = true

		// Determine the source URL and constraint for this package.
		req, isRoot := requestMap[encoded]
		sourceStr := depFn(pkgName, pkgType)
		constraint := "*"

		if isRoot {
			sourceStr = req.Source
			constraint = req.Constraint
		}

		// Fetch the package archive to obtain SHA-256 and manifest.
		sha256 := ""
		var spec *types.AgentSpec

		srcURL, parseErr := types.ParseSourceURL(sourceStr)
		if parseErr == nil {
			handler, handlerErr := r.getHandler(srcURL.Scheme)
			if handlerErr == nil {
				data, sha, fetchErr := handler.Fetch(srcURL, nv.Version.String())
				if fetchErr == nil {
					sha256 = sha
					manifestData, manErr := extractManifestFromTarGz(data)
					if manErr == nil {
						spec, _ = parser.ParseAgentPkg(manifestData)
					}
				} else {
					warnings = append(warnings, fmt.Sprintf("package %s: fetching: %v", pkgName, fetchErr))
				}
			}
		}

		rp := ResolvedPackage{
			Name:     pkgName,
			Type:     pkgType,
			Source:   sourceStr,
			Version:  constraint,
			Resolved: nv.Version.String(),
			SHA256:   sha256,
		}

		var deps []ResolvedDep
		if spec != nil {
			for _, dep := range spec.Dependencies {
				depConstraint := dep.Constraint
				if depConstraint == "" {
					depConstraint = "*"
				}
				depSrc := depFn(dep.Name, string(dep.Type))
				if dep.Source != "" {
					depSrc = dep.Source
				}

				depEncoded := EncodeName(string(dep.Type), dep.Name)
				if _, already := parentOf[depEncoded]; !already {
					parentOf[depEncoded] = pkgName
				}

				depResolved := ""
				if dv, found := solution.GetVersion(pubgrub.MakeName(depEncoded)); found {
					depResolved = dv.String()
				}

				deps = append(deps, ResolvedDep{
					Name:     dep.Name,
					Type:     string(dep.Type),
					Version:  depConstraint,
					Resolved: depResolved,
					Source:   depSrc,
				})
			}
		}

		buildMap[encoded] = &pkgBuild{rp: rp, deps: deps}
	}

	// Second pass: assign ResolvedBy using the parentOf map built above.
	for encoded, build := range buildMap {
		_, isRoot := requestMap[encoded]
		if isRoot {
			build.rp.ResolvedBy = "(root)"
		} else if parent, ok := parentOf[encoded]; ok {
			build.rp.ResolvedBy = parent
		}
		build.rp.Dependencies = build.deps
		result.Packages = append(result.Packages, build.rp)
	}

	// Add name-collision warnings.
	for name, types := range seenTypes {
		if len(types) > 1 {
			typeList := make([]string, 0, len(types))
			for t := range types {
				typeList = append(typeList, t)
			}
			warnings = append(warnings,
				fmt.Sprintf("tool name %q collision: multiple packages with types %v; consider auto-prefixing",
					name, typeList))
		}
	}
	result.Warnings = warnings
	result.Duration = time.Since(start)

	return result, nil
}

// ---------------------------------------------------------------------------
// resolverSource — a pubgrub.Source that dispatches by encoded package name
// ---------------------------------------------------------------------------

// resolverSource implements pubgrub.Source by decoding encoded package names
// (e.g. "skill:A") and dynamically resolving them to source handlers via
// depSource or an explicit source URL override map.
type resolverSource struct {
	getHandler     func(string) (source.SourceHandler, error)
	depSource      func(name, pkgType string) string
	sourceURLMap   map[string]string                    // encoded name -> explicit source URL
	rootConstraint map[string]PackageRequest             // encoded name -> request (for latest/* handling)
	versionCache   map[string][]pubgrub.Version          // encoded name -> versions
	depCache       map[string]map[string][]pubgrub.Term  // encoded name -> version string -> terms
}

func (s *resolverSource) resolveSourceURL(name pubgrub.Name) (string, error) {
	encoded := name.Value()
	if url, ok := s.sourceURLMap[encoded]; ok {
		return url, nil
	}
	pkgType, pkgName, err := DecodeName(encoded)
	if err != nil {
		return "", err
	}
	return s.depSource(pkgName, pkgType), nil
}

func (s *resolverSource) getVersionsFromHandler(sourceStr string) ([]pubgrub.Version, error) {
	srcURL, err := types.ParseSourceURL(sourceStr)
	if err != nil {
		return nil, err
	}
	handler, err := s.getHandler(srcURL.Scheme)
	if err != nil {
		return nil, err
	}
	rawVersions, err := handler.ListVersions(srcURL)
	if err != nil {
		return nil, err
	}
	if len(rawVersions) == 0 {
		return []pubgrub.Version{}, nil
	}
	result := make([]pubgrub.Version, 0, len(rawVersions))
	for _, raw := range rawVersions {
		sv, verr := pubgrub.ParseSemanticVersion(raw)
		if verr != nil {
			continue
		}
		result = append(result, sv)
	}
	slices.SortFunc(result, func(a, b pubgrub.Version) int {
		return a.Sort(b)
	})
	return result, nil
}

func (s *resolverSource) GetVersions(name pubgrub.Name) ([]pubgrub.Version, error) {
	encoded := name.Value()

	// Check cache
	if s.versionCache != nil {
		if versions, ok := s.versionCache[encoded]; ok {
			return cloneVersions(versions), nil
		}
	}

	sourceStr, err := s.resolveSourceURL(name)
	if err != nil {
		return nil, &pubgrub.PackageNotFoundError{Package: name}
	}

	versions, err := s.getVersionsFromHandler(sourceStr)
	if err != nil {
		return nil, err
	}

	// Initialize cache
	if s.versionCache == nil {
		s.versionCache = make(map[string][]pubgrub.Version)
	}
	s.versionCache[encoded] = cloneVersions(versions)

	return cloneVersions(versions), nil
}

func (s *resolverSource) GetDependencies(name pubgrub.Name, version pubgrub.Version) ([]pubgrub.Term, error) {
	encoded := name.Value()

	// Check dependency cache
	if s.depCache != nil {
		if byVer, ok := s.depCache[encoded]; ok {
			if terms, ok := byVer[version.String()]; ok {
				return cloneTerms(terms), nil
			}
		}
	}

	sourceStr, err := s.resolveSourceURL(name)
	if err != nil {
		return nil, &pubgrub.PackageVersionNotFoundError{Package: name, Version: version}
	}

	srcURL, parseErr := types.ParseSourceURL(sourceStr)
	if parseErr != nil {
		return nil, &pubgrub.PackageVersionNotFoundError{Package: name, Version: version}
	}

	handler, handlerErr := s.getHandler(srcURL.Scheme)
	if handlerErr != nil {
		return nil, &pubgrub.PackageVersionNotFoundError{Package: name, Version: version}
	}

	data, _, fetchErr := handler.Fetch(srcURL, version.String())
	if fetchErr != nil {
		return nil, fetchErr
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

		normalized := normalizeConstraint(constraint)
		vs, rangeErr := pubgrub.ParseVersionRange(normalized)
		if rangeErr != nil {
			continue
		}
		terms = append(terms, pubgrub.NewTerm(depName, pubgrub.NewVersionSetCondition(vs)))
	}

	if terms == nil {
		terms = []pubgrub.Term{}
	}

	// Store in cache
	if s.depCache == nil {
		s.depCache = make(map[string]map[string][]pubgrub.Term)
	}
	if _, ok := s.depCache[encoded]; !ok {
		s.depCache[encoded] = make(map[string][]pubgrub.Term)
	}
	s.depCache[encoded][version.String()] = cloneTerms(terms)

	return cloneTerms(terms), nil
}

func cloneVersions(in []pubgrub.Version) []pubgrub.Version {
	if len(in) == 0 {
		return nil
	}
	out := make([]pubgrub.Version, len(in))
	copy(out, in)
	return out
}

func cloneTerms(in []pubgrub.Term) []pubgrub.Term {
	if len(in) == 0 {
		return nil
	}
	out := make([]pubgrub.Term, len(in))
	copy(out, in)
	return out
}

// Compile-time check that resolverSource implements pubgrub.Source.
var _ pubgrub.Source = (*resolverSource)(nil)

// ---------------------------------------------------------------------------
// Constraint normalization
// ---------------------------------------------------------------------------

// normalizeConstraint converts a semver-style constraint string (which may
// include caret ^ and tilde ~ operators) into a form that pubgrub-go's
// ParseVersionRange can understand (comparison operators only).
//
// Examples:
//
//	"^1.0"      → ">=1.0.0, <2.0.0"
//	"^1.0.0"    → ">=1.0.0, <2.0.0"
//	">=2.0"     → ">=2.0.0"
//	"<2.0"      → "<2.0.0"
//	"*"         → "*"
//	"latest"    → "*"
//	""          → "*"
//	"1.0.0"     → "==1.0.0"
//	">=1.0, <2.0" → ">=1.0.0, <2.0.0"
func normalizeConstraint(c string) string {
	c = strings.TrimSpace(c)
	if c == "" || c == "*" || c == "latest" {
		return "*"
	}

	// Already starts with a comparison operator → ensure versions are
	// fully qualified (X.Y.Z) so that pubgrub uses SemanticVersion
	// instead of SimpleVersion string comparisons.
	for _, prefix := range []string{">=", "<=", "!=", ">", "<", "==", "="} {
		if strings.HasPrefix(c, prefix) {
			return normalizeOperatorConstraint(c, prefix)
		}
	}

	// Caret ^X.Y.Z → >=X.Y.Z, <(X+1).0.0
	if strings.HasPrefix(c, "^") {
		rest := c[1:]
		return expandCaret(rest)
	}

	// Tilde ~X.Y.Z → >=X.Y.Z, <X.(Y+1).0
	if strings.HasPrefix(c, "~") {
		rest := c[1:]
		return expandTilde(rest)
	}

	// Bare version → exact match
	return "==" + ensurePatchVersion(c)
}

func normalizeOperatorConstraint(c, prefix string) string {
	rest := strings.TrimSpace(c[len(prefix):])
	return prefix + ensurePatchVersion(rest)
}

func ensurePatchVersion(v string) string {
	v = strings.TrimSpace(v)
	parts := strings.Split(v, ".")
	switch len(parts) {
	case 0:
		return "0.0.0"
	case 1:
		return parts[0] + ".0.0"
	case 2:
		return parts[0] + "." + parts[1] + ".0"
	default:
		return v
	}
}

func expandCaret(v string) string {
	v = ensurePatchVersion(v)
	parts := strings.Split(v, ".")
	major, _ := strconv.Atoi(parts[0])
	// For semver ^X.Y.Z means >=X.Y.Z and <(X+1).0.0
	nextMajor := major + 1
	return fmt.Sprintf(">=%s, <%d.0.0", v, nextMajor)
}

func expandTilde(v string) string {
	v = ensurePatchVersion(v)
	parts := strings.Split(v, ".")
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	// For semver ~X.Y.Z means >=X.Y.Z and <X.(Y+1).0
	nextMinor := minor + 1
	return fmt.Sprintf(">=%s, <%d.%d.0", v, major, nextMinor)
}

// Compile-time check that PubGrubResolver satisfies the DependencyResolver interface.
var _ DependencyResolver = (*PubGrubResolver)(nil)
