package http

import (
	"encoding/json"

	"helm.sh/helm/v3/pkg/action"
)

type helmRelease struct {
	Release   string `json:"release"`
	Namespace string `json:"namespace"`
}

func List(cfg *action.Configuration) ([]byte, error) {
	list := action.NewList(cfg)

	list.SetStateMask()

	results, err := list.Run()
	if err != nil {
		return nil, err
	}

	var helmReleases []helmRelease

	for _, res := range results {
		r := helmRelease{Release: res.Name, Namespace: res.Namespace}
		helmReleases = append(helmReleases, r)
	}

	jsonReleases, err := json.Marshal(helmReleases)
	if err != nil {
		return nil, err
	}
	return jsonReleases, err
}
