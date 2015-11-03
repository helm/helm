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

	"github.com/ghodss/yaml"
)

// TODO(jackgr): Define an interface and a struct type for Configurator and move initialization to the caller.

// Configuration describes a configuration deserialized from a YAML or JSON file.
type Configuration struct {
	Resources []Resource `json:"resources"`
}

// Resource describes a resource in a deserialized configuration. A resource has
// a name, a type and a set of properties. The properties are passed directly to
// kubectl as the definition of the resource on the server.
type Resource struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

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

// Configure passes the configuration in the given deployment to kubectl
// and then updates the deployment with the completion status and completion time.
func (a *Configurator) Configure(c *Configuration, o operation) (string, error) {
	errors := &Error{}
	var output []string
	for _, resource := range c.Resources {
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

		// log.Printf("starting command:%s %s\nin directory: %s\nwith environment: %s\nwith stdin:\n%s\n",
		// cmd.Path, strings.Join(cmd.Args, " "), cmd.Dir, strings.Join(cmd.Env, "\n"), string(y))
		if err := cmd.Start(); err != nil {
			e := fmt.Errorf("cannot start kubetcl for resource: %v: %v", resource.Name, err)
			log.Println(errors.appendError(e))
			continue
		}

		if err := cmd.Wait(); err != nil {
			e := fmt.Errorf("kubetcl failed for resource: %v: %v: %v", resource.Name, err, combined.String())
			log.Println(errors.appendError(e))
			continue
		}

		output = append(output, combined.String())
		log.Printf("kubectl succeeded for resource: %v: SysTime: %v UserTime: %v\n%v",
			resource.Name, cmd.ProcessState.SystemTime(), cmd.ProcessState.UserTime(), combined.String())
	}

	if len(errors.errors) > 0 {
		return "", errors
	}

	return strings.Join(output, "\n"), nil
}
