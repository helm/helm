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

package manager

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

var template = Template{Name: "test", Content: "test"}

var layout = Layout{
	Resources: []*LayoutResource{&LayoutResource{Resource: Resource{Name: "test", Type: "test"}}},
}
var configuration = Configuration{
	Resources: []*Resource{&Resource{Name: "test", Type: "test"}},
}
var resourcesWithSuccessState = Configuration{
	Resources: []*Resource{&Resource{Name: "test", Type: "test", State: &ResourceState{Status: Created}}},
}
var resourcesWithFailureState = Configuration{
	Resources: []*Resource{&Resource{
		Name: "test",
		Type: "test",
		State: &ResourceState{
			Status: Failed,
			Errors:[]string{"test induced error",
			},
		},
	}},
}
var expandedConfig = ExpandedTemplate{
	Config: &configuration,
	Layout: &layout,
}

var deploymentName = "deployment"
var deploymentNoManifestName = "deploymentNoManifest"

var manifestName = "manifest-2"
var manifest = Manifest{Name: manifestName, ExpandedConfig: &configuration, Layout: &layout}
var manifestMap = map[string]*Manifest{manifest.Name: &manifest}

var deploymentNoManifest = Deployment{
	Name: "test",
}
var deployment = Deployment{
	Name:      "test",
	Manifests: manifestMap,
}
var deploymentWithConfiguration = Deployment{
	Name:      "test",
	Manifests: manifestMap,
	Current:   &configuration,
}
var deploymentList = []Deployment{deployment, {Name: "test2"}}

var typeInstMap = map[string][]string{"test": []string{"test"}}

var errTest = errors.New("test")

type expanderStub struct{}

func (expander *expanderStub) ExpandTemplate(t Template) (*ExpandedTemplate, error) {
	if reflect.DeepEqual(t, template) {
		return &expandedConfig, nil
	}

	return nil, errTest
}

type deployerStub struct {
	FailCreate bool
	Created    []*Configuration
	FailDelete bool
	Deleted    []*Configuration
	FailCreateResource bool
}

func (deployer *deployerStub) reset() {
	deployer.FailCreate = false
	deployer.Created = make([]*Configuration, 0)
	deployer.FailDelete = false
	deployer.Deleted = make([]*Configuration, 0)
	deployer.FailCreateResource = false
}

func newDeployerStub() *deployerStub {
	ret := &deployerStub{}
	return ret
}

func (deployer *deployerStub) GetConfiguration(cached *Configuration) (*Configuration, error) {
	return nil, nil
}

func (deployer *deployerStub) CreateConfiguration(configuration *Configuration) (*Configuration, error) {
	if deployer.FailCreate {
		return nil, errTest
	}
	if deployer.FailCreateResource {
		return &resourcesWithFailureState, errTest
	}

	deployer.Created = append(deployer.Created, configuration)
	return &resourcesWithSuccessState, nil
}

func (deployer *deployerStub) DeleteConfiguration(configuration *Configuration) (*Configuration, error) {
	if deployer.FailDelete {
		return nil, errTest
	}
	deployer.Deleted = append(deployer.Deleted, configuration)
	return nil, nil
}

func (deployer *deployerStub) PutConfiguration(configuration *Configuration) (*Configuration, error) {
	return nil, nil
}

type repositoryStub struct {
	FailListDeployments    bool
	Created                []string
	ManifestAdd            map[string]*Manifest
	Deleted                []string
	GetValid               []string
	TypeInstances          map[string][]string
	TypeInstancesCleared   bool
	GetTypeInstancesCalled bool
	ListTypesCalled        bool
	DeploymentStatuses     []DeploymentStatus
}

