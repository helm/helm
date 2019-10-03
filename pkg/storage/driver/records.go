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

package driver // import "helm.sh/helm/v3/pkg/storage/driver"

import (
	"sort"
	"strconv"

	rspb "helm.sh/helm/v3/pkg/release"
)

// records holds a list of in-memory release records
type records []*record

func (rs records) Len() int           { return len(rs) }
func (rs records) Swap(i, j int)      { rs[i], rs[j] = rs[j], rs[i] }
func (rs records) Less(i, j int) bool { return rs[i].rls.Version < rs[j].rls.Version }

func (rs *records) Add(r *record) error {
	if r == nil {
		return nil
	}

	if rs.Exists(r.key) {
		return ErrReleaseExists
	}

	*rs = append(*rs, r)
	sort.Sort(*rs)

	return nil
}

func (rs records) Get(key string) *record {
	if i, ok := rs.Index(key); ok {
		return rs[i]
	}
	return nil
}

func (rs *records) Iter(fn func(int, *record) bool) {
	cp := make([]*record, len(*rs))
	copy(cp, *rs)

	for i, r := range cp {
		if !fn(i, r) {
			return
		}
	}
}

func (rs *records) Index(key string) (int, bool) {
	for i, r := range *rs {
		if r.key == key {
			return i, true
		}
	}
	return -1, false
}

func (rs records) Exists(key string) bool {
	_, ok := rs.Index(key)
	return ok
}

func (rs *records) Remove(key string) (r *record) {
	if i, ok := rs.Index(key); ok {
		return rs.removeAt(i)
	}
	return nil
}

func (rs *records) Replace(key string, rec *record) *record {
	if i, ok := rs.Index(key); ok {
		old := (*rs)[i]
		(*rs)[i] = rec
		return old
	}
	return nil
}

func (rs *records) removeAt(index int) *record {
	r := (*rs)[index]
	(*rs)[index] = nil
	copy((*rs)[index:], (*rs)[index+1:])
	*rs = (*rs)[:len(*rs)-1]
	return r
}

// record is the data structure used to cache releases
// for the in-memory storage driver
type record struct {
	key string
	lbs labels
	rls *rspb.Release
}

// newRecord creates a new in-memory release record
func newRecord(key string, rls *rspb.Release) *record {
	var lbs labels

	lbs.init()
	lbs.set("name", rls.Name)
	lbs.set("owner", "helm")
	lbs.set("status", rls.Info.Status.String())
	lbs.set("version", strconv.Itoa(rls.Version))

	// return &record{key: key, lbs: lbs, rls: proto.Clone(rls).(*rspb.Release)}
	return &record{key: key, lbs: lbs, rls: rls}
}
