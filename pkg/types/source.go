package types

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// SourceURL represents a parsed package source URL.
// Supported schemes: github, npm, local, git, file, url.
type SourceURL struct {
	Raw     string
	Scheme  string // "github", "npm", "local", "git", "file", "url"
	Owner   string // github: owner
	Repo    string // github: repo
	SubPath string // github: path within repo
	Scope   string // npm: @scope
	Name    string // npm: package name
	Path    string // local/file: filesystem path
	URL     string // git/url: remote URL
}

// ParseSourceURL parses a raw source URL string into a SourceURL struct.
// Supported formats:
//
//	github:owner/repo
//	github:owner/repo/path/to/skill
//	npm:@scope/package
//	npm:package
//	local:./path
//	git:https://example.com/repo.git
//	file:/absolute/path
//	url:https://example.com/pkg.tar.gz
func ParseSourceURL(raw string) (SourceURL, error) {
	idx := strings.Index(raw, ":")
	if idx == -1 {
		return SourceURL{}, fmt.Errorf("invalid source URL %q: missing scheme separator", raw)
	}

	scheme := raw[:idx]
	rest := raw[idx+1:]
	if rest == "" {
		return SourceURL{}, fmt.Errorf("invalid source URL %q: empty path", raw)
	}

	switch scheme {
	case "github":
		parts := strings.SplitN(rest, "/", 3)
		if len(parts) < 2 {
			return SourceURL{}, fmt.Errorf("invalid github source %q: expected owner/repo", raw)
		}
		subPath := ""
		if len(parts) > 2 {
			subPath = parts[2]
		}
		return SourceURL{
			Raw: raw, Scheme: scheme,
			Owner: parts[0], Repo: parts[1], SubPath: subPath,
		}, nil

	case "npm":
		if strings.HasPrefix(rest, "@") {
			parts := strings.SplitN(rest, "/", 2)
			if len(parts) < 2 || parts[1] == "" {
				return SourceURL{}, fmt.Errorf("invalid npm source %q: expected @scope/package", raw)
			}
			return SourceURL{
				Raw: raw, Scheme: scheme,
				Scope: parts[0], Name: parts[1],
			}, nil
		}
		if rest == "" {
			return SourceURL{}, fmt.Errorf("invalid npm source %q: missing package name", raw)
		}
		return SourceURL{
			Raw: raw, Scheme: scheme,
			Name: rest,
		}, nil

	case "local":
		return SourceURL{Raw: raw, Scheme: scheme, Path: rest}, nil

	case "git":
		return SourceURL{Raw: raw, Scheme: scheme, URL: rest}, nil

	case "file":
		return SourceURL{Raw: raw, Scheme: scheme, Path: rest}, nil

	case "url":
		return SourceURL{Raw: raw, Scheme: scheme, URL: rest}, nil

	default:
		return SourceURL{}, fmt.Errorf("unknown source scheme %q in %q", scheme, raw)
	}
}

// IsValid returns true if the SourceURL has a known scheme and required fields.
func (s SourceURL) IsValid() bool {
	if s.Scheme == "" {
		return false
	}
	switch s.Scheme {
	case "github":
		return s.Owner != "" && s.Repo != ""
	case "npm":
		return s.Name != ""
	case "local":
		return s.Path != ""
	case "git":
		return s.URL != ""
	case "file":
		return s.Path != ""
	case "url":
		return s.URL != ""
	default:
		return false
	}
}

// String reconstructs the source URL string from its fields.
func (s SourceURL) String() string {
	switch s.Scheme {
	case "github":
		if s.SubPath != "" {
			return fmt.Sprintf("github:%s/%s/%s", s.Owner, s.Repo, s.SubPath)
		}
		return fmt.Sprintf("github:%s/%s", s.Owner, s.Repo)
	case "npm":
		if s.Scope != "" {
			return fmt.Sprintf("npm:%s/%s", s.Scope, s.Name)
		}
		return fmt.Sprintf("npm:%s", s.Name)
	case "local":
		return fmt.Sprintf("local:%s", s.Path)
	case "git":
		return fmt.Sprintf("git:%s", s.URL)
	case "file":
		return fmt.Sprintf("file:%s", s.Path)
	case "url":
		return fmt.Sprintf("url:%s", s.URL)
	default:
		return s.Raw
	}
}

// MarshalJSON serializes SourceURL as a JSON string.
func (s SourceURL) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON deserializes SourceURL from a JSON string.
func (s *SourceURL) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	parsed, err := ParseSourceURL(raw)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

// MarshalYAML serializes SourceURL as a YAML string.
func (s SourceURL) MarshalYAML() (interface{}, error) {
	return s.String(), nil
}

// UnmarshalYAML deserializes SourceURL from a YAML string.
func (s *SourceURL) UnmarshalYAML(value *yaml.Node) error {
	var raw string
	if err := value.Decode(&raw); err != nil {
		return err
	}
	parsed, err := ParseSourceURL(raw)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}
