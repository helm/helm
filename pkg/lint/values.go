package lint

import (
	"os"
	"path/filepath"

	"k8s.io/helm/pkg/chartutil"
)

// Values lints a chart's values.yaml file.
func Values(basepath string) (messages []Message) {
	vf := filepath.Join(basepath, "values.yaml")
	messages = []Message{}
	if _, err := os.Stat(vf); err != nil {
		messages = append(messages, Message{Severity: InfoSev, Text: "No values.yaml file"})
		return
	}
	_, err := chartutil.ReadValuesFile(vf)
	if err != nil {
		messages = append(messages, Message{Severity: ErrorSev, Text: err.Error()})
	}
	return messages
}
