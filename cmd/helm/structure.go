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

func cacheIndexFile(repoName string) string {
	return cacheDirectory(repoName + "-index.yaml")
}

func localRepoDirectory(paths ...string) string {
	fragments := append([]string{repositoryDirectory(), localRepoPath}, paths...)
	return filepath.Join(fragments...)
}

func repositoriesFile() string {
	return filepath.Join(repositoryDirectory(), repositoriesFilePath)
}
