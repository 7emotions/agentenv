package envfile

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// kebabCaseRegex matches names in kebab-case: lowercase start, then lowercase/numbers/hyphens.
var kebabCaseRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// supportedSchemes lists source URL schemes accepted by agentenv.
var supportedSchemes = []string{"github", "npm", "local", "git", "file", "url"}

// ValidationResult holds the result of validating an EnvironmentSpec.
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationWarning
}

// ValidationError represents a hard validation failure.
type ValidationError struct {
	Field   string
	Message string
}

// ValidationWarning represents a non-fatal concern.
type ValidationWarning struct {
	Field   string
	Message string
}

// Validate checks the EnvironmentSpec for correctness.
// It returns a result with both errors and warnings.
func Validate(spec *EnvironmentSpec) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if spec == nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spec",
			Message: "EnvironmentSpec is nil",
		})
		return result
	}

	validateName(spec, result)
	validateDescription(spec, result)
	validatePackages("skills", spec.Skills, result)
	validatePackages("mcps", spec.MCPs, result)
	validatePackages("agents", spec.Agents, result)
	validatePackages("tools", spec.Tools, result)
	validatePackages("hooks", spec.Hooks, result)
	validatePackages("prompts", spec.Prompts, result)
	validateDuplicateNames(spec, result)

	return result
}

// ValidateAndWarn validates and returns warnings separately.
// It returns a non-nil error when hard validation errors exist, but not for warnings alone.
func ValidateAndWarn(spec *EnvironmentSpec) (*ValidationResult, error) {
	result := Validate(spec)
	if !result.Valid {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("validation failed with %d error(s)", len(result.Errors)))
		for _, e := range result.Errors {
			sb.WriteString(fmt.Sprintf("\n  - %s: %s", e.Field, e.Message))
		}
		return result, fmt.Errorf("%s", sb.String())
	}
	return result, nil
}

// HasErrors returns true if the result contains any errors.
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if the result contains any warnings.
func (r *ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// validateName checks the environment name field.
func validateName(spec *EnvironmentSpec, result *ValidationResult) {
	if spec.Name == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "name",
			Message: "environment name is required",
		})
		return
	}

	if !kebabCaseRegex.MatchString(spec.Name) {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "name",
			Message: fmt.Sprintf("environment name %q is not valid kebab-case (must match %s)", spec.Name, kebabCaseRegex.String()),
		})
	}
}

// validateDescription checks the description field.
func validateDescription(spec *EnvironmentSpec, result *ValidationResult) {
	if spec.Description == "" {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "description",
			Message: "description is empty; a description is recommended",
		})
	}
}

// validatePackages validates a map of packages (skills, mcps, etc.).
func validatePackages(section string, pkgs map[string]PackageRef, result *ValidationResult) {
	for name, pkg := range pkgs {
		field := fmt.Sprintf("%s.%s", section, name)

		// Validate source
		if pkg.Source == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   field + ".source",
				Message: "source is required",
			})
		} else {
			if err := validateSource(pkg.Source); err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   field + ".source",
					Message: err.Error(),
				})
			}
		}

		// Validate version
		if pkg.Version != "" && pkg.Version != "*" && pkg.Version != "latest" {
			if _, err := semver.NewConstraint(pkg.Version); err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   field + ".version",
					Message: fmt.Sprintf("invalid version constraint %q: %v", pkg.Version, err),
				})
			}
		}

		// Validate package name
		if !kebabCaseRegex.MatchString(name) {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   field,
				Message: fmt.Sprintf("package name %q is not valid kebab-case", name),
			})
		}
	}
}

// validateSource checks that a source URL has a supported scheme.
func validateSource(source string) error {
	idx := strings.Index(source, ":")
	if idx == -1 {
		return fmt.Errorf("source %q is missing a scheme (supported: %s)", source, strings.Join(supportedSchemes, ", "))
	}

	scheme := source[:idx]
	for _, s := range supportedSchemes {
		if scheme == s {
			// Verify it has non-empty content after the separator
			if idx+1 >= len(source) {
				return fmt.Errorf("source %q has empty path after scheme", source)
			}
			return nil
		}
	}

	return fmt.Errorf("unsupported source scheme %q in %q (supported: %s)", scheme, source, strings.Join(supportedSchemes, ", "))
}

// validateDuplicateNames checks for duplicate package names across different sections.
func validateDuplicateNames(spec *EnvironmentSpec, result *ValidationResult) {
	type pkgLocation struct {
		section string
		_       struct{} // disallow unkeyed literals
	}

	seen := make(map[string][]string)

	for name := range spec.Skills {
		seen[name] = append(seen[name], "skills")
	}
	for name := range spec.MCPs {
		seen[name] = append(seen[name], "mcps")
	}
	for name := range spec.Agents {
		seen[name] = append(seen[name], "agents")
	}
	for name := range spec.Tools {
		seen[name] = append(seen[name], "tools")
	}
	for name := range spec.Hooks {
		seen[name] = append(seen[name], "hooks")
	}
	for name := range spec.Prompts {
		seen[name] = append(seen[name], "prompts")
	}

	for name, sections := range seen {
		if len(sections) > 1 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   name,
				Message: fmt.Sprintf("package %q appears in multiple sections: %s", name, strings.Join(sections, ", ")),
			})
		}
	}
}