func (repository *repositoryStub) reset() {
	repository.FailListDeployments = false
	repository.Created = make([]string, 0)
	repository.ManifestAdd = make(map[string]*Manifest)
	repository.Deleted = make([]string, 0)
	repository.GetValid = make([]string, 0)
	repository.TypeInstances = make(map[string][]string)
	repository.TypeInstancesCleared = false
	repository.GetTypeInstancesCalled = false
	repository.ListTypesCalled = false
	repository.DeploymentStatuses = make([]DeploymentStatus, 0)
}

func newRepositoryStub() *repositoryStub {
	ret := &repositoryStub{}
	return ret
}

func (repository *repositoryStub) ListDeployments() ([]Deployment, error) {
	if repository.FailListDeployments {
		return deploymentList, errTest
	}
	return deploymentList, nil
}

func (repository *repositoryStub) GetDeployment(d string) (*Deployment, error) {
	if d == deploymentName {
		return &deployment, nil
	}

	if d == deploymentNoManifestName {
		return &deploymentNoManifest, nil
	}

	return nil, errTest
}

func (repository *repositoryStub) GetValidDeployment(d string) (*Deployment, error) {
	repository.GetValid = append(repository.GetValid, d)
	return &deploymentWithConfiguration, nil
}

func (repository *repositoryStub) SetDeploymentStatus(name string, status DeploymentStatus) error {
	repository.DeploymentStatuses = append(repository.DeploymentStatuses, status)
	return nil
}

func (repository *repositoryStub) CreateDeployment(d string) (*Deployment, error) {
	repository.Created = append(repository.Created, d)
	return &deploymentWithConfiguration, nil
}

func (repository *repositoryStub) DeleteDeployment(d string, forget bool) (*Deployment, error) {
	repository.Deleted = append(repository.Deleted, d)
	return &deploymentWithConfiguration, nil
}

func (repository *repositoryStub) AddManifest(d string, manifest *Manifest) error {
	repository.ManifestAdd[d] = manifest
	return nil
}

func (repository *repositoryStub) ListManifests(d string) (map[string]*Manifest, error) {
	if d == deploymentName {
		return manifestMap, nil
	}

	return nil, errTest
}

func (repository *repositoryStub) GetManifest(d string, m string) (*Manifest, error) {
	if d == deploymentName && m == manifestName {
		return &manifest, nil
	}

	return nil, errTest
}

func (r *repositoryStub) ListTypes() []string {
	r.ListTypesCalled = true
	return []string{}
}

func (r *repositoryStub) GetTypeInstances(t string) []*TypeInstance {
	r.GetTypeInstancesCalled = true
	return []*TypeInstance{}
}

func (r *repositoryStub) ClearTypeInstances(d string) {
	r.TypeInstancesCleared = true
}

func (r *repositoryStub) SetTypeInstances(d string, is map[string][]*TypeInstance) {
	for k, _ := range is {
		r.TypeInstances[d] = append(r.TypeInstances[d], k)
	}
}

var testExpander = &expanderStub{}
var testRepository = newRepositoryStub()
var testDeployer = newDeployerStub()
var testManager = NewManager(testExpander, testDeployer, testRepository)

func TestListDeployments(t *testing.T) {
	testRepository.reset()
	d, err := testManager.ListDeployments()
	if !reflect.DeepEqual(d, deploymentList) || err != nil {
		t.FailNow()
	}
}

func TestListDeploymentsFail(t *testing.T) {
	testRepository.reset()
	testRepository.FailListDeployments = true
	d, err := testManager.ListDeployments()
	if d != nil || err != errTest {
		t.FailNow()
	}
}

func TestGetDeployment(t *testing.T) {
	testRepository.reset()
	d, err := testManager.GetDeployment(deploymentName)
	if !reflect.DeepEqual(d, &deploymentWithConfiguration) || err != nil {
		t.FailNow()
	}
}

func TestGetDeploymentNoManifest(t *testing.T) {
	testRepository.reset()
	d, err := testManager.GetDeployment(deploymentNoManifestName)
	if !reflect.DeepEqual(d, &deploymentNoManifest) || err != nil {
		t.FailNow()
	}
}

