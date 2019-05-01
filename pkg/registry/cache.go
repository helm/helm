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

package registry // import "helm.sh/helm/pkg/registry"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	orascontent "github.com/deislabs/oras/pkg/content"
	units "github.com/docker/go-units"
	checksum "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"

	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/chart/loader"
	"helm.sh/helm/pkg/chartutil"
)

var (
	tableHeaders = []string{"name", "version", "digest", "size", "created"}
)

type (
	filesystemCache struct {
		out     io.Writer
		rootDir string
		store   *orascontent.Memorystore
	}
)

func (cache *filesystemCache) LayersToChart(layers []ocispec.Descriptor) (*chart.Chart, error) {
	metaLayer, contentLayer, err := extractLayers(layers)
	if err != nil {
		return nil, err
	}

	name, version, err := extractChartNameVersionFromLayer(contentLayer)
	if err != nil {
		return nil, err
	}

	// Obtain raw chart meta content (json)
	_, metaJSONRaw, ok := cache.store.Get(metaLayer)
	if !ok {
		return nil, errors.New("error retrieving meta layer")
	}

	// Construct chart metadata object
	metadata := chart.Metadata{}
	err = json.Unmarshal(metaJSONRaw, &metadata)
	if err != nil {
		return nil, err
	}
	metadata.APIVersion = chart.APIVersionV1
	metadata.Name = name
	metadata.Version = version

	// Obtain raw chart content
	_, contentRaw, ok := cache.store.Get(contentLayer)
	if !ok {
		return nil, errors.New("error retrieving meta layer")
	}

	// Construct chart object and attach metadata
	ch, err := loader.LoadArchive(bytes.NewBuffer(contentRaw))
	if err != nil {
		return nil, err
	}
	ch.Metadata = &metadata

	return ch, nil
}

func (cache *filesystemCache) ChartToLayers(ch *chart.Chart) ([]ocispec.Descriptor, error) {

	// extract/separate the name and version from other metadata
	if err := ch.Validate(); err != nil {
		return nil, err
	}
	name := ch.Metadata.Name
	version := ch.Metadata.Version

	// Create meta layer, clear name and version from Chart.yaml and convert to json
	ch.Metadata.Name = ""
	ch.Metadata.Version = ""
	metaJSONRaw, err := json.Marshal(ch.Metadata)
	if err != nil {
		return nil, err
	}
	metaLayer := cache.store.Add(HelmChartMetaFileName, HelmChartMetaMediaType, metaJSONRaw)

	// Create content layer
	// TODO: something better than this hack. Currently needed for chartutil.Save()
	// If metadata does not contain Name or Version, an error is returned
	// such as "no chart name specified (Chart.yaml)"
	ch.Metadata = &chart.Metadata{
		APIVersion: chart.APIVersionV1,
		Name:       "-",
		Version:    "0.1.0",
	}
	destDir := mkdir(filepath.Join(cache.rootDir, "blobs", ".build"))
	tmpFile, err := chartutil.Save(ch, destDir)
	defer os.Remove(tmpFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to save")
	}
	contentRaw, err := ioutil.ReadFile(tmpFile)
	if err != nil {
		return nil, err
	}
	contentLayer := cache.store.Add(HelmChartContentFileName, HelmChartContentMediaType, contentRaw)

	// Set annotations
	contentLayer.Annotations[HelmChartNameAnnotation] = name
	contentLayer.Annotations[HelmChartVersionAnnotation] = version

	layers := []ocispec.Descriptor{metaLayer, contentLayer}
	return layers, nil
}

func (cache *filesystemCache) LoadReference(ref *Reference) ([]ocispec.Descriptor, error) {
	tagDir := filepath.Join(cache.rootDir, "refs", escape(ref.Repo), "tags", tagOrDefault(ref.Tag))

	// add meta layer
	metaJSONRaw, err := getSymlinkDestContent(filepath.Join(tagDir, "meta"))
	if err != nil {
		return nil, err
	}
	metaLayer := cache.store.Add(HelmChartMetaFileName, HelmChartMetaMediaType, metaJSONRaw)

	// add content layer
	contentRaw, err := getSymlinkDestContent(filepath.Join(tagDir, "content"))
	if err != nil {
		return nil, err
	}
	contentLayer := cache.store.Add(HelmChartContentFileName, HelmChartContentMediaType, contentRaw)

	// set annotations on content layer (chart name and version)
	err = setLayerAnnotationsFromChartLink(contentLayer, filepath.Join(tagDir, "chart"))
	if err != nil {
		return nil, err
	}

	printChartSummary(cache.out, metaLayer, contentLayer)
	layers := []ocispec.Descriptor{metaLayer, contentLayer}
	return layers, nil
}

