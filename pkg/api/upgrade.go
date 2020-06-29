package api

import (
	"encoding/json"
	"net/http"

	"helm.sh/helm/v3/pkg/api/logger"
)

type UpgradeRequest struct {
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace"`
	Chart     string                 `json:"chart"`
	Values    map[string]interface{} `json:"values"`
}

type UpgradeResponse struct {
	Error  string `json:"error,omitempty"`
	Status string `json:"status,omitempty"`
}

func Upgrade(svc Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		defer r.Body.Close()

		var req UpgradeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Errorf("[Upgrade] error decoding request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		var response UpgradeResponse
		cfg := ReleaseConfig{ChartName: req.Chart, Name: req.Name, Namespace: req.Namespace}
		res, err := svc.Upgrade(r.Context(), cfg, req.Values)
		if err != nil {
			respondError(w, "error while upgrading chart: %v", err)
			return
		}
		response.Status = res.status
		if err := json.NewEncoder(w).Encode(&response); err != nil {
			logger.Errorf("[Upgrade] error writing response %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}
