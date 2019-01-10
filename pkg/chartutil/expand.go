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

package chartutil

import (
	"io"
	"os"
)

// Expand uncompresses and extracts a chart into the specified directory.
func Expand(dir string, r io.Reader) error {
	ch, err := LoadArchive(r)
	if err != nil {
		return err
	}
	return SaveDir(ch, dir)
}

// ExpandFile expands the src file into the dest directory.
func ExpandFile(dest, src string) error {
	h, err := os.Open(src)
	if err != nil {
		return err
	}
	defer h.Close()
	return Expand(dest, h)
}
