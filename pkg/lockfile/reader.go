package lockfile

import (
	"fmt"
	"os"

	"github.com/7emotions/agentenv/pkg/parser"
	"github.com/7emotions/agentenv/pkg/types"
)

// Read parses a lockfile from bytes and validates required fields.
func Read(data []byte) (*types.Lockfile, error) {
	lf, err := parser.ParseLockfile(data)
	if err != nil {
		return nil, err
	}
	if len(lf.Packages) == 0 {
		return nil, fmt.Errorf("lockfile contains no packages")
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
