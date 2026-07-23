package action

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
)

func TestRenderRepoURL2_ValidAndInvalid(t *testing.T) {
	cfg := actionConfigFixture(t)

	tests := []struct {
		name       string
		chartPath  string
		expectPart string
	}{
		{
			name:       "valid repoURL",
			chartPath:  "testdata/charts/chart-with-repourl-valid",
			expectPart: `repoURL: "https://example.com/charts"`,
		},
		{
			name:       "invalid repoURL",
			chartPath:  "testdata/charts/chart-with-repourl-invalid",
			expectPart: `repoURL: "ht!tp://not-a-valid-url"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := loader.Load(tt.chartPath)
			require.NoError(t, err)
			if ch.Metadata == nil {
				md, err := chartutil.LoadChartfile(filepath.Join(tt.chartPath, "Chart.yaml"))
				require.NoError(t, err)
				ch.Metadata = md
			}

			_, buf, _, err := cfg.renderResources(
				ch,
				map[string]interface{}{},
				"test-release",
				"",
				false,
				false,
				false,
				nil,
				false,
				false,
				false,
			)
			require.NoError(t, err)
			require.NotNil(t, buf)
			assert.Contains(t, buf.String(), tt.expectPart)
		})
	}
}
