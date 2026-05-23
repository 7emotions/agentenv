package resolver

import (
	"testing"
)

func TestNamespaceEncodeName(t *testing.T) {
	tests := []struct {
		pkgType  string
		name     string
		expected string
	}{
		{"skill", "code-reviewer", "skill:code-reviewer"},
		{"mcp", "filesystem", "mcp:filesystem"},
		{"", "", ":"},
		{"tool", "a:b:c", "tool:a:b:c"},
	}

	for _, tc := range tests {
		got := EncodeName(tc.pkgType, tc.name)
		if got != tc.expected {
			t.Errorf("EncodeName(%q, %q) = %q, want %q", tc.pkgType, tc.name, got, tc.expected)
		}
	}
}

func TestNamespaceDecodeName(t *testing.T) {
	t.Run("valid two-part name", func(t *testing.T) {
		pkgType, name, err := DecodeName("skill:code-reviewer")
		if err != nil {
			t.Fatalf("DecodeName() unexpected error: %v", err)
		}
		if pkgType != "skill" {
			t.Errorf("pkgType = %q, want %q", pkgType, "skill")
		}
		if name != "code-reviewer" {
			t.Errorf("name = %q, want %q", name, "code-reviewer")
		}
	})

	t.Run("only first colon is delimiter", func(t *testing.T) {
		pkgType, name, err := DecodeName("a:b:c")
		if err != nil {
			t.Fatalf("DecodeName() unexpected error: %v", err)
		}
		if pkgType != "a" {
			t.Errorf("pkgType = %q, want %q", pkgType, "a")
		}
		if name != "b:c" {
			t.Errorf("name = %q, want %q", name, "b:c")
		}
	})

	t.Run("error on missing colon", func(t *testing.T) {
		_, _, err := DecodeName("bad")
		if err == nil {
			t.Fatal("DecodeName() expected error, got nil")
		}
	})

	t.Run("error on empty string", func(t *testing.T) {
		_, _, err := DecodeName("")
		if err == nil {
			t.Fatal("DecodeName() expected error, got nil")
		}
	})
}

func TestNamespaceRoundTrip(t *testing.T) {
	cases := []struct {
		pkgType string
		name    string
	}{
		{"skill", "code-reviewer"},
		{"mcp", "filesystem"},
		{"agent", "my-agent"},
		{"tool", "a:b:c"},
		{"", ""},
	}

	for _, tc := range cases {
		encoded := EncodeName(tc.pkgType, tc.name)
		pkgType, name, err := DecodeName(encoded)
		if err != nil {
			t.Errorf("DecodeName(%q) unexpected error: %v", encoded, err)
			continue
		}
		if pkgType != tc.pkgType {
			t.Errorf("round-trip pkgType = %q, want %q (encoded=%q)", pkgType, tc.pkgType, encoded)
		}
		if name != tc.name {
			t.Errorf("round-trip name = %q, want %q (encoded=%q)", name, tc.name, encoded)
		}
	}
}
