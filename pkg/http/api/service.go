package api

import (
	"context"
	"fmt"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/servercontext"
)

type Service struct {
	config  *cli.EnvSettings
	install *action.Install
}

type chartValues map[string]interface{}

type installResult struct {
	status string
}

func (s Service) getValues(vals chartValues) (chartValues, error) {
	valueOpts := &values.Options{}
	//valueOpts.Values = append(valueOpts.Values, vals)
	return valueOpts.MergeValues(getter.All(servercontext.App().Config))
}

func (s Service) Install(ctx context.Context, chartName string, values chartValues) (installResult, error) {
	var result installResult
	chart, err := s.loadChart(chartName)
	if err != nil {
		return result, err
	}
	vals, err := s.getValues(values)
	if err != nil {
		return result, fmt.Errorf("error merging values: %v", err)
	}
	release, err := s.install.Run(chart, vals)
	if err != nil {
		return result, fmt.Errorf("error in installing chart: %v", err)
	}
	if release.Info != nil {
		result.status = release.Info.Status.String()
	}
	return result, nil
}

func (s Service) loadChart(chartName string) (*chart.Chart, error) {
	cp, err := s.install.ChartPathOptions.LocateChart(chartName, s.config)
	if err != nil {
		return nil, fmt.Errorf("error in locating chart: %v", err)
	}
	var requestedChart *chart.Chart
	if requestedChart, err = loader.Load(cp); err != nil {
		return nil, fmt.Errorf("error loading chart: %v", err)
	}
	return requestedChart, nil
}

func NewService(cfg *cli.EnvSettings) Service {
	return Service{
		config: cfg,
		//TODO: not sure why this's needed, but we can refactor later,could be passed as param
		install: action.NewInstall(servercontext.App().ActionConfig),
	}
}
