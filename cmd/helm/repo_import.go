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
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/yaml"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/repo"
)

type repoImportOptions struct {
	importedFilePath string
	repoFile         string
	repoCache        string
}

func newRepoImportCmd(out io.Writer) *cobra.Command {
	o := &repoImportOptions{}

	cmd := &cobra.Command{
		Use:   "import [PATH]",
		Short: "import chart repositories",
		Args:  require.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// Allow file completion when completing the argument for the directory
				return nil, cobra.ShellCompDirectiveDefault
			}
			// No more completions, so disable file completion
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.importedFilePath = args[0]
			o.repoFile = settings.RepositoryConfig
			o.repoCache = settings.RepositoryCache
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.importedFilePath, "file", "", "path to the file to import")

	return cmd
}

func (o *repoImportOptions) run(out io.Writer) error {
	var helmRepoEntries []repo.Entry
	fileContent, err := os.ReadFile(o.importedFilePath)
	if err != nil {
		return err
	}

	err = yaml.UnmarshalStrict(fileContent, &helmRepoEntries)
	if err != nil {
		return errors.Errorf("%s is an invalid YAML file", o.importedFilePath)
	}

	for i := range helmRepoEntries {
		helmRepoEntry := helmRepoEntries[i]
		info := repoAddOptions{
			name:                  helmRepoEntry.Name,
			url:                   helmRepoEntry.URL,
			username:              helmRepoEntry.Username,
			password:              helmRepoEntry.Password,
			passCredentialsAll:    helmRepoEntry.PassCredentialsAll,
			certFile:              helmRepoEntry.CertFile,
			keyFile:               helmRepoEntry.KeyFile,
			caFile:                helmRepoEntry.CAFile,
			insecureSkipTLSverify: helmRepoEntry.InsecureSkipTLSverify,
			repoFile:              o.repoFile,
			repoCache:             o.repoCache}
		err = info.run(out)
		if err != nil {
			return err
		}
	}
	return nil
}
