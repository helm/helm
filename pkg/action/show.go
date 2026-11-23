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
	"errors"
	"fmt"
	"net/url"
	"strings"

	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/registry"
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
	// IncludeOCIAnnotations includes the annotations of the OCI manifest in the
	// output of 'show chart' and 'show all' when the chart is pulled from an
	// OCI registry. See LoadOCIAnnotations.
	IncludeOCIAnnotations bool
	chart                 *chart.Chart // for testing
	ociAnnotations        map[string]string
}

// NewShow creates a new Show object with the given configuration.
func NewShow(output ShowOutputFormat, cfg *Configuration) *Show {
	sh := &Show{
		OutputFormat: output,
	}
	sh.registryClient = cfg.RegistryClient

	return sh
}

// SetRegistryClient sets the registry client to use when pulling a chart from a registry.
func (s *Show) SetRegistryClient(client *registry.Client) {
	s.registryClient = client
}

// LoadOCIAnnotations fetches the annotations of the OCI manifest for the chart
// referenced by ref so that Run can include them in its output. It is a no-op
// unless IncludeOCIAnnotations is set and ref is an OCI reference. The
// reference is resolved with the same version rules used to locate the chart,
// and only the manifest is fetched; the chart layers are not downloaded again.
func (s *Show) LoadOCIAnnotations(ref string) error {
	if !s.IncludeOCIAnnotations || !registry.IsOCI(ref) {
		return nil
	}
	if s.registryClient == nil {
		return errors.New("unable to fetch OCI annotations, missing registry client")
	}

	u, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return fmt.Errorf("invalid chart URL format: %s", ref)
	}
	_, u, err = s.registryClient.ValidateReference(ref, strings.TrimSpace(s.Version), u)
	if err != nil {
		return err
	}

	annotations, err := s.registryClient.ManifestAnnotations(strings.TrimPrefix(u.String(), fmt.Sprintf("%s://", registry.OCIScheme)))
	if err != nil {
		return fmt.Errorf("unable to fetch OCI annotations for %s: %w", ref, err)
	}
	s.ociAnnotations = annotations
	return nil
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
		out.Write(cf)
		if len(s.ociAnnotations) > 0 {
			oa, err := yaml.Marshal(map[string]map[string]string{"ociAnnotations": s.ociAnnotations})
			if err != nil {
				return "", err
			}
			out.Write(oa)
		}
		out.WriteString("\n")
	}

	if (s.OutputFormat == ShowValues || s.OutputFormat == ShowAll) && s.chart.Values != nil {
		if s.OutputFormat == ShowAll {
			fmt.Fprintln(&out, "---")
		}
		if s.JSONPathTemplate != "" {
			printer, err := printers.NewJSONPathPrinter(s.JSONPathTemplate)
			if err != nil {
				return "", fmt.Errorf("error parsing jsonpath %s: %w", s.JSONPathTemplate, err)
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
			for _, crd := range crds {
				if !bytes.HasPrefix(crd.File.Data, []byte("---")) {
					fmt.Fprintln(&out, "---")
				}
				fmt.Fprintf(&out, "%s\n", string(crd.File.Data))
			}
		}
	}
	return out.String(), nil
}

func findReadme(files []*common.File) (file *common.File) {
	for _, file := range files {
		for _, n := range readmeFileNames {
			if file == nil {
				continue
			}
			if strings.EqualFold(file.Name, n) {
				return file
			}
		}
	}
	return nil
}
