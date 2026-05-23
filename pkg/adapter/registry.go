package adapter

import (
	"fmt"
	"sync"
)

var (
	mu       sync.RWMutex
	adapters = make(map[string]AgentAdapter)
)

// Register registers an agent adapter by name. It panics if name is empty
// or if an adapter with the same name has already been registered.
func Register(name string, a AgentAdapter) {
	mu.Lock()
	defer mu.Unlock()

	if name == "" {
		panic("adapter: Register called with empty name")
	}
	if _, ok := adapters[name]; ok {
		panic("adapter: Register called twice for " + name)
	}
	adapters[name] = a
}

// Get returns the adapter registered under the given name, or an error
// if no adapter is found.
func Get(name string) (AgentAdapter, error) {
	mu.RLock()
	defer mu.RUnlock()

	a, ok := adapters[name]
	if !ok {
		return nil, fmt.Errorf("adapter %q not found", name)
	}
	return a, nil
}

// List returns all registered adapter names.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(adapters))
	for name := range adapters {
		names = append(names, name)
	}
	return names
}

func init() {
	Register("claude-code", NewClaudeCodeAdapter())
	Register("opencode", NewOpenCodeAdapter())
	Register("cursor", NewCursorAdapter())
	Register("codex", NewCodexAdapter())
}
