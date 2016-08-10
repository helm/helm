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
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/repo"
)

var serveDesc = `This command starts a local chart repository server that serves charts from a local directory.`
var repoPath string

func init() {
	serveCmd.Flags().StringVar(&repoPath, "repo-path", localRepoDirectory(), "The local directory path from which to serve charts.")
	RootCommand.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "start a local http web server",
	Long:  serveDesc,
	RunE:  serve,
}

func serve(cmd *cobra.Command, args []string) error {

	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return err
	}

	repo.StartLocalRepo(repoPath)
	return nil
}
