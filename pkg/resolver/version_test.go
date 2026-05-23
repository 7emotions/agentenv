package resolver

import (
	"strings"
	"testing"
)

func TestParseConstraint(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		isLatest  bool
	}{
		{"caret", "^1.2.3", false, false},
		{"tilde", "~1.2.3", false, false},
		{"range", ">=1.0 <2.0", false, false},
		{"range with comma", ">=1.0, <2.0", false, false},
		{"exact", "1.2.3", false, false},
		{"latest", "latest", false, true},
		{"star", "*", false, true},
		{"empty", "", true, false},
		{"garbage", "not-a-constraint", true, false},
		{"caret zero", "^0.2.3", false, false},
		{"tilde with x", "~1.2.x", false, false},
		{"exact with v", "v1.2.3", false, false},
		{"range with prerelease", ">=1.0.0-alpha", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc, err := ParseConstraint(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseConstraint(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseConstraint(%q) unexpected error: %v", tt.input, err)
				return
			}
			if vc.IsLatest != tt.isLatest {
				t.Errorf("ParseConstraint(%q).IsLatest = %v, want %v", tt.input, vc.IsLatest, tt.isLatest)
			}
			if vc.Raw != tt.input {
				t.Errorf("ParseConstraint(%q).Raw = %q", tt.input, vc.Raw)
			}
		})
	}
}

func TestSatisfies(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		version    string
		want       bool
	}{
		// Caret tests
		{"caret: exact match", "^1.2.3", "1.2.3", true},
		{"caret: higher patch", "^1.2.3", "1.2.5", true},
		{"caret: higher minor", "^1.2.3", "1.5.0", true},
		{"caret: highest in range", "^1.2.3", "1.99.99", true},
		{"caret: major bump", "^1.2.3", "2.0.0", false},
		{"caret: below range", "^1.2.3", "0.9.0", false},
		{"caret: 1.0.0 exact", "^1.0.0", "1.0.0", true},
		{"caret: 1.0.0 higher", "^1.0.0", "1.5.0", true},

		// Tilde tests
		{"tilde: exact match", "~1.2.3", "1.2.3", true},
		{"tilde: higher patch", "~1.2.3", "1.2.9", true},
		{"tilde: minor bump", "~1.2.3", "1.3.0", false},
		{"tilde: below range", "~1.2.3", "1.2.2", false},

		// Range tests
		{"range: lower bound", ">=1.0 <2.0", "1.0.0", true},
		{"range: middle", ">=1.0 <2.0", "1.5.0", true},
		{"range: upper bound exclusive", ">=1.0 <2.0", "2.0.0", false},
		{"range: below", ">=1.0 <2.0", "0.9.0", false},
		{"range: equal lower", ">=1.5.0 <2.0.0", "1.5.0", true},
		{"range: equal upper excluded", ">=1.5.0 <2.0.0", "2.0.0", false},

		// Range with comma
		{"range comma: satisfies", ">=1.0, <3.0", "2.0.0", true},
		{"range comma: outside", ">=1.0, <3.0", "3.0.0", false},

		// Exact tests
		{"exact: matches", "1.2.3", "1.2.3", true},
		{"exact: no match patch", "1.2.3", "1.2.4", false},
		{"exact: no match minor", "1.2.3", "1.3.3", false},
		{"exact: no match major", "1.2.3", "2.2.3", false},

		// Latest and star
		{"latest: any version", "latest", "9.9.9", true},
		{"latest: prerelease", "latest", "1.0.0-alpha", true},
		{"star: any version", "*", "0.1.0", true},
		{"star: high version", "*", "999.0.0", true},

		// Pre-release tests
		{"caret: no prerelease match", "^1.0.0", "1.0.0-alpha", false},
		{"caret: no prerelease match 2", "^1.2.3", "1.2.3-beta", false},
		{"exact: no prerelease match without prerelease", "1.0.0", "1.0.0-alpha", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc, err := ParseConstraint(tt.constraint)
			if err != nil {
				t.Fatalf("ParseConstraint(%q) error: %v", tt.constraint, err)
			}
			got := vc.Satisfies(tt.version)
			if got != tt.want {
				t.Errorf("(%q).Satisfies(%q) = %v, want %v", tt.constraint, tt.version, got, tt.want)
			}
		})
	}
}

func TestSortVersions(t *testing.T) {
	input := []string{"1.0.0-alpha", "1.0.0", "2.0.0", "1.5.0", "0.9.0", "2.0.0-beta"}
	got := SortVersions(input)

	if len(got) != len(input) {
		t.Fatalf("SortVersions returned %d elements, want %d", len(got), len(input))
	}

	// First should be highest stable: 2.0.0
	if got[0] != "2.0.0" {
		t.Errorf("SortVersions[0] = %q, want 2.0.0", got[0])
	}
	// Second: 1.5.0
	if got[1] != "1.5.0" {
		t.Errorf("SortVersions[1] = %q, want 1.5.0", got[1])
	}
	// Third: 1.0.0
	if got[2] != "1.0.0" {
		t.Errorf("SortVersions[2] = %q, want 1.0.0", got[2])
	}
	// Fourth: 0.9.0
	if got[3] != "0.9.0" {
		t.Errorf("SortVersions[3] = %q, want 0.9.0", got[3])
	}

	// Pre-releases come after stable, in descending order: 2.0.0-beta before 1.0.0-alpha
	if got[4] != "2.0.0-beta" {
		t.Errorf("SortVersions[4] = %q, want 2.0.0-beta", got[4])
	}
	if got[5] != "1.0.0-alpha" {
		t.Errorf("SortVersions[5] = %q, want 1.0.0-alpha", got[5])
	}
}

