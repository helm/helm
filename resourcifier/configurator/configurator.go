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
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/kubernetes/deployment-manager/common"
	"github.com/ghodss/yaml"
)

// TODO(jackgr): Define an interface and a struct type for Configurator and move initialization to the caller.

type Configurator struct {
	KubePath  string
	Arguments []string
}

func NewConfigurator(kubectlPath string, arguments []string) *Configurator {
	return &Configurator{kubectlPath, arguments}
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

func (o operation) String() string {
	return string(o)
}

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

// Configure passes each resource in the configuration to kubectl and performs the appropriate
// action on it (create/delete/replace) and updates the State of the resource with the resulting
// status. In case of errors with a resource, Resource.State.Errors is set. 
// and then updates the deployment with the completion status and completion time.
func (a *Configurator) Configure(c *common.Configuration, o operation) (string, error) {
	errors := &Error{}
	var output []string
	for i, resource := range c.Resources {
		args := []string{o.String()}
		if o == GetOperation {
			args = append(args, "-o", "yaml")
			if resource.Type != "" {
				args = append(args, resource.Type)
				if resource.Name != "" {
					args = append(args, resource.Name)
				}
			}
		}

		var y []byte
		if len(resource.Properties) > 0 {
			var err error
			y, err = yaml.Marshal(resource.Properties)
			if err != nil {
				e := fmt.Errorf("yaml marshal failed for resource: %v: %v", resource.Name, err)
				log.Println(errors.appendError(e))
				c.Resources[i].State = &common.ResourceState{
					Status: common.Aborted,
					Errors: []string{e.Error()},
				}
				continue
			}
		}

		if len(y) > 0 {
			args = append(args, "-f", "-")
		}

		args = append(args, a.Arguments...)
		cmd := exec.Command(a.KubePath, args...)
		cmd.Stdin = bytes.NewBuffer(y)

		// Combine stdout and stderr into a single dynamically resized buffer
		combined := &bytes.Buffer{}
		cmd.Stdout = combined
		cmd.Stderr = combined

		if err := cmd.Start(); err != nil {
			e := fmt.Errorf("cannot start kubetcl for resource: %v: %v", resource.Name, err)
			c.Resources[i].State = &common.ResourceState{
				Status: common.Failed,
				Errors: []string{e.Error()},
			}
			log.Println(errors.appendError(e))
			continue
		}

		if err := cmd.Wait(); err != nil {
			// Treat delete special. If a delete is issued and a resource is not found, treat it as
			// success.
			if (o == DeleteOperation && strings.HasSuffix(strings.TrimSpace(combined.String()), "not found")) {
				log.Println(resource.Name + " not found, treating as success for delete")
			} else {
				e := fmt.Errorf("kubetcl failed for resource: %v: %v: %v", resource.Name, err, combined.String())
				c.Resources[i].State = &common.ResourceState{
					Status: common.Failed,
					Errors: []string{e.Error()},
				}
				log.Println(errors.appendError(e))
				continue
			}
		}

		output = append(output, combined.String())
		c.Resources[i].State = &common.ResourceState{Status: common.Created}
		log.Printf("kubectl succeeded for resource: %v: SysTime: %v UserTime: %v\n%v",
			resource.Name, cmd.ProcessState.SystemTime(), cmd.ProcessState.UserTime(), combined.String())
	}
	return strings.Join(output, "\n"), nil
}
