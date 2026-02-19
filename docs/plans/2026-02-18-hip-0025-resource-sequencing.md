# HIP-0025 Resource Sequencing Implementation Plan

Created: 2026-02-18
Status: VERIFIED
Approved: Yes
Iterations: 0
Worktree: Yes

> **Status Lifecycle:** PENDING → COMPLETE → VERIFIED
> **Iterations:** Tracks implement→verify cycles (incremented by verify phase)
>
> - PENDING: Initial state, awaiting implementation
> - COMPLETE: All tasks implemented
> - VERIFIED: All checks passed
>
> **Approval Gate:** Implementation CANNOT proceed until `Approved: Yes`
> **Worktree:** Set at plan creation (from dispatcher). `Yes` uses git worktree isolation; `No` works directly on current branch (default)

## Summary

**Goal:** Implement HIP-0025 — native resource and subchart sequencing for Helm v4, enabling chart authors to define deployment order via annotations (`helm.sh/resource-group`, `helm.sh/depends-on/resource-groups`) on template resources and via `depends-on` fields / `helm.sh/depends-on/subcharts` annotations in Chart.yaml. Includes custom readiness evaluation via `helm.sh/readiness-success` and `helm.sh/readiness-failure` annotations with JSONPath.

**Architecture:** A new DAG (Directed Acyclic Graph) engine in `pkg/chart/v2/util/` handles dependency resolution for both resource-groups and subcharts. The `--wait=ordered` WaitStrategy value activates sequenced deployment where resources are applied in topological batches. A readiness evaluation system in `pkg/kube/` extends the existing kstatus integration with custom JSONPath-based readiness checks. Sequencing metadata is stored in the Release object for rollback/uninstall support.

**Tech Stack:** Go standard library for DAG/toposort, existing kstatus library for default readiness, `k8s.io/client-go/util/jsonpath` for custom readiness JSONPath evaluation.

## Scope

### In Scope

- `helm.sh/resource-group` annotation on template resources
- `helm.sh/depends-on/resource-groups` annotation on template resources
- `depends-on` field on Chart.yaml `dependencies` entries
- `helm.sh/depends-on/subcharts` annotation on Chart.yaml
- `--wait=ordered` CLI flag and `OrderedWaitStrategy` SDK constant
- `--readiness-timeout` CLI flag and `ReadinessTimeout` SDK field
- Custom readiness evaluation via `helm.sh/readiness-success` and `helm.sh/readiness-failure`
- DAG construction, validation, circular dependency detection
- Sequencing metadata stored in Release object for rollback/uninstall
- Reverse-order uninstall when release was installed with sequencing
- `helm template` output with resource-group delimiters
- Lint rules for sequencing annotations (circular deps, orphaned groups, single readiness annotation)
- Warning system for misconfigured annotations

### Out of Scope

- Sequencing of hook resources (hooks continue to use `helm.sh/hook-weight`)
- Resource-group sequencing across charts (annotations are chart-scoped)
- Chart v2 changes (sequencing is Chart v3 only, Helm v4)
- `helm test` integration with sequencing
- DAG visualization command (deferred for future PR)

## Prerequisites

- Helm v4 codebase (current main branch)
- kstatus library already integrated (`github.com/fluxcd/cli-utils v0.37.1-flux.1`)
- `k8s.io/client-go/util/jsonpath` available in dependency tree

## Context for Implementer

> This section is critical for cross-session continuity.

- **Patterns to follow:**
  - Wait strategy: Follow the existing `WaitStrategy` enum pattern in `pkg/kube/client.go:103-120` — add `OrderedWaitStrategy` as a new const
  - CLI flags: Follow `AddWaitFlag()` in `pkg/cmd/flags.go:58-65` — extend `waitValue.Set()` to accept `"ordered"`
  - Action structs: `Install`, `Upgrade`, `Rollback`, `Uninstall` all carry `WaitStrategy` — follow existing patterns
  - Lint rules: Follow pattern in `pkg/chart/v2/lint/rules/dependencies.go` — add new rule functions
  - Dependency struct: `pkg/chart/v2/dependency.go:24` — add `DependsOn []string` field
  - Metadata annotations: `pkg/chart/v2/metadata.go:77` — `Annotations map[string]string` already exists
  - Hook execution pattern: `pkg/action/hooks.go` — execHook is called per-lifecycle, similar batching can be applied

