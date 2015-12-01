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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"

	"github.com/kubernetes/deployment-manager/manager/manager"
	"github.com/kubernetes/deployment-manager/common"
	"github.com/kubernetes/deployment-manager/manager/repository"
	"github.com/kubernetes/deployment-manager/util"
)

var deployments = []Route{
	{"ListDeployments", "/deployments", "GET", listDeploymentsHandlerFunc, ""},
	{"GetDeployment", "/deployments/{deployment}", "GET", getDeploymentHandlerFunc, ""},
	{"CreateDeployment", "/deployments", "POST", createDeploymentHandlerFunc, "JSON"},
	{"DeleteDeployment", "/deployments/{deployment}", "DELETE", deleteDeploymentHandlerFunc, ""},
	{"PutDeployment", "/deployments/{deployment}", "PUT", putDeploymentHandlerFunc, "JSON"},
	{"ListManifests", "/deployments/{deployment}/manifests", "GET", listManifestsHandlerFunc, ""},
	{"GetManifest", "/deployments/{deployment}/manifests/{manifest}", "GET", getManifestHandlerFunc, ""},
	{"Expand", "/expand", "POST", expandHandlerFunc, ""},
	{"ListTypes", "/types", "GET", listTypesHandlerFunc, ""},
	{"ListTypeInstances", "/types/{type}/instances", "GET", listTypeInstancesHandlerFunc, ""},
}

var (
	maxLength    = flag.Int64("maxLength", 1024, "The maximum length (KB) of a template.")
	expanderName = flag.String("expander", "expandybird-service", "The DNS name of the expander service.")
	expanderURL  = flag.String("expanderURL", "", "The URL for the expander service.")
	deployerName = flag.String("deployer", "resourcifier-service", "The DNS name of the deployer service.")
	deployerURL  = flag.String("deployerURL", "", "The URL for the deployer service.")
)

var backend manager.Manager

func init() {
	if !flag.Parsed() {
		flag.Parse()
	}

	routes = append(routes, deployments...)
	backend = newManager()
}

func newManager() manager.Manager {
	expander := manager.NewExpander(getServiceURL(*expanderURL, *expanderName), manager.NewTypeResolver())
	deployer := manager.NewDeployer(getServiceURL(*deployerURL, *deployerName))
	r := repository.NewMapBasedRepository()
	return manager.NewManager(expander, deployer, r)
}

func getServiceURL(serviceURL, serviceName string) string {
	if serviceURL == "" {
		serviceURL = makeEnvVariableURL(serviceName)
		if serviceURL == "" {
			addrs, err := net.LookupHost(serviceName)
			if err != nil || len(addrs) < 1 {
				log.Fatalf("cannot resolve service:%v. environment:%v", serviceName, os.Environ())
			}

			serviceURL = fmt.Sprintf("https://%s", addrs[0])
		}
	}

	return serviceURL
}

// makeEnvVariableURL takes a service name and returns the value of the
// environment variable that identifies its URL, if it exists, or the empty
// string, if it doesn't.
func makeEnvVariableURL(str string) string {
	prefix := makeEnvVariableName(str)
	url := os.Getenv(prefix + "_PORT")
	return strings.Replace(url, "tcp", "http", 1)
}

// makeEnvVariableName is copied from the Kubernetes source,
// which is referenced by the documentation for service environment variables.
func makeEnvVariableName(str string) string {
	// TODO: If we simplify to "all names are DNS1123Subdomains" this
	// will need two tweaks:
	//   1) Handle leading digits
	//   2) Handle dots
	return strings.ToUpper(strings.Replace(str, "-", "_", -1))
}

func listDeploymentsHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "manager: list deployments"
	util.LogHandlerEntry(handler, r)
	l, err := backend.ListDeployments()
	if err != nil {
		util.LogAndReturnError(handler, http.StatusInternalServerError, err, w)
		return
	}
	var names []string
	for _, d := range l {
		names = append(names, d.Name)
	}

	util.LogHandlerExitWithJSON(handler, w, names, http.StatusOK)
}

func getDeploymentHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "manager: get deployment"
	util.LogHandlerEntry(handler, r)
	name, err := getPathVariable(w, r, "deployment", handler)
	if err != nil {
		return
	}

	d, err := backend.GetDeployment(name)
	if err != nil {
		util.LogAndReturnError(handler, http.StatusBadRequest, err, w)
		return
	}

	util.LogHandlerExitWithJSON(handler, w, d, http.StatusOK)
}

func createDeploymentHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "manager: create deployment"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	t := getTemplate(w, r, handler)
	if t != nil {
		d, err := backend.CreateDeployment(t)
		if err != nil {
			util.LogAndReturnError(handler, http.StatusBadRequest, err, w)
			return
		}

		util.LogHandlerExitWithJSON(handler, w, d, http.StatusCreated)
		return
	}
}

func deleteDeploymentHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "manager: delete deployment"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	name, err := getPathVariable(w, r, "deployment", handler)
	if err != nil {
		return
	}

	d, err := backend.DeleteDeployment(name, true)
	if err != nil {
		util.LogAndReturnError(handler, http.StatusBadRequest, err, w)
		return
	}

	util.LogHandlerExitWithJSON(handler, w, d, http.StatusOK)
}

func putDeploymentHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "manager: update deployment"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	name, err := getPathVariable(w, r, "deployment", handler)
	if err != nil {
		return
	}

	t := getTemplate(w, r, handler)
	if t != nil {
		d, err := backend.PutDeployment(name, t)
		if err != nil {
			util.LogAndReturnError(handler, http.StatusBadRequest, err, w)
			return
		}

		util.LogHandlerExitWithJSON(handler, w, d, http.StatusCreated)
	}
}

func getPathVariable(w http.ResponseWriter, r *http.Request, variable, handler string) (string, error) {
	vars := mux.Vars(r)
	retVariable, ok := vars[variable]
	if !ok {
		e := fmt.Errorf("%s parameter not found in URL", variable)
		util.LogAndReturnError(handler, http.StatusBadRequest, e, w)
		return "", e
	}
	return retVariable, nil
}

func getTemplate(w http.ResponseWriter, r *http.Request, handler string) *common.Template {
	util.LogHandlerEntry(handler, r)
	b := io.LimitReader(r.Body, *maxLength*1024)
	y, err := ioutil.ReadAll(b)
	if err != nil {
		util.LogAndReturnError(handler, http.StatusBadRequest, err, w)
		return nil
	}

	// Reject the input if it exceeded the length limit,
	// since we may not have read all of it into the buffer.
	if _, err = b.Read(make([]byte, 0, 1)); err != io.EOF {
		e := fmt.Errorf("template exceeds maximum length of %d KB", *maxLength)
		util.LogAndReturnError(handler, http.StatusBadRequest, e, w)
		return nil
	}

	if err := r.Body.Close(); err != nil {
		util.LogAndReturnError(handler, http.StatusInternalServerError, err, w)
		return nil
	}

	j, err := yaml.YAMLToJSON(y)
	if err != nil {
		e := fmt.Errorf("%v\n%v", err, string(y))
		util.LogAndReturnError(handler, http.StatusBadRequest, e, w)
		return nil
	}

	t := &common.Template{}
	if err := json.Unmarshal(j, t); err != nil {
		e := fmt.Errorf("%v\n%v", err, string(j))
		util.LogAndReturnError(handler, http.StatusBadRequest, e, w)
		return nil
	}

	return t
}

func listManifestsHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "manager: list manifests"
	util.LogHandlerEntry(handler, r)
	deploymentName, err := getPathVariable(w, r, "deployment", handler)
	if err != nil {
		return
	}

	m, err := backend.ListManifests(deploymentName)
	if err != nil {
		util.LogAndReturnError(handler, http.StatusInternalServerError, err, w)
		return
	}

	var manifestNames []string
	for _, manifest := range m {
		manifestNames = append(manifestNames, manifest.Name)
	}

	util.LogHandlerExitWithJSON(handler, w, manifestNames, http.StatusOK)
}

func getManifestHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "manager: get manifest"
	util.LogHandlerEntry(handler, r)
	deploymentName, err := getPathVariable(w, r, "deployment", handler)
	if err != nil {
		return
	}

	manifestName, err := getPathVariable(w, r, "manifest", handler)
	if err != nil {
		return
	}

	m, err := backend.GetManifest(deploymentName, manifestName)
	if err != nil {
		util.LogAndReturnError(handler, http.StatusBadRequest, err, w)
		return
	}

	util.LogHandlerExitWithJSON(handler, w, m, http.StatusOK)
}

func expandHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "manager: expand config"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	t := getTemplate(w, r, handler)
	if t != nil {
		c, err := backend.Expand(t)
		if err != nil {
			util.LogAndReturnError(handler, http.StatusBadRequest, err, w)
			return
		}

		util.LogHandlerExitWithJSON(handler, w, c, http.StatusCreated)
		return
	}
}

// Putting Type handlers here for now because deployments.go
// currently owns its own Manager backend and doesn't like to share.
func listTypesHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "manager: list types"
	util.LogHandlerEntry(handler, r)
	util.LogHandlerExitWithJSON(handler, w, backend.ListTypes(), http.StatusOK)
}

func listTypeInstancesHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "manager: list instances"
	util.LogHandlerEntry(handler, r)
	typeName, err := getPathVariable(w, r, "type", handler)
	if err != nil {
		return
	}

	util.LogHandlerExitWithJSON(handler, w, backend.ListInstances(typeName), http.StatusOK)
}
