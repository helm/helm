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

package postrender

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var goodYaml = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: hollow-men
data: |-
  This is the way the world ends
  This is the way the world ends
  This is the way the world ends
  Not with a bang but a whimper.
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: waste-land
data: |-
  To Carthage then I came	
  Burning burning burning burning
`

var duplicateYaml = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: hollow-men
data: |-
  This is the way the world ends
  This is the way the world ends
  This is the way the world ends
  Not with a bang but a whimper.
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: hollow-men
data: |-
  This is the way the world ends
  This is the way the world ends
  This is the way the world ends
  Not with a bang but a whimper.
`

var nonameYaml = `
---
apiVersion: v1
kind: ConfigMap
metadata:
data: |-
  This is the way the world ends
  This is the way the world ends
  This is the way the world ends
  Not with a bang but a whimper.
`
var unkindYaml = `
---
apiVersion: v1
metadata:
  name: hollow-men
data: |-
  This is the way the world ends
  This is the way the world ends
  This is the way the world ends
  Not with a bang but a whimper.
`

var hollowMen = `apiVersion: v1
kind: ConfigMap
metadata:
  name: hollow-men
data: |-
  This is the way the world ends
  This is the way the world ends
  This is the way the world ends
  Not with a bang but a whimper.`

func TestReparse(t *testing.T) {
	is := assert.New(t)
	res, err := Reparse([]byte(goodYaml))
	is.NoError(err, goodYaml)

	is.Len(res, 2, "two map entries")
	names := []string{"v1.ConfigMap.hollow-men.yaml", "v1.ConfigMap.waste-land.yaml"}
	for _, name := range names {
		content, ok := res[name]
		is.True(ok, "entry for %s exists", name)
		is.NotEmpty(content)
	}
	is.Equal(hollowMen, res[names[0]], "content matches")

	// duplicate failure
	_, err = Reparse([]byte(duplicateYaml))
	is.Error(err, "duplicate YAML fails to parse")

	// name is missing
	_, err = Reparse([]byte(nonameYaml))
	is.Error(err, "unnamed object fails to parse")

	// kind is missing
	_, err = Reparse([]byte(unkindYaml))
	is.Error(err, "kindless object fails to parse")
}
