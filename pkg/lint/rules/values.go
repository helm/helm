package rules

import (
	"fmt"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/lint/support"
	"os"
	"path/filepath"
)

// Values lints a chart's values.yaml file.
func Values(linter *support.Linter) {
	vf := filepath.Join(linter.ChartDir, "values.yaml")
	fileExists := linter.RunLinterRule(support.InfoSev, validateValuesFileExistence(linter, vf))

	if !fileExists {
		return
	}

	linter.RunLinterRule(support.ErrorSev, validateValuesFile(linter, vf))
}

func validateValuesFileExistence(linter *support.Linter, valuesPath string) (lintError support.LintError) {
	_, err := os.Stat(valuesPath)
	if err != nil {
		lintError = fmt.Errorf("values.yaml file does not exists")
	}
	return
}

func validateValuesFile(linter *support.Linter, valuesPath string) (lintError support.LintError) {
	_, err := chartutil.ReadValuesFile(valuesPath)
	if err != nil {
		lintError = fmt.Errorf("values.yaml is malformed: %s", err.Error())
	}
	return
}
