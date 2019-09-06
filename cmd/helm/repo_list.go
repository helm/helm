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
	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
)

type repoListCmd struct {
	out    io.Writer
	home   helmpath.Home
	output string
}

type repositoryElement struct {
	Name string
	URL  string
}

func newRepoListCmd(out io.Writer) *cobra.Command {
	list := &repoListCmd{out: out}

	cmd := &cobra.Command{
		Use:   "list [flags]",
		Short: "List chart repositories",
		RunE: func(cmd *cobra.Command, args []string) error {
			list.home = settings.Home
			return list.run()
		},
	}

	bindOutputFlag(cmd, &list.output)
	return cmd
}

func (a *repoListCmd) run() error {
	repoFile, err := repo.LoadRepositoriesFile(a.home.RepositoryFile())
	if err != nil {
		return err
	}

	return write(a.out, &repoListWriter{repoFile.Repositories}, outputFormat(a.output))
}

//////////// Printer implementation below here
type repoListWriter struct {
	repos []*repo.Entry
}

func (r *repoListWriter) WriteTable(out io.Writer) error {
	table := uitable.New()
	table.AddRow("NAME", "URL")
	for _, re := range r.repos {
		table.AddRow(re.Name, re.URL)
	}
	return encodeTable(out, table)
}

func (r *repoListWriter) WriteJSON(out io.Writer) error {
	return r.encodeByFormat(out, outputJSON)
}

func (r *repoListWriter) WriteYAML(out io.Writer) error {
	return r.encodeByFormat(out, outputYAML)
}

func (r *repoListWriter) encodeByFormat(out io.Writer, format outputFormat) error {
	var repolist []repositoryElement

	for _, re := range r.repos {
		repolist = append(repolist, repositoryElement{Name: re.Name, URL: re.URL})
	}

	switch format {
	case outputJSON:
		return encodeJSON(out, repolist)
	case outputYAML:
		return encodeYAML(out, repolist)
	}

	// Because this is a non-exported function and only called internally by
	// WriteJSON and WriteYAML, we shouldn't get invalid types
	return nil
}
