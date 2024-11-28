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

package chartutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type Foo struct {
	Bar   int
	Ipsum []string
}

func makeExample() any {
	return map[string]any{
		"ab": []string{"hello", "world"},
		"bc": "qwerty",
		"cd": &Foo{1000, []string{"blue", "red"}},
		"nested": map[string]any{
			"alpha": 5,
		},
		"nums": []int{3, 5, 7},
		"nums map": map[string]int{
			"a": 2,
			"b": 4,
		},
	}
}

func TestStringCollectionsDeepCopy(t *testing.T) {
	tests := []struct {
		name    string
		factory func() any
	}{
		{
			"nil",
			func() any { return nil },
		},
		{
			"[]string",
			func() any { return []string{"hello", "world"} },
		},
		{
			"[]int",
			func() any { return []int{2, 4, 6} },
		},
		{
			"map[string]any",
			func() any {
				return map[string]any{
					"ab": []string{"hello", "world"},
					"bc": "qwerty",
				}
			},
		},
		{
			"custom type",
			func() any {
				return Foo{1000, []string{"blue", "red"}}
			},
		},
		{
			"map[string]any with custom type",
			func() any {
				return map[string]any{
					"ab": []string{"hello", "world"},
					"bc": "qwerty",
					"cd": Foo{1000, []string{"blue", "red"}},
				}
			},
		},
		{
			"complex example", makeExample,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := tt.factory()
			got := stringCollectionsDeepCopy(src)

			assert.Equal(t, tt.factory(), got)
		})
	}
}

func TestStringCollectionsDeepCopyAliasing(t *testing.T) {
	is := assert.New(t)

	src := makeExample().(map[string]any)
	got := stringCollectionsDeepCopy(src).(map[string]any)

	got["ab"].([]string)[1] = "globe"
	is.Equal("world", src["ab"].([]string)[1])

	got["bc"] = "quartz"
	is.Equal("qwerty", src["bc"])

	got["nested"].(map[string]any)["alpha"] = "9"
	is.Equal(5, src["nested"].(map[string]any)["alpha"])

	// notice, that Foo is not deep copied
	got["cd"].(*Foo).Bar = 500
	got["cd"].(*Foo).Ipsum[0] = "green"
	is.Equal(500, src["cd"].(*Foo).Bar)
	is.Equal("green", src["cd"].(*Foo).Ipsum[0])

	// non-any collections are not deep copied
	got["nums"].([]int)[1] = 10
	is.Equal(10, src["nums"].([]int)[1])

	got["nums map"].(map[string]int)["b"] = 10
	is.Equal(10, src["nums map"].(map[string]int)["b"])
}
