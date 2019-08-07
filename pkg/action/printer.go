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

	executions := executionsByHookEvent(rel)
	if tests, ok := executions[release.HookTest]; ok {
		for _, h := range tests {
			// Don't print anything if hook has not been initiated
			if h.LastRun.StartedAt.IsZero() {
				continue
			}
			fmt.Fprintf(out, "TEST SUITE:     %s\n%s\n%s\n%s\n\n",
				h.Name,
				fmt.Sprintf("Last Started:   %s", h.LastRun.StartedAt),
				fmt.Sprintf("Last Completed: %s", h.LastRun.CompletedAt),
				fmt.Sprintf("Phase:          %s", h.LastRun.Phase),
			)
		}
	}

	if strings.EqualFold(rel.Info.Description, "Dry run complete") {
		fmt.Fprintf(out, "MANIFEST:\n%s\n", rel.Manifest)
	}

	if len(rel.Info.Notes) > 0 {
		fmt.Fprintf(out, "NOTES:\n%s\n", strings.TrimSpace(rel.Info.Notes))
	}
}

func executionsByHookEvent(rel *release.Release) map[release.HookEvent][]*release.Hook {
	result := make(map[release.HookEvent][]*release.Hook)
	for _, h := range rel.Hooks {
		for _, e := range h.Events {
			executions, ok := result[e]
			if !ok {
				executions = []*release.Hook{}
			}
			result[e] = append(executions, h)
		}
	}
	return result
}
