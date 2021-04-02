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
	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/action"
)

const chartPushDesc = `
Upload a chart to a remote registry.

Note: the ref must already exist in the local registry cache.

Must first run "helm chart save" or "helm chart pull".
`

func newChartPushCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	var insecureOpt, plainHTTPOpt bool
	var caFile, certFile, keyFile string
	cmd := &cobra.Command{
		Use:    "push [ref]",
		Short:  "push a chart to remote",
		Long:   chartPushDesc,
		Args:   require.MinimumNArgs(1),
		Hidden: !FeatureGateOCI.IsEnabled(),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			registryClient, err := registry.NewClient(
				registry.ClientOptDebug(settings.Debug),
				registry.ClientOptWriter(out),
				registry.ClientOptCredentialsFile(settings.RegistryConfig),
				registry.ClientOptPlainHTTP(plainHTTPOpt),
				registry.ClientOptInsecureSkipVerifyTLS(insecureOpt),
				registry.ClientOptCAFile(caFile),
				registry.ClientOptCertKeyFiles(certFile, keyFile),
			)
			if err != nil {
				return err
			}

			cfg.RegistryClient = registryClient
			return action.NewChartPush(cfg).Run(out, ref)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&insecureOpt, "insecure-skip-tls-verify", "", false, "skip registry tls certificate checks")
	f.BoolVarP(&plainHTTPOpt, "plain-http", "", false, "use plain http to connect to the registry instead of https")
	f.StringVar(&certFile, "cert-file", "", "identify HTTPS client using this SSL certificate file")
	f.StringVar(&keyFile, "key-file", "", "identify HTTPS client using this SSL key file")
	f.StringVar(&caFile, "ca-file", "", "verify certificates of HTTPS-enabled registry using this CA bundle")
	return cmd
}
