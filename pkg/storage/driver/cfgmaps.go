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
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"

	rspb "k8s.io/helm/pkg/proto/hapi/release"

	"k8s.io/kubernetes/pkg/api"
	kberrs "k8s.io/kubernetes/pkg/api/errors"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

// ConfigMapsDriverName is the string name of the driver.
const ConfigMapsDriverName = "ConfigMap"

var b64 = base64.StdEncoding

// labels is a map of key value pairs to be included as metadata in a configmap object.
type labels map[string]string

func (lbs *labels) init()                   { *lbs = labels(make(map[string]string)) }
func (lbs labels) get(key string) string    { return lbs[key] }
func (lbs labels) set(key, val string)      { lbs[key] = val }
func (lbs labels) toMap() map[string]string { return lbs }

// ConfigMaps is a wrapper around an implementation of a kubernetes
// ConfigMapsInterface.
type ConfigMaps struct {
	impl client.ConfigMapsInterface
}

// NewConfigMaps initializes a new ConfigMaps wrapping an implmenetation of
// the kubernetes ConfigMapsInterface.
func NewConfigMaps(impl client.ConfigMapsInterface) *ConfigMaps {
	return &ConfigMaps{impl: impl}
}

// Name returns the name of the driver.
func (cfgmaps *ConfigMaps) Name() string {
	return ConfigMapsDriverName
}

// Get fetches the release named by key. The corresponding release is returned
// or error if not found.
func (cfgmaps *ConfigMaps) Get(key string) (*rspb.Release, error) {
	// fetch the configmap holding the release named by key
	obj, err := cfgmaps.impl.Get(key)
	if err != nil {
		if kberrs.IsNotFound(err) {
			return nil, ErrReleaseNotFound
		}

		logerrf(err, "get: failed to get %q", key)
		return nil, err
	}
	// found the configmap, decode the base64 data string
	r, err := decodeRelease(obj.Data["release"])
	if err != nil {
		logerrf(err, "get: failed to decode data %q", key)
		return nil, err
	}
	// return the release object
	return r, nil
}

// List fetches all releases and returns the list releases such
// that filter(release) == true. An error is returned if the
// configmap fails to retrieve the releases.
func (cfgmaps *ConfigMaps) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	list, err := cfgmaps.impl.List(api.ListOptions{})
	if err != nil {
		logerrf(err, "list: failed to list")
		return nil, err
	}

	var results []*rspb.Release

	// iterate over the configmaps object list
	// and decode each release
	for _, item := range list.Items {
		rls, err := decodeRelease(item.Data["release"])
		if err != nil {
			logerrf(err, "list: failed to decode release: %s", rls)
			continue
		}
		if filter(rls) {
			results = append(results, rls)
		}
	}
	return results, nil
}

// Create creates a new ConfigMap holding the release. If the
// ConfigMap already exists, ErrReleaseExists is returned.
func (cfgmaps *ConfigMaps) Create(rls *rspb.Release) error {
	// set labels for configmaps object meta data
	var lbs labels

	lbs.init()
	lbs.set("CREATED_AT", strconv.Itoa(int(time.Now().Unix())))

	// create a new configmap to hold the release
	obj, err := newConfigMapsObject(rls, lbs)
	if err != nil {
		logerrf(err, "create: failed to encode release %q", rls.Name)
		return err
	}
	// push the configmap object out into the kubiverse
	if _, err := cfgmaps.impl.Create(obj); err != nil {
		if kberrs.IsAlreadyExists(err) {
			return ErrReleaseExists
		}

		logerrf(err, "create: failed to create")
		return err
	}
	return nil
}

// Update updates the ConfigMap holding the release. If not found
// the ConfigMap is created to hold the release.
func (cfgmaps *ConfigMaps) Update(rls *rspb.Release) error {
	// set labels for configmaps object meta data
	var lbs labels

	lbs.init()
	lbs.set("MODIFIED_AT", strconv.Itoa(int(time.Now().Unix())))

	// create a new configmap object to hold the release
	obj, err := newConfigMapsObject(rls, lbs)
	if err != nil {
		logerrf(err, "update: failed to encode release %q", rls.Name)
		return err
	}
	// push the configmap object out into the kubiverse
	_, err = cfgmaps.impl.Update(obj)
	if err != nil {
		logerrf(err, "update: failed to update")
		return err
	}
	return nil
}

// Delete deletes the ConfigMap holding the release named by key.
func (cfgmaps *ConfigMaps) Delete(key string) (rls *rspb.Release, err error) {
	// fetch the release to check existence
	if rls, err = cfgmaps.Get(key); err != nil {
		if kberrs.IsNotFound(err) {
			return nil, ErrReleaseNotFound
		}

		logerrf(err, "delete: failed to get release %q", rls.Name)
		return nil, err
	}
	// delete the release
	if err = cfgmaps.impl.Delete(key); err != nil {
		return rls, err
	}
	return rls, nil
}

// newConfigMapsObject constructs a kubernetes ConfigMap object
// to store a release. Each configmap data entry is the base64
// encoded string of a release's binary protobuf encoding.
//
// The following labels are used within each configmap:
//
//    "MODIFIED_AT"    - timestamp indicating when this configmap was last modified. (set in Update)
//    "CREATED_AT"     - timestamp indicating when this configmap was created. (set in Create)
//    "VERSION"        - version of the release.
//    "STATUS"         - status of the release (see proto/hapi/release.status.pb.go for variants)
//    "OWNER"          - owner of the configmap, currently "TILLER".
//    "NAME"           - name of the release.
//
func newConfigMapsObject(rls *rspb.Release, lbs labels) (*api.ConfigMap, error) {
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

	// create and return configmap object
	return &api.ConfigMap{
		ObjectMeta: api.ObjectMeta{
			Name:   rls.Name,
			Labels: lbs.toMap(),
		},
		Data: map[string]string{"release": s},
	}, nil
}

// encodeRelease encodes a release returning a base64 encoded
// binary protobuf encoding representation, or error.
func encodeRelease(rls *rspb.Release) (string, error) {
	b, err := proto.Marshal(rls)
	if err != nil {
		return "", err
	}
	return b64.EncodeToString(b), nil
}

// decodeRelease decodes the bytes in data into a release
// type. Data must contain a base64 encoded string of a
// valid protobuf encoding of a release, otherwise
// an error is returned.
func decodeRelease(data string) (*rspb.Release, error) {
	// base64 decode string
	b, err := b64.DecodeString(data)
	if err != nil {
		return nil, err
	}

	var rls rspb.Release
	// unmarshal protobuf bytes
	if err := proto.Unmarshal(b, &rls); err != nil {
		return nil, err
	}
	return &rls, nil
}

// logerrf wraps an error with the a formatted string (used for debugging)
func logerrf(err error, format string, args ...interface{}) {
	log.Printf("configmaps: %s: %s\n", fmt.Sprintf(format, args...), err)
}
