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
	"fmt"
	rapi "k8s.io/helm/api"
	"strconv"
	"strings"
	"time"

	google_protobuf "github.com/golang/protobuf/ptypes/timestamp"
	"k8s.io/kubernetes/pkg/api"
	kberrs "k8s.io/kubernetes/pkg/api/errors"
	kblabels "k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/validation"

	"k8s.io/helm/client/clientset"
	rspb "k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

var _ Driver = (*Releases)(nil)

// ReleasesDriverName is the string name of the driver.
const ReleasesDriverName = "helm.sh/Release"

// Releases is a wrapper around an implementation of a kubernetes
// ReleasesInterface.
type Releases struct {
	impl clientset.ReleaseInterface
}

// NewReleases initializes a new Releases wrapping an implmenetation of
// the kubernetes ReleasesInterface.
func NewReleases(impl clientset.ReleaseInterface) *Releases {
	return &Releases{impl: impl}
}

// Name returns the name of the driver.
func (releases *Releases) Name() string {
	return ReleasesDriverName
}

// Get fetches the release named by key. The corresponding release is returned
// or error if not found.
func (releases *Releases) Get(key string) (*rspb.Release, error) {
	// fetch the release holding the release named by key
	obj, err := releases.impl.Get(key)
	if err != nil {
		if kberrs.IsNotFound(err) {
			return nil, ErrReleaseNotFound
		}

		logerrf(err, "get: failed to get %q", key)
		return nil, err
	}
	// found the release, decode the base64 data string
	r, err := decodeRelease(obj.Spec.Data.Inline)
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
		rls, err := decodeRelease(item.Spec.Data.Inline)
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
		rls, err := decodeRelease(item.Spec.Data.Inline)
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
	obj, err := newReleasesObject(key, rls, lbs)
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
	obj, err := newReleasesObject(key, rls, lbs)
	if err != nil {
		logerrf(err, "update: failed to encode release %q", rls.Name)
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
	if rls, err = releases.Get(key); err != nil {
		if kberrs.IsNotFound(err) {
			return nil, ErrReleaseNotFound
		}

		logerrf(err, "delete: failed to get release %q", key)
		return nil, err
	}
	// delete the release
	if err = releases.impl.Delete(key); err != nil {
		return rls, err
	}
	return rls, nil
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
		ObjectMeta: api.ObjectMeta{
			Name:   key,
			Labels: lbs.toMap(),
		},
		Spec: rapi.ReleaseSpec{
			Config:  rls.Config,
			Version: rls.Version,
			Data: rapi.ReleaseData{
				Inline: s,
			},
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
	return r, nil
}

func toKubeTime(pbt *google_protobuf.Timestamp) unversioned.Time {
	var t unversioned.Time
	if pbt != nil {
		t = unversioned.NewTime(time.Unix(pbt.Seconds, int64(pbt.Nanos)))
	}
	return t
}
