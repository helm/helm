# PLAN.md — worktree/1: HIP-0025 Subchart Sequencing (Full Stack)

## Context

HIP-0025 introduces native resource and subchart sequencing to Helm v4.
This worktree implements the complete subchart sequencing pipeline: chart metadata,
DAG engine, CLI flag, action layer integration, and release storage.

**Spec source:** `hip-0025/hip-0025.md`, `hip-0025/subchart-sequencing.md`

## Delivered

### Phase 2: Chart Metadata Extension — DONE
- `DependsOn []string` field on v2 + v3 `Dependency` struct
- Sanitization in `Validate()`
- Files: `pkg/chart/v2/dependency.go`, `internal/chart/v3/dependency.go`

### Phase 3: DAG Construction & Topo Sort — DONE
- DAG struct: AddNode, AddEdge, DetectCycles, TopologicalSort, Batches
- `BuildSubchartDAG` combines DependsOn field + annotation parsing
- `ParseDependsOnSubcharts` for `helm.sh/depends-on/subcharts` annotation
- Annotation constants for resource-group sequencing (future use)
- Files: `pkg/chart/v2/util/dag.go`, `pkg/chart/v2/util/sequencing.go` (+ v3 mirrors)

### Phase 4: WaitStrategy Extension — DONE
- `OrderedStrategy WaitStrategy = "ordered"` constant
- Delegates to StatusWatcher for readiness checks
- CLI accepts `--wait=ordered`
- Files: `pkg/kube/client.go`, `pkg/cmd/flags.go`

### Phase 5: Action System Integration — DONE
- `performOrderedInstall`: deploy resources in DAG batch order
- `SplitManifestsBySubchart`: partition manifest by `# Source:` comments
- `BuildInstallBatches`: compute ordered batches from chart metadata
- `createAndWaitResources`: extracted helper for create+wait
- Falls back to all-at-once when no sequencing metadata exists
- Files: `pkg/action/install.go`, `pkg/action/sequencing.go`

### Phase 6: CLI Integration — DONE (bundled with Phase 4)
- `--wait=ordered` accepted alongside watcher/hookOnly/legacy
- Error messages updated to include ordered option

### Phase 7: Release Storage — DONE
- `SequencingMetadata` struct: enabled, strategy, batches
- Persisted in `Release.Sequencing` during ordered install
- Enables rollback/uninstall to reconstruct deployment order
- Files: `pkg/release/v1/release.go`

## Test Coverage

- 10 DAG unit tests (empty, single, linear, diamond, parallel, cycles, self-dep, orphans, complex)
- 7 annotation parsing tests (nil, empty, valid, invalid JSON, etc.)
- 6 BuildSubchartDAG tests (HIP-0025 example, aliases, cycles, unknown refs)
- 7 action sequencing tests (manifest splitting, batch construction, circular deps)
- All existing tests pass — zero regressions

## Commits

```
01fdc96b feat(chart): add DependsOn field and DAG engine for HIP-0025
c8a23f04 feat(kube): add OrderedStrategy WaitStrategy for HIP-0025
1e79330b feat(action): implement ordered subchart install for HIP-0025
7550e729 feat(release): store sequencing metadata in Release
```

## Remaining (future worktrees)

- Resource-group sequencing (within single chart)
- Custom readiness evaluation (helm.sh/readiness-success/failure + JSONPath)
- Upgrade ordered sequencing
- Uninstall/rollback reverse-order logic
- `helm template` ordered output with START/END resource-group comments
- Readiness timeout flag (--readiness-timeout)

## bd Issues

| ID | Phase | Status |
|----|-------|--------|
| helm-btq | Phase 2: Chart Metadata | done |
| helm-ed3 | Phase 3: DAG Construction | done |
| helm-40n | Phase 4: WaitStrategy | done |
| helm-fby | Phase 5: Action System | done |
| helm-0d5 | Phase 6: CLI | done |
| helm-7an | Phase 7: Release Storage | done |
