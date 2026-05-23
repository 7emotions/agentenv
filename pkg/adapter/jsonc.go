package adapter

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tailscale/hujson"
)

// UnmarshalJSONC parses JSONC (JSON with Comments) data and stores the result
// in the value pointed to by v. It supports // line comments, /* */ block
// comments, and trailing commas via the hujson library.
//
// This function does NOT preserve comments on round-trips — it standardizes
// the input to valid JSON before unmarshaling. A future version may add
// comment-preserving read/write support.
func UnmarshalJSONC(data []byte, v interface{}) error {
	ast, err := hujson.Parse(data)
	if err != nil {
		return fmt.Errorf("parse jsonc: %w", err)
	}
	ast.Standardize()
	if err := json.Unmarshal(ast.Pack(), v); err != nil {
		return fmt.Errorf("unmarshal jsonc: %w", err)
	}
	return nil
}

// ReadAndUnmarshalJSONC reads a JSONC file from disk at the given path and
// unmarshals it into v. It is a convenience wrapper around os.ReadFile and
// UnmarshalJSONC.
func ReadAndUnmarshalJSONC(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read jsonc file %s: %w", path, err)
	}
	return UnmarshalJSONC(data, v)
}
