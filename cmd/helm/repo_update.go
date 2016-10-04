/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/pkg/repo"
)

const updateDesc = `
Update gets the latest information about charts from the respective chart repositories.
Information is cached locally, where it is used by commands like 'helm search'.

'helm update' is the deprecated form of 'helm repo update'. It will be removed in
future releases.
`

type repoUpdateCmd struct {
	repoFile string
	update   func([]*repo.Entry, bool, io.Writer, helmpath.Home)
	out      io.Writer
	home     helmpath.Home
}

func newRepoUpdateCmd(out io.Writer) *cobra.Command {
	u := &repoUpdateCmd{
		out:      out,
		update:   updateCharts,
		repoFile: repositoriesFile(),
	}
	cmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"up"},
		Short:   "update information on available charts in the chart repositories",
		Long:    updateDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			u.home = helmpath.Home(homePath())
			return u.run()
		},
	}
	return cmd
}

func (u *repoUpdateCmd) run() error {
	f, err := repo.LoadRepositoriesFile(u.repoFile)
	if err != nil {
		return err
	}

	if len(f.Repositories) == 0 {
		return errors.New("no repositories found. You must add one before updating")
	}

	u.update(f.Repositories, flagDebug, u.out, u.home)
	return nil
}

func updateCharts(repos []*repo.Entry, verbose bool, out io.Writer, home helmpath.Home) {
	fmt.Fprintln(out, "Hang tight while we grab the latest from your chart repositories...")
	var wg sync.WaitGroup
	for _, re := range repos {
		wg.Add(1)
		go func(n, u string) {
			defer wg.Done()
			if n == localRepository {
				// We skip local because the indices are symlinked.
				return
			}
			err := repo.DownloadIndexFile(n, u, home.CacheIndex(n))
			if err != nil {
				fmt.Fprintf(out, "...Unable to get an update from the %q chart repository (%s):\n\t%s\n", n, u, err)
			} else {
				fmt.Fprintf(out, "...Successfully got an update from the %q chart repository\n", n)
			}
		}(re.Name, re.URL)
	}
	wg.Wait()
	fmt.Fprintln(out, "Update Complete. ⎈ Happy Helming!⎈ ")
}
