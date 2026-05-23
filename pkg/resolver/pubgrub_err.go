package resolver

import (
	"fmt"
	"strings"

	agentenvError "github.com/7emotions/agentenv/pkg/errors"
	"github.com/contriboss/pubgrub-go"
)

type conflictDep struct {
	pkgName string
	version string
	depName string
	depCons string
}

// formatConflictError extracts a human-readable conflict chain from a
// NoSolutionError by walking the incompatibility tree to find the
// KindFromDependency leaves that caused the conflict.
//
// Returns the standard pubgrub error message if no structured conflict
// information is available.
func formatConflictError(noSolErr *pubgrub.NoSolutionError) string {
	if noSolErr.Incompatibility == nil {
		return noSolErr.Error()
	}

	deps := extractConflictDeps(noSolErr.Incompatibility)
	if len(deps) < 2 {
		return noSolErr.Error()
	}

	return buildConflictMessage(deps)
}

// extractConflictDeps walks the incompatibility tree and collects
// KindFromDependency leaves, each representing one package's
// dependency that contributed to the conflict.
func extractConflictDeps(inc *pubgrub.Incompatibility) []conflictDep {
	if inc == nil {
		return nil
	}

	if inc.Kind == pubgrub.KindFromDependency && len(inc.Terms) >= 2 {
		pkgName := decodePackageName(inc.Package.Value())
		version := inc.Version.String()

		depTerm := inc.Terms[1]
		if !depTerm.Positive {
			depTerm = depTerm.Negate()
		}
		depName := decodePackageName(depTerm.Name.Value())
		depCons := depTerm.Condition.String()

		return []conflictDep{{
			pkgName: pkgName,
			version: version,
			depName: depName,
			depCons: depCons,
		}}
	}

	if inc.Kind == pubgrub.KindConflict {
		deps := extractConflictDeps(inc.Cause1)
		deps = append(deps, extractConflictDeps(inc.Cause2)...)
		return deps
	}

	return nil
}

// decodePackageName extracts the human-readable package name from an
// encoded pubgrub name (e.g., "skill:A" → "A").
func decodePackageName(encoded string) string {
	_, name, err := DecodeName(encoded)
	if err != nil {
		return encoded
	}
	return name
}

func buildConflictMessage(deps []conflictDep) string {
	parts := make([]string, 0, len(deps))
	for i, d := range deps {
		prefix := ""
		if i == 1 {
			prefix = "but "
		} else if i > 1 {
			prefix = "and "
		}
		parts = append(parts, fmt.Sprintf("%s%s@%s requires %s@%s",
			prefix, d.pkgName, d.version, d.depName, d.depCons))
	}
	return "version conflict: " + strings.Join(parts, ", ")
}

// wrapNoSolutionError wraps a pubgrub NoSolutionError into an agentenv
// SystemError with a user-friendly conflict message.
func wrapNoSolutionError(noSolErr *pubgrub.NoSolutionError) *agentenvError.AgentError {
	msg := formatConflictError(noSolErr)
	return agentenvError.SystemError(msg,
		"check that your dependencies do not have conflicting version requirements")
}
