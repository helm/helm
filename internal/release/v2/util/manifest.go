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

package util // import "helm.sh/helm/v4/internal/release/v2/util"

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
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata,omitempty"`
}

// sep matches YAML document separators. A separator is `---` at the start of
// a line (or start of the stream), optionally followed by trailing horizontal
// whitespace, and then a newline or end of stream.
//
// This is intentionally stricter than the Chart v1/v2 regex in
// pkg/release/v1/util/manifest.go. The v1/v2 version tolerates
// `---<non-whitespace>` (e.g. `---apiVersion: v1`) by treating it as a
// separator glued to content — a silent correction for Go template whitespace
// trimming (`{{-`) eating the newline after `---`. Chart v3 does not carry
// that workaround forward; see the function comment below.
var sep = regexp.MustCompile("(?:^|\\s*\n)---[ \\t]*(?:\\r?\\n|$)")

// SplitManifests takes a manifest string and returns a map containing individual manifests.
//
// Chart API v3 note: unlike Chart v1/v2, this implementation does NOT silently
// repair YAML document separators glued to content by Go template whitespace
// trimming. A template such as
//
//	---
//	{{- include "mychart.service" . }}
//
// renders `---apiVersion: v1\n...` because `{{-` strips the newline after
// `---`. In Chart v1/v2, SplitManifests detects this and splits the input as
// if the newline were still there; in Chart v3, the glued `---` is left as
// part of the document body and downstream YAML parsing will surface the
// problem. Chart authors should drop the dash (`{{ include ... }}`) or omit
// the explicit `---` separator — Helm inserts one between templates on its
// own. See helm/helm#32036 and the v2 → v3 migration guide.
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
