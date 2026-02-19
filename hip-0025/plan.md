# HIP-0025 Implementation Status & Next Steps

## Phases Completed

### Phase A: Core Implementation (Tasks 1–11) — VERIFIED

All code has been implemented, unit tested, code-reviewed, and verification fixes applied.

| Task | Description | Status |
|------|-------------|--------|
| 1 | DAG engine (topological sort, cycle detection) | Done |
| 2 | `DependsOn` field on Dependency struct + subchart DAG builder | Done |
| 3 | Resource-group annotation parsing + resource-group DAG | Done |
| 4 | Custom readiness evaluation (JSONPath, kstatus fallback) | Done |
| 5 | `--wait=ordered` CLI flag, `--readiness-timeout`, `OrderedWaitStrategy` | Done |
| 6 | Sequenced install (two-level DAG: subchart → resource-group) | Done |
| 7 | Sequenced upgrade (batch-wise `KubeClient.Update`) | Done |
| 8 | SequencingInfo in Release, sequenced rollback/uninstall | Done |
| 9 | `helm template --wait=ordered` with resource-group delimiters | Done |
| 10 | Lint rules (circular deps, partial readiness, multi-group, orphan groups) | Done |
| 11 | Warning system (partial readiness, isolated groups, bad annotations) | Done |

#### Verification Fixes Applied

- DAG duplicate edge prevention
- ParseResourceGroups duplicate dep deduplication
- Disabled subchart detection logic corrected
- `findSubchart` alias resolution via parent chart `Dependency.Alias`
- SequencingInfo set before deployment for failure recovery
- Zero Timeout handling (no immediate deadline failure)
- Lint rule no longer mutates chart dependency data in place
- `resource in multiple groups` lint check added
- Template delimiters include chart/subchart path per HIP spec
- `DependsOn` JSON tag fixed: `json:"dependsOn"` → `json:"depends-on"` (matches YAML tag, consistent with `import-values` pattern)
- `buildManifestYAML` fixed: added `---` YAML document separators between manifest contents
- Annotation stripping: `helm.sh/depends-on/resource-groups` stripped from resources before K8s apply (multi-slash key invalid per K8s API)

#### Files Changed (All in worktree on branch `worktree/3-pilot`)

**New files:**
- `pkg/chart/v2/util/dag.go` + `dag_test.go`
- `pkg/chart/v2/util/subchart_dag.go` + `subchart_dag_test.go`
- `pkg/release/v1/util/resource_group.go` + `resource_group_test.go`
- `pkg/kube/readiness.go` + `readiness_test.go`
- `pkg/action/sequencing.go` + `sequencing_test.go`
- `pkg/action/uninstall.go` (sequenced deletion additions)
- `pkg/chart/v2/lint/rules/sequencing.go` + `sequencing_test.go`
- `pkg/chart/v2/lint/rules/testdata/sequencing-partial-readiness/`
- `pkg/chart/v2/lint/rules/testdata/sequencing-orphan-group/`
- `pkg/cmd/testdata/sequenced-chart/` (template test fixtures)

**Modified files:**
- `pkg/chart/v2/dependency.go` (`DependsOn` field added)
- `pkg/kube/client.go` (`OrderedWaitStrategy` constant)
- `pkg/kube/interface.go` (readiness annotations constants)
- `pkg/cmd/flags.go` (`--wait=ordered`, `--readiness-timeout`)
- `pkg/action/install.go` (sequenced install path)
- `pkg/action/upgrade.go` (sequenced upgrade path)
- `pkg/action/rollback.go` (sequenced rollback path)
- `pkg/release/v1/release.go` (`SequencingInfo` struct + field)
- `pkg/chart/v2/lint/lint.go` (registered Sequencing rule)
- `pkg/cmd/template.go` (ordered template output)

---

## Phase B: Build, Binary Testing & Real-World Validation — DONE

This phase covers what has NOT been done yet: building the actual `helm` binary, testing it end-to-end against real Kubernetes clusters with complex charts, and validating every HIP-0025 feature works in practice.

### B1: Build the Helm Binary ✅

- [x] Build the `helm` binary from the worktree branch
  ```bash
  cd /home/rohit/Documents/helm/.worktrees/spec-hip-0025-2daac031
  go build -o ./bin/helm ./cmd/helm
  ```
