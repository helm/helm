/*
Copyright 2017 The Kubernetes Authors All rights reserved.
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

package getter

import (
	"net/http"
	"path/filepath"
	"testing"
)

func TestHTTPGetter(t *testing.T) {
	g, err := newHTTPGetter("http://example.com", "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	if hg, ok := g.(*httpGetter); !ok {
		t.Fatal("Expected newHTTPGetter to produce an httpGetter")
	} else if hg.client != http.DefaultClient {
		t.Fatal("Expected newHTTPGetter to return a default HTTP client.")
	}

	// Test with SSL:
	cd := "../../testdata"
	join := filepath.Join
	ca, pub, priv := join(cd, "ca.pem"), join(cd, "crt.pem"), join(cd, "key.pem")
	g, err = newHTTPGetter("http://example.com/", pub, priv, ca)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := g.(*httpGetter); !ok {
		t.Fatal("Expected newHTTPGetter to produce an httpGetter")
	}
}
