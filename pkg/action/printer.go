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

package action

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/gosuri/uitable"
	"github.com/gosuri/uitable/util/strutil"

	"helm.sh/helm/pkg/release"
)

// PrintRelease prints info about a release
func PrintRelease(out io.Writer, rel *release.Release) {
	if rel == nil {
		return
	}
	fmt.Fprintf(out, "NAME: %s\n", rel.Name)
	if !rel.Info.LastDeployed.IsZero() {
		fmt.Fprintf(out, "LAST DEPLOYED: %s\n", rel.Info.LastDeployed)
	}
	fmt.Fprintf(out, "NAMESPACE: %s\n", rel.Namespace)
	fmt.Fprintf(out, "STATUS: %s\n", rel.Info.Status.String())
	fmt.Fprintf(out, "\n")
	if len(rel.Info.Resources) > 0 {
		re := regexp.MustCompile("  +")

		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', tabwriter.TabIndent)
		fmt.Fprintf(w, "RESOURCES:\n%s\n", re.ReplaceAllString(rel.Info.Resources, "\t"))
		w.Flush()
	}
	if rel.Info.LastTestSuiteRun != nil {
		lastRun := rel.Info.LastTestSuiteRun
		fmt.Fprintf(out, "TEST SUITE:\n%s\n%s\n\n%s\n",
			fmt.Sprintf("Last Started: %s", lastRun.StartedAt),
			fmt.Sprintf("Last Completed: %s", lastRun.CompletedAt),
			formatTestResults(lastRun.Results))
	}

	if strings.EqualFold(rel.Info.Description, "Dry run complete") {
		fmt.Fprintf(out, "MANIFEST:\n%s\n", rel.Manifest)
	}

	if len(rel.Info.Notes) > 0 {
		fmt.Fprintf(out, "NOTES:\n%s\n", strings.TrimSpace(rel.Info.Notes))
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
