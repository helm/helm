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

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
)

func TestUninstallRelease(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rs.env.Releases.Create(releaseStub())

	req := &services.UninstallReleaseRequest{
		Name: "angry-panda",
	}

	res, err := rs.UninstallRelease(c, req)
	if err != nil {
		t.Fatalf("Failed uninstall: %s", err)
	}

	if res.Release.Name != "angry-panda" {
		t.Errorf("Expected angry-panda, got %q", res.Release.Name)
	}

	if res.Release.Info.Status.Code != release.Status_DELETED {
		t.Errorf("Expected status code to be DELETED, got %d", res.Release.Info.Status.Code)
	}

	if res.Release.Hooks[0].LastRun.Seconds == 0 {
		t.Error("Expected LastRun to be greater than zero.")
	}

	if res.Release.Info.Deleted.Seconds <= 0 {
		t.Errorf("Expected valid UNIX date, got %d", res.Release.Info.Deleted.Seconds)
	}

	if res.Release.Info.Description != "Deletion complete" {
		t.Errorf("Expected Deletion complete, got %q", res.Release.Info.Description)
	}
}

func TestUninstallPurgeRelease(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rel := releaseStub()
	rs.env.Releases.Create(rel)
	upgradedRel := upgradeReleaseVersion(rel)
	rs.env.Releases.Update(rel)
	rs.env.Releases.Create(upgradedRel)

	req := &services.UninstallReleaseRequest{
		Name:  "angry-panda",
		Purge: true,
	}

	res, err := rs.UninstallRelease(c, req)
	if err != nil {
		t.Fatalf("Failed uninstall: %s", err)
	}

	if res.Release.Name != "angry-panda" {
		t.Errorf("Expected angry-panda, got %q", res.Release.Name)
	}

	if res.Release.Info.Status.Code != release.Status_DELETED {
		t.Errorf("Expected status code to be DELETED, got %d", res.Release.Info.Status.Code)
	}

	if res.Release.Info.Deleted.Seconds <= 0 {
		t.Errorf("Expected valid UNIX date, got %d", res.Release.Info.Deleted.Seconds)
	}
	rels, err := rs.GetHistory(helm.NewContext(), &services.GetHistoryRequest{Name: "angry-panda"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rels.Releases) != 0 {
		t.Errorf("Expected no releases in storage, got %d", len(rels.Releases))
	}
}

func TestUninstallPurgeDeleteRelease(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rs.env.Releases.Create(releaseStub())

	req := &services.UninstallReleaseRequest{
		Name: "angry-panda",
	}

	_, err := rs.UninstallRelease(c, req)
	if err != nil {
		t.Fatalf("Failed uninstall: %s", err)
	}

	req2 := &services.UninstallReleaseRequest{
		Name:  "angry-panda",
		Purge: true,
	}

	_, err2 := rs.UninstallRelease(c, req2)
	if err2 != nil && err2.Error() != "'angry-panda' has no deployed releases" {
		t.Errorf("Failed uninstall: %s", err2)
	}
}

func TestUninstallReleaseWithKeepPolicy(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	name := "angry-bunny"
	rs.env.Releases.Create(releaseWithKeepStub(name))

	req := &services.UninstallReleaseRequest{
		Name: name,
	}

	res, err := rs.UninstallRelease(c, req)
	if err != nil {
		t.Fatalf("Failed uninstall: %s", err)
	}

	if res.Release.Name != name {
		t.Errorf("Expected angry-bunny, got %q", res.Release.Name)
	}

	if res.Release.Info.Status.Code != release.Status_DELETED {
		t.Errorf("Expected status code to be DELETED, got %d", res.Release.Info.Status.Code)
	}

	if res.Info == "" {
		t.Errorf("Expected response info to not be empty")
	} else {
		if !strings.Contains(res.Info, "[ConfigMap] test-cm-keep") {
			t.Errorf("unexpected output: %s", res.Info)
		}
	}
}

func TestUninstallReleaseNoHooks(t *testing.T) {
	c := helm.NewContext()
	rs := rsFixture()
	rs.env.Releases.Create(releaseStub())

	req := &services.UninstallReleaseRequest{
		Name:         "angry-panda",
		DisableHooks: true,
	}

	res, err := rs.UninstallRelease(c, req)
	if err != nil {
		t.Errorf("Failed uninstall: %s", err)
	}

	// The default value for a protobuf timestamp is nil.
	if res.Release.Hooks[0].LastRun != nil {
		t.Errorf("Expected LastRun to be zero, got %d.", res.Release.Hooks[0].LastRun.Seconds)
	}
}
