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
	"helm.sh/helm/v3/pkg/http/api/logger"
	"helm.sh/helm/v3/pkg/servercontext"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

type Service struct {
	settings     *cli.EnvSettings
	actionConfig *action.Configuration
	chartloader  *action.ChartPathOptions
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
	valueOpts := &values.Options{}
	//valueOpts.Values = append(valueOpts.Values, vals)
	//TODO: we need to make this as Provider, so it'll be able to merge
	return valueOpts.MergeValues(getter.All(servercontext.App().Config))
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
	install := action.NewInstall(s.actionConfig)
	install.Namespace = icfg.Namespace
	install.ReleaseName = icfg.Name

	release, err := install.Run(ch, vals)
	if err != nil {
		return nil, fmt.Errorf("error in installing chart: %v", err)
	}
	result := new(installResult)
	fmt.Println(result)
	if release.Info != nil {
		result.status = release.Info.Status.String()
	}
	return result, nil
}

func NewService(settings *cli.EnvSettings, actionConfig *action.Configuration) Service {
	return Service{
		settings:     settings,
		actionConfig: actionConfig,
		chartloader:  new(action.ChartPathOptions),
	}
}
