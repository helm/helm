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
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/client"
)

const initDesc = `
This command installs Tiller (the helm server side component) onto your
Kubernetes Cluster and sets up local configuration in $HELM_HOME (default: ~/.helm/)
`

var (
	tillerImg            string
	clientOnly           bool
	defaultRepository    = "kubernetes-charts"
	defaultRepositoryURL = "http://storage.googleapis.com/kubernetes-charts"
)

func init() {
	f := initCmd.Flags()
	f.StringVarP(&tillerImg, "tiller-image", "i", "", "override tiller image")
	f.BoolVarP(&clientOnly, "client-only", "c", false, "If set does not install tiller")
	RootCommand.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "initialize Helm on both client and server",
	Long:  initDesc,
	RunE:  runInit,
}

// runInit initializes local config and installs tiller to Kubernetes Cluster
func runInit(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("This command does not accept arguments. \n")
	}

	if err := ensureHome(); err != nil {
		return err
	}

	if !clientOnly {
		if err := installTiller(); err != nil {
			return err
		}
	} else {
		fmt.Println("Not installing tiller due to 'client-only' flag having been set")
	}

	fmt.Println("Happy Helming!")
	return nil
}

func installTiller() error {
	if err := client.Install(tillerNamespace, tillerImg, flagDebug); err != nil {
		return fmt.Errorf("error installing: %s", err)
	}
	fmt.Println("\nTiller (the helm server side component) has been installed into your Kubernetes Cluster.")

	return nil
}

// requireHome checks to see if $HELM_HOME exists, and returns an error if it does not.
func requireHome() error {
	dirs := []string{homePath(), repositoryDirectory(), cacheDirectory(), localRepoDirectory()}
	for _, d := range dirs {
		if fi, err := os.Stat(d); err != nil {
			return fmt.Errorf("directory %q is not configured", d)
		} else if !fi.IsDir() {
			return fmt.Errorf("expected %q to be a directory", d)
		}
	}
	return nil
}

// ensureHome checks to see if $HELM_HOME exists
//
// If $HELM_HOME does not exist, this function will create it.
func ensureHome() error {
	configDirectories := []string{homePath(), repositoryDirectory(), cacheDirectory(), localRepoDirectory()}

	for _, p := range configDirectories {
		if fi, err := os.Stat(p); err != nil {
			fmt.Printf("Creating %s \n", p)
			if err := os.MkdirAll(p, 0755); err != nil {
				return fmt.Errorf("Could not create %s: %s", p, err)
			}
		} else if !fi.IsDir() {
			return fmt.Errorf("%s must be a directory", p)
		}
	}

	repoFile := repositoriesFile()
	if fi, err := os.Stat(repoFile); err != nil {
		fmt.Printf("Creating %s \n", repoFile)
		if _, err := os.Create(repoFile); err != nil {
			return err
		}
		if err := addRepository(defaultRepository, defaultRepositoryURL); err != nil {
			return err
		}
	} else if fi.IsDir() {
		return fmt.Errorf("%s must be a file, not a directory", repoFile)
	}

	localRepoIndexFile := localRepoDirectory(localRepoIndexFilePath)
	if fi, err := os.Stat(localRepoIndexFile); err != nil {
		fmt.Printf("Creating %s \n", localRepoIndexFile)
		_, err := os.Create(localRepoIndexFile)
		if err != nil {
			return err
		}

		//TODO: take this out and replace with helm update functionality
		os.Symlink(localRepoIndexFile, cacheDirectory("local-index.yaml"))
	} else if fi.IsDir() {
		return fmt.Errorf("%s must be a file, not a directory", localRepoIndexFile)
	}

	fmt.Printf("$HELM_HOME has been configured at %s.\n", helmHome)
	return nil
}
