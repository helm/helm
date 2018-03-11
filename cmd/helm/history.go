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

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/timeconv"
)

var historyHelp = `
History prints historical revisions for a given release.

A default maximum of 256 revisions will be returned. Setting '--max'
configures the maximum length of the revision list returned.

The historical release set is printed as a formatted table, e.g:

    $ helm history angry-bird --max=4
    REVISION   UPDATED                      STATUS           CHART        DESCRIPTION
    1           Mon Oct 3 10:15:13 2016     SUPERSEDED      alpine-0.1.0  Initial install
    2           Mon Oct 3 10:15:13 2016     SUPERSEDED      alpine-0.1.0  Upgraded successfully
    3           Mon Oct 3 10:15:13 2016     SUPERSEDED      alpine-0.1.0  Rolled back to 2
    4           Mon Oct 3 10:15:13 2016     DEPLOYED        alpine-0.1.0  Upgraded successfully
`

type historyCmd struct {
	max      int32
	rls      string
	out      io.Writer
	helmc    helm.Interface
	colWidth uint
}

func newHistoryCmd(c helm.Interface, w io.Writer) *cobra.Command {
	his := &historyCmd{out: w, helmc: c}

	cmd := &cobra.Command{
		Use:     "history [flags] RELEASE_NAME",
		Long:    historyHelp,
		Short:   "fetch release history",
		Aliases: []string{"hist"},
		PreRunE: func(_ *cobra.Command, _ []string) error { return setupConnection() },
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case len(args) == 0:
				return errReleaseRequired
			case his.helmc == nil:
				his.helmc = newClient()
			}
			his.rls = args[0]
			return his.run()
		},
	}

	f := cmd.Flags()
	f.Int32Var(&his.max, "max", 256, "maximum number of revision to include in history")
	f.UintVar(&his.colWidth, "col-width", 60, "specifies the max column width of output")

	return cmd
}

func (cmd *historyCmd) run() error {
	r, err := cmd.helmc.ReleaseHistory(cmd.rls, helm.WithMaxHistory(cmd.max))
	if err != nil {
		return prettyError(err)
	}
	if len(r.Releases) == 0 {
		return nil
	}

	fmt.Fprintln(cmd.out, formatHistory(r.Releases, cmd.colWidth))
	return nil
}

func formatHistory(rls []*release.Release, colWidth uint) string {
	tbl := uitable.New()

	tbl.MaxColWidth = colWidth
	tbl.AddRow("REVISION", "UPDATED", "STATUS", "CHART", "DESCRIPTION")
	for i := len(rls) - 1; i >= 0; i-- {
		r := rls[i]
		c := formatChartname(r.Chart)
		t := timeconv.String(r.Info.LastDeployed)
		s := r.Info.Status.Code.String()
		v := r.Version
		d := r.Info.Description
		tbl.AddRow(v, t, s, c, d)
	}
	return tbl.String()
}

func formatChartname(c *chart.Chart) string {
	if c == nil || c.Metadata == nil {
		// This is an edge case that has happened in prod, though we don't
		// know how: https://github.com/kubernetes/helm/issues/1347
		return "MISSING"
	}
	return fmt.Sprintf("%s-%s", c.Metadata.Name, c.Metadata.Version)
}