func TestSortVersions_Empty(t *testing.T) {
	got := SortVersions([]string{})
	if len(got) != 0 {
		t.Errorf("SortVersions of empty = %v, want empty", got)
	}
}

func TestSortVersions_Invalid(t *testing.T) {
	// Invalid versions should be silently skipped
	input := []string{"not-a-version", "1.0.0", "garbage"}
	got := SortVersions(input)
	if len(got) != 1 {
		t.Errorf("SortVersions with invalids returned %d elements, want 1", len(got))
	}
	if got[0] != "1.0.0" {
		t.Errorf("SortVersions[0] = %q, want 1.0.0", got[0])
	}
}

func TestSortVersions_OnlyPrerelease(t *testing.T) {
	input := []string{"1.0.0-beta", "1.0.0-alpha"}
	got := SortVersions(input)
	if len(got) != 2 {
		t.Fatalf("SortVersions returned %d elements, want 2", len(got))
	}
	// Descending: beta > alpha
	if got[0] != "1.0.0-beta" {
		t.Errorf("SortVersions[0] = %q, want 1.0.0-beta", got[0])
	}
	if got[1] != "1.0.0-alpha" {
		t.Errorf("SortVersions[1] = %q, want 1.0.0-alpha", got[1])
	}
}

func TestHighestVersion(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		want     string
		wantErr  bool
	}{
		{
			name:     "normal",
			versions: []string{"1.0.0", "2.0.0", "1.5.0"},
			want:     "2.0.0",
		},
		{
			name:     "with prerelease ignored",
			versions: []string{"1.0.0-alpha", "1.0.0", "2.0.0", "1.5.0"},
			want:     "2.0.0",
		},
		{
			name:     "all prerelease",
			versions: []string{"1.0.0-alpha", "2.0.0-beta"},
			wantErr:  true,
		},
		{
			name:    "empty",
			versions: []string{},
			wantErr: true,
		},
		{
			name:     "single",
			versions: []string{"3.2.1"},
			want:     "3.2.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HighestVersion(tt.versions)
			if tt.wantErr {
				if err == nil {
					t.Errorf("HighestVersion(%v) expected error, got nil", tt.versions)
				}
				return
			}
			if err != nil {
				t.Errorf("HighestVersion(%v) unexpected error: %v", tt.versions, err)
				return
			}
			if got != tt.want {
				t.Errorf("HighestVersion(%v) = %q, want %q", tt.versions, got, tt.want)
			}
		})
	}
}

func TestHighestCompatible(t *testing.T) {
	versions := []string{"1.0.0", "1.5.0", "1.2.0", "2.0.0", "1.9.0"}

	tests := []struct {
		name       string
		constraint string
		want       string
		wantErr    bool
	}{
		{"caret 1", "^1.0.0", "1.9.0", false},
		{"exact", "1.2.0", "1.2.0", false},
		{"range", ">=1.5", "2.0.0", false},
		{"no match", "^3.0.0", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc, err := ParseConstraint(tt.constraint)
			if err != nil {
				t.Fatalf("ParseConstraint(%q) error: %v", tt.constraint, err)
			}
			got, err := HighestCompatible(versions, vc)
			if tt.wantErr {
				if err == nil {
					t.Errorf("HighestCompatible(%q) expected error, got %q", tt.constraint, got)
				}
				return
			}
			if err != nil {
				t.Errorf("HighestCompatible(%q) unexpected error: %v", tt.constraint, err)
				return
			}
			if got != tt.want {
				t.Errorf("HighestCompatible(%q) = %q, want %q", tt.constraint, got, tt.want)
			}
		})
	}
}

func TestHighestCompatible_Latest(t *testing.T) {
	vc, _ := ParseConstraint("latest")
	got, err := HighestCompatible([]string{"1.0.0-alpha", "1.0.0", "2.0.0"}, vc)
	if err != nil {
		t.Errorf("HighestCompatible(latest) unexpected error: %v", err)
	}
	// "latest" matches everything, should return highest: 2.0.0
	if got != "2.0.0" {
		t.Errorf("HighestCompatible(latest) = %q, want 2.0.0", got)
	}
}

