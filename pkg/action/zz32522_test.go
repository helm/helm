package action

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	common "helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
)

// TestInstallRunSubchartNullOverrideIsNotOverriddenByDefault reproduces
// helm/helm#32522 end-to-end through the exact `helm template` render path
// (install.Run -> ProcessDependencies -> ToRenderValues -> engine.Render).
//
// It builds a parent chart whose values carry a subchart override that
// nullifies a subchart default (grafana.securityContext.runAsUser: null), while
// the grafana subchart itself defaults runAsUser to 472. With the bug, the
// rendered manifest shows runAsUser: 472 (the default silently re-injected).
// With the fix, runAsUser renders as null/absent and 472 never appears.
func TestInstallRunSubchartNullOverrideIsNotOverriddenByDefault(t *testing.T) {
	req := require.New(t)

	// Subchart "grafana" with a default that the user wants to erase.
	grafana := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "grafana",
			Version:    "0.1.0",
			APIVersion: "v1",
		},
		Values: map[string]any{
			"securityContext": map[string]any{
				"runAsUser":  int64(472),
				"runAsGroup": int64(472),
				"fsGroup":    int64(472),
			},
		},
		Templates: []*common.File{
			{
				Name: "templates/cm.yaml",
				Data: []byte(
					"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: grafana\n" +
						"data:\n  runAsUser: '{{ .Values.securityContext.runAsUser }}'\n",
				),
			},
		},
	}

	// Parent chart whose values carry the subchart override that nullifies the
	// default. This mirrors what loader.Load puts into chrt.Values() from a
	// parent values.yaml `grafana:` block.
	parent := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "parent",
			Version:    "0.1.0",
			APIVersion: "v1",
		},
		Values: map[string]any{
			"grafana": map[string]any{
				"securityContext": map[string]any{
					"runAsUser":  nil,
					"runAsGroup": nil,
					"fsGroup":    nil,
				},
			},
		},
	}
	parent.AddDependency(grafana)

	inst := installAction(t)
	inst.DisableHooks = true
	// Client-side dry-run: the render path (ProcessDependencies -> ToRenderValues
	// -> engine.Render) is identical to a real install/template, but no cluster
	// interaction occurs.
	inst.DryRunStrategy = DryRunClient

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	rel, err := inst.RunWithContext(ctx, parent, map[string]any{})
	req.NoError(err)
	req.NotNil(rel)

	rendered, err := releaserToV1Release(rel)
	req.NoError(err)
	manifest := rendered.Manifest
	req.NotEmpty(manifest, "expected a rendered manifest")

	// The subchart default 472 must NOT have been re-injected.
	req.NotContains(manifest, "472", "subchart default 472 was re-injected (helm/helm#32522)")
	// The user's null must win: runAsUser renders as empty (null), not 472.
	req.Contains(manifest, "runAsUser: ''", "expected runAsUser to render as null/empty, not the subchart default")
	// Confirm we are actually rendering the grafana subchart (not a no-op).
	req.Contains(manifest, "name: grafana")
}
