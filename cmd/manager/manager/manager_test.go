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
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/repo"
	"github.com/kubernetes/helm/pkg/util"

	"errors"
	"reflect"
	"strings"
	"testing"
)

var layout = common.Layout{
	Resources: []*common.LayoutResource{{Resource: common.Resource{Name: "test", Type: "test"}}},
}
var configuration = common.Configuration{
	Resources: []*common.Resource{{Name: "test", Type: "test"}},
}
var resourcesWithSuccessState = common.Configuration{
	Resources: []*common.Resource{{Name: "test", Type: "test", State: &common.ResourceState{Status: common.Created}}},
}
var resourcesWithFailureState = common.Configuration{
	Resources: []*common.Resource{{
		Name: "test",
		Type: "test",
		State: &common.ResourceState{
			Status: common.Failed,
			Errors: []string{"test induced error"},
		},
	}},
}
var template = common.Template{Name: "test", Content: util.ToYAMLOrError(&configuration)}

var expandedConfig = ExpandedConfiguration{
	Config: &configuration,
	Layout: &layout,
}

var deploymentName = "deployment"

var manifestName = "manifest-2"
var manifest = common.Manifest{Name: manifestName, ExpandedConfig: &configuration, Layout: &layout}
var manifestMap = map[string]*common.Manifest{manifest.Name: &manifest}

var deployment = common.Deployment{
	Name: "test",
}

var deploymentList = []common.Deployment{deployment, {Name: "test2"}}

var typeInstMap = map[string][]string{"test": {"test"}}

var errTest = errors.New("test error")

type expanderStub struct{}

func (expander *expanderStub) ExpandConfiguration(conf *common.Configuration) (*ExpandedConfiguration, error) {
	if reflect.DeepEqual(conf, &configuration) {
		return &expandedConfig, nil
	}

	return nil, errTest
}

type deployerStub struct {
	FailCreate         bool
	Created            []*common.Configuration
	FailDelete         bool
	Deleted            []*common.Configuration
	FailCreateResource bool
}

func (deployer *deployerStub) reset() {
	deployer.FailCreate = false
	deployer.Created = make([]*common.Configuration, 0)
	deployer.FailDelete = false
	deployer.Deleted = make([]*common.Configuration, 0)
	deployer.FailCreateResource = false
}

func newDeployerStub() *deployerStub {
	ret := &deployerStub{}
	return ret
}

func (deployer *deployerStub) GetConfiguration(cached *common.Configuration) (*common.Configuration, error) {
	return nil, nil
}

func (deployer *deployerStub) CreateConfiguration(configuration *common.Configuration) (*common.Configuration, error) {
	if deployer.FailCreate {
		return nil, errTest
	}
	if deployer.FailCreateResource {
		return &resourcesWithFailureState, errTest
	}

	deployer.Created = append(deployer.Created, configuration)
	return &resourcesWithSuccessState, nil
}

func (deployer *deployerStub) DeleteConfiguration(configuration *common.Configuration) (*common.Configuration, error) {
	if deployer.FailDelete {
		return nil, errTest
	}
	deployer.Deleted = append(deployer.Deleted, configuration)
	return nil, nil
}

func (deployer *deployerStub) PutConfiguration(configuration *common.Configuration) (*common.Configuration, error) {
	return nil, nil
}

type repositoryStub struct {
	FailListDeployments     bool
	Created                 []string
	ManifestAdd             map[string]*common.Manifest
	ManifestSet             map[string]*common.Manifest
	Deleted                 []string
	GetValid                []string
	ChartInstances          map[string][]string
	ChartInstancesCleared   bool
	GetChartInstancesCalled bool
	ListTypesCalled         bool
	DeploymentStates        []*common.DeploymentState
}

func (repository *repositoryStub) reset() {
	repository.FailListDeployments = false
	repository.Created = make([]string, 0)
	repository.ManifestAdd = make(map[string]*common.Manifest)
	repository.ManifestSet = make(map[string]*common.Manifest)
	repository.Deleted = make([]string, 0)
	repository.GetValid = make([]string, 0)
	repository.ChartInstances = make(map[string][]string)
	repository.ChartInstancesCleared = false
	repository.GetChartInstancesCalled = false
	repository.ListTypesCalled = false
	repository.DeploymentStates = []*common.DeploymentState{}
}