func (cache *filesystemCache) StoreReference(ref *Reference, layers []ocispec.Descriptor) (bool, error) {
	tag := tagOrDefault(ref.Tag)
	tagDir := mkdir(filepath.Join(cache.rootDir, "refs", escape(ref.Repo), "tags", tag))

	// Retrieve just the meta and content layers
	metaLayer, contentLayer, err := extractLayers(layers)
	if err != nil {
		return false, err
	}

	// Extract chart name and version
	name, version, err := extractChartNameVersionFromLayer(contentLayer)
	if err != nil {
		return false, err
	}

	// Create chart file
	chartPath, err := createChartFile(filepath.Join(cache.rootDir, "charts"), name, version)
	if err != nil {
		return false, err
	}

	// Create chart symlink
	err = createSymlink(chartPath, filepath.Join(tagDir, "chart"))
	if err != nil {
		return false, err
	}

	// Save meta blob
	metaExists, metaPath := digestPath(filepath.Join(cache.rootDir, "blobs"), metaLayer.Digest)
	if !metaExists {
		fmt.Fprintf(cache.out, "%s: Saving meta (%s)\n",
			shortDigest(metaLayer.Digest.Hex()), byteCountBinary(metaLayer.Size))
		_, metaJSONRaw, ok := cache.store.Get(metaLayer)
		if !ok {
			return false, errors.New("error retrieving meta layer")
		}
		err = writeFile(metaPath, metaJSONRaw)
		if err != nil {
			return false, err
		}
	}

	// Create meta symlink
	err = createSymlink(metaPath, filepath.Join(tagDir, "meta"))
	if err != nil {
		return false, err
	}

	// Save content blob
	contentExists, contentPath := digestPath(filepath.Join(cache.rootDir, "blobs"), contentLayer.Digest)
	if !contentExists {
		fmt.Fprintf(cache.out, "%s: Saving content (%s)\n",
			shortDigest(contentLayer.Digest.Hex()), byteCountBinary(contentLayer.Size))
		_, contentRaw, ok := cache.store.Get(contentLayer)
		if !ok {
			return false, errors.New("error retrieving content layer")
		}
		err = writeFile(contentPath, contentRaw)
		if err != nil {
			return false, err
		}
	}

	// Create content symlink
	err = createSymlink(contentPath, filepath.Join(tagDir, "content"))
	if err != nil {
		return false, err
	}

	printChartSummary(cache.out, metaLayer, contentLayer)
	return metaExists && contentExists, nil
}

func (cache *filesystemCache) DeleteReference(ref *Reference) error {
	tagDir := filepath.Join(cache.rootDir, "refs", escape(ref.Repo), "tags", tagOrDefault(ref.Tag))
	if _, err := os.Stat(tagDir); os.IsNotExist(err) {
		return errors.New("ref not found")
	}
	return os.RemoveAll(tagDir)
}

func (cache *filesystemCache) TableRows() ([][]interface{}, error) {
	return getRefsSorted(filepath.Join(cache.rootDir, "refs"))
}

// escape sanitizes a registry URL to remove characters such as ":"
// which are illegal on windows
func escape(s string) string {
	return strings.ReplaceAll(s, ":", "_")
}

// escape reverses escape
func unescape(s string) string {
	return strings.ReplaceAll(s, "_", ":")
}

// printChartSummary prints details about a chart layers
func printChartSummary(out io.Writer, metaLayer ocispec.Descriptor, contentLayer ocispec.Descriptor) {
	fmt.Fprintf(out, "Name: %s\n", contentLayer.Annotations[HelmChartNameAnnotation])
	fmt.Fprintf(out, "Version: %s\n", contentLayer.Annotations[HelmChartVersionAnnotation])
	fmt.Fprintf(out, "Meta: %s\n", metaLayer.Digest)
	fmt.Fprintf(out, "Content: %s\n", contentLayer.Digest)
}

// fileExists determines if a file exists
func fileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// mkdir will create a directory (no error check) and return the path
func mkdir(dir string) string {
	os.MkdirAll(dir, 0755)
	return dir
}

// createSymlink creates a symbolic link, deleting existing one if exists
func createSymlink(src string, dest string) error {
	os.Remove(dest)
	err := os.Symlink(src, dest)
	return err
}

// getSymlinkDestContent returns the file contents of a symlink's destination
func getSymlinkDestContent(linkPath string) ([]byte, error) {
	src, err := os.Readlink(linkPath)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadFile(src)
}

// setLayerAnnotationsFromChartLink will set chart name/version annotations on a layer
// based on the path of the chart link destination
func setLayerAnnotationsFromChartLink(layer ocispec.Descriptor, chartLinkPath string) error {
	src, err := os.Readlink(chartLinkPath)
	if err != nil {
		return err
	}
	// example path: /some/path/charts/mychart/versions/1.2.0
	chartName := filepath.Base(filepath.Dir(filepath.Dir(src)))
	chartVersion := filepath.Base(src)
	layer.Annotations[HelmChartNameAnnotation] = chartName
	layer.Annotations[HelmChartVersionAnnotation] = chartVersion
	return nil
}

