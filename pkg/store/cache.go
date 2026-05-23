package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Cache provides a simple URL-to-file download cache. Downloaded files are
// stored under <root>/<sha256-of-URL> so repeated downloads of the same URL
// are fast.
type Cache struct {
	root string
}

// NewCache creates a new Cache rooted at the given directory. The directory
// is created if it does not exist.
func NewCache(root string) (*Cache, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create cache root: %w", err)
	}
	return &Cache{root: root}, nil
}

// Download fetches the resource at url and returns the path to the cached
// file. If the URL has already been downloaded, the cached copy is returned
// without making a network request.
func (c *Cache) Download(url string) (string, error) {
	h := sha256.Sum256([]byte(url))
	key := hex.EncodeToString(h[:])
	cachePath := filepath.Join(c.root, key)

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	tmpPath := cachePath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("write download: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpPath, cachePath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename to cache: %w", err)
	}

	return cachePath, nil
}
