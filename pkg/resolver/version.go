package resolver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// VersionConstraint represents a parsed version constraint.
type VersionConstraint struct {
	Raw        string
	Constraint *semver.Constraints // nil for "latest" and "*"
	IsLatest   bool                // true if constraint was "latest" or "*"
}

// ParseConstraint parses a version constraint string.
//
// Supported formats:
//
//	^1.2.3       caret range (semver-native)
//	~1.2.3       tilde range (semver-native)
//	>=1.0 <2.0   range with comparison operators
//	1.2.3        exact version match
//	latest       always satisfied
//	*            always satisfied
func ParseConstraint(raw string) (VersionConstraint, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return VersionConstraint{}, fmt.Errorf("empty version constraint")
	}

	if trimmed == "latest" || trimmed == "*" {
		return VersionConstraint{Raw: trimmed, IsLatest: true}, nil
	}

	c, err := semver.NewConstraint(trimmed)
	if err != nil {
		return VersionConstraint{}, fmt.Errorf("invalid version constraint %q: %w", raw, err)
	}

	return VersionConstraint{Raw: trimmed, Constraint: c}, nil
}

// Satisfies returns true if the version satisfies the constraint.
// "latest" and "*" satisfy any version.
// Pre-releases only match if the constraint explicitly requests a pre-release
// version (semver's built-in behavior).
func (vc VersionConstraint) Satisfies(version string) bool {
	if vc.IsLatest {
		return true
	}
	if vc.Constraint == nil {
		return false
	}
	v, err := semver.NewVersion(version)
	if err != nil {
		return false
	}
	return vc.Constraint.Check(v)
}

// SortVersions sorts version strings in descending order with non-prerelease
// versions first. For example: ["1.0.0-alpha", "1.0.0", "2.0.0", "1.5.0"]
// becomes ["2.0.0", "1.5.0", "1.0.0", "1.0.0-alpha"].
func SortVersions(versions []string) []string {
	vs := make([]*semver.Version, 0, len(versions))
	for _, raw := range versions {
		v, err := semver.NewVersion(raw)
		if err != nil {
			continue
		}
		vs = append(vs, v)
	}

	// Separate stable and pre-release
	stable := make([]*semver.Version, 0, len(vs))
	pre := make([]*semver.Version, 0)
	for _, v := range vs {
		if v.Prerelease() == "" {
			stable = append(stable, v)
		} else {
			pre = append(pre, v)
		}
	}

	// Sort each group descending
	sort.Sort(sort.Reverse(semver.Collection(stable)))
	sort.Sort(sort.Reverse(semver.Collection(pre)))

	result := make([]string, 0, len(versions))
	for _, v := range stable {
		result = append(result, v.String())
	}
	for _, v := range pre {
		result = append(result, v.String())
	}
	return result
}

// HighestVersion returns the highest non-prerelease version from a list.
func HighestVersion(versions []string) (string, error) {
	if len(versions) == 0 {
		return "", fmt.Errorf("no versions provided")
	}

	var best *semver.Version
	for _, raw := range versions {
		v, err := semver.NewVersion(raw)
		if err != nil {
			continue
		}
		if v.Prerelease() != "" {
			continue
		}
		if best == nil || v.GreaterThan(best) {
			best = v
		}
	}

	if best == nil {
		return "", fmt.Errorf("no non-prerelease versions found")
	}

	return best.String(), nil
}

// HighestCompatible returns the highest version from the list that satisfies
// the given constraint. Pre-releases are only considered if explicitly
// requested by the constraint.
func HighestCompatible(versions []string, constraint VersionConstraint) (string, error) {
	sorted := SortVersions(versions)
	for _, v := range sorted {
		if constraint.Satisfies(v) {
			return v, nil
		}
	}
	return "", fmt.Errorf("no version satisfies constraint %q", constraint.Raw)
}

// ConstraintIntersection finds the intersection of two constraints.
// Returns the overlapping constraint range as a string, or an error if
// the constraints do not overlap.
//
// Example:
//
//	"^1.0.0" ∩ ">=1.5 <2.0" → ">=1.5.0, <2.0.0"
//	"^1.0.0" ∩ "^2.0.0" → error (no overlap)
func ConstraintIntersection(a, b string) (string, error) {
	ca, err := semver.NewConstraint(a)
	if err != nil {
		return "", fmt.Errorf("parsing first constraint %q: %w", a, err)
	}
	cb, err := semver.NewConstraint(b)
	if err != nil {
		return "", fmt.Errorf("parsing second constraint %q: %w", b, err)
	}

	lower := findIntersectionLower(ca, cb)
	if lower == nil {
		return "", fmt.Errorf("constraints %q and %q do not overlap", a, b)
	}

	upper := findIntersectionUpper(ca, cb, lower)
	if upper == nil {
		return "", fmt.Errorf("constraints %q and %q do not overlap", a, b)
	}

	return fmt.Sprintf(">=%s, <%s", lower.String(), upper.String()), nil
}

// findIntersectionLower finds the lowest (non-prerelease) version that
// satisfies both constraints. Returns nil if no version satisfies both.
func findIntersectionLower(ca, cb *semver.Constraints) *semver.Version {
	for major := 0; major < 1000; major++ {
		for minor := 0; minor < 500; minor++ {
			vStr := fmt.Sprintf("%d.%d.0", major, minor)
			v, err := semver.NewVersion(vStr)
			if err != nil {
				continue
			}
			if ca.Check(v) && cb.Check(v) {
				for patch := 0; patch < 500; patch++ {
					vpStr := fmt.Sprintf("%d.%d.%d", major, minor, patch)
					vp, err := semver.NewVersion(vpStr)
					if err != nil {
						continue
					}
					if ca.Check(vp) && cb.Check(vp) {
						return vp
					}
				}
				return v
			}
		}
	}
	return nil
}

// findIntersectionUpper finds the version just beyond the highest version
// that satisfies both constraints. This is the exclusive upper bound.
// It searches upward from lower to find where constraints stop being satisfied.
func findIntersectionUpper(ca, cb *semver.Constraints, lower *semver.Version) *semver.Version {
	startMajor := int(lower.Major())

	// Walk upward by major. For each major, check the NEXT major's base version.
	// If the next major base doesn't satisfy, the upper bound is within current major.
	for major := startMajor; major < 1000; major++ {
		nextMajor, _ := semver.NewVersion(fmt.Sprintf("%d.0.0", major+1))
		if ca.Check(nextMajor) && cb.Check(nextMajor) {
			continue
		}
		return highestInMajor(ca, cb, major)
	}
	return nil
}

// highestInMajor finds the exclusive upper bound within a given major version.
// It searches from the highest possible minor/patch downward.
func highestInMajor(ca, cb *semver.Constraints, major int) *semver.Version {
	for minor := 999; minor >= 0; minor-- {
		minv, _ := semver.NewVersion(fmt.Sprintf("%d.%d.0", major, minor))
		if ca.Check(minv) && cb.Check(minv) {
			for patch := 999; patch >= 0; patch-- {
				pv, _ := semver.NewVersion(fmt.Sprintf("%d.%d.%d", major, minor, patch))
				if ca.Check(pv) && cb.Check(pv) {
					next := pv.IncPatch()
					return &next
				}
			}
		}
	}
	return nil
}
