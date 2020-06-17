package main

import (
	"fmt"
	"net/http"

	"helm.sh/helm/v3/cmd/endpoints/install"
	"helm.sh/helm/v3/cmd/endpoints/list"
	"helm.sh/helm/v3/cmd/endpoints/ping"
	"helm.sh/helm/v3/cmd/servercontext"
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

	err := http.ListenAndServe(fmt.Sprintf(":%d", 8080), router)
	if err != nil {
		fmt.Println("error starting server", err)
	}
}
