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
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/internal/fileutil"
	"helm.sh/helm/v4/internal/third_party/dep/fs"
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

// WriteFile writes a repositories file to the given path atomically.
// The path is made absolute before writing so the temporary file is created in
// the destination directory (including for bare relative names). A destination
// that is a symlink is resolved and the shared target is updated, including when
// the final target is missing. perm is used only when creating a new file and is
// subject to the process umask, matching os.WriteFile; an existing destination
// keeps its mode.
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

	if fi, err := os.Stat(writePath); err == nil {
		return fileutil.AtomicWriteFile(writePath, bytes.NewReader(data), fi.Mode().Perm())
	} else if !os.IsNotExist(err) {
		return err
	}

	return atomicWriteRepoFileNew(writePath, data, perm)
}

// atomicWriteRepoFileNew creates a new destination with perm subject to umask.
// Unlike fileutil.AtomicWriteFile it does not Chmod after create, so a call with
// perm=0o644 and umask=0o077 yields 0o600 the way os.WriteFile does.
func atomicWriteRepoFileNew(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)
	f, err := os.CreateTemp(dir, ".repositories.yaml-")
	if err != nil {
		return err
	}
	tempName := f.Name()
	_ = f.Close()
	_ = os.Remove(tempName)

	f, err = os.OpenFile(tempName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tempName)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tempName)
		return err
	}

	if err := fs.RenameWithFallback(tempName, filename); err != nil {
		_ = os.Remove(tempName)
		return err
	}
	return nil
}

// maxRepoFileSymlinkHops bounds how many links resolveRepoFileWritePath
// follows, matching the limit Linux applies to path resolution.
const maxRepoFileSymlinkHops = 40

// resolveRepoFileWritePath returns an absolute write path, following symlinks
// the way the kernel does for LoadFile. When the path ends in a missing
// target, every link in front of it is followed to that target so it can be
// created while all links are kept. A final target that exists but is not a
// regular file is an error.
//
// Components are resolved one at a time rather than cleaning the path first:
// for "alias/../repositories.yaml" with alias -> actual/sub, the kernel reads
// the parent of the link's target (actual/repositories.yaml), while a lexical
// clean lands beside alias and would write a file LoadFile never reads.
func resolveRepoFileWritePath(path string) (string, error) {
	abs := path
	if !filepath.IsAbs(abs) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		// filepath.Abs would clean the path, collapsing ".." before the link in
		// front of it is resolved.
		abs = wd + string(filepath.Separator) + abs
	}

	volume, pending := splitRepoFilePath(abs)
	cur := volume + string(filepath.Separator)
	hops := 0

	for len(pending) > 0 {
		comp := pending[0]
		pending = pending[1:]

		switch comp {
		case "", ".":
			continue
		case "..":
			// cur holds no unresolved links, so its parent is the real one.
			if parent := filepath.Dir(cur); parent != cur {
				cur = parent
			}
			continue
		}

		next := filepath.Join(cur, comp)
		fi, err := os.Lstat(next)
		if err != nil {
			if os.IsNotExist(err) {
				// Nothing under a missing directory can be a link, so the rest
				// of the path is kept as it is.
				return filepath.Join(append([]string{next}, pending...)...), nil
			}
			return "", err
		}

		if fi.Mode()&os.ModeSymlink == 0 {
			cur = next
			continue
		}

		hops++
		if hops > maxRepoFileSymlinkHops {
			return "", fmt.Errorf("%s: too many levels of symbolic links", abs)
		}
		link, err := os.Readlink(next)
		if err != nil {
			return "", err
		}
		if filepath.IsAbs(link) {
			var linkPending []string
			volume, linkPending = splitRepoFilePath(link)
			pending = append(linkPending, pending...)
			cur = volume + string(filepath.Separator)
			continue
		}
		pending = append(splitComponents(link), pending...)
	}

	fi, err := os.Lstat(cur)
	if err != nil {
		return "", err
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("repository destination %q is not a regular file (mode %s)", cur, fi.Mode())
	}
	return cur, nil
}

// splitRepoFilePath splits an absolute path into its volume and the components
// below it.
func splitRepoFilePath(path string) (string, []string) {
	volume := filepath.VolumeName(path)
	return volume, splitComponents(strings.TrimPrefix(path, volume))
}

// splitComponents splits a path fragment into its components, dropping the
// leading separator of an absolute path.
func splitComponents(path string) []string {
	path = strings.TrimPrefix(path, string(filepath.Separator))
	if path == "" {
		return nil
	}
	return strings.Split(path, string(filepath.Separator))
}
