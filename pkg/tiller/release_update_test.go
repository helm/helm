/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package tiller

import (
	"fmt"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"

	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
)

func TestUpdateRelease(t *testing.T) {
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)

	req := &services.UpdateReleaseRequest{
		Name: rel.Name,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithUpgradeHooks)},
			},
		},
	}
	res, err := rs.UpdateRelease(req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}

	if res.Name == "" {
		t.Errorf("Expected release name.")
	}

	if res.Name != rel.Name {
		t.Errorf("Updated release name does not match previous release name. Expected %s, got %s", rel.Name, res.Name)
	}

	if res.Namespace != rel.Namespace {
		t.Errorf("Expected release namespace '%s', got '%s'.", rel.Namespace, res.Namespace)
	}

	updated := compareStoredAndReturnedRelease(t, *rs, res)

	if len(updated.Hooks) != 1 {
		t.Fatalf("Expected 1 hook, got %d", len(updated.Hooks))
	}
	if updated.Hooks[0].Manifest != manifestWithUpgradeHooks {
		t.Errorf("Unexpected manifest: %v", updated.Hooks[0].Manifest)
	}

	if updated.Hooks[0].Events[0] != release.Hook_POST_UPGRADE {
		t.Errorf("Expected event 0 to be post upgrade")
	}

	if updated.Hooks[0].Events[1] != release.Hook_PRE_UPGRADE {
		t.Errorf("Expected event 0 to be pre upgrade")
	}

	if len(updated.Manifest) == 0 {
		t.Errorf("Expected manifest in %v", res)
	}

	if res.Config == nil {
		t.Errorf("Got release without config: %#v", res)
	} else if res.Config.Raw != rel.Config.Raw {
		t.Errorf("Expected release values %q, got %q", rel.Config.Raw, res.Config.Raw)
	}

	if !strings.Contains(updated.Manifest, "---\n# Source: hello/templates/hello\nhello: world") {
		t.Errorf("unexpected output: %s", updated.Manifest)
	}

	if res.Version != 2 {
		t.Errorf("Expected release version to be %v, got %v", 2, res.Version)
	}

	edesc := "Upgrade complete"
	if got := res.Info.Description; got != edesc {
		t.Errorf("Expected description %q, got %q", edesc, got)
	}
}
func TestUpdateRelease_ResetValues(t *testing.T) {
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)

	req := &services.UpdateReleaseRequest{
		Name: rel.Name,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithUpgradeHooks)},
			},
		},
		ResetValues: true,
	}
	res, err := rs.UpdateRelease(req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}
	// This should have been unset. Config:  &chart.Config{Raw: `name: value`},
	if res.Config != nil && res.Config.Raw != "" {
		t.Errorf("Expected chart config to be empty, got %q", res.Config.Raw)
	}
}

// This is a regression test for bug found in issue #3655
func TestUpdateRelease_ComplexReuseValues(t *testing.T) {
	rs := rsFixture()

	installReq := &services.InstallReleaseRequest{
		Namespace: "spaced",
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithHook)},
			},
			Values: &chart.Config{Raw: "defaultFoo: defaultBar"},
		},
		Values: &chart.Config{Raw: "foo: bar"},
	}

	fmt.Println("Running Install release with foo: bar override")
	installResp, err := rs.InstallRelease(installReq)
	if err != nil {
		t.Fatal(err)
	}

	rel := installResp
	req := &services.UpdateReleaseRequest{
		Name: rel.Name,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithUpgradeHooks)},
			},
			Values: &chart.Config{Raw: "defaultFoo: defaultBar"},
		},
	}

	fmt.Println("Running Update release with no overrides and no reuse-values flag")
	res, err := rs.UpdateRelease(req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}

	expect := "foo: bar"
	if res.Config != nil && res.Config.Raw != expect {
		t.Errorf("Expected chart values to be %q, got %q", expect, res.Config.Raw)
	}

	rel = res
	req = &services.UpdateReleaseRequest{
		Name: rel.Name,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithUpgradeHooks)},
			},
			Values: &chart.Config{Raw: "defaultFoo: defaultBar"},
		},
		Values:      &chart.Config{Raw: "foo2: bar2"},
		ReuseValues: true,
	}

	fmt.Println("Running Update release with foo2: bar2 override and reuse-values")
	rel, err = rs.UpdateRelease(req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}

	// This should have the newly-passed overrides.
	expect = "foo: bar\nfoo2: bar2\n"
	if rel.Config != nil && rel.Config.Raw != expect {
		t.Errorf("Expected request config to be %q, got %q", expect, rel.Config.Raw)
	}

	req = &services.UpdateReleaseRequest{
		Name: rel.Name,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithUpgradeHooks)},
			},
			Values: &chart.Config{Raw: "defaultFoo: defaultBar"},
		},
		Values:      &chart.Config{Raw: "foo: baz"},
		ReuseValues: true,
	}

	fmt.Println("Running Update release with foo=baz override with reuse-values flag")
	res, err = rs.UpdateRelease(req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}
	expect = "foo: baz\nfoo2: bar2\n"
	if res.Config != nil && res.Config.Raw != expect {
		t.Errorf("Expected chart values to be %q, got %q", expect, res.Config.Raw)
	}
}

