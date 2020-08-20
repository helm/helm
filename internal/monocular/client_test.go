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

package monocular

import (
	"testing"
)

func TestNew(t *testing.T) {
	c, err := New("https://artifacthub.io")
	if err != nil {
		t.Errorf("error creating client: %s", err)
	}
	if c.BaseURL != "https://artifacthub.io" {
		t.Errorf("incorrect BaseURL. Expected \"https://artifacthub.io\" but got %q", c.BaseURL)
	}
}
