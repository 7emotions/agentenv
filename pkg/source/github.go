package source

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/agentenv/agentenv/pkg/types"
)

// semverRe matches a semantic version string optionally prefixed with 'v'.
var semverRe = regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[\w.]+)?(\+[\w.]+)?$`)

// GitHubSource fetches packages from GitHub repositories via the GitHub API.
type GitHubSource struct {
	client  *http.Client
	baseURL string
}

func (g *GitHubSource) httpClient() *http.Client {
	if g.client != nil {
		return g.client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (g *GitHubSource) apiURL() string {
	if g.baseURL != "" {
		return g.baseURL
	}
	return apiBase
}

// apiBase returns the base URL for GitHub API requests.
const apiBase = "https://api.github.com"

type githubTag struct {
	Name string `json:"name"`
}

// ListVersions fetches tags from the GitHub API and returns semver tags
// sorted highest-first.
func (g *GitHubSource) ListVersions(src types.SourceURL) ([]string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/tags?per_page=100", g.apiURL(), src.Owner, src.Repo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("github list tags: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := g.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("github list tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("github list tags: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tags []githubTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("github list tags: decode: %w", err)
	}

	var versions []string
	for _, t := range tags {
		v := strings.TrimPrefix(t.Name, "v")
		if semverRe.MatchString(t.Name) {
			versions = append(versions, v)
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return compareSemver(versions[i], versions[j]) > 0
	})

	return versions, nil
}

// Fetch downloads a tarball for the given version from GitHub, extracts files
// matching src.SubPath, and returns the data and its SHA-256 digest.
func (g *GitHubSource) Fetch(src types.SourceURL, version string) ([]byte, string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/tarball/v%s", g.apiURL(), src.Owner, src.Repo, version)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("github fetch: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := g.httpClient().Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("github fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, "", fmt.Errorf("github fetch: HTTP %d: %s", resp.StatusCode, string(body))
	}

	data, err := extractTarballSubPath(resp.Body, src.SubPath)
	if err != nil {
		return nil, "", fmt.Errorf("github fetch: extract: %w", err)
	}

	h := sha256.Sum256(data)
	return data, hex.EncodeToString(h[:]), nil
}

// extractTarballSubPath reads a gzip-compressed tar stream and extracts all
// files under the given subPath. The files are packed into a new tar.gz in
// memory and returned as bytes. If subPath is empty, the entire tarball is
// returned.
func extractTarballSubPath(r io.Reader, subPath string) ([]byte, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	files := make(map[string][]byte)

	var prefix string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		if prefix == "" {
			idx := strings.IndexByte(hdr.Name, '/')
			if idx > 0 {
				prefix = hdr.Name[:idx+1]
			}
		}

		// Strip the prefix to get the relative path.
		relPath := strings.TrimPrefix(hdr.Name, prefix)

		if subPath != "" {
			trimmed := strings.TrimPrefix(relPath, subPath)
			if trimmed == relPath && !strings.HasPrefix(relPath, subPath+"/") {
				continue
			}
			relPath = strings.TrimPrefix(trimmed, "/")
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[relPath] = data
	}

	if len(files) == 0 {
		if subPath != "" {
			return nil, fmt.Errorf("no files found under subpath %q", subPath)
		}
	}

	return packTarGz(files)
}

// compareSemver compares two semantic version strings. Returns >0 if a > b,
// <0 if a < b, 0 if equal.
func compareSemver(a, b string) int {
	partsA := semverParts(a)
	partsB := semverParts(b)
	for i := 0; i < 3; i++ {
		if partsA[i] != partsB[i] {
			return partsA[i] - partsB[i]
		}
	}
	// Pre-release comparison (simplified: no pre-release < release)
	preA := strings.Contains(a, "-")
	preB := strings.Contains(b, "-")
	if preA && !preB {
		return -1
	}
	if !preA && preB {
		return 1
	}
	return 0
}

// semverParts extracts major, minor, patch as ints.
func semverParts(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}
	var parts [3]int
	_, _ = fmt.Sscanf(v, "%d.%d.%d", &parts[0], &parts[1], &parts[2])
	return parts
}
