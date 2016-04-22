package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/deis/tiller/pkg/client"
	"github.com/deis/tiller/pkg/kubectl"
	"github.com/spf13/cobra"
)

const initDesc = `
This command installs Tiller (the helm server side component) onto your
Kubernetes Cluster and sets up local configuration in $HELM_HOME (default: ~/.helm/)
`

const repositoriesPath = ".repositories"
const cachePath = "cache"
const localPath = "local"
const localCacheFilePath = localPath + "/cache.yaml"

var defaultRepo = map[string]string{"default-name": "default-url"}
var tillerImg string

func init() {
	initCmd.Flags().StringVarP(&tillerImg, "tiller-image", "i", "", "override tiller image")
	RootCommand.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Helm on both client and server.",
	Long:  installDesc,
	RunE:  runInit,
}

// runInit initializes local config and installs tiller to Kubernetes Cluster
func runInit(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("This command does not accept arguments. \n")
	}

	if err := ensureHome(os.ExpandEnv(helmHome)); err != nil {
		return err
	}

	if err := installTiller(); err != nil {
		return err
	}

	fmt.Printf("Tiller (the helm server side component) has been installed into your Kubernetes Cluster.\n$HELM_HOME has also been configured at %s.\nHappy Helming!\n", helmHome)
	return nil
}

func installTiller() error {
	// TODO: take value of global flag kubectl and pass that in
	runner := buildKubectlRunner("")

	i := client.NewInstaller()
	i.Tiller["Image"] = tillerImg
	out, err := i.Install(runner)

	if err != nil {
		return fmt.Errorf("error installing %s %s", string(out), err)
	}

	return nil
}

func buildKubectlRunner(kubectlPath string) kubectl.Runner {
	if kubectlPath != "" {
		kubectl.Path = kubectlPath
	}
	return &kubectl.RealRunner{}
}

// ensureHome checks to see if $HELM_HOME exists
//
// If $HELM_HOME does not exist, this function will create it.
func ensureHome(home string) error {
	configDirectories := []string{home, cacheDirectory(home), localDirectory(home)}

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

	repoPath := repositoriesFile(home)
	if fi, err := os.Stat(repoPath); err != nil {
		fmt.Printf("Creating %s \n", repoPath)
		if err := ioutil.WriteFile(repoPath, []byte("local: localhost:8879/charts\n"), 0644); err != nil {
			return err
		}
	} else if fi.IsDir() {
		return fmt.Errorf("%s must be a file, not a directory", repoPath)
	}

	localCacheFile := localDirCacheFile(home)
	if fi, err := os.Stat(localCacheFile); err != nil {
		fmt.Printf("Creating %s \n", localCacheFile)
		_, err := os.Create(localCacheFile)
		if err != nil {
			return err
		}

		//TODO: take this out and replace with helm update functionality
		os.Symlink(localCacheFile, cacheDirectory(home)+"/local-cache.yaml")
	} else if fi.IsDir() {
		return fmt.Errorf("%s must be a file, not a directory", repoPath)
	}
	return nil
}

func cacheDirectory(home string) string {
	return filepath.Join(home, cachePath)
}

func repositoriesFile(home string) string {
	return filepath.Join(home, repositoriesPath)
}

func localDirectory(home string) string {
	return filepath.Join(home, localPath)
}

func localDirCacheFile(home string) string {
	return filepath.Join(home, localCacheFilePath)
}
