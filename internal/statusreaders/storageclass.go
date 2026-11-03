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

package statusreaders

import (
	storagev1 "k8s.io/api/storage/v1"
)

// Annotations that mark a StorageClass as the cluster default.
const (
	defaultStorageClassAnnotation     = "storageclass.kubernetes.io/is-default-class"
	betaDefaultStorageClassAnnotation = "storageclass.beta.kubernetes.io/is-default-class"
)

// WaitsForFirstConsumer reports whether a PersistentVolumeClaim with the given
// storage class name uses a StorageClass whose volume binding mode is
// WaitForFirstConsumer. Such a claim stays Pending until a Pod that uses it is
// scheduled, so it cannot become Bound while Helm waits for it.
//
// getClass returns the named StorageClass. listClasses returns all of them and
// is only called to find the default StorageClass, for a claim that does not
// name one.
func WaitsForFirstConsumer(storageClassName *string, getClass func(name string) (*storagev1.StorageClass, error), listClasses func() ([]storagev1.StorageClass, error)) (bool, error) {
	var sc *storagev1.StorageClass
	switch {
	case storageClassName == nil:
		classes, err := listClasses()
		if err != nil {
			return false, err
		}
		sc = defaultStorageClass(classes)
	case *storageClassName == "":
		// An empty name explicitly requests a claim without a StorageClass.
		return false, nil
	default:
		var err error
		if sc, err = getClass(*storageClassName); err != nil {
			return false, err
		}
	}
	return sc != nil && sc.VolumeBindingMode != nil && *sc.VolumeBindingMode == storagev1.VolumeBindingWaitForFirstConsumer, nil
}

// defaultStorageClass returns the default StorageClass, or nil if there is
// none. Like Kubernetes, it picks the most recently created one when several
// are marked as default.
func defaultStorageClass(classes []storagev1.StorageClass) *storagev1.StorageClass {
	var def *storagev1.StorageClass
	for i := range classes {
		sc := &classes[i]
		annotations := sc.GetAnnotations()
		if annotations[defaultStorageClassAnnotation] != "true" && annotations[betaDefaultStorageClassAnnotation] != "true" {
			continue
		}
		if def == nil || sc.CreationTimestamp.After(def.CreationTimestamp.Time) {
			def = sc
		}
	}
	return def
}
