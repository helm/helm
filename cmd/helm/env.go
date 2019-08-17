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
	"os"
	"strings"

	"helm.sh/helm/pkg/helmpath"
	"helm.sh/helm/pkg/helmpath/xdg"

	"helm.sh/helm/pkg/cli"

	"helm.sh/helm/pkg/plugin"

	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/require"
)

var (
	envHelp = `
Env prints out all the environment information in use by Helm.
`
)

func newEnvCmd(out io.Writer) *cobra.Command {
	o := &envOptions{}

	cmd := &cobra.Command{
		Use:   "env",
		Short: "environment information in use the Helm client",
		Long:  envHelp,
		Args:  require.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(out)
		},
	}

	return cmd
}

type envOptions struct {
}

// run .
func (o *envOptions) run(out io.Writer) error {
	plugin.SetupPluginEnv(&cli.EnvSettings{}, "", "")
	o.setXdgPaths()
	o.setGoPaths()
	for _, prefix := range []string{
		"HELM_",
		"XDG_",
		"GO",
	} {
		for _, e := range os.Environ() {
			if strings.HasPrefix(e, prefix) {
				fmt.Println(e)
			}
		}
	}
	return nil
}

func (o *envOptions) setXdgPaths() {
	for key, val := range map[string]string{
		xdg.CacheHomeEnvVar:  helmpath.CachePath(),
		xdg.ConfigHomeEnvVar: helmpath.ConfigPath(),
		xdg.DataHomeEnvVar:   helmpath.DataPath(),
	} {
		if eVal := os.Getenv(key); len(eVal) <= 0 {
			os.Setenv(key, val)
		}
	}
}

func (o *envOptions) setGoPaths() {
	for key, val := range map[string]string{
		"GOPATH": "~/go",
		"GOBIN":  "~/go/bin",
	} {
		if eVal := os.Getenv(key); len(eVal) <= 0 {
			os.Setenv(key, val)
		}
	}
}
