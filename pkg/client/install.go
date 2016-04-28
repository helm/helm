package client

import (
	"bytes"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/kubernetes/helm/pkg/kubectl"
)

// Installer installs tiller into Kubernetes
//
// See InstallYAML.
type Installer struct {

	// Metadata holds any global metadata attributes for the resources
	Metadata map[string]interface{}

	// Tiller specific metadata
	Tiller map[string]interface{}
}

// NewInstaller creates a new Installer
func NewInstaller() *Installer {
	return &Installer{
		Metadata: map[string]interface{}{},
		Tiller:   map[string]interface{}{},
	}
}

// Install uses kubectl to install tiller
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

// InstallYAML is the installation YAML for DM.
const InstallYAML = `
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
kind: ReplicationController
metadata:
  labels:
    app: helm
    name: tiller
  name: tiller-rc
  namespace: helm
spec:
  replicas: 1
  selector:
    app: helm
    name: tiller
  template:
    metadata:
      labels:
        app: helm
        name: tiller
    spec:
      containers:
      - env: []
        image: {{default "gcr.io/deis-sandbox/tiller:canary" .Tiller.Image}}
        name: tiller
        ports:
        - containerPort: 8080
          name: tiller
        imagePullPolicy: Always
---
`
