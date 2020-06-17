package upgrade

import (
	"encoding/json"
	"fmt"
	"net/http"

	"helm.sh/helm/v3/cmd/endpoints/install"
	"helm.sh/helm/v3/cmd/servercontext"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/storage/driver"
)

func Handler() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		defer req.Body.Close()

		var request UpgradeRequest
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
		valueOpts := &values.Options{}

		status, releaseStatus, err := UpgradeRelease(request.ReleaseName, request.ReleaseNamespace, request.ChartPath, valueOpts)
		if err != nil {
			fmt.Printf("error in request: %v", err)
			return
		}

		response := UpgradeResponse{Status: status, ReleaseStatus: releaseStatus}
		payload, err := json.Marshal(response)
		if err != nil {
			fmt.Printf("error parsing response %v", err)
			return
		}

		res.Write(payload)
	})
}

func UpgradeRelease(releaseName, releaseNamespace, chartPath string, valueOpts *values.Options) (bool, string, error) {
	upgrade := action.NewUpgrade(servercontext.App().ActionConfig)
	upgrade.Namespace = releaseNamespace

	vals, err := valueOpts.MergeValues(getter.All(servercontext.App().Config))
	if err != nil {
		return false, "", err
	}

	chartPath, err = upgrade.ChartPathOptions.LocateChart(chartPath, servercontext.App().Config)
	if err != nil {
		return false, "", err
	}
	if upgrade.Install {
		history := action.NewHistory(servercontext.App().ActionConfig)
		history.Max = 1
		if _, err := history.Run(releaseName); err == driver.ErrReleaseNotFound {
			fmt.Printf("Release %q does not exist. Installing it now.\n", releaseName)

			status, releaseStatus, err := install.InstallChart(releaseName, releaseNamespace, chartPath, valueOpts)
			if err != nil {
				fmt.Printf("error in request: %v", err)
				return false, "", err
			}
			return status, releaseStatus, nil
		}
	}

	ch, err := loader.Load(chartPath)
	if err != nil {
		fmt.Printf("error in loading: %v", err)
		return false, "", err
	}
	if req := ch.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(ch, req); err != nil {
			fmt.Printf("error in dependencies: %v", err)
			return false, "", err
		}
	}

	if ch.Metadata.Deprecated {
		fmt.Printf("WARNING: This chart is deprecated")
	}

	release, err := upgrade.Run(releaseName, ch, vals)
	if err != nil {
		fmt.Printf("error in installing chart: %v", err)
		return false, "", err
	}

	return true, release.Info.Status.String(), nil
}
