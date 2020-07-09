package api

import (
	"encoding/json"
	"io"
	"net/http"

	"helm.sh/helm/v3/pkg/api/logger"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/time"
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
	Name       string         `json:"name"`
	Namespace  string         `json:"namespace"`
	Revision   int            `json:"revision"`
	Updated    time.Time      `json:"updated_at,omitempty"`
	Status     release.Status `json:"status"`
	Chart      string         `json:"chart"`
	AppVersion string         `json:"app_version"`
}

func List(svc Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var response ListResponse
		var request ListRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err == io.EOF || err != nil {
			logger.Errorf("[List] error decoding request: %v", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			response.Error = err.Error()
			json.NewEncoder(w).Encode(response)
			return
		}
		defer r.Body.Close()

		helmReleases, err := svc.List(request.ReleaseStatus, request.NameSpace)

		if err != nil {
			respondInstallError(w, "error while listing charts: %v", err)
		}

		response = ListResponse{"", helmReleases}
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			respondInstallError(w, "error writing response: %v", err)
			return
		}
	})
}

func respondListError(w http.ResponseWriter, logprefix string, err error) {
	response := ListResponse{Error: err.Error()}
	w.WriteHeader(http.StatusInternalServerError)
	if err := json.NewEncoder(w).Encode(&response); err != nil {
		logger.Errorf("[List] %s %v", logprefix, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
