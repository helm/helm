package main

import (
	"os"
	"path/filepath"
)

const (
	repositoryDir          string = "repository"
	repositoriesFilePath   string = "repositories.yaml"
	cachePath              string = "cache"
	localRepoPath          string = "local"
	localRepoIndexFilePath string = "index.yaml"
)

func homePath() string {
	return os.ExpandEnv(helmHome)
}

// All other directories go under the $HELM_HOME/repository.
func repositoryDirectory() string {
	return homePath() + "/" + repositoryDir
}

func cacheDirectory(paths ...string) string {
	fragments := append([]string{repositoryDirectory(), cachePath}, paths...)
	return filepath.Join(fragments...)
}

func localRepoDirectory(paths ...string) string {
	fragments := append([]string{repositoryDirectory(), localRepoPath}, paths...)
	return filepath.Join(fragments...)
}

func repositoriesFile() string {
	return filepath.Join(repositoryDirectory(), repositoriesFilePath)
}
