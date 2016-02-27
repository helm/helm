/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package util

import (
	"fmt"
	"time"

	"github.com/ghodss/yaml"
	"github.com/kubernetes/deployment-manager/pkg/common"
)

// ParseKubernetesObject parses a Kubernetes API object in YAML format.
func ParseKubernetesObject(object []byte) (*common.Resource, error) {
	o := &common.KubernetesObject{}
	if err := yaml.Unmarshal(object, &o); err != nil {
		return nil, fmt.Errorf("cannot unmarshal native kubernetes object (%#v)", err)
	}

	// Ok, it appears to be a valid object, create a Resource out of it.
	r := &common.Resource{}
	r.Name = getRandomName(o.Metadata["name"].(string))
	r.Type = o.Kind

	r.Properties = make(map[string]interface{})
	if err := yaml.Unmarshal(object, &r.Properties); err != nil {
		return nil, fmt.Errorf("cannot unmarshal native kubernetes object (%#v)", err)
	}
	return r, nil
}

func getRandomName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
}
