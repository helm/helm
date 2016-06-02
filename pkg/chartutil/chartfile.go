package chartutil

import (
	"io/ioutil"

	"github.com/ghodss/yaml"
	"github.com/kubernetes/helm/pkg/proto/hapi/chart"
)

// UnmarshalChartfile takes raw Chart.yaml data and unmarshals it.
func UnmarshalChartfile(data []byte) (*chart.Metadata, error) {
	y := &chart.Metadata{}
	err := yaml.Unmarshal(data, y)
	if err != nil {
		return nil, err
	}
	return y, nil
}

// LoadChartfile loads a Chart.yaml file into a *chart.Metadata.
func LoadChartfile(filename string) (*chart.Metadata, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return UnmarshalChartfile(b)
}
