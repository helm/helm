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
	"fmt"
	"io"
	"strings"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/cmd/helm/search"
	"k8s.io/helm/pkg/repo"
)

const searchDesc = `
Search reads through all of the repositories configured on the system, and
looks for matches.

Repositories are managed with 'helm repo' commands.
`

// searchMaxScore suggests that any score higher than this is not considered a match.
const searchMaxScore = 25

type searchCmd struct {
	out      io.Writer
	helmhome helmpath.Home

	versions bool
	regexp   bool
}

func newSearchCmd(out io.Writer) *cobra.Command {
	sc := &searchCmd{out: out, helmhome: helmpath.Home(homePath())}

	cmd := &cobra.Command{
		Use:   "search [keyword]",
		Short: "search for a keyword in charts",
		Long:  searchDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			return sc.run(args)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&sc.regexp, "regexp", "r", false, "use regular expressions for searching")
	f.BoolVarP(&sc.versions, "versions", "l", false, "show the long listing, with each version of each chart on its own line")

	return cmd
}

func (s *searchCmd) run(args []string) error {
	index, err := s.buildIndex()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		s.showAllCharts(index)
		return nil
	}

	q := strings.Join(args, " ")
	res, err := index.Search(q, searchMaxScore, s.regexp)
	if err != nil {
		return nil
	}
	search.SortScore(res)

	fmt.Fprintln(s.out, s.formatSearchResults(res))

	return nil
}

func (s *searchCmd) showAllCharts(i *search.Index) {
	res := i.All()
	search.SortScore(res)
	fmt.Fprintln(s.out, s.formatSearchResults(res))
}

func (s *searchCmd) formatSearchResults(res []*search.Result) string {
	if len(res) == 0 {
		return "No results found"
	}
	table := uitable.New()
	table.MaxColWidth = 50
	table.AddRow("NAME", "VERSION", "DESCRIPTION")
	for _, r := range res {
		table.AddRow(r.Name, r.Chart.Version, r.Chart.Description)
	}
	return table.String()
}

func (s *searchCmd) buildIndex() (*search.Index, error) {
	// Load the repositories.yaml
	rf, err := repo.LoadRepositoriesFile(s.helmhome.RepositoryFile())
	if err != nil {
		return nil, err
	}

	i := search.NewIndex()
	for _, re := range rf.Repositories {
		n := re.Name
		f := s.helmhome.CacheIndex(n)
		ind, err := repo.LoadIndexFile(f)
		if err != nil {
			fmt.Fprintf(s.out, "WARNING: Repo %q is corrupt or missing. Try 'helm repo update'.", n)
			continue
		}

		i.AddRepo(n, ind, s.versions)
	}
	return i, nil
}
