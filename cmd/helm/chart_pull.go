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
	"path/filepath"

	"helm.sh/helm/internal/experimental/registry"

	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/internal/experimental/tuf"
	"helm.sh/helm/pkg/action"
	"helm.sh/helm/pkg/helmpath"
)

const chartPullDesc = `
Download a chart from a remote registry.

This will store the chart in the local registry cache to be used later.
`

func newChartPullCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	opts := &signOpts{}
	cmd := &cobra.Command{
		Use:    "pull [ref]",
		Short:  "pull a chart from remote",
		Long:   chartPullDesc,
		Args:   require.MinimumNArgs(1),
		Hidden: !FeatureGateOCI.IsEnabled(),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			err := action.NewChartPull(cfg).Run(out, ref)
			if err != nil {
				return err
			}

			if opts.signature {
				sha, err := tuf.GetSHA(opts.trustDir, opts.trustServer, ref, opts.tlscacert, opts.rootkey)
				if err != nil {
					return err
				}

				r, err := registry.ParseReference(ref)
				if err != nil {
					return err
				}

				c, err := registry.NewCache(
					registry.CacheOptWriter(out),
					registry.CacheOptRoot(filepath.Join(helmpath.Registry(), registry.CacheRootDir)))

				cs, err := c.FetchReference(r)
				if err != nil {
					return err
				}

				if cs.Digest.Hex() != sha {
					fmt.Fprintf(out, "digests do not match: %v and %v", cs.Digest.Hex(), sha)
					_, err = c.DeleteReference(r)
					return err
				}
			}
			return nil
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
