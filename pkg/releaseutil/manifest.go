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

package releaseutil

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"helm.sh/helm/v3/pkg/release"
)

// SimpleHead defines what the structure of the head of a manifest file
type SimpleHead struct {
	Version  string `json:"apiVersion"`
	Kind     string `json:"kind,omitempty"`
	Metadata *struct {
		Name        string            `json:"name"`
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata,omitempty"`
}

var sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")

// SplitManifests takes a string of manifest and returns a map contains individual manifests
func SplitManifests(bigFile string) map[string]string {
	// Basically, we're quickly splitting a stream of YAML documents into an
	// array of YAML docs. The file name is just a place holder, but should be
	// integer-sortable so that manifests get output in the same order as the
	// input (see `BySplitManifestsOrder`).
	tpl := "manifest-%d"
	res := map[string]string{}
	// Making sure that any extra whitespace in YAML stream doesn't interfere in splitting documents correctly.
	bigFileTmp := strings.TrimSpace(bigFile)
	docs := sep.Split(bigFileTmp, -1)
	var count int
	for _, d := range docs {
		if d == "" {
			continue
		}

		d = strings.TrimSpace(d)
		res[fmt.Sprintf(tpl, count)] = d
		count = count + 1
	}
	return res
}

// BySplitManifestsOrder sorts by in-file manifest order, as provided in function `SplitManifests`
type BySplitManifestsOrder []string

func (a BySplitManifestsOrder) Len() int { return len(a) }
func (a BySplitManifestsOrder) Less(i, j int) bool {
	// Split `manifest-%d`
	anum, _ := strconv.ParseInt(a[i][len("manifest-"):], 10, 0)
	bnum, _ := strconv.ParseInt(a[j][len("manifest-"):], 10, 0)
	return anum < bnum
}
func (a BySplitManifestsOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// FilterManifestsAndHooks takes a map of manifests and a map of *hooks and returns only those which match
// the fileFilter map
func FilterManifestsAndHooks(manifests []Manifest, hooks []*release.Hook, fileFilter []string) ([]Manifest, []*release.Hook, error) {
	if len(fileFilter) == 0 {
		return manifests, hooks, nil
	}

	// Ignore everything until the first slash
	// This will look like: dir/templates/template.yaml
	// And the regex will return templates/template.yaml
	pathRegex := regexp.MustCompile("[^/]+/(.+)")

	filteredManifests := make([]Manifest, 0)
	filteredHooks := make([]*release.Hook, 0)
	var missing bool
	for _, fileName := range fileFilter {
		missing = true
		// Use linux-style filepath separators to unify user's input path
		fileName = filepath.ToSlash(fileName)
		for _, manifest := range manifests {
			if !IsPathMatch(fileName, manifest.Name, pathRegex) {
				continue
			}
			missing = false
			filteredManifests = append(filteredManifests, manifest)
		}
		// If the path was found in the manifest, we do not have to search for it in the hooks
		if !missing {
			continue
		}
		for _, hook := range hooks {
			if !IsPathMatch(fileName, hook.Path, pathRegex) {
				continue
			}
			missing = false
			filteredHooks = append(filteredHooks, hook)
		}
		if missing {
			return nil, nil, fmt.Errorf("Could not find template %s in chart", fileName)
		}
	}

	return filteredManifests, filteredHooks, nil
}

func IsPathMatch(fileName string, path string, pathRegex *regexp.Regexp) bool {
	submatch := pathRegex.FindStringSubmatch(path)
	if len(submatch) == 0 {
		return false
	}

	submatchPath := submatch[1]
	// hook.Path is rendered using linux-style filepath separators on Windows as
	// well as macOS/linux.
	pathSplit := strings.Split(submatchPath, "/")
	// hook.Path is connected using linux-style filepath separators on Windows as
	// well as macOS/linux
	joinedPath := strings.Join(pathSplit, "/")
	// if the filepath provided matches a manifest path in the
	// chart, render that manifest
	matched, _ := filepath.Match(fileName, joinedPath)
	return matched
}
