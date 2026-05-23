// Package store provides a content-addressed package store with symlink-based
// deduplication and garbage collection.
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Store is a content-addressed package store. Packages are stored as blobs
// under <root>/<type>/<sourceSlug>/<version> and referenced from environment
// package directories via symlinks (with copy fallback for cross-filesystem).
type Store struct {
	root        string
	envBasePath string
}

// NewStore creates a new Store rooted at the given directory.
// The directory is created if it does not exist.
func NewStore(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create store root: %w", err)
	}
	return &Store{root: root}, nil
}

// Slugify converts a source identifier string into a filesystem-safe slug.
// It replaces '@' with 'at_', '/' with '_', and all other non-alphanumeric
// characters (except '.', '-', '_') with '_'.
func Slugify(raw string) string {
	s := strings.ReplaceAll(raw, "@", "at_")
	s = strings.ReplaceAll(s, "/", "_")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// PackagePath returns the canonical filesystem path for a package in the store.
func (s *Store) PackagePath(pkgType, sourceSlug, version string) string {
	return filepath.Join(s.root, pkgType, sourceSlug, version)
}

// Put writes package data into the store and returns the SHA-256 hex digest.
func (s *Store) Put(pkgType, sourceSlug, version string, data []byte) (string, error) {
	pkgPath := s.PackagePath(pkgType, sourceSlug, version)

	if err := os.MkdirAll(filepath.Dir(pkgPath), 0o755); err != nil {
		return "", fmt.Errorf("create store dir: %w", err)
	}

	h := sha256.Sum256(data)
	digest := hex.EncodeToString(h[:])

	if err := os.WriteFile(pkgPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write package data: %w", err)
	}

	return digest, nil
}

// Get reads package data from the store.
func (s *Store) Get(pkgType, sourceSlug, version string) ([]byte, error) {
	pkgPath := s.PackagePath(pkgType, sourceSlug, version)
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("read package: %w", err)
	}
	return data, nil
}

// Exists reports whether a package exists in the store.
func (s *Store) Exists(pkgType, sourceSlug, version string) bool {
	pkgPath := s.PackagePath(pkgType, sourceSlug, version)
	_, err := os.Stat(pkgPath)
	return err == nil
}

// Link creates a reference from an environment's package directory to a store
// entry. It creates a symlink at <envPath>/packages/<type>/<name> pointing to
// the store package path. If the symlink would cross filesystem boundaries,
// it falls back to copying the data.
func (s *Store) Link(envPath, pkgType, name, sourceSlug, version string) error {
	linkPath := filepath.Join(envPath, "packages", pkgType, name)
	targetPath := s.PackagePath(pkgType, sourceSlug, version)

	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return fmt.Errorf("create link dir: %w", err)
	}

	os.Remove(linkPath)

	if err := os.Symlink(targetPath, linkPath); err != nil {
		if isCrossDeviceErr(err) {
			return copyFile(targetPath, linkPath)
		}
		return fmt.Errorf("symlink: %w", err)
	}
	return nil
}

// Unlink removes the reference from the environment's package directory.
// This only removes the symlink or copy; it does NOT delete the package
// from the store.
func (s *Store) Unlink(envPath, pkgType, name string) error {
	linkPath := filepath.Join(envPath, "packages", pkgType, name)
	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unlink: %w", err)
	}
	return nil
}

// SetEnvBasePath sets the base directory where environment directories live.
// This is used by ReferencedBy and GC to discover environments.
func (s *Store) SetEnvBasePath(path string) {
	s.envBasePath = path
}

// envsPath returns the environment base directory. If SetEnvBasePath has been
// called, its value is used. Otherwise it defaults to <parent-of-store>/envs.
func (s *Store) envsPath() string {
	if s.envBasePath != "" {
		return s.envBasePath
	}
	return filepath.Join(filepath.Dir(s.root), "envs")
}

