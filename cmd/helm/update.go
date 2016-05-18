package main

import (
	"errors"
	"fmt"
	"sync"

	"github.com/spf13/cobra"

	"github.com/kubernetes/helm/pkg/repo"
)

var verboseUpdate bool

var updateCommand = &cobra.Command{
	Use:     "update",
	Aliases: []string{"up"},
	Short:   "Update information on available charts in the chart repositories.",
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
			indexFileName := cacheDirectory(n + "-index.yaml")
			err := repo.DownloadIndexFile(n, u, indexFileName)
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
