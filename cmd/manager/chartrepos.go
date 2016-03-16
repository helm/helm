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
	util.LogHandlerExitWithJSON(handler, w, "a repo here", http.StatusOK)
	return nil
}

func addChartRepoHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: add chart repository"
	util.LogHandlerExitWithJSON(handler, w, "add a repo", http.StatusOK)
	return nil
}

func removeChartRepoHandlerFunc(w http.ResponseWriter, r *http.Request, c *router.Context) error {
	handler := "manager: remove chart repository"
	util.LogHandlerExitWithJSON(handler, w, "remove a repo", http.StatusOK)
	return nil
}