// extractLayers obtains the meta and content layers from a list of layers
func extractLayers(layers []ocispec.Descriptor) (ocispec.Descriptor, ocispec.Descriptor, error) {
	var metaLayer, contentLayer ocispec.Descriptor

	if len(layers) != 2 {
		return metaLayer, contentLayer, errors.New("manifest does not contain exactly 2 layers")
	}

	for _, layer := range layers {
		switch layer.MediaType {
		case HelmChartMetaMediaType:
			metaLayer = layer
		case HelmChartContentMediaType:
			contentLayer = layer
		}
	}

	if metaLayer.Size == 0 {
		return metaLayer, contentLayer, errors.New("manifest does not contain a Helm chart meta layer")
	}

	if contentLayer.Size == 0 {
		return metaLayer, contentLayer, errors.New("manifest does not contain a Helm chart content layer")
	}

	return metaLayer, contentLayer, nil
}

// extractChartNameVersionFromLayer retrieves the chart name and version from layer annotations
func extractChartNameVersionFromLayer(layer ocispec.Descriptor) (string, string, error) {
	name, ok := layer.Annotations[HelmChartNameAnnotation]
	if !ok {
		return "", "", errors.New("could not find chart name in annotations")
	}
	version, ok := layer.Annotations[HelmChartVersionAnnotation]
	if !ok {
		return "", "", errors.New("could not find chart version in annotations")
	}
	return name, version, nil
}

// createChartFile creates a file under "<chartsdir>" dir which is linked to by ref
func createChartFile(chartsRootDir string, name string, version string) (string, error) {
	chartPathDir := filepath.Join(chartsRootDir, name, "versions")
	chartPath := filepath.Join(chartPathDir, version)
	if _, err := os.Stat(chartPath); err != nil && os.IsNotExist(err) {
		os.MkdirAll(chartPathDir, 0755)
		err := ioutil.WriteFile(chartPath, []byte("-"), 0644)
		if err != nil {
			return "", err
		}
	}
	return chartPath, nil
}

// digestPath returns the path to addressable content, and whether the file exists
func digestPath(rootDir string, digest checksum.Digest) (bool, string) {
	path := filepath.Join(rootDir, "sha256", digest.Hex())
	exists := fileExists(path)
	return exists, path
}

// writeFile creates a path, ensuring parent directory
func writeFile(path string, c []byte) error {
	os.MkdirAll(filepath.Dir(path), 0755)
	return ioutil.WriteFile(path, c, 0644)
}

// byteCountBinary produces a human-readable file size
func byteCountBinary(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// tagOrDefault returns the tag if present, if not the default tag
func tagOrDefault(tag string) string {
	if tag != "" {
		return tag
	}
	return HelmChartDefaultTag
}

// shortDigest returns first 7 characters of a sha256 digest
func shortDigest(digest string) string {
	if len(digest) == 64 {
		return digest[:7]
	}
	return digest
}

// getRefsSorted returns a map of all refs stored in a refsRootDir
func getRefsSorted(refsRootDir string) ([][]interface{}, error) {
	refsMap := map[string]map[string]string{}

	// Walk the storage dir, check for symlinks under "refs" dir pointing to valid files in "blobs/" and "charts/"
	err := filepath.Walk(refsRootDir, func(path string, fileInfo os.FileInfo, fileError error) error {

		// Check if this file is a symlink
		linkPath, err := os.Readlink(path)
		if err == nil {
			destFileInfo, err := os.Stat(linkPath)
			if err == nil {
				tagDir := filepath.Dir(path)

				// Determine the ref
				repo := unescape(strings.TrimLeft(
					strings.TrimPrefix(filepath.Dir(filepath.Dir(tagDir)), refsRootDir), "/\\"))
				tag := filepath.Base(tagDir)
				ref := fmt.Sprintf("%s:%s", repo, tag)

				// Init hashmap entry if does not exist
				if _, ok := refsMap[ref]; !ok {
					refsMap[ref] = map[string]string{}
				}

				// Add data to entry based on file name (symlink name)
				base := filepath.Base(path)
				switch base {
				case "chart":
					refsMap[ref]["name"] = filepath.Base(filepath.Dir(filepath.Dir(linkPath)))
					refsMap[ref]["version"] = destFileInfo.Name()
				case "content":

					// Make sure the filename looks like a sha256 digest (64 chars)
					digest := destFileInfo.Name()
					if len(digest) == 64 {
						refsMap[ref]["digest"] = shortDigest(digest)
						refsMap[ref]["size"] = byteCountBinary(destFileInfo.Size())
						refsMap[ref]["created"] = units.HumanDuration(time.Now().UTC().Sub(destFileInfo.ModTime()))
					}
				}
			}
		}

		return nil
	})

	// Filter out any refs that are incomplete (do not have all required fields)
	for k, ref := range refsMap {
		allKeysFound := true
		for _, v := range tableHeaders {
			if _, ok := ref[v]; !ok {
				allKeysFound = false
				break
			}
		}
		if !allKeysFound {
			delete(refsMap, k)
		}
	}

	// Sort and convert to format expected by uitable
	refs := make([][]interface{}, len(refsMap))
	keys := make([]string, 0, len(refsMap))
	for key := range refsMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		refs[i] = make([]interface{}, len(tableHeaders)+1)
		refs[i][0] = key
		ref := refsMap[key]
		for j, k := range tableHeaders {
			refs[i][j+1] = ref[k]
		}
	}

	return refs, err
}
