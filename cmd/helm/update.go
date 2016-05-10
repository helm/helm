package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/kubernetes/helm/pkg/repo"
)

var verboseUpdate bool

var updateCommand = &cobra.Command{
	Use:   "update",
	Short: "Update information on available charts in the chart repositories.",
	RunE:  runUpdate,
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
			err := downloadCacheFile(n, u)
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

func downloadCacheFile(name, url string) error {
	var cacheURL string

	cacheURL = strings.TrimSuffix(url, "/") + "/cache.yaml"
	resp, err := http.Get(cacheURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var cacheFile *os.File
	var r repo.RepoFile

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(b, &r); err != nil {
		return err
	}

	cacheFile, err = os.Create(cacheDirectory(name + "-cache.yaml"))
	if err != nil {
		return err
	}

	if _, err := io.Copy(cacheFile, resp.Body); err != nil {
		return err
	}

	return nil
}
