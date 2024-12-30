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
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/kubectl/pkg/cmd/get"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli/output"
	"helm.sh/helm/v3/pkg/release"
)

// NOTE: Keep the list of statuses up-to-date with pkg/release/status.go.
var statusHelp = `
This command shows the status of a named release.
The status consists of:
- last deployment time
- k8s namespace in which the release lives
- state of the release (can be: unknown, deployed, uninstalled, superseded, failed, uninstalling, pending-install, pending-upgrade or pending-rollback)
- revision of the release
- description of the release (can be completion message or error message, need to enable --show-desc)
- list of resources that this release consists of (need to enable --show-resources)
- details on last test suite run, if applicable
- additional notes provided by the chart
`

func newStatusCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewStatus(cfg)
	var outfmt output.Format

	cmd := &cobra.Command{
		Use:   "status RELEASE_NAME",
		Short: "display the status of the named release",
		Long:  statusHelp,
		Args:  require.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return noMoreArgsComp()
			}
			return compListReleases(toComplete, args, cfg)
		},
		RunE: func(_ *cobra.Command, args []string) error {

			// When the output format is a table the resources should be fetched
			// and displayed as a table. When YAML or JSON the resources will be
			// returned. This mirrors the handling in kubectl.
			if outfmt == output.Table {
				client.ShowResourcesTable = true
			}
			rel, err := client.Run(args[0])
			if err != nil {
				return err
			}

			// strip chart metadata from the output
			rel.Chart = nil

			return outfmt.Write(out, &statusPrinter{rel, false, client.ShowDescription, client.ShowResources, false, false})
		},
	}

	f := cmd.Flags()

	f.IntVar(&client.Version, "revision", 0, "if set, display the status of the named release with revision")

	err := cmd.RegisterFlagCompletionFunc("revision", func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 1 {
			return compListRevisions(toComplete, cfg, args[0])
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	})

	if err != nil {
		log.Fatal(err)
	}

	bindOutputFlag(cmd, &outfmt)
	f.BoolVar(&client.ShowDescription, "show-desc", false, "if set, display the description message of the named release")

	f.BoolVar(&client.ShowResources, "show-resources", false, "if set, display the resources of the named release")

	return cmd
}

type statusPrinter struct {
	release         *release.Release
	debug           bool
	showDescription bool
	showResources   bool
	showMetadata    bool
	hideNotes       bool
}

func (s statusPrinter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, s.release)
}

func (s statusPrinter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, s.release)
}

func (s statusPrinter) WriteTable(out io.Writer) error {
	if s.release == nil {
		return nil
	}
	_, _ = fmt.Fprintf(out, "NAME: %s\n", s.release.Name)
	if !s.release.Info.LastDeployed.IsZero() {
		_, _ = fmt.Fprintf(out, "LAST DEPLOYED: %s\n", s.release.Info.LastDeployed.Format(time.ANSIC))
	}
	_, _ = fmt.Fprintf(out, "NAMESPACE: %s\n", s.release.Namespace)
	_, _ = fmt.Fprintf(out, "STATUS: %s\n", s.release.Info.Status.String())
	_, _ = fmt.Fprintf(out, "REVISION: %d\n", s.release.Version)
	if s.showMetadata {
		_, _ = fmt.Fprintf(out, "CHART: %s\n", s.release.Chart.Metadata.Name)
		_, _ = fmt.Fprintf(out, "VERSION: %s\n", s.release.Chart.Metadata.Version)
		_, _ = fmt.Fprintf(out, "APP_VERSION: %s\n", s.release.Chart.Metadata.AppVersion)
	}
	if s.showDescription {
		_, _ = fmt.Fprintf(out, "DESCRIPTION: %s\n", s.release.Info.Description)
	}

	if s.showResources && len(s.release.Info.Resources) > 0 {
		buf := new(bytes.Buffer)
		printFlags := get.NewHumanPrintFlags()
		typePrinter, _ := printFlags.ToPrinter("")
		printer := &get.TablePrinter{Delegate: typePrinter}

		var keys []string
		for key := range s.release.Info.Resources {
			keys = append(keys, key)
		}

		for _, t := range keys {
			_, _ = fmt.Fprintf(buf, "==> %s\n", t)

			vk := s.release.Info.Resources[t]
			for _, resource := range vk {
				if err := printer.PrintObj(resource, buf); err != nil {
					_, _ = fmt.Fprintf(buf, "failed to print object type %s: %v\n", t, err)
				}
			}

			buf.WriteString("\n")
		}

		_, _ = fmt.Fprintf(out, "RESOURCES:\n%s\n", buf.String())
	}

	executions := executionsByHookEvent(s.release)
	if tests, ok := executions[release.HookTest]; !ok || len(tests) == 0 {
		_, _ = fmt.Fprintln(out, "TEST SUITE: None")
	} else {
		for _, h := range tests {
			// Don't print anything if hook has not been initiated
			if h.LastRun.StartedAt.IsZero() {
				continue
			}
			_, _ = fmt.Fprintf(out, "TEST SUITE:     %s\n%s\n%s\n%s\n",
				h.Name,
				fmt.Sprintf("Last Started:   %s", h.LastRun.StartedAt.Format(time.ANSIC)),
				fmt.Sprintf("Last Completed: %s", h.LastRun.CompletedAt.Format(time.ANSIC)),
				fmt.Sprintf("Phase:          %s", h.LastRun.Phase),
			)
		}
	}

	if s.debug {
		_, _ = fmt.Fprintln(out, "USER-SUPPLIED VALUES:")
		err := output.EncodeYAML(out, s.release.Config)
		if err != nil {
			return err
		}
		// Print an extra newline
		_, _ = fmt.Fprintln(out)

		cfg, err := chartutil.CoalesceValues(s.release.Chart, s.release.Config)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintln(out, "COMPUTED VALUES:")
		err = output.EncodeYAML(out, cfg.AsMap())
		if err != nil {
			return err
		}
		// Print an extra newline
		_, _ = fmt.Fprintln(out)
	}

	if strings.EqualFold(s.release.Info.Description, "Dry run complete") || s.debug {
		_, _ = fmt.Fprintln(out, "HOOKS:")
		for _, h := range s.release.Hooks {
			_, _ = fmt.Fprintf(out, "---\n# Source: %s\n%s\n", h.Path, h.Manifest)
		}
		_, _ = fmt.Fprintf(out, "MANIFEST:\n%s\n", s.release.Manifest)
	}

	// Hide notes from output - option in install and upgrades
	if !s.hideNotes && len(s.release.Info.Notes) > 0 {
		fmt.Fprintf(out, "NOTES:\n%s\n", strings.TrimSpace(s.release.Info.Notes))
	}
	return nil
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
