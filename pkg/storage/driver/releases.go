/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package driver // import "k8s.io/helm/pkg/storage/driver"

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	google_protobuf "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/graymeta/stow"
	"k8s.io/kubernetes/pkg/api"
	kberrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kblabels "k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/validation"

	rapi "k8s.io/helm/api"
	"k8s.io/helm/client/clientset"
	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

var _ Driver = (*Releases)(nil)

// ReleasesDriverName is the string name of the driver.
const ReleasesDriverName = "helm.sh/Release"

// Releases is a wrapper around an implementation of a kubernetes
// ReleasesInterface.
type Releases struct {
	impl      clientset.ReleaseInterface
	container stow.Container
	prefix    string
}

// NewReleases initializes a new Releases wrapping an implmenetation of
// the kubernetes ReleasesInterface.
func NewReleases(impl clientset.ReleaseInterface) *Releases {
	return &Releases{impl: impl}
}

func NewObjectStoreReleases(impl clientset.ReleaseInterface, c stow.Container, prefix string) *Releases {
	p := prefix
	if prefix == "" {
		p = "tiller"
	}
	return &Releases{impl: impl, container: c, prefix: p}
}

// Name returns the name of the driver.
func (releases *Releases) Name() string {
	return ReleasesDriverName
}

// Get fetches the release named by key. The corresponding release is returned
// or error if not found.
func (releases *Releases) Get(key string) (*rspb.Release, error) {
	// fetch the release holding the release named by key
	obj, err := releases.impl.Get(toTPRSafeKey(key))
	if err != nil {
		if kberrs.IsNotFound(err) {
			return nil, ErrReleaseNotFound
		}

		logerrf(err, "get: failed to get %q", key)
		return nil, err
	}

	// found the release, decode the base64 data string
	data, err := releases.getReleaseData(obj)
	if err != nil {
		return nil, err
	}
	r, err := decodeRelease(data)
	if err != nil {
		logerrf(err, "get: failed to decode data %q", key)
		return nil, err
	}
	// return the release object
	return r, nil
}

// List fetches all releases and returns the list releases such
// that filter(release) == true. An error is returned if the
// release fails to retrieve the releases.
func (releases *Releases) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	lsel := kblabels.Set{"OWNER": "TILLER"}.AsSelector()
	opts := api.ListOptions{LabelSelector: lsel}

	list, err := releases.impl.List(opts)
	if err != nil {
		logerrf(err, "list: failed to list")
		return nil, err
	}

	var results []*rspb.Release

	// iterate over the releases object list
	// and decode each release
	for _, item := range list.Items {
		data, err := releases.getReleaseData(&item)
		if err != nil {
			return nil, err
		}
		rls, err := decodeRelease(data)
		if err != nil {
			logerrf(err, "list: failed to decode release: %v", item)
			continue
		}
		if filter(rls) {
			results = append(results, rls)
		}
	}
	return results, nil
}

// Query fetches all releases that match the provided map of labels.
// An error is returned if the release fails to retrieve the releases.
func (releases *Releases) Query(labels map[string]string) ([]*rspb.Release, error) {
	ls := kblabels.Set{}
	for k, v := range labels {
		if errs := validation.IsValidLabelValue(v); len(errs) != 0 {
			return nil, fmt.Errorf("invalid label value: %q: %s", v, strings.Join(errs, "; "))
		}
		ls[k] = v
	}

	opts := api.ListOptions{LabelSelector: ls.AsSelector()}

	list, err := releases.impl.List(opts)
	if err != nil {
		logerrf(err, "query: failed to query with labels")
		return nil, err
	}

	if len(list.Items) == 0 {
		return nil, ErrReleaseNotFound
	}

	var results []*rspb.Release
	for _, item := range list.Items {
		data, err := releases.getReleaseData(&item)
		if err != nil {
			return nil, err
		}
		rls, err := decodeRelease(data)
		if err != nil {
			logerrf(err, "query: failed to decode release: %s", err)
			continue
		}
		results = append(results, rls)
	}
	return results, nil
}

