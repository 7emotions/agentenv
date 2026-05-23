# agent.lock Format Reference

agent.lock is the lockfile for agentenv. It records the exact resolved state of
an environment: every package, its version, source, integrity hash, and
dependencies. The lockfile ensures reproducible environments across machines
and over time.

## Schema Version

The lockfile has two schema versions:

| Version | Description |
|---|---|
| 1 | Original format. Dependencies do not carry a `source` field. |
| 2 | Current format. Dependencies include an optional `source` field. |

agentenv always writes version 1 (v1) lockfiles. The `source` field on
dependencies was added to the schema in v2 and is available for use in future
releases.

### v1 to v2 Migration

When agentenv reads a v1 lockfile (dependencies without a `source` field), it
parses without error. The missing `source` field defaults to an empty string.
No data is lost during migration. The file on disk is not modified.

## Top-Level Fields

```yaml
version: 1
generated: "2026-05-24T12:00:00Z"
packages:
  - name: code-reviewer
    type: skill
    source: github:myorg/code-review-skill
    version: 0.5.1
    resolved: https://github.com/myorg/code-review-skill/releases/download/v0.5.1/pkg.tar.gz
    dependencies:
      - name: linter-core
        version: 3.0.1
        resolved: https://github.com/myorg/linter-core/releases/download/v3.0.1/pkg.tar.gz
environment_snapshot:
  skills_count: 1
  mcps_count: 0
  agents_count: 0
  tools_count: 0
  hooks_count: 0
  prompts_count: 0
```

| Field | Type | Required | Description |
|---|---|---|---|
| `version` | int | yes | Schema version. Currently 1. |
| `generated` | string | yes | ISO 8601 / RFC 3339 timestamp of when the lockfile was generated. |
| `packages` | array | yes | List of resolved packages. Can be empty for an environment with no packages. |
| `environment_snapshot` | object | no | Count of packages by type at lock time. |

## Packages

Each entry in the `packages` array describes a single resolved package.

```yaml
packages:
  - name: code-reviewer
    type: skill
    source: github:myorg/code-review-skill
    version: 0.5.1
    resolved: https://github.com/myorg/code-review-skill/releases/download/v0.5.1/pkg.tar.gz
    sha256: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
    dependencies:
      - name: formatter
        version: 1.2.0
        resolved: https://github.com/myorg/formatter/releases/download/v1.2.0/pkg.tar.gz
```

### LockedPackage Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Package name. |
| `type` | string | yes | Package type. One of: `skill`, `mcp`, `agent`, `tool`, `hook`, `prompt`. |
| `source` | string | yes | Source URL where the package was resolved from. Uses the same scheme format as agent.yaml. |
| `version` | string | yes | Resolved version string (not a constraint). |
| `resolved` | string | yes | Direct download URL for the package archive. |
| `sha256` | string | no | SHA-256 hex digest of the resolved package archive for integrity verification. |
| `dependencies` | array | no | List of resolved transitive dependencies. |
| `installed_at` | string | no | Timestamp of when the package was installed. |

## Dependencies

Each dependency in a package's `dependencies` array records a resolved
transitive dependency.

### LockedDep Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Dependency package name. |
| `version` | string | yes | Resolved version string. |
| `resolved` | string | yes | Direct download URL for the dependency archive. |
| `source` | string | no | Source URL for the dependency. New in v2 schema. Can be empty for backward compatibility with v1 lockfiles. |

## Environment Snapshot

The `environment_snapshot` field records package counts by type at lock time.
This is informational and not used for resolution.

```yaml
environment_snapshot:
  skills_count: 2
  mcps_count: 1
  agents_count: 0
  tools_count: 0
  hooks_count: 0
  prompts_count: 0
```

| Field | Type | Description |
|---|---|---|
| `skills_count` | int | Number of skill packages. |
| `mcps_count` | int | Number of MCP server packages. |
| `agents_count` | int | Number of agent packages. |
| `tools_count` | int | Number of tool packages. |
| `hooks_count` | int | Number of lifecycle hook packages. |
| `prompts_count` | int | Number of prompt template packages. |

## Complete Example

```yaml
version: 1
generated: "2026-05-24T14:30:00Z"
packages:
  - name: code-reviewer
    type: skill
    source: github:myorg/code-review-skill
    version: 0.5.1
    resolved: https://github.com/myorg/code-review-skill/releases/download/v0.5.1/pkg.tar.gz
    sha256: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
    dependencies:
      - name: formatter
        version: 1.2.0
        resolved: https://github.com/myorg/formatter/releases/download/v1.2.0/pkg.tar.gz
        source: ""
      - name: linter-core
        version: 3.0.1
        resolved: https://github.com/myorg/linter-core/releases/download/v3.0.1/pkg.tar.gz
        source: ""

  - name: filesystem-mcp
    type: mcp
    source: npm:@modelcontextprotocol/server-filesystem
    version: 0.6.0
    resolved: https://registry.npmjs.org/@modelcontextprotocol/server-filesystem/-/server-filesystem-0.6.0.tgz
    sha256: 01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b

  - name: senior-dev
    type: agent
    source: github:myorg/agents
    version: 1.2.3
    resolved: https://github.com/myorg/agents/releases/download/v1.2.3/senior-dev.tar.gz

environment_snapshot:
  skills_count: 1
  mcps_count: 1
  agents_count: 1
  tools_count: 0
  hooks_count: 0
  prompts_count: 0
```

## Notes

- The lockfile is deterministic. Packages are sorted by (type, name).
  Dependencies within each package are sorted by name.
- An empty `packages` array is valid. This represents an environment with no
  packages installed.
- The `sha256` field is optional but should be verified when present. agentenv
  checks the hash after downloading a package.
- The `source` field on dependencies is always present in v2+ lockfiles. It is
  empty when the dependency had no explicit source in agent.yaml and no
  implicit source was resolved.
