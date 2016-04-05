/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package client

import (
	"bytes"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/kubernetes/helm/pkg/format"
	"github.com/kubernetes/helm/pkg/kubectl"
)

// Installer is capable of installing DM into Kubernetes.
//
// See InstallYAML.
type Installer struct {
	// TODO: At some point we could transform these from maps to structs.

	// Expandybird params are used to render the expandybird manifest.
	Expandybird map[string]interface{}
	// Resourcifier params are used to render the resourcifier manifest.
	Resourcifier map[string]interface{}
	// Manager params are used to render the manager manifest.
	Manager map[string]interface{}
}

// NewInstaller creates a new Installer.
func NewInstaller() *Installer {
	return &Installer{
		Expandybird:  map[string]interface{}{},
		Resourcifier: map[string]interface{}{},
		Manager:      map[string]interface{}{},
	}
}

// Install uses kubectl to install the base DM.
//
// Returns the string output received from the operation, and an error if the
// command failed.
func (i *Installer) Install(runner kubectl.Runner) (string, error) {
	b, err := i.expand()
	if err != nil {
		return "", err
	}

	o, err := runner.Create(b)
	return string(o), err
}

func (i *Installer) expand() ([]byte, error) {
	var b bytes.Buffer
	t := template.Must(template.New("manifest").Funcs(sprig.TxtFuncMap()).Parse(InstallYAML))
	err := t.Execute(&b, i)
	return b.Bytes(), err
}

// IsInstalled checks whether DM has been installed.
func IsInstalled(runner kubectl.Runner) bool {
	// Basically, we test "all-or-nothing" here: if this returns without error
	// we know that we have both the namespace and the manager API server.
	out, err := runner.GetByKind("rc", "manager-rc", "helm")
	if err != nil {
		format.Err("Installation not found: %s %s", out, err)
		return false
	}
	return true
}

// InstallYAML is the installation YAML for DM.
const InstallYAML = `
######################################################################
# Copyright 2015 The Kubernetes Authors All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
######################################################################

---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    app: helm
    name: helm-namespace
  name: helm
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: helm
    name: expandybird-service
  name: expandybird-service
  namespace: helm
spec:
  ports:
  - name: expandybird
    port: 8081
    targetPort: 8080
  selector:
    app: helm
    name: expandybird
---
apiVersion: v1
kind: ReplicationController
metadata:
  labels:
    app: helm
    name: expandybird-rc
  name: expandybird-rc
  namespace: helm
spec:
  replicas: 2
  selector:
    app: helm
    name: expandybird
  template:
    metadata:
      labels:
        app: helm
        name: expandybird
    spec:
      containers:
      - env: []
        image: {{default "gcr.io/kubernetes-helm/expandybird:latest" .Expandybird.Image}}
        name: expandybird
        ports:
        - containerPort: 8080
          name: expandybird
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: helm
    name: resourcifier-service
  name: resourcifier-service
  namespace: helm
spec:
  ports:
  - name: resourcifier
    port: 8082
    targetPort: 8080
  selector:
    app: helm
    name: resourcifier
---
apiVersion: v1
kind: ReplicationController
metadata:
  labels:
    app: helm
    name: resourcifier-rc
  name: resourcifier-rc
  namespace: helm
spec:
  replicas: 2
  selector:
    app: helm
    name: resourcifier
  template:
    metadata:
      labels:
        app: helm
        name: resourcifier
    spec:
      containers:
      - env: []
        image: {{ default "gcr.io/kubernetes-helm/resourcifier:latest" .Resourcifier.Image }}
        name: resourcifier
        ports:
        - containerPort: 8080
          name: resourcifier
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: helm
    name: manager-service
  name: manager-service
  namespace: helm
spec:
  ports:
  - name: manager
    port: 8080
    targetPort: 8080
  selector:
    app: helm
    name: manager
---
apiVersion: v1
kind: ReplicationController
metadata:
  labels:
    app: helm
    name: manager-rc
  name: manager-rc
  namespace: helm
spec:
  replicas: 1
  selector:
    app: helm
    name: manager
  template:
    metadata:
      labels:
        app: helm
        name: manager
    spec:
      containers:
      - env: []
        image: {{ default "gcr.io/kubernetes-helm/manager:latest" .Manager.Image }}
        name: manager
        ports:
        - containerPort: 8080
          name: manager
`
