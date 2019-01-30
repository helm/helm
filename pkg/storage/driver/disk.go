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

package driver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	rspb "k8s.io/helm/pkg/proto/hapi/release"
	storageerrors "k8s.io/helm/pkg/storage/errors"
)

var _ Driver = (*Disk)(nil)

// DiskDriverName is the string name of this driver.
const DiskDriverName = "Disk"

// Disk is the in-Disk storage driver implementation.
type Disk struct {
	dir string
	Log func(string, ...interface{})
}

// NewDisk initializes a new Disk driver.
func NewDisk() (*Disk, error) {
	disk := &Disk{dir: "releases/data", Log: func(_ string, _ ...interface{}) {}}
	if _, err := os.Stat(disk.dir); err != nil {
		err := os.MkdirAll(disk.dir, 0744)
		if err != nil {
			disk.Log("unable to create releases directory", err)
			return nil, fmt.Errorf("unable to create releases directory %v", err)
		}
	}
	return disk, nil
}

// Name returns the name of the driver.
func (disk *Disk) Name() string {
	return DiskDriverName
}

// Get returns the release named by key or returns ErrReleaseNotFound.
func (disk *Disk) Get(key string) (*rspb.Release, error) {
	files, err := ioutil.ReadDir(disk.dir)
	if err != nil {
		disk.Log(fmt.Sprintf("unable to list files in %v", disk.dir))
		return nil, fmt.Errorf("unable to list files in %v", disk.dir)
	}
	for _, v := range files {
		if v.IsDir() {
			continue
		}
		if v.Name() == key {
			rel, err := torelease(fmt.Sprintf("%v%v%v", disk.dir, string(os.PathSeparator), v.Name()))
			if err != nil {
				return nil, err
			}
			return rel, nil
		}
	}
	disk.Log(fmt.Sprintf("release %v not found", key))
	return nil, storageerrors.ErrReleaseNotFound(key)
}

func torelease(f string) (*rspb.Release, error) {
	rel := &rspb.Release{}
	d, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %v", f)
	}
	err = json.Unmarshal(d, rel)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal file %v", f)
	}
	return rel, nil
}

// List returns the list of all releases such that filter(release) == true
func (disk *Disk) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	files, err := ioutil.ReadDir(disk.dir)
	if err != nil {
		return nil, fmt.Errorf("unable to list files in %v", disk.dir)
	}

	result := []*rspb.Release{}
	for _, v := range files {
		if v.IsDir() {
			continue
		}
		rel, err := torelease(fmt.Sprintf("%v%v%v", disk.dir, string(os.PathSeparator), v.Name()))
		if err != nil {
			return nil, fmt.Errorf("unable to process file %v", v.Name())
		}
		if filter(rel) {
			result = append(result, rel)
		}
	}

	return result, nil
}

// Query returns the set of releases that match the provided set of labels
func (disk *Disk) Query(keyvals map[string]string) ([]*rspb.Release, error) {
	var lbs labels
	var ls []*rspb.Release
	lbs.init()
	lbs.fromMap(keyvals)
	disk.List(func(r *rspb.Release) bool {
		n := strings.Split(r.GetName(), ".")
		rec := newRecord(n[0], r)
		if rec == nil {
			return false
		}
		if rec.lbs.match(lbs) {
			ls = append(ls, rec.rls)
		}
		return true
	})
	if len(ls) == 0 {
		return nil, storageerrors.ErrReleaseNotFound(lbs["NAME"])
	}
	return ls, nil
}

// Create creates a new release or returns ErrReleaseExists.
func (disk *Disk) Create(key string, rls *rspb.Release) error {
	d, err := json.Marshal(rls)
	if err != nil {
		return fmt.Errorf("unable to convert release to json")
	}
	file := fmt.Sprintf("%v%v%v", disk.dir, string(os.PathSeparator), key)
	err = ioutil.WriteFile(file, d, 0644)
	if err != nil {
		return fmt.Errorf("unable to write release to disk")
	}
	return nil
}

// Update updates a release or returns ErrReleaseNotFound.
func (disk *Disk) Update(key string, rls *rspb.Release) error {
	d, err := json.Marshal(rls)
	if err != nil {
		return fmt.Errorf("unable to convert release to json")
	}
	err = ioutil.WriteFile(fmt.Sprintf("%v%v%v", disk.dir, string(os.PathSeparator), key), d, 0644)
	if err != nil {
		return fmt.Errorf("unable to write release to disk")
	}
	return nil
}

// Delete deletes a release or returns ErrReleaseNotFound.
func (disk *Disk) Delete(key string) (*rspb.Release, error) {
	file := fmt.Sprintf("%v%v%v", disk.dir, string(os.PathSeparator), key)
	rel, err := disk.Get(key)
	if err != nil {
		return nil, storageerrors.ErrReleaseNotFound(key)
	}
	err = os.Remove(file)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("unable to delete file %v", file))
	}
	return rel, nil
}