func TestListManifests(t *testing.T) {
	testRepository.reset()
	m, err := testManager.ListManifests(deploymentName)
	if !reflect.DeepEqual(m, manifestMap) || err != nil {
		t.FailNow()
	}
}

func TestGetManifest(t *testing.T) {
	testRepository.reset()
	m, err := testManager.GetManifest(deploymentName, manifestName)
	if !reflect.DeepEqual(m, &manifest) || err != nil {
		t.FailNow()
	}
}

func TestCreateDeployment(t *testing.T) {
	testRepository.reset()
	testDeployer.reset()
	d, err := testManager.CreateDeployment(&template)
	if !reflect.DeepEqual(d, &deploymentWithConfiguration) || err != nil {
		t.Errorf("Expected a different set of response values from invoking CreateDeployment."+
			"Received: %s, %s. Expected: %s, %s.", d, err, &deploymentWithConfiguration, "nil")
	}

	if testRepository.Created[0] != template.Name {
		t.Errorf("Repository CreateDeployment was called with %s but expected %s.",
			testRepository.Created[0], template.Name)
	}

	if !strings.HasPrefix(testRepository.ManifestAdd[template.Name].Name, "manifest-") {
		t.Errorf("Repository AddManifest was called with %s but expected manifest name"+
			"to begin with manifest-.", testRepository.ManifestAdd[template.Name].Name)
	}

	if !reflect.DeepEqual(*testDeployer.Created[0], configuration) || err != nil {
		t.Errorf("Deployer CreateConfiguration was called with %s but expected %s.",
			testDeployer.Created[0], configuration)
	}

	if testRepository.DeploymentStatuses[0] != DeployedStatus {
		t.Error("CreateDeployment success did not mark deployment as deployed")
	}

	if !testRepository.TypeInstancesCleared {
		t.Error("Repository did not clear type instances during creation")
	}

	if !reflect.DeepEqual(testRepository.TypeInstances, typeInstMap) {
		t.Errorf("Unexpected type instances after CreateDeployment: %s", testRepository.TypeInstances)
	}
}

func TestCreateDeploymentCreationFailure(t *testing.T) {
	testRepository.reset()
	testDeployer.reset()
	testDeployer.FailCreate = true
	d, err := testManager.CreateDeployment(&template)

	if testRepository.Created[0] != template.Name {
		t.Errorf("Repository CreateDeployment was called with %s but expected %s.",
			testRepository.Created[0], template.Name)
	}

	if len(testRepository.Deleted) != 0 {
		t.Errorf("DeleteDeployment was called with %s but not expected",
			testRepository.Created[0])
	}

	if testRepository.DeploymentStatuses[0] != FailedStatus {
		t.Error("CreateDeployment failure did not mark deployment as failed")
	}

	if err != errTest || d != nil {
		t.Errorf("Expected a different set of response values from invoking CreateDeployment."+
			"Received: %s, %s. Expected: %s, %s.", d, err, "nil", errTest)
	}

	if testRepository.TypeInstancesCleared {
		t.Error("Unexpected change to type instances during CreateDeployment failure.")
	}
}

func TestCreateDeploymentCreationResourceFailure(t *testing.T) {
	testRepository.reset()
	testDeployer.reset()
	testDeployer.FailCreateResource = true
	d, err := testManager.CreateDeployment(&template)

	if testRepository.Created[0] != template.Name {
		t.Errorf("Repository CreateDeployment was called with %s but expected %s.",
			testRepository.Created[0], template.Name)
	}

	if len(testRepository.Deleted) != 0 {
		t.Errorf("DeleteDeployment was called with %s but not expected",
			testRepository.Created[0])
	}

	if testRepository.DeploymentStatuses[0] != FailedStatus {
		t.Error("CreateDeployment failure did not mark deployment as failed")
	}

	if !strings.HasPrefix(testRepository.ManifestAdd[template.Name].Name, "manifest-") {
		t.Errorf("Repository AddManifest was called with %s but expected manifest name"+
			"to begin with manifest-.", testRepository.ManifestAdd[template.Name].Name)
	}

//	if err != errTest || d != nil {
	if d == nil {
		t.Errorf("Expected a different set of response values from invoking CreateDeployment."+
			"Received: %s, %s. Expected: %s, %s.", d, err, "nil", errTest)
	}

	if !testRepository.TypeInstancesCleared {
		t.Error("Repository did not clear type instances during creation")
	}
}

