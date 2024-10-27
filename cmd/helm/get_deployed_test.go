/*
Copyright The Helm Authors.

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

package main

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"
	"time"

	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	helmtime "helm.sh/helm/v3/pkg/time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const manifestTemplate = `---
# Source: templates/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Namespace }}
  creationTimestamp: {{ .CreationTimestamp }}
---
# Source: templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: nami
  namespace: {{ .Namespace }}
  creationTimestamp: {{ .CreationTimestamp }}
data:
  attack: "Gomu Gomu no King Kong Gun!"
---
# Source: templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: zoro
  namespace: {{ .Namespace }}
  creationTimestamp: {{ .CreationTimestamp }}
spec:
  type: ClusterIP
  selector:
    app: one-piece
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
---
# Source: templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: luffy
  namespace: {{ .Namespace }}
  creationTimestamp: {{ .CreationTimestamp }}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: one-piece
  template:
    metadata:
      labels:
        app: one-piece
    spec:
      containers:
        - name: luffy-arsenal
          image: "nginx:1.21.6"
          ports:
            - containerPort: 80
          env:
            - name: ATTACK
              valueFrom:
                configMapKeyRef:
                  name: luffy
                  key: attack
`

func TestGetDeployed(t *testing.T) {
	const (
		namespace   = `thousand-sunny`
		releaseName = `one-piece`
	)
	var (
		is                           = assert.New(t)
		manifest                     bytes.Buffer
		relativeCreationTimestamp    = time.Now().Add(-2 * time.Minute)
		relativeCreationTimestampStr = relativeCreationTimestamp.Format(time.RFC3339)
		exactCreationTimestamp       = time.Date(2024, time.October, 28, 0, 4, 30, 0, time.FixedZone("IST", 19800))
		exactCreationTimestampStr    = exactCreationTimestamp.Format(time.RFC3339)
		scheme                       = runtime.NewScheme()
	)

	manifestTemplateParser, err := template.New("manifestTemplate").Parse(manifestTemplate)
	is.NoError(err)

	is.NoError(corev1.AddToScheme(scheme))
	is.NoError(appsv1.AddToScheme(scheme))

	restMapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
	configFlags := genericclioptions.NewTestConfigFlags().
		WithRESTMapper(restMapper)

	prepareReleaseFunc := func(name, namespace, timestamp string, manifest bytes.Buffer, info *release.Info) []*release.Release {
		err = manifestTemplateParser.Execute(&manifest, struct {
			Namespace         string
			CreationTimestamp string
		}{
			Namespace:         namespace,
			CreationTimestamp: timestamp,
		})
		is.NoError(err)

		return []*release.Release{{
			Name:      name,
			Namespace: namespace,
			Info:      info,
			Manifest:  manifest.String(),
		}}
	}

	tests := []cmdTestCase{
		{
			name:   "get deployed with release",
			cmd:    fmt.Sprintf("get deployed %s --namespace %s", releaseName, namespace),
			golden: "output/get-deployed.txt",
			rels: prepareReleaseFunc(
				releaseName,
				namespace,
				relativeCreationTimestampStr,
				manifest,
				&release.Info{
					LastDeployed: helmtime.Unix(relativeCreationTimestamp.Unix(), 0).UTC(),
					Status:       release.StatusDeployed,
				},
			),
			restClientGetter: configFlags,
			kubeClientOpts: &kubefake.Options{
				GetReturnResourceMap:    true,
				BuildReturnResourceList: true,
			},
		},
		{
			name:   "get deployed with release in json format",
			cmd:    fmt.Sprintf("get deployed %s --namespace %s --output json", releaseName, namespace),
			golden: "output/get-deployed.json",
			rels: prepareReleaseFunc(
				releaseName,
				namespace,
				exactCreationTimestampStr,
				manifest,
				&release.Info{
					LastDeployed: helmtime.Unix(exactCreationTimestamp.Unix(), 0).UTC(),
					Status:       release.StatusDeployed,
				},
			),
			restClientGetter: configFlags,
			kubeClientOpts: &kubefake.Options{
				GetReturnResourceMap:    true,
				BuildReturnResourceList: true,
			},
		},
		{
			name:   "get deployed with release in yaml format",
			cmd:    fmt.Sprintf("get deployed %s --namespace %s --output yaml", releaseName, namespace),
			golden: "output/get-deployed.yaml",
			rels: prepareReleaseFunc(
				releaseName,
				namespace,
				exactCreationTimestampStr,
				manifest,
				&release.Info{
					LastDeployed: helmtime.Unix(exactCreationTimestamp.Unix(), 0).UTC(),
					Status:       release.StatusDeployed,
				},
			),
			restClientGetter: configFlags,
			kubeClientOpts: &kubefake.Options{
				GetReturnResourceMap:    true,
				BuildReturnResourceList: true,
			},
		},
	}

	runTestCmd(t, tests)
}
