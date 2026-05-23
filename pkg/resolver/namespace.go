package resolver

import (
	"fmt"
	"strings"
)

// EncodeName encodes a package type and name into a single namespace-safe
// identifier of the form "{type}:{name}". This ensures packages with the same
// name but different types (e.g., "skill:code-reviewer" vs "tool:code-reviewer")
// do not collide in PubGrub's flat Name space.
func EncodeName(pkgType, name string) string {
	return fmt.Sprintf("%s:%s", pkgType, name)
}

// DecodeName reverses EncodeName, splitting an encoded identifier back into
// its package type and name components. It returns an error if the encoded
// string does not contain a colon separator.
//
// Only the first colon is used as the delimiter, so names containing colons
// (e.g., "a:b:c") are split into type "a" and name "b:c".
func DecodeName(encoded string) (pkgType, name string, err error) {
	parts := strings.SplitN(encoded, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid encoded name %q: expected format \"{type}:{name}\"", encoded)
	}
	return parts[0], parts[1], nil
}