- **Key files:**
  - `pkg/action/install.go` — Main install flow; `performInstall()` at line 486 is where resources are created and waited on
  - `pkg/action/upgrade.go` — Upgrade flow, mirrors install
  - `pkg/action/rollback.go` — Rollback flow, must respect sequencing flag from stored release
  - `pkg/action/uninstall.go` — Uninstall flow, reverse order when sequenced
  - `pkg/action/action.go:222` — `renderResources()` renders templates and sorts manifests
  - `pkg/kube/client.go` — KubeClient with WaitStrategy, Create, Update methods
  - `pkg/kube/interface.go` — Interface/Waiter definitions
  - `pkg/release/v1/release.go` — Release struct, needs SequencingInfo field
  - `pkg/release/v1/util/manifest_sorter.go` — SortManifests, Manifest struct
  - `pkg/chart/v2/dependency.go` — Dependency struct
  - `pkg/chart/v2/metadata.go` — Metadata struct with Annotations
  - `pkg/chart/v2/util/dependencies.go` — ProcessDependencies
  - `pkg/cmd/flags.go` — CLI flag definitions

- **Conventions:**
  - Helm annotations use `helm.sh/` prefix
  - Error wrapping uses `fmt.Errorf("context: %w", err)` pattern
  - Logging uses `slog` (structured logging)
  - Tests use table-driven style with `t.Run()`

- **Gotchas:**
  - `renderResources()` processes ALL subcharts together into a single manifest buffer — sequencing requires splitting this into per-subchart manifests
  - Post-renderer runs AFTER rendering but BEFORE sorting — must preserve Source comments for subchart identification
  - The `Manifest` field on Release is a single string — sequencing order must be reconstructable from it
  - Hooks are separated from manifests during `SortManifests()` — sequencing should not affect hooks

- **Domain context:**
  - "Resource-group" = a named group of K8s resources within a single chart, defined by `helm.sh/resource-group` annotation
  - "Readiness" = a resource's status indicates it is functioning (default: kstatus, override: custom JSONPath)
  - "Batch" = a set of resource-groups or subcharts at the same topological level that can be deployed simultaneously

## Progress Tracking

**MANDATORY: Update this checklist as tasks complete. Change `[ ]` to `[x]`.**

- [x] Task 1: DAG engine with topological sort and cycle detection
- [x] Task 2: Extend Dependency struct and Chart.yaml parsing
- [x] Task 3: Resource-group annotation parsing and DAG construction
- [x] Task 4: Custom readiness evaluation with JSONPath
- [x] Task 5: OrderedWaitStrategy and CLI integration
- [x] Task 6: Sequenced install action
- [x] Task 7: Sequenced upgrade action
- [x] Task 8: Release metadata and sequenced rollback/uninstall
- [x] Task 9: helm template resource-group delimiters
- [x] Task 10: Lint rules for sequencing
- [x] Task 11: Warning system for misconfigured annotations

**Total Tasks:** 11 | **Completed:** 11 | **Remaining:** 0

## Implementation Tasks

### Task 1: DAG Engine with Topological Sort and Cycle Detection

