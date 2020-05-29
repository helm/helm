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
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
)

// LoadLocalFile loads a file from the local filesystem.
func LoadLocalFile(path string) ([]byte, error) {
	if fi, err := os.Stat(path); err != nil {
		return nil, err
	} else if fi.IsDir() {
		return nil, errors.New("cannot load a directory")
	}

	return ioutil.ReadFile(path)
}
