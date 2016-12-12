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

package httputil

import (
	"fmt"
	"net/http"

	"k8s.io/helm/pkg/tlsutil"
)

// NewHTTPClientTLS constructs http.Client with configured TLS for http.Transport
func NewHTTPClientTLS(certFile, keyFile, caFile string) (*http.Client, error) {
	tlsConf, err := tlsutil.NewClientTLS(certFile, keyFile, caFile)
	if err != nil {
		return nil, fmt.Errorf("can't create TLS config for client: %s", err.Error())
	}
	tlsConf.BuildNameToCertificate()
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConf,
		},
	}
	return client, nil
}
