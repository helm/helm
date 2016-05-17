package main

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/gosuri/uitable"
	"github.com/kubernetes/helm/pkg/repo"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func init() {
	repoCmd.AddCommand(repoAddCmd)
	repoCmd.AddCommand(repoListCmd)
	repoCmd.AddCommand(repoRemoveCmd)
	RootCommand.AddCommand(repoCmd)
}

var repoCmd = &cobra.Command{
	Use:   "repo add|remove|list [ARG]",
	Short: "add, list, or remove chart repositories",
}

var repoAddCmd = &cobra.Command{
	Use:   "add [flags] [NAME] [URL]",
	Short: "add a chart repository",
	RunE:  runRepoAdd,
}

var repoListCmd = &cobra.Command{
	Use:   "list [flags]",
	Short: "list chart repositories",
	RunE:  runRepoList,
}

var repoRemoveCmd = &cobra.Command{
	Use:   "remove [flags] [NAME]",
	Short: "remove a chart repository",
	RunE:  runRepoRemove,
}

func runRepoAdd(cmd *cobra.Command, args []string) error {
	if err := checkArgsLength(2, len(args), "name for the chart repository", "the url of the chart repository"); err != nil {
		return err
	}
	name, url := args[0], args[1]

	if err := repo.DownloadIndexFile(name, url, cacheDirectory(name, "-index.yaml")); err != nil {
		return errors.New("Oops! Looks like " + url + " is not a valid chart repository or cannot be reached\n")
	}

	if err := insertRepoLine(name, url); err != nil {
		return err
	}

	fmt.Println(args[0] + " has been added to your repositories")
	return nil
}

func runRepoList(cmd *cobra.Command, args []string) error {
	f, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		return err
	}
	if len(f.Repositories) == 0 {
		return errors.New("no repositories to show")
	}
	table := uitable.New()
	table.MaxColWidth = 50
	table.AddRow("NAME", "URL")
	for k, v := range f.Repositories {
		table.AddRow(k, v)
	}
	fmt.Println(table)
	return nil
}

func runRepoRemove(cmd *cobra.Command, args []string) error {
	if err := checkArgsLength(1, len(args), "name of chart repository"); err != nil {
		return err
	}
	if err := removeRepoLine(args[0]); err != nil {
		return err
	}
	return nil
}

func removeRepoLine(name string) error {
	r, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		return err
	}

	_, ok := r.Repositories[name]
	if ok {
		delete(r.Repositories, name)
		b, err := yaml.Marshal(&r.Repositories)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(repositoriesFile(), b, 0666); err != nil {
			return err
		}

	} else {
		return fmt.Errorf("The repository, %s, does not exist in your repositories list", name)
	}

	return nil
}

func insertRepoLine(name, url string) error {
	f, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		return err
	}
	_, ok := f.Repositories[name]
	if ok {
		return fmt.Errorf("The repository name you provided (%s) already exists. Please specifiy a different name.", name)
	}

	if f.Repositories == nil {
		f.Repositories = make(map[string]string)
	}

	f.Repositories[name] = url

	b, _ := yaml.Marshal(&f.Repositories)
	if err := ioutil.WriteFile(repositoriesFile(), b, 0666); err != nil {
		return err
	}

	return nil
}
