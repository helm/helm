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
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"helm.sh/helm/v4/pkg/kube"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

// makeTestManifest creates a minimal Manifest for testing.
func makeTestManifest(name, sourcePath string, annotations map[string]string) releaseutil.Manifest {
	content := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + name + "\n"
	if len(annotations) > 0 {
		content += "  annotations:\n"
		for k, v := range annotations {
			content += "    " + k + ": \"" + v + "\"\n"
		}
	}
	head := &releaseutil.SimpleHead{}
	head.Metadata = &struct {
		Name        string            `json:"name"`
		Annotations map[string]string `json:"annotations"`
	}{
		Name:        name,
		Annotations: annotations,
	}
	return releaseutil.Manifest{
		Name:    sourcePath,
		Content: content,
		Head:    head,
	}
}

func TestGroupManifestsByDirectSubchart(t *testing.T) {
	manifests := []releaseutil.Manifest{
		makeTestManifest("cm-parent", "mychart/templates/cm.yaml", nil),
		makeTestManifest("deploy-db", "mychart/charts/database/templates/deploy.yaml", nil),
		makeTestManifest("svc-db", "mychart/charts/database/templates/svc.yaml", nil),
		makeTestManifest("cm-app", "mychart/charts/app/templates/cm.yaml", nil),
		// nested subchart — belongs to "database" at parent level
		makeTestManifest("cm-nested", "mychart/charts/database/charts/cache/templates/cm.yaml", nil),
	}

	groups := GroupManifestsByDirectSubchart(manifests, "mychart")

	// Parent chart
	if len(groups[""]) != 1 {
		t.Errorf("expected 1 parent manifest, got %d", len(groups[""]))
	}
	if groups[""][0].Name != "mychart/templates/cm.yaml" {
		t.Errorf("unexpected parent manifest: %s", groups[""][0].Name)
	}

	// database subchart (including its nested subchart at this level)
	if len(groups["database"]) != 3 {
		t.Errorf("expected 3 database manifests (including nested), got %d: %v",
			len(groups["database"]), manifestNames(groups["database"]))
	}

	// app subchart
	if len(groups["app"]) != 1 {
		t.Errorf("expected 1 app manifest, got %d", len(groups["app"]))
	}
}

func TestGroupManifestsByDirectSubchart_EmptyChartName(t *testing.T) {
	// Edge case: chart name is empty — everything goes to parent
	manifests := []releaseutil.Manifest{
		makeTestManifest("cm", "templates/cm.yaml", nil),
	}
	groups := GroupManifestsByDirectSubchart(manifests, "")
	if len(groups[""]) != 1 {
		t.Errorf("expected 1 parent manifest for empty chart name, got %d", len(groups[""]))
	}
}

func TestGroupManifestsByDirectSubchart_OnlySubcharts(t *testing.T) {
	manifests := []releaseutil.Manifest{
		makeTestManifest("deploy", "chart/charts/sub1/templates/deploy.yaml", nil),
		makeTestManifest("svc", "chart/charts/sub2/templates/svc.yaml", nil),
	}
	groups := GroupManifestsByDirectSubchart(manifests, "chart")
	if len(groups[""]) != 0 {
		t.Errorf("expected no parent manifests, got %d", len(groups[""]))
	}
	if len(groups["sub1"]) != 1 {
		t.Errorf("expected 1 sub1 manifest, got %d", len(groups["sub1"]))
	}
	if len(groups["sub2"]) != 1 {
		t.Errorf("expected 1 sub2 manifest, got %d", len(groups["sub2"]))
	}
}

func TestBuildManifestYAML(t *testing.T) {
	manifests := []releaseutil.Manifest{
		makeTestManifest("cm1", "chart/templates/cm1.yaml", nil),
		makeTestManifest("cm2", "chart/templates/cm2.yaml", nil),
	}
	result := buildManifestYAML(manifests)
	if result == "" {
		t.Error("expected non-empty YAML output")
	}
	// Should contain content from both manifests
	if !contains(result, "name: cm1") {
		t.Error("expected cm1 in output")
	}
	if !contains(result, "name: cm2") {
		t.Error("expected cm2 in output")
	}
}

