package types

// UniversalMCPConfig is an MCP settings block compatible with
// the Model Context Protocol specification.
type UniversalMCPConfig struct {
	Servers map[string]UniversalMCPServer `json:"servers" yaml:"servers"`
}

// UniversalMCPServer describes a single MCP server configuration.
type UniversalMCPServer struct {
	Command     string            `json:"command,omitempty" yaml:"command,omitempty"`
	Args        []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	URL         string            `json:"url,omitempty" yaml:"url,omitempty"`
	Transport   string            `json:"transport" yaml:"transport"` // stdio, sse, http
	Headers     map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
}
