package main

import (
	"os"

	"github.com/deis/tiller/pkg/repo"
	"github.com/spf13/cobra"
)

var serveDesc = `This command starts a local chart repository server that serves the charts saved in your $HELM_HOME/local/ directory.`

//TODO: add repoPath flag to be passed in in case you want
//  to serve charts from a different local dir

func init() {
	RootCommand.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a local http web server",
	Long:  serveDesc,
	Run:   ServeLocal,
}

func ServeLocal(cmd *cobra.Command, args []string) {
	localRepoPath := os.ExpandEnv(helmHome) + "/local/"
	repo.StartLocalRepo(localRepoPath)
}
