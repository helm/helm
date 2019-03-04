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
	"regexp"
	"text/tabwriter"

	"github.com/ghodss/yaml"
	"github.com/gosuri/uitable"
	"github.com/gosuri/uitable/util/strutil"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/helm"
)

var statusHelp = `
This command shows the status of a named release.
The status consists of:
- last deployment time
- k8s namespace in which the release lives
- state of the release (can be: unknown, deployed, deleted, superseded, failed or deleting)
- list of resources that this release consists of, sorted by kind
- details on last test suite run, if applicable
- additional notes provided by the chart
`

type statusOptions struct {
	release string
	client  helm.Interface
	version int
	outfmt  string
}

func newStatusCmd(client helm.Interface, out io.Writer) *cobra.Command {
	o := &statusOptions{client: client}

	cmd := &cobra.Command{
		Use:   "status RELEASE_NAME",
		Short: "displays the status of the named release",
		Long:  statusHelp,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.release = args[0]
			o.client = ensureHelmClient(o.client, false)
			return o.run(out)
		},
	}

	cmd.PersistentFlags().IntVar(&o.version, "revision", 0, "if set, display the status of the named release with revision")
	cmd.PersistentFlags().StringVarP(&o.outfmt, "output", "o", "", "output the status in the specified format (json or yaml)")

	return cmd
}

func (o *statusOptions) run(out io.Writer) error {
	res, err := o.client.ReleaseContent(o.release, o.version)
	if err != nil {
		return err
	}

	switch o.outfmt {
	case "":
		PrintStatus(out, res)
		return nil
	case "json":
		data, err := json.Marshal(res)
		if err != nil {
			return errors.Wrap(err, "failed to Marshal JSON output")
		}
		out.Write(data)
		return nil
	case "yaml":
		data, err := yaml.Marshal(res)
		if err != nil {
			return errors.Wrap(err, "failed to Marshal YAML output")
		}
		out.Write(data)
		return nil
	}

	return errors.Errorf("unknown output format %q", o.outfmt)
}

// PrintStatus prints out the status of a release. Shared because also used by
// install / upgrade
func PrintStatus(out io.Writer, res *release.Release) {
	if !res.Info.LastDeployed.IsZero() {
		fmt.Fprintf(out, "LAST DEPLOYED: %s\n", res.Info.LastDeployed)
	}
	fmt.Fprintf(out, "NAMESPACE: %s\n", res.Namespace)
	fmt.Fprintf(out, "STATUS: %s\n", res.Info.Status.String())
	fmt.Fprintf(out, "\n")
	if len(res.Info.Resources) > 0 {
		re := regexp.MustCompile("  +")

		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', tabwriter.TabIndent)
		fmt.Fprintf(w, "RESOURCES:\n%s\n", re.ReplaceAllString(res.Info.Resources, "\t"))
		w.Flush()
	}
	if res.Info.LastTestSuiteRun != nil {
		lastRun := res.Info.LastTestSuiteRun
		fmt.Fprintf(out, "TEST SUITE:\n%s\n%s\n\n%s\n",
			fmt.Sprintf("Last Started: %s", lastRun.StartedAt),
			fmt.Sprintf("Last Completed: %s", lastRun.CompletedAt),
			formatTestResults(lastRun.Results))
	}

	if len(res.Info.Notes) > 0 {
		fmt.Fprintf(out, "NOTES:\n%s\n", res.Info.Notes)
	}
}

func formatTestResults(results []*release.TestRun) string {
	tbl := uitable.New()
	tbl.MaxColWidth = 50
	tbl.AddRow("TEST", "STATUS", "INFO", "STARTED", "COMPLETED")
	for i := 0; i < len(results); i++ {
		r := results[i]
		n := r.Name
		s := strutil.PadRight(r.Status.String(), 10, ' ')
		i := r.Info
		ts := r.StartedAt
		tc := r.CompletedAt
		tbl.AddRow(n, s, i, ts, tc)
	}
	return tbl.String()
}
