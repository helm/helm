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
package chart

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCRDs(t *testing.T) {
	chrt := Chart{
		Files: []*File{
			{
				Name: "crds/foo.yaml",
				Data: []byte("hello"),
			},
			{
				Name: "bar.yaml",
				Data: []byte("hello"),
			},
			{
				Name: "crds/foo/bar/baz.yaml",
				Data: []byte("hello"),
			},
			{
				Name: "crdsfoo/bar/baz.yaml",
				Data: []byte("hello"),
			},
		},
	}

	is := assert.New(t)
	crds := chrt.CRDs()
	is.Equal(2, len(crds))
	is.Equal("crds/foo.yaml", crds[0].Name)
	is.Equal("crds/foo/bar/baz.yaml", crds[1].Name)
}

func TestSaveChartNoRawData(t *testing.T) {
	chrt := Chart{
		Raw: []*File{
			{
				Name: "fhqwhgads.yaml",
				Data: []byte("Everybody to the Limit"),
			},
		},
	}

	is := assert.New(t)
	data, err := json.Marshal(chrt)
	if err != nil {
		t.Fatal(err)
	}

	res := &Chart{}
	if err := json.Unmarshal(data, res); err != nil {
		t.Fatal(err)
	}

	is.Equal([]*File(nil), res.Raw)
}
