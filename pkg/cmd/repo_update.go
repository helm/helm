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
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/repo/v1"
)

const updateDesc = `
Update gets the latest information about charts from the respective chart repositories.
Information is cached locally, where it is used by commands like 'helm search'.

You can optionally specify a list of repositories you want to update.
	$ helm repo update <repo_name> ...
To update all the repositories, use 'helm repo update'.
`

var errNoRepositories = errors.New("no repositories found. You must add one before updating")

type repoUpdateOptions struct {
	update    func([]*repo.ChartRepository, io.Writer) error
	repoFile  string
	repoCache string
	names     []string
	timeout   time.Duration
}

func newRepoUpdateCmd(out io.Writer) *cobra.Command {
	o := &repoUpdateOptions{update: updateCharts}

	cmd := &cobra.Command{
		Use:     "update [REPO1 [REPO2 ...]]",
		Aliases: []string{"up"},
		Short:   "update information of available charts locally from chart repositories",
		Long:    updateDesc,
		Args:    require.MinimumNArgs(0),
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return compListRepos(toComplete, args), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(_ *cobra.Command, args []string) error {
			o.repoFile = settings.RepositoryConfig
			o.repoCache = settings.RepositoryCache
			o.names = args
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.DurationVar(&o.timeout, "timeout", getter.DefaultHTTPTimeout*time.Second, "time to wait for the index file download to complete")

	return cmd
}

func (o *repoUpdateOptions) run(out io.Writer) error {
	f, err := repo.LoadFile(o.repoFile)
	switch {
	case isNotExist(err):
		return errNoRepositories
	case err != nil:
		return fmt.Errorf("failed loading file: %s: %w", o.repoFile, err)
	case len(f.Repositories) == 0:
		return errNoRepositories
	}

	var repos []*repo.ChartRepository
	updateAllRepos := len(o.names) == 0

	if !updateAllRepos {
		// Fail early if the user specified an invalid repo to update
		if err := checkRequestedRepos(o.names, f.Repositories); err != nil {
			return err
		}
	}

	for _, cfg := range f.Repositories {
		if updateAllRepos || isRepoRequested(cfg.Name, o.names) {
			r, err := repo.NewChartRepository(cfg, getter.All(settings, getter.WithTimeout(o.timeout)))
			if err != nil {
				return err
			}
			if o.repoCache != "" {
				r.CachePath = o.repoCache
			}
			repos = append(repos, r)
		}
	}

	return o.update(repos, out)
}

func updateCharts(repos []*repo.ChartRepository, out io.Writer) error {
	fmt.Fprintln(out, "Hang tight while we grab the latest from your chart repositories...")
	var wg sync.WaitGroup
	failRepoURLChan := make(chan string, len(repos))

	writeMutex := sync.Mutex{}
	for _, re := range repos {
		wg.Add(1)
		go func(re *repo.ChartRepository) {
			defer wg.Done()
			if _, err := re.DownloadIndexFile(); err != nil {
				writeMutex.Lock()
				defer writeMutex.Unlock()
				fmt.Fprintf(out, "...Unable to get an update from the %q chart repository (%s):\n\t%s\n", re.Config.Name, re.Config.URL, err)
				failRepoURLChan <- re.Config.URL
			} else {
				writeMutex.Lock()
				defer writeMutex.Unlock()
				fmt.Fprintf(out, "...Successfully got an update from the %q chart repository\n", re.Config.Name)
			}
		}(re)
	}

	go func() {
		wg.Wait()
		close(failRepoURLChan)
	}()

	var repoFailList []string
	for url := range failRepoURLChan {
		repoFailList = append(repoFailList, url)
	}

	if len(repoFailList) > 0 {
		return fmt.Errorf("failed to update the following repositories: %s",
			repoFailList)
	}

	fmt.Fprintln(out, "Update Complete. ⎈Happy Helming!⎈")
	return nil
}

func checkRequestedRepos(requestedRepos []string, validRepos []*repo.Entry) error {
	for _, requestedRepo := range requestedRepos {
		found := false
		for _, repo := range validRepos {
			if requestedRepo == repo.Name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no repositories found matching '%s'.  Nothing will be updated", requestedRepo)
		}
	}
	return nil
}

func isRepoRequested(repoName string, requestedRepos []string) bool {
	return slices.Contains(requestedRepos, repoName)
}
