package action

import (
	"bytes"
	"fmt"
	"strings"

	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/registry"
)

type ShowOutputFormat string

const (
	ShowAll    ShowOutputFormat = "all"
	ShowChart  ShowOutputFormat = "chart"
	ShowValues ShowOutputFormat = "values"
	ShowReadme ShowOutputFormat = "readme"
	ShowCRDs   ShowOutputFormat = "crds"
)

var readmeFileNames = []string{"readme.md", "readme.txt", "readme"}

func (o ShowOutputFormat) String() string {
	return string(o)
}

type Show struct {
	ChartPathOptions
	Devel            bool
	OutputFormat     ShowOutputFormat
	JSONPathTemplate string
	chart            *chart.Chart
}

func NewShow(output ShowOutputFormat, cfg *Configuration) *Show {
	sh := &Show{
		OutputFormat: output,
	}
	sh.registryClient = cfg.RegistryClient
	return sh
}

func (s *Show) SetRegistryClient(client *registry.Client) {
	s.registryClient = client
}

func (s *Show) Run(chartpath string) (string, error) {
	if s.chart == nil {
		chrt, err := loader.Load(chartpath)
		if err != nil {
			return "", err
		}
		s.chart = chrt
	}

	var out strings.Builder

	// =========================
	// CHART METADATA (FIXED)
	// =========================
	if s.OutputFormat == ShowChart || s.OutputFormat == ShowAll {
		data := map[string]interface{}{
			"apiVersion":   s.chart.Metadata.APIVersion,
			"name":         s.chart.Metadata.Name,
			"version":      s.chart.Metadata.Version,
			"appVersion":   s.chart.Metadata.AppVersion,
			"description":  s.chart.Metadata.Description,
			"home":         s.chart.Metadata.Home,
			"sources":      s.chart.Metadata.Sources,
			"keywords":     s.chart.Metadata.Keywords,
			"maintainers":  s.chart.Metadata.Maintainers,
			"icon":         s.chart.Metadata.Icon,
			"condition":    s.chart.Metadata.Condition,
			"tags":         s.chart.Metadata.Tags,
			"deprecated":   s.chart.Metadata.Deprecated,
			"annotations":  s.chart.Metadata.Annotations,
			"kubeVersion":  s.chart.Metadata.KubeVersion,
			"dependencies": s.chart.Metadata.Dependencies,
			"type":         s.chart.Metadata.Type,
		}
		var buf bytes.Buffer
		encoder := yaml.NewEncoder(&buf)
		encoder.SetSortMapKeys(true) // Sorts annotations and any other maps
		err := encoder.Encode(data)
		encoder.Close()
		if err != nil {
			return "", fmt.Errorf("failed to marshal chart metadata: %w", err)
		}
		finalYAML := buf.Bytes()
		fmt.Fprintf(&out, "%s\n", finalYAML)
	}

	// =========================
	// VALUES
	// =========================
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

	// =========================
	// README
	// =========================
	if s.OutputFormat == ShowReadme || s.OutputFormat == ShowAll {
		readme := findReadme(s.chart.Files)
		if readme != nil {
			if s.OutputFormat == ShowAll {
				fmt.Fprintln(&out, "---")
			}
			fmt.Fprintf(&out, "%s\n", readme.Data)
		}
	}

	// =========================
	// CRDs
	// =========================
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

func findReadme(files []*common.File) *common.File {
	for _, f := range files {
		if f == nil {
			continue
		}
		for _, name := range readmeFileNames {
			if strings.EqualFold(f.Name, name) {
				return f
			}
		}
	}
	return nil
}