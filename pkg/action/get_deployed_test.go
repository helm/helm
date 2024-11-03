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

package action

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	texttemplate "text/template"
	"time"

	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	helmtime "helm.sh/helm/v3/pkg/time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metatable "k8s.io/apimachinery/pkg/api/meta/table"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	manifestTemplate = `---
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
	tableOutputTemplate = `NAMESPACE     	NAME                     	API_VERSION	AGE
              	namespaces/{{ .Namespace }}	v1         	{{ .Age }} 
{{ .Namespace }}	configmaps/nami          	v1         	{{ .Age }} 
{{ .Namespace }}	services/zoro            	v1         	{{ .Age }} 
{{ .Namespace }}	deployments/luffy        	apps/v1    	{{ .Age }} 
`
	jsonOutputTemplate = `[{` +
		`"name":"{{ .Namespace }}",` +
		`"namespace":"",` +
		`"apiVersion":"v1",` +
		`"resource":"namespaces",` +
		`"creationTimestamp":"{{ .CreationTimestamp }}"},{"name":"nami",` +
		`"namespace":"{{ .Namespace }}",` +
		`"apiVersion":"v1",` +
		`"resource":"configmaps",` +
		`"creationTimestamp":"{{ .CreationTimestamp }}"},{"name":"zoro",` +
		`"namespace":"{{ .Namespace }}",` +
		`"apiVersion":"v1",` +
		`"resource":"services",` +
		`"creationTimestamp":"{{ .CreationTimestamp }}"},{"name":"luffy",` +
		`"namespace":"{{ .Namespace }}",` +
		`"apiVersion":"apps/v1",` +
		`"resource":"deployments",` +
		`"creationTimestamp":"{{ .CreationTimestamp }}"` +
		`}]
`
	yamlOutputTemplate = `- apiVersion: v1
  creationTimestamp: "{{ .CreationTimestamp }}"
  name: {{ .Namespace }}
  namespace: ""
  resource: namespaces
- apiVersion: v1
  creationTimestamp: "{{ .CreationTimestamp }}"
  name: nami
  namespace: {{ .Namespace }}
  resource: configmaps
- apiVersion: v1
  creationTimestamp: "{{ .CreationTimestamp }}"
  name: zoro
  namespace: {{ .Namespace }}
  resource: services
- apiVersion: apps/v1
  creationTimestamp: "{{ .CreationTimestamp }}"
  name: luffy
  namespace: {{ .Namespace }}
  resource: deployments
`
)

type getDeployedOutputData struct {
	Namespace         string
	CreationTimestamp string
	Age               string
}

