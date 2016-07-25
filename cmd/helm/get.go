/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"errors"
	"io"
	"text/template"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/timeconv"
)

var getHelp = `
This command shows the details of a named release.

It can be used to get extended information about the release, including:

  - The values used to generate the release
  - The chart used to generate the release
  - The generated manifest file

By default, this prints a human readable collection of information about the
chart, the supplied values, and the generated manifest file.
`

var errReleaseRequired = errors.New("release name is required")

type getCmd struct {
	release string
	out     io.Writer
	client  helm.Interface
}

func newGetCmd(client helm.Interface, out io.Writer) *cobra.Command {
	get := &getCmd{
		out:    out,
		client: client,
	}
	cmd := &cobra.Command{
		Use:               "get [flags] RELEASE_NAME",
		Short:             "download a named release",
		Long:              getHelp,
		PersistentPreRunE: setupConnection,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errReleaseRequired
			}
			get.release = args[0]
			if get.client == nil {
				get.client = helm.NewClient(helm.Host(tillerHost))
			}
			return get.run()
		},
	}
	cmd.AddCommand(newGetValuesCmd(nil, out))
	cmd.AddCommand(newGetManifestCmd(nil, out))
	cmd.AddCommand(newGetHooksCmd(nil, out))
	return cmd
}

var getTemplate = `VERSION: {{.Release.Version}}
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

// getCmd is the command that implements 'helm get'
func (g *getCmd) run() error {
	res, err := g.client.ReleaseContent(g.release)
	if err != nil {
		return prettyError(err)
	}

	cfg, err := chartutil.CoalesceValues(res.Release.Chart, res.Release.Config)
	if err != nil {
		return err
	}
	cfgStr, err := cfg.YAML()
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"Release":        res.Release,
		"ComputedValues": cfgStr,
		"ReleaseDate":    timeconv.Format(res.Release.Info.LastDeployed, time.ANSIC),
	}
	return tpl(getTemplate, data, g.out)
}

func tpl(t string, vals map[string]interface{}, out io.Writer) error {
	tt, err := template.New("_").Parse(t)
	if err != nil {
		return err
	}
	return tt.Execute(out, vals)
}

func ensureHelmClient(h helm.Interface) helm.Interface {
	if h != nil {
		return h
	}
	return helm.NewClient(helm.Host(tillerHost))
}
