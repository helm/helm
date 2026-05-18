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

package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// SimpleHead defines what the structure of the head of a manifest file
type SimpleHead struct {
	Version  string `json:"apiVersion"`
	Kind     string `json:"kind,omitempty"`
	Metadata *struct {
		Name        string            `json:"name"`
		Namespace   string            `json:"namespace,omitempty"`
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata,omitempty"`
}

var sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")

// SplitManifests takes a manifest string and returns a map containing individual manifests.
//
// **Note for Chart API v3**: This function (due to the regex above) has allowed _WRONG_
// Go templates to be defined inside charts across the years. The generated text from Go
// templates may contain `---apiVersion: v1`, and this function magically splits this back
// to `---\napiVersion: v1`. This has caused issues recently after Helm 4 introduced
// kio.ParseAll to inject annotations when post-renderers are used. In Chart API v3,
// we should kill this regex with fire (or change it) and expose charts doing the wrong
// thing Go template-wise. Helm should say a big _NO_ to charts doing the wrong thing,
// with or without post-renderers.
func SplitManifests(bigFile string) map[string]string {
	// Basically, we're quickly splitting a stream of YAML documents into an
	// array of YAML docs. The file name is just a place holder, but should be
	// integer-sortable so that manifests get output in the same order as the
	// input (see `BySplitManifestsOrder`).
	tpl := "manifest-%d"
	res := map[string]string{}
	// Making sure that any extra whitespace in YAML stream doesn't interfere in splitting documents correctly.
	bigFileTmp := strings.TrimLeftFunc(bigFile, unicode.IsSpace)
	docs := sep.Split(bigFileTmp, -1)
	var count int
	for _, d := range docs {
		if strings.TrimSpace(d) == "" {
			continue
		}

		d = strings.TrimLeftFunc(d, unicode.IsSpace)
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

// helmInternalAnnotationLineRE matches a single YAML line whose key is one of
// HelmInternalSequencingAnnotations. The pattern is intentionally line-based:
// Helm always emits these annotations as single-line JSON-encoded values, so a
// surgical line strip preserves the surrounding manifest byte-for-byte and
// keeps `helm template | diff` workflows stable. The regex is compiled lazily
// from HelmInternalSequencingAnnotations so that adding a new helm-internal
// key only requires updating the slice.
var helmInternalAnnotationLineRE = func() *regexp.Regexp {
	keys := make([]string, len(HelmInternalSequencingAnnotations))
	for i, k := range HelmInternalSequencingAnnotations {
		keys[i] = regexp.QuoteMeta(k)
	}
	return regexp.MustCompile(`(?m)^[ \t]+(?:` + strings.Join(keys, "|") + `):[^\n]*\r?\n?`)
}()

// StripHelmInternalAnnotations returns the manifest content with any
// HelmInternalSequencingAnnotations removed. The strip is line-based and
// preserves the surrounding byte order of the input document. Empty content
// passes through unchanged.
//
// This exists so that `helm template` output remains directly apply-able via
// `kubectl apply -f -`, even when charts use HIP-0025 sequencing annotations
// whose keys contain multiple `/` separators (which fail Kubernetes
// annotation-key validation).
func StripHelmInternalAnnotations(content string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}
	return helmInternalAnnotationLineRE.ReplaceAllString(content, "")
}
