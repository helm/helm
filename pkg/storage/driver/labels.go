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
	"fmt"
)

// labels is a map of key value pairs to be included as metadata in a configmap object.
type labels map[string]string

// reservedLabels specifies release secret/configmap labels for helm
var reservedLabels = []string{"createdAt", "modifiedAt", "name", "owner", "status", "version"}

func (lbs *labels) init()                { *lbs = labels(make(map[string]string)) }
func (lbs labels) get(key string) string { return lbs[key] }
func (lbs labels) set(key, val string)   { lbs[key] = val }

func (lbs labels) keys() (ls []string) {
	for key := range lbs {
		ls = append(ls, key)
	}
	return
}

func (lbs labels) match(set labels) bool {
	for _, key := range set.keys() {
		if lbs.get(key) != set.get(key) {
			return false
		}
	}
	return true
}

func (lbs labels) toMap() map[string]string { return lbs }

func (lbs *labels) fromMap(kvs map[string]string) {
	for k, v := range kvs {
		lbs.set(k, v)
	}
}

// validate validates whether user set the labels using Helm preserved labels
func validate(labels map[string]string) error {
	for _, lk := range reservedLabels {
		if _, found := labels[lk]; found {
			return fmt.Errorf("label key '%s' is reserved for helm, not available for users", lk)
		}
	}
	return nil
}

// retrieveCustomizedLabels retrieves the real customized labels from the given labels that might contain Helm preserved labels
func retrieveCustomizedLabels(labels map[string]string) map[string]string {
	copiedLabels := deepCopyStringMap(labels)
	for _, lk := range reservedLabels {
		delete(copiedLabels, lk)
	}
	return copiedLabels
}

func deepCopyStringMap(m map[string]string) map[string]string {
	ret := make(map[string]string, len(m))
	for k, v := range m {
		ret[k] = v
	}
	return ret
}
