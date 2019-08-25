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
	"path/filepath"

	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/pkg/action"
	"helm.sh/helm/pkg/helmpath"
)

type signOpts struct {
	trustDir    string
	trustServer string
	tlscacert   string
	rootkey     string

	signature bool
}

const chartPushDesc = `
Upload a chart to a remote registry.

Note: the ref must already exist in the local registry cache.

Must first run "helm chart save" or "helm chart pull".
`

func newChartPushCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	opts := &signOpts{}
	cmd := &cobra.Command{
		Use:    "push [ref]",
		Short:  "push a chart to remote",
		Long:   chartPushDesc,
		Args:   require.MinimumNArgs(1),
		Hidden: !FeatureGateOCI.IsEnabled(),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]

			if opts.signature {
				err := action.NewChartSign(
					cfg, opts.trustDir,
					opts.trustServer,
					ref,
					opts.tlscacert,
					opts.rootkey).Run(out, ref)
				if err != nil {
					return err
				}
			}

			return action.NewChartPush(cfg).Run(out, ref)
		},
	}

	td := filepath.Join(helmpath.Registry(), "trust")
	cmd.Flags().StringVarP(&opts.trustDir, "trustdir", "", td, "Directory where the trust data is persisted to")
	cmd.Flags().StringVarP(&opts.trustServer, "server", "", "", "The trust server to use")
	cmd.Flags().StringVarP(&opts.tlscacert, "tlscacert", "", "", "Trust certs signed only by this CA")
	cmd.Flags().StringVarP(&opts.rootkey, "rootkey", "", "", "Root key to initialize the repository with")
	cmd.Flags().BoolVarP(&opts.signature, "signature", "", false, "Root key to initialize the repository with")

	return cmd
}
