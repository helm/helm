/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package action

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/kube"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

func captureWarningOutput(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	return &buf
}

func runSequencedInstallWithWarnings(t *testing.T, templates ...*common.File) string {
	t.Helper()

	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	install.cfg.SetLogger(handler)

	mustRunInstall(t, install, buildChartWithTemplates(templates))

	return buf.String()
}

func TestWarning_PartialReadinessAnnotations(t *testing.T) {
	output := runSequencedInstallWithWarnings(t,
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			kube.AnnotationReadinessSuccess: `["{.ready} == true"]`,
		}),
	)

	assert.Contains(t, output, "only one readiness annotation")
	assert.Contains(t, output, "resource=app")
}

func TestWarning_IsolatedResourceGroups(t *testing.T) {
	output := runSequencedInstallWithWarnings(t,
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/cache.yaml", "cache", map[string]string{
			releaseutil.AnnotationResourceGroup: "cache",
		}),
	)

	assert.Contains(t, output, "resource-group is isolated")
	assert.Contains(t, output, "group=database")
	assert.Contains(t, output, "group=cache")
}

func TestWarning_NonExistentGroupReference(t *testing.T) {
	output := runSequencedInstallWithWarnings(t,
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["missing"]`,
		}),
	)

	assert.Contains(t, output, "resource-group annotation warning")
	assert.Contains(t, output, "group")
	assert.Contains(t, output, "app")
	assert.Contains(t, output, "non-existent group")
	assert.Contains(t, output, "missing")
}

func TestWarning_WellFormedSequencing(t *testing.T) {
	output := runSequencedInstallWithWarnings(t,
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
			kube.AnnotationReadinessSuccess:     `["{.ready} == true"]`,
			kube.AnnotationReadinessFailure:     `["{.failed} == true"]`,
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	)

	assert.Empty(t, output)
}
