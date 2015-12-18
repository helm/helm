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

func TestRepositoryListEmpty(t *testing.T) {
	r := NewMapBasedRepository()
	d, err := r.ListDeployments()
	if err != nil {
		t.Fatal("List Deployments failed")
	}
	if len(d) != 0 {
		t.Fatal("Returned non zero list")
	}
}

func TestRepositoryGetFailsWithNonExistentDeployment(t *testing.T) {
	r := NewMapBasedRepository()
	_, err := r.GetDeployment("nothere")
	if err == nil {
		t.Fatal("GetDeployment didn't fail with non-existent deployment")
	}
	if err.Error() != "deployment nothere not found" {
		t.Fatal("Error message doesn't match")
	}
}

func testCreateDeploymentWithManifests(t *testing.T, count int) {
	var deploymentName = "mydeployment"
	r := NewMapBasedRepository()

	d, err := r.CreateDeployment(deploymentName)
	if err != nil {
		t.Fatalf("CreateDeployment failed: %v", err)
	}

	l, err := r.ListDeployments()
	if err != nil {
		t.Fatalf("ListDeployments failed: %v", err)
	}
	if len(l) != 1 {
		t.Fatalf("List of deployments is not 1: %d", len(l))
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
		err := r.AddManifest(deploymentName, &manifest)
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

func TestRepositoryCreateDeploymentWorks(t *testing.T) {
	testCreateDeploymentWithManifests(t, 1)
}

func TestRepositoryMultipleManifestsWorks(t *testing.T) {
	testCreateDeploymentWithManifests(t, 7)
}

func TestRepositoryDeleteFailsWithNonExistentDeployment(t *testing.T) {
	var deploymentName = "mydeployment"
	r := NewMapBasedRepository()
	d, err := r.DeleteDeployment(deploymentName, false)
	if err == nil {
		t.Fatalf("DeleteDeployment didn't fail with non existent deployment")
	}
	if d != nil {
		t.Fatalf("DeleteDeployment returned non-nil for non existent deployment")
	}
}

func TestRepositoryDeleteWorksWithNoLatestManifest(t *testing.T) {
	var deploymentName = "mydeployment"
	r := NewMapBasedRepository()
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

func TestRepositoryDeleteDeploymentWorksNoForget(t *testing.T) {
	var deploymentName = "mydeployment"
	var manifestName = "manifest-0"
	r := NewMapBasedRepository()
	manifest := common.Manifest{Deployment: deploymentName, Name: manifestName}
	_, err := r.CreateDeployment(deploymentName)
	if err != nil {
		t.Fatalf("CreateDeployment failed: %v", err)
	}
	err = r.AddManifest(deploymentName, &manifest)
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

func TestRepositoryDeleteDeploymentWorksForget(t *testing.T) {
	var deploymentName = "mydeployment"
	var manifestName = "manifest-0"
	r := NewMapBasedRepository()
	manifest := common.Manifest{Deployment: deploymentName, Name: manifestName}
	_, err := r.CreateDeployment(deploymentName)
	if err != nil {
		t.Fatalf("CreateDeployment failed: %v", err)
	}
	err = r.AddManifest(deploymentName, &manifest)
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

func TestRepositoryTypeInstances(t *testing.T) {
	r := NewMapBasedRepository()

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

	if instances := r.GetTypeInstances("noinstances"); len(instances) != 0 {
		t.Fatalf("expected no instances: %v", instances)
	}

	if types := r.ListTypes(); len(types) != 0 {
		t.Fatalf("expected no types: %v", types)
	}

	r.SetTypeInstances("d1", d1Map)
	r.SetTypeInstances("d2", d2Map)
	r.SetTypeInstances("d3", d3Map)

	if instances := r.GetTypeInstances("unknowntype"); len(instances) != 0 {
		t.Fatalf("expected no instances: %v", instances)
	}

	if instances := r.GetTypeInstances("t1"); len(instances) != 1 {
		t.Fatalf("expected one instance: %v", instances)
	}

	if instances := r.GetTypeInstances("t2"); len(instances) != 2 {
		t.Fatalf("expected two instances: %v", instances)
	}

	if instances := r.GetTypeInstances("all"); len(instances) != 3 {
		t.Fatalf("expected three total instances: %v", instances)
	}

	if types := r.ListTypes(); len(types) != 2 {
		t.Fatalf("expected two total types: %v", types)
	}

	r.ClearTypeInstances("d1")
	if instances := r.GetTypeInstances("t1"); len(instances) != 0 {
		t.Fatalf("expected no instances after clear: %v", instances)
	}

	if types := r.ListTypes(); len(types) != 1 {
		t.Fatalf("expected one total type: %v", types)
	}
}

// TODO(vaikas): Add more tests
