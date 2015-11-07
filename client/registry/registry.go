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

package registry

// Registry abstracts a types registry which holds types that can be
// used in a Deployment Manager configurations. A registry root must have
// a 'types' directory which contains all the available types. Each type
// then contains version directories which in turn contains all the files
// necessary for that type.
// For example a type registry holding two types:
//   redis v1               (implemented in jinja)
//   replicatedservice v2   (implemented in python)
// would have a directory structure like so:
// /types/redis/v1
//   redis.jinja
//   redis.jinja.schema
// /types/replicatedservice/v2
//   replicatedservice.python
//   replicatedservice.python.schema

const TypesDir string = "types"

type Type struct {
	Name string
	Version string
}

// Registry abstracts type interactions.
type Registry interface {
	// List all the types in the given registry
	List() ([]Type, error)
	// Get the download URL for a given type and version
	GetURL(t Type) (string, error)
}
