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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/internal/fileutil"
)

// File represents the repositories.yaml file
type File struct {
	APIVersion   string    `json:"apiVersion"`
	Generated    time.Time `json:"generated"`
	Repositories []*Entry  `json:"repositories"`
}

// NewFile generates an empty repositories file.
//
// Generated and APIVersion are automatically set.
func NewFile() *File {
	return &File{
		APIVersion:   APIVersionV1,
		Generated:    time.Now(),
		Repositories: []*Entry{},
	}
}

// LoadFile takes a file at the given path and returns a File object
func LoadFile(path string) (*File, error) {
	r := new(File)
	b, err := os.ReadFile(path)
	if err != nil {
		return r, fmt.Errorf("couldn't load repositories file (%s): %w", path, err)
	}

	err = yaml.Unmarshal(b, r)
	return r, err
}

// Add adds one or more repo entries to a repo file.
func (r *File) Add(re ...*Entry) {
	r.Repositories = append(r.Repositories, re...)
}

// Update attempts to replace one or more repo entries in a repo file. If an
// entry with the same name doesn't exist in the repo file it will add it.
func (r *File) Update(re ...*Entry) {
	for _, target := range re {
		r.update(target)
	}
}

func (r *File) update(e *Entry) {
	for j, repo := range r.Repositories {
		if repo.Name == e.Name {
			r.Repositories[j] = e
			return
		}
	}
	r.Add(e)
}

// Has returns true if the given name is already a repository name.
func (r *File) Has(name string) bool {
	entry := r.Get(name)
	return entry != nil
}

// Get returns an entry with the given name if it exists, otherwise returns nil
func (r *File) Get(name string) *Entry {
	for _, entry := range r.Repositories {
		if entry.Name == name {
			return entry
		}
	}
	return nil
}

// Remove removes the entry from the list of repositories.
func (r *File) Remove(name string) bool {
	cp := []*Entry{}
	found := false
	for _, rf := range r.Repositories {
		if rf == nil {
			continue
		}
		if rf.Name == name {
			found = true
			continue
		}
		cp = append(cp, rf)
	}
	r.Repositories = cp
	return found
}

// WriteFile writes a repositories file to the given path via AtomicWriteFile.
// The path is made absolute before writing so the temporary file is created in
// the destination directory (including for bare relative names). A destination
// that is a symlink is resolved and the shared target is updated. perm is used
// only when creating a new file; an existing destination keeps its mode.
func (r *File) WriteFile(path string, perm os.FileMode) error {
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}

	writePath, err := resolveRepoFileWritePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(writePath), 0o755); err != nil {
		return err
	}

	mode := perm
	if fi, err := os.Stat(writePath); err == nil {
		mode = fi.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}

	return fileutil.AtomicWriteFile(writePath, bytes.NewReader(data), mode)
}

// resolveRepoFileWritePath returns an absolute write path, following a symlink
// destination to its target when present.
func resolveRepoFileWritePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	fi, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return abs, nil
		}
		return "", err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return abs, nil
	}

	target, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return target, nil
}
