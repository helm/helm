package api

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"helm.sh/helm/v3/pkg/action"

	"helm.sh/helm/v3/pkg/api/logger"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/storage/driver"
)

type upgrader interface {
	SetConfig(ReleaseConfig)
	GetInstall() bool
	upgraderunner
}

type history interface {
	historyrunner
}

type Service struct {
	settings *cli.EnvSettings
	chartloader
	ListRunner
	InstallRunner
	upgrader
	history
}

type ReleaseConfig struct {
	Name      string
	Namespace string
	ChartName string
}

type ChartValues map[string]interface{}

type ReleaseResult struct {
	Status string
}

func (s Service) getValues(vals ChartValues) (ChartValues, error) {
	//	valueOpts := &values.Options{}
	//valueOpts.Values = append(valueOpts.Values, vals)
	//TODO: we need to make this as Provider, so it'll be able to merge
	// why do we need getter.ALl?
	return vals, nil
}

func (s Service) Install(ctx context.Context, cfg ReleaseConfig, values ChartValues) (*ReleaseResult, error) {
	if err := s.validate(cfg, values); err != nil {
		return nil, fmt.Errorf("error request validation: %v", err)
	}
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

func (s Service) Upgrade(ctx context.Context, cfg ReleaseConfig, values ChartValues) (*ReleaseResult, error) {
	if err := s.validate(cfg, values); err != nil {
		return nil, fmt.Errorf("error request validation: %v", err)
	}
	chart, err := s.loadChart(cfg.ChartName)
	if err != nil {
		return nil, err
	}
	vals, err := s.getValues(values)
	if err != nil {
		return nil, fmt.Errorf("error merging values: %v", err)
	}

	if s.upgrader.GetInstall() {
		if _, err := s.history.Run(cfg.Name); err == driver.ErrReleaseNotFound {
			logger.Debugf("release %q does not exist. Installing it now.\n", cfg.Name)
			return s.installChart(cfg, chart, vals)
		}
	}

	return s.upgradeRelease(cfg, chart, vals)
}

func (s Service) loadChart(chartName string) (*chart.Chart, error) {
	logger.Debugf("[install/upgrade] chart name: %s", chartName)
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

func (s Service) installChart(icfg ReleaseConfig, ch *chart.Chart, vals ChartValues) (*ReleaseResult, error) {
	s.InstallRunner.SetConfig(icfg)
	release, err := s.InstallRunner.Run(ch, vals)
	if err != nil {
		return nil, fmt.Errorf("error in installing chart: %v", err)
	}
	result := new(ReleaseResult)
	if release.Info != nil {
		result.Status = release.Info.Status.String()
	}
	return result, nil
}

func (s Service) upgradeRelease(ucfg ReleaseConfig, ch *chart.Chart, vals ChartValues) (*ReleaseResult, error) {
	s.upgrader.SetConfig(ucfg)
	release, err := s.upgrader.Run(ucfg.Name, ch, vals)
	if err != nil {
		return nil, fmt.Errorf("error in upgrading chart: %v", err)
	}
	result := new(ReleaseResult)
	if release.Info != nil {
		result.Status = release.Info.Status.String()
	}
	return result, nil
}

func (s Service) validate(icfg ReleaseConfig, values ChartValues) error {
	if strings.HasPrefix(icfg.ChartName, ".") ||
		strings.HasPrefix(icfg.ChartName, "/") {
		return errors.New("cannot refer local chart")
	}
	return nil
}

func (s Service) List(releaseStatus string) ([]Release, error) {
	listStates := new(action.ListStates)

	state := action.ListAll
	if releaseStatus != "" {
		state = listStates.FromName(releaseStatus)
	}

	if state == action.ListUnknown {
		return nil, errors.New("invalid release status")
	}

	s.ListRunner.SetState(state)
	s.ListRunner.SetStateMask()
	releases, err := s.ListRunner.Run()
	if err != nil {
		return nil, err
	}

	var helmReleases []Release
	for _, r := range releases {
		helmRelease := Release{Name: r.Name,
			Namespace:  r.Namespace,
			Revision:   r.Version,
			Updated:    r.Info.LastDeployed,
			Status:     r.Info.Status,
			Chart:      fmt.Sprintf("%s-%s", r.Chart.Metadata.Name, r.Chart.Metadata.Version),
			AppVersion: r.Chart.Metadata.AppVersion,
		}
		helmReleases = append(helmReleases, helmRelease)
	}

	return helmReleases, nil
}

func NewService(settings *cli.EnvSettings, cl chartloader, l ListRunner, i InstallRunner, u upgrader, h history) Service {
	return Service{settings, cl, l, i, u, h}
}