func TestGetDeployed(t *testing.T) {
	var (
		is                = assert.New(t)
		chartName         = `one-piece`
		namespace         = `thousand-sunny`
		exactTimestamp    = time.Date(2024, time.October, 28, 0, 4, 30, 0, time.FixedZone("IST", 19800)).UTC()
		relativeTimestamp = time.Now().Add(-2 * time.Minute).UTC()
	)

	type (
		testFunc struct {
			writeTable bool
			writeJSON  bool
			writeYAML  bool
		}

		testCase struct {
			name              string
			creationTimestamp time.Time
			testFunc          testFunc
		}
	)

	tests := []testCase{
		{
			name:              "With Exact Creation Time",
			creationTimestamp: exactTimestamp,
			testFunc: testFunc{
				writeTable: false,
				writeJSON:  true,
				writeYAML:  true,
			},
		},
		{
			name:              "With Relative Creation Time",
			creationTimestamp: relativeTimestamp,
			testFunc: testFunc{
				writeTable: true,
				writeJSON:  false,
				writeYAML:  false,
			},
		},
	}

	scheme := runtime.NewScheme()
	is.NoError(corev1.AddToScheme(scheme))
	is.NoError(appsv1.AddToScheme(scheme))
	restMapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
	configFlags := genericclioptions.NewTestConfigFlags().
		WithRESTMapper(restMapper)

	formatResourceList := func(creationTimestamp time.Time) []ResourceElement {
		creationTimestampStr := creationTimestamp.Format(time.RFC3339)

		manifest, err := parseGetDeployedTestTemplate(namespace, creationTimestampStr, "", manifestTemplate)
		is.NoError(err)

		config := actionConfigFixture(t)
		config.KubeClient = &kubefake.PrintingKubeClient{
			Out: io.Discard,
			Options: &kubefake.Options{
				GetReturnResourceMap:    true,
				BuildReturnResourceList: true,
			},
		}
		config.RESTClientGetter = configFlags

		client := NewGetDeployed(config)
		releases := []*release.Release{
			{
				Name: chartName,
				Info: &release.Info{
					LastDeployed: helmtime.Unix(creationTimestamp.Unix(), 0),
					Status:       release.StatusDeployed,
				},
				Manifest:  manifest.String(),
				Namespace: namespace,
			},
		}

		for _, rel := range releases {
			err = client.cfg.Releases.Create(rel)
			is.NoError(err)
		}

		resourceList, err := client.Run(chartName)
		is.NoError(err)

		return resourceList
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := assert.New(t)
			resourceList := formatResourceList(tc.creationTimestamp)
			is.NotEmpty(resourceList)

			writer := NewResourceListWriter(resourceList, false)
			creationTimestampStr := tc.creationTimestamp.Format(time.RFC3339)
			creationTimestampAgeStr := metatable.ConvertToHumanReadableDateType(metav1.NewTime(tc.creationTimestamp))

			var (
				out         bytes.Buffer
				expectedOut fmt.Stringer
				err         error
			)

			if tc.testFunc.writeTable {
				t.Run("Write Table", func(t *testing.T) {
					is := assert.New(t)
					out.Truncate(0)

					expectedOut, err = parseGetDeployedTestTemplate(
						namespace,
						"", // Creation timestamp is not used in table output, but creation timestamp's age
						creationTimestampAgeStr,
						tableOutputTemplate,
					)
					is.NoError(err)

					err = writer.WriteTable(&out)
					is.NoError(err)
					is.Equal(expectedOut.String(), out.String())
				})
			}

			if tc.testFunc.writeJSON {
				t.Run("Write JSON", func(t *testing.T) {
					is := assert.New(t)
					out.Truncate(0)

					expectedOut, err = parseGetDeployedTestTemplate(
						namespace,
						creationTimestampStr,
						"", // Creation timestamp's age is not used in JSON output, but the creation timestamp itself
						jsonOutputTemplate,
					)
					is.NoError(err)

					err = writer.WriteJSON(&out)
					is.NoError(err)
					is.Equal(expectedOut.String(), out.String())
				})
			}

			if tc.testFunc.writeYAML {
				t.Run("Write YAML", func(t *testing.T) {
					is := assert.New(t)
					out.Truncate(0)

					expectedOut, err = parseGetDeployedTestTemplate(
						namespace,
						creationTimestampStr,
						"", // Creation timestamp's age is not used in YAML output, but the creation timestamp itself
						yamlOutputTemplate,
					)
					is.NoError(err)

					err = writer.WriteYAML(&out)
					is.NoError(err)
					is.Equal(expectedOut.String(), out.String())
				})
			}
		})

	}
}

func parseGetDeployedTestTemplate(namespace, creationTimestamp, age, template string) (fmt.Stringer, error) {
	outputParser, err := texttemplate.New("template").Parse(template)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	err = outputParser.Execute(&out, getDeployedOutputData{
		Namespace:         namespace,
		CreationTimestamp: creationTimestamp,
		Age:               age,
	})
	if err != nil {
		return nil, err
	}

	return &out, nil
}

func TestGetDeployed_ErrorKubeClientNotReachable(t *testing.T) {
	is := assert.New(t)
	chartName := `one-piece`
	config := actionConfigFixture(t)
	config.KubeClient = &kubefake.PrintingKubeClient{
		Out: io.Discard,
		Options: &kubefake.Options{
			IsReachableReturnsError: true,
		},
	}

	client := NewGetDeployed(config)

	resourceList, err := client.Run(chartName)
	is.Nil(resourceList)
	is.Error(err)
	is.ErrorIs(err, kubefake.ErrPrintingKubeClientNotReachable)
}

func TestGetDeployed_ErrorReleaseNotFound(t *testing.T) {
	is := assert.New(t)
	chartName := `one-piece`
	config := actionConfigFixture(t)
	config.KubeClient = &kubefake.PrintingKubeClient{
		Out: io.Discard,
		Options: &kubefake.Options{
			IsReachableReturnsError: false,
		},
	}

	client := NewGetDeployed(config)

	resourceList, err := client.Run(chartName)
	is.Nil(resourceList)
	is.Error(err)
	is.Contains(err.Error(), "release: not found")
}

func TestGetDeployed_RESTMapperNotFound(t *testing.T) {
	var (
		is        = assert.New(t)
		chartName = `one-piece`
	)

	configFlags := genericclioptions.NewTestConfigFlags().
		WithRESTMapper(nil)

	config := actionConfigFixture(t)
	config.KubeClient = &kubefake.PrintingKubeClient{
		Out: io.Discard,
		Options: &kubefake.Options{
			GetReturnResourceMap:    true,
			BuildReturnResourceList: true,
		},
	}
	config.RESTClientGetter = configFlags

	client := NewGetDeployed(config)

	err := client.cfg.Releases.Create(&release.Release{
		Name: chartName,
		Info: &release.Info{},
	})
	is.NoError(err)

	resourceList, err := client.Run(chartName)
	is.Nil(resourceList)
	is.Error(err)
	is.Contains(err.Error(), "failed to extract the REST mapper: no restmapper")
}

