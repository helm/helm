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
	"github.com/satori/go.uuid"

	rspb "k8s.io/helm/pkg/proto/hapi/release"

	"k8s.io/kubernetes/pkg/api"
	kberrs "k8s.io/kubernetes/pkg/api/errors"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
)

var b64 = base64.StdEncoding

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

// Get fetches the release named by key. The corresponding release is returned
// or error if not found.
func (cfgmaps *ConfigMaps) Get(key string) (*rspb.Release, error) {
	// fetch the configmap holding the release named by key
	obj, err := cfgmaps.impl.Get(key)
	if err != nil {
		logerrf(err, "configmaps: failed to get %q", key)
		return nil, err
	}
	// found the configmap, decode the base64 data string
	r, err := decodeRelease(obj.Data[key])
	if err != nil {
		logerrf(err, "configmaps: failed to decode data %q", key)
		return nil, err
	}
	// return the release object
	return r, nil
}

// List fetches all releases and returns a list for all releases
// where filter(release) == true. An error is returned if the
// configmap fails to retrieve the releases.
//
// TODO: revisit List and use labels correctly.
func (cfgmaps *ConfigMaps) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	// initialize list options to return all configmaps. TODO: Apply appropriate labels
	var lbls labels.Set

	objs, err := cfgmaps.impl.List(api.ListOptions{LabelSelector: lbls.AsSelector()})
	if err != nil {
		logerrf(err, "configmaps: failed to list")
		return nil, err
	}

	// TODO: apply filter here
	var list []*rspb.Release
	_ = objs

	return list, nil
}

// Create creates a ConfigMap and stores the release. An error
// is returned if the rls already exists.
func (cfgmaps *ConfigMaps) Create(rls *rspb.Release) error {
	// create a new configmap object from the release
	obj, err := newConfigMapsObject(rls)
	if err != nil {
		logerrf(err, "configmaps: failed to encode release %q", rls.Name)
		return err
	}
	// push the configmap object out into the kubiverse
	if _, err := cfgmaps.impl.Create(obj); err != nil {
		logerrf(err, "configmaps: failed to create")
		return err
	}
	return nil
}

// Update updates the ConfigMap holding the release. If not found
// the ConfigMap is created to hold the release.
func (cfgmaps *ConfigMaps) Update(rls *rspb.Release) error {
	// create a new configmap object from the release
	obj, err := newConfigMapsObject(rls)
	if err != nil {
		logerrf(err, "configmaps: update failed to encode release %q", rls.Name)
		return err
	}
	// push the configmap object out into the kubiverse
	if _, err = cfgmaps.impl.Create(obj); err != nil {
		if !kberrs.IsAlreadyExists(err) {
			logerrf(err, "configmaps: update failed to create")
			return err
		}
	}
	// try update
	_, err = cfgmaps.impl.Update(obj)
	return err
}

// Delete deletes the ConfigMap holding the release named by key.
func (cfgmaps *ConfigMaps) Delete(key string) (rls *rspb.Release, err error) {
	// fetch the release
	if rls, err = cfgmaps.Get(key); err != nil {
		return nil, err
	}
	// delete the release
	if err = cfgmaps.impl.Delete(key); err != nil {
		return rls, err
	}
	return
}

// newConfigMapsObject constructs a kubernetes ConfigMap object
// from a release. Each configmap data entry is the base64 encoded
// string of a release's binary protobuf encoding.
//
// The following labels are used within each configmap:
//
//    "LAST_MODIFIED" - timestamp indicating when this configmap was last modified.
//    "CREATED_AT"    - timestamp indicating when this configmap was created.
//    "VERSION"        - version of the release.
//    "OWNER"          - owner of the configmap, currently "TILLER".
//    "NAME"           - name of the release.
//
func newConfigMapsObject(rls *rspb.Release) (*api.ConfigMap, error) {
	const owner = "TILLER"

	// encode the release
	s, err := encodeRelease(rls)
	if err != nil {
		return nil, err
	}

	// default labels applied to the configmap
	var labels = map[string]string{
		"MODIFIED_AT": time.Now().String(),
		"CREATED_AT":  time.Now().String(),
		"VERSION":     strconv.Itoa(int(rls.Version)), // release version
		"OWNER":       owner,                          // release owner
		"NAME":        rls.Name,                       // release name
	}

	// create and return configmap object
	return &api.ConfigMap{
		ObjectMeta: api.ObjectMeta{
			Name:   genObjKey(rls.Name, rls.Namespace),
			Labels: labels,
		},
		Data: map[string]string{rls.Name: s},
	}, nil
}

// encodeRelease encodes a release returning a base64 encoded binary protobuf
// encoding representation, or error.
func encodeRelease(rls *rspb.Release) (string, error) {
	b, err := proto.Marshal(rls)
	if err != nil {
		return "", err
	}
	return b64.EncodeToString(b), nil
}

// decodeRelease decodes the bytes in data into a release
// type. Data must contain a valid base64 encoded string
// of a valid protobuf encoding of a release, otherwise
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

// genObjKey generates a key to identify a configmap.
// The resulting key is a composition of the release name,
// the namespace of the release, and a v4 uuid.
func genObjKey(rls string, ns string) string {
	return fmt.Sprintf("%s.%s.%s", rls, ns, uuid.NewV4())
}

// for debugging
func logerrf(err error, format string, args ...interface{}) {
	log.Println("%s: %s", fmt.Sprintf(format, args...), err)
}
