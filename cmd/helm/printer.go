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
	"io"
	"text/template"
	"time"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
)

var printReleaseTemplate = `REVISION: {{.Release.Version}}
RELEASED: {{.ReleaseDate}}
CHART: {{.Release.Chart.Metadata.Name}}-{{.Release.Chart.Metadata.Version}}
USER-SUPPLIED VALUES:
{{.Config}}
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
	computed, err := cfg.YAML()
	if err != nil {
		return err
	}

	config, err := yaml.Marshal(rel.Config)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"Release":        rel,
		"Config":         string(config),
		"ComputedValues": computed,
		"ReleaseDate":    rel.Info.LastDeployed.Format(time.ANSIC),
	}
	return tpl(printReleaseTemplate, data, out)
}

func tpl(t string, vals map[string]interface{}, out io.Writer) error {
	tt, err := template.New("_").Parse(t)
	if err != nil {
		return err
	}
	return tt.Execute(out, vals)
}
