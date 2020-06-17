package list

import (
	"encoding/json"
	"fmt"
	"net/http"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/servercontext"
)

func Handler() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

		res.Header().Set("Content-Type", "application/json")
		defer req.Body.Close()

		var request ListRequest
		decoder := json.NewDecoder(req.Body)
		decoder.UseNumber()

		if err := decoder.Decode(&request); err != nil {
			fmt.Println("error in request")
			return
		}

		request.RequestID = req.Header.Get("Request-Id")

		list := action.NewList(servercontext.App().ActionConfig)

		list.SetStateMask()
		results, err := list.Run()
		if err != nil {
			fmt.Printf("error while running helm list %v", err)
		}

		var helmReleases []HelmRelease
		for _, res := range results {
			r := HelmRelease{Release: res.Name, Namespace: res.Namespace}
			helmReleases = append(helmReleases, r)
		}

		response := ListRespose{Status: true, Data: helmReleases}
		payload, err := json.Marshal(response)
		if err != nil {
			fmt.Println("error parsing response")
			return
		}

		res.Write(payload)
	})
}
