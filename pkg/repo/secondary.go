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

package repo

import (
	"io/ioutil"
	"os"
	"time"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/internal/urlutil"
)

type ChartVerEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type SecondaryIndexes struct {
	ByURL map[string]ChartVerEntry `json:"byURL,omitempty"`
}

func NewSecondaryIndexes() *SecondaryIndexes {
	return &SecondaryIndexes{}
}

type SecondaryIndexFile struct {
	APIVersion string            `json:"apiVersion"`
	Generated  time.Time         `json:"generated"`
	Digest     string            `json:"digest"`
	Indexes    *SecondaryIndexes `json:"indexes"`
}

func NewSecondaryIndexFile() *SecondaryIndexFile {
	return &SecondaryIndexFile{
		APIVersion: APIVersionV1,
		Generated:  time.Now(),
		Indexes:    NewSecondaryIndexes(),
	}
}

//TODO: this function should ensure the loaded secondary is consistent with the
//existing primary index by comparing digests.
func LoadSecondaryIndexFile(path string) (*SecondaryIndexFile, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := &SecondaryIndexFile{}
	if err := yaml.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SecondaryIndexFile) WriteFile(dest string, mode os.FileMode) error {
	b, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dest, b, mode)
}

func (s *SecondaryIndexFile) IsComputedFrom(index *IndexFile) bool {
	digest, err := index.digest()
	if err != nil {
		return false
	}
	return s.Digest == digest
}

func (s *SecondaryIndexFile) buildURLIndex(index *IndexFile) error {
	urlIx := make(map[string]ChartVerEntry)
	for _, entry := range index.Entries {
		for _, ver := range entry {
			for _, dl := range ver.URLs {
				u := urlutil.Canonical(dl)
				urlIx[u] = ChartVerEntry{
					Name:    ver.Name,
					Version: ver.Version,
				}
			}
		}
	}
	s.Indexes.ByURL = urlIx
	return nil
}
