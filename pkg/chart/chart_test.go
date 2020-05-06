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
			{
				Name: "crds/README.md",
				Data: []byte("# hello"),
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

func TestMetadata(t *testing.T) {
	chrt := Chart{
		Metadata: &Metadata{
			Name:       "foo.yaml",
			AppVersion: "1.0.0",
			APIVersion: "v2",
			Version:    "1.0.0",
			Type:       "application",
		},
	}

	is := assert.New(t)

	is.Equal("foo.yaml", chrt.Name())
	is.Equal("1.0.0", chrt.AppVersion())
	is.Equal(nil, chrt.Validate())
}

func TestIsRoot(t *testing.T) {
	chrt1 := Chart{
		parent: &Chart{
			Metadata: &Metadata{
				Name: "foo",
			},
		},
	}

	chrt2 := Chart{
		Metadata: &Metadata{
			Name: "foo",
		},
	}

	is := assert.New(t)

	is.Equal(false, chrt1.IsRoot())
	is.Equal(true, chrt2.IsRoot())
}

func TestChartPath(t *testing.T) {
	chrt1 := Chart{
		parent: &Chart{
			Metadata: &Metadata{
				Name: "foo",
			},
		},
	}

	chrt2 := Chart{
		Metadata: &Metadata{
			Name: "foo",
		},
	}

	is := assert.New(t)

	is.Equal("foo.", chrt1.ChartPath())
	is.Equal("foo", chrt2.ChartPath())
}

func TestChartFullPath(t *testing.T) {
	chrt1 := Chart{
		parent: &Chart{
			Metadata: &Metadata{
				Name: "foo",
			},
		},
	}

	chrt2 := Chart{
		Metadata: &Metadata{
			Name: "foo",
		},
	}

	is := assert.New(t)

	is.Equal("foo/charts/", chrt1.ChartFullPath())
	is.Equal("foo", chrt2.ChartFullPath())
}

func TestCRDObjects(t *testing.T) {
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
			{
				Name: "crds/README.md",
				Data: []byte("# hello"),
			},
		},
	}

	expected := []CRD{
		{
			Name:     "crds/foo.yaml",
			Filename: "crds/foo.yaml",
			File: &File{
				Name: "crds/foo.yaml",
				Data: []byte("hello"),
			},
		},
		{
			Name:     "crds/foo/bar/baz.yaml",
			Filename: "crds/foo/bar/baz.yaml",
			File: &File{
				Name: "crds/foo/bar/baz.yaml",
				Data: []byte("hello"),
			},
		},
	}

	is := assert.New(t)
	crds := chrt.CRDObjects()
	is.Equal(expected, crds)
}