// Create creates a new Release holding the release. If the
// Release already exists, ErrReleaseExists is returned.
func (releases *Releases) Create(key string, rls *rspb.Release) error {
	// set labels for releases object meta data
	var lbs labels

	lbs.init()
	lbs.set("CREATED_AT", strconv.Itoa(int(time.Now().Unix())))

	// create a new release to hold the release
	obj, err := newReleasesObject(toTPRSafeKey(key), rls, lbs)
	if err != nil {
		logerrf(err, "create: failed to encode release %q", rls.Name)
		return err
	}
	// push the release object data to object store if configured
	err = releases.writeReleaseData(obj)
	if err != nil {
		logerrf(err, "create: failed to encode release %q", rls.Name)
		return err
	}
	// push the release object out into the kubiverse
	if _, err := releases.impl.Create(obj); err != nil {
		if kberrs.IsAlreadyExists(err) {
			return ErrReleaseExists
		}

		logerrf(err, "create: failed to create")
		return err
	}
	return nil
}

// Update updates the Release holding the release. If not found
// the Release is created to hold the release.
func (releases *Releases) Update(key string, rls *rspb.Release) error {
	// set labels for releases object meta data
	var lbs labels

	lbs.init()
	lbs.set("MODIFIED_AT", strconv.Itoa(int(time.Now().Unix())))

	// create a new release object to hold the release
	obj, err := newReleasesObject(toTPRSafeKey(key), rls, lbs)
	if err != nil {
		logerrf(err, "update: failed to encode release %q", rls.Name)
		return err
	}
	// push the release object data to object store if configured
	err = releases.writeReleaseData(obj)
	if err != nil {
		logerrf(err, "create: failed to encode release %q", rls.Name)
		return err
	}
	// push the release object out into the kubiverse
	_, err = releases.impl.Update(obj)
	if err != nil {
		logerrf(err, "update: failed to update")
		return err
	}
	return nil
}

// Delete deletes the Release holding the release named by key.
func (releases *Releases) Delete(key string) (rls *rspb.Release, err error) {
	// fetch the release to check existence
	if rls, err = releases.Get(toTPRSafeKey(key)); err != nil {
		if kberrs.IsNotFound(err) {
			return nil, ErrReleaseNotFound
		}

		logerrf(err, "delete: failed to get release %q", key)
		return nil, err
	}
	// delete the release
	if err = releases.deleteReleaseData(rls); err != nil {
		return rls, err
	}
	if err = releases.impl.Delete(key); err != nil {
		return rls, err
	}
	return rls, nil
}

func (releases *Releases) itemIDFromTPR(rls *rapi.Release) string {
	return fmt.Sprintf("%v/releases/%v", releases.prefix, rls.Name)
}

func (releases *Releases) itemIDFromProto(rls *rspb.Release) string {
	return fmt.Sprintf("%v/releases/%v", releases.prefix, toTPRSafeKey(rls.Name))
}

func (releases *Releases) deleteReleaseData(rls *rspb.Release) error {
	if releases.container != nil {
		return releases.container.RemoveItem(releases.itemIDFromProto(rls))
	}
	return nil
}