func TestConstraintIntersection(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		b       string
		wantErr bool
		// Check that the result constraint is satisfied by "check" versions
		// and not by "exclude"
		check   []string
		exclude []string
	}{
		{
			name:    "overlapping ranges",
			a:       "^1.0.0",
			b:       ">=1.5 <2.0",
			check:   []string{"1.5.0", "1.9.0", "1.5.1"},
			exclude: []string{"1.0.0", "1.4.9", "2.0.0", "0.9.0"},
		},
		{
			name:    "overlapping ranges reversed",
			a:       ">=1.5 <2.0",
			b:       "^1.0.0",
			check:   []string{"1.5.0", "1.9.0"},
			exclude: []string{"1.0.0", "2.0.0"},
		},
		{
			name:    "non-overlapping",
			a:       "^1.0.0",
			b:       "^2.0.0",
			wantErr: true,
		},
		{
			name:    "exact with range overlap",
			a:       ">=1.0 <3.0",
			b:       "~2.1.0",
			check:   []string{"2.1.0", "2.1.5"},
			exclude: []string{"1.0.0", "2.2.0", "3.0.0"},
		},
		{
			name:    "exact intersection",
			a:       "1.5.0",
			b:       ">=1.5.0 <1.6.0",
			check:   []string{"1.5.0"},
			exclude: []string{"1.5.1", "1.4.9"},
		},
		{
			name:    "same constraint",
			a:       "^1.0.0",
			b:       "^1.0.0",
			check:   []string{"1.0.0", "1.9.9"},
			exclude: []string{"2.0.0", "0.9.0"},
		},
		{
			name:    "non-overlapping exact",
			a:       "=1.0.0",
			b:       "=2.0.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConstraintIntersection(tt.a, tt.b)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ConstraintIntersection(%q, %q) expected error, got %q", tt.a, tt.b, result)
				}
				return
			}
			if err != nil {
				t.Errorf("ConstraintIntersection(%q, %q) unexpected error: %v", tt.a, tt.b, err)
				return
			}

			// Parse the result to verify it's a valid constraint
			resultVC, parseErr := ParseConstraint(result)
			if parseErr != nil {
				t.Errorf("ConstraintIntersection result %q is not a valid constraint: %v", result, parseErr)
				return
			}

			// Check that included versions satisfy the result
			for _, v := range tt.check {
				if !resultVC.Satisfies(v) {
					t.Errorf("result constraint %q should satisfy %q but does not", result, v)
				}
			}

			// Check that excluded versions do NOT satisfy the result
			for _, v := range tt.exclude {
				if resultVC.Satisfies(v) {
					t.Errorf("result constraint %q should NOT satisfy %q but does", result, v)
				}
			}

			// Verify result satisfies both original constraints
			aVC, _ := ParseConstraint(tt.a)
			bVC, _ := ParseConstraint(tt.b)
			for _, v := range tt.check {
				if !aVC.Satisfies(v) || !bVC.Satisfies(v) {
					t.Fatalf("test setup error: %q should satisfy both %q and %q", v, tt.a, tt.b)
				}
			}
		})
	}
}

func TestConstraintIntersection_InvalidInput(t *testing.T) {
	_, err := ConstraintIntersection("not-valid", "^1.0.0")
	if err == nil {
		t.Error("expected error for invalid first constraint")
	}

	_, err = ConstraintIntersection("^1.0.0", "not-valid")
	if err == nil {
		t.Error("expected error for invalid second constraint")
	}
}

func TestSatisfies_InvalidVersion(t *testing.T) {
	vc, _ := ParseConstraint("^1.0.0")
	if vc.Satisfies("not-a-version") {
		t.Error("Satisfies should return false for invalid version")
	}
}

func TestSatisfies_NilConstraint(t *testing.T) {
	// A VersionConstraint with nil Constraint and IsLatest=false should return false
	vc := VersionConstraint{Raw: "broken", IsLatest: false}
	if vc.Satisfies("1.0.0") {
		t.Error("nil Constraint should not satisfy any version")
	}
}

func TestHighestCompatible_EmptyList(t *testing.T) {
	vc, _ := ParseConstraint("^1.0.0")
	_, err := HighestCompatible([]string{}, vc)
	if err == nil {
		t.Error("expected error for empty version list")
	}
}

func TestSortVersions_PreservesInvalid(t *testing.T) {
	// Invalid versions like "v1.2.3" (with v prefix) are valid per semver NewVersion
	// but versions like "abc" are not and should be dropped
	input := []string{"1.0.0", "abc", "2.0.0"}
	got := SortVersions(input)
	if len(got) != 2 {
		t.Errorf("SortVersions with invalid returned %d elements, want 2: %v", len(got), got)
	}
}

func TestConstraintResultStringFormat(t *testing.T) {
	// Verify the intersection result string is in a valid format
	result, err := ConstraintIntersection(">=1.0 <2.0", "^1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, ">=") {
		t.Errorf("result %q should contain '>='", result)
	}
	if !strings.Contains(result, "<") {
		t.Errorf("result %q should contain '<'", result)
	}

	// Result should be parseable
	vc, err := ParseConstraint(result)
	if err != nil {
		t.Errorf("result %q not parseable: %v", result, err)
	}
	if !vc.Satisfies("1.5.0") {
		t.Errorf("result %q should satisfy 1.5.0", result)
	}
}
