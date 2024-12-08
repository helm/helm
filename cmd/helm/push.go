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

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/pusher"
)

const pushDesc = `
Upload a chart to a registry.

If the chart has an associated provenance file,
it will also be uploaded.
`

type registryPushOptions struct {
	certFile              string
	keyFile               string
	caFile                string
	insecureSkipTLSverify bool
	plainHTTP             bool
	password              string
	username              string
}

func newPushCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	o := &registryPushOptions{}

	cmd := &cobra.Command{
		Use:   "push [chart] [remote]",
		Short: "push a chart to remote",
		Long:  pushDesc,
		Args:  require.MinimumNArgs(2),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// Do file completion for the chart file to push
				return nil, cobra.ShellCompDirectiveDefault
			}
			if len(args) == 1 {
				providers := []pusher.Provider(pusher.All(settings))
				var comps []string
				for _, p := range providers {
					for _, scheme := range p.Schemes {
						comps = append(comps, fmt.Sprintf("%s://", scheme))
					}
				}
				return comps, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
			}
			return noMoreArgsComp()
		},
		RunE: func(_ *cobra.Command, args []string) error {
			registryClient, err := newRegistryClient(
				o.certFile, o.keyFile, o.caFile, o.insecureSkipTLSverify, o.plainHTTP, o.username, o.password,
			)

			if err != nil {
				return fmt.Errorf("missing registry client: %w", err)
			}
			cfg.RegistryClient = registryClient
			chartRef := args[0]
			remote := args[1]
			client := action.NewPushWithOpts(action.WithPushConfig(cfg),
				action.WithTLSClientConfig(o.certFile, o.keyFile, o.caFile),
				action.WithInsecureSkipTLSVerify(o.insecureSkipTLSverify),
				action.WithPlainHTTP(o.plainHTTP),
				action.WithPushOptWriter(out))
			client.Settings = settings
			output, err := client.Run(chartRef, remote)
			if err != nil {
				return err
			}
			fmt.Fprint(out, output)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.certFile, "cert-file", "", "identify registry client using this SSL certificate file")
	f.StringVar(&o.keyFile, "key-file", "", "identify registry client using this SSL key file")
	f.StringVar(&o.caFile, "ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")
	f.BoolVar(&o.insecureSkipTLSverify, "insecure-skip-tls-verify", false, "skip tls certificate checks for the chart upload")
	f.BoolVar(&o.plainHTTP, "plain-http", false, "use insecure HTTP connections for the chart upload")
	f.StringVar(&o.username, "username", "", "chart repository username where to locate the requested chart")
	f.StringVar(&o.password, "password", "", "chart repository password where to locate the requested chart")

	return cmd
}
