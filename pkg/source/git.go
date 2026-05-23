package source

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/agentenv/agentenv/pkg/types"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GitSource fetches packages from a git repository.
type GitSource struct{}

// gitSemverRe matches a semantic version string optionally prefixed with 'v'.
var gitSemverRe = regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[\w.]+)?(\+[\w.]+)?$`)

// ListVersions clones the repository shallowly and lists tags matching the
// semver pattern, sorted highest-first.
func (g *GitSource) ListVersions(src types.SourceURL) ([]string, error) {
	dir, err := os.MkdirTemp("", "agentenv-git-")
	if err != nil {
		return nil, fmt.Errorf("git list versions: %w", err)
	}
	defer os.RemoveAll(dir)

	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:   src.URL,
		Depth: 1,
		Tags:  git.AllTags,
	})
	if err != nil {
		return nil, fmt.Errorf("git list versions: clone: %w", err)
	}

	tagRefs, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("git list versions: tags: %w", err)
	}

	var versions []string
	err = tagRefs.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		if gitSemverRe.MatchString(name) {
			versions = append(versions, strings.TrimPrefix(name, "v"))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("git list versions: iterate tags: %w", err)
	}

	sort.Slice(versions, func(i, j int) bool {
		return compareSemver(versions[i], versions[j]) > 0
	})

	return versions, nil
}

// Fetch clones the repository, checks out the given tag, reads all files
// (optionally filtered by src.SubPath), packs them into a tar.gz, and returns
// the bytes with SHA-256.
func (g *GitSource) Fetch(src types.SourceURL, version string) ([]byte, string, error) {
	dir, err := os.MkdirTemp("", "agentenv-git-fetch-")
	if err != nil {
		return nil, "", fmt.Errorf("git fetch: %w", err)
	}
	defer os.RemoveAll(dir)

	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:           src.URL,
		Depth:         1,
		SingleBranch:  true,
		ReferenceName: plumbing.NewTagReferenceName(version),
	})
	if err != nil {
		repo, err = git.PlainClone(dir, false, &git.CloneOptions{
			URL:   src.URL,
			Depth: 1,
		})
		if err != nil {
			return nil, "", fmt.Errorf("git fetch: clone: %w", err)
		}
	}

	tagRef, err := repo.Tag("v" + version)
	if err != nil {
		tagRef, err = repo.Tag(version)
		if err != nil {
			return nil, "", fmt.Errorf("git fetch: tag %s not found", version)
		}
	}

	tagObj, err := repo.TagObject(tagRef.Hash())
	var commit *object.Commit
	if err == nil {
		commit, err = tagObj.Commit()
		if err != nil {
			return nil, "", fmt.Errorf("git fetch: tag commit: %w", err)
		}
	} else {
		commit, err = repo.CommitObject(tagRef.Hash())
		if err != nil {
			return nil, "", fmt.Errorf("git fetch: commit: %w", err)
		}
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, "", fmt.Errorf("git fetch: worktree: %w", err)
	}

	if err := wt.Checkout(&git.CheckoutOptions{
		Hash: commit.Hash,
	}); err != nil {
		return nil, "", fmt.Errorf("git fetch: checkout: %w", err)
	}

	files := make(map[string][]byte)
	readRoot := dir
	if src.SubPath != "" {
		readRoot = filepath.Join(dir, src.SubPath)
	}

	err = filepath.Walk(readRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), ".git") {
			return nil
		}

		relPath, err := filepath.Rel(readRoot, path)
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
		return nil, "", fmt.Errorf("git fetch: walk: %w", err)
	}

	if len(files) == 0 {
		return nil, "", fmt.Errorf("git fetch: no files found")
	}

	data, err := packTarGz(files)
	if err != nil {
		return nil, "", fmt.Errorf("git fetch: pack: %w", err)
	}

	h := sha256.Sum256(data)
	return data, hex.EncodeToString(h[:]), nil
}
