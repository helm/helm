package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"net/http"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/api"
	"helm.sh/helm/v3/pkg/api/logger"
	"helm.sh/helm/v3/pkg/api/ping"
	"helm.sh/helm/v3/pkg/servercontext"
)

func main() {
	servercontext.NewApp()
	startServer()
}

func setContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func startServer() {
	router := mux.NewRouter()

	app := servercontext.App()
	logger.Setup("debug")

	actionList := action.NewList(app.ActionConfig)
	actionInstall := action.NewInstall(app.ActionConfig)
	actionUpgrade := action.NewUpgrade(app.ActionConfig)
	actionHistory := action.NewHistory(app.ActionConfig)

	service := api.NewService(app.Config,
		new(action.ChartPathOptions),
		api.NewList(actionList),
		api.NewInstall(actionInstall),
		api.NewUpgrader(actionUpgrade),
		api.NewHistory(actionHistory))

	router.Handle("/ping", setContentType(ping.Handler())).Methods(http.MethodGet)
	router.Handle("/list", setContentType(api.List(service))).Methods(http.MethodPost)
	router.Handle("/install", setContentType(api.Install(service))).Methods(http.MethodPost)
	router.Handle("/upgrade", setContentType(api.Upgrade(service))).Methods(http.MethodPost)

	err := http.ListenAndServe(fmt.Sprintf(":%d", 8080), router)
	if err != nil {
		fmt.Println("error starting server", err)
	}
}
