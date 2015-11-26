/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package repository

import (
	"github.com/kubernetes/deployment-manager/common"

	"fmt"
	"testing"
)

// TestRepositoryListEmpty checks that listing an empty repository works.
func TestRepositoryListEmpty(t *testing.T, r Repository) {
	d, err := r.ListDeployments()
	if err != nil {
		t.Fatal("List Deployments failed")
	}

	if len(d) != 0 {
		t.Fatal("Returned non zero list")
	}
}

// TestRepositoryGetFailsWithNonExistentDeployment checks that getting a non-existent deployment fails.
func TestRepositoryGetFailsWithNonExistentDeployment(t *testing.T, r Repository) {
	_, err := r.GetDeployment("nothere")
	if err == nil {
		t.Fatal("GetDeployment didn't fail with non-existent deployment")
	}
}

func testCreateDeploymentWithManifests(t *testing.T, r Repository, count int) {
	var deploymentName = "mydeployment"

	d, err := r.CreateDeployment(deploymentName)
	if err != nil {
		t.Fatalf("CreateDeployment failed: %v", err)
	}

	l, err := r.ListDeployments()
	if err != nil {
		t.Fatalf("ListDeployments failed: %v", err)
	}

	if len(l) != 1 {
		t.Fatalf("Number of deployments listed is not 1: %d", len(l))
	}

	dNew, err := r.GetDeployment(deploymentName)
	if err != nil {
		t.Fatalf("GetDeployment failed: %v", err)
	}

	if dNew.Name != d.Name {
		t.Fatalf("Deployment Names don't match, got: %v, expected %v", dNew, d)
	}

	mList, err := r.ListManifests(deploymentName)
	if err != nil {
		t.Fatalf("ListManifests failed: %v", err)
	}

	if len(mList) != 0 {
		t.Fatalf("Deployment has non-zero manifest count: %d", len(mList))
	}

	for i := 0; i < count; i++ {
		var manifestName = fmt.Sprintf("manifest-%d", i)
		manifest := common.Manifest{Deployment: deploymentName, Name: manifestName}
		err := r.AddManifest(&manifest)
		if err != nil {
			t.Fatalf("AddManifest failed: %v", err)
		}

		d, err = r.GetDeployment(deploymentName)
		if err != nil {
			t.Fatalf("GetDeployment failed: %v", err)
		}

		if d.LatestManifest != manifestName {
			t.Fatalf("AddManifest did not update LatestManifest: %#v", d)
		}

		mListNew, err := r.ListManifests(deploymentName)
		if err != nil {
			t.Fatalf("ListManifests failed: %v", err)
		}

		if len(mListNew) != i+1 {
			t.Fatalf("Deployment has unexpected manifest count: want %d, have %d", i+1, len(mListNew))
		}

		m, err := r.GetManifest(deploymentName, manifestName)
		if err != nil {
			t.Fatalf("GetManifest failed: %v", err)
		}

		if m.Name != manifestName {
			t.Fatalf("Unexpected manifest name: want %s, have %s", manifestName, m.Name)
		}
	}
}

// TestRepositoryCreateDeploymentWorks checks that creating a deployment works.
func TestRepositoryCreateDeploymentWorks(t *testing.T, r Repository) {
	testCreateDeploymentWithManifests(t, r, 1)
}

// TestRepositoryMultipleManifestsWorks checks that creating a deploymente with multiple manifests works.
func TestRepositoryMultipleManifestsWorks(t *testing.T, r Repository) {
	testCreateDeploymentWithManifests(t, r, 7)
}

// TestRepositoryDeleteFailsWithNonExistentDeployment checks that deleting a non-existent deployment fails.
func TestRepositoryDeleteFailsWithNonExistentDeployment(t *testing.T, r Repository) {
	var deploymentName = "mydeployment"
	d, err := r.DeleteDeployment(deploymentName, false)
	if err == nil {
		t.Fatalf("DeleteDeployment didn't fail with non existent deployment")
	}

	if d != nil {
		t.Fatalf("DeleteDeployment returned non-nil for non existent deployment")
	}
}

// TestRepositoryDeleteWorksWithNoLatestManifest checks that deleting a deployment with no latest manifest works.
func TestRepositoryDeleteWorksWithNoLatestManifest(t *testing.T, r Repository) {
	var deploymentName = "mydeployment"
	_, err := r.CreateDeployment(deploymentName)
	if err != nil {
		t.Fatalf("CreateDeployment failed: %v", err)
	}

	dDeleted, err := r.DeleteDeployment(deploymentName, false)
	if err != nil {
		t.Fatalf("DeleteDeployment failed: %v", err)
	}

	if dDeleted.State.Status != common.DeletedStatus {
		t.Fatalf("Deployment Status is not deleted")
	}

	if _, err := r.ListManifests(deploymentName); err == nil {
		t.Fatalf("Manifests are not deleted")
	}
}

