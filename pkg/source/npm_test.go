package source

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentenv/agentenv/pkg/types"
)

func TestNPMListVersions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/express") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		info := npmPackageInfo{
			Versions: map[string]npmVersion{
				"1.0.0": {Dist: npmDist{Tarball: "https://example.com/pkg-1.0.0.tgz"}},
				"0.9.0": {Dist: npmDist{Tarball: "https://example.com/pkg-0.9.0.tgz"}},
				"2.0.0": {Dist: npmDist{Tarball: "https://example.com/pkg-2.0.0.tgz"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer server.Close()

	ns := &NPMSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "npm", Name: "express"}

	versions, err := ns.ListVersions(src)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d: %v", len(versions), versions)
	}
	if versions[0] != "2.0.0" {
		t.Errorf("expected 2.0.0 first, got %q", versions[0])
	}
}

func TestNPMListVersionsScoped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "@scope") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		info := npmPackageInfo{
			Versions: map[string]npmVersion{
				"1.0.0": {Dist: npmDist{Tarball: "https://example.com/pkg-1.0.0.tgz"}},
			},
		}
		json.NewEncoder(w).Encode(info)
	}))
	defer server.Close()

	ns := &NPMSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "npm", Scope: "@scope", Name: "pkg"}

	versions, err := ns.ListVersions(src)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
}

func TestNPMListVersionsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	ns := &NPMSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "npm", Name: "nonexistent"}

	_, err := ns.ListVersions(src)
	if err == nil {
		t.Error("expected error for 404")
	}
}

func TestNPMListVersionsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	ns := &NPMSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "npm", Name: "test"}

	_, err := ns.ListVersions(src)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNPMFetch(t *testing.T) {
	tarballServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("tarball-content"))
	}))
	defer tarballServer.Close()

	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ver := npmVersion{
			Dist: npmDist{
				Tarball: tarballServer.URL + "/pkg-1.0.0.tgz",
			},
		}
		json.NewEncoder(w).Encode(ver)
	}))
	defer registryServer.Close()

	ns := &NPMSource{client: registryServer.Client(), baseURL: registryServer.URL}
	src := types.SourceURL{Scheme: "npm", Name: "test-pkg"}

	data, sha256, err := ns.Fetch(src, "1.0.0")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(data) != "tarball-content" {
		t.Errorf("expected 'tarball-content', got %q", string(data))
	}
	if sha256 == "" {
		t.Error("expected non-empty sha256")
	}
}

func TestNPMFetchNoTarball(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ver := npmVersion{}
		json.NewEncoder(w).Encode(ver)
	}))
	defer server.Close()

	ns := &NPMSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "npm", Name: "no-tarball"}

	_, _, err := ns.Fetch(src, "1.0.0")
	if err == nil {
		t.Error("expected error for missing tarball URL")
	}
}

func TestNPMFetchHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ns := &NPMSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "npm", Name: "nonexistent"}

	_, _, err := ns.Fetch(src, "1.0.0")
	if err == nil {
		t.Error("expected error for 404")
	}
}

func TestNPMHTTPTimeout(t *testing.T) {
	ns := &NPMSource{}
	client := ns.httpClient()
	if client.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", client.Timeout)
	}
}

func TestNPMPackageName(t *testing.T) {
	tests := []struct {
		src  types.SourceURL
		want string
	}{
		{types.SourceURL{Name: "express"}, "express"},
		{types.SourceURL{Scope: "@scope", Name: "pkg"}, "@scope/pkg"},
		{types.SourceURL{Name: "lodash"}, "lodash"},
	}

	for _, tt := range tests {
		got := npmPackageName(tt.src)
		if got != tt.want {
			t.Errorf("npmPackageName(%+v) = %q, want %q", tt.src, got, tt.want)
		}
	}
}
