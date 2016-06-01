package client

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/kubernetes/helm/pkg/kube"
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

// Install uses kubernetes client to install tiller
//
// Returns the string output received from the operation, and an error if the
// command failed.
//
// If verbose is true, this will print the manifest to stdout.
//
// If createNS is true, this will also create the namespace.
func (i *Installer) Install(verbose, createNS bool) error {

	var b bytes.Buffer
	t := template.New("manifest").Funcs(sprig.TxtFuncMap())

	// Add namespace
	if createNS {
		if err := template.Must(t.Parse(NamespaceYAML)).Execute(&b, i); err != nil {
			return err
		}
	}

	// Add main install YAML
	if err := template.Must(t.Parse(InstallYAML)).Execute(&b, i); err != nil {
		return err
	}

	if verbose {
		fmt.Println(b.String())
	}

	return kube.New(nil).Create(i.Tiller["Namespace"].(string), &b)
}

// NamespaceYAML is the installation for a namespace.
const NamespaceYAML = `
---{{$namespace := default "helm" .Tiller.Namespace}}
apiVersion: v1
kind: Namespace
metadata:
  labels:
    app: helm
    name: helm-namespace
  name: {{$namespace}}
`

// InstallYAML is the installation YAML for DM.
const InstallYAML = `
---{{$namespace := default "helm" .Tiller.Namespace}}
apiVersion: v1
kind: ReplicationController
metadata:
  labels:
    app: helm
    name: tiller
  name: tiller-rc
  namespace: {{$namespace}}
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
      - env:
          - name: DEFAULT_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
        image: {{default "gcr.io/kubernetes-helm/tiller:canary" .Tiller.Image}}
        name: tiller
        ports:
        - containerPort: 44134
          name: tiller
        imagePullPolicy: Always
				livenessProbe:
					httpGet:
						path: /liveness
						port: 44135
					initialDelaySeconds: 1
					timeoutSeconds: 1
				readinessProbe:
					httpGet:
						path: /readiness
						port: 44135
					initialDelaySeconds: 1
					timeoutSeconds:1
`