func newRepositoryStub() *repositoryStub {
	ret := &repositoryStub{}
	return ret
}

// Deployments.
func (repository *repositoryStub) ListDeployments() ([]common.Deployment, error) {
	if repository.FailListDeployments {
		return deploymentList, errTest
	}

	return deploymentList, nil
}

func (repository *repositoryStub) GetDeployment(d string) (*common.Deployment, error) {
	if d == deploymentName {
		return &deployment, nil
	}

	return nil, errTest
}

func (repository *repositoryStub) GetValidDeployment(d string) (*common.Deployment, error) {
	repository.GetValid = append(repository.GetValid, d)
	return &deployment, nil
}

func (repository *repositoryStub) CreateDeployment(d string) (*common.Deployment, error) {
	repository.Created = append(repository.Created, d)
	return &deployment, nil
}

func (repository *repositoryStub) DeleteDeployment(d string, forget bool) (*common.Deployment, error) {
	repository.Deleted = append(repository.Deleted, d)
	return &deployment, nil
}

func (repository *repositoryStub) SetDeploymentState(name string, state *common.DeploymentState) error {
	repository.DeploymentStates = append(repository.DeploymentStates, state)
	return nil
}

// Manifests.
func (repository *repositoryStub) AddManifest(manifest *common.Manifest) error {
	repository.ManifestAdd[manifest.Deployment] = manifest
	return nil
}

func (repository *repositoryStub) SetManifest(manifest *common.Manifest) error {
	repository.ManifestSet[manifest.Deployment] = manifest
	return nil
}

func (repository *repositoryStub) ListManifests(d string) (map[string]*common.Manifest, error) {
	if d == deploymentName {
		return manifestMap, nil
	}

	return nil, errTest
}

func (repository *repositoryStub) GetManifest(d string, m string) (*common.Manifest, error) {
	if d == deploymentName && m == manifestName {
		return &manifest, nil
	}

	return nil, errTest
}

func (repository *repositoryStub) GetLatestManifest(d string) (*common.Manifest, error) {
	if d == deploymentName {
		return repository.ManifestAdd[d], nil
	}

	return nil, errTest
}

// Types.
func (repository *repositoryStub) ListCharts() ([]string, error) {
	repository.ListTypesCalled = true
	return []string{}, nil
}

func (repository *repositoryStub) GetChartInstances(t string) ([]*common.ChartInstance, error) {
	repository.GetChartInstancesCalled = true
	return []*common.ChartInstance{}, nil
}

func (repository *repositoryStub) ClearChartInstancesForDeployment(d string) error {
	repository.ChartInstancesCleared = true
	return nil
}

func (repository *repositoryStub) AddChartInstances(is map[string][]*common.ChartInstance) error {
	for t, instances := range is {
		for _, instance := range instances {
			d := instance.Deployment
			repository.ChartInstances[d] = append(repository.ChartInstances[d], t)
		}
	}

	return nil
}

func (repository *repositoryStub) Close() {}

var testExpander = &expanderStub{}
var testRepository = newRepositoryStub()
var testDeployer = newDeployerStub()
var testRepoService = repo.NewInmemRepoService()
var testCredentialProvider = repo.NewInmemCredentialProvider()
var testProvider = repo.NewRepoProvider(nil, repo.NewGCSRepoProvider(testCredentialProvider), testCredentialProvider)
var testManager = NewManager(testExpander, testDeployer, testRepository, testProvider, testRepoService, testCredentialProvider)

func TestListDeployments(t *testing.T) {
	testRepository.reset()
	d, err := testManager.ListDeployments()
	if err != nil {
		t.Fatalf(err.Error())
	}

	if !reflect.DeepEqual(d, deploymentList) {
		t.Fatalf("invalid deployment list")
	}
}

func TestListDeploymentsFail(t *testing.T) {
	testRepository.reset()
	testRepository.FailListDeployments = true
	d, err := testManager.ListDeployments()
	if err != errTest {
		t.Fatalf(err.Error())
	}

	if d != nil {
		t.Fatalf("deployment list is not empty")
	}
}

func TestGetDeployment(t *testing.T) {
	testRepository.reset()
	d, err := testManager.GetDeployment(deploymentName)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if !reflect.DeepEqual(d, &deployment) {
		t.Fatalf("invalid deployment")
	}
}

