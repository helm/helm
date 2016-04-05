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

package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/kubernetes/helm/cmd/manager/manager"
	"github.com/kubernetes/helm/cmd/manager/repository"
	"github.com/kubernetes/helm/cmd/manager/repository/persistent"
	"github.com/kubernetes/helm/cmd/manager/repository/transient"
	"github.com/kubernetes/helm/cmd/manager/router"
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/httputil"
	"github.com/kubernetes/helm/pkg/repo"
	"github.com/kubernetes/helm/pkg/util"
)

func registerDeploymentRoutes(c *router.Context, h *router.Handler) {
	h.Add("GET /healthz", healthz)
	h.Add("GET /deployments", listDeploymentsHandlerFunc)
	h.Add("GET /deployments/*", getDeploymentHandlerFunc)
	h.Add("POST /deployments", createDeploymentHandlerFunc)
	h.Add("DELETE /deployments/*", deleteDeploymentHandlerFunc)
	h.Add("PUT /deployments/*", putDeploymentHandlerFunc)
	h.Add("GET /deployments/*/manifests", listManifestsHandlerFunc)
	h.Add("GET /deployments/*/manifests/*", getManifestHandlerFunc)
	h.Add("POST /expand", expandHandlerFunc)
	h.Add("GET /charts", listChartsHandlerFunc)
	h.Add("GET /charts/*/instances", listChartInstancesHandlerFunc)
	h.Add("GET /charts/*/repository", getRepoForChartHandlerFunc)
	h.Add("GET /charts/*/metadata", getMetadataForChartHandlerFunc)
	h.Add("GET /charts/*", getChartHandlerFunc)
	h.Add("POST /credentials/*", createCredentialHandlerFunc)
	h.Add("GET /credentials/*", getCredentialHandlerFunc)
}

func healthz(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	log.Println("manager: healthz checkpoint")
	// TODO: This should check the availability of the repository, and fail if it
	// cannot connect.
	fmt.Fprintln(w, "OK")
	return nil
}

func setupDependencies(c *router.Context) error {
	var credentialProvider repo.ICredentialProvider
	if c.Config.CredentialFile != "" {
		if c.Config.CredentialSecrets {
			return errors.New("Both credentialFile and credentialSecrets are set")
		}
		var err error
		credentialProvider, err = repo.NewFilebasedCredentialProvider(c.Config.CredentialFile)
		if err != nil {
			return fmt.Errorf("cannot create credential provider %s: %s", c.Config.CredentialFile, err)
		}
	} else if *credentialSecrets {
		credentialProvider = repo.NewSecretsCredentialProvider()
	} else {
		credentialProvider = repo.NewInmemCredentialProvider()
	}
	c.CredentialProvider = credentialProvider
	c.Manager = newManager(c)

	return nil
}

const expanderPort = "8080"
const deployerPort = "8080"

func newManager(c *router.Context) manager.Manager {
	cfg := c.Config
	service := repo.NewInmemRepoService()
	cp := c.CredentialProvider
	rp := repo.NewRepoProvider(service, repo.NewGCSRepoProvider(cp), cp)
	expander := manager.NewExpander(util.GetServiceURL(cfg.ExpanderURL, cfg.ExpanderName, expanderPort), rp)
	deployer := manager.NewDeployer(util.GetServiceURL(cfg.DeployerURL, cfg.DeployerName, deployerPort))
	address := strings.TrimPrefix(util.GetServiceURL(cfg.MongoAddress, cfg.MongoName, cfg.MongoPort), "http://")
	repository := createRepository(address)
	return manager.NewManager(expander, deployer, repository, rp, service, c.CredentialProvider)
}

func createRepository(address string) repository.Repository {
	r, err := persistent.NewRepository(address)
	if err != nil {
		r = transient.NewRepository()
	}

	return r
}

func listDeploymentsHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: list deployments"
	util.LogHandlerEntry(handler, r)
	l, err := c.Manager.ListDeployments()
	if err != nil {
		util.LogAndReturnError(handler, http.StatusInternalServerError, err, w)
		return nil
	}
	var names []string
	for _, d := range l {
		names = append(names, d.Name)
	}

	util.LogHandlerExitWithJSON(handler, w, names, http.StatusOK)
	return nil
}

func getDeploymentHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: get deployment"
	util.LogHandlerEntry(handler, r)
	name, err := pos(w, r, 2)
	if err != nil {
		return nil
	}

	d, err := c.Manager.GetDeployment(name)
	if err != nil {
		util.LogAndReturnError(handler, http.StatusBadRequest, err, w)
		return nil
	}

	util.LogHandlerExitWithJSON(handler, w, d, http.StatusOK)
	return nil
}

func createDeploymentHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: create deployment"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	depReq := getDeploymentRequest(w, r, handler)
	if depReq != nil {
		d, err := c.Manager.CreateDeployment(depReq)
		if err != nil {
			httputil.BadRequest(w, r, err)
			return nil
		}

		util.LogHandlerExitWithJSON(handler, w, d, http.StatusCreated)
	}

	return nil
}

func deleteDeploymentHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: delete deployment"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	name, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	d, err := c.Manager.DeleteDeployment(name, true)
	if err != nil {
		return err
	}

	util.LogHandlerExitWithJSON(handler, w, d, http.StatusOK)
	return nil
}

func putDeploymentHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: update deployment"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	name, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	depReq := getDeploymentRequest(w, r, handler)
	if depReq != nil {
		d, err := c.Manager.PutDeployment(name, depReq)
		if err != nil {
			httputil.BadRequest(w, r, err)
			return nil
		}

		util.LogHandlerExitWithJSON(handler, w, d, http.StatusCreated)
	}

	return nil
}

// pos gets a path item by position.
//
// For example. the path "/foo/bar" has three positions: "" at 0, "foo" at
// 1, and "bar" at 2.
//
// For verb/path combos, position 0 will be the verb: "GET /foo/bar" will have
// "GET " at position 0.
func pos(w http.ResponseWriter, r *http.Request, i int) (string, error) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < i-1 {
		return "", fmt.Errorf("No index for %d", i)
	}
	return parts[i], nil
}

func getDeploymentRequest(w http.ResponseWriter, r *http.Request, handler string) *common.DeploymentRequest {
	util.LogHandlerEntry(handler, r)
	depReq := &common.DeploymentRequest{}
	if err := httputil.Decode(w, r, depReq); err != nil {
		httputil.BadRequest(w, r, err)
		return nil
	}

	return depReq
}

func listManifestsHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: list manifests"
	util.LogHandlerEntry(handler, r)
	deploymentName, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	m, err := c.Manager.ListManifests(deploymentName)
	if err != nil {
		return err
	}

	var manifestNames []string
	for _, manifest := range m {
		manifestNames = append(manifestNames, manifest.Name)
	}

	util.LogHandlerExitWithJSON(handler, w, manifestNames, http.StatusOK)
	return nil
}

func getManifestHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: get manifest"
	util.LogHandlerEntry(handler, r)
	deploymentName, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	manifestName, err := pos(w, r, 4)
	if err != nil {
		return err
	}

	m, err := c.Manager.GetManifest(deploymentName, manifestName)
	if err != nil {
		httputil.BadRequest(w, r, err)
		return nil
	}

	util.LogHandlerExitWithJSON(handler, w, m, http.StatusOK)
	return nil
}

func expandHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: expand config"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	depReq := getDeploymentRequest(w, r, handler)
	if depReq != nil {
		c, err := c.Manager.Expand(depReq)
		if err != nil {
			httputil.BadRequest(w, r, err)
			return nil
		}

		util.LogHandlerExitWithJSON(handler, w, c, http.StatusCreated)
	}

	return nil
}

// Putting Type handlers here for now because deployments.go
// currently owns its own Manager backend and doesn't like to share.
func listChartsHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: list charts"
	util.LogHandlerEntry(handler, r)
	types, err := c.Manager.ListCharts()
	if err != nil {
		httputil.BadRequest(w, r, err)
		return nil
	}

	util.LogHandlerExitWithJSON(handler, w, types, http.StatusOK)
	return nil
}

func listChartInstancesHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: list chart instances"
	util.LogHandlerEntry(handler, r)
	chartName, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	instances, err := c.Manager.ListChartInstances(chartName)
	if err != nil {
		httputil.BadRequest(w, r, err)
		return nil
	}

	util.LogHandlerExitWithJSON(handler, w, instances, http.StatusOK)
	return nil
}

func getRepoForChartHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: get repository for chart"
	util.LogHandlerEntry(handler, r)
	chartName, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	repository, err := c.Manager.GetRepoForChart(chartName)
	if err != nil {
		httputil.BadRequest(w, r, err)
		return nil
	}

	util.LogHandlerExitWithJSON(handler, w, repository, http.StatusOK)
	return nil
}

func getMetadataForChartHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: get chart metadata"
	util.LogHandlerEntry(handler, r)
	chartName, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	metadata, err := c.Manager.GetMetadataForChart(chartName)
	if err != nil {
		httputil.BadRequest(w, r, err)
		return nil
	}

	util.LogHandlerExitWithJSON(handler, w, metadata, http.StatusOK)
	return nil
}

func getChartHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: get chart"
	util.LogHandlerEntry(handler, r)
	chartName, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	ch, err := c.Manager.GetChart(chartName)
	if err != nil {
		httputil.BadRequest(w, r, err)
		return nil
	}

	util.LogHandlerExitWithJSON(handler, w, ch, http.StatusOK)
	return nil
}

func getCredential(w http.ResponseWriter, r *http.Request, handler string) *repo.Credential {
	util.LogHandlerEntry(handler, r)
	t := &repo.Credential{}
	if err := httputil.Decode(w, r, t); err != nil {
		httputil.BadRequest(w, r, err)
		return nil
	}
	return t
}

func createCredentialHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: create credential"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	credentialName, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	cr := getCredential(w, r, handler)
	if cr != nil {
		err = c.Manager.CreateCredential(credentialName, cr)
		if err != nil {
			httputil.BadRequest(w, r, err)
			return nil
		}
	}

	util.LogHandlerExitWithJSON(handler, w, c, http.StatusOK)
	return nil
}

func getCredentialHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: get credential"
	util.LogHandlerEntry(handler, r)
	credentialName, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	cr, err := c.Manager.GetCredential(credentialName)
	if err != nil {
		httputil.BadRequest(w, r, err)
		return nil
	}

	util.LogHandlerExitWithJSON(handler, w, cr, http.StatusOK)
	return nil
}
