/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"github.com/kubernetes/helm/cmd/manager/router"
	"github.com/kubernetes/helm/pkg/httputil"
	"github.com/kubernetes/helm/pkg/repo"
	"github.com/kubernetes/helm/pkg/util"

	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
)

func registerChartRepoRoutes(c *router.Context, h *router.Handler) {
	h.Add("GET /repositories", listChartReposHandlerFunc)
	h.Add("GET /repositories/*", getChartRepoHandlerFunc)
	h.Add("GET /repositories/*/charts", listRepoChartsHandlerFunc)
	h.Add("GET /repositories/*/charts/*", getRepoChartHandlerFunc)
	h.Add("POST /repositories", addChartRepoHandlerFunc)
	h.Add("DELETE /repositories/*", removeChartRepoHandlerFunc)
}

func listChartReposHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: list chart repositories"
	repos, err := c.Manager.ListRepos()
	if err != nil {
		return err
	}

	util.LogHandlerExitWithJSON(handler, w, repos, http.StatusOK)
	return nil
}

func addChartRepoHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: add chart repository"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	cr := &repo.Repo{}
	if err := httputil.Decode(w, r, cr); err != nil {
		httputil.BadRequest(w, r, err)
		return nil
	}

	if string(cr.Format) == "" {
		cr.Type = repo.GCSRepoType
	}

	if string(cr.Type) == "" {
		cr.Type = repo.GCSRepoType
	}

	if err := c.Manager.AddRepo(cr); err != nil {
		httputil.BadRequest(w, r, err)
		return nil
	}

	msg, _ := json.Marshal(cr.Name + " has been added to the list of chart repositories.")
	util.LogHandlerExitWithJSON(handler, w, msg, http.StatusCreated)
	return nil
}

func removeChartRepoHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: remove chart repository"
	util.LogHandlerEntry(handler, r)
	defer r.Body.Close()
	name, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	err = c.Manager.RemoveRepo(name)
	if err != nil {
		return err
	}

	msg, _ := json.Marshal(name + " has been removed from the list of chart repositories.")
	util.LogHandlerExitWithJSON(handler, w, msg, http.StatusOK)
	return nil
}

func getChartRepoHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: get repository"
	util.LogHandlerEntry(handler, r)
	repoURL, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	cr, err := c.Manager.GetRepo(repoURL)
	if err != nil {
		httputil.BadRequest(w, r, err)
		return nil
	}

	util.LogHandlerExitWithJSON(handler, w, cr, http.StatusOK)
	return nil
}

func listRepoChartsHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: list repository charts"
	util.LogHandlerEntry(handler, r)
	repoURL, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		httputil.BadRequest(w, r, err)
		return nil
	}

	var regex *regexp.Regexp
	regexString := values.Get("regex")
	if regexString != "" {
		regex, err = regexp.Compile(regexString)
		if err != nil {
			httputil.BadRequest(w, r, err)
			return nil
		}
	}

	repoCharts, err := c.Manager.ListRepoCharts(repoURL, regex)
	if err != nil {
		return err
	}

	util.LogHandlerExitWithJSON(handler, w, repoCharts, http.StatusOK)
	return nil
}

func getRepoChartHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: get repository charts"
	util.LogHandlerEntry(handler, r)
	repoURL, err := pos(w, r, 2)
	if err != nil {
		return err
	}

	chartName, err := pos(w, r, 4)
	if err != nil {
		return err
	}

	repoChart, err := c.Manager.GetChartForRepo(repoURL, chartName)
	if err != nil {
		return err
	}

	util.LogHandlerExitWithJSON(handler, w, repoChart, http.StatusOK)
	return nil
}
