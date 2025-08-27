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

package registry

import (
	"net"
	"os"
	"testing"

	"github.com/foxcpp/go-mockdns"
)

func TestMain(m *testing.M) {
	// A mock DNS server needed for TLS connection testing.
	var srv *mockdns.Server
	var err error

	srv, err = mockdns.NewServer(map[string]mockdns.Zone{
		"helm-test-registry.": {
			A: []string{"127.0.0.1"},
		},
	}, false)
	if err != nil {
		panic(err)
	}

	saveDialFunction := net.DefaultResolver.Dial
	srv.PatchNet(net.DefaultResolver)

	// Run all tests in the package
	code := m.Run()

	net.DefaultResolver.Dial = saveDialFunction
	_ = srv.Close()

	os.Exit(code)
}
