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

// Package persistent implements a persistent deployment repository.
//
// This package is currently implemented using MondoDB, but there is no
// guarantee that it will continue to be implemented using MondoDB in the
// future.
package persistent

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/kubernetes/deployment-manager/common"
	"github.com/kubernetes/deployment-manager/manager/repository"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type pDeployment struct {
	ID string `bson:"_id"`
	common.Deployment
}

type pManifest struct {
	ID string `bson:"_id"`
	common.Manifest
}

type pInstance struct {
	ID string `bson:"_id"`
	common.TypeInstance
}

type pRepository struct {
	Session     *mgo.Session    // mongodb session
	Deployments *mgo.Collection // deployments collection
	Manifests   *mgo.Collection // manifests collection
	Instances   *mgo.Collection // instances collection
}

// Constants used to configure the MongoDB database.
const (
	DatabaseName              = "deployment_manager"
	DeploymentsCollectionName = "deployments_collection"
	ManifestsCollectionName   = "manifests_collection"
	InstancesCollectionName   = "instances_collection"
)

// NewRepository returns a new persistent repository. Its lifetime is decopuled
// from the lifetime of the current process. When the process dies, its contents
// will not be affected.
//
// The server argument provides connection information for the repository server.
// It is parsed as a URL, and the username, password, host and port, if provided,
// are used to create the connection string.
func NewRepository(server string) (repository.Repository, error) {
	travis := os.Getenv("TRAVIS")
	if travis == "true" {
		err := fmt.Errorf("cannot use MongoDB in Travis CI due to gopkg.in/mgo.v2 issue #218")
		log.Println(err.Error())
		return nil, err
	}

	u, err := url.Parse(server)
	if err != nil {
		err2 := fmt.Errorf("cannot parse url '%s': %s\n", server, err)
		log.Println(err2.Error())
		return nil, err2
	}

	u2 := &url.URL{Scheme: "mongodb", User: u.User, Host: u.Host}
	server = u2.String()

	session, err := mgo.Dial(server)
	if err != nil {
		err2 := fmt.Errorf("cannot connect to MongoDB at %s: %s\n", server, err)
		log.Println(err2.Error())
		return nil, err2
	}

	session.SetMode(mgo.Strong, false)
	session.SetSafe(&mgo.Safe{WMode: "majority"})
	database := session.DB(DatabaseName)
	deployments, err := createCollection(database, DeploymentsCollectionName, nil)
	if err != nil {
		return nil, err
	}

	manifests, err := createCollection(database, ManifestsCollectionName,
		[][]string{{"manifest.deployment"}})
	if err != nil {
		return nil, err
	}

	instances, err := createCollection(database, InstancesCollectionName,
		[][]string{{"typeinstance.type"}, {"typeinstance.deployment"}})
	if err != nil {
		return nil, err
	}

	pr := &pRepository{
		Session:     session,
		Deployments: deployments,
		Manifests:   manifests,
		Instances:   instances,
	}

	return pr, nil
}

func createCollection(db *mgo.Database, cName string, keys [][]string) (*mgo.Collection, error) {
	c := db.C(cName)
	for _, key := range keys {
		if err := createIndex(c, key...); err != nil {
			return nil, err
		}
	}

	return c, nil
}

func createIndex(c *mgo.Collection, key ...string) error {
	if err := c.EnsureIndexKey(key...); err != nil {
		err2 := fmt.Errorf("cannot create index %v for collection %s: %s\n", key, c.Name, err)
		log.Println(err2.Error())
		return err2
	}

	return nil
}

// Reset returns the repository to its initial state.
func (r *pRepository) Reset() error {
	database := r.Session.DB(DatabaseName)
	if err := database.DropDatabase(); err != nil {
		return fmt.Errorf("cannot drop database %s", database.Name)
	}

	r.Close()
	return nil
}

// Close cleans up any resources used by the repository.
func (r *pRepository) Close() {
	r.Session.Close()
}

// ListDeployments returns of all of the deployments in the repository.
func (r *pRepository) ListDeployments() ([]common.Deployment, error) {
	var result []pDeployment
	if err := r.Deployments.Find(nil).All(&result); err != nil {
		return nil, fmt.Errorf("cannot list deployments: %s", err)
	}

	deployments := []common.Deployment{}
	for _, pd := range result {
		deployments = append(deployments, pd.Deployment)
	}

	return deployments, nil
}

// GetDeployment returns the deployment with the supplied name.
// If the deployment is not found, it returns an error.
func (r *pRepository) GetDeployment(name string) (*common.Deployment, error) {
	result := pDeployment{}
	if err := r.Deployments.FindId(name).One(&result); err != nil {
		return nil, fmt.Errorf("cannot get deployment %s: %s", name, err)
	}

	return &result.Deployment, nil
}

