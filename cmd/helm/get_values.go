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
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm"
)

var getValuesHelp = `
This command downloads a values file for a given release.
`

type getValuesOptions struct {
	allValues bool // --all
	version   int  // --revision

	release string

	client helm.Interface
}

func newGetValuesCmd(client helm.Interface, out io.Writer) *cobra.Command {
	o := &getValuesOptions{client: client}

	cmd := &cobra.Command{
		Use:   "values RELEASE_NAME",
		Short: "download the values file for a named release",
		Long:  getValuesHelp,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.release = args[0]
			o.client = ensureHelmClient(o.client, false)
			return o.run(out)
		},
	}

	cmd.Flags().BoolVarP(&o.allValues, "all", "a", false, "dump all (computed) values")
	cmd.Flags().IntVar(&o.version, "revision", 0, "get the named release with revision")
	return cmd
}

// getValues implements 'helm get values'
func (o *getValuesOptions) run(out io.Writer) error {
	res, err := o.client.ReleaseContent(o.release, o.version)
	if err != nil {
		return err
	}

	// If the user wants all values, compute the values and return.
	if o.allValues {
		cfg, err := chartutil.CoalesceValues(res.Chart, res.Config)
		if err != nil {
			return err
		}
		cfgStr, err := cfg.YAML()
		if err != nil {
			return err
		}
		fmt.Fprintln(out, cfgStr)
		return nil
	}

	fmt.Fprintln(out, string(res.Config))
	return nil
}
