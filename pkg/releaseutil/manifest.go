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
	"regexp"
	"strconv"
	"strings"

	"k8s.io/helm/pkg/manifest"
)

var (
	sep  = regexp.MustCompile("(?:^|\\s*\n)---\\s*")
	wei  = regexp.MustCompile("\\#\\s*Weight\\:\\s*(\\d+),(\\d+)\\s*")
	path = regexp.MustCompile("\\#\\s*Source\\:\\s*(.*)\\s*")
)

// SplitManifests takes a string of manifest and returns a map contains individual manifests
func SplitManifests(bigFile string) map[string]string {
	// Basically, we're quickly splitting a stream of YAML documents into an
	// array of YAML docs. In the current implementation, the file name is just
	// a place holder, and doesn't have any further meaning.
	tpl := "manifest-%d"
	res := map[string]string{}
	// Making sure that any extra whitespace in YAML stream doesn't interfere in splitting documents correctly.
	bigFileTmp := strings.TrimSpace(bigFile)
	docs := sep.Split(bigFileTmp, -1)
	var count int
	var name string
	for _, d := range docs {

		if d == "" {
			continue
		}

		d = strings.TrimSpace(d)
		match := path.FindStringSubmatch(d)
		if match != nil {
			name = match[1]
		} else {
			name = fmt.Sprintf(tpl, count)
		}
		res[name] = d
		count = count + 1
	}
	return res
}

// SplitManifestContent takes a string of manifest and returns a slice containing individual manifests
func SplitManifestContent(bigFile string) []string {
	res := []string{}
	bigFileTmp := strings.TrimSpace(bigFile)
	docs := sep.Split(bigFileTmp, -1)
	for _, d := range docs {
		if d == "" {
			continue
		}
		res = append(res, d)
	}
	return res
}

// GroupManifestsByWeight takes a map of manifests and returns an ordered grouping of them
func GroupManifestsByWeight(manifests []string) []string {
	groups := []string{}
	var groupWeight *manifest.Weight
	var weight *manifest.Weight

	for _, m := range manifests {
		weight = getManifestWeight(m)
		tmp := fmt.Sprintf("---\n%s\n", m)
		if weight.Equalto(groupWeight) {
			groups[len(groups)-1] += tmp
		} else {
			groups = append(groups, tmp)
			groupWeight = weight
		}
	}
	return groups
}

func getManifestWeight(file string) (weight *manifest.Weight) {
	weight = new(manifest.Weight)
	match := wei.FindStringSubmatch(file)
	if match == nil {
		return
	}
	c, _ := strconv.ParseUint(match[1], 10, 32)
	m, _ := strconv.ParseUint(match[2], 10, 32)
	weight.Chart = uint32(c)
	weight.Manifest = uint32(m)
	return
}
