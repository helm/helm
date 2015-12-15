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
// used in a Deployment Manager configurations. There can be multiple
// implementations of a registry. Currently we support Deployment Manager
// github.com/kubernetes/application-dm-templates
// and helm packages
// github.com/helm/charts
//
type Type struct {
	Collection string
	Name       string
	Version    string
}

// Registry abstracts type interactions.
type Registry interface {
	// List all the templates at the given path
	List() ([]Type, error)
	// Get the download URL(s) for a given type
	GetURLs(t Type) ([]string, error)
}
