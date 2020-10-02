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

// ExpandLocalPath expands a local file, dir or glob path to a list of files
func ExpandLocalPath(name string, path string) (map[string]string, error) {
	if strings.Contains(path, "*") {
		// if this is a glob, we expand it and return a list of files
		return expandGlob(name, path)
	}

	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		// if this is a valid dir, we return all files within
		return expandDir(name, path)
	}

	// finally, this is a file, so we return it
	return map[string]string{name: path}, nil
}

func expandGlob(name string, path string) (map[string]string, error) {
	fmap := make(map[string]string)
	paths, err := filepath.Glob(path)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errors.New("empty glob")
	}

	namePrefix := strings.TrimRight(name, "/") + "/"
	for _, p := range paths {
		key := namePrefix + filepath.Base(p)
		fmap[key] = p
	}

	return fmap, nil
}

func expandDir(name string, path string) (map[string]string, error) {
	fmap := make(map[string]string)

	f, err := os.Open(path)

	if err != nil {
		return nil, err
	}
	defer f.Close()

	files, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}

	localDirName := strings.TrimRight(path, "/") + "/"
	namePrefix := strings.TrimRight(name, "/") + "/"
	for _, file := range files {
		key := namePrefix + file.Name()
		fmap[key] = localDirName + file.Name()
	}
	return fmap, nil
}
