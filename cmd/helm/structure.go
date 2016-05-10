package main

import (
	"os"
	"path/filepath"
)

const (
	repositoriesFilePath   string = "repositories.yaml"
	cachePath              string = "cache"
	localRepoPath          string = "local"
	localRepoIndexFilePath string = "index.yaml"
)

func homePath() string {
	return os.ExpandEnv(helmHome)
}

func cacheDirectory(paths ...string) string {
	fragments := append([]string{homePath(), cachePath}, paths...)
	return filepath.Join(fragments...)
}

func localRepoDirectory(paths ...string) string {
	fragments := append([]string{homePath(), localRepoPath}, paths...)
	return filepath.Join(fragments...)
}

func repositoriesFile() string {
	return filepath.Join(homePath(), repositoriesFilePath)
}