func TestGetDeployed_ResourceListBuildFailure(t *testing.T) {
	var (
		is             = assert.New(t)
		chartName      = `one-piece`
		namespace      = `thousand-sunny`
		exactTimestamp = time.Date(2024, time.October, 28, 0, 4, 30, 0, time.FixedZone("IST", 19800)).UTC()
	)

	scheme := runtime.NewScheme()
	is.NoError(corev1.AddToScheme(scheme))
	is.NoError(appsv1.AddToScheme(scheme))
	restMapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
	configFlags := genericclioptions.NewTestConfigFlags().
		WithRESTMapper(restMapper)

	creationTimestampStr := exactTimestamp.Format(time.RFC3339)
	manifest, err := parseGetDeployedTestTemplate(namespace, creationTimestampStr, "", manifestTemplate)
	is.NoError(err)

	config := actionConfigFixture(t)
	config.KubeClient = &kubefake.PrintingKubeClient{
		Out: io.Discard,
		Options: &kubefake.Options{
			BuildReturnError: true,
		},
	}
	config.RESTClientGetter = configFlags

	client := NewGetDeployed(config)

	err = client.cfg.Releases.Create(&release.Release{
		Name: chartName,
		Info: &release.Info{
			LastDeployed: helmtime.Unix(exactTimestamp.Unix(), 0),
			Status:       release.StatusDeployed,
		},
		Manifest:  manifest.String(),
		Namespace: namespace,
	})
	is.NoError(err)

	resourceList, err := client.Run(chartName)
	is.Nil(resourceList)
	is.Error(err)
	is.ErrorIs(err, kubefake.ErrPrintingKubeClientBuildFailure)
}

func TestGetDeployed_GetResourceFailure(t *testing.T) {
	var (
		is             = assert.New(t)
		chartName      = `one-piece`
		namespace      = `thousand-sunny`
		exactTimestamp = time.Date(2024, time.October, 28, 0, 4, 30, 0, time.FixedZone("IST", 19800)).UTC()
	)

	scheme := runtime.NewScheme()
	is.NoError(corev1.AddToScheme(scheme))
	is.NoError(appsv1.AddToScheme(scheme))
	restMapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
	configFlags := genericclioptions.NewTestConfigFlags().
		WithRESTMapper(restMapper)

	creationTimestampStr := exactTimestamp.Format(time.RFC3339)
	manifest, err := parseGetDeployedTestTemplate(namespace, creationTimestampStr, "", manifestTemplate)
	is.NoError(err)

	config := actionConfigFixture(t)
	config.KubeClient = &kubefake.PrintingKubeClient{
		Out: io.Discard,
		Options: &kubefake.Options{
			GetReturnError: true,
		},
	}
	config.RESTClientGetter = configFlags

	client := NewGetDeployed(config)

	err = client.cfg.Releases.Create(&release.Release{
		Name: chartName,
		Info: &release.Info{
			LastDeployed: helmtime.Unix(exactTimestamp.Unix(), 0),
			Status:       release.StatusDeployed,
		},
		Manifest:  manifest.String(),
		Namespace: namespace,
	})
	is.NoError(err)

	resourceList, err := client.Run(chartName)
	is.Nil(resourceList)
	is.Error(err)
	is.ErrorIs(err, kubefake.ErrPrintingKubeClientGetFailure)
}

func TestGetDeployed_MissingGVK(t *testing.T) {
	var (
		is             = assert.New(t)
		chartName      = `one-piece`
		namespace      = `thousand-sunny`
		exactTimestamp = time.Date(2024, time.October, 28, 0, 4, 30, 0, time.FixedZone("IST", 19800)).UTC()
	)

	scheme := runtime.NewScheme()
	is.NoError(corev1.AddToScheme(scheme))
	restMapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
	configFlags := genericclioptions.NewTestConfigFlags().
		WithRESTMapper(restMapper)

	creationTimestampStr := exactTimestamp.Format(time.RFC3339)

	manifest, err := parseGetDeployedTestTemplate(namespace, creationTimestampStr, "", manifestTemplate)
	is.NoError(err)

	config := actionConfigFixture(t)
	config.KubeClient = &kubefake.PrintingKubeClient{
		Out: io.Discard,
		Options: &kubefake.Options{
			GetReturnResourceMap:    true,
			BuildReturnResourceList: true,
		},
	}
	config.RESTClientGetter = configFlags

	client := NewGetDeployed(config)
	err = client.cfg.Releases.Create(&release.Release{
		Name: chartName,
		Info: &release.Info{
			LastDeployed: helmtime.Unix(exactTimestamp.Unix(), 0),
			Status:       release.StatusDeployed,
		},
		Manifest:  manifest.String(),
		Namespace: namespace,
	})
	is.NoError(err)

	resourceList, err := client.Run(chartName)
	is.Nil(resourceList)
	is.Error(err)
	is.Contains(err.Error(), "no matches for kind \"Deployment\" in version \"apps/v1\"")
}
