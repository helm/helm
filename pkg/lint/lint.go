package lint

import (
	"k8s.io/helm/pkg/lint/rules"
	"k8s.io/helm/pkg/lint/support"
	"os"
	"path/filepath"
)

// All runs all of the available linters on the given base directory.
func All(basedir string) []support.Message {
	// Using abs path to get directory context
	current, _ := os.Getwd()
	chartDir := filepath.Join(current, basedir)

	linter := support.Linter{ChartDir: chartDir}
	rules.Chartfile(&linter)
	rules.Values(&linter)
	rules.Templates(&linter)
	return linter.Messages
}
