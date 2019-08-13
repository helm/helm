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

	"github.com/gosuri/uitable"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/internal/monocular"
)

const searchHubDesc = `
Search the Helm Hub or an instance of Monocular for Helm charts.

The Helm Hub provides a centralized search for publicly available distributed
charts. It is maintained by the Helm project. It can be visited at
https://hub.helm.sh

Monocular is a web-based application that enables the search and discovery of
charts from multiple Helm Chart repositories. It is the codebase that powers the
Helm Hub. You can find it at https://github.com/helm/monocular
`

type searchHubOptions struct {
	searchEndpoint string
	maxColWidth    uint
}

func newSearchHubCmd(out io.Writer) *cobra.Command {
	o := &searchHubOptions{}

	cmd := &cobra.Command{
		Use:   "hub [keyword]",
		Short: "search for charts in the Helm Hub or an instance of Monocular",
		Long:  searchHubDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(out, args)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.searchEndpoint, "endpoint", "https://hub.helm.sh", "monocular instance to query for charts")
	f.UintVar(&o.maxColWidth, "max-col-width", 50, "maximum column width for output table")

	return cmd
}

func (o *searchHubOptions) run(out io.Writer, args []string) error {

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

func (o *searchHubOptions) formatSearchResults(endpoint string, res []monocular.SearchResult) string {
	if len(res) == 0 {
		return "No results found"
	}
	table := uitable.New()

	// The max column width is configurable because a URL could be longer than the
	// max value and we want the user to have the ability to display the whole url
	table.MaxColWidth = o.maxColWidth
	table.AddRow("URL", "CHART VERSION", "APP VERSION", "DESCRIPTION")
	var url string
	for _, r := range res {
		url = endpoint + "/charts/" + r.ID
		table.AddRow(url, r.Relationships.LatestChartVersion.Data.Version, r.Relationships.LatestChartVersion.Data.AppVersion, r.Attributes.Description)
	}
	return table.String()
}
