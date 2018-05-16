/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/helm"
)

var getHelp = `
This command shows the details of a named release.

It can be used to get extended information about the release, including:

  - The values used to generate the release
  - The chart used to generate the release
  - The generated manifest file

By default, this prints a human readable collection of information about the
chart, the supplied values, and the generated manifest file.
`

type getOptions struct {
	version int // --revision

	release string

	client helm.Interface
}

func newGetCmd(client helm.Interface, out io.Writer) *cobra.Command {
	o := &getOptions{client: client}

	cmd := &cobra.Command{
		Use:   "get RELEASE_NAME",
		Short: "download a named release",
		Long:  getHelp,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.release = args[0]
			o.client = ensureHelmClient(o.client, false)
			return o.run(out)
		},
	}

	cmd.Flags().IntVar(&o.version, "revision", 0, "get the named release with revision")

	cmd.AddCommand(newGetValuesCmd(client, out))
	cmd.AddCommand(newGetManifestCmd(client, out))
	cmd.AddCommand(newGetHooksCmd(client, out))

	return cmd
}

func (g *getOptions) run(out io.Writer) error {
	res, err := g.client.ReleaseContent(g.release, g.version)
	if err != nil {
		return err
	}
	return printRelease(out, res)
}