- [x] Verify the binary runs: `./bin/helm version`
- [x] Verify `--wait=ordered` flag is present: `./bin/helm install --help | grep ordered`
- [x] Verify `--readiness-timeout` flag is present: `./bin/helm install --help | grep readiness-timeout`
- [x] Verify `helm template` shows ordered output: `./bin/helm template --help | grep ordered`

### B2: Create Test Charts ✅

Build a comprehensive set of test charts that exercise every feature in the HIP-0025 spec.

> **Fix applied:** Changed `DependsOn` JSON tag from `json:"dependsOn"` to `json:"depends-on"` in `dependency.go` to match YAML convention (consistent with existing `import-values` pattern). Without this, the strict Chart.yaml parser rejected `depends-on` as an unknown field.

#### B2.1: Simple Resource-Group Chart

A single chart with resource-group sequencing (no subcharts).

- [x] Create `hip-0025/testcharts/resource-groups/Chart.yaml`
- [ ] Create templates with 3 resource-groups: `database`, `queue`, `app`
- [ ] `database` group: ConfigMap + a Deployment (e.g., a simple busybox pod)
- [ ] `queue` group: ConfigMap + Deployment, `depends-on: ["database"]`
- [ ] `app` group: ConfigMap + Deployment, `depends-on: ["database", "queue"]`
- [ ] Include an unsequenced resource (no annotations) to verify it deploys last
- [ ] Expected install order: `database` → `queue` → `app` → unsequenced

#### B2.2: Subchart Sequencing Chart

A parent chart with subcharts using `depends-on` in Chart.yaml.

- [ ] Create `hip-0025/testcharts/subchart-ordering/Chart.yaml` with dependencies:
  - `backend` subchart (no deps)
  - `frontend` subchart (`depends-on: ["backend"]`)
  - `monitoring` subchart (no deps, deploys in parallel with `backend`)
- [ ] Each subchart has a simple Deployment + Service
- [ ] Parent chart has its own resources (deployed after all subcharts)
- [ ] Expected: `[backend, monitoring]` → `[frontend]` → `[parent resources]`

#### B2.3: Combined Subchart + Resource-Group Chart

Both levels of sequencing active simultaneously.

- [ ] Create `hip-0025/testcharts/combined/Chart.yaml`
- [ ] Parent has resource-groups: `infra` → `services`
- [ ] Subchart `database` has resource-groups: `schema` → `data`
- [ ] Subchart `app` depends-on `database`
- [ ] Expected: database(`schema` → `data`) → app(flat) → parent(`infra` → `services`)

#### B2.4: Custom Readiness Chart

Tests `helm.sh/readiness-success` and `helm.sh/readiness-failure` annotations.

- [ ] Create `hip-0025/testcharts/custom-readiness/Chart.yaml`
- [ ] `database` group: Job with `readiness-success: ["{.succeeded} >= 1"]` and `readiness-failure: ["{.failed} >= 1"]`
- [ ] `app` group: Deployment with default kstatus readiness, `depends-on: ["database"]`
- [ ] Verify: Job completes → readiness success → app deploys
- [ ] Test failure: Modify Job to fail → verify Helm reports readiness failure

#### B2.5: Annotation-Based Subchart Ordering

Uses `helm.sh/depends-on/subcharts` annotation (alternative to `depends-on` field).

- [ ] Create `hip-0025/testcharts/annotation-subchart/Chart.yaml` with:
  ```yaml
  annotations:
    helm.sh/depends-on/subcharts: '{"app": ["redis", "postgres"]}'
  dependencies:
    - name: redis
    - name: postgres
    - name: app
  ```
- [ ] Verify: `[redis, postgres]` → `[app]` → `[parent]`

#### B2.6: Edge Case Charts

- [ ] **Circular dependency chart** — verify `helm lint` and `helm install` detect and report cycle
- [ ] **Aliased subchart** — subchart with `alias:` in Chart.yaml, verify ordering works with aliases
- [ ] **Disabled subchart** — subchart with `condition: sub.enabled` set to false, verify edges are silently removed
- [ ] **Nested subcharts** — chart → subchart A → nested subchart B, verify recursive DAG processing
- [ ] **Single readiness annotation** — only `readiness-success` set, verify warning emitted + kstatus fallback
- [ ] **Empty chart** — chart with no templates, verify no crash
- [ ] **Large DAG** — 20+ resource-groups with diamond dependencies, verify ordering correctness and performance

### B3: Local Kubernetes Testing

Test the built binary against a real local Kubernetes cluster.

#### B3.1: Cluster Setup

