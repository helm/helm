package main

import (
	"fmt"
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

func startServer() {
	router := http.NewServeMux()

	//TODO: use gorilla mux and add middleware to write content type and other headers
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

	router.Handle("/ping", ping.Handler())
	router.Handle("/list", api.List(service))
	router.Handle("/install", api.Install(service))
	router.Handle("/upgrade", api.Upgrade(service))

	err := http.ListenAndServe(fmt.Sprintf(":%d", 8080), router)
	if err != nil {
		fmt.Println("error starting server", err)
	}
}
