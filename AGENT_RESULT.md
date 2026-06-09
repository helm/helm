# Agent Result

## Root Cause

In `processDependencyEnabled(c *chart.Chart, v map[string]any, path string)`, recursive calls (when `path != ""`) called `util.CoalesceValues(c, v)` where `v` was the fully coalesced parent values. These parent values contain top-level keys (e.g., `tolerations: {}`) that are unrelated to the sub-chart being processed. When `coalesceValues` iterated over sub-chart defaults (e.g., `tolerations: []`), it found the parent-level `tolerations: {}` entry in `v` as the destination. Since a map cannot be merged with a slice, the warning `warning: skipped value for <path>tolerations: Not a table.` was emitted spuriously.

The second `CoalesceValues` call in recursive invocations is redundant: when `ProcessDependencies` calls `processDependencyEnabled(parent, userVals, "")`, the initial `CoalesceValues(parent, userVals)` internally runs `coalesceDeps` which already merges all sub-chart defaults recursively. The subsequent `CoalesceValues(sub-chart, full_parent_cvals)` in each recursive call re-processes sub-chart defaults against the wrong scope.

## Change Made

- `pkg/chart/v2/util/dependencies.go` - `processDependencyEnabled`: When `path != ""` (recursive call), skip `util.CoalesceValues` and set `cvals = v` directly. The parent values are already fully coalesced and scoped correctly for tag/condition evaluation.
- `internal/chart/v3/util/dependencies.go` - `processDependencyEnabled`: Identical change for the v3 chart format.

## Testing

Added regression test `TestProcessDependencyEnabledNoSpuriousWarnings` to both:
- `pkg/chart/v2/util/dependencies_test.go`
- `internal/chart/v3/util/dependencies_test.go`

The test creates a 3-level chart hierarchy (parent -> subchart -> subsubchart) where `parent.Values["tolerations"]` is a map and `subchart.Values["tolerations"]` is a slice. It redirects standard log output via `log.SetOutput` and verifies no "Not a table" warning is emitted after the fix.

Go is not available in this environment so tests could not be executed directly.

## Lint

Golangci-lint is not available in this environment. The changes are minimal - only flow control logic was modified (added `if/else` block replacing a direct `CoalesceValues` call). No new functions, types, or external dependencies were introduced. All imports used in the modified files were already present.
