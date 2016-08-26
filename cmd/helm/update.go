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
	"sync"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/repo"
)

var verboseUpdate bool

var updateCommand = &cobra.Command{
	Use:     "update",
	Aliases: []string{"up"},
	Short:   "update information on available charts in the chart repositories",
	RunE:    runUpdate,
}

func init() {
	updateCommand.Flags().BoolVar(&verboseUpdate, "verbose", false, "verbose error messages")
	RootCommand.AddCommand(updateCommand)
}

func runUpdate(cmd *cobra.Command, args []string) error {

	f, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		return err
	}

	if len(f.Repositories) == 0 {
		return errors.New("no repositories found. You must add one before updating")
	}

	updateCharts(f.Repositories, verboseUpdate)
	return nil
}

func updateCharts(repos map[string]string, verbose bool) {
	fmt.Println("Hang tight while we grab the latest from your chart repositories...")
	var wg sync.WaitGroup
	for name, url := range repos {
		wg.Add(1)
		go func(n, u string) {
			defer wg.Done()
			err := repo.DownloadIndexFile(n, u, cacheIndexFile(n))
			if err != nil {
				updateErr := "...Unable to get an update from the " + n + " chart repository"
				if verbose {
					updateErr = updateErr + ": " + err.Error()
				}
				fmt.Println(updateErr)
			} else {
				fmt.Println("...Successfully got an update from the " + n + " chart repository")
			}
		}(name, url)
	}
	wg.Wait()
	fmt.Println("Update Complete. Happy Helming!")
}
