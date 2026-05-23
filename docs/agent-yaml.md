# agent.yaml Format Reference

agent.yaml is the environment definition file for agentenv. It describes a named
set of packages (skills, MCP servers, agents, tools, hooks, and prompts) that
form a complete agent runtime environment.

## Top-Level Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Environment name. Alphanumeric characters and hyphens only. |
| `description` | string | no | Human-readable description of the environment. |
| `skills` | object | no | Map of skill packages to install. |
| `mcps` | object | no | Map of MCP server packages to install. |
| `agents` | object | no | Map of agent packages to install. |
| `tools` | object | no | Map of tool packages to install. |
| `hooks` | object | no | Map of lifecycle hook packages to install. |
| `prompts` | object | no | Map of prompt template packages to install. |

All six package sections (`skills`, `mcps`, `agents`, `tools`, `hooks`,
`prompts`) share the same structure. Each is a map where the key is the package
name and the value is a package spec.

## Package Sections

Each package section is a map of package name to a package spec:

```yaml
skills:
  my-skill:
    source: github:owner/repo
    version: ">=1.0"
    config:
      key: value
```

### Package Spec Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `source` | string | yes | Package source URL. Determines where to fetch the package. |
| `version` | string | no | Version constraint. Supports semver constraints. Defaults to `*` (any version). |
| `config` | object | no | Arbitrary configuration passed to the package at install time. |

## Source URL Formats

The `source` field uses a scheme-prefixed format to describe where a package
lives.

| Scheme | Format | Example |
|---|---|---|
| `github` | `github:owner/repo` or `github:owner/repo/path` | `github:myorg/skill-repo` |
| `npm` | `npm:package` or `npm:@scope/package` | `npm:@anthropic/claude-code` |
| `local` | `local:./relative/path` | `local:./vendor/my-skill` |
| `git` | `git:https://...` | `git:https://github.com/org/repo.git` |
| `file` | `file:/absolute/path` | `file:/home/user/packages/skill.tar.gz` |
| `url` | `url:https://...` | `url:https://example.com/pkg.tar.gz` |

## Complete Example

```yaml
name: my-ai-env
description: "My personal AI development environment"

skills:
  code-reviewer:
    source: github:myorg/code-review-skill
    version: ">=0.5"

  git-helper:
    source: local:./vendor/git-helper

mcps:
  filesystem:
    source: npm:@modelcontextprotocol/server-filesystem
    version: "0.6.0"
    config:
      root: /home/user/projects

agents:
  senior-dev:
    source: github:myorg/agents
    version: "~1.2"

tools:
  formatter:
    source: url:https://example.com/tools/formatter.tar.gz

hooks:
  pre-activate:
    source: file:/home/user/hooks/check-env.sh
  post-deactivate:
    source: local:./hooks/cleanup.sh

prompts:
  code-review:
    source: git:https://github.com/myorg/prompts.git
    version: ">=2.0"
```

## Multiple Package Types

Each environment can combine any number of package types. You are not limited
to a single type per environment. Mix skills with MCPs, tools, agents, hooks,
and prompts freely.

## Notes

- Package names must be unique within their section. They can repeat across
  sections (a skill and an MCP can share a name).
- The `version` field defaults to `*` when omitted, matching any available
  version of the package.
- The `config` field is package-specific. agentenv does not validate its
  contents.
- Source URLs must use one of the six supported schemes. An unknown scheme
  produces a validation error.
