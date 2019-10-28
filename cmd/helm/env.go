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

	"helm.sh/helm/v3/pkg/cli"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
)

var (
	envHelp = `
Env prints out all the environment information in use by Helm.
`
)

func newEnvCmd(out io.Writer) *cobra.Command {
	o := &envOptions{}
	o.settings = cli.New()

	cmd := &cobra.Command{
		Use:   "env",
		Short: "Helm client environment information",
		Long:  envHelp,
		Args:  require.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(out)
		},
	}

	return cmd
}

type envOptions struct {
	settings *cli.EnvSettings
}

func (o *envOptions) run(out io.Writer) error {
	for k, v := range o.settings.EnvVars() {
		fmt.Printf("%s=\"%s\"\n", k, v)
	}
	return nil
}
