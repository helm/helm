package api

import (
	"encoding/json"
	"helm.sh/helm/v3/pkg/api/logger"
	"net/http"
)

type ListRequest struct {
	RequestID string
}

type ListResponse struct {
	Error  string `json:"error,omitempty"`
	Data   []HelmRelease
}

type HelmRelease struct {
	Release   string `json:"release"`
	Namespace string `json:"namespace"`
}

func List(svc Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var response ListResponse
		var request ListRequest
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		if err := decoder.Decode(&request); err != nil {
			logger.Errorf("[List] error decoding request: %v", err)
			response.Error = err.Error()
			payload, _ := json.Marshal(response)
			w.WriteHeader(http.StatusBadRequest)
			w.Write(payload)
			return
		}
		defer r.Body.Close()

		request.RequestID = r.Header.Get("Request-Id")

		svc.lister.SetStateMask()
		res, err := svc.lister.Run()

		if err != nil {
			logger.Errorf("[List] error while installing chart: %v", err)
			response.Error = err.Error()
			payload, _ := json.Marshal(response)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(payload)
			return
		}

		var helmReleases []HelmRelease
		for _, eachRes := range res {
			r := HelmRelease{Release: eachRes.Name, Namespace: eachRes.Namespace}
			helmReleases = append(helmReleases, r)
		}
		response = ListResponse{"", helmReleases}
		payload, err := json.Marshal(response)
		if err != nil {
			logger.Errorf("[List] error writing response %v", err)
			response.Error = err.Error()
			payload, _ := json.Marshal(response)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(payload)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(payload)
	})
}