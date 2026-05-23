package source

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/7emotions/agentenv/pkg/types"
)

// LocalSource reads packages from a local filesystem directory.
type LocalSource struct{}

// ListVersions reads agentpkg.yaml from the directory to determine the
// version. If no agentpkg.yaml exists, it returns ["0.0.0-dev"].
func (l *LocalSource) ListVersions(src types.SourceURL) ([]string, error) {
	dir := src.Path
	if dir == "" {
		return nil, fmt.Errorf("local source: empty path")
	}

	apPath := filepath.Join(dir, "agentpkg.yaml")
	data, err := os.ReadFile(apPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{"0.0.0-dev"}, nil
		}
		return nil, fmt.Errorf("local source: read agentpkg.yaml: %w", err)
	}

	version := "0.0.0-dev"
	for _, line := range bytes.Split(data, []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("version:")) {
			v := bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("version:")))
			v = bytes.Trim(v, `"'`)
			if len(v) > 0 {
				version = string(v)
			}
			break
		}
	}

	return []string{version}, nil
}

// Fetch reads all files from the local directory, packs them into a tar.gz,
// and returns the bytes with their SHA-256 digest.
func (l *LocalSource) Fetch(src types.SourceURL, version string) ([]byte, string, error) {
	dir := src.Path
	if dir == "" {
		return nil, "", fmt.Errorf("local source: empty path")
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, "", fmt.Errorf("local source: %w", err)
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("local source: %s is not a directory", dir)
	}

	files := make(map[string][]byte)

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		files[relPath] = data
		return nil
	})
	if err != nil {
		return nil, "", fmt.Errorf("local source: walk: %w", err)
	}

	if len(files) == 0 {
		return nil, "", fmt.Errorf("local source: no files found in %s", dir)
	}

	data, err := packTarGz(files)
	if err != nil {
		return nil, "", fmt.Errorf("local source: pack: %w", err)
	}

	h := sha256.Sum256(data)
	return data, hex.EncodeToString(h[:]), nil
}
