package api

import (
	"context"
	"fmt"

	"helm.sh/helm/v3/pkg/api/logger"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

type Service struct {
	settings *cli.EnvSettings
	Installer
	lister
	chartloader
}

type InstallConfig struct {
	Name      string
	Namespace string
	ChartName string
}

type chartValues map[string]interface{}

type installResult struct {
	status string
}

func (s Service) getValues(vals chartValues) (chartValues, error) {
	//	valueOpts := &values.Options{}
	//valueOpts.Values = append(valueOpts.Values, vals)
	//TODO: we need to make this as Provider, so it'll be able to merge
	// why do we need getter.ALl?
	return vals, nil
}

func (s Service) Install(ctx context.Context, cfg InstallConfig, values chartValues) (*installResult, error) {
	chart, err := s.loadChart(cfg.ChartName)
	if err != nil {
		return nil, err
	}
	vals, err := s.getValues(values)
	if err != nil {
		return nil, fmt.Errorf("error merging values: %v", err)
	}
	return s.installChart(cfg, chart, vals)
}

func (s Service) loadChart(chartName string) (*chart.Chart, error) {
	logger.Debugf("[Install] chart name: %s", chartName)
	cp, err := s.chartloader.LocateChart(chartName, s.settings)
	if err != nil {
		return nil, fmt.Errorf("error in locating chart: %v", err)
	}
	var requestedChart *chart.Chart
	if requestedChart, err = loader.Load(cp); err != nil {
		return nil, fmt.Errorf("error loading chart: %v", err)
	}
	return requestedChart, nil
}

func (s Service) installChart(icfg InstallConfig, ch *chart.Chart, vals chartValues) (*installResult, error) {
	s.Installer.SetConfig(icfg)
	release, err := s.Installer.Run(ch, vals)
	if err != nil {
		return nil, fmt.Errorf("error in installing chart: %v", err)
	}
	result := new(installResult)
	if release.Info != nil {
		result.status = release.Info.Status.String()
	}
	return result, nil
}

func NewService(settings *cli.EnvSettings, cl chartloader, i Installer, l lister) Service {
	return Service{
		settings,
		i,
		l,
		cl,
	}
}
