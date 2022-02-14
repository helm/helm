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
	"io/fs"
	"os"
	"path/filepath"
)

// ExpandFilePath expands a local file, dir or glob path to a list of files
func ExpandFilePath(path string) ([]string, error) {
	_, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var files []string
	filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			files = append(files, filepath.ToSlash(path))
		}
		return nil
	})

	return files, nil
}
