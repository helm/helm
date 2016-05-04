package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/kubernetes/helm/pkg/repo"
)

var verboseUpdate bool

var updateCommand = &cobra.Command{
	Use:   "update",
	Short: "Update information on available charts in the chart repositories.",
	RunE:  update,
}

func init() {
	updateCommand.Flags().BoolVar(&verboseUpdate, "verbose", false, "verbose error messages")
	RootCommand.AddCommand(updateCommand)
}

func update(cmd *cobra.Command, args []string) error {

	f, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		return err
	}

	updateCharts(f.Repositories, verboseUpdate)
	return nil
}

func updateCharts(repos map[string]string, verbose bool) {
	fmt.Println("Hang tight while we grab the latest from your chart repositories...")
	var wg sync.WaitGroup
	for name, url := range repos {
		wg.Add(1)
		go downloadCacheFile(name, url, verbose, &wg)
	}
	wg.Wait()
	fmt.Println("Update Complete. Happy Helming!")
}

func downloadCacheFile(name, url string, verbose bool, wg *sync.WaitGroup) {
	defer wg.Done()
	var cacheURL string
	updateErr := "...Unable to get an update from the " + name + " chart repository"
	fail := false

	cacheURL = strings.TrimSuffix(url, "/") + "/cache.yaml"
	resp, err := http.Get(cacheURL)
	if err != nil {
		fail = true
		if verbose {
			fmt.Println(updateErr + ": " + err.Error())
		} else {
			fmt.Println(updateErr)
		}
	}

	var cacheFile *os.File
	if !fail {
		defer resp.Body.Close()
		cacheFile, err = os.Create(cacheDirectory(name + "-cache.yaml"))
		if err != nil {
			fail = true
			if verbose {
				fmt.Println(updateErr + ": " + err.Error())
			} else {
				fmt.Println(updateErr)
			}
		}
	}

	if !fail {
		if _, err := io.Copy(cacheFile, resp.Body); err != nil {
			fail = true
			if verbose {
				fmt.Println(updateErr + ": " + err.Error())
			} else {
				fmt.Println(updateErr)
			}
		}
	}

	if !fail {
		fmt.Println("...Successfully got an update from the " + name + " chart repository")
	}
}