func TestUpdateRelease_ReuseValues(t *testing.T) {
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)

	req := &services.UpdateReleaseRequest{
		Name: rel.Name,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithUpgradeHooks)},
			},
			// Since reuseValues is set, this should get ignored.
			Values: &chart.Config{Raw: "foo: bar\n"},
		},
		Values:      &chart.Config{Raw: "name2: val2"},
		ReuseValues: true,
	}
	res, err := rs.UpdateRelease(req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}
	// This should have been overwritten with the old value.
	expect := "name: value\n"
	if res.Chart.Values != nil && res.Chart.Values.Raw != expect {
		t.Errorf("Expected chart values to be %q, got %q", expect, res.Chart.Values.Raw)
	}
	// This should have the newly-passed overrides and any other computed values. `name: value` comes from release Config via releaseStub()
	expect = "name: value\nname2: val2\n"
	if res.Config != nil && res.Config.Raw != expect {
		t.Errorf("Expected request config to be %q, got %q", expect, res.Config.Raw)
	}
	compareStoredAndReturnedRelease(t, *rs, res)
}

func TestUpdateRelease_ResetReuseValues(t *testing.T) {
	// This verifies that when both reset and reuse are set, reset wins.
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)

	req := &services.UpdateReleaseRequest{
		Name: rel.Name,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithUpgradeHooks)},
			},
		},
		ResetValues: true,
		ReuseValues: true,
	}
	res, err := rs.UpdateRelease(req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}
	// This should have been unset. Config:  &chart.Config{Raw: `name: value`},
	if res.Config != nil && res.Config.Raw != "" {
		t.Errorf("Expected chart config to be empty, got %q", res.Config.Raw)
	}
	compareStoredAndReturnedRelease(t, *rs, res)
}

func TestUpdateReleaseFailure(t *testing.T) {
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)
	rs.env.KubeClient = newUpdateFailingKubeClient()
	rs.Log = t.Logf

	req := &services.UpdateReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/something", Data: []byte("hello: world")},
			},
		},
	}

	res, err := rs.UpdateRelease(req)
	if err == nil {
		t.Error("Expected failed update")
	}

	if updatedStatus := res.Info.Status.Code; updatedStatus != release.Status_FAILED {
		t.Errorf("Expected FAILED release. Got %d", updatedStatus)
	}

	compareStoredAndReturnedRelease(t, *rs, res)

	expectedDescription := "Upgrade \"angry-panda\" failed: Failed update in kube client"
	if got := res.Info.Description; got != expectedDescription {
		t.Errorf("Expected description %q, got %q", expectedDescription, got)
	}

	oldRelease, err := rs.env.Releases.Get(rel.Name, rel.Version)
	if err != nil {
		t.Errorf("Expected to be able to get previous release")
	}
	if oldStatus := oldRelease.Info.Status.Code; oldStatus != release.Status_DEPLOYED {
		t.Errorf("Expected Deployed status on previous Release version. Got %v", oldStatus)
	}
}

func TestUpdateReleaseFailure_Force(t *testing.T) {
	rs := rsFixture()
	rel := namedReleaseStub("forceful-luke", release.Status_FAILED)
	rs.env.Releases.Create(rel)
	rs.Log = t.Logf

	req := &services.UpdateReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/something", Data: []byte("text: 'Did you ever hear the tragedy of Darth Plagueis the Wise? I thought not. It’s not a story the Jedi would tell you. It’s a Sith legend. Darth Plagueis was a Dark Lord of the Sith, so powerful and so wise he could use the Force to influence the Midichlorians to create life... He had such a knowledge of the Dark Side that he could even keep the ones he cared about from dying. The Dark Side of the Force is a pathway to many abilities some consider to be unnatural. He became so powerful... The only thing he was afraid of was losing his power, which eventually, of course, he did. Unfortunately, he taught his apprentice everything he knew, then his apprentice killed him in his sleep. Ironic. He could save others from death, but not himself.'")},
			},
		},
		Force: true,
	}

	res, err := rs.UpdateRelease(req)
	if err != nil {
		t.Errorf("Expected successful update, got %v", err)
	}

	if updatedStatus := res.Info.Status.Code; updatedStatus != release.Status_DEPLOYED {
		t.Errorf("Expected DEPLOYED release. Got %d", updatedStatus)
	}

	compareStoredAndReturnedRelease(t, *rs, res)

	expectedDescription := "Upgrade complete"
	if got := res.Info.Description; got != expectedDescription {
		t.Errorf("Expected description %q, got %q", expectedDescription, got)
	}

	oldRelease, err := rs.env.Releases.Get(rel.Name, rel.Version)
	if err != nil {
		t.Errorf("Expected to be able to get previous release")
	}
	if oldStatus := oldRelease.Info.Status.Code; oldStatus != release.Status_DELETED {
		t.Errorf("Expected Deleted status on previous Release version. Got %v", oldStatus)
	}
}

func TestUpdateReleaseNoHooks(t *testing.T) {
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)

	req := &services.UpdateReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "hello"},
			Templates: []*chart.Template{
				{Name: "templates/hello", Data: []byte("hello: world")},
				{Name: "templates/hooks", Data: []byte(manifestWithUpgradeHooks)},
			},
		},
	}

	res, err := rs.UpdateRelease(req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}

	if hl := res.Hooks[0].LastRun; hl != nil {
		t.Errorf("Expected that no hooks were run. Got %d", hl)
	}

}

func TestUpdateReleaseNoChanges(t *testing.T) {
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)

	req := &services.UpdateReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
		Chart:        rel.GetChart(),
	}

	_, err := rs.UpdateRelease(req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}
}

func compareStoredAndReturnedRelease(t *testing.T, rs ReleaseServer, res *release.Release) *release.Release {
	storedRelease, err := rs.env.Releases.Get(res.Name, res.Version)
	if err != nil {
		t.Fatalf("Expected release for %s (%v).", res.Name, rs.env.Releases)
	}

	if !proto.Equal(storedRelease, res) {
		t.Errorf("Stored release doesn't match returned Release")
	}

	return storedRelease
}
