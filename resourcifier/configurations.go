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
	"github.com/kubernetes/deployment-manager/common"
	"github.com/kubernetes/deployment-manager/resourcifier/configurator"
	"github.com/kubernetes/deployment-manager/util"

	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
)

var configurations = []Route{
	{"ListConfigurations", "/configurations/{type}", "GET", listConfigurationsHandlerFunc, ""},
	{"GetConfiguration", "/configurations/{type}/{name}", "GET", getConfigurationHandlerFunc, ""},
	{"CreateConfiguration", "/configurations", "POST", createConfigurationHandlerFunc, "JSON"},
	{"DeleteConfiguration", "/configurations", "DELETE", deleteConfigurationHandlerFunc, "JSON"},
	{"PutConfiguration", "/configurations", "PUT", putConfigurationHandlerFunc, "JSON"},
}

var (
	maxLength      = flag.Int64("maxLength", 1024*8, "The maximum length (KB) of a configuration.")
	kubePath       = flag.String("kubectl", "./kubectl", "The path to the kubectl binary.")
	kubeService    = flag.String("service", "", "The DNS name of the kubernetes service.")
	kubeServer     = flag.String("server", "", "The IP address and optional port of the kubernetes master.")
	kubeInsecure   = flag.Bool("insecure-skip-tls-verify", false, "Do not check the server's certificate for validity.")
	kubeConfig     = flag.String("config", "", "Path to a kubeconfig file.")
	kubeCertAuth   = flag.String("certificate-authority", "", "Path to a file for the certificate authority.")
	kubeClientCert = flag.String("client-certificate", "", "Path to a client certificate file.")
	kubeClientKey  = flag.String("client-key", "", "Path to a client key file.")
	kubeToken      = flag.String("token", "", "A service account token.")
	kubeUsername   = flag.String("username", "", "The username to use for basic auth.")
	kubePassword   = flag.String("password", "", "The password to use for basic auth.")
)

var backend *configurator.Configurator

func init() {
	if !flag.Parsed() {
		flag.Parse()
	}

	routes = append(routes, configurations...)
	backend = getConfigurator()
}

func listConfigurationsHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "resourcifier: list configurations"
	util.LogHandlerEntry(handler, r)
	rtype, err := getPathVariable(w, r, "type", handler)
	if err != nil {
		return
	}

	c := &common.Configuration{
		Resources: []*common.Resource{
			{Type: rtype},
		},
	}

	output, err := backend.Configure(c, configurator.GetOperation)
	if err != nil {
		util.LogAndReturnError(handler, http.StatusBadRequest, err, w)
		return
	}

	util.LogHandlerExit(handler, http.StatusOK, output, w)
	util.WriteYAML(handler, w, []byte(output), http.StatusOK)
}

func getConfigurationHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "resourcifier: get configuration"
	util.LogHandlerEntry(handler, r)
	rtype, err := getPathVariable(w, r, "type", handler)
	if err != nil {
		return
	}

	rname, err := getPathVariable(w, r, "name", handler)
	if err != nil {
		return
	}

	c := &common.Configuration{
		Resources: []*common.Resource{
			{Name: rname, Type: rtype},
		},
	}

	output, err := backend.Configure(c, configurator.GetOperation)
	if err != nil {
		util.LogAndReturnError(handler, http.StatusBadRequest, err, w)
		return
	}

	util.LogHandlerExit(handler, http.StatusOK, output, w)
	util.WriteYAML(handler, w, []byte(output), http.StatusOK)
}

func createConfigurationHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "resourcifier: create configuration"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	c := getConfiguration(w, r, handler)
	if c != nil {
		_, err := backend.Configure(c, configurator.CreateOperation)
		if err != nil {
			util.LogAndReturnError(handler, http.StatusBadRequest, err, w)
			return
		}

		util.LogHandlerExitWithYAML(handler, w, c, http.StatusCreated)
		return
	}

	util.LogHandlerExit(handler, http.StatusOK, "OK", w)
}

func deleteConfigurationHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "resourcifier: delete configuration"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	c := getConfiguration(w, r, handler)
	if c != nil {
		if _, err := backend.Configure(c, configurator.DeleteOperation); err != nil {
			e := errors.New("cannot delete configuration: " + err.Error() + "\n")
			util.LogAndReturnError(handler, http.StatusBadRequest, e, w)
			return
		}

		w.WriteHeader(http.StatusNoContent)
		util.LogHandlerExit(handler, http.StatusNoContent, "No Content", w)
		return
	}

	util.LogHandlerExit(handler, http.StatusOK, "OK", w)
}

func putConfigurationHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "resourcifier: update configuration"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	c := getConfiguration(w, r, handler)
	if c != nil {
		if _, err := backend.Configure(c, configurator.ReplaceOperation); err != nil {
			e := errors.New("cannot replace configuration: " + err.Error() + "\n")
			util.LogAndReturnError(handler, http.StatusBadRequest, e, w)
			return
		}

		util.LogHandlerExitWithYAML(handler, w, c, http.StatusCreated)
		return
	}

	util.LogHandlerExit(handler, http.StatusOK, "OK", w)
}

func getConfigurator() *configurator.Configurator {
	kubernetesConfig := &util.KubernetesConfig{
		KubePath:       *kubePath,
		KubeService:    *kubeService,
		KubeServer:     *kubeServer,
		KubeInsecure:   *kubeInsecure,
		KubeConfig:     *kubeConfig,
		KubeCertAuth:   *kubeCertAuth,
		KubeClientCert: *kubeClientCert,
		KubeClientKey:  *kubeClientKey,
		KubeToken:      *kubeToken,
		KubeUsername:   *kubeUsername,
		KubePassword:   *kubePassword,
	}
	return configurator.NewConfigurator(util.NewKubernetesKubectl(kubernetesConfig))
}

func getPathVariable(w http.ResponseWriter, r *http.Request, variable, handler string) (string, error) {
	vars := mux.Vars(r)
	escaped, ok := vars[variable]
	if !ok {
		e := errors.New(fmt.Sprintf("%s name not found in URL", variable))
		util.LogAndReturnError(handler, http.StatusBadRequest, e, w)
		return "", e
	}

	unescaped, err := url.QueryUnescape(escaped)
	if err != nil {
		e := fmt.Errorf("cannot decode name (%v)", variable)
		util.LogAndReturnError(handler, http.StatusBadRequest, e, w)
		return "", e
	}

	return unescaped, nil
}

func getConfiguration(w http.ResponseWriter, r *http.Request, handler string) *common.Configuration {
	b := io.LimitReader(r.Body, *maxLength*1024)
	y, err := ioutil.ReadAll(b)
	if err != nil {
		util.LogAndReturnError(handler, http.StatusBadRequest, err, w)
		return nil
	}

	// Reject the input if it exceeded the length limit,
	// since we may not have read all of it into the buffer.
	if _, err = b.Read(make([]byte, 0, 1)); err != io.EOF {
		e := fmt.Errorf("configuration exceeds maximum length of %d KB.", *maxLength)
		util.LogAndReturnError(handler, http.StatusBadRequest, e, w)
		return nil
	}

	j, err := yaml.YAMLToJSON(y)
	if err != nil {
		e := errors.New(err.Error() + "\n" + string(y))
		util.LogAndReturnError(handler, http.StatusBadRequest, e, w)
		return nil
	}

	c := &common.Configuration{}
	if err := json.Unmarshal(j, c); err != nil {
		e := errors.New(err.Error() + "\n" + string(j))
		util.LogAndReturnError(handler, http.StatusBadRequest, e, w)
		return nil
	}

	if len(c.Resources) < 1 {
		e := fmt.Errorf("configuration is empty")
		util.LogAndReturnError(handler, http.StatusBadRequest, e, w)
		return nil
	}

	return c
}
