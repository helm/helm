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

package expander

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ghodss/yaml"
	"log"
	"os"
	"os/exec"

	"github.com/kubernetes/helm/pkg/expansion"
)

type expander struct {
	ExpansionBinary string
}

// NewExpander returns an ExpandyBird expander.
func NewExpander(binary string) expansion.Expander {
	return &expander{binary}
}

type expandyBirdConfigOutput struct {
	Resources []interface{} `yaml:"resources,omitempty"`
}

type expandyBirdOutput struct {
	Config *expandyBirdConfigOutput `yaml:"config,omitempty"`
	Layout interface{}              `yaml:"layout,omitempty"`
}

// ExpandChart passes the given configuration to the expander and returns the
// expanded configuration as a string on success.
func (e *expander) ExpandChart(request *expansion.ServiceRequest) (*expansion.ServiceResponse, error) {

	err := expansion.ValidateRequest(request)
	if err != nil {
		return nil, err
	}

	chartInv := request.ChartInvocation
	chartFile := request.Chart.Chartfile
	chartMembers := request.Chart.Members

	if e.ExpansionBinary == "" {
		message := fmt.Sprintf("expansion binary cannot be empty")
		return nil, fmt.Errorf("%s: %s", chartInv.Name, message)
	}

	entrypointIndex := -1
	schemaIndex := -1
	for i, f := range chartMembers {
		if f.Path == chartFile.Expander.Entrypoint {
			entrypointIndex = i
		}
		if f.Path == chartFile.Schema {
			schemaIndex = i
		}
	}
	if entrypointIndex == -1 {
		message := fmt.Sprintf("The entrypoint in the chart.yaml cannot be found: %s", chartFile.Expander.Entrypoint)
		return nil, fmt.Errorf("%s: %s", chartInv.Name, message)
	}
	if chartFile.Schema != "" && schemaIndex == -1 {
		message := fmt.Sprintf("The schema in the chart.yaml cannot be found: %s", chartFile.Schema)
		return nil, fmt.Errorf("%s: %s", chartInv.Name, message)
	}

	// Those are automatically increasing buffers, so writing arbitrary large
	// data here won't block the child process.
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Now we convert the new chart representation into the form that classic ExpandyBird takes.

	chartInvJSON, err := json.Marshal(chartInv)
	if err != nil {
		return nil, fmt.Errorf("error marshalling chart invocation %s: %s", chartInv.Name, err)
	}
	content := "{ \"resources\": [" + string(chartInvJSON) + "] }"

	cmd := &exec.Cmd{
		Path: e.ExpansionBinary,
		// Note, that binary name still has to be passed argv[0].
		Args:   []string{e.ExpansionBinary, content},
		Stdout: &stdout,
		Stderr: &stderr,
	}

	if chartFile.Schema != "" {
		// appending to exsiting Env is required
		cmd.Env = append(os.Environ(), "VALIDATE_SCHEMA=1")
	}

	for i, f := range chartMembers {
		name := f.Path
		path := f.Path
		if i == entrypointIndex {
			// This is how expandyBird identifies the entrypoint.
			name = chartInv.Type
		} else if i == schemaIndex {
			// Doesn't matter what it was originally called, expandyBird expects to find it here.
			name = chartInv.Type + ".schema"
		}
		cmd.Args = append(cmd.Args, name, path, string(f.Content))
	}

	if err := cmd.Start(); err != nil {
		log.Printf("error starting expansion process: %s", err)
		return nil, err
	}

	cmd.Wait()

	log.Printf("Expansion process: pid: %d SysTime: %v UserTime: %v", cmd.ProcessState.Pid(),
		cmd.ProcessState.SystemTime(), cmd.ProcessState.UserTime())
	if stderr.String() != "" {
		return nil, fmt.Errorf("%s: %s", chartInv.Name, stderr.String())
	}

	output := &expandyBirdOutput{}
	if err := yaml.Unmarshal(stdout.Bytes(), output); err != nil {
		return nil, fmt.Errorf("cannot unmarshal expansion result (%s):\n%s", err, output)
	}

	return &expansion.ServiceResponse{Resources: output.Config.Resources}, nil
}
