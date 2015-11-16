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

// Registry abstracts a registry that holds templates, which can be
// used in a Deployment Manager configurations. A registry root must be a
// directory that contains all the available templates, one directory per
// template. Each template directory then contains version directories, each
// of which in turn contains all the files necessary for that version of the
// template.
// For example, a template registry containing two versions of redis
// (implemented in jinja), and one version of replicatedservice (implemented
// in python) would have a directory structure that looks something like this:
// /redis
//   /v1
//     redis.jinja
//     redis.jinja.schema
//   /v2
//     redis.jinja
//     redis.jinja.schema
// /replicatedservice
//   /v1
//     replicatedservice.python
//     replicatedservice.python.schema

type Type struct {
	Name    string
	Version string
}

// Registry abstracts type interactions.
type Registry interface {
	// List all the templates at the given path
	List() ([]Type, error)
	// Get the download URL for a given template and version
	GetURL(t Type) (string, error)
}
