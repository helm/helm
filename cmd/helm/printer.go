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

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"text/template"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/timeconv"
)

type outputFormat string

const (
	outputFlag = "output"

	outputTable outputFormat = "table"
	outputJSON  outputFormat = "json"
	outputYAML  outputFormat = "yaml"
)

var printReleaseTemplate = `REVISION: {{.Release.Version}}
RELEASED: {{.ReleaseDate}}
CHART: {{.Release.Chart.Metadata.Name}}-{{.Release.Chart.Metadata.Version}}
USER-SUPPLIED VALUES:
{{.Release.Config.Raw}}
COMPUTED VALUES:
{{.ComputedValues}}
HOOKS:
{{- range .Release.Hooks }}
---
# {{.Name}}
{{.Manifest}}
{{- end }}
MANIFEST:
{{.Release.Manifest}}
`

func printRelease(out io.Writer, rel *release.Release) error {
	if rel == nil {
		return nil
	}

	cfg, err := chartutil.CoalesceValues(rel.Chart, rel.Config)
	if err != nil {
		return err
	}
	cfgStr, err := cfg.YAML()
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"Release":        rel,
		"ComputedValues": cfgStr,
		"ReleaseDate":    timeconv.Format(rel.Info.LastDeployed, time.ANSIC),
	}
	return tpl(printReleaseTemplate, data, out)
}

func tpl(t string, vals interface{}, out io.Writer) error {
	tt, err := template.New("_").Parse(t)
	if err != nil {
		return err
	}
	return tt.Execute(out, vals)
}

func debug(format string, args ...interface{}) {
	if settings.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		fmt.Printf(format, args...)
	}
}

// bindOutputFlag will add the output flag to the given command and bind the
// value to the given string pointer
func bindOutputFlag(cmd *cobra.Command, varRef *string) {
	cmd.Flags().StringVarP(varRef, outputFlag, "o", string(outputTable), fmt.Sprintf("Prints the output in the specified format. Allowed values: %s, %s, %s", outputTable, outputJSON, outputYAML))
}

type outputWriter interface {
	WriteTable(out io.Writer) error
	WriteJSON(out io.Writer) error
	WriteYAML(out io.Writer) error
}

func write(out io.Writer, ow outputWriter, format outputFormat) error {
	switch format {
	case outputTable:
		return ow.WriteTable(out)
	case outputJSON:
		return ow.WriteJSON(out)
	case outputYAML:
		return ow.WriteYAML(out)
	}
	return fmt.Errorf("unsupported format %s", format)
}

// encodeJSON is a helper function to decorate any error message with a bit more
// context and avoid writing the same code over and over for printers
func encodeJSON(out io.Writer, obj interface{}) error {
	enc := json.NewEncoder(out)
	err := enc.Encode(obj)
	if err != nil {
		return fmt.Errorf("unable to write JSON output: %s", err)
	}
	return nil
}

// encodeYAML is a helper function to decorate any error message with a bit more
// context and avoid writing the same code over and over for printers
func encodeYAML(out io.Writer, obj interface{}) error {
	raw, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("unable to write YAML output: %s", err)
	}
	// Append a newline, as with a JSON encoder
	raw = append(raw, []byte("\n")...)
	_, err = out.Write(raw)
	if err != nil {
		return fmt.Errorf("unable to write YAML output: %s", err)
	}
	return nil
}

// encodeTable is a helper function to decorate any error message with a bit
// more context and avoid writing the same code over and over for printers
func encodeTable(out io.Writer, table *uitable.Table) error {
	raw := table.Bytes()
	raw = append(raw, []byte("\n")...)
	_, err := out.Write(raw)
	if err != nil {
		return fmt.Errorf("unable to write table output: %s", err)
	}
	return nil
}
