/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package action

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
)

// ShowOutputFormat is the format of the output of `helm show`
type ShowOutputFormat string

const (
	// ShowAll is the format which shows all the information of a chart
	ShowAll ShowOutputFormat = "all"
	// ShowChart is the format which only shows the chart's definition
	ShowChart ShowOutputFormat = "chart"
	// ShowValues is the format which only shows the chart's values
	ShowValues ShowOutputFormat = "values"
	// ShowReadme is the format which only shows the chart's README
	ShowReadme ShowOutputFormat = "readme"
	// ShowCRDs is the format which only shows the chart's CRDs
	ShowCRDs ShowOutputFormat = "crds"
)

var readmeFileNames = []string{"readme.md", "readme.txt", "readme"}

func (o ShowOutputFormat) String() string {
	return string(o)
}

// Show is the action for checking a given release's information.
//
// It provides the implementation of 'helm show' and its respective subcommands.
type Show struct {
	ChartPathOptions
	Devel            bool
	OutputFormat     ShowOutputFormat
	JSONPathTemplate string
	chart            *chart.Chart // for testing
}

// NewShow creates a new Show object with the given configuration.
func NewShow(output ShowOutputFormat) *Show {
	return &Show{
		OutputFormat: output,
	}
}

// Run executes 'helm show' against the given release.
func (s *Show) Run(chartpath string) (string, error) {
	if s.chart == nil {
		chrt, err := loader.Load(chartpath)
		if err != nil {
			return "", err
		}
		s.chart = chrt
	}
	cf, err := yaml.Marshal(s.chart.Metadata)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	if s.OutputFormat == ShowChart || s.OutputFormat == ShowAll {
		fmt.Fprintf(&out, "%s\n", cf)
	}

	if (s.OutputFormat == ShowValues || s.OutputFormat == ShowAll) && s.chart.Values != nil {
		if s.OutputFormat == ShowAll {
			fmt.Fprintln(&out, "---")
		}
		if s.JSONPathTemplate != "" {
			printer, err := printers.NewJSONPathPrinter(s.JSONPathTemplate)
			if err != nil {
				return "", errors.Wrapf(err, "error parsing jsonpath %s", s.JSONPathTemplate)
			}
			printer.Execute(&out, s.chart.Values)
		} else {
			for _, f := range s.chart.Raw {
				if f.Name == chartutil.ValuesfileName {
					fmt.Fprintln(&out, string(f.Data))
				}
			}
		}
	}

	if s.OutputFormat == ShowReadme || s.OutputFormat == ShowAll {
		readme := findReadme(s.chart.Files)
		if readme != nil {
			if s.OutputFormat == ShowAll {
				fmt.Fprintln(&out, "---")
			}
			fmt.Fprintf(&out, "%s\n", readme.Data)
		}
	}

	if s.OutputFormat == ShowCRDs || s.OutputFormat == ShowAll {
		crds := s.chart.CRDObjects()
		if len(crds) > 0 {
			if s.OutputFormat == ShowAll && !bytes.HasPrefix(crds[0].File.Data, []byte("---")) {
				fmt.Fprintln(&out, "---")
			}
			for _, crd := range crds {
				fmt.Fprintf(&out, "%s\n", string(crd.File.Data))
			}
		}
	}
	return out.String(), nil
}

func findReadme(files []*chart.File) (file *chart.File) {
	for _, file := range files {
		for _, n := range readmeFileNames {
			if strings.EqualFold(file.Name, n) {
				return file
			}
		}
	}
	return nil
}
