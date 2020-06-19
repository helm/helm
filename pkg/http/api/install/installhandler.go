package install

import (
	"encoding/json"
	"fmt"
	"net/http"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/servercontext"
)

func Handler() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		defer req.Body.Close()

		var request InstallRequest
		decoder := json.NewDecoder(req.Body)
		decoder.UseNumber()

		if err := decoder.Decode(&request); err != nil {
			fmt.Printf("error in request: %v", err)
			return
		}

		request.RequestID = req.Header.Get("Request-Id")
		request.ReleaseName = req.Header.Get("Release-Name")
		request.ReleaseNamespace = req.Header.Get("Release-Namespace")
		request.ChartPath = req.Header.Get("Chart-Path")
		request.Values = req.Header.Get("Values")

		valueOpts := &values.Options{}
		valueOpts.Values = append(valueOpts.Values, request.Values)

		status, releaseStatus, err := InstallChart(request.ReleaseName, request.ReleaseNamespace, request.ChartPath, valueOpts)
		if err != nil {
			fmt.Printf("error in request: %v", err)
			return
		}

		response := InstallResponse{Status: status, ReleaseStatus: releaseStatus}
		payload, err := json.Marshal(response)
		if err != nil {
			fmt.Printf("error parsing response %v", err)
			return
		}

		res.Write(payload)
	})
}

func InstallChart(releaseName, releaseNamespace, chartPath string, valueOpts *values.Options) (bool, string, error) {
	install := action.NewInstall(servercontext.App().ActionConfig)
	install.ReleaseName = releaseName
	install.Namespace = releaseNamespace

	vals, err := valueOpts.MergeValues(getter.All(servercontext.App().Config))
	if err != nil {
		return false, "", err
	}

	cp, err := install.ChartPathOptions.LocateChart(chartPath, servercontext.App().Config)
	if err != nil {
		fmt.Printf("error in locating chart: %v", err)
		return false, "", err
	}

	var requestedChart *chart.Chart
	if requestedChart, err = loader.Load(cp); err != nil {
		fmt.Printf("error in loading chart: %v", err)
		return false, "", err
	}

	release, err := install.Run(requestedChart, vals)
	if err != nil {
		fmt.Printf("error in installing chart: %v", err)
		return false, "", err
	}

	return true, release.Info.Status.String(), nil
}