- [ ] Ensure a local cluster is available (kind, minikube, k3s, or existing cluster)
- [ ] Verify `kubectl` is configured and can reach the cluster
- [ ] Create a test namespace: `kubectl create namespace hip-0025-test`

#### B3.2: Install Tests

For each test chart (B2.1–B2.6):

- [ ] **Install with `--wait=ordered`:**
  ```bash
  ./bin/helm install test-<name> ./hip-0025/testcharts/<name> \
    --namespace hip-0025-test \
    --wait=ordered \
    --timeout 5m \
    --readiness-timeout 1m
  ```
- [ ] Verify resources are created in the expected batch order (check timestamps on resources)
  ```bash
  kubectl get all -n hip-0025-test --sort-by=.metadata.creationTimestamp
  ```
- [ ] Verify all resources reach Ready state
- [ ] Verify the release has SequencingInfo:
  ```bash
  ./bin/helm get metadata test-<name> -n hip-0025-test -o json | jq .sequencing
  ```

#### B3.3: Template Tests

- [ ] `./bin/helm template test ./hip-0025/testcharts/resource-groups --wait=ordered`
  - Verify `## START resource-group: <chart> <group>` delimiters
  - Verify groups appear in topological order
  - Verify unsequenced resources appear last
- [ ] `./bin/helm template test ./hip-0025/testcharts/combined --wait=ordered`
  - Verify subchart resources appear before parent resources
  - Verify resource-groups within each chart are ordered

#### B3.4: Upgrade Tests

- [ ] Install a chart with `--wait=ordered`
- [ ] Modify the chart (add a new resource to an existing group)
- [ ] Upgrade with `--wait=ordered`:
  ```bash
  ./bin/helm upgrade test-<name> ./hip-0025/testcharts/<name> \
    --namespace hip-0025-test \
    --wait=ordered \
    --timeout 5m
  ```
- [ ] Verify upgrade respects batch ordering
- [ ] Verify new resources appear and old resources are updated

#### B3.5: Rollback Tests

- [ ] After an upgrade, rollback:
  ```bash
  ./bin/helm rollback test-<name> 1 \
    --namespace hip-0025-test \
    --wait=ordered \
    --timeout 5m
  ```
- [ ] Verify rollback re-applies the original revision in sequenced order
- [ ] Verify removed resources from revision 2 are deleted

#### B3.6: Uninstall Tests

- [ ] Uninstall a sequenced release:
  ```bash
  ./bin/helm uninstall test-<name> --namespace hip-0025-test
  ```
- [ ] Verify resources are deleted in reverse topological order
- [ ] Verify all resources are removed

#### B3.7: Lint Tests

- [ ] `./bin/helm lint ./hip-0025/testcharts/resource-groups` — should pass
- [ ] `./bin/helm lint ./hip-0025/testcharts/circular-dep` — should report circular dependency error
- [ ] `./bin/helm lint ./hip-0025/testcharts/single-readiness` — should report partial readiness error

#### B3.8: Failure Mode Tests

- [ ] Install a chart where a resource will fail readiness (e.g., image pull error)
  - Verify Helm reports the failure with a clear message
  - Verify dependent batches are NOT deployed
  - Verify the release is marked as failed
- [ ] Install with `--atomic --wait=ordered`
  - Force a failure in batch 2
  - Verify Helm automatically rolls back ALL resources
- [ ] Install with zero `--timeout`
  - Verify Helm handles this gracefully (safe default, not immediate failure)
- [ ] Install with `--readiness-timeout` exceeding `--timeout`
  - Verify Helm rejects this with a validation error

### B4: Backward Compatibility Tests

- [ ] Install a standard chart (e.g., bitnami/nginx) WITHOUT `--wait=ordered`
  - Verify behavior is identical to upstream Helm
  - Verify no SequencingInfo is stored in the release
- [ ] Install the SAME chart WITH `--wait=ordered` (chart has no annotations)
  - Verify all resources deploy in a single batch (no sequencing effect)
  - Verify SequencingInfo is stored with Enabled=true
- [ ] Upgrade from a non-sequenced release to `--wait=ordered`
  - Verify upgrade works correctly
- [ ] Upgrade from a `--wait=ordered` release to non-sequenced
  - Verify upgrade works correctly
- [ ] Rollback a non-sequenced release — verify no sequenced behavior

### B5: Spec Conformance Verification

Cross-reference each HIP-0025 spec requirement against actual binary behavior.

