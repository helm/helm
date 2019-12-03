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

	"helm.sh/helm/v3/internal/monocular"
	"helm.sh/helm/v3/pkg/cli/output"
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
	outputFormat   output.Format
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
	bindOutputFlag(cmd, &o.outputFormat)

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

	return o.outputFormat.Write(out, newHubSearchWriter(results, o.searchEndpoint, o.maxColWidth))
}

type hubChartElement struct {
	URL         string `json:"url"`
	Version     string `json:"version"`
	AppVersion  string `json:"app_version"`
	Description string `json:"description"`
}

type hubSearchWriter struct {
	elements    []hubChartElement
	columnWidth uint
}

func newHubSearchWriter(results []monocular.SearchResult, endpoint string, columnWidth uint) *hubSearchWriter {
	var elements []hubChartElement
	for _, r := range results {
		url := endpoint + "/charts/" + r.ID
		elements = append(elements, hubChartElement{url, r.Relationships.LatestChartVersion.Data.Version, r.Relationships.LatestChartVersion.Data.AppVersion, r.Attributes.Description})
	}
	return &hubSearchWriter{elements, columnWidth}
}

func (h *hubSearchWriter) WriteTable(out io.Writer) error {
	if len(h.elements) == 0 {
		_, err := out.Write([]byte("No results found\n"))
		if err != nil {
			return fmt.Errorf("unable to write results: %s", err)
		}
		return nil
	}
	table := uitable.New()
	table.MaxColWidth = h.columnWidth
	table.AddRow("URL", "CHART VERSION", "APP VERSION", "DESCRIPTION")
	for _, r := range h.elements {
		table.AddRow(r.URL, r.Version, r.AppVersion, r.Description)
	}
	return output.EncodeTable(out, table)
}

func (h *hubSearchWriter) WriteJSON(out io.Writer) error {
	return h.encodeByFormat(out, output.JSON)
}

func (h *hubSearchWriter) WriteYAML(out io.Writer) error {
	return h.encodeByFormat(out, output.YAML)
}

func (h *hubSearchWriter) encodeByFormat(out io.Writer, format output.Format) error {
	// Initialize the array so no results returns an empty array instead of null
	chartList := make([]hubChartElement, 0, len(h.elements))

	for _, r := range h.elements {
		chartList = append(chartList, hubChartElement{r.URL, r.Version, r.AppVersion, r.Description})
	}

	switch format {
	case output.JSON:
		return output.EncodeJSON(out, chartList)
	case output.YAML:
		return output.EncodeYAML(out, chartList)
	}

	// Because this is a non-exported function and only called internally by
	// WriteJSON and WriteYAML, we shouldn't get invalid types
	return nil
}
