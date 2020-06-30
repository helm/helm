package api

import (
	"encoding/json"
	"helm.sh/helm/v3/pkg/api/logger"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/time"
	"net/http"
)

type ListRequest struct {
	NameSpace     string `json:"namespace"`
	ReleaseStatus string `json:"release_status"`
}

type ListResponse struct {
	Error    string    `json:"error,omitempty"`
	Releases []Release `json:"releases,omitempty"`
}

type Release struct {
	Name        string         `json:"name"`
	Namespace   string         `json:"namespace"`
	Revision    int            `json:"revision"`
	Updated     time.Time      `json:"updated_at,omitempty"`
	Status      release.Status `json:"status"`
	Chart       string         `json:"chart"`
	AppVersion  string         `json:"app_version"`
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
