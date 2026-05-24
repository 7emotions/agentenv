package resolver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/7emotions/agentenv/pkg/types"
)

type mockSource struct {
	versions map[string][]string
	pkgs     map[string][]byte
	listErr  map[string]error
	fetchErr map[string]error
}

func (m *mockSource) ListVersions(src types.SourceURL) ([]string, error) {
	key := src.String()
	if err, ok := m.listErr[key]; ok {
		return nil, err
	}
	v, ok := m.versions[key]
	if !ok {
		return nil, fmt.Errorf("unknown package: %s", key)
	}
	return v, nil
}

func (m *mockSource) Fetch(src types.SourceURL, version string) ([]byte, string, error) {
	key := src.String()
	if err, ok := m.fetchErr[key]; ok {
		return nil, "", err
	}
	data, ok := m.pkgs[key]
	if !ok {
		return nil, "", fmt.Errorf("unknown package: %s", key)
	}
	h := sha256.Sum256(data)
	return data, fmt.Sprintf("%x", h), nil
}

func makeTarGzManifest(data []byte) []byte {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	hdr := &tar.Header{
		Name: "agentpkg.yaml",
		Size: int64(len(data)),
		Mode: 0644,
	}
	tarWriter.WriteHeader(hdr)
	tarWriter.Write(data)
	tarWriter.Close()
	gzWriter.Close()
	return buf.Bytes()
}

// makeAgentPkg creates a test agentpkg manifest wrapped in a tar.gz archive,
// as returned by real Fetch() implementations.
func makeAgentPkg(name, version string, deps ...string) []byte {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("name: %s\n", name))
	sb.WriteString(fmt.Sprintf("version: %q\n", version))
	sb.WriteString(fmt.Sprintf("source: github:test/%s\n", name))
	if len(deps) > 0 {
		sb.WriteString("dependencies:\n")
	}
	for _, d := range deps {
		parts := strings.SplitN(d, "@", 2)
		depName := parts[0]
		constraint := "*"
		if len(parts) > 1 {
			constraint = parts[1]
		}
		sb.WriteString(fmt.Sprintf("  - name: %s\n", depName))
		sb.WriteString("    type: skill\n")
		sb.WriteString(fmt.Sprintf("    constraint: %q\n", constraint))
	}
	return makeTarGzManifest([]byte(sb.String()))
}

func req(name, source, constraint string) PackageRequest {
	return PackageRequest{
		Name:       name,
		Type:       "skill",
		Source:     source,
		Constraint: constraint,
	}
}
