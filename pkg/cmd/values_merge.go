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

package cmd

import (
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cli/output"
	"helm.sh/helm/v4/pkg/cmd/require"
)

var valuesMergeHelp = `
This command intelligently merges values from multiple release revisions into a single
values file, helping to solve the "version hell" problem when migrating applications
that have undergone many releases, updates, and rollbacks.

It supports different merge strategies:
- latest: Later values override earlier ones (default)
- first: Earlier values take precedence
- merge: Deep merge with intelligent conflict resolution

Examples:
    # Merge all deployed revisions of a release
    helm values merge my-release

    # Merge specific revisions
    helm values merge my-release --revisions 1,3,5

    # Merge a range of revisions
    helm values merge my-release --revisions 1..5

    # Use first-wins strategy instead of latest-wins
    helm values merge my-release --strategy first

    # Output in JSON format
    helm values merge my-release --output json
`

type mergeValuesWriter struct {
	vals map[string]interface{}
}

func newValuesMergeCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	var outfmt output.Format
	var revisions string
	var strategy string
	var allRevisions bool

	client := action.NewMergeValues(cfg)

	cmd := &cobra.Command{
		Use:   "merge RELEASE_NAME",
		Short: "intelligently merge values from multiple release revisions",
		Long:  valuesMergeHelp,
		Args:  require.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return noMoreArgsComp()
			}
			return compListReleases(toComplete, args, cfg)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			// Parse revisions if specified
			if revisions != "" {
				parsedRevisions, err := action.ParseRevisions(revisions)
				if err != nil {
					return fmt.Errorf("invalid revisions specification: %w", err)
				}
				client.Revisions = parsedRevisions
			}

			client.AllRevisions = allRevisions
			client.MergeStrategy = strategy
			client.OutputFormat = string(outfmt)

			mergedVals, err := client.Run(args[0])
			if err != nil {
				return err
			}

			// Remove merge metadata if user doesn't want it
			if helmMeta, ok := mergedVals["helm"].(map[string]interface{}); ok {
				if mergeMeta, ok := helmMeta["mergeMetadata"].(map[string]interface{}); ok {
					fmt.Fprintf(out, "# Merged values for release: %s\n", args[0])
					if revisionsInfo, ok := mergeMeta["revisionInfo"].([]interface{}); ok {
						fmt.Fprintf(out, "# Revisions merged: %d\n", len(revisionsInfo))
						fmt.Fprintf(out, "# Merge strategy: %s\n", strategy)
						for _, revInfo := range revisionsInfo {
							fmt.Fprintf(out, "#   %s\n", revInfo)
						}
						fmt.Fprintf(out, "\n")
					}
					// Remove metadata from output
					delete(helmMeta, "mergeMetadata")
					if len(helmMeta) == 0 {
						delete(mergedVals, "helm")
					}
				}
			}

			// Remove internal merge sources if present
			delete(mergedVals, "_mergeSources")

			return outfmt.Write(out, &mergeValuesWriter{mergedVals})
		},
	}

	f := cmd.Flags()
	f.StringVar(&revisions, "revisions", "", "comma-separated list of revisions to merge (e.g., '1,3,5' or '1..5')")
	f.StringVar(&strategy, "strategy", "latest", "merge strategy: latest, first, or merge (default: latest)")
	f.BoolVar(&allRevisions, "all", false, "merge all revisions of the release")
	bindOutputFlag(cmd, &outfmt)

	// Register completion for the revisions flag
	err := cmd.RegisterFlagCompletionFunc("revisions", func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 1 {
			return compListRevisions(toComplete, cfg, args[0])
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	})
	if err != nil {
		log.Fatal(err)
	}

	// Register completion for the strategy flag
	err = cmd.RegisterFlagCompletionFunc("strategy", func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		strategies := []string{"latest\tLater values override earlier ones (default)",
			"first\tEarlier values take precedence",
			"merge\tDeep merge with intelligent conflict resolution"}
		var completions []string
		for _, s := range strategies {
			if strings.HasPrefix(s, toComplete) {
				completions = append(completions, s)
			}
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	})
	if err != nil {
		log.Fatal(err)
	}

	return cmd
}

func (m mergeValuesWriter) WriteTable(out io.Writer) error {
	fmt.Fprintln(out, "MERGED VALUES:")
	return output.EncodeYAML(out, m.vals)
}

func (m mergeValuesWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, m.vals)
}

func (m mergeValuesWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, m.vals)
}