package api

import (
	"encoding/json"
	"net/http"

	"helm.sh/helm/v3/pkg/api/logger"
)

type ListRequest struct {
	NameSpace     string `json:"namespace"`
	ReleaseStatus string `json:"release_status"`
}

type ListResponse struct {
	Error    string `json:"error,omitempty"`
	Releases []Release
}

type Release struct {
	Name      string `json:"release"`
	Namespace string `json:"namespace"`
}

func List(svc Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var response ListResponse
		var request ListRequest
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&request); err != nil {
			logger.Errorf("[List] error decoding request: %v", err)
			response.Error = err.Error()
			payload, _ := json.Marshal(response)
			w.WriteHeader(http.StatusBadRequest)
			w.Write(payload)
			return
		}
		defer r.Body.Close()

		helmReleases, err := svc.List(request.ReleaseStatus)

		if err != nil {
			logger.Errorf("[List] error while installing chart: %v", err)
			response.Error = err.Error()
			payload, _ := json.Marshal(response)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(payload)
			return
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
