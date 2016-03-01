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

package transient

import (
	"github.com/kubernetes/deployment-manager/cmd/manager/repository"
	"testing"
)

func TestRepositoryListEmpty(t *testing.T) {
	repository.TestRepositoryListEmpty(t, NewRepository())
}

func TestRepositoryGetFailsWithNonExistentDeployment(t *testing.T) {
	repository.TestRepositoryGetFailsWithNonExistentDeployment(t, NewRepository())
}

func TestRepositoryCreateDeploymentWorks(t *testing.T) {
	repository.TestRepositoryCreateDeploymentWorks(t, NewRepository())
}

func TestRepositoryMultipleManifestsWorks(t *testing.T) {
	repository.TestRepositoryMultipleManifestsWorks(t, NewRepository())
}

func TestRepositoryDeleteFailsWithNonExistentDeployment(t *testing.T) {
	repository.TestRepositoryDeleteFailsWithNonExistentDeployment(t, NewRepository())
}

func TestRepositoryDeleteWorksWithNoLatestManifest(t *testing.T) {
	repository.TestRepositoryDeleteWorksWithNoLatestManifest(t, NewRepository())
}

func TestRepositoryDeleteDeploymentWorksNoForget(t *testing.T) {
	repository.TestRepositoryDeleteDeploymentWorksNoForget(t, NewRepository())
}

func TestRepositoryDeleteDeploymentWorksForget(t *testing.T) {
	repository.TestRepositoryDeleteDeploymentWorksForget(t, NewRepository())
}

func TestRepositoryTypeInstances(t *testing.T) {
	repository.TestRepositoryTypeInstances(t, NewRepository())
}
