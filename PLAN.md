# PLAN.md — worktree/1: HIP-0025 Foundation (Chart Metadata + DAG)

## Context

HIP-0025 introduces native resource and subchart sequencing to Helm v4.
This worktree implements the **foundation layer**: chart metadata extensions
and DAG construction needed before action/CLI integration.

**Spec source:** `hip-0025/hip-0025.md`, `hip-0025/subchart-sequencing.md`

## Scope (this branch)

### 1. Chart Metadata Extension — `DependsOn` field
- Add `DependsOn []string` to `Dependency` struct in both v2 and v3
- Supports `depends-on` YAML key in Chart.yaml dependencies list
- Add validation: each DependsOn entry must reference a known dependency name/alias
- Sanitize DependsOn strings in `Validate()`

**Files:**
- `pkg/chart/v2/dependency.go` — add field + validation
- `internal/chart/v3/dependency.go` — add field + validation

### 2. Annotation Parsing — `helm.sh/depends-on/subcharts`
- Parse `helm.sh/depends-on/subcharts` from `Metadata.Annotations`
- Already available: `Annotations map[string]string` exists on both v2/v3 Metadata
- Utility function to extract and parse the annotation value (JSON array of strings)

**Files:**
- New: `pkg/chart/v2/util/sequencing.go` — annotation constants, parse helpers
- New: `internal/chart/v3/util/sequencing.go` — v3 variant

### 3. DAG Construction & Topological Sort
- Build subchart dependency graph from:
  - `Dependency.DependsOn` field entries
  - `Metadata.Annotations["helm.sh/depends-on/subcharts"]` entries
- Topological sort → produce ordered batches (layers)
- Circular dependency detection with clear error messages
- Orphaned subchart handling (no deps → deployed with parent in final batch)

**Files:**
- New: `pkg/chart/v2/util/dag.go` — DAG struct, AddNode, AddEdge, TopologicalSort, DetectCycles
- New: `internal/chart/v3/util/dag.go` — v3 variant (or shared via common)

### 4. Integration with ProcessDependencies
- After existing enable/disable logic, build subchart DAG
- Validate DAG (no cycles, all DependsOn refs resolve)
- Attach DAG to chart processing result for downstream use

**Files:**
- `pkg/chart/v2/util/dependencies.go` — add DAG building after ProcessDependencies
- `internal/chart/v3/util/dependencies.go` — same

### 5. Tests
- Unit tests for DependsOn parsing and validation
- Unit tests for DAG construction (linear, diamond, parallel, orphan patterns)
- Unit tests for circular dependency detection
- Unit tests for annotation parsing
- Integration test with chart fixtures

**Files:**
- `pkg/chart/v2/util/dag_test.go`
- `pkg/chart/v2/util/sequencing_test.go`
- `pkg/chart/v2/dependency_test.go` (extend existing)

## Verification

```bash
make test-unit                    # Full unit test suite
go test ./pkg/chart/v2/...       # Chart v2 tests
go test ./pkg/chart/v2/util/...  # DAG + sequencing tests
go test ./internal/chart/v3/...  # Chart v3 tests
make test-style                   # Linting
```

## bd Issues

| ID | Phase | Status |
|----|-------|--------|
| helm-btq | Phase 2: Chart Metadata | in-progress |
| helm-ed3 | Phase 3: DAG Construction | blocked-by helm-btq |
| helm-40n | Phase 4: WaitStrategy | future worktree |
| helm-fby | Phase 5: Action System | future worktree |
| helm-0d5 | Phase 6: CLI | future worktree |
| helm-7an | Phase 7: Release Storage | future worktree |
