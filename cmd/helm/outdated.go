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
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/gosuri/uitable"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/cmd/helm/search"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/output"
	"helm.sh/helm/v3/pkg/release"
)

var outdatedHelp = `
This Command lists all releases which are outdated.

By default, the output is printed in a Table but you can change this behavior
with the '--output' Flag.
`

func newOutdatedCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewList(cfg)
	var outfmt output.Format

	cmd := &cobra.Command{
		Use:     "outdated",
		Short:   "list outdated releases",
		Long:    outdatedHelp,
		Aliases: []string{"od"},
		Args:    require.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if client.AllNamespaces {
				if err := cfg.Init(settings.RESTClientGetter(), "", os.Getenv("HELM_DRIVER"), debug); err != nil {
					return err
				}
			}
			client.SetStateMask()

			releases, err := client.Run()
			if err != nil {
				return err
			}

			devel, err := cmd.Flags().GetBool("devel")
			if err != nil {
				return err
			}
			return outfmt.Write(out, newOutdatedListWriter(releases, cfg, out, devel))
		},
	}

	flags := cmd.Flags()
	flags.Bool("devel", false, "use development versions (alpha, beta, and release candidate releases), too. Equivalent to version '>0.0.0-0'.")
	flags.BoolVarP(&client.Short, "short", "q", false, "output short (quiet) listing format")
	flags.BoolVarP(&client.ByDate, "date", "d", false, "sort by release date")
	flags.BoolVarP(&client.SortReverse, "reverse", "r", false, "reverse the sort order")
	flags.BoolVarP(&client.All, "all", "a", false, "show all releases, not just the ones marked deployed or failed")
	flags.BoolVar(&client.Uninstalled, "uninstalled", false, "show uninstalled releases")
	flags.BoolVar(&client.Superseded, "superseded", false, "show superseded releases")
	flags.BoolVar(&client.Uninstalling, "uninstalling", false, "show releases that are currently being uninstalled")
	flags.BoolVar(&client.Deployed, "deployed", false, "show deployed releases. If no other is specified, this will be automatically enabled")
	flags.BoolVar(&client.Failed, "failed", false, "show failed releases")
	flags.BoolVar(&client.Pending, "pending", false, "show pending releases")
	flags.BoolVarP(&client.AllNamespaces, "all-namespaces", "A", false, "list releases across all namespaces")
	flags.IntVarP(&client.Limit, "max", "m", 256, "maximum number of releases to fetch")
	flags.IntVar(&client.Offset, "offset", 0, "next release name in the list, used to offset from start value")
	bindOutputFlag(cmd, &outfmt)

	return cmd
}

type outdatedElement struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	InstalledVer string `json:"installed_version"`
	LatestVer    string `json:"latest_version"`
	Chart        string `json:"chart"`
}

type outdatedListWriter struct {
	releases []outdatedElement // Outdated releases
}

func newOutdatedListWriter(releases []*release.Release, cfg *action.Configuration, out io.Writer, devel bool) *outdatedListWriter {
	outdated := make([]outdatedElement, 0, len(releases))

	// we initialize the Struct with default Options but the 'devel' option
	// can be set by the User, all the other ones are not relevant.
	searchRepo := searchRepoOptions{
		versions:     false,
		regexp:       false,
		devel:        devel,
		maxColWidth:  50,
		version:      "",
		repoFile:     settings.RepositoryConfig,
		repoCacheDir: settings.RepositoryCache,
	}

	// initialize Repo index first
	index, err := initSearch(out, &searchRepo)
	if err != nil {
		// TODO: Find a better way to exit
		fmt.Fprintf(out, "%s", errors.Wrap(err, "ERROR: Could not initialize search index").Error())
		os.Exit(1)
	}

	results := index.All()
	for _, r := range releases {
		// search if it exists a newer Chart in the Chart-Repository
		repoResult, err := searchChart(results, r.Chart.Name(), r.Chart.Metadata.Version, devel)
		if err != nil {
			fmt.Fprintf(out, "%s", errors.Wrap(err, "ERROR: Could not initialize search index").Error())
			os.Exit(1)
		}

		// skip if no newer Chart was found
		if repoResult == nil {
			continue
		}

		outdated = append(outdated, outdatedElement{
			Name:         r.Name,
			Namespace:    r.Namespace,
			InstalledVer: r.Chart.Metadata.Version,
			LatestVer:    repoResult.Chart.Metadata.Version,
			Chart:        repoResult.Chart.Name,
		})
	}

	return &outdatedListWriter{outdated}
}

func initSearch(out io.Writer, o *searchRepoOptions) (*search.Index, error) {
	index, err := o.buildIndex(out)
	if err != nil {
		return nil, err
	}

	return index, nil
}

// searchChart searches for Repositories which are containing that chart.
//
// It will return a Pointer to the Chart Result (the Pointer points to the
// Result of the Index).
// If no results are found, nil will be returned instead of type *Result.
func searchChart(r []*search.Result, name string, chartVersion string, devel bool) (*search.Result, error) {
	found := false // found describres if Charts where found but no one is newer than the actual one

	// TODO: implement a better Searchalgorithm. Because this is an
	// linear search algorithm so it takes O(len(r)) steps in the worst case
	for _, result := range r {
		// check if the Chart-Result Name is that one we are searching for.
		if strings.HasSuffix(strings.ToLower(result.Name), strings.ToLower(name)) {
			// check if Version is newer than the actual one
			version, err := semver.NewVersion(result.Chart.Metadata.Version)
			if err != nil {
				return nil, err
			}

			var constrainStr string
			if devel {
				constrainStr = "> " + chartVersion + "-0" + " != " + chartVersion
			} else {
				constrainStr = "> " + chartVersion
			}

			constrain, err := semver.NewConstraint(constrainStr)
			if err != nil {
				return nil, err
			}

			debug("Comparing version of original chart '%s' => %s with version (%s) %s",
				name, chartVersion, result.Name, result.Chart.Metadata.Version)
			debug("Using '%s' as constrain against '%s'", constrainStr, result.Chart.Metadata.Version)
			if constrain.Check(version) {
				return result, nil
			} else {
				// set 'found' to true because a Repository contains
				// the Chart but the Version is not newer than
				// the installed one.
				found = true
			}
		}
	}

	if !found {
		debug("Could not find any Repo which contains %s", name)
		return nil, errors.New(fmt.Sprintf("Could not find any Repo which contains %s", name))
	}

	debug("No newer Chart was found for '%s'", name)
	return nil, nil
}

func (r *outdatedListWriter) WriteTable(out io.Writer) error {
	table := uitable.New()
	table.AddRow("NAME", "NAMESPACE", "INSTALLED VERSION", "LATEST VERSION", "CHART")
	for _, r := range r.releases {
		table.AddRow(r.Name, r.Namespace, r.InstalledVer, r.LatestVer, r.Chart)
	}
	return output.EncodeTable(out, table)
}

func (r *outdatedListWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, r.releases)
}

func (r *outdatedListWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, r.releases)
}
