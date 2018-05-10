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

	"github.com/Masterminds/semver"
	"github.com/gosuri/uitable"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/search"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
)

const searchDesc = `
Search reads through all of the repositories configured on the system, and
looks for matches.

Repositories are managed with 'helm repo' commands.
`

// searchMaxScore suggests that any score higher than this is not considered a match.
const searchMaxScore = 25

type searchOptions struct {
	helmhome helmpath.Home

	versions bool
	regexp   bool
	version  string
}

func newSearchCmd(out io.Writer) *cobra.Command {
	o := &searchOptions{}

	cmd := &cobra.Command{
		Use:   "search [keyword]",
		Short: "search for a keyword in charts",
		Long:  searchDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.helmhome = settings.Home
			return o.run(out, args)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.regexp, "regexp", "r", false, "use regular expressions for searching")
	f.BoolVarP(&o.versions, "versions", "l", false, "show the long listing, with each version of each chart on its own line")
	f.StringVarP(&o.version, "version", "v", "", "search using semantic versioning constraints")

	return cmd
}

func (o *searchOptions) run(out io.Writer, args []string) error {
	index, err := o.buildIndex(out)
	if err != nil {
		return err
	}

	var res []*search.Result
	if len(args) == 0 {
		res = index.All()
	} else {
		q := strings.Join(args, " ")
		res, err = index.Search(q, searchMaxScore, o.regexp)
		if err != nil {
			return err
		}
	}

	search.SortScore(res)
	data, err := o.applyConstraint(res)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, o.formatSearchResults(data))

	return nil
}

func (o *searchOptions) applyConstraint(res []*search.Result) ([]*search.Result, error) {
	if len(o.version) == 0 {
		return res, nil
	}

	constraint, err := semver.NewConstraint(o.version)
	if err != nil {
		return res, errors.Wrap(err, "an invalid version/constraint format")
	}

	data := res[:0]
	foundNames := map[string]bool{}
	for _, r := range res {
		if _, found := foundNames[r.Name]; found {
			continue
		}
		v, err := semver.NewVersion(r.Chart.Version)
		if err != nil || constraint.Check(v) {
			data = append(data, r)
			if !o.versions {
				foundNames[r.Name] = true // If user hasn't requested all versions, only show the latest that matches
			}
		}
	}

	return data, nil
}

func (o *searchOptions) formatSearchResults(res []*search.Result) string {
	if len(res) == 0 {
		return "No results found"
	}
	table := uitable.New()
	table.MaxColWidth = 50
	table.AddRow("NAME", "CHART VERSION", "APP VERSION", "DESCRIPTION")
	for _, r := range res {
		table.AddRow(r.Name, r.Chart.Version, r.Chart.AppVersion, r.Chart.Description)
	}
	return table.String()
}

func (o *searchOptions) buildIndex(out io.Writer) (*search.Index, error) {
	// Load the repositories.yaml
	rf, err := repo.LoadRepositoriesFile(o.helmhome.RepositoryFile())
	if err != nil {
		return nil, err
	}

	i := search.NewIndex()
	for _, re := range rf.Repositories {
		n := re.Name
		f := o.helmhome.CacheIndex(n)
		ind, err := repo.LoadIndexFile(f)
		if err != nil {
			// TODO should print to stderr
			fmt.Fprintf(out, "WARNING: Repo %q is corrupt or missing. Try 'helm repo update'.", n)
			continue
		}

		i.AddRepo(n, ind, o.versions || len(o.version) > 0)
	}
	return i, nil
}