// GetValidDeployment returns the deployment with the supplied name.
// If the deployment is not found or marked as deleted, it returns an error.
func (r *pRepository) GetValidDeployment(name string) (*common.Deployment, error) {
	d, err := r.GetDeployment(name)
	if err != nil {
		return nil, err
	}

	if d.State.Status == common.DeletedStatus {
		return nil, fmt.Errorf("deployment %s is deleted", name)
	}

	return d, nil
}

// CreateDeployment creates a new deployment and stores it in the repository.
func (r *pRepository) CreateDeployment(name string) (*common.Deployment, error) {
	exists, _ := r.GetValidDeployment(name)
	if exists != nil {
		return nil, fmt.Errorf("deployment %s already exists", name)
	}

	d := common.NewDeployment(name)
	if err := r.insertDeployment(d); err != nil {
		return nil, err
	}

	log.Printf("created deployment: %v", d)
	return d, nil
}

// SetDeploymentStatus sets the DeploymentStatus of the deployment and updates ModifiedAt
func (r *pRepository) SetDeploymentState(name string, state *common.DeploymentState) error {
	d, err := r.GetValidDeployment(name)
	if err != nil {
		return err
	}

	d.State = state
	return r.updateDeployment(d)
}

func (r *pRepository) AddManifest(manifest *common.Manifest) error {
	deploymentName := manifest.Deployment
	d, err := r.GetValidDeployment(deploymentName)
	if err != nil {
		return err
	}

	count, err := r.Manifests.FindId(manifest.Name).Count()
	if err != nil {
		return fmt.Errorf("cannot search for manifest %s: %s", manifest.Name, err)
	}

	if count > 0 {
		return fmt.Errorf("manifest %s already exists", manifest.Name)
	}

	if err := r.insertManifest(manifest); err != nil {
		return err
	}

	d.LatestManifest = manifest.Name
	if err := r.updateDeployment(d); err != nil {
		return err
	}

	log.Printf("Added manifest %s to deployment: %s", manifest.Name, deploymentName)
	return nil
}

// DeleteDeployment deletes the deployment with the supplied name.
// If forget is true, then the deployment is removed from the repository.
// Otherwise, it is marked as deleted and retained.
func (r *pRepository) DeleteDeployment(name string, forget bool) (*common.Deployment, error) {
	d, err := r.GetValidDeployment(name)
	if err != nil {
		return nil, err
	}

	if !forget {
		d.DeletedAt = time.Now()
		d.State = &common.DeploymentState{Status: common.DeletedStatus}
		if err := r.updateDeployment(d); err != nil {
			return nil, err
		}
	} else {
		d.LatestManifest = ""
		if err := r.removeManifestsForDeployment(d); err != nil {
			return nil, err
		}

		if err := r.removeDeployment(d); err != nil {
			return nil, err
		}
	}

	log.Printf("deleted deployment: %v", d)
	return d, nil
}

func (r *pRepository) insertDeployment(d *common.Deployment) error {
	if d != nil && d.Name != "" {
		wrapper := pDeployment{ID: d.Name, Deployment: *d}
		if err := r.Deployments.Insert(&wrapper); err != nil {
			return fmt.Errorf("cannot insert deployment %v: %s", wrapper, err)
		}
	}

	return nil
}

func (r *pRepository) removeDeployment(d *common.Deployment) error {
	if d != nil && d.Name != "" {
		if err := r.Deployments.RemoveId(d.Name); err != nil {
			return fmt.Errorf("cannot remove deployment %s: %s", d.Name, err)
		}
	}

	return nil
}

func (r *pRepository) updateDeployment(d *common.Deployment) error {
	if d != nil && d.Name != "" {
		if d.State.Status != common.DeletedStatus {
			d.ModifiedAt = time.Now()
		}

		wrapper := pDeployment{ID: d.Name, Deployment: *d}
		if err := r.Deployments.UpdateId(d.Name, &wrapper); err != nil {
			return fmt.Errorf("cannot update deployment %v: %s", wrapper, err)
		}
	}

	return nil
}

func (r *pRepository) ListManifests(deploymentName string) (map[string]*common.Manifest, error) {
	_, err := r.GetValidDeployment(deploymentName)
	if err != nil {
		return nil, err
	}

	return r.listManifestsForDeployment(deploymentName)
}

func (r *pRepository) GetManifest(deploymentName string, manifestName string) (*common.Manifest, error) {
	_, err := r.GetValidDeployment(deploymentName)
	if err != nil {
		return nil, err
	}

	return r.getManifestForDeployment(deploymentName, manifestName)
}

