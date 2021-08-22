/*
Copyright The Helm Authors.
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

package loader

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ExpandFilePath expands a local file, dir or glob path to a list of files
func ExpandFilePath(path string) ([]string, error) {
	if strings.Contains(path, "*") {
		// if this is a glob, we expand it and return a list of files
		return expandGlob(path)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fileInfo.IsDir() {
		// if this is a valid dir, we return all files within
		return expandDir(path)
	}

	// finally, this is a file, so we return it
	return []string{path}, nil
}

func expandGlob(path string) ([]string, error) {
	paths, err := filepath.Glob(path)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errors.New("empty glob")
	}

	return paths, err
}

func expandDir(path string) ([]string, error) {
	f, err := os.Open(path)

	if err != nil {
		return nil, err
	}
	defer f.Close()

	filesInfos, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}

	var filesPaths []string
	localDirName := strings.TrimRight(path, "/") + "/"
	for _, file := range filesInfos {
		filesPaths = append(filesPaths, localDirName+file.Name())
	}
	return filesPaths, nil
}
