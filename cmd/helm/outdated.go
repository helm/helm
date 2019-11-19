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
	flags.BoolVarP(&client.AllNamespaces, "all-namespaces", "A", false, "list releases across all namespaces")
	flags.Bool("devel", false, "use development versions (alpha, beta, and release candidate releases), too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored")
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
		repoResult, err := searchChart(results, r.Name)
		if err != nil {
			fmt.Fprintf(out, "%s", errors.Wrap(err, "ERROR: Could not initialize search index").Error())
			os.Exit(1)
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
func searchChart(r []*search.Result, name string) (*search.Result, error) {
	// TODO: implement a better Searchalgorithm.
	for _, result := range r {
		if strings.Contains(strings.ToLower(result.Name), strings.ToLower(name)) {
			return result, nil
		}
	}

	debug("Could not find any Repo which contains %s", name)
	return nil, errors.New(fmt.Sprintf("Could not find any Repo which contains %s", name))
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
