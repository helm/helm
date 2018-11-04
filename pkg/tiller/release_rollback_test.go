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

package tiller

import (
	"strings"
	"testing"

	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
)

func TestRollbackRelease(t *testing.T) {
	rs := rsFixture(t)
	rel := releaseStub()
	rs.Releases.Create(rel)
	upgradedRel := upgradeReleaseVersion(rel)
	upgradedRel.Hooks = []*release.Hook{
		{
			Name:     "test-cm",
			Kind:     "ConfigMap",
			Path:     "test-cm",
			Manifest: manifestWithRollbackHooks,
			Events: []release.HookEvent{
				release.HookPreRollback,
				release.HookPostRollback,
			},
		},
	}

	upgradedRel.Manifest = "hello world"
	rs.Releases.Update(rel)
	rs.Releases.Create(upgradedRel)

	req := &hapi.RollbackReleaseRequest{
		Name: rel.Name,
	}
	res, err := rs.RollbackRelease(req)
	if err != nil {
		t.Fatalf("Failed rollback: %s", err)
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

	if res.Version != 3 {
		t.Errorf("Expected release version to be %v, got %v", 3, res.Version)
	}

	updated, err := rs.Releases.Get(res.Name, res.Version)
	if err != nil {
		t.Errorf("Expected release for %s (%v).", res.Name, rs.Releases)
	}

	if len(updated.Hooks) != 2 {
		t.Fatalf("Expected 2 hooks, got %d", len(updated.Hooks))
	}

	if updated.Hooks[0].Manifest != manifestWithHook {
		t.Errorf("Unexpected manifest: %v", updated.Hooks[0].Manifest)
	}

	anotherUpgradedRelease := upgradeReleaseVersion(upgradedRel)
	rs.Releases.Update(upgradedRel)
	rs.Releases.Create(anotherUpgradedRelease)

	res, err = rs.RollbackRelease(req)
	if err != nil {
		t.Fatalf("Failed rollback: %s", err)
	}

	updated, err = rs.Releases.Get(res.Name, res.Version)
	if err != nil {
		t.Errorf("Expected release for %s (%v).", res.Name, rs.Releases)
	}

	if len(updated.Hooks) != 1 {
		t.Fatalf("Expected 1 hook, got %d", len(updated.Hooks))
	}

	if updated.Hooks[0].Manifest != manifestWithRollbackHooks {
		t.Errorf("Unexpected manifest: %v", updated.Hooks[0].Manifest)
	}

	if res.Version != 4 {
		t.Errorf("Expected release version to be %v, got %v", 3, res.Version)
	}

	if updated.Hooks[0].Events[0] != release.HookPreRollback {
		t.Errorf("Expected event 0 to be pre rollback")
	}

	if updated.Hooks[0].Events[1] != release.HookPostRollback {
		t.Errorf("Expected event 1 to be post rollback")
	}

	if len(res.Manifest) == 0 {
		t.Errorf("No manifest returned: %v", res)
	}

	if len(updated.Manifest) == 0 {
		t.Errorf("Expected manifest in %v", res)
	}

	if !strings.Contains(updated.Manifest, "hello world") {
		t.Errorf("unexpected output: %s", rel.Manifest)
	}

	if res.Info.Description != "Rollback to 2" {
		t.Errorf("Expected rollback to 2, got %q", res.Info.Description)
	}
}

func TestRollbackWithReleaseVersion(t *testing.T) {
	rs := rsFixture(t)
	rel2 := releaseStub()
	rel2.Name = "other"
	rs.Releases.Create(rel2)
	rel := releaseStub()
	rs.Releases.Create(rel)
	v2 := upgradeReleaseVersion(rel)
	rs.Releases.Update(rel)
	rs.Releases.Create(v2)
	v3 := upgradeReleaseVersion(v2)
	// retain the original release as DEPLOYED while the update should fail
	v2.Info.Status = release.StatusDeployed
	v3.Info.Status = release.StatusFailed
	rs.Releases.Update(v2)
	rs.Releases.Create(v3)

	req := &hapi.RollbackReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
		Version:      1,
	}

	_, err := rs.RollbackRelease(req)
	if err != nil {
		t.Fatalf("Failed rollback: %s", err)
	}
	// check that v2 is now in a SUPERSEDED state
	oldRel, err := rs.Releases.Get(rel.Name, 2)
	if err != nil {
		t.Fatalf("Failed to retrieve v1: %s", err)
	}
	if oldRel.Info.Status != release.StatusSuperseded {
		t.Errorf("Expected v2 to be in a SUPERSEDED state, got %q", oldRel.Info.Status)
	}
	// make sure we didn't update some other deployments.
	otherRel, err := rs.Releases.Get(rel2.Name, 1)
	if err != nil {
		t.Fatalf("Failed to retrieve other v1: %s", err)
	}
	if otherRel.Info.Status != release.StatusDeployed {
		t.Errorf("Expected other deployed release to stay untouched, got %q", otherRel.Info.Status)
	}
}

func TestRollbackReleaseNoHooks(t *testing.T) {
	rs := rsFixture(t)
	rel := releaseStub()
	rel.Hooks = []*release.Hook{
		{
			Name:     "test-cm",
			Kind:     "ConfigMap",
			Path:     "test-cm",
			Manifest: manifestWithRollbackHooks,
			Events: []release.HookEvent{
				release.HookPreRollback,
				release.HookPostRollback,
			},
		},
	}
	rs.Releases.Create(rel)
	upgradedRel := upgradeReleaseVersion(rel)
	rs.Releases.Update(rel)
	rs.Releases.Create(upgradedRel)

	req := &hapi.RollbackReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
	}

	res, err := rs.RollbackRelease(req)
	if err != nil {
		t.Fatalf("Failed rollback: %s", err)
	}

	if hl := res.Hooks[0].LastRun; !hl.IsZero() {
		t.Errorf("Expected that no hooks were run. Got %s", hl)
	}
}

func TestRollbackReleaseFailure(t *testing.T) {
	rs := rsFixture(t)
	rel := releaseStub()
	rs.Releases.Create(rel)
	upgradedRel := upgradeReleaseVersion(rel)
	rs.Releases.Update(rel)
	rs.Releases.Create(upgradedRel)

	req := &hapi.RollbackReleaseRequest{
		Name:         rel.Name,
		DisableHooks: true,
	}

	rs.KubeClient = newUpdateFailingKubeClient()
	res, err := rs.RollbackRelease(req)
	if err == nil {
		t.Error("Expected failed rollback")
	}

	if targetStatus := res.Info.Status; targetStatus != release.StatusFailed {
		t.Errorf("Expected FAILED release. Got %v", targetStatus)
	}

	oldRelease, err := rs.Releases.Get(rel.Name, rel.Version)
	if err != nil {
		t.Errorf("Expected to be able to get previous release")
	}
	if oldStatus := oldRelease.Info.Status; oldStatus != release.StatusSuperseded {
		t.Errorf("Expected SUPERSEDED status on previous Release version. Got %v", oldStatus)
	}
}
