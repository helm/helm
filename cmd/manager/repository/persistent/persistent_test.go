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

package persistent

import (
	"github.com/kubernetes/helm/cmd/manager/repository"

	"sync"
	"testing"
)

var tryRepository = true
var repositoryLock sync.RWMutex

func createRepository() repository.Repository {
	repositoryLock.Lock()
	defer repositoryLock.Unlock()

	if tryRepository {
		r, err := NewRepository("mongodb://localhost")
		if err == nil {
			return r
		}
	}

	tryRepository = false
	return nil
}

func resetRepository(t *testing.T, r repository.Repository) {
	if r != nil {
		if err := r.(*pRepository).Reset(); err != nil {
			t.Fatalf("cannot reset repository: %s\n", err)
		}
	}
}

func TestRepositoryListEmpty(t *testing.T) {
	if r := createRepository(); r != nil {
		defer resetRepository(t, r)
		repository.TestRepositoryListEmpty(t, r)
	}
}

func TestRepositoryGetFailsWithNonExistentDeployment(t *testing.T) {
	if r := createRepository(); r != nil {
		defer resetRepository(t, r)
		repository.TestRepositoryGetFailsWithNonExistentDeployment(t, r)
	}
}

func TestRepositoryCreateDeploymentWorks(t *testing.T) {
	if r := createRepository(); r != nil {
		defer resetRepository(t, r)
		repository.TestRepositoryCreateDeploymentWorks(t, r)
	}
}

func TestRepositoryMultipleManifestsWorks(t *testing.T) {
	if r := createRepository(); r != nil {
		defer resetRepository(t, r)
		repository.TestRepositoryMultipleManifestsWorks(t, r)
	}
}

func TestRepositoryDeleteFailsWithNonExistentDeployment(t *testing.T) {
	if r := createRepository(); r != nil {
		defer resetRepository(t, r)
		repository.TestRepositoryDeleteFailsWithNonExistentDeployment(t, r)
	}
}

func TestRepositoryDeleteWorksWithNoLatestManifest(t *testing.T) {
	if r := createRepository(); r != nil {
		defer resetRepository(t, r)
		repository.TestRepositoryDeleteWorksWithNoLatestManifest(t, r)
	}
}

func TestRepositoryDeleteDeploymentWorksNoForget(t *testing.T) {
	if r := createRepository(); r != nil {
		defer resetRepository(t, r)
		repository.TestRepositoryDeleteDeploymentWorksNoForget(t, r)
	}
}

func TestRepositoryDeleteDeploymentWorksForget(t *testing.T) {
	if r := createRepository(); r != nil {
		defer resetRepository(t, r)
		repository.TestRepositoryDeleteDeploymentWorksForget(t, r)
	}
}

func TestRepositoryChartInstances(t *testing.T) {
	if r := createRepository(); r != nil {
		defer resetRepository(t, r)
		repository.TestRepositoryChartInstances(t, r)
	}
}
