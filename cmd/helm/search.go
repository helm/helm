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
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/ghodss/yaml"
	"github.com/gosuri/uitable"
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

type searchCmd struct {
	out      io.Writer
	helmhome helmpath.Home

	versions bool
	regexp   bool
	version  string
	colWidth uint
	output   string
}

type chartElement struct {
	Name        string
	Version     string
	AppVersion  string
	Description string
}

type searchResult struct {
	Charts []*chartElement
}

func newSearchCmd(out io.Writer) *cobra.Command {
	sc := &searchCmd{out: out}

	cmd := &cobra.Command{
		Use:   "search [keyword]",
		Short: "Search for a keyword in charts",
		Long:  searchDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			sc.helmhome = settings.Home
			return sc.run(args)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&sc.regexp, "regexp", "r", false, "Use regular expressions for searching")
	f.BoolVarP(&sc.versions, "versions", "l", false, "Show the long listing, with each version of each chart on its own line")
	f.StringVarP(&sc.version, "version", "v", "", "Search using semantic versioning constraints")
	f.UintVar(&sc.colWidth, "col-width", 60, "Specifies the max column width of output")
	f.StringVarP(&sc.output, "output", "o", "table", "Prints the output in the specified format (json|table|yaml)")

	return cmd
}

func (s *searchCmd) run(args []string) error {
	index, err := s.buildIndex()
	if err != nil {
		return err
	}

	var res []*search.Result
	if len(args) == 0 {
		res = index.All()
	} else {
		q := strings.Join(args, " ")
		res, err = index.Search(q, searchMaxScore, s.regexp)
		if err != nil {
			return err
		}
	}

	search.SortScore(res)
	data, err := s.applyConstraint(res)
	if err != nil {
		return err
	}

	o, err := s.formatSearchResults(s.output, data, s.colWidth)
	if err != nil {
		return err
	}

	fmt.Fprintln(s.out, o)

	return nil
}

func (s *searchCmd) applyConstraint(res []*search.Result) ([]*search.Result, error) {
	if len(s.version) == 0 {
		return res, nil
	}

	constraint, err := semver.NewConstraint(s.version)
	if err != nil {
		return res, fmt.Errorf("an invalid version/constraint format: %s", err)
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
			if !s.versions {
				foundNames[r.Name] = true // If user hasn't requested all versions, only show the latest that matches
			}
		}
	}

	return data, nil
}

func (s *searchCmd) formatSearchResults(format string, res []*search.Result, colWidth uint) (string, error) {
	var output string
	var err error

	switch format {
	case "table":
		if len(res) == 0 {
			return "No results found", nil
		}
		table := uitable.New()
		table.MaxColWidth = colWidth
		table.AddRow("NAME", "CHART VERSION", "APP VERSION", "DESCRIPTION")
		for _, r := range res {
			table.AddRow(r.Name, r.Chart.Version, r.Chart.AppVersion, r.Chart.Description)
		}
		output = table.String()

	case "json":
		output, err = s.printFormated(format, res, json.Marshal)

	case "yaml":
		output, err = s.printFormated(format, res, yaml.Marshal)
	}

	return output, err
}

func (s *searchCmd) printFormated(format string, res []*search.Result, obj func(v interface{}) ([]byte, error)) (string, error) {
	var sResult searchResult
	var output string
	var err error

	if len(res) == 0 {
		return "[]", nil
	}

	for _, r := range res {
		sResult.Charts = append(sResult.Charts, &chartElement{r.Name, r.Chart.Version, r.Chart.AppVersion, r.Chart.Description})
	}

	o, e := obj(sResult)
	if e != nil {
		err = fmt.Errorf("Failed to Marshal %s output: %s", strings.ToUpper(format), e)
	} else {
		output = string(o)
	}
	return output, err
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
			fmt.Fprintf(s.out, "WARNING: Repo %q is corrupt or missing. Try 'helm repo update'.\n", n)
			continue
		}

		i.AddRepo(n, ind, s.versions || len(s.version) > 0)
	}
	return i, nil
}
