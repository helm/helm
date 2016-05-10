package lint

import (
	"os"
	"path/filepath"

	chartutil "github.com/kubernetes/helm/pkg/chart"
)

// Chartfile checks the Chart.yaml file for errors and warnings.
func Chartfile(basepath string) (m []Message) {
	m = []Message{}

	path := filepath.Join(basepath, "Chart.yaml")
	if fi, err := os.Stat(path); err != nil {
		m = append(m, Message{Severity: ErrorSev, Text: "No Chart.yaml file"})
		return
	} else if fi.IsDir() {
		m = append(m, Message{Severity: ErrorSev, Text: "Chart.yaml is a directory."})
		return
	}

	cf, err := chartutil.LoadChartfile(path)
	if err != nil {
		m = append(m, Message{
			Severity: ErrorSev,
			Text:     err.Error(),
		})
		return
	}

	if cf.Name == "" {
		m = append(m, Message{
			Severity: ErrorSev,
			Text:     "Chart.yaml: 'name' is required",
		})
	}

	if cf.Version == "" || cf.Version == "0.0.0" {
		m = append(m, Message{
			Severity: ErrorSev,
			Text:     "Chart.yaml: 'version' is required, and must be greater than 0.0.0",
		})
	}
	return
}
