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
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
)

func TestUpdateRelease(t *testing.T) {
	c := helm.NewContext()
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
	res, err := rs.UpdateRelease(c, req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}

	if res.Release.Name == "" {
		t.Errorf("Expected release name.")
	}

	if res.Release.Name != rel.Name {
		t.Errorf("Updated release name does not match previous release name. Expected %s, got %s", rel.Name, res.Release.Name)
	}

	if res.Release.Namespace != rel.Namespace {
		t.Errorf("Expected release namespace '%s', got '%s'.", rel.Namespace, res.Release.Namespace)
	}

	updated := compareStoredAndReturnedRelease(t, *rs, *res)

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

	if res.Release.Config == nil {
		t.Errorf("Got release without config: %#v", res.Release)
	} else if res.Release.Config.Raw != rel.Config.Raw {
		t.Errorf("Expected release values %q, got %q", rel.Config.Raw, res.Release.Config.Raw)
	}

	if !strings.Contains(updated.Manifest, "---\n# Source: hello/templates/hello\nhello: world") {
		t.Errorf("unexpected output: %s", updated.Manifest)
	}

	if res.Release.Version != 2 {
		t.Errorf("Expected release version to be %v, got %v", 2, res.Release.Version)
	}

	edesc := "Upgrade complete"
	if got := res.Release.Info.Description; got != edesc {
		t.Errorf("Expected description %q, got %q", edesc, got)
	}
}
func TestUpdateRelease_ResetValues(t *testing.T) {
	c := helm.NewContext()
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
	res, err := rs.UpdateRelease(c, req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}
	// This should have been unset. Config:  &chart.Config{Raw: `name: value`},
	if res.Release.Config != nil && res.Release.Config.Raw != "" {
		t.Errorf("Expected chart config to be empty, got %q", res.Release.Config.Raw)
	}
}

func TestUpdateRelease_ReuseValues(t *testing.T) {
	c := helm.NewContext()
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
	res, err := rs.UpdateRelease(c, req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}
	// This should have been overwritten with the old value.
	expect := "name: value\n"
	if res.Release.Chart.Values != nil && res.Release.Chart.Values.Raw != expect {
		t.Errorf("Expected chart values to be %q, got %q", expect, res.Release.Chart.Values.Raw)
	}
	// This should have the newly-passed overrides.
	expect = "name2: val2"
	if res.Release.Config != nil && res.Release.Config.Raw != expect {
		t.Errorf("Expected request config to be %q, got %q", expect, res.Release.Config.Raw)
	}
	compareStoredAndReturnedRelease(t, *rs, *res)
}

func TestUpdateRelease_ResetReuseValues(t *testing.T) {
	// This verifies that when both reset and reuse are set, reset wins.
	c := helm.NewContext()
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
	res, err := rs.UpdateRelease(c, req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}
	// This should have been unset. Config:  &chart.Config{Raw: `name: value`},
	if res.Release.Config != nil && res.Release.Config.Raw != "" {
		t.Errorf("Expected chart config to be empty, got %q", res.Release.Config.Raw)
	}
	compareStoredAndReturnedRelease(t, *rs, *res)
}

func TestUpdateReleaseFailure(t *testing.T) {
	c := helm.NewContext()
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

	res, err := rs.UpdateRelease(c, req)
	if err == nil {
		t.Error("Expected failed update")
	}

	if updatedStatus := res.Release.Info.Status.Code; updatedStatus != release.Status_FAILED {
		t.Errorf("Expected FAILED release. Got %d", updatedStatus)
	}

	compareStoredAndReturnedRelease(t, *rs, *res)

	edesc := "Upgrade \"angry-panda\" failed: Failed update in kube client"
	if got := res.Release.Info.Description; got != edesc {
		t.Errorf("Expected description %q, got %q", edesc, got)
	}

	oldRelease, err := rs.env.Releases.Get(rel.Name, rel.Version)
	if err != nil {
		t.Errorf("Expected to be able to get previous release")
	}
	if oldStatus := oldRelease.Info.Status.Code; oldStatus != release.Status_DEPLOYED {
		t.Errorf("Expected Deployed status on previous Release version. Got %v", oldStatus)
	}
}

func TestUpdateReleaseNoHooks(t *testing.T) {
	c := helm.NewContext()
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

	res, err := rs.UpdateRelease(c, req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}

	if hl := res.Release.Hooks[0].LastRun; hl != nil {
		t.Errorf("Expected that no hooks were run. Got %d", hl)
	}

}

func TestUpdateReleaseNoChanges(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)

	req := &services.UpdateReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
		Chart:        rel.GetChart(),
	}

	_, err := rs.UpdateRelease(c, req)
	if err != nil {
		t.Fatalf("Failed updated: %s", err)
	}
}

func compareStoredAndReturnedRelease(t *testing.T, rs ReleaseServer, res services.UpdateReleaseResponse) *release.Release {
	storedRelease, err := rs.env.Releases.Get(res.Release.Name, res.Release.Version)
	if err != nil {
		t.Fatalf("Expected release for %s (%v).", res.Release.Name, rs.env.Releases)
	}

	if !proto.Equal(storedRelease, res.Release) {
		t.Errorf("Stored release doesn't match returned Release")
	}

	return storedRelease
}
