package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubernetes/helm/pkg/repo"
	"github.com/spf13/cobra"
)

func init() {
	RootCommand.AddCommand(searchCmd)
}

var searchCmd = &cobra.Command{
	Use:   "search [CHART]",
	Short: "Search for charts",
	Long:  "", //TODO: add search command description
	RunE:  search,
}

func search(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errors.New("This command needs at least one argument (search string)")
	}

	results, err := searchCacheForPattern(args[0], cacheDirectory())
	if err != nil {
		return err
	}
	cmd.Println("Charts:")
	for _, result := range results {
		fmt.Println(result)
	}
	return nil
}

func searchChartRefsForPattern(search string, chartRefs map[string]*repo.ChartRef) []string {
	matches := []string{}
	for k, c := range chartRefs {
		if strings.Contains(c.Name, search) {
			matches = append(matches, k)
			continue
		}
		for _, keyword := range c.Keywords {
			if strings.Contains(keyword, search) {
				matches = append(matches, k)
			}
		}
	}
	return matches
}

func searchCacheForPattern(dir string, search string) ([]string, error) {
	fileList := []string{}
	filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			fileList = append(fileList, path)
		}
		return nil
	})
	matches := []string{}
	for _, f := range fileList {
		cache, _ := repo.LoadCacheFile(f)
		m := searchChartRefsForPattern(search, cache.Entries)
		repoName := strings.TrimSuffix(filepath.Base(f), "-cache.yaml")
		for _, c := range m {
			matches = append(matches, repoName+"/"+c)
		}
	}
	return matches, nil
}
