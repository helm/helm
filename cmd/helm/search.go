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
	"strings"

	"github.com/Masterminds/semver"
	"github.com/gosuri/uitable"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/search"
	"helm.sh/helm/internal/monocular"
	"helm.sh/helm/pkg/helmpath"
	"helm.sh/helm/pkg/repo"
)

const searchDesc = `
Search reads through all of the repositories configured on the system, and
looks for matches.

Repositories are managed with 'helm repo' commands.
`

// searchMaxScore suggests that any score higher than this is not considered a match.
const searchMaxScore = 25

type searchOptions struct {
	versions       bool
	regexp         bool
	version        string
	searchEndpoint string
	repositories   bool
	maxColWidth    uint
}

func newSearchCmd(out io.Writer) *cobra.Command {
	o := &searchOptions{}

	cmd := &cobra.Command{
		Use:   "search [keyword]",
		Short: "search for a keyword in charts",
		Long:  searchDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(out, args)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.searchEndpoint, "endpoint", "https://hub.helm.sh", "monocular instance to query for charts")
	f.BoolVarP(&o.repositories, "repositories", "r", false, "search repositories you have added instead of monocular")
	f.BoolVarP(&o.regexp, "regexp", "", false, "use regular expressions for searching repositories you have added")
	f.BoolVarP(&o.versions, "versions", "l", false, "show the long listing, with each version of each chart on its own line, for repositories you have added")
	f.StringVar(&o.version, "version", "", "search using semantic versioning constraints on repositories you have added")
	f.UintVar(&o.maxColWidth, "maxColumnWidth", 50, "maximum column width for output table")

	return cmd
}

func (o *searchOptions) run(out io.Writer, args []string) error {

	// If searching the repositories added via helm repo add follow that path
	if o.repositories {
		debug("searching repositories")
		// If an endpoint was passed in but searching repositories the user should
		// know the option is being skipped
		if o.searchEndpoint != "https://hub.helm.sh" {
			fmt.Fprintln(out, "Notice: Setting the \"endpoint\" flag has no effect when searching repositories you have added")
		}

		return o.runRepositories(out, args)
	}

	// Search the Helm Hub or other monocular instance
	debug("searching monocular")
	// If an an option used against repository searches is used the user should
	// know the option is being skipped
	if o.regexp {
		fmt.Fprintln(out, "Notice: Setting the \"regexp\" flag has no effect when searching monocular (e.g., Helm Hub)")
	}
	if o.versions {
		fmt.Fprintln(out, "Notice: Setting the \"versions\" flag has no effect when searching monocular (e.g., Helm Hub)")
	}
	if o.version != "" {
		fmt.Fprintln(out, "Notice: Setting the \"version\" flag has no effect when searching monocular (e.g., Helm Hub)")
	}

	c, err := monocular.New(o.searchEndpoint)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to create connection to %q", o.searchEndpoint))
	}

	q := strings.Join(args, " ")
	results, err := c.Search(q)
	if err != nil {
		debug("%s", err)
		return fmt.Errorf("unable to perform search against %q", o.searchEndpoint)
	}

	fmt.Fprintln(out, o.formatSearchResults(o.searchEndpoint, results))

	return nil
}

func (o *searchOptions) runRepositories(out io.Writer, args []string) error {
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

	fmt.Fprintln(out, o.formatRepoSearchResults(data))

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

func (o *searchOptions) formatSearchResults(endpoint string, res []monocular.SearchResult) string {
	if len(res) == 0 {
		return "No results found"
	}
	table := uitable.New()
	table.MaxColWidth = o.maxColWidth
	table.AddRow("URL", "CHART VERSION", "APP VERSION", "DESCRIPTION")
	var url string
	for _, r := range res {
		url = endpoint + "/charts/" + r.ID
		table.AddRow(url, r.Relationships.LatestChartVersion.Data.Version, r.Relationships.LatestChartVersion.Data.AppVersion, r.Attributes.Description)
	}
	return table.String()
}

func (o *searchOptions) formatRepoSearchResults(res []*search.Result) string {
	if len(res) == 0 {
		return "No results found"
	}
	table := uitable.New()
	table.MaxColWidth = o.maxColWidth
	table.AddRow("NAME", "CHART VERSION", "APP VERSION", "DESCRIPTION")
	for _, r := range res {
		table.AddRow(r.Name, r.Chart.Version, r.Chart.AppVersion, r.Chart.Description)
	}
	return table.String()
}

func (o *searchOptions) buildIndex(out io.Writer) (*search.Index, error) {
	// Load the repositories.yaml
	rf, err := repo.LoadFile(helmpath.RepositoryFile())
	if err != nil {
		return nil, err
	}

	i := search.NewIndex()
	for _, re := range rf.Repositories {
		n := re.Name
		f := helmpath.CacheIndex(n)
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