**Objective:** Build a generic DAG data structure with topological sorting (Kahn's algorithm) and cycle detection that can be used for both resource-group and subchart sequencing.

**Dependencies:** None

**Files:**

- Create: `pkg/chart/v2/util/dag.go`
- Test: `pkg/chart/v2/util/dag_test.go`

**Key Decisions / Notes:**

- Use Kahn's algorithm for topological sort — it naturally detects cycles (nodes remaining after sort = cycle participants)
- DAG is generic (string-keyed nodes) so it can serve both resource-groups and subcharts
- `GetBatches()` returns `[][]string` — each inner slice is a set of nodes at the same topological level (can be deployed in parallel)
- Error messages for cycles should list the cycle path for debugging

**Definition of Done:**

- [ ] DAG struct with AddNode, AddEdge, GetBatches methods
- [ ] Cycle detection returns error with cycle path description
- [ ] GetBatches returns correct topological layers for linear, diamond, and complex graphs
- [ ] Empty graph and single-node graph handled correctly
- [ ] All tests pass

**Verify:**

- `cd /home/rohit/Documents/helm/.worktrees/spec-hip-0025-2daac031 && go test ./pkg/chart/v2/util/ -run TestDAG -v`

---

### Task 2: Extend Dependency Struct and Chart.yaml Parsing

**Objective:** Add `DependsOn` field to the `Dependency` struct and parse `helm.sh/depends-on/subcharts` annotation from Chart.yaml metadata. Build subchart DAG from these declarations.

**Dependencies:** Task 1

**Files:**

- Modify: `pkg/chart/v2/dependency.go` — add `DependsOn []string` field
- Create: `pkg/chart/v2/util/subchart_dag.go` — subchart DAG construction from chart metadata
- Test: `pkg/chart/v2/util/subchart_dag_test.go`

**Key Decisions / Notes:**

- `DependsOn` field uses YAML tag `"depends-on,omitempty"` to match HIP spec Chart.yaml format; JSON tag `"dependsOn,omitempty"` following Go/JSON conventions (existing Dependency fields use kebab-case YAML but this is the first list field — verify consistency with existing tags)
- Subchart DAG construction reads both the `depends-on` field on dependencies AND `helm.sh/depends-on/subcharts` annotation on Chart metadata
- Subcharts identified by `name` or `alias` (alias takes precedence)
- Validation: referenced subcharts must exist in the dependencies list
- **Disabled subchart handling:** When a referenced subchart is disabled via condition/tags, the dependency edge is silently removed from the DAG (the disabled chart produces no resources, so dependents can proceed immediately). Emit an info-level log. This is different from referencing a truly non-existent subchart name, which is an error

**Definition of Done:**

- [ ] `Dependency.DependsOn` field added with correct JSON/YAML tags
- [ ] `BuildSubchartDAG(chart)` constructs DAG from both annotation and field-based declarations
- [ ] DAG correctly resolves alias vs name references
- [ ] Error returned when referencing non-existent subchart name
- [ ] Disabled subchart references silently removed from DAG with info log
- [ ] Cycle detection for subchart dependencies
- [ ] All tests pass

**Verify:**

- `cd /home/rohit/Documents/helm/.worktrees/spec-hip-0025-2daac031 && go test ./pkg/chart/v2/... -v`

---

### Task 3: Resource-Group Annotation Parsing and DAG Construction

**Objective:** Parse `helm.sh/resource-group` and `helm.sh/depends-on/resource-groups` annotations from rendered manifests and build a per-chart resource-group DAG.

**Dependencies:** Task 1

**Files:**

- Create: `pkg/release/v1/util/resource_group.go` — annotation parsing and resource-group DAG
- Test: `pkg/release/v1/util/resource_group_test.go`

**Key Decisions / Notes:**

- Parse annotations from rendered YAML manifests (after templating, before sending to K8s)
- `helm.sh/resource-group` is a single string annotation (one group name per resource); validation ensures each resource has at most one
- `helm.sh/depends-on/resource-groups` is a JSON array string (e.g., `["database", "queue"]`)
- Resources without annotations go into an "unsequenced" batch deployed last
- Resources referencing non-existent groups get a warning and are moved to unsequenced batch
- A resource-group with no `depends-on` edges is a root node and goes in batch 0 (earliest). Only resources that reference non-existent groups or have parse errors go to the unsequenced batch. Groups with valid annotations but no connection to other groups are still sequenced (just at batch 0)
- Manifests are grouped by Source comment to scope resource-groups within a chart

**Definition of Done:**

- [ ] `ParseResourceGroups(manifests)` extracts group assignments and dependencies
- [ ] `BuildResourceGroupDAG(groups)` creates DAG from parsed annotations
- [ ] Resources with no annotations assigned to unsequenced batch
- [ ] Root groups (no deps) correctly placed in batch 0 (earliest), not unsequenced
- [ ] Warning emitted for references to non-existent groups
- [ ] Resources scoped to their chart (via Source comment path)
- [ ] All tests pass

**Verify:**

- `cd /home/rohit/Documents/helm/.worktrees/spec-hip-0025-2daac031 && go test ./pkg/release/v1/util/ -run TestResourceGroup -v`

---

### Task 4: Custom Readiness Evaluation with JSONPath

**Objective:** Implement readiness evaluation using `helm.sh/readiness-success` and `helm.sh/readiness-failure` annotations with JSONPath expressions against `.status`.

**Dependencies:** None

**Files:**

- Create: `pkg/kube/readiness.go` — custom readiness evaluator
- Test: `pkg/kube/readiness_test.go`

**Key Decisions / Notes:**

- Uses `k8s.io/client-go/util/jsonpath` for JSONPath evaluation
- Expression format: `{<jsonpath_query>} <operator> <value>` where operator is `==`, `!=`, `<`, `<=`, `>`, `>=`
- JSONPath is scoped to `.status` — queries like `{.succeeded}` map to `.status.succeeded`
- OR semantics: if ANY success condition is true, resource is ready; if ANY failure condition is true, resource is failed
- Failure conditions take precedence over success conditions (checked first)
- Both annotations must be present to override default kstatus — if only one is present, fall back to kstatus with warning at runtime; at lint time (Task 10) this is an error
- Value comparison: string, number (float64), boolean
- Readiness is only evaluated for resources in groups that have downstream dependents in the DAG — the final batch (no dependents) does not need readiness polling
- **Expression parsing:** Split on the first space-surrounded operator token. For multi-value JSONPath results (arrays), ALL values must satisfy the condition. Empty JSONPath result = "not ready yet" (not error)
- Timeout defaults to 1 minute, configurable via `--readiness-timeout`
- Timeout must not exceed `--timeout`

**Definition of Done:**

- [ ] `EvaluateReadiness(resource, successExprs, failureExprs)` correctly evaluates JSONPath conditions
- [ ] Failure conditions take precedence over success
- [ ] Supports all comparison operators
- [ ] Falls back to kstatus when only one annotation is present (with warning)
- [ ] Handles missing `.status` fields gracefully
- [ ] All tests pass

**Verify:**

- `cd /home/rohit/Documents/helm/.worktrees/spec-hip-0025-2daac031 && go test ./pkg/kube/ -run TestReadiness -v`

---

### Task 5: OrderedWaitStrategy and CLI Integration

**Objective:** Add `OrderedWaitStrategy` to the WaitStrategy enum, add `--readiness-timeout` flag, and wire up CLI flags for install, upgrade, rollback, and uninstall commands.

**Dependencies:** None

**Files:**

- Modify: `pkg/kube/client.go` — add `OrderedWaitStrategy` constant
- Modify: `pkg/cmd/flags.go` — accept `"ordered"` in `waitValue.Set()`, add `--readiness-timeout` flag helper
- Modify: `pkg/cmd/install.go` — add `--readiness-timeout` flag
- Modify: `pkg/cmd/upgrade.go` — add `--readiness-timeout` flag
- Modify: `pkg/action/install.go` — add `ReadinessTimeout` field to Install struct
- Modify: `pkg/action/upgrade.go` — add `ReadinessTimeout` field to Upgrade struct
- Test: `pkg/cmd/flags_test.go` — test `--wait=ordered` parsing

**Key Decisions / Notes:**

- `OrderedWaitStrategy WaitStrategy = "ordered"` follows existing naming convention
- `--readiness-timeout` defaults to 1 minute; validation: must not exceed `--timeout`
- `GetWaiterWithOptions` for `OrderedWaitStrategy` should return the status watcher (sequencing logic lives in the action layer, not the kube client)
- The `--wait=ordered` flag description: "enable ordered resource and subchart sequencing"

**Definition of Done:**

- [ ] `OrderedWaitStrategy` constant defined in `pkg/kube/client.go`
- [ ] `--wait=ordered` accepted by CLI without error
- [ ] `--readiness-timeout` flag added to install and upgrade commands
- [ ] `ReadinessTimeout` field added to Install and Upgrade action structs
- [ ] Validation: readiness-timeout must not exceed timeout
- [ ] All tests pass

**Verify:**

- `cd /home/rohit/Documents/helm/.worktrees/spec-hip-0025-2daac031 && go test ./pkg/cmd/ -run TestWait -v && go test ./pkg/kube/ -run TestWaitStrategy -v`

---

### Task 6: Sequenced Install Action

**Objective:** Modify the install action to deploy resources in DAG-ordered batches when `--wait=ordered` is used. Process subcharts in dependency order, and within each chart, process resource-groups in order.

**Dependencies:** Task 1, Task 2, Task 3, Task 4, Task 5

**Files:**

- Modify: `pkg/action/install.go` — add `performSequencedInstall()` method
- Create: `pkg/action/sequencing.go` — shared sequencing logic (manifest splitting, batch execution)
- Test: `pkg/action/install_test.go` — sequenced install tests
- Test: `pkg/action/sequencing_test.go`

**Key Decisions / Notes:**

- **Two-level DAG composition:** When `WaitStrategy == OrderedWaitStrategy`:
  1. Build subchart DAG from chart metadata (Task 2)
  2. Get subchart installation batches (topological layers)
  3. For each subchart batch: process each subchart in the batch
  4. Within each subchart: build resource-group DAG (Task 3), get resource-group batches, deploy groups in order
  5. Parent chart's own resources (templates not belonging to any subchart) are deployed in the final batch, after all subchart batches complete. Within the parent chart, resource-group sequencing applies if annotations are present
  6. Unsequenced resources (no annotations or isolated groups) deployed last within their chart
- **Nested subcharts:** Handled recursively — each chart level processes its own direct dependencies via its own DAG. A subchart that itself has subcharts will recursively process its own DAG before being considered ready. DAG construction for all nesting levels happens upfront before any resources are created (fail-fast on cycles at any level). A subchart is "ready" only when its entire internal sequencing (including nested subcharts) is complete. The `--readiness-timeout` applies per-resource-group, not per-nesting-level
- **Manifest-to-subchart mapping:** Build the subchart-to-manifest mapping BEFORE the post-renderer runs, using the rendered file map keys (which contain subchart paths like `parentchart/charts/subchart/templates/deployment.yaml`). After post-rendering, use the preserved filename annotation from `annotateAndMerge`/`splitAndDeannotate` to map rendered manifests back to their subchart. Resources introduced by the post-renderer (not in the original file map) go to the unsequenced batch with a warning. Do NOT rely on inline `# Source:` comments as they are not present in the YAML stream sent to post-renderers
- CRD installation happens before sequencing begins, preserving existing behavior (CRDs are always earliest)
- Each batch: `KubeClient.Create()` then `Waiter.Wait()` (or custom readiness if annotations present)
- On failure: release marked as failed, rollback-on-failure respected
- **--atomic interaction:** When `--atomic` is set alongside `--wait=ordered`, a failure in any batch triggers automatic rollback to the last successful revision. The rollback itself uses reverse sequencing order if the previous release was also sequenced
- **Failure handling:** On batch failure, remaining batches are skipped, release is marked failed with info about which batch failed. If `--atomic`, resources are deleted in reverse of the batches that were successfully applied plus any partial batch. Partial progress is stored in SequencingInfo so rollback knows exactly which batches completed
- **Cumulative timeout:** The overall `--timeout` is the hard wall-clock limit for the entire operation. Each batch's readiness wait checks both its own `--readiness-timeout` AND the remaining time against `--timeout`, using whichever is smaller. `--readiness-timeout` is per-batch, not per-install
- When `WaitStrategy != OrderedWaitStrategy`: existing behavior unchanged

**Definition of Done:**

- [ ] `performSequencedInstall()` deploys resources in topological batch order
- [ ] Subchart ordering respected (subcharts with no deps installed first)
- [ ] Resource-group ordering within each chart respected
- [ ] Custom readiness annotations evaluated when present
- [ ] Unsequenced resources deployed after all sequenced groups
- [ ] Parent chart resources deployed after all subchart batches
- [ ] Nested subcharts with their own DAGs processed recursively
- [ ] Failure in any batch marks release as failed
- [ ] --atomic + --wait=ordered triggers rollback on batch failure
- [ ] Non-ordered installs unchanged
- [ ] All tests pass

**Verify:**

- `cd /home/rohit/Documents/helm/.worktrees/spec-hip-0025-2daac031 && go test ./pkg/action/ -run TestSequenc -v && go test ./pkg/action/ -run TestInstall -v`

---

### Task 7: Sequenced Upgrade Action

**Objective:** Modify the upgrade action to use sequenced deployment when `--wait=ordered` is used, following the same DAG-ordered batch approach as install.

**Dependencies:** Task 6

**Files:**

- Modify: `pkg/action/upgrade.go` — add sequenced upgrade path
- Test: `pkg/action/upgrade_test.go` — sequenced upgrade tests

**Key Decisions / Notes:**

- Upgrade follows the same sequencing order as install (not reverse)
- Reuse `sequencing.go` helpers from Task 6
- The upgrade action uses `KubeClient.Update()` instead of `Create()` — batch logic must handle this
- The `performUpgrade` goroutine pattern in upgrade.go should be preserved
- **Sequencing mode transitions:** When upgrading from non-sequenced to sequenced (`--wait=ordered` on v2 but not v1): apply resources in sequence order using `Update()` for existing resources. When upgrading from sequenced to non-sequenced: standard upgrade behavior, no sequencing. When resource-group assignments change between versions: use the NEW version's DAG for ordering

**Definition of Done:**

- [ ] Sequenced upgrade deploys in topological batch order
- [ ] Upgrade uses `Update()` with correct old/new resource lists per batch
- [ ] Non-ordered upgrades unchanged
- [ ] All tests pass

**Verify:**

- `cd /home/rohit/Documents/helm/.worktrees/spec-hip-0025-2daac031 && go test ./pkg/action/ -run TestUpgrade -v`

---

### Task 8: Release Metadata and Sequenced Rollback/Uninstall

**Objective:** Store sequencing metadata in the Release object so rollback and uninstall can respect the original deployment order (reversed for uninstall).

**Dependencies:** Task 6

**Files:**

- Modify: `pkg/release/v1/release.go` — add `SequencingInfo` field
- Modify: `pkg/action/rollback.go` — respect sequencing flag from stored release
- Modify: `pkg/action/uninstall.go` — reverse-order uninstall when sequenced
- Test: `pkg/action/rollback_test.go` — sequenced rollback tests
- Test: `pkg/action/uninstall_test.go` — sequenced uninstall tests

**Key Decisions / Notes:**

- `SequencingInfo` stored compactly with `json:",omitempty"` — only dependency edges stored, batches reconstructed at runtime via topological sort. Fields: `Enabled bool`, `Strategy string`, `Dependencies map[string][]string` (subchart/group -> list of dependencies). Batches are NOT pre-computed in storage to minimize Secret/ConfigMap size. Worst-case size increase verified to be acceptable for charts with 50+ subcharts
- Uninstall reverses the batch order: dependent resources deleted first, then their dependencies
- **Rollback two-phase behavior:** (1) Install the target revision's resources in sequenced order (if target was sequenced), and (2) delete resources from the current revision that are no longer needed, in reverse sequencing order (if current revision was sequenced)
- Rollback checks `SequencingInfo.Enabled` on the target revision — if true, use ordered install for that revision
- Backward compatibility: releases without `SequencingInfo` treated as non-sequenced (existing behavior)

**Definition of Done:**

- [ ] `SequencingInfo` stored in Release when `--wait=ordered` used
- [ ] Uninstall reverses sequencing order (dependents deleted before dependencies)
- [ ] Rollback installs target revision in sequenced order (if target was sequenced)
- [ ] Rollback deletes current revision's extra resources in reverse sequencing order
- [ ] Old releases without SequencingInfo handled gracefully
- [ ] All tests pass

**Verify:**

- `cd /home/rohit/Documents/helm/.worktrees/spec-hip-0025-2daac031 && go test ./pkg/action/ -run TestRollback -v && go test ./pkg/action/ -run TestUninstall -v`

---

### Task 9: helm template Resource-Group Delimiters

**Objective:** When `--wait=ordered` is used with `helm template`, output manifests in deployment order with `## START resource-group` and `## END resource-group` delimiters.

**Dependencies:** Task 3, Task 5

**Files:**

- Modify: `pkg/action/action.go` — modify `renderResources()` or add post-processing for sequenced output
- Modify: `pkg/cmd/template.go` — pass sequencing flag through
- Test: `pkg/cmd/template_test.go` — test delimited output

**Key Decisions / Notes:**

- Delimiter format per HIP spec: `## START resource-group: <chart>/<subchart> <group-name>` and `## END resource-group: <chart>/<subchart> <group-name>`
- Only applied when `--wait=ordered` is set on the template command
- Resources without groups are output at the end without delimiters
- Subchart ordering is also reflected in output order
- When post-renderer is used with `--wait=ordered` template, sequencing order is computed from pre-post-renderer manifests; emit a warning that displayed order may not match actual install order if post-renderer restructures manifests

**Definition of Done:**

- [ ] `helm template --wait=ordered` outputs manifests in deployment order
- [ ] Resource-group delimiters present in output matching HIP spec format
- [ ] Subchart manifests appear in dependency order
- [ ] Unsequenced resources appear at the end
- [ ] All tests pass

**Verify:**

- `cd /home/rohit/Documents/helm/.worktrees/spec-hip-0025-2daac031 && go test ./pkg/cmd/ -run TestTemplate -v`

---

### Task 10: Lint Rules for Sequencing

**Objective:** Add lint rules to detect sequencing misconfigurations: circular dependencies in resource-groups, circular dependencies in subcharts, and partial readiness annotations.

**Dependencies:** Task 1, Task 2, Task 3, Task 4

**Files:**

- Create: `pkg/chart/v2/lint/rules/sequencing.go` — sequencing lint rules
- Test: `pkg/chart/v2/lint/rules/sequencing_test.go`
- Modify: `pkg/chart/v2/lint/lint.go` — register new rules

**Key Decisions / Notes:**

- Circular dependency in subcharts: error severity
- Circular dependency in resource-groups: error severity (requires rendering templates first)
- Only one of `readiness-success`/`readiness-failure` present: error severity (per HIP spec, linting should fail)
- Resource referencing non-existent group: warning severity
- Resource in multiple groups: error severity
- Subchart depends-on referencing non-existent subchart name: error severity (disabled subcharts are handled at runtime by silently removing edges — lint cannot evaluate conditions/tags)

**Definition of Done:**

- [ ] Circular subchart dependency detected with error
- [ ] Partial readiness annotation (only one of success/failure) detected with error
- [ ] Resource referencing non-existent group detected with warning
- [ ] Lint rules registered in lint pipeline
- [ ] All tests pass

**Verify:**

- `cd /home/rohit/Documents/helm/.worktrees/spec-hip-0025-2daac031 && go test ./pkg/chart/v2/lint/... -v`

---

### Task 11: Warning System for Misconfigured Annotations

**Objective:** Emit clear warnings during install/upgrade when sequencing annotations are misconfigured (referencing non-existent groups, isolated groups, etc.).

**Dependencies:** Task 6

**Files:**

- Modify: `pkg/action/sequencing.go` — add warning emission during DAG construction
- Test: `pkg/action/sequencing_test.go` — warning tests

**Key Decisions / Notes:**

- Warnings emitted via `slog.Warn()` following existing Helm patterns
- Cases: resource references non-existent group, resource has sequencing annotations but falls into unsequenced batch, only one readiness annotation present
- Warnings are non-fatal — deployment continues with affected resources in unsequenced batch
- Per HIP spec: "Helm will emit a warning to alert the user of the potential issue"

**Definition of Done:**

- [ ] Warning emitted for non-existent group references
- [ ] Warning emitted for single readiness annotation (fallback to kstatus)
- [ ] Warning emitted for isolated groups (no dependencies, no dependents)
- [ ] Warnings use `slog.Warn()` with descriptive messages
- [ ] All tests pass

**Verify:**

- `cd /home/rohit/Documents/helm/.worktrees/spec-hip-0025-2daac031 && go test ./pkg/action/ -run TestSequencing -v`

## Testing Strategy

- **Unit tests:** DAG engine, annotation parsing, JSONPath evaluation, CLI flag parsing, readiness evaluation — all in isolation with mocked dependencies
- **Integration tests:** Install/upgrade/rollback/uninstall actions with `kubefake.PrintingKubeClient` — verify correct ordering and batching
- **Lint tests:** Chart fixtures with various sequencing configurations — verify correct error/warning detection
- **Manual verification:** `helm template --wait=ordered` with test charts demonstrating sequencing

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Circular dependencies cause install hang | Med | High | DAG cycle detection runs before any resources are created; clear error message with cycle path |
| Post-renderer restructures manifests breaking subchart identification | Med | Med | Use pre-post-renderer file map keys (preserved through annotateAndMerge/splitAndDeannotate) for subchart mapping; post-renderer-introduced resources go to unsequenced batch with warning |
| Readiness timeout exceeds overall --timeout | Med | Med | Validate `--readiness-timeout <= --timeout` at CLI parse time; error if violated |
| Large charts with many resource-groups cause performance regression | Low | Med | DAG construction is O(V+E); topological sort is O(V+E); no nested loops over manifests |
| Backward compatibility broken for Chart v2 | Low | High | Sequencing only activates with `--wait=ordered`; without it, zero behavior change |
| JSONPath evaluation fails on unexpected status shapes | Med | Low | Graceful error handling: log warning, treat as "not ready yet" rather than fatal error |

## Open Questions

- Should `helm get manifest` show resource-group delimiters for sequenced releases?
- Should there be a `helm sequencing` or `helm dag` subcommand for debugging dependency graphs? (Deferred per scope)
- Consider placing DAG engine and resource-group parsing in a dedicated `pkg/sequencing/` package instead of splitting across `pkg/chart/v2/util/` and `pkg/release/v1/util/` — this avoids cross-package coupling between release and chart utilities

### Deferred Ideas

- Interactive DAG visualization command (`helm dag show <chart>`) — explicitly mentioned in HIP-0025 spec as a planned feature
- Resource-group dependencies across charts (currently sandboxed per chart)
- `helm test` integration with sequencing order
- Readiness webhooks as an alternative to JSONPath annotations
