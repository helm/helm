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
	"helm.sh/helm/v3/pkg/helmpath"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
)

const chartPushDesc = `
Upload a chart to a remote registry.

Note: the ref must already exist in the local registry cache.

Must first run "helm chart save" or "helm chart pull".
`

// Used if --check-signature flag is used
type signatureOptions struct {
	Sign        bool
	trustServer string
	trustDir    string
	caCert      string
	rootKey     string
}

func newChartPushCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	signOpts := &signatureOptions{}
	cmd := &cobra.Command{
		Use:    "push [ref]",
		Short:  "push a chart to remote",
		Long:   chartPushDesc,
		Args:   require.MinimumNArgs(1),
		Hidden: !FeatureGateOCI.IsEnabled(),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]

			if signOpts.Sign {
				err := action.NewChartSign(
					cfg,
					signOpts.trustServer,
					signOpts.trustDir,
					signOpts.caCert,
					signOpts.rootKey).Run(out, ref)
				if err != nil {
					return err
				}
			}

			return action.NewChartPush(cfg).Run(out, ref)
		},
	}
	td := filepath.Join(helmpath.ConfigPath(), ".trust")
	cmd.Flags().StringVarP(&signOpts.trustServer, "trust-server", "", "", "The trust server to use for signature verification")
	cmd.Flags().StringVarP(&signOpts.trustDir, "trust-dir", "", td, "Location where trust data is stored")
	cmd.Flags().StringVarP(&signOpts.rootKey, "root-key", "", "", "Root Key to initialize repository with")
	cmd.Flags().StringVarP(&signOpts.caCert, "ca-cert", "", "", "Trust certs signed only by this CA will be considered")
	cmd.Flags().BoolVarP(&signOpts.Sign, "sign", "", true, "Enable signature checking")

	return cmd
}