func TestListManifests(t *testing.T) {
	testRepository.reset()
	m, err := testManager.ListManifests(deploymentName)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if !reflect.DeepEqual(m, manifestMap) {
		t.Fatalf("invalid manifest map")
	}
}

func TestGetManifest(t *testing.T) {
	testRepository.reset()
	m, err := testManager.GetManifest(deploymentName, manifestName)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if !reflect.DeepEqual(m, &manifest) {
		t.Fatalf("invalid manifest")
	}
}

func TestCreateDeployment(t *testing.T) {
	testRepository.reset()
	testDeployer.reset()
	d, err := testManager.CreateDeployment(&template)
	if !reflect.DeepEqual(d, &deployment) || err != nil {
		t.Fatalf("Expected a different set of response values from invoking CreateDeployment."+
			"Received: %v, %s. Expected: %#v, %s.", d, err, &deployment, "nil")
	}

	if testRepository.Created[0] != template.Name {
		t.Fatalf("Repository CreateDeployment was called with %s but expected %s.",
			testRepository.Created[0], template.Name)
	}

	if !strings.HasPrefix(testRepository.ManifestAdd[template.Name].Name, "manifest-") {
		t.Fatalf("Repository AddManifest was called with %s but expected manifest name"+
			"to begin with manifest-.", testRepository.ManifestAdd[template.Name].Name)
	}

	if !strings.HasPrefix(testRepository.ManifestSet[template.Name].Name, "manifest-") {
		t.Fatalf("Repository SetManifest was called with %s but expected manifest name"+
			"to begin with manifest-.", testRepository.ManifestSet[template.Name].Name)
	}

	if !reflect.DeepEqual(*testDeployer.Created[0], configuration) || err != nil {
		t.Fatalf("Deployer CreateConfiguration was called with %s but expected %s.",
			testDeployer.Created[0], configuration)
	}

	if testRepository.DeploymentStates[0].Status != common.DeployedStatus {
		t.Fatal("CreateDeployment success did not mark deployment as deployed")
	}

	if !testRepository.ChartInstancesCleared {
		t.Fatal("Repository did not clear type instances during creation")
	}

	if !reflect.DeepEqual(testRepository.ChartInstances, typeInstMap) {
		t.Fatalf("Unexpected type instances after CreateDeployment: %s", testRepository.ChartInstances)
	}
}

func TestCreateDeploymentCreationFailure(t *testing.T) {
	testRepository.reset()
	testDeployer.reset()
	testDeployer.FailCreate = true
	d, err := testManager.CreateDeployment(&template)

	if testRepository.Created[0] != template.Name {
		t.Fatalf("Repository CreateDeployment was called with %s but expected %s.",
			testRepository.Created[0], template.Name)
	}

	if len(testRepository.Deleted) != 0 {
		t.Fatalf("DeleteDeployment was called with %s but not expected",
			testRepository.Created[0])
	}

	if testRepository.DeploymentStates[0].Status != common.FailedStatus {
		t.Fatal("CreateDeployment failure did not mark deployment as failed")
	}

	if err != errTest || d != nil {
		t.Fatalf("Expected a different set of response values from invoking CreateDeployment."+
			"Received: %v, %s. Expected: %s, %s.", d, err, "nil", errTest)
	}

	if testRepository.ChartInstancesCleared {
		t.Fatal("Unexpected change to type instances during CreateDeployment failure.")
	}
}

func TestCreateDeploymentCreationResourceFailure(t *testing.T) {
	testRepository.reset()
	testDeployer.reset()
	testDeployer.FailCreateResource = true
	d, err := testManager.CreateDeployment(&template)

	if testRepository.Created[0] != template.Name {
		t.Fatalf("Repository CreateDeployment was called with %s but expected %s.",
			testRepository.Created[0], template.Name)
	}

	if len(testRepository.Deleted) != 0 {
		t.Fatalf("DeleteDeployment was called with %s but not expected",
			testRepository.Created[0])
	}

	if testRepository.DeploymentStates[0].Status != common.FailedStatus {
		t.Fatal("CreateDeployment failure did not mark deployment as failed")
	}

	if manifest, ok := testRepository.ManifestAdd[template.Name]; ok {
		if !strings.HasPrefix(manifest.Name, "manifest-") {
			t.Fatalf("Repository AddManifest was called with %s but expected manifest name"+
				"to begin with manifest-.", manifest.Name)
		}
	}

	if manifest, ok := testRepository.ManifestSet[template.Name]; ok {
		if !strings.HasPrefix(manifest.Name, "manifest-") {
			t.Fatalf("Repository AddManifest was called with %s but expected manifest name"+
				"to begin with manifest-.", manifest.Name)
		}
	}

	if err != nil || !reflect.DeepEqual(d, &deployment) {
		t.Fatalf("Expected a different set of response values from invoking CreateDeployment.\n"+
			"Received: %v, %v. Expected: %v, %v.", d, err, &deployment, "nil")
	}

	if !testRepository.ChartInstancesCleared {
		t.Fatal("Repository did not clear type instances during creation")
	}
}

