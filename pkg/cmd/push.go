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

package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cli/output"
	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/pusher"
	"helm.sh/helm/v4/pkg/registry"
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
	insecureSkipTLSVerify bool
	plainHTTP             bool
	password              string
	username              string
}

func newPushCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	o := &registryPushOptions{}
	var outfmt output.Format

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
						comps = append(comps, scheme+"://")
					}
				}
				return comps, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
			}
			return noMoreArgsComp()
		},
		RunE: func(_ *cobra.Command, args []string) error {
			// The registry client writes a human-readable push summary
			// directly to its writer. Suppress it for the machine-readable
			// output formats so that only the structured result is written.
			registryClientOut := out
			if outfmt != output.Table {
				registryClientOut = io.Discard
			}
			registryClient, err := newRegistryClient(
				registryClientOut, o.certFile, o.keyFile, o.caFile, o.insecureSkipTLSVerify, o.plainHTTP, o.username, o.password,
			)
			if err != nil {
				return fmt.Errorf("missing registry client: %w", err)
			}
			cfg.RegistryClient = registryClient
			chartRef := args[0]
			remote := args[1]
			var result *registry.PushResult
			client := action.NewPushWithOpts(action.WithPushConfig(cfg),
				action.WithTLSClientConfig(o.certFile, o.keyFile, o.caFile),
				action.WithInsecureSkipTLSVerify(o.insecureSkipTLSVerify),
				action.WithPlainHTTP(o.plainHTTP),
				action.WithPushOptWriter(out),
				action.WithPushResultHandler(func(r *registry.PushResult) {
					result = r
				}))
			client.Settings = settings
			uploadOutput, err := client.Run(chartRef, remote)
			if err != nil {
				return err
			}
			if outfmt == output.Table {
				fmt.Fprint(out, uploadOutput)
				return nil
			}
			if result == nil {
				return fmt.Errorf("no push result available to write as %s", outfmt)
			}
			return outfmt.Write(out, newPushWriter(result))
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.certFile, "cert-file", "", "identify registry client using this SSL certificate file")
	f.StringVar(&o.keyFile, "key-file", "", "identify registry client using this SSL key file")
	f.StringVar(&o.caFile, "ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")
	f.BoolVar(&o.insecureSkipTLSVerify, "insecure-skip-tls-verify", false, "skip tls certificate checks for the chart upload")
	f.BoolVar(&o.plainHTTP, "plain-http", false, "use insecure HTTP connections for the chart upload")
	f.StringVar(&o.username, "username", "", "chart repository username where to locate the requested chart")
	f.StringVar(&o.password, "password", "", "chart repository password where to locate the requested chart")

	bindOutputFlag(cmd, &outfmt)

	return cmd
}

// pushResult is the structure written by the push command for the
// machine-readable output formats.
type pushResult struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

type pushWriter struct {
	result pushResult
}

func newPushWriter(result *registry.PushResult) *pushWriter {
	w := &pushWriter{result: pushResult{Ref: result.Ref}}
	if result.Manifest != nil {
		w.result.Digest = result.Manifest.Digest
	}
	return w
}

// WriteTable mirrors the push summary the registry client writes for the
// default table output format.
func (w *pushWriter) WriteTable(out io.Writer) error {
	if _, err := fmt.Fprintf(out, "Pushed: %s\n", w.result.Ref); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "Digest: %s\n", w.result.Digest)
	return err
}

func (w *pushWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, w.result)
}

func (w *pushWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, w.result)
}
