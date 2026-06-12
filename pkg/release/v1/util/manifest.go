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
	"slices"
	"strconv"
	"strings"
	"unicode"

	"sigs.k8s.io/yaml"
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

// StripHelmInternalAnnotations returns manifest content with Helm-internal
// sequencing annotations removed from top-level object metadata. This exists
// so that `helm template | kubectl apply -f -` remains valid even when charts
// use HIP-0025 sequencing annotation keys with multiple `/` separators, which
// fail Kubernetes annotation-key validation.
//
// The strip is block-scoped and conservative. Each YAML document first takes a
// byte-identity fast path unless the parsed SimpleHead confirms that a
// Helm-internal sequencing key is present in metadata.annotations. Confirmed
// documents are scanned as text, and the function only deletes whole lines
// inside the confirmed top-level metadata.annotations block, including
// continuation lines for stripped values. On any structural doubt, it returns
// the original document verbatim. The failure mode is "annotation retained",
// never "manifest corrupted".
//
// Stored release manifests keep these annotations because they are the
// uninstall/rollback plan input recovered by sequence.ParseStoredManifests.
// `helm get manifest` prints the stored record verbatim by design. Only output
// surfaces such as `helm template` and apply-time paths strip them. This aligns
// with the object-domain visitor stripSequencingAnnotations in pkg/action,
// which also strips only top-level object metadata. Annotations nested in pod
// templates or List items are not functional for sequencing and are no longer
// masked by this text strip.
func StripHelmInternalAnnotations(content string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}

	var out strings.Builder
	var doc strings.Builder
	for _, line := range strings.SplitAfter(content, "\n") {
		if isYAMLDocumentSeparatorLine(line) {
			out.WriteString(stripHelmInternalAnnotationsFromDoc(doc.String()))
			doc.Reset()
			out.WriteString(line)
			continue
		}
		doc.WriteString(line)
	}
	out.WriteString(stripHelmInternalAnnotationsFromDoc(doc.String()))
	return out.String()
}

func stripHelmInternalAnnotationsFromDoc(doc string) string {
	if strings.TrimSpace(doc) == "" {
		return doc
	}

	var head SimpleHead
	if err := yaml.Unmarshal([]byte(doc), &head); err != nil {
		return doc
	}
	if head.Metadata == nil || !hasHelmInternalSequencingAnnotation(head.Metadata.Annotations) {
		return doc
	}

	lines := strings.SplitAfter(doc, "\n")
	metadataLine := -1
	for i, line := range lines {
		if isExactYAMLKeyLine(line, 0, "metadata") {
			metadataLine = i
			break
		}
	}
	if metadataLine == -1 {
		return doc
	}

	metadataStart := metadataLine + 1
	metadataEnd := len(lines)
	for i := metadataStart; i < len(lines); i++ {
		if isBlankYAMLLine(lines[i]) {
			continue
		}
		if yamlLineIndent(lines[i]) == 0 {
			metadataEnd = i
			break
		}
	}

	metadataChildIndent, ok := minimumYAMLIndent(lines[metadataStart:metadataEnd])
	if !ok {
		return doc
	}

	annotationsLine := -1
	for i := metadataStart; i < metadataEnd; i++ {
		if isExactYAMLKeyLine(lines[i], metadataChildIndent, "annotations") {
			annotationsLine = i
			break
		}
	}
	if annotationsLine == -1 {
		return doc
	}

	annotationsStart := annotationsLine + 1
	annotationsEnd := metadataEnd
	for i := annotationsStart; i < metadataEnd; i++ {
		if isBlankYAMLLine(lines[i]) {
			continue
		}
		if yamlLineIndent(lines[i]) <= metadataChildIndent {
			annotationsEnd = i
			break
		}
	}

	annotationEntryIndent, ok := minimumYAMLIndent(lines[annotationsStart:annotationsEnd])
	if !ok {
		return doc
	}

	var out strings.Builder
	for i := 0; i < len(lines); {
		if i >= annotationsStart &&
			i < annotationsEnd &&
			yamlLineIndent(lines[i]) == annotationEntryIndent &&
			isHelmInternalAnnotationEntryLine(lines[i], annotationEntryIndent) {
			i++
			for i < annotationsEnd {
				if isBlankYAMLLine(lines[i]) {
					nextNonBlank := i + 1
					for nextNonBlank < annotationsEnd && isBlankYAMLLine(lines[nextNonBlank]) {
						nextNonBlank++
					}
					if nextNonBlank < annotationsEnd && yamlLineIndent(lines[nextNonBlank]) > annotationEntryIndent {
						i = nextNonBlank
						continue
					}
					break
				}
				if yamlLineIndent(lines[i]) <= annotationEntryIndent {
					break
				}
				i++
			}
			continue
		}
		out.WriteString(lines[i])
		i++
	}
	return out.String()
}

func hasHelmInternalSequencingAnnotation(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	for _, key := range helmInternalSequencingAnnotations {
		if _, ok := annotations[key]; ok {
			return true
		}
	}
	return false
}

func isYAMLDocumentSeparatorLine(line string) bool {
	body := strings.TrimSuffix(line, "\n")
	return strings.TrimRight(body, " \t\r") == "---"
}

func lineBody(line string) string {
	body := strings.TrimSuffix(line, "\n")
	return strings.TrimSuffix(body, "\r")
}

func isBlankYAMLLine(line string) bool {
	return strings.TrimSpace(lineBody(line)) == ""
}

func yamlLineIndent(line string) int {
	body := lineBody(line)
	for i, r := range body {
		if r != ' ' && r != '\t' {
			return i
		}
	}
	return len(body)
}

func isExactYAMLKeyLine(line string, indent int, key string) bool {
	body := lineBody(line)
	if yamlLineIndent(line) != indent {
		return false
	}
	return strings.TrimRight(body[indent:], " \t") == key+":"
}

func minimumYAMLIndent(lines []string) (int, bool) {
	minIndent := 0
	found := false
	for _, line := range lines {
		if isBlankYAMLLine(line) {
			continue
		}
		indent := yamlLineIndent(line)
		if !found || indent < minIndent {
			minIndent = indent
			found = true
		}
	}
	return minIndent, found
}

func isHelmInternalAnnotationEntryLine(line string, indent int) bool {
	key, ok := yamlEntryKey(line, indent)
	if !ok {
		return false
	}
	return slices.Contains(helmInternalSequencingAnnotations, key)
}

func yamlEntryKey(line string, indent int) (string, bool) {
	body := lineBody(line)
	if yamlLineIndent(line) != indent {
		return "", false
	}
	entry := body[indent:]
	if entry == "" {
		return "", false
	}
	switch entry[0] {
	case '\'', '"':
		quote := entry[0]
		end := strings.IndexByte(entry[1:], quote)
		if end == -1 {
			return "", false
		}
		key := entry[1 : end+1]
		rest := entry[end+2:]
		if !strings.HasPrefix(rest, ":") {
			return "", false
		}
		return key, true
	default:
		for _, key := range helmInternalSequencingAnnotations {
			if strings.HasPrefix(entry, key+":") {
				return key, true
			}
		}
		return "", false
	}
}
