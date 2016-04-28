package main

import (
	"fmt"
	"os"

	"github.com/gosuri/uitable"
	"github.com/kubernetes/helm/pkg/repo"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func init() {
	repoCmd.AddCommand(repoAddCmd)
	repoCmd.AddCommand(repoListCmd)
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

func runRepoAdd(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("This command needs two argument, a name for the chart repository and the url of the chart repository")
	}

	err := insertRepoLine(args[0], args[1])
	if err != nil {
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
		fmt.Println("No repositories to show")
		return nil
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

func insertRepoLine(name, url string) error {
	err := checkUniqueName(name)
	if err != nil {
		return err
	}

	b, _ := yaml.Marshal(map[string]string{name: url})
	f, err := os.OpenFile(repositoriesFile(), os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(b)
	if err != nil {
		return err
	}

	return nil
}

func checkUniqueName(name string) error {
	file, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		return err
	}

	_, ok := file.Repositories[name]
	if ok {
		return fmt.Errorf("The repository name you provided (%s) already exists. Please specifiy a different name.", name)
	}
	return nil
}
