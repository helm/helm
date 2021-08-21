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
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/internal/fileutil"
	"helm.sh/helm/v3/internal/urlutil"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/provenance"
)

var indexPath = "index.yaml"

// APIVersionV1 is the v1 API version for index and repository files.
const APIVersionV1 = "v1"

var (
	// ErrNoAPIVersion indicates that an API version was not specified.
	ErrNoAPIVersion = errors.New("no API version specified")
	// ErrNoChartVersion indicates that a chart with the given version is not found.
	ErrNoChartVersion = errors.New("no chart version found")
	// ErrNoChartName indicates that a chart with the given name is not found.
	ErrNoChartName = errors.New("no chart name found")
	// ErrEmptyIndexYaml indicates that the content of index.yaml is empty.
	ErrEmptyIndexYaml = errors.New("empty index.yaml file")
)

// ChartVersions is a list of versioned chart references.
// Implements a sorter on Version.
type ChartVersions []*ChartVersion

// Len returns the length.
func (c ChartVersions) Len() int { return len(c) }

// Swap swaps the position of two items in the versions slice.
func (c ChartVersions) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

// Less returns true if the version of entry a is less than the version of entry b.
func (c ChartVersions) Less(a, b int) bool {
	// Failed parse pushes to the back.
	i, err := semver.NewVersion(c[a].Version)
	if err != nil {
		return true
	}
	j, err := semver.NewVersion(c[b].Version)
	if err != nil {
		return false
	}
	return i.LessThan(j)
}

