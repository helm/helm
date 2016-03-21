package main

import (
	"github.com/kubernetes/helm/cmd/manager/router"
	"github.com/kubernetes/helm/pkg/util"
	"net/http"
)

func registerChartRepoRoutes(c *router.Context, h *router.Handler) {
	h.Add("GET /chart_repositories", listChartRepositoriesHandlerFunc)
	h.Add("POST /chart_repositories", addChartRepoHandlerFunc)
	h.Add("DELETE /chart_repositories", removeChartRepoHandlerFunc)
}

func listChartRepositoriesHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: list chart repositories"
	repos, err := c.Manager.ListChartRepos()
	if err != nil {
		return err
	}
	util.LogHandlerExitWithJSON(handler, w, repos, http.StatusOK)
	return nil
}

func addChartRepoHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: add chart repository"
	name, err := pos(w, r, 2)
	if err != nil {
		return err
	}
	err = c.Manager.AddChartRepo(name)
	if err != nil {
		return err
	}
	util.LogHandlerExitWithJSON(handler, w, "added", http.StatusOK)
	return nil
}

func removeChartRepoHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: remove chart repository"
	name, err := pos(w, r, 2)
	if err != nil {
		return err
	}
	err = c.Manager.RemoveChartRepo(name)
	if err != nil {
		return err
	}
	util.LogHandlerExitWithJSON(handler, w, "removed", http.StatusOK)
	return nil
}
