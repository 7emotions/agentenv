package lockfile

import (
	"fmt"
	"os"

	"github.com/7emotions/agentenv/pkg/parser"
	"github.com/7emotions/agentenv/pkg/types"
)

// Read parses a lockfile from bytes and validates required fields.
// An empty packages list is valid — it represents an environment with no packages.
func Read(data []byte) (*types.Lockfile, error) {
	lf, err := parser.ParseLockfile(data)
	if err != nil {
		return nil, err
	}
	return lf, nil
}

// ReadFromFile reads a lockfile from disk.
func ReadFromFile(path string) (*types.Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading lockfile: %w", err)
	}
	return Read(data)
}