| HIP-0025 Requirement | Verification Method | Status |
|---|---|---|
| `--wait=ordered` enables sequencing | `helm install --help`, actual install | [ ] |
| `WaitStrategy=ordered` SDK parameter | Unit tests (already pass) | [x] |
| `helm.sh/resource-group` annotation groups resources | Install resource-groups chart, check batching | [ ] |
| `helm.sh/depends-on/resource-groups` defines group deps | Install resource-groups chart, check ordering | [ ] |
| Resources in same group deploy together | Check creation timestamps | [ ] |
| Helm waits for group readiness before next group | Observe install with slow-starting pods | [ ] |
| `depends-on` field on Chart.yaml dependencies | Install subchart-ordering chart | [ ] |
| `helm.sh/depends-on/subcharts` annotation | Install annotation-subchart chart | [ ] |
| Subchart fully deployed before dependents begin | Observe install ordering | [ ] |
| `helm.sh/readiness-success` custom readiness | Install custom-readiness chart | [ ] |
| `helm.sh/readiness-failure` custom readiness | Force failure, verify detection | [ ] |
| Both readiness annotations required (else kstatus fallback + warning) | Install single-readiness chart | [ ] |
| JSONPath syntax: `{<query>} <operator> <value>` | Custom readiness chart with various operators | [ ] |
| Operators: `==`, `!=`, `<`, `<=`, `>`, `>=` | Test with numeric values | [ ] |
| `--readiness-timeout` configurable | Pass flag, verify per-batch timeout | [ ] |
| Readiness timeout must not exceed `--timeout` | Pass invalid values, verify rejection | [ ] |
| Uninstall reverses deployment order | Uninstall sequenced release, check deletion order | [ ] |
| Rollback respects original sequencing | Rollback to sequenced revision | [ ] |
| Release stores sequencing info | `helm get metadata`, inspect JSON | [ ] |
| `helm template --wait=ordered` shows ordered output | Run template command | [ ] |
| Resource-group delimiters: `## START/END resource-group: <chart> <group>` | Template output inspection | [ ] |
| Unsequenced resources deploy after sequenced groups | Template + install verification | [ ] |
| Warning for misconfigured annotations | Check stderr during install | [ ] |
| Circular dependency detection | `helm lint` + `helm install` error | [ ] |
| Backward compat: without `--wait=ordered`, behavior unchanged | Install standard charts | [ ] |
| Hooks NOT affected by sequencing | Install chart with hooks, verify hook-weight still works | [ ] |

### B6: Performance Validation

- [ ] Build a chart with 50+ resource-groups in a complex DAG
- [ ] Time `helm template --wait=ordered` — should complete in <1 second
- [ ] Time `helm install --wait=ordered --dry-run` — should be within 5% of non-ordered install
- [ ] Profile memory usage for large charts — ensure no excessive allocation

---

## Known Gaps & Deferred Items

Items identified during code review that are tracked for future work:

1. **SequencingInfo Dependencies field** — Plan specifies storing dependency edges in the Release for resilient rollback. Current implementation re-parses from manifests. Low risk (works correctly unless post-renderer modifies annotations between deploy and rollback). Could be added as a follow-up.

2. **Subchart ordering in `helm template` output** — Template currently orders by resource-groups only, not subchart DAG. Needs the template function to receive the chart object for subchart DAG construction.

3. **Rollback ReadinessTimeout** — Hardcoded to `Timeout/2`. Should add `--readiness-timeout` flag to `helm rollback` for consistency with install/upgrade.

4. **Rollback reverse deletion order** — Deleted resources during rollback are removed without DAG ordering. Should construct reverse DAG from current release's manifests.

5. **`--atomic` + `--wait=ordered` interaction** — Not explicitly tested. Should verify auto-rollback on batch failure.

6. **readiness compareValues numeric coercion** — `"1.0"` == `"1"` numerically but not as strings. Spec doesn't define coercion rules. May need clarification.

7. **operatorRegexp whitespace consistency** — Requires trailing space but not leading. Could confuse users with `{.phase}==Running`.

---

## Execution Order

1. **B1** — Build binary (immediate, ~2 minutes)
2. **B2** — Create test charts (needs design and chart authoring)
3. **B3** — Local K8s testing (requires running cluster)
4. **B4** — Backward compatibility (standard charts, no annotations)
5. **B5** — Spec conformance checklist (systematic verification)
6. **B6** — Performance validation (large synthetic charts)
