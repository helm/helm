# AGENTS.md

## Overview

Helm is a package manager for Kubernetes written in Go. It enables users to define, install, and upgrade complex Kubernetes applications using charts.
This document provides an overview of the codebase structure, development guidelines, and key patterns for contributors.

The codebase supports both an SDK for advanced users, and a CLI for direct end user usage.

The project currently supports Helm v3 and Helm v4 versions, based on the `dev-v3` and `main` branches respectively.

## Build and test

```bash
make build              # Build binary
make test               # Run all tests (style + unit)
make test-unit          # Unit tests only
make test-coverage      # With coverage
make test-style         # Linting (wraps golangci-lint)
go test -run TestName   # Specific test
```

## Code structure

Major packages:

- `cmd/helm/` - CLI entry point, wires CLI flags to `pkg/cmd/` commands
- `pkg/` - Public API
  - `action/` - Core operations (install, upgrade, rollback)
  - `cmd/` - Cobra command implementations bridging CLI flags to `pkg/action/`
  - `chart/v2/` - Stable chart format
  - `engine/` - Template rendering (Go templates + Sprig)
  - `kube/` - Kubernetes client abstraction layer
  - `registry/` - OCI support
  - `release/` - Release types and interfaces (`v1/`, `common/`)
  - `repo/` - Chart repository indexing and interaction
  - `storage/` - Release backends (Secrets/ConfigMaps/SQL)
- `internal/` - Private implementations
  - `chart/v3/` - Next-gen chart format
  - `release/v2/` - Release package for chart v3 support

## Development

### Compatibility

Changes are required to maintain backward compatibility as described in [HIP-0004: Document backwards-compatibility rules](https://github.com/helm/community/blob/main/hips/hip-0004.md).

Typically this means that:

- the signatures of public APIs, i.e., those in the `pkg/` directory should not change
- CLI commands and parameters should not be removed or changed in a way that would break existing scripts or workflows
- functional behaviour (as implied or documented) must not be modified in a way that would break existing users' expectations

An exception to the above is where incompatible changes are needed to fix a security vulnerability, where minimal breaking changes may be made to address the issue.

### Code standards

- Use table-driven tests with testify
- Golden files in `testdata/` for complex output
- Mock Kubernetes clients for action tests
- All commits must include DCO sign-off: `git commit -s`

### Branching

Standard workflow is for PR development changes to the `main` branch. Minor release branches are cut from `main`, then maintained for critical fixes via patch releases.
Bug and security fixes are also backported to `dev-v3` where applicable.

Development branches:

- `main` - Helm v4
- `dev-v3` - Helm v3 (backport security and bugfixes from main)

Release branches:

- `release-v3.X` - Release branches for v3.X versions
- `release-v4.X` - Release branches for v4.X versions

### Major dependencies

- `k8s.io/client-go` - Kubernetes interaction
- `github.com/spf13/cobra` - CLI framework
- `github.com/Masterminds/sprig` - Template functions

### Key patterns

- **Actions**: High-level operations live in `pkg/action/`, typically using a shared Configuration
- **Chart versions**: Charts v2 (stable) in `pkg/chart/v2`, v3 (under development) in `internal/chart/v3`
- **Plugins and extensibility**: Enabling additional functionality via plugins and extension points, such as custom template functions or storage backends is preferred over incorporating into Helm's codebase
