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

	"k8s.io/helm/pkg/repo"
)

const updateDesc = `
Update gets the latest information about charts from the respective chart repositories.
Information is cached locally, where it is used by commands like 'helm search'.
`

type updateCmd struct {
	repoFile string
	update   func(map[string]string, bool, io.Writer)
	out      io.Writer
}

func newUpdateCmd(out io.Writer) *cobra.Command {
	u := &updateCmd{
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
			return u.run()
		},
	}
	return cmd
}

func (u *updateCmd) run() error {
	f, err := repo.LoadRepositoriesFile(u.repoFile)
	if err != nil {
		return err
	}

	if len(f.Repositories) == 0 {
		return errors.New("no repositories found. You must add one before updating")
	}

	u.update(f.Repositories, flagDebug, u.out)
	return nil
}

func updateCharts(repos map[string]string, verbose bool, out io.Writer) {
	fmt.Fprintln(out, "Hang tight while we grab the latest from your chart repositories...")
	var wg sync.WaitGroup
	for name, url := range repos {
		wg.Add(1)
		go func(n, u string) {
			defer wg.Done()
			err := repo.DownloadIndexFile(n, u, cacheIndexFile(n))
			if err != nil {
				updateErr := fmt.Sprintf("...Unable to get an update from the %q chart repository", n)
				if verbose {
					updateErr = updateErr + ": " + err.Error()
				}
				fmt.Fprintln(out, updateErr)
			} else {
				fmt.Fprintf(out, "...Successfully got an update from the %q chart repository\n", n)
			}
		}(name, url)
	}
	wg.Wait()
	fmt.Fprintln(out, "Update Complete. Happy Helming!")
}
