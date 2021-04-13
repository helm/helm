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

package getter

import (
	"path/filepath"
	"testing"
)

func TestNewOCIGetter(t *testing.T) {
	_, err := NewOCIGetter()
	if err != nil {
		t.Fatal(err)
	}

	// Test with options
	cd := "../../testdata"
	join := filepath.Join
	ca, pub, priv := join(cd, "rootca.crt"), join(cd, "crt.pem"), join(cd, "key.pem")
	insecure := true

	g, err := NewOCIGetter(
		WithInsecureSkipVerifyTLS(insecure),
		WithTLSClientConfig(pub, priv, ca),
	)

	if err != nil {
		t.Fatal(err)
	}

	og, ok := g.(*OCIGetter)
	if !ok {
		t.Fatal("expected NewOCIGetter to produce an *OCIGetter")
	}

	if og.opts.certFile != pub {
		t.Errorf("Expected NewOCIGetter to contain %q as the public key file, got %q", pub, og.opts.certFile)
	}

	if og.opts.keyFile != priv {
		t.Errorf("Expected NewOCIGetter to contain %q as the private key file, got %q", priv, og.opts.keyFile)
	}

	if og.opts.caFile != ca {
		t.Errorf("Expected NewOCIGetter to contain %q as the CA file, got %q", ca, og.opts.caFile)
	}

	if og.opts.insecureSkipVerifyTLS != insecure {
		t.Errorf("Expected NewOCIGetter to contain %t as InsecureSkipVerifyTLs flag, got %t", false, og.opts.insecureSkipVerifyTLS)
	}
}