// GC performs garbage collection: it removes all store entries that are not
// referenced by any symlink in any environment's package directory.
// It returns the list of removed package IDs (type/sourceSlug/version).
func (s *Store) GC() ([]string, error) {
	storeEntries, err := s.listStoreEntries()
	if err != nil {
		return nil, fmt.Errorf("list store entries: %w", err)
	}

	referenced := make(map[string]bool)
	envsDir := s.envsPath()

	entries, err := os.ReadDir(envsDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No environments exist — all store entries are orphans.
			var removed []string
			for _, entry := range storeEntries {
				absPath := filepath.Join(s.root, entry)
				os.RemoveAll(absPath) //nolint:errcheck
				removed = append(removed, entry)
			}
			return removed, nil
		}
		return nil, fmt.Errorf("read envs dir: %w", err)
	}

	for _, envEntry := range entries {
		if !envEntry.IsDir() {
			continue
		}
		pkgDir := filepath.Join(envsDir, envEntry.Name(), "packages")
		_ = s.collectLinks(pkgDir, referenced)
	}

	var removed []string
	for _, entry := range storeEntries {
		if referenced[entry] {
			continue
		}
		absPath := filepath.Join(s.root, entry)
		if err := os.RemoveAll(absPath); err != nil {
			return removed, fmt.Errorf("remove unreferenced %s: %w", entry, err)
		}
		removed = append(removed, entry)
	}

	return removed, nil
}

// ReferencedBy returns the names of environments that reference the given
// store package entry.
func (s *Store) ReferencedBy(pkgType, sourceSlug, version string) ([]string, error) {
	targetPath := s.PackagePath(pkgType, sourceSlug, version)
	var refs []string

	envsDir := s.envsPath()
	entries, err := os.ReadDir(envsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return refs, nil
		}
		return nil, fmt.Errorf("read envs dir: %w", err)
	}

	for _, envEntry := range entries {
		if !envEntry.IsDir() {
			continue
		}
		pkgDir := filepath.Join(envsDir, envEntry.Name(), "packages")
		_ = filepath.WalkDir(pkgDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			link, err := os.Readlink(path)
			if err != nil {
				return nil
			}
			if link == targetPath {
				refs = append(refs, envEntry.Name())
			}
			return nil
		})
	}

	return refs, nil
}

// Size returns the total number of bytes used by the store directory
// (sum of all file sizes).
func (s *Store) Size() (int64, error) {
	var total int64
	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("compute store size: %w", err)
	}
	return total, nil
}

// listStoreEntries returns all store entry paths relative to the store root.
// Each entry is of the form "type/sourceSlug/version".
func (s *Store) listStoreEntries() ([]string, error) {
	var entries []string

	typeDirs, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, err
	}

	for _, typeDir := range typeDirs {
		if !typeDir.IsDir() {
			continue
		}
		sourceDirs, err := os.ReadDir(filepath.Join(s.root, typeDir.Name()))
		if err != nil {
			continue
		}
		for _, sourceDir := range sourceDirs {
			if !sourceDir.IsDir() {
				continue
			}
			versionEntries, err := os.ReadDir(filepath.Join(s.root, typeDir.Name(), sourceDir.Name()))
			if err != nil {
				continue
			}
			for _, versionEntry := range versionEntries {
			if versionEntry.IsDir() {
				continue
			}
				entry := filepath.Join(typeDir.Name(), sourceDir.Name(), versionEntry.Name())
				entries = append(entries, entry)
			}
		}
	}

	return entries, nil
}

// collectLinks walks a directory and records all symlink targets (resolved
// to store-relative paths) in the referenced map.
func (s *Store) collectLinks(dir string, referenced map[string]bool) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || d.Type()&fs.ModeSymlink == 0 {
			return nil
		}
		target, err := os.Readlink(path)
		if err != nil {
			return nil
		}
		// Resolve relative target to absolute using the link's directory.
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		// Convert to a store-relative path.
		rel, err := filepath.Rel(s.root, target)
		if err != nil {
			return nil
		}
		if strings.HasPrefix(rel, "..") {
			return nil // target escapes the store — ignore
		}
		referenced[rel] = true
		return nil
	})
}

// isCrossDeviceErr reports whether err is caused by attempting a symlink
// across filesystem boundaries.
func isCrossDeviceErr(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}

// copyFile copies the regular file at src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}
	return nil
}
