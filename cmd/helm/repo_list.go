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

	"github.com/gosuri/uitable"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/pkg/action"
	"helm.sh/helm/pkg/repo"
)

func newRepoListCmd(out io.Writer) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list chart repositories",
		Args:  require.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate the output format first so we don't waste time running a
			// request that we'll throw away
			outfmt, err := action.ParseOutputFormat(output)
			if err != nil {
				return err
			}
			f, err := repo.LoadFile(settings.RepositoryConfig)
			if isNotExist(err) || len(f.Repositories) == 0 {
				return errors.New("no repositories to show")
			}

			return outfmt.Write(out, &repoListWriter{f.Repositories})
		},
	}

	bindOutputFlag(cmd, &output)

	return cmd
}

type repositoryElement struct {
	Name string
	URL  string
}

type repoListWriter struct {
	repos []*repo.Entry
}

func (r *repoListWriter) WriteTable(out io.Writer) error {
	table := uitable.New()
	table.AddRow("NAME", "URL")
	for _, re := range r.repos {
		table.AddRow(re.Name, re.URL)
	}
	return action.EncodeTable(out, table)
}

func (r *repoListWriter) WriteJSON(out io.Writer) error {
	return r.encodeByFormat(out, action.JSON)
}

func (r *repoListWriter) WriteYAML(out io.Writer) error {
	return r.encodeByFormat(out, action.YAML)
}

func (r *repoListWriter) encodeByFormat(out io.Writer, format action.OutputFormat) error {
	// Initialize the array so no results returns an empty array instead of null
	repolist := make([]repositoryElement, 0, len(r.repos))

	for _, re := range r.repos {
		repolist = append(repolist, repositoryElement{Name: re.Name, URL: re.URL})
	}

	switch format {
	case action.JSON:
		return action.EncodeJSON(out, repolist)
	case action.YAML:
		return action.EncodeYAML(out, repolist)
	}

	// Because this is a non-exported function and only called internally by
	// WriteJSON and WriteYAML, we shouldn't get invalid types
	return nil
}
