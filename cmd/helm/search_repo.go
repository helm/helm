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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/gosuri/uitable"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/search"
	"helm.sh/helm/v3/pkg/cli/output"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/repo"
)

const searchRepoDesc = `
Search reads through all of the repositories configured on the system, and
looks for matches. Search of these repositories uses the metadata stored on
the system.

It will display the latest stable versions of the charts found. If you
specify the --devel flag, the output will include pre-release versions.
If you want to search using a version constraint, use --version.

Examples:

    # Search for stable release versions matching the keyword "nginx"
    $ helm search repo nginx

    # Search for release versions matching the keyword "nginx", including pre-release versions
    $ helm search repo nginx --devel

    # Search for the latest stable release for nginx-ingress with a major version of 1
    $ helm search repo nginx-ingress --version ^1.0.0

Repositories are managed with 'helm repo' commands.
`

// searchMaxScore suggests that any score higher than this is not considered a match.
const searchMaxScore = 25

type searchRepoOptions struct {
	versions     bool
	regexp       bool
	devel        bool
	version      string
	maxColWidth  uint
	repoFile     string
	repoCacheDir string
	outputFormat output.Format
}

func newSearchRepoCmd(out io.Writer) *cobra.Command {
	o := &searchRepoOptions{}

	cmd := &cobra.Command{
		Use:   "repo [keyword]",
		Short: "search repositories for a keyword in charts",
		Long:  searchRepoDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.repoFile = settings.RepositoryConfig
			o.repoCacheDir = settings.RepositoryCache
			return o.run(out, args)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.regexp, "regexp", "r", false, "use regular expressions for searching repositories you have added")
	f.BoolVarP(&o.versions, "versions", "l", false, "show the long listing, with each version of each chart on its own line, for repositories you have added")
	f.BoolVar(&o.devel, "devel", false, "use development versions (alpha, beta, and release candidate releases), too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored")
	f.StringVar(&o.version, "version", "", "search using semantic versioning constraints on repositories you have added")
	f.UintVar(&o.maxColWidth, "max-col-width", 50, "maximum column width for output table")
	bindOutputFlag(cmd, &o.outputFormat)

	return cmd
}

func (o *searchRepoOptions) run(out io.Writer, args []string) error {
	o.setupSearchedVersion()

	index, err := o.buildIndex()
	if err != nil {
		return err
	}

	var res []*search.Result
	if len(args) == 0 {
		res = index.All()
	} else {
		q := strings.Join(args, " ")
		res, err = index.Search(q, searchMaxScore, o.regexp)
		if err != nil {
			return err
		}
	}

	search.SortScore(res)
	data, err := o.applyConstraint(res)
	if err != nil {
		return err
	}

	return o.outputFormat.Write(out, &repoSearchWriter{data, o.maxColWidth})
}

func (o *searchRepoOptions) setupSearchedVersion() {
	debug("Original chart version: %q", o.version)

	if o.version != "" {
		return
	}

	if o.devel { // search for releases and prereleases (alpha, beta, and release candidate releases).
		debug("setting version to >0.0.0-0")
		o.version = ">0.0.0-0"
	} else { // search only for stable releases, prerelease versions will be skip
		debug("setting version to >0.0.0")
		o.version = ">0.0.0"
	}
}

func (o *searchRepoOptions) applyConstraint(res []*search.Result) ([]*search.Result, error) {
	if o.version == "" {
		return res, nil
	}

	constraint, err := semver.NewConstraint(o.version)
	if err != nil {
		return res, errors.Wrap(err, "an invalid version/constraint format")
	}

	data := res[:0]
	foundNames := map[string]bool{}
	for _, r := range res {
		// if not returning all versions and already have found a result,
		// you're done!
		if !o.versions && foundNames[r.Name] {
			continue
		}
		v, err := semver.NewVersion(r.Chart.Version)
		if err != nil {
			continue
		}
		if constraint.Check(v) {
			data = append(data, r)
			foundNames[r.Name] = true
		}
	}

	return data, nil
}

func (o *searchRepoOptions) buildIndex() (*search.Index, error) {
	// Load the repositories.yaml
	rf, err := repo.LoadFile(o.repoFile)
	if isNotExist(err) || len(rf.Repositories) == 0 {
		return nil, errors.New("no repositories configured")
	}

	i := search.NewIndex()
	for _, re := range rf.Repositories {
		n := re.Name
		f := filepath.Join(o.repoCacheDir, helmpath.CacheIndexFile(n))
		ind, err := repo.LoadIndexFile(f)
		if err != nil {
			warning("Repo %q is corrupt or missing. Try 'helm repo update'.", n)
			warning("%s", err)
			continue
		}

		i.AddRepo(n, ind, o.versions || len(o.version) > 0)
	}
	return i, nil
}

type repoChartElement struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	AppVersion  string `json:"app_version"`
	Description string `json:"description"`
}

type repoSearchWriter struct {
	results     []*search.Result
	columnWidth uint
}

func (r *repoSearchWriter) WriteTable(out io.Writer) error {
	if len(r.results) == 0 {
		_, err := out.Write([]byte("No results found\n"))
		if err != nil {
			return fmt.Errorf("unable to write results: %s", err)
		}
		return nil
	}
	table := uitable.New()
	table.MaxColWidth = r.columnWidth
	table.AddRow("NAME", "CHART VERSION", "APP VERSION", "DESCRIPTION")
	for _, r := range r.results {
		table.AddRow(r.Name, r.Chart.Version, r.Chart.AppVersion, r.Chart.Description)
	}
	return output.EncodeTable(out, table)
}

