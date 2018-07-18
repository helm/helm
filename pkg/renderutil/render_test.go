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

package renderutil

import (
	"testing"

	"github.com/stretchr/testify/require"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

const cmTemplate = `kind: ConfigMap
apiVersion: v1
metadata:
  name: example
data:
  Chart:
{{.Chart | toYaml | indent 4}}
  Release:
{{.Release | toYaml | indent 4}}
  Values:
{{.Values | toYaml | indent 4}}
`

func TestRender(t *testing.T) {

	testChart := &chart.Chart{
		Metadata: &chart.Metadata{Name: "hello"},
		Templates: []*chart.Template{
			{Name: "templates/cm.yaml", Data: []byte(cmTemplate)},
		},
		Values: &chart.Config{Raw: "meow: defaultmeow"},
	}

	newConfig := &chart.Config{Raw: "meow: newmeow"}
	defaultConfig := &chart.Config{Raw: "{}"}

	tests := map[string]struct {
		chart  *chart.Chart
		config *chart.Config
		opts   Options
		want   map[string]string
	}{
		"BasicWithValues": {
			chart:  testChart,
			config: newConfig,
			opts:   Options{},
			want: map[string]string{
				"hello/templates/cm.yaml": `kind: ConfigMap
apiVersion: v1
metadata:
  name: example
data:
  Chart:
    name: hello
    
  Release:
    IsInstall: false
    IsUpgrade: false
    Name: ""
    Namespace: ""
    Revision: 0
    Service: Tiller
    Time: null
    
  Values:
    meow: newmeow
    
`,
			},
		},
		"BasicNoValues": {
			chart:  testChart,
			config: defaultConfig,
			opts:   Options{},
			want: map[string]string{
				"hello/templates/cm.yaml": `kind: ConfigMap
apiVersion: v1
metadata:
  name: example
data:
  Chart:
    name: hello
    
  Release:
    IsInstall: false
    IsUpgrade: false
    Name: ""
    Namespace: ""
    Revision: 0
    Service: Tiller
    Time: null
    
  Values:
    meow: defaultmeow
    
`,
			},
		},
		"SetSomeReleaseValues": {
			chart:  testChart,
			config: defaultConfig,
			opts:   Options{ReleaseOptions: chartutil.ReleaseOptions{Name: "meow"}},
			want: map[string]string{
				"hello/templates/cm.yaml": `kind: ConfigMap
apiVersion: v1
metadata:
  name: example
data:
  Chart:
    name: hello
    
  Release:
    IsInstall: false
    IsUpgrade: false
    Name: meow
    Namespace: ""
    Revision: 0
    Service: Tiller
    Time: null
    
  Values:
    meow: defaultmeow
    
`,
			},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			got, err := Render(tt.chart, tt.config, tt.opts)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
