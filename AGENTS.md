# AGENTS.md

## Overview
Helm is a package manager for Kubernetes written in Go, supporting v3 (stable) and v4 (unstable) APIs.

## Build & Test
```bash
make build              # Build binary
make test               # Run all tests (style + unit)
make test-unit          # Unit tests only
make test-coverage      # With coverage
make test-style         # Linting
golangci-lint run       # Direct linting
go test -run TestName   # Specific test
```

## Code Structure
- `/cmd/helm/` - CLI entry point (Cobra-based)
- `/pkg/` - Public API
  - `action/` - Core operations (install, upgrade, rollback)
  - `chart/v2/` - Stable chart format
  - `engine/` - Template rendering (Go templates + Sprig)
  - `registry/` - OCI support
  - `storage/` - Release backends (Secrets/ConfigMaps/SQL)
- `/internal/` - Private implementation
  - `chart/v3/` - Next-gen chart format

## Development Guidelines

### Code Standards
- Use table-driven tests with testify
- Golden files in `testdata/` for complex output
- Mock Kubernetes clients for action tests
- All commits must include DCO sign-off: `git commit -s`

### Branching
- `main` - Helm v4 development
- `dev-v3` - Helm v3 stable (backport from main)

### Dependencies
- `k8s.io/client-go` - Kubernetes interaction
- `github.com/spf13/cobra` - CLI framework
- `github.com/Masterminds/sprig` - Template functions

### Key Patterns
- **Actions**: Operations in `/pkg/action/` use shared Configuration
- **Dual Chart Support**: v2 (stable) in `/pkg/`, v3 (dev) in `/internal/`
- **Storage Abstraction**: Pluggable release storage backends
