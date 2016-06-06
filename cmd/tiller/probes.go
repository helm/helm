package main

import (
	"net/http"
)

func readinessProbe(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func livenessProbe(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func newProbesMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/readiness", readinessProbe)
	mux.HandleFunc("/liveness", livenessProbe)
	return mux
}
