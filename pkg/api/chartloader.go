package api

import (
	"helm.sh/helm/v3/pkg/cli"
)

type chartloader interface {
	LocateChart(name string, settings *cli.EnvSettings) (string, error)
}
