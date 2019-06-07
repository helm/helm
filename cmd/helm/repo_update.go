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
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/installer"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
)

const updateDesc = `
Update gets the latest information about charts from the respective chart repositories.
Information is cached locally, where it is used by commands like 'helm search'.

'helm update' is the deprecated form of 'helm repo update'. It will be removed in
future releases.

You can specify the name of a repository you want to update.

	$ helm repo update <repo_name>

To update all the repositories, use 'helm repo update'.

`

var errNoRepositories = errors.New("no repositories found. You must add one before updating")
var errNoRepositoriesMatchingRepoName = errors.New("no repositories found matching the provided name. Verify if the repo exists")

type repoUpdateCmd struct {
	update func([]*repo.ChartRepository, io.Writer, helmpath.Home, bool) error
	home   helmpath.Home
	out    io.Writer
	strict bool
	name   string
}

func newRepoUpdateCmd(out io.Writer) *cobra.Command {
	u := &repoUpdateCmd{
		out:    out,
		update: updateCharts,
	}
	cmd := &cobra.Command{
		Use:     "update [REPO_NAME]",
		Aliases: []string{"up"},
		Short:   "Update information of available charts locally from chart repositories",
		Long:    updateDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			u.home = settings.Home
			if len(args) != 0 {
				u.name = args[0]
			}
			return u.run()
		},
	}

	f := cmd.Flags()
	f.BoolVar(&u.strict, "strict", false, "Fail on update warnings")

	return cmd
}

func (u *repoUpdateCmd) run() error {
	f, err := repo.LoadRepositoriesFile(u.home.RepositoryFile())
	if err != nil {
		return err
	}

	if len(f.Repositories) == 0 {
		return errNoRepositories
	}
	var repos []*repo.ChartRepository
	for _, cfg := range f.Repositories {
		r, err := repo.NewChartRepository(cfg, getter.All(settings))
		if err != nil {
			return err
		}
		if len(u.name) != 0 {
			if cfg.Name == u.name {
				repos = append(repos, r)
				break
			} else {
				continue
			}
		} else {
			repos = append(repos, r)
		}
	}

	if len(repos) == 0 {
		return errNoRepositoriesMatchingRepoName
	}

	return u.update(repos, u.out, u.home, u.strict)
}

func updateCharts(repos []*repo.ChartRepository, out io.Writer, home helmpath.Home, strict bool) error {
	fmt.Fprintln(out, "Hang tight while we grab the latest from your chart repositories...")
	var (
		errorCounter int
		wg           sync.WaitGroup
		mu           sync.Mutex
	)
	for _, re := range repos {
		wg.Add(1)
		go func(re *repo.ChartRepository) {
			defer wg.Done()
			if re.Config.Name == installer.LocalRepository {
				mu.Lock()
				fmt.Fprintf(out, "...Skip %s chart repository\n", re.Config.Name)
				mu.Unlock()
				return
			}
			err := re.DownloadIndexFile(home.Cache())
			if err != nil {
				mu.Lock()
				errorCounter++
				fmt.Fprintf(out, "...Unable to get an update from the %q chart repository (%s):\n\t%s\n", re.Config.Name, re.Config.URL, err)
				mu.Unlock()
			} else {
				mu.Lock()
				fmt.Fprintf(out, "...Successfully got an update from the %q chart repository\n", re.Config.Name)
				mu.Unlock()
			}
		}(re)
	}
	wg.Wait()

	if errorCounter != 0 && strict {
		return errors.New("Update Failed. Check log for details")
	}

	fmt.Fprintln(out, "Update Complete.")
	return nil
}
