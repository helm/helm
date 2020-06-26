package api

import (
	"encoding/json"
	"net/http"

	"helm.sh/helm/v3/pkg/api/logger"
)

type InstallRequest struct {
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace"`
	Chart     string                 `json:"chart"`
	Values    map[string]interface{} `json:"values"`
}

type InstallResponse struct {
	Error  string `json:"error,omitempty"`
	Status string `json:"status,omitempty"`
}

// RODO: we could use interface as well if everything's in same package
func Install(svc Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var req InstallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Errorf("[Install] error decoding request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		var response InstallResponse
		cfg := ReleaseConfig{ChartName: req.Chart, Name: req.Name, Namespace: req.Namespace}
		res, err := svc.Install(r.Context(), cfg, req.Values)
		if err != nil {
			respondError(w, "error while installing chart: %v", err)
			return
		}
		response.Status = res.status
		if err := json.NewEncoder(w).Encode(&response); err != nil {
			logger.Errorf("[Install] error writing response %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}

func respondError(w http.ResponseWriter, logprefix string, err error) {
	response := InstallResponse{Error: err.Error()}
	w.WriteHeader(http.StatusInternalServerError)
	if err := json.NewEncoder(w).Encode(&response); err != nil {
		logger.Errorf("[Install] %s %v", logprefix, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
