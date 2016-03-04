/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package kubectl

import (
	"testing"
)

func TestGet(t *testing.T) {
	Client = TestRunner{
		out: []byte("running the get command"),
	}

	expects := "running the get command"
	out, _ := Client.Get([]byte{}, "")
	if string(out) != expects {
		t.Errorf("%s != %s", string(out), expects)
	}
}

func TestGetByKind(t *testing.T) {
	Client = TestRunner{
		out: []byte("running the GetByKind command"),
	}

	expects := "running the GetByKind command"
	out, _ := Client.GetByKind("pods", "", "")
	if out != expects {
		t.Errorf("%s != %s", out, expects)
	}
}
