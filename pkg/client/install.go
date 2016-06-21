package client

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig"

	"k8s.io/helm/pkg/kube"
)

// Install uses kubernetes client to install tiller
//
// Returns the string output received from the operation, and an error if the
// command failed.
//
// If verbose is true, this will print the manifest to stdout.
func Install(namespace, image string, verbose bool) error {
	kc := kube.New(nil)

	if namespace == "" {
		ns, _, err := kc.DefaultNamespace()
		if err != nil {
			return err
		}
		namespace = ns
	}

	var b bytes.Buffer

	// Add main install YAML
	istpl := template.New("install").Funcs(sprig.TxtFuncMap())

	cfg := struct {
		Namespace, Image string
	}{namespace, image}

	if err := template.Must(istpl.Parse(InstallYAML)).Execute(&b, cfg); err != nil {
		return err
	}

	if verbose {
		fmt.Println(b.String())
	}

	return kc.Create(namespace, &b)
}

// InstallYAML is the installation YAML for DM.
const InstallYAML = `
---
apiVersion: v1
kind: ReplicationController
metadata:
  labels:
    app: helm
    name: tiller
  name: tiller-rc
  namespace: {{ .Namespace }}
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
        image: {{default "gcr.io/kubernetes-helm/tiller:canary" .Image}}
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
          timeoutSeconds: 1
`
