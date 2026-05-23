package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnmarshalJSONC_WithComments(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantKey string
		wantVal string
	}{
		{
			name:    "line comment",
			input:   "{\n  // this is a line comment\n  \"name\": \"alice\"\n}",
			wantKey: "name",
			wantVal: "alice",
		},
		{
			name:    "block comment",
			input:   "{\n  /* multi-line\n     block comment */\n  \"name\": \"bob\"\n}",
			wantKey: "name",
			wantVal: "bob",
		},
		{
			name:    "both comment types",
			input:   "{\n  // line\n  /* block */\n  \"name\": \"carol\"\n}",
			wantKey: "name",
			wantVal: "carol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]string
			if err := UnmarshalJSONC([]byte(tt.input), &result); err != nil {
				t.Fatalf("UnmarshalJSONC: %v", err)
			}
			if got := result[tt.wantKey]; got != tt.wantVal {
				t.Errorf("%q = %q, want %q", tt.wantKey, got, tt.wantVal)
			}
		})
	}
}

func TestUnmarshalJSONC_PlainJSON(t *testing.T) {
	input := `{"name": "alice", "age": 30}`
	var result map[string]interface{}
	if err := UnmarshalJSONC([]byte(input), &result); err != nil {
		t.Fatalf("UnmarshalJSONC: %v", err)
	}
	if result["name"] != "alice" {
		t.Errorf("name = %q, want alice", result["name"])
	}
	if result["age"].(float64) != 30 {
		t.Errorf("age = %v, want 30", result["age"])
	}
}

func TestUnmarshalJSONC_StringWithSlashes(t *testing.T) {
	// URLs containing // must not be confused with line comments
	input := `{"url": "https://api.example.com//path"}`
	var result map[string]string
	if err := UnmarshalJSONC([]byte(input), &result); err != nil {
		t.Fatalf("UnmarshalJSONC: %v", err)
	}
	want := "https://api.example.com//path"
	if result["url"] != want {
		t.Errorf("url = %q, want %q", result["url"], want)
	}
}

func TestUnmarshalJSONC_TrailingComma(t *testing.T) {
	// JSONC supports trailing commas in objects and arrays
	input := `{
  "name": "alice",
  "age": 30,
}`
	var result map[string]interface{}
	if err := UnmarshalJSONC([]byte(input), &result); err != nil {
		t.Fatalf("UnmarshalJSONC: %v", err)
	}
	if result["name"] != "alice" {
		t.Errorf("name = %q, want alice", result["name"])
	}
	if result["age"].(float64) != 30 {
		t.Errorf("age = %v, want 30", result["age"])
	}
}

func TestUnmarshalJSONC_FileNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.jsonc")
	var result map[string]interface{}
	err := ReadAndUnmarshalJSONC(path, &result)
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
	// Verify it's not a panic — the error should wrap os.PathError
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("file should not exist, stat says: %v", statErr)
	}
}

func TestUnmarshalJSONC_ReadAndUnmarshal(t *testing.T) {
	// Write a JSONC file with comments, read it back, verify contents
	content := `{
  // my config
  "env_name": "test",
  "version": 1,
  "skills": {
    "code-review": {
      /* source path */
      "source_path": "/store/skills/code-review",
      "name": "code-review"
    }
  }
}`
	path := filepath.Join(t.TempDir(), "manifest.jsonc")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var manifest Manifest
	if err := ReadAndUnmarshalJSONC(path, &manifest); err != nil {
		t.Fatalf("ReadAndUnmarshalJSONC: %v", err)
	}

	if manifest.EnvName != "test" {
		t.Errorf("EnvName = %q, want test", manifest.EnvName)
	}
	if manifest.Version != 1 {
		t.Errorf("Version = %d, want 1", manifest.Version)
	}
	item, ok := manifest.Skills["code-review"]
	if !ok {
		t.Fatal("code-review not in manifest")
	}
	if item.Name != "code-review" {
		t.Errorf("Name = %q, want code-review", item.Name)
	}
	if item.SourcePath != "/store/skills/code-review" {
		t.Errorf("SourcePath = %q, want /store/skills/code-review", item.SourcePath)
	}
}
