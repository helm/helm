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

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/output"
)

var getDeployedHelp = `
This command prints list of resources deployed under a release.

Example output:

  NAMESPACE     	NAME                     	API_VERSION	AGE
  thousand-sunny	services/zoro            	v1         	2m
                	namespaces/thousand-sunny	v1         	2m
  thousand-sunny	configmaps/nami          	v1         	2m
  thousand-sunny	deployments/luffy        	apps/v1    	2m
`

// newGetDeployedCmd creates a command for listing the resources deployed under a named release
func newGetDeployedCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	// Output format for the command output. This will be set by input flag -o (or --output).
	var outfmt output.Format

	// Create get-deployed action's client
	client := action.NewGetDeployed(cfg)

	cmd := &cobra.Command{
		Use:   "deployed RELEASE_NAME",
		Short: "list resources deployed under a named release",
		Long:  getDeployedHelp,
		Args:  require.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			return compListReleases(toComplete, args, cfg)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			// Run the client to list resources under the release
			resourceList, err := client.Run(args[0])
			if err != nil {
				return err
			}

			// Create an output writer with resources listed
			writer := action.NewResourceListWriter(resourceList, false)

			// Write the resources list with output format provided with input flag
			return outfmt.Write(out, writer)
		},
	}

	// Add flag for specifying the output format
	bindOutputFlag(cmd, &outfmt)

	return cmd
}