func TestDeleteDeploymentForget(t *testing.T) {
	testRepository.reset()
	testDeployer.reset()
	d, err := testManager.CreateDeployment(&template)
	if !reflect.DeepEqual(d, &deploymentWithConfiguration) || err != nil {
		t.Errorf("Expected a different set of response values from invoking CreateDeployment."+
			"Received: %s, %s. Expected: %s, %s.", d, err, &deploymentWithConfiguration, "nil")
	}

	if testRepository.Created[0] != template.Name {
		t.Errorf("Repository CreateDeployment was called with %s but expected %s.",
			testRepository.Created[0], template.Name)
	}

	if !strings.HasPrefix(testRepository.ManifestAdd[template.Name].Name, "manifest-") {
		t.Errorf("Repository AddManifest was called with %s but expected manifest name"+
			"to begin with manifest-.", testRepository.ManifestAdd[template.Name].Name)
	}

	if !reflect.DeepEqual(*testDeployer.Created[0], configuration) || err != nil {
		t.Errorf("Deployer CreateConfiguration was called with %s but expected %s.",
			testDeployer.Created[0], configuration)
	}
	oldManifestName := testRepository.ManifestAdd[template.Name].Name
	d, err = testManager.DeleteDeployment("test", true)
	if err != nil {
		t.Errorf("DeleteDeployment failed with %v", err)
	}
	if testRepository.ManifestAdd[template.Name].Name == oldManifestName {
		t.Errorf("New manifest was not created, is still: %s", oldManifestName)
	}
	if testRepository.ManifestAdd[template.Name].InputConfig != nil {
		t.Errorf("New manifest has non-nil config, is still: %v", testRepository.ManifestAdd[template.Name].InputConfig)
	}
	// Make sure the resources were deleted through deployer.
	if !reflect.DeepEqual(*testDeployer.Deleted[0], configuration) || err != nil {
		t.Errorf("Deployer DeleteConfiguration was called with %s but expected %s.",
			testDeployer.Created[0], configuration)
	}

	if !testRepository.TypeInstancesCleared {
		t.Error("Expected type instances to be cleared during DeleteDeployment.")
	}
}

func TestExpand(t *testing.T) {
	m, err := testManager.Expand(&template)
	if err != nil {
		t.Error("Failed to expand template into manifest.")
	}

	if m.Name != "" {
		t.Errorf("Name was not empty: %v", *m)
	}

	if m.Deployment != "" {
		t.Errorf("Deployment was not empty: %v", *m)
	}

	if m.InputConfig != nil {
		t.Errorf("Input config not nil: %v", *m)
	}

	if !reflect.DeepEqual(*m.ExpandedConfig, configuration) {
		t.Errorf("Expanded config not correct in output manifest: %v", *m)
	}

	if !reflect.DeepEqual(*m.Layout, layout) {
		t.Errorf("Layout not correct in output manifest: %v", *m)
	}
}

func TestListTypes(t *testing.T) {
	testRepository.reset()

	testManager.ListTypes()

	if !testRepository.ListTypesCalled {
		t.Error("expected repository ListTypes() call.")
	}
}

func TestListInstances(t *testing.T) {
	testRepository.reset()

	testManager.ListInstances("all")

	if !testRepository.GetTypeInstancesCalled {
		t.Error("expected repository GetTypeInstances() call.")
	}
}
