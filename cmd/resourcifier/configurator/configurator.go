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

package configurator

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/util"
)

// Configurator configures a Kubernetes cluster using kubectl.
type Configurator struct {
	k util.Kubernetes
}

// NewConfigurator creates a new Configurator.
func NewConfigurator(kubernetes util.Kubernetes) *Configurator {
	return &Configurator{kubernetes}
}

// operation is an enumeration type for kubectl operations.
type operation string

// These constants implement the operation enumeration type.
const (
	CreateOperation  operation = "create"
	DeleteOperation  operation = "delete"
	GetOperation     operation = "get"
	ReplaceOperation operation = "replace"
)

// TODO(jackgr): Configure resources without dependencies in parallel.

// Error is an error type that captures errors from the multiple calls to kubectl
// made for a single configuration.
type Error struct {
	errors []error
}

// Error returns the string value of an Error.
func (e *Error) Error() string {
	errs := []string{}
	for _, err := range e.errors {
		errs = append(errs, err.Error())
	}

	return strings.Join(errs, "\n")
}

func (e *Error) appendError(err error) error {
	e.errors = append(e.errors, err)
	return err
}

// DependencyMap maps a resource name to a set of dependencies.
type DependencyMap map[string]map[string]bool

var refRe = regexp.MustCompile("\\$\\(ref\\.([^\\.]+)\\.([^\\)]+)\\)")

// Configure passes each resource in the configuration to kubectl and performs the appropriate
// action on it (create/delete/replace) and updates the State of the resource with the resulting
// status. In case of errors with a resource, Resource.State.Errors is set.
// and then updates the deployment with the completion status and completion time.
func (a *Configurator) Configure(c *common.Configuration, o operation) (string, error) {
	errors := &Error{}
	var output []string

	deps, err := getDependencies(c, o)
	if err != nil {
		e := fmt.Errorf("Error generating dependencies: %s", err.Error())
		return "", e
	}

	for {
		resources := getUnprocessedResources(c)

		// No more resources to process.
		if len(resources) == 0 {
			break
		}

		for _, r := range resources {
			// Resource still has dependencies.
			if len(deps[r.Name]) != 0 {
				continue
			}

			out, err := a.configureResource(r, o)
			if err != nil {
				log.Println(errors.appendError(err))
				abortDependants(c, deps, r.Name)

				// Resource states have changed, need to recalculate unprocessed
				// resources.
				break
			}

			output = append(output, out)
			removeDependencies(deps, r.Name)
		}
	}

	return strings.Join(output, "\n"), nil
}

func marshalResource(resource *common.Resource) (string, error) {
	if len(resource.Properties) > 0 {
		y, err := yaml.Marshal(resource.Properties)
		if err != nil {
			return "", fmt.Errorf("yaml marshal failed for resource: %v: %v", resource.Name, err)
		}
		return string(y), nil
	}
	return "", nil
}

func (a *Configurator) configureResource(resource *common.Resource, o operation) (string, error) {
	ret := ""
	var err error

	switch o {
	case CreateOperation:
		obj, err := marshalResource(resource)
		if err != nil {
			resource.State = failState(err)
			return "", err
		}
		ret, err = a.k.Create(obj)
		if err != nil {
			resource.State = failState(err)
		} else {
			resource.State = &common.ResourceState{Status: common.Created}
		}
		return ret, nil
	case ReplaceOperation:
		obj, err := marshalResource(resource)
		if err != nil {
			resource.State = failState(err)
			return "", err
		}
		ret, err = a.k.Replace(obj)
		if err != nil {
			resource.State = failState(err)
		} else {
			resource.State = &common.ResourceState{Status: common.Created}
		}
		return ret, nil
	case GetOperation:
		return a.k.Get(resource.Name, resource.Type)
	case DeleteOperation:
		obj, err := marshalResource(resource)
		if err != nil {
			resource.State = failState(err)
			return "", err
		}
		ret, err = a.k.Delete(obj)
		// Treat deleting a non-existent resource as success.
		if err != nil {
			if strings.HasSuffix(strings.TrimSpace(ret), "not found") {
				resource.State = &common.ResourceState{Status: common.Created}
				return ret, nil
			}
			resource.State = failState(err)
		}
		return ret, err
	default:
		return "", fmt.Errorf("invalid operation %s for resource: %v: %v", o, resource.Name, err)
	}
}

func failState(e error) *common.ResourceState {
	return &common.ResourceState{
		Status: common.Failed,
		Errors: []string{e.Error()},
	}
}

func getUnprocessedResources(c *common.Configuration) []*common.Resource {
	var resources []*common.Resource
	for _, r := range c.Resources {
		if r.State == nil {
			resources = append(resources, r)
		}
	}

	return resources
}

// getDependencies iterates over resources and returns a map of resource name to
// the set of dependencies that resource has.
//
// Dependencies are reversed for delete operation.
func getDependencies(c *common.Configuration, o operation) (DependencyMap, error) {
	deps := DependencyMap{}

	// Prepopulate map. This will be used later to validate referenced resources
	// actually exist.
	for _, r := range c.Resources {
		deps[r.Name] = make(map[string]bool)
	}

	for _, r := range c.Resources {
		props, err := yaml.Marshal(r.Properties)
		if err != nil {
			return nil, fmt.Errorf("Failed to deserialize resource properties for resource %s: %v", r.Name, r.Properties)
		}

		refs := refRe.FindAllStringSubmatch(string(props), -1)
		for _, ref := range refs {
			// Validate referenced resource exists in config.
			if _, ok := deps[ref[1]]; !ok {
				return nil, fmt.Errorf("Invalid resource name in reference: %s", ref[1])
			}

			// Delete dependencies should be reverse of create.
			if o == DeleteOperation {
				deps[ref[1]][r.Name] = true
			} else {
				deps[r.Name][ref[1]] = true
			}
		}
	}

	return deps, nil
}

// updateDependants removes the dependency dep from the set of dependencies for
// all resource.
func removeDependencies(deps DependencyMap, dep string) {
	for _, d := range deps {
		delete(d, dep)
	}
}

// abortDependants changes the state of all of the dependants of a resource to
// Aborted.
func abortDependants(c *common.Configuration, deps DependencyMap, dep string) {
	for _, r := range c.Resources {
		if _, ok := deps[r.Name][dep]; ok {
			r.State = &common.ResourceState{Status: common.Aborted}
		}
	}
}