func (r *repoSearchWriter) WriteJSON(out io.Writer) error {
	return r.encodeByFormat(out, output.JSON)
}

func (r *repoSearchWriter) WriteYAML(out io.Writer) error {
	return r.encodeByFormat(out, output.YAML)
}

func (r *repoSearchWriter) encodeByFormat(out io.Writer, format output.Format) error {
	// Initialize the array so no results returns an empty array instead of null
	chartList := make([]repoChartElement, 0, len(r.results))

	for _, r := range r.results {
		chartList = append(chartList, repoChartElement{r.Name, r.Chart.Version, r.Chart.AppVersion, r.Chart.Description})
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

// Provides the list of charts that are part of the specified repo, and that starts with 'prefix'.
func compListChartsOfRepo(repoName string, prefix string) []string {
	var charts []string

	path := filepath.Join(settings.RepositoryCache, helmpath.CacheChartsFile(repoName))
	content, err := ioutil.ReadFile(path)
	if err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(content))
		for scanner.Scan() {
			fullName := fmt.Sprintf("%s/%s", repoName, scanner.Text())
			if strings.HasPrefix(fullName, prefix) {
				charts = append(charts, fullName)
			}
		}
		return charts
	}

	if isNotExist(err) {
		// If there is no cached charts file, fallback to the full index file.
		// This is much slower but can happen after the caching feature is first
		// installed but before the user  does a 'helm repo update' to generate the
		// first cached charts file.
		path = filepath.Join(settings.RepositoryCache, helmpath.CacheIndexFile(repoName))
		if indexFile, err := repo.LoadIndexFile(path); err == nil {
			for name := range indexFile.Entries {
				fullName := fmt.Sprintf("%s/%s", repoName, name)
				if strings.HasPrefix(fullName, prefix) {
					charts = append(charts, fullName)
				}
			}
			return charts
		}
	}

	return []string{}
}

// Provide dynamic auto-completion for commands that operate on charts (e.g., helm show)
// When true, the includeFiles argument indicates that completion should include local files (e.g., local charts)
func compListCharts(toComplete string, includeFiles bool) ([]string, cobra.ShellCompDirective) {
	cobra.CompDebugln(fmt.Sprintf("compListCharts with toComplete %s", toComplete), settings.Debug)

	noSpace := false
	noFile := false
	var completions []string

	// First check completions for repos
	repos := compListRepos("", nil)
	for _, repo := range repos {
		repoWithSlash := fmt.Sprintf("%s/", repo)
		if strings.HasPrefix(toComplete, repoWithSlash) {
			// Must complete with charts within the specified repo
			completions = append(completions, compListChartsOfRepo(repo, toComplete)...)
			noSpace = false
			break
		} else if strings.HasPrefix(repo, toComplete) {
			// Must complete the repo name
			completions = append(completions, repoWithSlash)
			noSpace = true
		}
	}
	cobra.CompDebugln(fmt.Sprintf("Completions after repos: %v", completions), settings.Debug)

	// Now handle completions for url prefixes
	for _, url := range []string{"https://", "http://", "file://"} {
		if strings.HasPrefix(toComplete, url) {
			// The user already put in the full url prefix; we don't have
			// anything to add, but make sure the shell does not default
			// to file completion since we could be returning an empty array.
			noFile = true
			noSpace = true
		} else if strings.HasPrefix(url, toComplete) {
			// We are completing a url prefix
			completions = append(completions, url)
			noSpace = true
		}
	}
	cobra.CompDebugln(fmt.Sprintf("Completions after urls: %v", completions), settings.Debug)

	// Finally, provide file completion if we need to.
	// We only do this if:
	// 1- There are other completions found (if there are no completions,
	//    the shell will do file completion itself)
	// 2- If there is some input from the user (or else we will end up
	//    listing the entire content of the current directory which will
	//    be too many choices for the user to find the real repos)
	if includeFiles && len(completions) > 0 && len(toComplete) > 0 {
		if files, err := ioutil.ReadDir("."); err == nil {
			for _, file := range files {
				if strings.HasPrefix(file.Name(), toComplete) {
					// We are completing a file prefix
					completions = append(completions, file.Name())
				}
			}
		}
	}
	cobra.CompDebugln(fmt.Sprintf("Completions after files: %v", completions), settings.Debug)

	// If the user didn't provide any input to completion,
	// we provide a hint that a path can also be used
	if includeFiles && len(toComplete) == 0 {
		completions = append(completions, "./", "/")
	}
	cobra.CompDebugln(fmt.Sprintf("Completions after checking empty input: %v", completions), settings.Debug)

	directive := cobra.ShellCompDirectiveDefault
	if noFile {
		directive = directive | cobra.ShellCompDirectiveNoFileComp
	}
	if noSpace {
		directive = directive | cobra.ShellCompDirectiveNoSpace
	}
	if !includeFiles {
		// If we should not include files in the completions,
		// we should disable file completion
		directive = directive | cobra.ShellCompDirectiveNoFileComp
	}
	return completions, directive
}
