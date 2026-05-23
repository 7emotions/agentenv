package source

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/7emotions/agentenv/pkg/types"
)

// NPMSource fetches packages from the npm registry.
type NPMSource struct {
	client    *http.Client
	baseURL   string
}

func (n *NPMSource) httpClient() *http.Client {
	if n.client != nil {
		return n.client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (n *NPMSource) registryURL() string {
	if n.baseURL != "" {
		return n.baseURL
	}
	return npmRegistry
}

const npmRegistry = "https://registry.npmjs.org"

type npmPackageInfo struct {
	Versions map[string]npmVersion `json:"versions"`
}

type npmVersion struct {
	Dist npmDist `json:"dist"`
}

type npmDist struct {
	Tarball string `json:"tarball"`
	SHA256  string `json:"shasum"` // npm returns sha1 as "shasum"
	Integrity string `json:"integrity"` // e.g., "sha512-..."
}

// npmPackageName builds the full npm package name from the source URL.
func npmPackageName(src types.SourceURL) string {
	if src.Scope != "" {
		return src.Scope + "/" + src.Name
	}
	return src.Name
}

// ListVersions fetches version information from the npm registry and returns
// versions sorted highest-first.
func (n *NPMSource) ListVersions(src types.SourceURL) ([]string, error) {
	pkgName := npmPackageName(src)
	url := fmt.Sprintf("%s/%s", n.registryURL(), pkgName)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("npm list versions: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := n.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("npm list versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("npm list versions: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var info npmPackageInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("npm list versions: decode: %w", err)
	}

	versions := make([]string, 0, len(info.Versions))
	for v := range info.Versions {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareSemver(versions[i], versions[j]) > 0
	})

	return versions, nil
}

// Fetch downloads the tarball for a specific version from npm, optionally
// extracts a subpath, and returns the data with its SHA-256 digest.
func (n *NPMSource) Fetch(src types.SourceURL, version string) ([]byte, string, error) {
	pkgName := npmPackageName(src)
	url := fmt.Sprintf("%s/%s/%s", n.registryURL(), pkgName, version)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("npm fetch: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := n.httpClient().Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("npm fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, "", fmt.Errorf("npm fetch: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var ver npmVersion
	if err := json.NewDecoder(resp.Body).Decode(&ver); err != nil {
		return nil, "", fmt.Errorf("npm fetch: decode: %w", err)
	}

	if ver.Dist.Tarball == "" {
		return nil, "", fmt.Errorf("npm fetch: no tarball URL for %s@%s", pkgName, version)
	}

	tarballReq, err := http.NewRequest(http.MethodGet, ver.Dist.Tarball, nil)
	if err != nil {
		return nil, "", fmt.Errorf("npm fetch tarball: %w", err)
	}

	tarballResp, err := n.httpClient().Do(tarballReq)
	if err != nil {
		return nil, "", fmt.Errorf("npm fetch tarball: %w", err)
	}
	defer tarballResp.Body.Close()

	if tarballResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(tarballResp.Body, 1024))
		return nil, "", fmt.Errorf("npm fetch tarball: HTTP %d: %s", tarballResp.StatusCode, string(body))
	}

	if src.SubPath != "" {
		data, err := extractTarballSubPath(tarballResp.Body, "package/"+src.SubPath)
		if err != nil {
			return nil, "", fmt.Errorf("npm fetch extract: %w", err)
		}
		h := sha256.Sum256(data)
		return data, hex.EncodeToString(h[:]), nil
	}

	data, err := io.ReadAll(tarballResp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("npm fetch read: %w", err)
	}

	h := sha256.Sum256(data)
	return data, hex.EncodeToString(h[:]), nil
}
