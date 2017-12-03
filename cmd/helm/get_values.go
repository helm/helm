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
	"encoding/json"
	"fmt"
	"github.com/ghodss/yaml"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm"
)

var getValuesHelp = `
This command downloads a values file for a given release.
`

type getValuesCmd struct {
	release   string
	allValues bool
	outfmt    string
	out       io.Writer
	client    helm.Interface
	version   int32
}

func newGetValuesCmd(client helm.Interface, out io.Writer) *cobra.Command {
	get := &getValuesCmd{
		out:    out,
		client: client,
	}
	cmd := &cobra.Command{
		Use:     "values [flags] RELEASE_NAME",
		Short:   "download the values file for a named release",
		Long:    getValuesHelp,
		PreRunE: setupConnection,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errReleaseRequired
			}
			get.release = args[0]
			get.client = ensureHelmClient(get.client)
			return get.run()
		},
	}

	cmd.Flags().Int32Var(&get.version, "revision", 0, "get the named release with revision")
	cmd.Flags().BoolVarP(&get.allValues, "all", "a", false, "dump all (computed) values")
	cmd.Flags().StringVarP(&get.outfmt, "output", "o", "yaml", "output the status in the specified format (json or yaml)")
	return cmd
}

func (g *getValuesCmd) printValues(vals string) error {
	switch g.outfmt {
	case "json":
		out := map[string]interface{}{}
		err := yaml.Unmarshal([]byte(vals), &out)
		if err != nil {
			return err
		}
		json, err := json.MarshalIndent(out, "", "    ")
		if err != nil {
			return err
		}
		fmt.Fprintln(g.out, string(json))

	default:
		fmt.Fprintln(g.out, vals)
	}
	return nil
}

// getValues implements 'helm get values'
func (g *getValuesCmd) run() error {
	res, err := g.client.ReleaseContent(g.release, helm.ContentReleaseVersion(g.version))
	if err != nil {
		return prettyError(err)
	}
	cfg := res.Release.Config.Raw
	// If the user wants all values, compute the values and return.
	if g.allValues {
		cfg, err := chartutil.CoalesceValues(res.Release.Chart, res.Release.Config)
		if err != nil {
			return err
		}
		cfgStr, err := cfg.YAML()
		if err != nil {
			return err
		}
		g.printValues(cfgStr)
		return nil
	}
	g.printValues(cfg)
	return nil
}