// GetLatestManifest returns the latest manifest for a given deployment,
// which by definition is the manifest with the largest time stamp.
func (r *pRepository) GetLatestManifest(deploymentName string) (*common.Manifest, error) {
	d, err := r.GetValidDeployment(deploymentName)
	if err != nil {
		return nil, err
	}

	if d.LatestManifest == "" {
		return nil, nil
	}

	return r.getManifestForDeployment(deploymentName, d.LatestManifest)
}

// SetManifest sets an existing manifest in the repository to provided manifest.
func (r *pRepository) SetManifest(manifest *common.Manifest) error {
	_, err := r.GetManifest(manifest.Deployment, manifest.Name)
	if err != nil {
		return err
	}

	return r.updateManifest(manifest)
}

func (r *pRepository) updateManifest(m *common.Manifest) error {
	if m != nil && m.Name != "" {
		wrapper := pManifest{ID: m.Name, Manifest: *m}
		if err := r.Manifests.UpdateId(m.Name, &wrapper); err != nil {
			return fmt.Errorf("cannot update manifest %v: %s", wrapper, err)
		}
	}

	return nil
}

func (r *pRepository) listManifestsForDeployment(deploymentName string) (map[string]*common.Manifest, error) {
	query := bson.M{"manifest.deployment": deploymentName}
	var result []pManifest
	if err := r.Manifests.Find(query).All(&result); err != nil {
		return nil, fmt.Errorf("cannot list manifests for deployment %s: %s", deploymentName, err)
	}

	l := make(map[string]*common.Manifest, 0)
	for _, pm := range result {
		l[pm.Name] = &pm.Manifest
	}

	return l, nil
}

func (r *pRepository) getManifestForDeployment(deploymentName string, manifestName string) (*common.Manifest, error) {
	result := pManifest{}
	if err := r.Manifests.FindId(manifestName).One(&result); err != nil {
		return nil, fmt.Errorf("cannot get manifest %s: %s", manifestName, err)
	}

	if result.Deployment != deploymentName {
		return nil, fmt.Errorf("manifest %s not found in deployment %s", manifestName, deploymentName)
	}

	return &result.Manifest, nil
}

func (r *pRepository) insertManifest(m *common.Manifest) error {
	if m != nil && m.Name != "" {
		wrapper := pManifest{ID: m.Name, Manifest: *m}
		if err := r.Manifests.Insert(&wrapper); err != nil {
			return fmt.Errorf("cannot insert manifest %v: %s", wrapper, err)
		}
	}

	return nil
}

func (r *pRepository) removeManifestsForDeployment(d *common.Deployment) error {
	if d != nil && d.Name != "" {
		query := bson.M{"manifest.deployment": d.Name}
		if _, err := r.Manifests.RemoveAll(query); err != nil {
			return fmt.Errorf("cannot remove all manifests for deployment %s: %s", d.Name, err)
		}
	}

	return nil
}

// ListTypes returns all types known from existing instances.
func (r *pRepository) ListTypes() ([]string, error) {
	var result []string
	if err := r.Instances.Find(nil).Distinct("typeinstance.type", &result); err != nil {
		return nil, fmt.Errorf("cannot list type instances: %s", err)
	}

	return result, nil
}

// GetTypeInstances returns all instances of a given type. If typeName is empty
// or equal to "all", returns all instances of all types.
func (r *pRepository) GetTypeInstances(typeName string) ([]*common.TypeInstance, error) {
	query := bson.M{"typeinstance.type": typeName}
	if typeName == "" || typeName == "all" {
		query = nil
	}

	var result []pInstance
	if err := r.Instances.Find(query).All(&result); err != nil {
		return nil, fmt.Errorf("cannot get instances of type %s: %s", typeName, err)
	}

	instances := []*common.TypeInstance{}
	for _, pi := range result {
		instances = append(instances, &pi.TypeInstance)
	}

	return instances, nil
}

// ClearTypeInstancesForDeployment deletes all type instances associated with the given
// deployment from the repository.
func (r *pRepository) ClearTypeInstancesForDeployment(deploymentName string) error {
	if deploymentName != "" {
		query := bson.M{"typeinstance.deployment": deploymentName}
		if _, err := r.Instances.RemoveAll(query); err != nil {
			return fmt.Errorf("cannot clear type instances for deployment %s: %s", deploymentName, err)
		}
	}

	return nil
}

// AddTypeInstances adds the supplied type instances to the repository.
func (r *pRepository) AddTypeInstances(instances map[string][]*common.TypeInstance) error {
	for _, is := range instances {
		for _, i := range is {
			key := fmt.Sprintf("%s.%s.%s", i.Deployment, i.Type, i.Name)
			wrapper := pInstance{ID: key, TypeInstance: *i}
			if err := r.Instances.Insert(&wrapper); err != nil {
				return fmt.Errorf("cannot insert type instance %v: %s", wrapper, err)
			}
		}
	}

	return nil
}
