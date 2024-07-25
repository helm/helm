package action

import (
    "os"
    "path/filepath"
    "strings"

    "github.com/pkg/errors"

    "helm.sh/helm/v3/pkg/chartutil"
    "helm.sh/helm/v3/pkg/lint"
    "helm.sh/helm/v3/pkg/lint/support"
)

// Lint is the action for checking that the semantics of a chart are well-formed.
type Lint struct {
    Strict        bool
    Namespace     string
    WithSubcharts bool
    Quiet         bool
    KubeVersion   *chartutil.KubeVersion
    IgnoreFilePath string
    Debug bool
}
type LintResult struct {
    TotalChartsLinted int
    Messages          []support.Message
    Errors            []error
}
func NewLint() *Lint {
    return &Lint{}
}
func (l *Lint) Run(paths []string, vals map[string]interface{}) *LintResult {
    lowestTolerance := support.ErrorSev
    if l.Strict {
        lowestTolerance = support.WarningSev
    }

    result := &LintResult{}
    for _, path := range paths {
        linter, err := lintChart(path, vals, l.Namespace, l.KubeVersion)
        if err != nil {
            result.Errors = append(result.Errors, err)
            continue
        }

        result.Messages = append(result.Messages, linter.Messages...)
        result.TotalChartsLinted++
        for _, msg := range linter.Messages {
            if msg.Severity >= lowestTolerance {
                result.Errors = append(result.Errors, msg.Err)
            }
        }
    }
    return result
}

func HasWarningsOrErrors(result *LintResult) bool {
	for _, msg := range result.Messages {
		if msg.Severity > support.InfoSev {
			return true
		}
	}
	return len(result.Errors) > 0
}

func lintChart(path string, vals map[string]interface{}, namespace string, kubeVersion *chartutil.KubeVersion) (support.Linter, error) {
    var chartPath string
    linter := support.Linter{}

    if strings.HasSuffix(path, ".tgz") || strings.HasSuffix(path, ".tar.gz") {
        tempDir, err := os.MkdirTemp("", "helm-lint")
        if err != nil {
            return linter, errors.Wrap(err, "unable to create temp dir to extract tarball")
        }
        defer os.RemoveAll(tempDir)

        file, err := os.Open(path)
        if err != nil {
            return linter, errors.Wrap(err, "unable to open tarball")
        }
        defer file.Close()

        if err = chartutil.Expand(tempDir, file); err != nil {
            return linter, errors.Wrap(err, "unable to extract tarball")
        }

        files, err := os.ReadDir(tempDir)
        if err != nil {
            return linter, errors.Wrapf(err, "unable to read temporary output directory %s", tempDir)
        }
        if !files[0].IsDir() {
            return linter, errors.Errorf("unexpected file %s in temporary output directory %s", files[0].Name(), tempDir)
        }

        chartPath = filepath.Join(tempDir, files[0].Name())
    } else {
        chartPath = path
    }

    if _, err := os.Stat(filepath.Join(chartPath, "Chart.yaml")); err != nil {
        return linter, errors.Wrap(err, "Chart.yaml file not found in chart")
    }
    return lint.AllWithKubeVersion(chartPath, vals, namespace, kubeVersion), nil
}
