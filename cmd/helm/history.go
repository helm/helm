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

	"github.com/ghodss/yaml"
	"github.com/gosuri/uitable"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/helm"
)

type releaseInfo struct {
	Revision    int    `json:"revision"`
	Updated     string `json:"updated"`
	Status      string `json:"status"`
	Chart       string `json:"chart"`
	Description string `json:"description"`
}

type releaseHistory []releaseInfo

var historyHelp = `
History prints historical revisions for a given release.

A default maximum of 256 revisions will be returned. Setting '--max'
configures the maximum length of the revision list returned.

The historical release set is printed as a formatted table, e.g:

    $ helm history angry-bird --max=4
    REVISION   UPDATED                      STATUS           CHART        DESCRIPTION
    1           Mon Oct 3 10:15:13 2016     superseded      alpine-0.1.0  Initial install
    2           Mon Oct 3 10:15:13 2016     superseded      alpine-0.1.0  Upgraded successfully
    3           Mon Oct 3 10:15:13 2016     superseded      alpine-0.1.0  Rolled back to 2
    4           Mon Oct 3 10:15:13 2016     deployed        alpine-0.1.0  Upgraded successfully
`

type historyOptions struct {
	colWidth     uint   // --col-width
	max          int    // --max
	outputFormat string // --output

	release string

	client helm.Interface
}

func newHistoryCmd(c helm.Interface, out io.Writer) *cobra.Command {
	o := &historyOptions{client: c}

	cmd := &cobra.Command{
		Use:     "history RELEASE_NAME",
		Long:    historyHelp,
		Short:   "fetch release history",
		Aliases: []string{"hist"},
		Args:    require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.client = ensureHelmClient(o.client, false)
			o.release = args[0]
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.IntVar(&o.max, "max", 256, "maximum number of revision to include in history")
	f.UintVar(&o.colWidth, "col-width", 60, "specifies the max column width of output")
	f.StringVarP(&o.outputFormat, "output", "o", "table", "prints the output in the specified format (json|table|yaml)")

	return cmd
}

func (o *historyOptions) run(out io.Writer) error {
	rels, err := o.client.ReleaseHistory(o.release, o.max)
	if err != nil {
		return err
	}
	if len(rels) == 0 {
		return nil
	}

	releaseHistory := getReleaseHistory(rels)

	var history []byte
	var formattingError error

	switch o.outputFormat {
	case "yaml":
		history, formattingError = yaml.Marshal(releaseHistory)
	case "json":
		history, formattingError = json.Marshal(releaseHistory)
	case "table":
		history = formatAsTable(releaseHistory, o.colWidth)
	default:
		return errors.Errorf("unknown output format %q", o.outputFormat)
	}

	if formattingError != nil {
		return formattingError
	}

	fmt.Fprintln(out, string(history))
	return nil
}

func getReleaseHistory(rls []*release.Release) (history releaseHistory) {
	for i := len(rls) - 1; i >= 0; i-- {
		r := rls[i]
		c := formatChartname(r.Chart)
		s := r.Info.Status.String()
		v := r.Version
		d := r.Info.Description

		rInfo := releaseInfo{
			Revision:    v,
			Status:      s,
			Chart:       c,
			Description: d,
		}
		if !r.Info.LastDeployed.IsZero() {
			rInfo.Updated = r.Info.LastDeployed.String()

		}
		history = append(history, rInfo)
	}

	return history
}

func formatAsTable(releases releaseHistory, colWidth uint) []byte {
	tbl := uitable.New()

	tbl.MaxColWidth = colWidth
	tbl.AddRow("REVISION", "UPDATED", "STATUS", "CHART", "DESCRIPTION")
	for i := 0; i <= len(releases)-1; i++ {
		r := releases[i]
		tbl.AddRow(r.Revision, r.Updated, r.Status, r.Chart, r.Description)
	}
	return tbl.Bytes()
}

func formatChartname(c *chart.Chart) string {
	if c == nil || c.Metadata == nil {
		// This is an edge case that has happened in prod, though we don't
		// know how: https://github.com/helm/helm/issues/1347
		return "MISSING"
	}
	return fmt.Sprintf("%s-%s", c.Name(), c.Metadata.Version)
}
