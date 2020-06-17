package main

import (
	"fmt"
	"net/http"

	"helm.sh/helm/v3/pkg/http/api/install"
	"helm.sh/helm/v3/pkg/http/api/list"
	"helm.sh/helm/v3/pkg/http/api/ping"
	"helm.sh/helm/v3/pkg/http/api/upgrade"
	"helm.sh/helm/v3/pkg/servercontext"
)

func main() {
	app := servercontext.NewApp()
	startServer(app)
}

func startServer(appconfig *servercontext.Application) {
	router := http.NewServeMux()
	router.Handle("/ping", ping.Handler())
	router.Handle("/list", list.Handler())
	router.Handle("/install", install.Handler())
	router.Handle("/upgrade", upgrade.Handler())

	err := http.ListenAndServe(fmt.Sprintf(":%d", 8080), router)
	if err != nil {
		fmt.Println("error starting server", err)
	}
}
