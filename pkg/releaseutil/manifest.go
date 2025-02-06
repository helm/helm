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
	"strconv"
	"strings"
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

// SplitManifests takes a string of manifest and returns a map contains individual manifests
func SplitManifests(bigFile string) map[string]string {
	// Basically, we're quickly splitting a stream of YAML documents into an
	// array of YAML docs. The file name is just a place holder, but should be
	// integer-sortable so that manifests get output in the same order as the
	// input (see `BySplitManifestsOrder`).
	// Making sure that any extra whitespace in YAML stream doesn't interfere in splitting documents correctly.
	bigFileTmp := strings.TrimSpace(bigFile)
	docs := splitDocs(bigFileTmp)
	res := make(map[string]string, len(docs))
	for count, _ := range docs {
		res["manifest-"+strconv.Itoa(count)] = docs[count]
	}
	return res
}

const yamlDocumentTermination = "\n---"

func splitDocs(bigFile string) []string {
	docs := make([]string, 0)
	docStartIdx := 0

	// strip off a leading --- avoiding a special start case
	bigFile = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(bigFile), "---"))

	// using our own index so we can manually skip forward
	i := 0
	for i < len(bigFile) {
		// see if we find a document termination sequence, i.e. "\n---"
		if !strings.HasPrefix(bigFile[i:], yamlDocumentTermination) {
			i++
			continue
		}

		// this is the end of the document, slicing the bytes array
		doc := strings.TrimSpace(bigFile[docStartIdx:i])

		// ignore empty docs
		if doc != "" {
			docs = append(docs, doc)
		}

		// skip the document termination characters
		docStartIdx = i + len(yamlDocumentTermination)
		i = docStartIdx
	}

	// append the 'rest' of the document as the last document
	doc := strings.TrimSpace(bigFile[docStartIdx:])
	if doc != "" {
		docs = append(docs, doc)
	}

	return docs
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
