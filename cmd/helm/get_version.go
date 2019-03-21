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

	"k8s.io/helm/pkg/helm"
)

var getVersionHelp = "This command fetches the chart version for a given release."

type getVersionCmd struct {
	release  string
	out      io.Writer
	client   helm.Interface
	revision int32
}

func newGetVersionCmd(client helm.Interface, out io.Writer) *cobra.Command {
	get := &getVersionCmd{
		out:    out,
		client: client,
	}
	cmd := &cobra.Command{
		Use:     "version [flags] RELEASE_NAME",
		Short:   "download the chart version for a named release",
		Long:    getVersionHelp,
		PreRunE: func(_ *cobra.Command, _ []string) error { return setupConnection() },
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errReleaseRequired
			}
			get.release = args[0]
			get.client = ensureHelmClient(get.client)
			return get.run()
		},
	}

	f := cmd.Flags()
	settings.AddFlagsTLS(f)
	f.Int32Var(&get.revision, "revision", 0, "get the named release with revision")

	// set defaults from environment
	settings.InitTLS(f)

	return cmd
}

// getVersion implements 'helm get version'
func (g *getVersionCmd) run() error {
	res, err := g.client.ReleaseContent(g.release, helm.ContentReleaseVersion(g.revision))
	if err != nil {
		return prettyError(err)
	}
	fmt.Fprintln(g.out, res.Release.Chart.Metadata.Version)
	return nil
}