// IndexFile represents the index file in a chart repository
type IndexFile struct {
	// This is used ONLY for validation against chartmuseum's index files and is discarded after validation.
	ServerInfo map[string]interface{}   `json:"serverInfo,omitempty"`
	APIVersion string                   `json:"apiVersion"`
	Generated  time.Time                `json:"generated"`
	Entries    map[string]ChartVersions `json:"entries"`
	PublicKeys []string                 `json:"publicKeys,omitempty"`

	// Annotations are additional mappings uninterpreted by Helm. They are made available for
	// other applications to add information to the index file.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// NewIndexFile initializes an index.
func NewIndexFile() *IndexFile {
	return &IndexFile{
		APIVersion: APIVersionV1,
		Generated:  time.Now(),
		Entries:    map[string]ChartVersions{},
		PublicKeys: []string{},
	}
}

// LoadIndexFile takes a file at the given path and returns an IndexFile object
func LoadIndexFile(path string) (*IndexFile, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	i, err := loadIndex(b, path)
	if err != nil {
		return nil, errors.Wrapf(err, "error loading %s", path)
	}
	return i, nil
}

// MustAdd adds a file to the index
// This can leave the index in an unsorted state
func (i IndexFile) MustAdd(md *chart.Metadata, filename, baseURL, digest string) error {
	if md.APIVersion == "" {
		md.APIVersion = chart.APIVersionV1
	}
	if err := md.Validate(); err != nil {
		return errors.Wrapf(err, "validate failed for %s", filename)
	}

	u := filename
	if baseURL != "" {
		_, file := filepath.Split(filename)
		var err error
		u, err = urlutil.URLJoin(baseURL, file)
		if err != nil {
			u = path.Join(baseURL, file)
		}
	}
	cr := &ChartVersion{
		URLs:     []string{u},
		Metadata: md,
		Digest:   digest,
		Created:  time.Now(),
	}
	ee := i.Entries[md.Name]
	i.Entries[md.Name] = append(ee, cr)
	return nil
}

// Add adds a file to the index and logs an error.
//
// Deprecated: Use index.MustAdd instead.
func (i IndexFile) Add(md *chart.Metadata, filename, baseURL, digest string) {
	if err := i.MustAdd(md, filename, baseURL, digest); err != nil {
		log.Printf("skipping loading invalid entry for chart %q %q from %s: %s", md.Name, md.Version, filename, err)
	}
}

// Has returns true if the index has an entry for a chart with the given name and exact version.
func (i IndexFile) Has(name, version string) bool {
	_, err := i.Get(name, version)
	return err == nil
}

// SortEntries sorts the entries by version in descending order.
//
// In canonical form, the individual version records should be sorted so that
// the most recent release for every version is in the 0th slot in the
// Entries.ChartVersions array. That way, tooling can predict the newest
// version without needing to parse SemVers.
func (i IndexFile) SortEntries() {
	for _, versions := range i.Entries {
		sort.Sort(sort.Reverse(versions))
	}
}

// Get returns the ChartVersion for the given name.
//
// If version is empty, this will return the chart with the latest stable version,
// prerelease versions will be skipped.
func (i IndexFile) Get(name, version string) (*ChartVersion, error) {
	vs, ok := i.Entries[name]
	if !ok {
		return nil, ErrNoChartName
	}
	if len(vs) == 0 {
		return nil, ErrNoChartVersion
	}

	var constraint *semver.Constraints
	if version == "" {
		constraint, _ = semver.NewConstraint("*")
	} else {
		var err error
		constraint, err = semver.NewConstraint(version)
		if err != nil {
			return nil, err
		}
	}

	// when customer input exact version, check whether have exact match one first
	if len(version) != 0 {
		for _, ver := range vs {
			if version == ver.Version {
				return ver, nil
			}
		}
	}

	for _, ver := range vs {
		test, err := semver.NewVersion(ver.Version)
		if err != nil {
			continue
		}

		if constraint.Check(test) {
			return ver, nil
		}
	}
	return nil, errors.Errorf("no chart version found for %s-%s", name, version)
}

// WriteFile writes an index file to the given destination path.
//
// The mode on the file is set to 'mode'.
func (i IndexFile) WriteFile(dest string, mode os.FileMode) error {
	b, err := yaml.Marshal(i)
	if err != nil {
		return err
	}
	return fileutil.AtomicWriteFile(dest, bytes.NewReader(b), mode)
}

// Merge merges the given index file into this index.
//
// This merges by name and version.
//
// If one of the entries in the given index does _not_ already exist, it is added.
// In all other cases, the existing record is preserved.
//
// This can leave the index in an unsorted state
func (i *IndexFile) Merge(f *IndexFile) {
	for _, cvs := range f.Entries {
		for _, cv := range cvs {
			if !i.Has(cv.Name, cv.Version) {
				e := i.Entries[cv.Name]
				i.Entries[cv.Name] = append(e, cv)
			}
		}
	}
}

// ChartVersion represents a chart entry in the IndexFile
type ChartVersion struct {
	*chart.Metadata
	URLs    []string  `json:"urls"`
	Created time.Time `json:"created,omitempty"`
	Removed bool      `json:"removed,omitempty"`
	Digest  string    `json:"digest,omitempty"`

	// ChecksumDeprecated is deprecated in Helm 3, and therefore ignored. Helm 3 replaced
	// this with Digest. However, with a strict YAML parser enabled, a field must be
	// present on the struct for backwards compatibility.
	ChecksumDeprecated string `json:"checksum,omitempty"`

	// EngineDeprecated is deprecated in Helm 3, and therefore ignored. However, with a strict
	// YAML parser enabled, this field must be present.
	EngineDeprecated string `json:"engine,omitempty"`

	// TillerVersionDeprecated is deprecated in Helm 3, and therefore ignored. However, with a strict
	// YAML parser enabled, this field must be present.
	TillerVersionDeprecated string `json:"tillerVersion,omitempty"`

	// URLDeprecated is deprecated in Helm 3, superseded by URLs. It is ignored. However,
	// with a strict YAML parser enabled, this must be present on the struct.
	URLDeprecated string `json:"url,omitempty"`
}

// IndexDirectory reads a (flat) directory and generates an index.
//
// It indexes only charts that have been packaged (*.tgz).
//
// The index returned will be in an unsorted state
func IndexDirectory(dir, baseURL string) (*IndexFile, error) {
	archives, err := filepath.Glob(filepath.Join(dir, "*.tgz"))
	if err != nil {
		return nil, err
	}
	moreArchives, err := filepath.Glob(filepath.Join(dir, "**/*.tgz"))
	if err != nil {
		return nil, err
	}
	archives = append(archives, moreArchives...)

	index := NewIndexFile()
	for _, arch := range archives {
		fname, err := filepath.Rel(dir, arch)
		if err != nil {
			return index, err
		}

		var parentDir string
		parentDir, fname = filepath.Split(fname)
		// filepath.Split appends an extra slash to the end of parentDir. We want to strip that out.
		parentDir = strings.TrimSuffix(parentDir, string(os.PathSeparator))
		parentURL, err := urlutil.URLJoin(baseURL, parentDir)
		if err != nil {
			parentURL = path.Join(baseURL, parentDir)
		}

		c, err := loader.Load(arch)
		if err != nil {
			// Assume this is not a chart.
			continue
		}
		hash, err := provenance.DigestFile(arch)
		if err != nil {
			return index, err
		}
		if err := index.MustAdd(c.Metadata, fname, parentURL, hash); err != nil {
			return index, errors.Wrapf(err, "failed adding to %s to index", fname)
		}
	}
	return index, nil
}

// loadIndex loads an index file and does minimal validity checking.
//
// The source parameter is only used for logging.
// This will fail if API Version is not set (ErrNoAPIVersion) or if the unmarshal fails.
func loadIndex(data []byte, source string) (*IndexFile, error) {
	i := &IndexFile{}

	if len(data) == 0 {
		return i, ErrEmptyIndexYaml
	}

	if err := yaml.UnmarshalStrict(data, i); err != nil {
		return i, err
	}

	for name, cvs := range i.Entries {
		for idx := len(cvs) - 1; idx >= 0; idx-- {
			if cvs[idx].APIVersion == "" {
				cvs[idx].APIVersion = chart.APIVersionV1
			}
			if err := cvs[idx].Validate(); err != nil {
				log.Printf("skipping loading invalid entry for chart %q %q from %s: %s", name, cvs[idx].Version, source, err)
				cvs = append(cvs[:idx], cvs[idx+1:]...)
			}
		}
	}
	i.SortEntries()
	if i.APIVersion == "" {
		return i, ErrNoAPIVersion
	}
	return i, nil
}
