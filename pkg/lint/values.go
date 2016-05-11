package lint

import (
	"os"

	"github.com/kubernetes/helm/pkg/chart"
	"path/filepath"
)

// Values lints a chart's values.toml file.
func Values(basepath string) (messages []Message) {
	vf := filepath.Join(basepath, "values.toml")
	messages = []Message{}
	if _, err := os.Stat(vf); err != nil {
		messages = append(messages, Message{Severity: InfoSev, Text: "No values.toml file"})
		return
	}
	_, err := chart.ReadValuesFile(vf)
	if err != nil {
		messages = append(messages, Message{Severity: ErrorSev, Text: err.Error()})
	}
	return messages
}
