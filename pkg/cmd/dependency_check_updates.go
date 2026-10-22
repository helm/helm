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
	"path/filepath"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cli/output"
	"helm.sh/helm/v4/pkg/downloader"
	"helm.sh/helm/v4/pkg/getter"
)

const dependencyCheckUpdatesDesc = `
Compare locked dependency versions with available versions for one or more charts.

COMPATIBLE is the newest version allowed by Chart.yaml. LATEST is the newest
stable version, even if it is outside that constraint. A prerelease is considered
for COMPATIBLE only when the constraint explicitly allows it. Empty version
fields mean that no locked or matching version is available.

This command refreshes repository metadata unless --skip-refresh is set. It does
not download chart archives or modify Chart.yaml, Chart.lock, or charts/.
OCI dependencies always query registry tags; --skip-refresh applies to cached
chart repository indexes only. Local dependencies report the local chart version.

Examples:
  helm dependency check-updates .
  helm dependency check-updates ./frontend ./backend -o json
`

func newDependencyCheckUpdatesCmd(out io.Writer) *cobra.Command {
	client := action.NewDependency()
	var format output.Format
	cmd := &cobra.Command{
		Use:   "check-updates [CHART...]",
		Short: "check available chart dependency versions without updating charts",
		Long:  dependencyCheckUpdatesDesc,
		RunE: func(cmd *cobra.Command, paths []string) error {
			if len(paths) == 0 {
				paths = []string{"."}
			}
			registryClient, err := newRegistryClient(cmd.ErrOrStderr(), client.CertFile, client.KeyFile, client.CaFile,
				client.InsecureSkipTLSVerify, client.PlainHTTP, client.Username, client.Password)
			if err != nil {
				return err
			}
			rows := make(dependencyUpdatesWriter, 0)
			for _, path := range paths {
				manager := downloader.Manager{
					Out:              cmd.ErrOrStderr(),
					ChartPath:        filepath.Clean(path),
					SkipUpdate:       client.SkipRefresh,
					Getters:          getter.All(settings),
					RegistryClient:   registryClient,
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
					Debug:            settings.Debug,
				}
				updates, err := manager.CheckUpdates()
				if err != nil {
					return fmt.Errorf("check updates for %s: %w", path, err)
				}
				rows = append(rows, updates...)
			}
			return format.Write(out, rows)
		},
	}
	cmd.Flags().BoolVar(&client.SkipRefresh, "skip-refresh", false, "do not refresh cached chart repository indexes")
	cmd.Flags().StringVar(&client.Username, "username", "", "chart repository username")
	cmd.Flags().StringVar(&client.Password, "password", "", "chart repository password")
	cmd.Flags().StringVar(&client.CertFile, "cert-file", "", "identify HTTPS client using this SSL certificate file")
	cmd.Flags().StringVar(&client.KeyFile, "key-file", "", "identify HTTPS client using this SSL key file")
	cmd.Flags().StringVar(&client.CaFile, "ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")
	cmd.Flags().BoolVar(&client.InsecureSkipTLSVerify, "insecure-skip-tls-verify", false, "skip TLS certificate checks for chart downloads")
	cmd.Flags().BoolVar(&client.PlainHTTP, "plain-http", false, "use insecure HTTP connections for OCI registries")
	bindOutputFlag(cmd, &format)
	return cmd
}

type dependencyUpdatesWriter []downloader.DependencyUpdate

func (rows dependencyUpdatesWriter) WriteTable(out io.Writer) error {
	table := uitable.New()
	table.AddRow("CHART", "NAME", "CONSTRAINT", "LOCKED", "COMPATIBLE", "LATEST")
	for _, row := range rows {
		table.AddRow(row.Chart, row.Name, row.Constraint, row.Locked, row.Compatible, row.Latest)
	}
	return output.EncodeTable(out, table)
}

func (rows dependencyUpdatesWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, rows)
}

func (rows dependencyUpdatesWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, rows)
}
