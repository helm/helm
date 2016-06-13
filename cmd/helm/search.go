package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/repo"
)

func init() {
	RootCommand.AddCommand(searchCmd)
}

var searchCmd = &cobra.Command{
	Use:     "search [keyword]",
	Short:   "Search for a keyword in charts",
	Long:    "Searches the known repositories cache files for the specified search string, looks at name and keywords",
	RunE:    search,
	PreRunE: requireInit,
}

func search(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errors.New("This command needs at least one argument (search string)")
	}

	results, err := searchCacheForPattern(cacheDirectory(), args[0])
	if err != nil {
		return err
	}
	if len(results) > 0 {
		for _, result := range results {
			fmt.Println(result)
		}
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
		if c.Chartfile == nil {
			continue
		}
		for _, keyword := range c.Chartfile.Keywords {
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
		index, err := repo.LoadIndexFile(f)
		if err != nil {
			return matches, fmt.Errorf("index %s corrupted: %s", f, err)
		}

		m := searchChartRefsForPattern(search, index.Entries)
		repoName := strings.TrimSuffix(filepath.Base(f), "-index.yaml")
		for _, c := range m {
			// TODO: Is it possible for this file to be missing? Or to have
			// an extension other than .tgz? Should the actual filename be in
			// the YAML?
			fname := filepath.Join(repoName, c+".tgz")
			matches = append(matches, fname)
		}
	}
	return matches, nil
}