func (releases *Releases) writeReleaseData(rls *rapi.Release) error {
	if releases.container != nil {
		b := bytes.NewBufferString(rls.Spec.Data)
		sz := len(rls.Spec.Data)
		rls.Spec.Data = ""
		_, err := releases.container.Put(releases.itemIDFromTPR(rls), b, int64(sz), nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (releases *Releases) getReleaseData(rls *rapi.Release) (string, error) {
	if rls.Spec.Data != "" {
		return rls.Spec.Data, nil
	} else if releases.container != nil {
		item, err := releases.container.Item(releases.itemIDFromTPR(rls))
		if err != nil {
			return "", err
		}

		f, err := item.Open()
		if err != nil {
			return "", err
		}
		defer f.Close()
		// It's a good but not certain bet that FileInfo will tell us exactly how much to
		// read, so let's try it but be prepared for the answer to be wrong.
		var n int64
		// Don't preallocate a huge buffer, just in case.
		if size, err := item.Size(); err != nil && size < 1e9 {
			n = size
		}
		// As initial capacity for readAll, use n + a little extra in case Size is zero,
		// and to avoid another allocation after Read has filled the buffer. The readAll
		// call will read into its allocated internal buffer cheaply. If the size was
		// wrong, we'll either waste some space off the end or reallocate as needed, but
		// in the overwhelmingly common case we'll get it just right.
		b, err := readAll(f, n+bytes.MinRead)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return "", fmt.Errorf("Missing release data for %v", rls.Name)
}

var (
	protoRegex = regexp.MustCompile(`^[a-z0-9][-a-z0-9]*.v[0-9]+$`)
)

func toTPRSafeKey(key string) string {
	if protoRegex.MatchString(key) {
		i := strings.LastIndex(key, ".v")
		return key[:i] + "-" + key[i+1:]
	} else {
		return key
	}
}

// readAll reads from r until an error or EOF and returns the data it read
// from the internal buffer allocated with a specified capacity.
func readAll(r io.Reader, capacity int64) (b []byte, err error) {
	buf := bytes.NewBuffer(make([]byte, 0, capacity))
	// If the buffer overflows, we will get bytes.ErrTooLarge.
	// Return that as an error. Any other panic remains.
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		if panicErr, ok := e.(error); ok && panicErr == bytes.ErrTooLarge {
			err = panicErr
		} else {
			panic(e)
		}
	}()
	_, err = buf.ReadFrom(r)
	return buf.Bytes(), err
}

// newReleasesObject constructs a kubernetes Release object
// to store a release. Each release data entry is the base64
// encoded string of a release's binary protobuf encoding.
//
// The following labels are used within each release:
//
//    "MODIFIED_AT"    - timestamp indicating when this release was last modified. (set in Update)
//    "CREATED_AT"     - timestamp indicating when this release was created. (set in Create)
//    "VERSION"        - version of the release.
//    "STATUS"         - status of the release (see proto/hapi/release.status.pb.go for variants)
//    "OWNER"          - owner of the release, currently "TILLER".
//    "NAME"           - name of the release.
//
func newReleasesObject(key string, rls *rspb.Release, lbs labels) (*rapi.Release, error) {
	const owner = "TILLER"

	// TODO(tamal): Just store the proto bytes directly in cloud bucket.
	// encode the release
	s, err := encodeRelease(rls)
	if err != nil {
		return nil, err
	}

	if lbs == nil {
		lbs.init()
	}

	// apply labels
	lbs.set("NAME", rls.Name)
	lbs.set("OWNER", owner)
	lbs.set("STATUS", rspb.Status_Code_name[int32(rls.Info.Status.Code)])
	lbs.set("VERSION", strconv.Itoa(int(rls.Version)))

	// create and return release object
	r := &rapi.Release{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Release",
			APIVersion: "helm.sh/v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:   key,
			Labels: lbs.toMap(),
		},
		Spec: rapi.ReleaseSpec{
			Config:  map[string]string{},
			Version: rls.Version,
			Data:    s,
		},
		Status: rapi.ReleaseStatus{
			FirstDeployed: toKubeTime(rls.Info.FirstDeployed),
			LastDeployed:  toKubeTime(rls.Info.LastDeployed),
			Deleted:       toKubeTime(rls.Info.Deleted),
		},
	}
	if rls.Info != nil {
		r.Spec.Description = rls.Info.Description
		if rls.Info.Status != nil {
			r.Status.Code = rls.Info.Status.Code.String()
			r.Status.Resources = rls.Info.Status.Resources
			r.Status.Notes = rls.Info.Status.Notes
		}
	}
	if rls.Chart != nil {
		r.Spec.ChartMetadata = rls.Chart.Metadata
	}
	if rls.Config != nil {
		for k, v := range rls.Config.Values {
			r.Spec.Config[k] = v.Value
		}
	}
	return r, nil
}

func toKubeTime(pbt *google_protobuf.Timestamp) *unversioned.Time {
	if pbt != nil {
		t := unversioned.NewTime(time.Unix(pbt.Seconds, int64(pbt.Nanos)))
		return &t
	}
	return nil
}