func TestDeleteDeploymentForget(t *testing.T) {
	testRepository.reset()
	testDeployer.reset()
	d, err := testManager.CreateDeployment(&template)
	if !reflect.DeepEqual(d, &deployment) || err != nil {
		t.Fatalf("Expected a different set of response values from invoking CreateDeployment."+
			"Received: %v, %s. Expected: %#v, %s.", d, err, &deployment, "nil")
	}

	if testRepository.Created[0] != template.Name {
		t.Fatalf("Repository CreateDeployment was called with %s but expected %s.",
			testRepository.Created[0], template.Name)
	}

	if !strings.HasPrefix(testRepository.ManifestAdd[template.Name].Name, "manifest-") {
		t.Fatalf("Repository AddManifest was called with %s but expected manifest name"+
			"to begin with manifest-.", testRepository.ManifestAdd[template.Name].Name)
	}

	if !strings.HasPrefix(testRepository.ManifestSet[template.Name].Name, "manifest-") {
		t.Fatalf("Repository SetManifest was called with %s but expected manifest name"+
			"to begin with manifest-.", testRepository.ManifestSet[template.Name].Name)
	}

	if !reflect.DeepEqual(*testDeployer.Created[0], configuration) || err != nil {
		t.Fatalf("Deployer CreateConfiguration was called with %s but expected %s.",
			testDeployer.Created[0], configuration)
	}
	d, err = testManager.DeleteDeployment(deploymentName, true)
	if err != nil {
		t.Fatalf("DeleteDeployment failed with %v", err)
	}

	// Make sure the resources were deleted through deployer.
	if len(testDeployer.Deleted) > 0 {
		c := testDeployer.Deleted[0]
		if c != nil {
			if !reflect.DeepEqual(*c, configuration) || err != nil {
				t.Fatalf("Deployer DeleteConfiguration was called with %s but expected %s.",
					testDeployer.Created[0], configuration)
			}
		}
	}

	if !testRepository.ChartInstancesCleared {
		t.Fatal("Expected type instances to be cleared during DeleteDeployment.")
	}
}

func TestExpand(t *testing.T) {
	m, err := testManager.Expand(&template)
	if err != nil {
		t.Fatal("Failed to expand template into manifest.")
	}

	if !reflect.DeepEqual(*m.ExpandedConfig, configuration) {
		t.Fatalf("Expanded config not correct in output manifest: %v", *m)
	}

	if !reflect.DeepEqual(*m.Layout, layout) {
		t.Fatalf("Layout not correct in output manifest: %v", *m)
	}
}

func TestListTypes(t *testing.T) {
	testRepository.reset()

	testManager.ListCharts()

	if !testRepository.ListTypesCalled {
		t.Fatal("expected repository ListCharts() call.")
	}
}

func TestListInstances(t *testing.T) {
	testRepository.reset()

	testManager.ListChartInstances("all")

	if !testRepository.GetChartInstancesCalled {
		t.Fatal("expected repository GetChartInstances() call.")
	}
}

// TODO(jackgr): Implement TestListRepoCharts
func TestListRepoCharts(t *testing.T) {
	/*
		types, err := testManager.ListRepoCharts("", nil)
		if err != nil {
		    t.Fatalf("cannot list repository types: %s", err)
		}
	*/
}

// TODO(jackgr): Implement TestGetDownloadURLs
func TestGetDownloadURLs(t *testing.T) {
	/*
		    urls, err := testManager.GetDownloadURLs("", repo.Type{})
			if err != nil {
			    t.Fatalf("cannot list get download urls: %s", err)
			}
	*/
}
