package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deis/tiller/pkg/repo"
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
		return errors.New("This command needs at least one argument")
	}

	results, err := searchCacheForPattern(args[0])
	if err != nil {
		return err
	}
	cmd.Println("Charts:")
	for _, result := range results {
		fmt.Println(result)
	}
	return nil
}

func searchCacheForPattern(name string) ([]string, error) {
	fileList := []string{}
	filepath.Walk(cachePath, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			fileList = append(fileList, path)
		}
		return nil
	})
	matches := []string{}
	for _, f := range fileList {
		cache, _ := repo.LoadCacheFile(f)
		repoName := filepath.Base(strings.TrimRight(f, "-cache.txt"))
		for k := range cache.Entries {
			if strings.Contains(k, name) {
				matches = append(matches, repoName+"/"+k)
			}
		}
	}
	return matches, nil
}
