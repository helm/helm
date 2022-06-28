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
	"helm.sh/helm/v3/pkg/action"
)

func newRepoAddCmd(out io.Writer) *cobra.Command {
	o := &action.RepoAddOptions{}

	cmd := &cobra.Command{
		Use:               "add [NAME] [URL]",
		Short:             "add a chart repository",
		Args:              require.ExactArgs(2),
		ValidArgsFunction: noCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name = args[0]
			o.URL = args[1]
			o.RepoFile = settings.RepositoryConfig
			o.RepoCache = settings.RepositoryCache

			return o.Run(settings, out)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.Username, "username", "", "chart repository username")
	f.StringVar(&o.Password, "password", "", "chart repository password")
	f.BoolVarP(&o.PasswordFromStdinOpt, "password-stdin", "", false, "read chart repository password from stdin")
	f.BoolVar(&o.ForceUpdate, "force-update", false, "replace (overwrite) the repo if it already exists")
	f.BoolVar(&o.DeprecatedNoUpdate, "no-update", false, "Ignored. Formerly, it would disabled forced updates. It is deprecated by force-update.")
	f.StringVar(&o.CertFile, "cert-file", "", "identify HTTPS client using this SSL certificate file")
	f.StringVar(&o.KeyFile, "key-file", "", "identify HTTPS client using this SSL key file")
	f.StringVar(&o.CaFile, "ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")
	f.BoolVar(&o.InsecureSkipTLSverify, "insecure-skip-tls-verify", false, "skip tls certificate checks for the repository")
	f.BoolVar(&o.AllowDeprecatedRepos, "allow-deprecated-repos", false, "by default, this command will not allow adding official repos that have been permanently deleted. This disables that behavior")
	f.BoolVar(&o.PassCredentialsAll, "pass-credentials", false, "pass credentials to all domains")

	return cmd
}