// TestRepositoryDeleteDeploymentWorksNoForget checks that deleting a deployment without forgetting it works.
func TestRepositoryDeleteDeploymentWorksNoForget(t *testing.T, r Repository) {
	var deploymentName = "mydeployment"
	var manifestName = "manifest-0"
	manifest := common.Manifest{Deployment: deploymentName, Name: manifestName}
	_, err := r.CreateDeployment(deploymentName)
	if err != nil {
		t.Fatalf("CreateDeployment failed: %v", err)
	}

	err = r.AddManifest(&manifest)
	if err != nil {
		t.Fatalf("AddManifest failed: %v", err)
	}

	dDeleted, err := r.DeleteDeployment(deploymentName, false)
	if err != nil {
		t.Fatalf("DeleteDeployment failed: %v", err)
	}

	if dDeleted.State.Status != common.DeletedStatus {
		t.Fatalf("Deployment Status is not deleted")
	}
}

// TestRepositoryDeleteDeploymentWorksForget checks that deleting and forgetting a deployment works.
func TestRepositoryDeleteDeploymentWorksForget(t *testing.T, r Repository) {
	var deploymentName = "mydeployment"
	var manifestName = "manifest-0"
	manifest := common.Manifest{Deployment: deploymentName, Name: manifestName}
	_, err := r.CreateDeployment(deploymentName)
	if err != nil {
		t.Fatalf("CreateDeployment failed: %v", err)
	}

	err = r.AddManifest(&manifest)
	if err != nil {
		t.Fatalf("AddManifest failed: %v", err)
	}

	dDeleted, err := r.DeleteDeployment(deploymentName, true)
	if err != nil {
		t.Fatalf("DeleteDeployment failed: %v", err)
	}

	if dDeleted.State.Status != common.CreatedStatus {
		t.Fatalf("Deployment Status is not created")
	}
}

// TestRepositoryTypeInstances checks that type instances can be listed and retrieved successfully.
func TestRepositoryTypeInstances(t *testing.T, r Repository) {
	d1Map := map[string][]*common.TypeInstance{
		"t1": []*common.TypeInstance{
			&common.TypeInstance{
				Name:       "i1",
				Type:       "t1",
				Deployment: "d1",
				Manifest:   "m1",
				Path:       "p1",
			},
		},
	}

	d2Map := map[string][]*common.TypeInstance{
		"t2": []*common.TypeInstance{
			&common.TypeInstance{
				Name:       "i2",
				Type:       "t2",
				Deployment: "d2",
				Manifest:   "m2",
				Path:       "p2",
			},
		},
	}

	d3Map := map[string][]*common.TypeInstance{
		"t2": []*common.TypeInstance{
			&common.TypeInstance{
				Name:       "i3",
				Type:       "t2",
				Deployment: "d3",
				Manifest:   "m3",
				Path:       "p3",
			},
		},
	}

	instances, err := r.GetTypeInstances("noinstances")
	if err != nil {
		t.Fatal(err)
	}

	if len(instances) != 0 {
		t.Fatalf("expected no instances: %v", instances)
	}

	types, err := r.ListTypes()
	if err != nil {
		t.Fatal(err)
	}

	if len(types) != 0 {
		t.Fatalf("expected no types: %v", types)
	}

	r.AddTypeInstances(d1Map)
	r.AddTypeInstances(d2Map)
	r.AddTypeInstances(d3Map)

	instances, err = r.GetTypeInstances("unknowntype")
	if err != nil {
		t.Fatal(err)
	}

	if len(instances) != 0 {
		t.Fatalf("expected no instances: %v", instances)
	}

	instances, err = r.GetTypeInstances("t1")
	if err != nil {
		t.Fatal(err)
	}

	if len(instances) != 1 {
		t.Fatalf("expected one instance: %v", instances)
	}

	instances, err = r.GetTypeInstances("t2")
	if err != nil {
		t.Fatal(err)
	}

	if len(instances) != 2 {
		t.Fatalf("expected two instances: %v", instances)
	}

	instances, err = r.GetTypeInstances("all")
	if err != nil {
		t.Fatal(err)
	}

	if len(instances) != 3 {
		t.Fatalf("expected three total instances: %v", instances)
	}

	types, err = r.ListTypes()
	if err != nil {
		t.Fatal(err)
	}

	if len(types) != 2 {
		t.Fatalf("expected two total types: %v", types)
	}

	err = r.ClearTypeInstancesForDeployment("d1")
	if err != nil {
		t.Fatal(err)
	}

	instances, err = r.GetTypeInstances("t1")
	if err != nil {
		t.Fatal(err)
	}

	if len(instances) != 0 {
		t.Fatalf("expected no instances after clear: %v", instances)
	}

	types, err = r.ListTypes()
	if err != nil {
		t.Fatal(err)
	}

	if len(types) != 1 {
		t.Fatalf("expected one total type: %v", types)
	}
}