func TestBuildManifestYAML_Empty(t *testing.T) {
	result := buildManifestYAML(nil)
	if result != "" {
		t.Errorf("expected empty string for nil manifests, got %q", result)
	}
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestInstallRelease_StoresSequencingInfo verifies that a sequenced install stores
// SequencingInfo in the release record.
func TestInstallRelease_StoresSequencingInfo(t *testing.T) {
	config := actionConfigFixture(t)
	instAction := NewInstall(config)
	instAction.Namespace = "spaced"
	instAction.ReleaseName = "seq-info-test"
	instAction.WaitStrategy = kube.OrderedWaitStrategy
	instAction.Timeout = 5 * time.Minute
	instAction.ReadinessTimeout = time.Minute

	ch := buildChart(withSampleTemplates())
	reli, err := instAction.RunWithContext(context.Background(), ch, map[string]interface{}{})
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	rel, err := releaserToV1Release(reli)
	if err != nil {
		t.Fatalf("type assertion failed: %v", err)
	}
	if rel.SequencingInfo == nil {
		t.Fatal("expected SequencingInfo to be set after ordered install, got nil")
	}
	if !rel.SequencingInfo.Enabled {
		t.Error("expected SequencingInfo.Enabled to be true")
	}
}

// TestInstallRelease_OrderedWaitStrategy verifies that --wait=ordered installs
// succeed end-to-end using the fake kube client.
func TestInstallRelease_OrderedWaitStrategy(t *testing.T) {
	config := actionConfigFixture(t)
	instAction := NewInstall(config)
	instAction.Namespace = "spaced"
	instAction.ReleaseName = "seq-test"
	instAction.WaitStrategy = kube.OrderedWaitStrategy
	instAction.Timeout = 5 * time.Minute
	instAction.ReadinessTimeout = time.Minute

	ch := buildChart(withSampleTemplates())
	_, err := instAction.RunWithContext(context.Background(), ch, map[string]interface{}{})
	if err != nil {
		t.Fatalf("ordered install failed: %v", err)
	}
}

// TestInstallRelease_OrderedWaitStrategy_NilChart ensures a nil chart doesn't panic.
func TestInstallRelease_ReadinessTimeoutValidation(t *testing.T) {
	config := actionConfigFixture(t)
	instAction := NewInstall(config)
	instAction.Namespace = "spaced"
	instAction.ReleaseName = "timeout-test"
	instAction.WaitStrategy = kube.OrderedWaitStrategy
	instAction.Timeout = 5
	instAction.ReadinessTimeout = 10 // exceeds Timeout

	ch := buildChart(withSampleTemplates())
	_, err := instAction.RunWithContext(context.Background(), ch, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for ReadinessTimeout > Timeout, got nil")
	}
}

func manifestNames(ms []releaseutil.Manifest) []string {
	names := make([]string, len(ms))
	for i, m := range ms {
		names[i] = m.Name
	}
	return names
}

// captureWarnings redirects the default slog logger to a buffer for the duration
// of the test and returns a function to retrieve the captured output.
func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })
	return &buf
}

func TestWarnIfPartialReadinessAnnotations_OnlySuccess(t *testing.T) {
	buf := captureWarnings(t)
	manifests := []releaseutil.Manifest{
		makeTestManifest("cm", "chart/templates/cm.yaml", map[string]string{
			kube.AnnotationReadinessSuccess: "{.status.ready} == true",
		}),
	}
	warnIfPartialReadinessAnnotations(manifests)
	if !strings.Contains(buf.String(), "readiness") {
		t.Errorf("expected warning about partial readiness annotation, got: %q", buf.String())
	}
}

func TestWarnIfPartialReadinessAnnotations_OnlyFailure(t *testing.T) {
	buf := captureWarnings(t)
	manifests := []releaseutil.Manifest{
		makeTestManifest("cm", "chart/templates/cm.yaml", map[string]string{
			kube.AnnotationReadinessFailure: "{.status.failed} == true",
		}),
	}
	warnIfPartialReadinessAnnotations(manifests)
	if !strings.Contains(buf.String(), "readiness") {
		t.Errorf("expected warning about partial readiness annotation, got: %q", buf.String())
	}
}

func TestWarnIfPartialReadinessAnnotations_BothPresent(t *testing.T) {
	buf := captureWarnings(t)
	manifests := []releaseutil.Manifest{
		makeTestManifest("cm", "chart/templates/cm.yaml", map[string]string{
			kube.AnnotationReadinessSuccess: "{.status.ready} == true",
			kube.AnnotationReadinessFailure: "{.status.failed} == true",
		}),
	}
	warnIfPartialReadinessAnnotations(manifests)
	if buf.Len() > 0 {
		t.Errorf("expected no warning when both annotations present, got: %q", buf.String())
	}
}

func TestWarnIfPartialReadinessAnnotations_NeitherPresent(t *testing.T) {
	buf := captureWarnings(t)
	manifests := []releaseutil.Manifest{
		makeTestManifest("cm", "chart/templates/cm.yaml", nil),
	}
	warnIfPartialReadinessAnnotations(manifests)
	if buf.Len() > 0 {
		t.Errorf("expected no warning when neither annotation present, got: %q", buf.String())
	}
}

func TestWarnIfIsolatedGroups_TwoGroupsNoConnections(t *testing.T) {
	buf := captureWarnings(t)
	result := releaseutil.ResourceGroupResult{
		Groups: map[string][]releaseutil.Manifest{
			"groupA": {makeTestManifest("cmA", "chart/templates/cmA.yaml", nil)},
			"groupB": {makeTestManifest("cmB", "chart/templates/cmB.yaml", nil)},
		},
		GroupDeps: map[string][]string{},
	}
	warnIfIsolatedGroups(result)
	output := buf.String()
	if !strings.Contains(output, "isolated") {
		t.Errorf("expected warning about isolated groups, got: %q", output)
	}
}

func TestWarnIfIsolatedGroups_ConnectedGroups(t *testing.T) {
	buf := captureWarnings(t)
	result := releaseutil.ResourceGroupResult{
		Groups: map[string][]releaseutil.Manifest{
			"groupA": {makeTestManifest("cmA", "chart/templates/cmA.yaml", nil)},
			"groupB": {makeTestManifest("cmB", "chart/templates/cmB.yaml", nil)},
		},
		GroupDeps: map[string][]string{
			"groupB": {"groupA"}, // groupB depends on groupA
		},
	}
	warnIfIsolatedGroups(result)
	if buf.Len() > 0 {
		t.Errorf("expected no warning when groups are connected, got: %q", buf.String())
	}
}

func TestWarnIfIsolatedGroups_SingleGroup(t *testing.T) {
	buf := captureWarnings(t)
	result := releaseutil.ResourceGroupResult{
		Groups: map[string][]releaseutil.Manifest{
			"groupA": {makeTestManifest("cmA", "chart/templates/cmA.yaml", nil)},
		},
		GroupDeps: map[string][]string{},
	}
	warnIfIsolatedGroups(result)
	if buf.Len() > 0 {
		t.Errorf("expected no warning for single group, got: %q", buf.String())
	}
}
