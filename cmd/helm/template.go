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
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/pkg/action"
)

const templateDesc = `
Render chart templates locally and display the output.

This does not require Helm. However, any values that would normally be
looked up or retrieved in-cluster will be faked locally. Additionally, none
of the server-side testing of chart validity (e.g. whether an API is supported)
is done.
`

func newTemplateCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	var validate bool
	client := action.NewInstall(cfg)

	cmd := &cobra.Command{
		Use:   "template [NAME] [CHART]",
		Short: fmt.Sprintf("locally render templates"),
		Long:  templateDesc,
		Args:  require.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			client.DryRun = true
			client.ReleaseName = "RELEASE-NAME"
			client.Replace = true // Skip the name check
			client.ClientOnly = !validate
			rel, err := runInstall(args, client, out)
			if err != nil {
				return err
			}
			fmt.Fprintln(out, strings.TrimSpace(rel.Manifest))
			return nil
		},
	}

	f := cmd.Flags()
	addInstallFlags(f, client)
	f.StringVar(&client.OutputDir, "output-dir", "", "writes the executed templates to files in output-dir instead of stdout")
	f.BoolVar(&validate, "validate", false, "establish a connection to Kubernetes for schema validation")

	return cmd
}
